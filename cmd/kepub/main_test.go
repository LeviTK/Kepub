package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/LeviTK/Kepub/internal/testfixture"
)

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

func TestJSONSuccessFailureAndSelection(t *testing.T) {
	dir := t.TempDir()
	book := filepath.Join(dir, "含 空格.epub")
	testfixture.ZIP(t, book, testfixture.EPUB("3.0", false))
	info := invoke(t, []string{"info", book, "--json"}, 0)["data"].(map[string]any)
	for _, section := range []string{"metadata", "manifest", "spine", "capabilities"} {
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
		{[]string{"toc", book}, 3, "CAPABILITY_UNAVAILABLE"},
		{[]string{"pack", dir}, 3, "CAPABILITY_UNAVAILABLE"},
		{[]string{"validate", book}, 3, "CAPABILITY_UNAVAILABLE"},
		{[]string{"info", filepath.Join(dir, "missing")}, 6, "IO_ERROR"},
		{[]string{"inspect", book, "--section", "references"}, 3, "CAPABILITY_UNAVAILABLE"},
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
	available := 0
	for _, v := range cap["data"].([]any) {
		c := v.(map[string]any)
		if c["implementationStatus"] == "available" {
			available++
		}
	}
	if available != 3 {
		t.Fatal("overstated capabilities", cap)
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
	testfixture.ZIP(t, book, testfixture.EPUB("3.0", false))
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"info", "--json", "--", "-含 空格.epub"}, 0},
		{[]string{"inspect", "--section=manifest", "--json", "--", "-含 空格.epub"}, 0},
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
	}
}
