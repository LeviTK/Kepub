package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/LeviTK/Kepub/internal/app"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

// Deliver cancellation when the command next observes context after the output
// exists. This deterministically exercises the old post-commit ctx.Err rewrite.
type cancelAfterPublication struct {
	context.Context
	output string
	cancel context.CancelFunc
}

func (c cancelAfterPublication) Err() error {
	if _, e := os.Stat(c.output); e == nil {
		c.cancel()
	}
	return c.Context.Err()
}

func TestPackCommitCancellationSemantics(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"package.opf":            `<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:test:commit</dc:identifier><dc:title>Commit</dc:title><dc:language>en</dc:language></metadata><manifest><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="chapter"/></spine></package>`,
		"chapter.xhtml":          `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Commit</title></head><body><p>Unchanged.</p></body></html>`,
	}
	for path, b := range files {
		file := filepath.Join(dir, path)
		if e := os.MkdirAll(filepath.Dir(file), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(file, []byte(b), 0600); e != nil {
			t.Fatal(e)
		}
	}
	for _, before := range []bool{true, false} {
		out := filepath.Join(t.TempDir(), "output.epub")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var request context.Context = cancelAfterPublication{ctx, out, cancel}
		if before {
			cancel()
		}
		data, e := execute(request, options{command: "pack", book: dir, output: out, draft: true})
		if before {
			var f *fault.Error
			if !errors.As(e, &f) || f.Exit != 130 {
				t.Fatal("precommit cancellation", e)
			}
			if _, e := os.Stat(out); !errors.Is(e, os.ErrNotExist) {
				t.Fatal("cancelled publication exists", e)
			}
		} else {
			if e != nil {
				t.Fatal("postcommit cancelled success", e)
			}
			p := data.(app.PackResult)
			if p.ArchiveSHA256 == "" || !p.Draft || p.Verified {
				t.Fatal(p)
			}
			if !errors.Is(request.Err(), context.Canceled) {
				t.Fatal("postcommit cancellation not exercised")
			}
			if _, e := os.Stat(out); e != nil {
				t.Fatal("committed artifact absent", e)
			}
		}
	}
}

func invoke(t *testing.T, args []string, want int) map[string]any {
	t.Helper()
	var out, errout bytes.Buffer
	code := run(args, &out, &errout)
	if code != want {
		t.Fatalf("%v exit %d want %d stdout=%s stderr=%s", args, code, want, out.String(), errout.String())
	}
	var result map[string]any
	d := json.NewDecoder(&out)
	if e := d.Decode(&result); e != nil {
		t.Fatal("not envelope", e)
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		t.Fatalf("extra stdout %v %v", extra, e)
	}
	if result["schemaVersion"] != float64(1) || result["ok"] != (want == 0) || result["requestId"] == "" {
		t.Fatal(result)
	}
	if errout.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", errout.String())
	}
	return result
}

func TestJSONModeDoesNotInterpretOptionValues(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	for _, tc := range []struct {
		args []string
		json bool
	}{
		{[]string{"pack", missing, "--output", "--", "--json"}, true},
		{[]string{"pack", missing, "--output", "--json"}, false},
		{[]string{"pack", missing, "--output=--json"}, false},
		{[]string{"pack", missing, "--output=--", "--json"}, true},
		{[]string{"pack", missing, "--unknown", "--output", "--json"}, false},
		{[]string{"pack", missing, "--unknown", "--output", "--", "--json"}, true},
		{[]string{"pack", missing, "--unknown", "--", "--json"}, false},
	} {
		t.Run(fmt.Sprint(tc.args), func(t *testing.T) {
			var out, errout bytes.Buffer
			if code := run(tc.args, &out, &errout); code == 0 {
				t.Fatal("missing path/unknown option succeeded")
			}
			var e envelope
			machine := json.Unmarshal(out.Bytes(), &e) == nil && e.SchemaVersion == 1
			if machine != tc.json || tc.json && errout.Len() != 0 || !tc.json && errout.Len() == 0 {
				t.Fatalf("wrong mode: stdout=%s stderr=%s", out.String(), errout.String())
			}
		})
	}
}

