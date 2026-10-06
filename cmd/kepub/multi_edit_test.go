package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
)

// multiCLI is a conformance-positive EPUB3 with two chapters so the real
// binary can exercise a multi-operation, multi-resource transaction.
func multiCLI(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	dir := t.TempDir()
	files := map[string][]byte{
		"mimetype":               []byte("application/epub+zip"),
		"META-INF/container.xml": []byte(`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`),
		"EPUB/package.opf":       []byte(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:uuid:12345678-1234-1234-1234-123456789012</dc:identifier><dc:title id="title">Title</dc:title><dc:creator id="creator">Writer</dc:creator><dc:language>en</dc:language><meta property="dcterms:modified">2026-10-04T00:00:00Z</meta></metadata><manifest><item id="c1" href="chapter1.xhtml" media-type="application/xhtml+xml"/><item id="c2" href="chapter2.xhtml" media-type="application/xhtml+xml"/><item id="css" href="style.css" media-type="text/css"/><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>` + "\r\n"),
		"EPUB/chapter1.xhtml":    []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>One</title></head><body><p>Alpha &amp; one.</p><p>Second node.</p></body></html>` + "\r\n"),
		"EPUB/chapter2.xhtml":    []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Two</title></head><body><p>Beta &amp; two.</p></body></html>` + "\r\n"),
		"EPUB/style.css":         []byte("p { color: #123456; }\r\n"),
		"EPUB/nav.xhtml":         []byte(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="chapter1.xhtml">One</a></li><li><a href="chapter2.xhtml">Two</a></li></ol></nav></body></html>`),
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

func TestMultiOperationBinaryLifecycle(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("real pinned EPUBCheck required")
	}
	binary := workspaceBinary(t)
	book, files := multiCLI(t)
	original, err := os.ReadFile(book)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ws := filepath.Join(dir, "ws")
	opsPath := filepath.Join(dir, "ops.json")
	planPath := filepath.Join(dir, "plan.json")
	run := func(args []string, want int) map[string]any {
		return processJSON(t, binary, args, want, 60*time.Second)
	}
	run([]string{"workspace", "open", book, "--output", ws}, 0)
	contentData := func(resource string) map[string]any {
		return run([]string{"content", "--workspace", ws, "--resource", resource}, 0)["data"].(map[string]any)
	}
	contentOp := func(resource, new string) map[string]any {
		c := contentData(resource)
		for _, node := range c["nodes"].([]any) {
			n := node.(map[string]any)
			if n["localName"] == "p" {
				return map[string]any{"operationId": "content.text.set", "operationVersion": 1, "params": map[string]any{
					"bookPath": c["bookPath"], "revisionId": c["revisionId"], "resourceSha256": c["resourceSha256"],
					"locatorVersion": c["locatorVersion"], "locator": n["locator"], "expectedOldValue": n["text"], "newValue": new}}
			}
		}
		t.Fatal("missing p node in", resource)
		return nil
	}
	metadataOp := map[string]any{"operationId": "metadata.set", "operationVersion": 1, "params": map[string]any{
		"namespace": "http://purl.org/dc/elements/1.1/", "localName": "title", "id": "title", "expectedOldValue": "Title", "newValue": "Multi < & >"}}
	writeOps := func(ops []any) {
		t.Helper()
		b, err := json.Marshal(map[string]any{"schemaVersion": 3, "operations": ops})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(opsPath, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// A duplicate target is rejected before any plan output exists.
	dup := contentOp("EPUB/chapter1.xhtml", "dup")
	writeOps([]any{dup, dup})
	r := run([]string{"plan", "--workspace", ws, "--operations", opsPath, "--output", planPath}, 2)
	if r["error"].(map[string]any)["code"] != "INVALID_OPERATIONS" {
		t.Fatal(r)
	}
	if _, err := os.Stat(planPath); !os.IsNotExist(err) {
		t.Fatal("rejected multi-operation request published a plan", err)
	}
	writeOps([]any{metadataOp, contentOp("EPUB/chapter1.xhtml", "First < & > 😀"), contentOp("EPUB/chapter2.xhtml", "Second changed")})
	p := run([]string{"plan", "--workspace", ws, "--operations", opsPath, "--output", planPath}, 0)["data"].(map[string]any)
	if p["schemaVersion"] != float64(3) || !reflect.DeepEqual(p["writeSet"], []any{"EPUB/chapter1.xhtml", "EPUB/chapter2.xhtml", "EPUB/package.opf"}) {
		t.Fatal(p)
	}
	e := run([]string{"apply", "--workspace", ws, "--plan", planPath}, 0)["data"].(map[string]any)
	task := e["taskId"].(string)
	if e["version"] != float64(3) || e["reviewRequired"] != true || e["conformance"] != "not_run" {
		t.Fatal(e)
	}
	review := run([]string{"task", "diff", task, "--workspace", ws}, 0)["data"].(map[string]any)
	if review["matchesExecution"] != true || len(review["diff"].(map[string]any)["changes"].([]any)) != 3 {
		t.Fatal(review)
	}
	operations := review["operations"].([]any)
	for i, want := range []string{"Multi < & >", "First < & > 😀", "Second changed"} {
		op := operations[i].(map[string]any)
		if op["newValue"] != want || op["plannedValue"] != want {
			t.Fatalf("operation %d: %v", i, op)
		}
	}
	// Accepted content stays untouched while the candidate holds all writes.
	if contentData("EPUB/chapter1.xhtml")["nodes"].([]any)[0].(map[string]any)["text"] != "Alpha & one." {
		t.Fatal("candidate leaked into accepted content")
	}
	// Candidate drift is reviewable and rejectable but never acceptable.
	candidate := filepath.Join(ws, "tasks/active/work/pub/EPUB/chapter2.xhtml")
	tampered := bytes.Replace(files["EPUB/chapter2.xhtml"], []byte(">Beta &amp; two.</p>"), []byte(">Actual tamper</p>"), 1)
	if err := os.WriteFile(candidate, tampered, 0600); err != nil {
		t.Fatal(err)
	}
	drifted := run([]string{"task", "diff", task, "--workspace", ws}, 0)["data"].(map[string]any)
	if drifted["matchesExecution"] != false || drifted["operations"].([]any)[2].(map[string]any)["newValue"] != "Actual tamper" {
		t.Fatal(drifted)
	}
	run([]string{"task", "accept", task, "--workspace", ws}, 4)
	run([]string{"task", "reject", task, "--workspace", ws}, 0)
	run([]string{"apply", "--workspace", ws, "--plan", planPath}, 4)
	// Re-plan the same transaction, accept it formally and export.
	plan2Path := filepath.Join(dir, "plan2.json")
	run([]string{"plan", "--workspace", ws, "--operations", opsPath, "--output", plan2Path}, 0)
	e = run([]string{"apply", "--workspace", ws, "--plan", plan2Path}, 0)["data"].(map[string]any)
	task = e["taskId"].(string)
	accepted := run([]string{"task", "accept", task, "--workspace", ws}, 0)["data"].(map[string]any)
	if accepted["validation"].(map[string]any)["status"] != "pass" {
		t.Fatal(accepted)
	}
	after := contentData("EPUB/chapter1.xhtml")
	if after["revisionId"] != accepted["revisionId"] || after["nodes"].([]any)[0].(map[string]any)["text"] != "First < & > 😀" {
		t.Fatal(after)
	}
	files["EPUB/chapter1.xhtml"] = bytes.Replace(files["EPUB/chapter1.xhtml"], []byte(">Alpha &amp; one.</p>"), []byte(">First &lt; &amp; &gt; 😀</p>"), 1)
	files["EPUB/chapter2.xhtml"] = bytes.Replace(files["EPUB/chapter2.xhtml"], []byte(">Beta &amp; two.</p>"), []byte(">Second changed</p>"), 1)
	files["EPUB/package.opf"] = bytes.Replace(files["EPUB/package.opf"], []byte(">Title</dc:title>"), []byte(">Multi &lt; &amp; &gt;</dc:title>"), 1)
	out := filepath.Join(dir, "final.epub")
	exported := run([]string{"workspace", "export", ws, "--output", out}, 0)["data"].(map[string]any)
	if exported["verified"] != true || exported["revisionId"] != accepted["revisionId"] {
		t.Fatal(exported)
	}
	checkExport(t, out, files)
	got, err := os.ReadFile(book)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatal("original bytes changed", err)
	}
	// The exported ZIP is the accepted multi-resource tree, not only its first
	// changed resource.
	a, err := archive.Open(out, archive.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	for _, name := range []string{"EPUB/chapter1.xhtml", "EPUB/chapter2.xhtml", "EPUB/package.opf"} {
		b, err := a.Read(bookpath.BookPath(name), 8<<20)
		if err != nil || !bytes.Equal(b, files[name]) {
			t.Fatalf("export %s: %v", name, err)
		}
	}
}
