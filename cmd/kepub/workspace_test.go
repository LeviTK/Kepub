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
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/app"
	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/testfixture"
	"github.com/LeviTK/Kepub/internal/workspace"
	"golang.org/x/sys/unix"
)

func workspaceBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "kepub")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %v %s", err, out)
	}
	return binary
}
func processJSON(t *testing.T, binary string, args []string, want int, timeout time.Duration) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, append(args, "--json")...)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("command did not finish within bound: %v %v", args, ctx.Err())
	}
	exit := 0
	if err != nil {
		e, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		exit = e.ExitCode()
	}
	if exit != want {
		t.Fatalf("%v exit %d want %d: %s %s", args, exit, want, out.String(), stderr.String())
	}
	decoder := json.NewDecoder(&out)
	var env map[string]any
	if err := decoder.Decode(&env); err != nil {
		t.Fatal(err, out.String())
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatal("extra stdout", err)
	}
	if stderr.Len() != 0 || env["ok"] != (want == 0) || env["schemaVersion"] != float64(1) || env["requestId"] == "" {
		t.Fatalf("bad envelope: %v stderr=%s", env, stderr.String())
	}
	return env
}
func legalCLI(t *testing.T, version string) (string, map[string][]byte) {
	t.Helper()
	dir := t.TempDir()
	meta := `<meta property="dcterms:modified">2026-10-04T00:00:00Z</meta>`
	nav := `<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`
	spine := `<spine>`
	if version == "2.0" {
		meta = ""
		nav = `<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>`
		spine = `<spine toc="ncx">`
	}
	files := map[string][]byte{
		"mimetype":               []byte("application/epub+zip"),
		"META-INF/container.xml": []byte(`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`),
		"EPUB/package.opf":       []byte(fmt.Sprintf(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="%s" unique-identifier="uid"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:uuid:12345678-1234-1234-1234-123456789012</dc:identifier><!-- untouched Title --><dc:title>Title</dc:title><dc:creator>Writer</dc:creator><dc:language>en</dc:language>%s</metadata><manifest><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/><item id="css" href="style.css" media-type="text/css"/>%s</manifest>%s<itemref idref="chapter"/></spine></package>`, version, meta, nav, spine) + "\r\n"),
		"EPUB/chapter.xhtml":     []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter</title><link rel="stylesheet" type="text/css" href="style.css"/></head><body><h1 id="start">Chapter</h1><p>Original &amp; precise.</p></body></html>` + "\r\n"),
		"EPUB/style.css":         []byte("p { color: #123456; }\r\n"),
	}
	if version == "3.0" {
		files["EPUB/nav.xhtml"] = []byte(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="chapter.xhtml#start">Chapter</a></li></ol></nav></body></html>`)
	} else {
		files["EPUB/toc.ncx"] = []byte(`<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head><meta name="dtb:uid" content="urn:uuid:12345678-1234-1234-1234-123456789012"/><meta name="dtb:depth" content="1"/><meta name="dtb:totalPageCount" content="0"/><meta name="dtb:maxPageNumber" content="0"/></head><docTitle><text>Title</text></docTitle><navMap><navPoint id="n1" playOrder="1"><navLabel><text>Chapter</text></navLabel><content src="chapter.xhtml#start"/></navPoint></navMap></ncx>`)
	}
	for name, b := range files {
		file := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	a, tree, err := archive.SnapshotDirectory(dir, archive.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	book := filepath.Join(t.TempDir(), "book.epub")
	f, err := os.Create(book)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.WriteZIP(f, tree); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return book, files
}
func operationFile(t *testing.T, file, field, old, new string) {
	t.Helper()
	v := map[string]any{"schemaVersion": 1, "operations": []any{map[string]any{"operationId": "metadata.set", "operationVersion": 1, "params": map[string]any{"namespace": "http://purl.org/dc/elements/1.1/", "localName": field, "expectedOldValue": old, "newValue": new}}}}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, b, 0600); err != nil {
		t.Fatal(err)
	}
}
func checkExport(t *testing.T, file string, expected map[string][]byte) {
	t.Helper()
	a, err := archive.Open(file, archive.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	for name, want := range expected {
		b, err := a.Read(bookpath.BookPath(name), 8<<20)
		if err != nil || !bytes.Equal(b, want) {
			t.Fatalf("export resource %s changed: %v", name, err)
		}
	}
}

func TestCLIWorkspaceLifecycle(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("real pinned EPUBCheck required")
	}
	binary := workspaceBinary(t)
	for _, version := range []string{"2.0", "3.0"} {
		t.Run(version, func(t *testing.T) {
			book, files := legalCLI(t, version)
			original, err := os.ReadFile(book)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			ws := filepath.Join(dir, "ws")
			ops := filepath.Join(dir, "ops.json")
			plan := filepath.Join(dir, "plan.json")
			run := func(args []string, want int) map[string]any {
				return processJSON(t, binary, args, want, 45*time.Second)
			}
			opened := run([]string{"workspace", "open", book, "--output", ws}, 0)["data"].(map[string]any)
			if opened["conformance"] != "not_run" || opened["workspaceId"] == "" {
				t.Fatal("import pretended conformance", opened)
			}
			refusal := run([]string{"workspace", "open", book, "--output", ws}, 2)
			if refusal["error"].(map[string]any)["code"] != "OUTPUT_EXISTS" {
				t.Fatal("existing output was not a policy refusal", refusal)
			}
			operationFile(t, ops, "title", "Title", "New < & >")
			for _, root := range []string{"original", "revisions/initial/pub", "."} {
				out := filepath.Join(ws, root, "absent-new-file")
				run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", out}, 2)
				run([]string{"workspace", "export", ws, "--output", out, "--draft"}, 2)
				if _, err := os.Lstat(out); !os.IsNotExist(err) {
					t.Fatal("output polluted protected tree")
				}
			}
			run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", plan}, 0)
			run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", plan}, 2)
			e := run([]string{"apply", "--workspace", ws, "--plan", plan}, 0)["data"].(map[string]any)
			task := e["taskId"].(string)
			if e["reviewRequired"] != true || e["conformance"] != "not_run" {
				t.Fatal(e)
			}
			out := filepath.Join(ws, "tasks/active/work/pub", "absent-new-file")
			run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", out}, 2)
			run([]string{"workspace", "export", ws, "--output", out, "--draft"}, 2)
			if _, err := os.Lstat(out); !os.IsNotExist(err) {
				t.Fatal("candidate polluted by output")
			}
			diff := run([]string{"task", "diff", task, "--workspace", ws}, 0)["data"].(map[string]any)
			m := diff["metadata"].(map[string]any)
			if m["oldValue"] != "Title" || m["newValue"] != "New < & >" || len(diff["diff"].(map[string]any)["changes"].([]any)) != 1 {
				t.Fatal(diff)
			}
			pre := filepath.Join(dir, "preaccept.epub")
			result := run([]string{"workspace", "export", ws, "--output", pre}, 0)["data"].(map[string]any)
			if result["revisionId"] != "initial" || result["verified"] != true {
				t.Fatal(result)
			}
			checkExport(t, pre, files)
			accepted := run([]string{"task", "accept", task, "--workspace", ws}, 0)["data"].(map[string]any)
			if accepted["status"] != "accepted" || accepted["validation"].(map[string]any)["status"] != "pass" {
				t.Fatal(accepted)
			}
			run([]string{"task", "accept", task, "--workspace", ws}, 4)
			run([]string{"apply", "--workspace", ws, "--plan", plan}, 4)
			plan2 := filepath.Join(dir, "plan2.json")
			operationFile(t, ops, "creator", "Writer", "Next & Δ")
			p2 := run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", plan2}, 0)["data"].(map[string]any)
			if p2["baseRevision"] != accepted["revisionId"] {
				t.Fatal("second baseline stale", p2)
			}
			e2 := run([]string{"apply", "--workspace", ws, "--plan", plan2}, 0)["data"].(map[string]any)
			run([]string{"task", "accept", e2["taskId"].(string), "--workspace", ws}, 0)
			files["EPUB/package.opf"] = bytes.Replace(files["EPUB/package.opf"], []byte(`>Title</dc:title>`), []byte(`>New &lt; &amp; &gt;</dc:title>`), 1)
			files["EPUB/package.opf"] = bytes.Replace(files["EPUB/package.opf"], []byte(`>Writer</dc:creator>`), []byte(`>Next &amp; Δ</dc:creator>`), 1)
			final := filepath.Join(dir, "final.epub")
			exported := run([]string{"workspace", "export", ws, "--output", final}, 0)["data"].(map[string]any)
			if exported["verified"] != true || exported["validation"].(map[string]any)["status"] != "pass" {
				t.Fatal(exported)
			}
			checkExport(t, final, files)
			run([]string{"workspace", "export", ws, "--output", final}, 2)
			checkExport(t, final, files)
			before, err := os.ReadFile(book)
			if err != nil || !bytes.Equal(before, original) {
				t.Fatal("original book changed", err)
			}
			// No-op and reject use the advanced baseline and explicit IDs.
			operationFile(t, ops, "creator", "Next & Δ", "Next & Δ")
			noop := filepath.Join(dir, "noop.json")
			run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", noop}, 0)
			ne := run([]string{"apply", "--workspace", ws, "--plan", noop}, 0)["data"].(map[string]any)
			if ne["diff"].(map[string]any)["changed"] != false {
				t.Fatal("no-op changed bytes", ne)
			}
			run([]string{"task", "reject", ne["taskId"].(string), "--workspace", ws}, 0)
			run([]string{"apply", "--workspace", ws, "--plan", noop}, 4)
			operationFile(t, ops, "title", "New < & >", "Next")
			next := filepath.Join(dir, "next.json")
			run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", next}, 0)
			run([]string{"apply", "--workspace", ws, "--plan", next}, 0)
		})
	}
}

func TestEditFIFOInputDoesNotBlock(t *testing.T) {
	binary := workspaceBinary(t)
	dir := t.TempDir()
	book := filepath.Join(dir, "book.epub")
	testfixture.ZIP(t, book, testfixture.EPUB("3.0", false))
	ws := filepath.Join(dir, "ws")
	processJSON(t, binary, []string{"workspace", "open", book, "--output", ws}, 0, 3*time.Second)
	fifo := filepath.Join(dir, "no-writer.fifo")
	if err := unix.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"plan", "--workspace", ws, "--operations", fifo, "--output", filepath.Join(dir, "plan.json")}, {"apply", "--workspace", ws, "--plan", fifo}} {
		processJSON(t, binary, args, 2, 3*time.Second)
		if _, err := os.Lstat(filepath.Join(ws, "tasks/active")); !os.IsNotExist(err) {
			t.Fatal("FIFO input generated candidate")
		}
	}
}

func TestWorkspaceArgumentsAndPlannedBoundaries(t *testing.T) {
	for _, args := range [][]string{{"workspace", "open"}, {"workspace", "export", "x"}, {"plan", "x", "--workspace", "w", "--operations", "o", "--output", "p"}, {"apply", "p", "--workspace", "w"}, {"task", "accept", "t", "--workspace", "w", "--draft"}, {"task", "diff", "t", "--workspace", "w", "--strict"}, {"workspace", "open", "b", "--output", "w", "--plan", "p"}, {"info", "b", "--workspace", "w"}, {"workspace", "open", "b", "--output", "w", "extra"}, {"task", "accept", "latest", "--passed", "true"}} {
		invoke(t, append(args, "--json"), 2)
	}
	invoke(t, []string{"workspace", "list", "--json"}, 3)
	invoke(t, []string{"task", "run", "--json"}, 3)
}

func TestWorkspaceAcceptCommitCancellationSemantics(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("real pinned EPUBCheck required")
	}
	book, _ := legalCLI(t, "3.0")
	dir := t.TempDir()
	ws := filepath.Join(dir, "ws")
	ops := filepath.Join(dir, "ops.json")
	plan := filepath.Join(dir, "plan.json")
	if _, err := app.OpenWorkspace(t.Context(), book, ws, ""); err != nil {
		t.Fatal(err)
	}
	operationFile(t, ops, "title", "Title", "Accepted")
	if _, err := app.PlanWorkspace(t.Context(), ws, ops, plan); err != nil {
		t.Fatal(err)
	}
	e, err := app.ApplyWorkspace(t.Context(), ws, plan)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observing := cancelAfterPublication{Context: ctx, output: filepath.Join(ws, "accepted.json"), cancel: cancel}
	data, err := execute(observing, options{command: "task", action: "accept", workspace: ws, book: e.TaskID})
	d, ok := data.(workspace.Decision)
	if err != nil || !ok || d.Status != "accepted" {
		t.Fatalf("committed accept rewritten to cancellation: %+v %v", data, err)
	}
	if !errors.Is(observing.Err(), context.Canceled) {
		t.Fatal("late cancellation not delivered")
	}
	w, err := workspace.Open(ws)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	a, _, r, err := w.AcceptedSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	a.Close()
	if r.ID != d.RevisionID || r.Validation.Status != "pass" {
		t.Fatal("late cancellation revoked durable acceptance")
	}
}