func TestJSONSuccessFailureAndSelection(t *testing.T) {
	t.Setenv("KEPUB_EPUBCHECK_JAR", "")
	dir := t.TempDir()
	book := filepath.Join(dir, "含 空格.epub")
	testfixture.ZIP(t, book, testfixture.EPUB("3.0", false))
	info := invoke(t, []string{"info", book, "--json"}, 0)["data"].(map[string]any)
	for _, section := range []string{"metadata", "manifest", "spine", "navigation", "references", "capabilities"} {
		r := invoke(t, []string{"inspect", book, "--section", section, "--json"}, 0)
		data := r["data"].(map[string]any)
		if data["rootfile"] != info["rootfile"] || data["section"] != section {
			t.Fatal(data)
		}
		if section == "manifest" {
			a, _ := json.Marshal(data["value"])
			b, _ := json.Marshal(info["manifest"])
			if !bytes.Equal(a, b) {
				t.Fatal("different inspect/info use case")
			}
		}
	}
	for _, tc := range []struct {
		args []string
		exit int
		code string
	}{
		{[]string{"info"}, 2, "INVALID_ARGUMENT"},
		{[]string{"info", book, "--bogus"}, 2, "INVALID_ARGUMENT"},
		{[]string{"--json", "info", book, "--bogus"}, 2, "INVALID_ARGUMENT"},
		{[]string{"info", book, "--rootfile", "书/Deep/package.opf", "--rootfile", "alternate.opf"}, 2, "INVALID_ARGUMENT"},
		{[]string{"toc", book, "--resource", "x"}, 2, "INVALID_ARGUMENT"},
		{[]string{"pack", dir}, 2, "INVALID_ARGUMENT"},
		{[]string{"validate", book}, 3, "DEPENDENCY_UNAVAILABLE"},
		{[]string{"validate", book, "--draft"}, 2, "INVALID_ARGUMENT"},
		{[]string{"validate", book, "--timeout", "0"}, 2, "INVALID_ARGUMENT"},
		{[]string{"info", book, "--strict"}, 2, "INVALID_ARGUMENT"},
		{[]string{"info", filepath.Join(dir, "missing")}, 6, "IO_ERROR"},
		{[]string{"inspect", book, "--section", "references", "--direction", "incoming"}, 2, "INVALID_ARGUMENT"},
		{[]string{"inspect", book, "--section", "references", "--resource", "x", "--direction", "sideways"}, 2, "INVALID_ARGUMENT"},
		{[]string{"inspect", book, "--section", "references", "--resource", "../x"}, 2, "INVALID_ARGUMENT"},
		{[]string{"inspect", filepath.Join(dir, "missing"), "--section", "references", "--direction", "incoming"}, 2, "INVALID_ARGUMENT"},
		{[]string{"inspect", book, "--section", "navigation", "--resource", "x"}, 2, "INVALID_ARGUMENT"},
		{[]string{"info", dir}, 3, "UNSUPPORTED_INPUT"},
	} {
		args := append(tc.args, "--json")
		r := invoke(t, args, tc.exit)
		if r["error"].(map[string]any)["code"] != tc.code {
			t.Fatal(r)
		}
		if tc.args[0] == "--json" && r["command"] != "info" {
			t.Fatal("lost command on parse error", r)
		}
	}
	multi := filepath.Join(dir, "multi.epub")
	testfixture.ZIP(t, multi, testfixture.EPUB("3.0", true))
	invoke(t, []string{"info", multi, "--json"}, 2)
	r := invoke(t, []string{"info", multi, "--rootfile", "alternate.opf", "--json"}, 0)
	if r["data"].(map[string]any)["version"] != "2.0" {
		t.Fatal(r)
	}
	cap := invoke(t, []string{"capabilities", "--json"}, 0)
	statuses := map[string]string{}
	for _, v := range cap["data"].([]any) {
		c := v.(map[string]any)
		statuses[c["operationId"].(string)] = c["implementationStatus"].(string)
	}
	for _, id := range []string{"version", "doctor", "publication.inspect", "publication.validate", "publication.pack", "metadata.set", "plan", "apply", "workspace.open", "workspace.export", "task.diff", "task.accept", "task.reject"} {
		if statuses[id] != "available" {
			t.Fatalf("missing implemented capability %s", id)
		}
	}
	for _, id := range []string{"resource.rename", "workspace.list", "amp", "preview"} {
		if statuses[id] != "planned" {
			t.Fatalf("overstated capability %s", id)
		}
	}
}

func TestUnpackFailureAndBytes(t *testing.T) {
	dir := t.TempDir()
	book := filepath.Join(dir, "book.epub")
	entries := testfixture.EPUB("2.0", false)
	testfixture.ZIP(t, book, entries)
	output := filepath.Join(dir, "output")
	invoke(t, []string{"unpack", book, "-o", output, "--json"}, 0)
	for _, entry := range entries {
		if entry.Mode.IsDir() {
			continue
		}
		b, e := os.ReadFile(filepath.Join(output, entry.Name))
		if e != nil || !bytes.Equal(b, entry.Data) {
			t.Fatal("byte preservation", entry.Name, e)
		}
	}
	invoke(t, []string{"unpack", book, "--output", output, "--json"}, 2)
	entries[2].Data = []byte(`<package>`)
	bad := filepath.Join(dir, "bad.epub")
	testfixture.ZIP(t, bad, entries)
	dest := filepath.Join(dir, "failed")
	r := invoke(t, []string{"unpack", bad, "--output", dest, "--json"}, 1)
	if r["error"].(map[string]any)["code"] != "XML_NOT_WELL_FORMED" {
		t.Fatal(r)
	}
	if _, e := os.Lstat(dest); !os.IsNotExist(e) {
		t.Fatal("partial output", e)
	}
}

