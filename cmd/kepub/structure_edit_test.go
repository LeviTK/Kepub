package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

// structureCLI is a conformance-positive EPUB3 whose first chapter carries an id
// the navigation references and an unreferenced id, so the real binary exercises
// the structural transaction and the reference gate.
func structureCLI(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	dir := t.TempDir()
	files := map[string][]byte{
		"mimetype":               []byte("application/epub+zip"),
		"META-INF/container.xml": []byte(`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`),
		"EPUB/package.opf":       []byte(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:uuid:12345678-1234-1234-1234-123456789012</dc:identifier><dc:title id="title">Title</dc:title><dc:creator id="creator">Writer</dc:creator><dc:language>en</dc:language><meta property="dcterms:modified">2026-10-04T00:00:00Z</meta></metadata><manifest><item id="c1" href="chapter1.xhtml" media-type="application/xhtml+xml"/><item id="css" href="style.css" media-type="text/css"/><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="c1"/></spine></package>` + "\r\n"),
		"EPUB/chapter1.xhtml": []byte(`<?xml version="1.0" encoding="utf-8"?>` + "\n" +
			`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>One</title><link rel="stylesheet" type="text/css" href="style.css"/></head><body><h1 id="start">One</h1><p>Alpha &amp; one.</p><p id="second">Second node.</p></body></html>` + "\n"),
		"EPUB/style.css": []byte("p { color: #123456; }\r\n"),
		"EPUB/nav.xhtml": []byte(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="chapter1.xhtml#start">One</a></li></ol></nav></body></html>`),
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

func TestStructureBinaryLifecycle(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("real pinned EPUBCheck required")
	}
	binary := workspaceBinary(t)
	book, files := structureCLI(t)
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
	available := map[string]bool{}
	for _, raw := range run([]string{"capabilities"}, 0)["data"].([]any) {
		c := raw.(map[string]any)
		available[c["operationId"].(string)] = c["implementationStatus"] == "available"
	}
	for _, id := range []string{"xhtml.attribute.set", "xhtml.attribute.remove", "xhtml.element.insert", "xhtml.element.replace", "xhtml.element.delete", "xhtml.element.move"} {
		if !available[id] {
			t.Fatalf("missing available capability %s", id)
		}
	}
	content := run([]string{"content", "--workspace", ws, "--resource", "EPUB/chapter1.xhtml"}, 0)["data"].(map[string]any)
	nodes := content["nodes"].([]any)
	node := func(local string, n int) map[string]any {
		seen := 0
		for _, raw := range nodes {
			entry := raw.(map[string]any)
			if entry["localName"] != local {
				continue
			}
			if seen == n {
				return entry
			}
			seen++
		}
		t.Fatalf("missing %s[%d]", local, n)
		return nil
	}
	bind := func(local string, n int) map[string]any {
		entry := node(local, n)
		return map[string]any{
			"bookPath": content["bookPath"], "revisionId": content["revisionId"], "resourceSha256": content["resourceSha256"],
			"locatorVersion": content["locatorVersion"], "locator": entry["locator"],
		}
	}
	withParams := func(id string, base map[string]any, extra map[string]any) map[string]any {
		params := map[string]any{}
		for k, v := range base {
			params[k] = v
		}
		for k, v := range extra {
			params[k] = v
		}
		return map[string]any{"operationId": id, "operationVersion": 1, "params": params}
	}
	writeOps := func(ops []any) {
		t.Helper()
		b, err := json.Marshal(map[string]any{"schemaVersion": 4, "operations": ops})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(opsPath, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	metadata := map[string]any{"operationId": "metadata.set", "operationVersion": 1, "params": map[string]any{
		"namespace": "http://purl.org/dc/elements/1.1/", "localName": "title", "id": "title", "expectedOldValue": "Title", "newValue": "Structure < & >"}}
	// A referenced identity may not be deleted; the refusal happens before any
	// plan output exists.
	writeOps([]any{withParams("xhtml.element.delete", bind("h1", 0), nil)})
	refused := run([]string{"plan", "--workspace", ws, "--operations", opsPath, "--output", planPath}, 1)
	if refused["error"].(map[string]any)["code"] != "REFERENCE_CONFLICT" {
		t.Fatal(refused)
	}
	if _, err := os.Stat(planPath); !os.IsNotExist(err) {
		t.Fatal("refused structural request published a plan", err)
	}
	// A four-operation structural transaction: metadata, an attribute write, an
	// insertion and a deletion of an unreferenced identity.
	writeOps([]any{
		metadata,
		withParams("xhtml.attribute.set", bind("p", 0), map[string]any{"name": "dir", "value": "rtl"}),
		withParams("xhtml.element.insert", bind("h1", 0), map[string]any{"position": "after", "fragment": `<p id="note">Inserted <em>em</em>.</p>`}),
		withParams("xhtml.element.delete", bind("p", 1), nil),
	})
	p := run([]string{"plan", "--workspace", ws, "--operations", opsPath, "--output", planPath}, 0)["data"].(map[string]any)
	if p["schemaVersion"] != float64(4) {
		t.Fatal(p)
	}
	if got := p["writeSet"].([]any); len(got) != 2 || got[0] != "EPUB/chapter1.xhtml" || got[1] != "EPUB/package.opf" {
		t.Fatal(p)
	}
	e := run([]string{"apply", "--workspace", ws, "--plan", planPath}, 0)["data"].(map[string]any)
	task := e["taskId"].(string)
	if e["reviewRequired"] != true || e["conformance"] != "not_run" {
		t.Fatal(e)
	}
	review := run([]string{"task", "diff", task, "--workspace", ws}, 0)["data"].(map[string]any)
	if review["matchesExecution"] != true {
		t.Fatal(review)
	}
	ops := review["operations"].([]any)
	if len(ops) != 4 {
		t.Fatal(ops)
	}
	attribute := ops[1].(map[string]any)["attribute"].(map[string]any)
	if attribute["name"] != "dir" || attribute["newValue"] != "rtl" {
		t.Fatalf("attribute observation: %v", attribute)
	}
	if candidate := ops[2].(map[string]any)["element"].(map[string]any)["candidate"]; candidate == nil || !bytes.Contains([]byte(candidate.(string)), []byte("present")) {
		t.Fatal(ops[2])
	}
	if candidate := ops[3].(map[string]any)["element"].(map[string]any)["candidate"]; candidate == nil || !bytes.Contains([]byte(candidate.(string)), []byte("removed ids absent: second")) {
		t.Fatal(ops[3])
	}
	// Candidate drift is reported from real bytes, never from the plan.
	candidate := filepath.Join(ws, "tasks/active/work/pub/EPUB/chapter1.xhtml")
	applied, err := os.ReadFile(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(candidate, bytes.Replace(applied, []byte(`dir="rtl"`), []byte(`dir="ttb"`), 1), 0600); err != nil {
		t.Fatal(err)
	}
	review = run([]string{"task", "diff", task, "--workspace", ws}, 0)["data"].(map[string]any)
	if review["matchesExecution"] != false {
		t.Fatal(review)
	}
	if attribute := review["operations"].([]any)[1].(map[string]any)["attribute"].(map[string]any); attribute["newValue"] != "ttb" {
		t.Fatal(attribute)
	}
	run([]string{"task", "accept", task, "--workspace", ws}, 4)
	run([]string{"task", "reject", task, "--workspace", ws}, 0)
	// Re-plan into a fresh output, accept formally and export the accepted tree.
	plan2Path := filepath.Join(dir, "plan2.json")
	run([]string{"plan", "--workspace", ws, "--operations", opsPath, "--output", plan2Path}, 0)
	e = run([]string{"apply", "--workspace", ws, "--plan", plan2Path}, 0)["data"].(map[string]any)
	task = e["taskId"].(string)
	accepted := run([]string{"task", "accept", task, "--workspace", ws}, 0)["data"].(map[string]any)
	if accepted["validation"].(map[string]any)["status"] != "pass" {
		t.Fatal(accepted)
	}
	after := run([]string{"content", "--workspace", ws, "--resource", "EPUB/chapter1.xhtml"}, 0)["data"].(map[string]any)
	if after["revisionId"] != accepted["revisionId"] {
		t.Fatal(after)
	}
	texts := []string{}
	for _, raw := range after["nodes"].([]any) {
		texts = append(texts, raw.(map[string]any)["text"].(string))
	}
	if len(texts) != 4 || texts[0] != "One" || texts[1] != "Inserted em." || texts[3] != "Alpha & one." {
		t.Fatalf("accepted content: %v", texts)
	}
	for _, text := range texts {
		if strings.Contains(text, "Second node.") {
			t.Fatalf("deleted content survived: %v", texts)
		}
	}
	// The exported archive carries the structural edit and every untouched byte.
	want := bytes.Replace(files["EPUB/chapter1.xhtml"], []byte(`<p>Alpha &amp; one.</p>`), []byte(`<p dir="rtl">Alpha &amp; one.</p>`), 1)
	want = bytes.Replace(want, []byte(`</h1>`), []byte(`</h1><p id="note">Inserted <em>em</em>.</p>`), 1)
	want = bytes.Replace(want, []byte(`<p id="second">Second node.</p>`), nil, 1)
	files["EPUB/chapter1.xhtml"] = want
	files["EPUB/package.opf"] = bytes.Replace(files["EPUB/package.opf"], []byte(`>Title</dc:title>`), []byte(`>Structure &lt; &amp; &gt;</dc:title>`), 1)
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
	a, err := archive.Open(out, archive.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if b, err := a.Read(bookpath.BookPath("EPUB/style.css"), 1<<20); err != nil || !bytes.Equal(b, files["EPUB/style.css"]) {
		t.Fatalf("untouched stylesheet changed: %v", err)
	}
}

func TestN2IDREFBinaryLifecycle(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("real pinned EPUBCheck required")
	}
	binary := workspaceBinary(t)
	for _, literal := range []bool{true, false} {
		t.Run(map[bool]string{true: "literal-conflict", false: "formal-positive"}[literal], func(t *testing.T) {
			book, files := structureCLI(t)
			id := "normal"
			if literal {
				id = "p%41"
			}
			files["EPUB/chapter1.xhtml"] = bytes.Replace(files["EPUB/chapter1.xhtml"], []byte(`<p id="second">Second node.</p>`), []byte(`<p id="`+id+`">Target.</p><p id="pA">Other.</p><div role="group" aria-labelledby="`+id+`">Reference.</div>`), 1)
			entries := []testfixture.Entry{}
			for _, name := range []string{"mimetype", "META-INF/container.xml", "EPUB/package.opf", "EPUB/chapter1.xhtml", "EPUB/style.css", "EPUB/nav.xhtml"} {
				entries = append(entries, testfixture.Entry{Name: name, Data: files[name]})
			}
			testfixture.ZIP(t, book, entries)
			original, err := os.ReadFile(book)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			ws := filepath.Join(dir, "ws")
			run := func(args []string, want int) map[string]any {
				return processJSON(t, binary, args, want, 60*time.Second)
			}
			run([]string{"workspace", "open", book, "--output", ws}, 0)
			graph := run([]string{"inspect", "--section=references", "--resource=EPUB/chapter1.xhtml", "--direction=incoming", book}, 0)["data"].(map[string]any)["value"].(map[string]any)
			if graph["parserVersion"] != float64(2) {
				t.Fatal("unversioned reference graph", graph)
			}
			found := 0
			for _, raw := range graph["edges"].([]any) {
				e := raw.(map[string]any)
				if e["syntax"] != "xhtml.idref" {
					continue
				}
				if e["href"] != id || e["target"].(map[string]any)["fragment"] != id || e["fragmentStatus"] != "resolved" || !strings.HasSuffix(e["location"].(string), "/@aria-labelledby") {
					t.Fatal("literal source/target relationship lost", e)
				}
				found++
			}
			if found != 1 {
				t.Fatalf("incoming literal IDREF count %d", found)
			}
			content := run([]string{"content", "--workspace", ws, "--resource", "EPUB/chapter1.xhtml"}, 0)["data"].(map[string]any)
			locator := ""
			for _, raw := range content["nodes"].([]any) {
				n := raw.(map[string]any)
				if n["id"] == id {
					locator = n["locator"].(string)
				}
			}
			if locator == "" {
				t.Fatal("missing frozen target locator")
			}
			params := map[string]any{"bookPath": content["bookPath"], "revisionId": content["revisionId"], "resourceSha256": content["resourceSha256"], "locatorVersion": content["locatorVersion"], "locator": locator}
			opsPath, planPath := filepath.Join(dir, "ops.json"), filepath.Join(dir, "plan.json")
			writeRequest := func(operation string, params map[string]any) {
				b, err := json.Marshal(map[string]any{"schemaVersion": 4, "operations": []any{map[string]any{"operationId": operation, "operationVersion": 1, "params": params}}})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(opsPath, b, 0600); err != nil {
					t.Fatal(err)
				}
			}
			writeRequest("xhtml.element.delete", params)
			refused := run([]string{"plan", "--workspace", ws, "--operations", opsPath, "--output", planPath}, 1)
			if refused["error"].(map[string]any)["code"] != "REFERENCE_CONFLICT" {
				t.Fatal(refused)
			}
			if _, err := os.Stat(planPath); !os.IsNotExist(err) {
				t.Fatal("refused request published plan", err)
			}
			if _, err := os.Stat(filepath.Join(ws, "tasks/active")); !os.IsNotExist(err) {
				t.Fatal("refused request created task", err)
			}
			if !literal {
				params["name"], params["value"] = "class", "n2"
				writeRequest("xhtml.attribute.set", params)
				run([]string{"plan", "--workspace", ws, "--operations", opsPath, "--output", planPath}, 0)
				e := run([]string{"apply", "--workspace", ws, "--plan", planPath}, 0)["data"].(map[string]any)
				task := e["taskId"].(string)
				run([]string{"task", "accept", task, "--workspace", ws}, 0)
				files["EPUB/chapter1.xhtml"] = bytes.Replace(files["EPUB/chapter1.xhtml"], []byte(`id="normal"`), []byte(`id="normal" class="n2"`), 1)
				out := filepath.Join(dir, "formal.epub")
				exported := run([]string{"workspace", "export", ws, "--output", out}, 0)["data"].(map[string]any)
				if exported["verified"] != true || exported["validation"].(map[string]any)["status"] != "pass" {
					t.Fatal("formal export skipped checker", exported)
				}
				checkExport(t, out, files)
				z, err := zip.OpenReader(out)
				if err != nil {
					t.Fatal(err)
				}
				defer z.Close()
				seen := map[string]bool{}
				for _, entry := range z.File {
					_, file := files[entry.Name]
					directory := entry.Name == "EPUB/" || entry.Name == "META-INF/"
					if seen[entry.Name] || !file && !directory || entry.FileInfo().IsDir() != directory || directory && entry.UncompressedSize64 != 0 {
						t.Fatal("unexpected ZIP inventory", entry.Name)
					}
					seen[entry.Name] = true
				}
				if len(seen) != 8 {
					t.Fatalf("ZIP inventory got %d, want 6 files + 2 directories", len(seen))
				}
			}
			got, err := os.ReadFile(book)
			if err != nil || !bytes.Equal(got, original) {
				t.Fatal("original changed", err)
			}
		})
	}
}