func TestCLIProcessSmoke(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "kepub")
	build := exec.Command("go", "build", "-o", binary, ".")
	if out, e := build.CombinedOutput(); e != nil {
		t.Fatalf("build: %s %v", out, e)
	}
	book := filepath.Join(dir, "-含 空格.epub")
	testfixture.ZIP(t, book, testfixture.ReferenceEPUB())
	testfixture.ZIP(t, filepath.Join(dir, "ncx.epub"), testfixture.NavigationEPUB("2.0"))
	before, err := os.ReadFile(book)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"info", "--json", "--", "-含 空格.epub"}, 0},
		{[]string{"inspect", "--section=manifest", "--json", "--", "-含 空格.epub"}, 0},
		{[]string{"toc", "--json", "--", "-含 空格.epub"}, 0},
		{[]string{"toc", "--json", "--", "ncx.epub"}, 0},
		{[]string{"inspect", "--section=navigation", "--json", "--", "-含 空格.epub"}, 0},
		{[]string{"inspect", "--section=references", "--resource=书/Text/-first.xhtml", "--direction=outgoing", "--json", "--", "-含 空格.epub"}, 0},
		{[]string{"inspect", "--section=references", "--resource=书/Text/-first.xhtml", "--direction=incoming", "--json", "--", "-含 空格.epub"}, 0},
		{[]string{"inspect", "--section=references", "--direction=incoming", "--json", "--", "-含 空格.epub"}, 2},
		{[]string{"info", "--json", "--", "missing.epub"}, 6},
	} {
		c := exec.Command(binary, tc.args...)
		c.Dir = dir
		var out, stderr bytes.Buffer
		c.Stdout = &out
		c.Stderr = &stderr
		e := c.Run()
		code := 0
		if e != nil {
			ee, ok := e.(*exec.ExitError)
			if !ok {
				t.Fatal(e)
			}
			code = ee.ExitCode()
		}
		if code != tc.code {
			t.Fatalf("process exit %d: %s %s", code, out.String(), stderr.String())
		}
		d := json.NewDecoder(&out)
		var env envelope
		if e = d.Decode(&env); e != nil {
			t.Fatal(e)
		}
		var extra any
		if e = d.Decode(&extra); e != io.EOF {
			t.Fatal("extra output", e)
		}
		if env.OK != (tc.code == 0) {
			t.Fatal(env)
		}
		if stderr.Len() != 0 {
			t.Fatal("unexpected process stderr", stderr.String())
		}
		if env.OK && (tc.args[0] == "toc" || tc.args[1] == "--section=navigation") {
			v := env.Data.(map[string]any)["value"].(map[string]any)
			label := "Part A"
			if tc.args[len(tc.args)-1] == "ncx.epub" {
				label = "Second & final"
				if v["format"] != "epub2-ncx" {
					t.Fatal("wrong NCX process format", v)
				}
			}
			if v["status"] != "complete" || len(v["entries"].([]any)) != 2 || v["entries"].([]any)[0].(map[string]any)["label"] != label {
				t.Fatal("process returned wrong TOC", env.Data)
			}
		}
		t.Logf("kepub %v: exit=%d, single JSON envelope, no stderr", tc.args, code)
		if env.OK && tc.args[1] == "--section=references" {
			v := env.Data.(map[string]any)["value"].(map[string]any)
			want := 2
			if v["direction"] == "incoming" {
				want = 7
			}
			if len(v["edges"].([]any)) != want || v["status"] != "partial" || len(v["coverage"].([]any)) == 0 || len(v["diagnostics"].([]any)) == 0 {
				t.Fatal("process lost edges/coverage/diagnostics", env.Data)
			}
		}
	}
	after, err := os.ReadFile(book)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("CLI changed original archive", err)
	}
}

func TestTOCAliasAndBlockedQueryData(t *testing.T) {
	for _, version := range []string{"2.0", "3.0"} {
		book := filepath.Join(t.TempDir(), "book.epub")
		entries := testfixture.NavigationEPUB(version)
		testfixture.ZIP(t, book, entries)
		toc := invoke(t, []string{"toc", book, "--json"}, 0)
		inspect := invoke(t, []string{"inspect", book, "--section", "navigation", "--json"}, 0)
		if !reflect.DeepEqual(toc["data"], inspect["data"]) {
			t.Fatal("toc and inspect diverged", toc, inspect)
		}
		for i := range entries {
			if version == "2.0" && entries[i].Name == "书/toc.ncx" || version == "3.0" && entries[i].Name == "书/nav.xhtml" {
				entries[i].Data = []byte(`<broken>`)
			}
		}
		testfixture.ZIP(t, book, entries)
		r := invoke(t, []string{"toc", book, "--json"}, 0)["data"].(map[string]any)["value"].(map[string]any)
		if r["status"] != "blocked" || len(r["entries"].([]any)) != 0 || r["diagnostics"].([]any)[0].(map[string]any)["code"] != "XML_NOT_WELL_FORMED" {
			t.Fatal("query success hid blocked navigation", r)
		}
		r = invoke(t, []string{"inspect", book, "--section", "references", "--json"}, 0)["data"].(map[string]any)["value"].(map[string]any)
		if r["status"] != "partial" || len(r["diagnostics"].([]any)) == 0 {
			t.Fatal("query success hid blocked references", r)
		}
	}
}
