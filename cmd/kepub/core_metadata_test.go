package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Run the same public lifecycle with an instrumented child, not merely a race
// instrumented test harness whose workspaceBinary child is an ordinary build.
func TestCoreMetadataBinaryLifecycle(t *testing.T) {
	for _, race := range []bool{false, true} {
		name := "ordinary"
		if race {
			name = "race-cli"
		}
		t.Run(name, func(t *testing.T) {
			var binary string
			if !race {
				binary = workspaceBinary(t)
			} else {
				binary = filepath.Join(t.TempDir(), "kepub")
				cmd := exec.Command("go", "build", "-race", "-o", binary, ".")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("race build: %v %s", err, out)
				}
			}
			for _, native := range []bool{false, true} {
				kind := "imported"
				if native {
					kind = "native"
				}
				t.Run(kind, func(t *testing.T) { coreMetadataLifecycle(t, binary, native) })
			}
		})
	}
}

func coreMetadataLifecycle(t *testing.T, binary string, native bool) {
	t.Helper()
	run := func(args []string, exit int) map[string]any {
		return processJSON(t, binary, args, exit, 90*time.Second)
	}
	dir := t.TempDir()
	ws := filepath.Join(dir, "workspace")
	book, files := legalCLI(t, "3.0")
	original, err := os.ReadFile(book)
	if err != nil {
		t.Fatal(err)
	}
	if native {
		if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
			t.Skip("native creation requires real pinned EPUBCheck 5.3.0")
		}
		definition := filepath.Join(dir, "definition.json")
		coreMetadataJSON(t, definition, map[string]any{"schemaVersion": 1, "title": "Title", "creator": "Writer", "language": "en", "identifier": "urn:uuid:12345678-1234-1234-1234-123456789012", "modified": "2026-10-04T00:00:00Z", "chapters": []any{map[string]any{"title": "Chapter", "paragraphs": []string{"Original & precise."}}}})
		created := run([]string{"workspace", "create", "--definition", definition, "--output", ws}, 0)["data"].(map[string]any)
		if created["conformance"] != "pass" {
			t.Fatal(created)
		}
		files = coreMetadataNativeBytes()
	} else {
		run([]string{"workspace", "open", book, "--output", ws}, 0)
	}
	read := func() map[string]any {
		return run([]string{"workspace", "metadata", "--workspace", ws}, 0)["data"].(map[string]any)
	}
	baseline := read()
	if baseline["resourceSha256"] != fmt.Sprintf("%x", sha256.Sum256(files["EPUB/package.opf"])) {
		t.Fatal("binding differs from independent OPF", baseline)
	}
	coreMetadataCapabilities(t, run([]string{"capabilities"}, 0)["data"].([]any))
	makeOp := func(field, old, value string) map[string]any {
		params := map[string]any{"bookPath": "EPUB/package.opf", "revisionId": baseline["revisionId"], "resourceSha256": baseline["resourceSha256"], "namespace": "http://purl.org/dc/elements/1.1/", "localName": field, "expectedOldValue": old, "newValue": value}
		if field == "meta" {
			params["namespace"] = "http://www.idpf.org/2007/opf"
			params["property"] = "dcterms:modified"
		}
		return map[string]any{"operationId": "metadata.set", "operationVersion": 2, "params": params}
	}
	ops, plan := filepath.Join(dir, "ops.json"), filepath.Join(dir, "plan.json")
	// A valid first operation primes the shared OPF cache; the second must still
	// validate its own bindings, even when its replacement is a no-op.
	for _, bad := range []string{"resourceSha256", "revisionId", "field", "language", "modified", "unknown", "schema-mix"} {
		t.Run(bad, func(t *testing.T) {
			run := func(args []string, exit int) map[string]any {
				return processJSON(t, binary, args, exit, 90*time.Second)
			}
			second := makeOp("language", "en", "en")
			p := second["params"].(map[string]any)
			exit, code, schema := 2, "INVALID_OPERATIONS", 8
			switch bad {
			case "resourceSha256":
				p[bad] = strings.Repeat("0", 64)
				exit, code = 4, "INPUT_DRIFT"
			case "revisionId":
				p[bad] = strings.Repeat("0", 32)
				exit, code = 4, "INPUT_DRIFT"
			case "field":
				p["localName"] = "publisher"
			case "language":
				p["newValue"] = "en_US"
			case "modified":
				second = makeOp("meta", "2026-10-04T00:00:00Z", "2024-02-30T00:00:00Z")
			case "unknown":
				p["extra"] = true
			case "schema-mix":
				schema = 3
			}
			coreMetadataJSON(t, ops, map[string]any{"schemaVersion": schema, "operations": []any{makeOp("title", "Title", "Changed"), second}})
			env := run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", plan}, exit)
			if env["error"].(map[string]any)["code"] != code {
				t.Fatal(env)
			}
			if _, err := os.Stat(plan); !os.IsNotExist(err) {
				t.Fatal("failed request published plan", err)
			}
			if !reflect.DeepEqual(read(), baseline) {
				t.Fatal("failure advanced accepted metadata")
			}
		})
	}
	// Legacy v1 keeps its schema and text semantics after v2 registration.
	operationFile(t, ops, "title", "Title", "Legacy")
	legacy := run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", plan}, 0)["data"].(map[string]any)
	if legacy["schemaVersion"] != float64(1) {
		t.Fatal(legacy)
	}
	task := run([]string{"apply", "--workspace", ws, "--plan", plan}, 0)["data"].(map[string]any)["taskId"].(string)
	if run([]string{"task", "diff", task, "--workspace", ws}, 0)["data"].(map[string]any)["metadata"].(map[string]any)["newValue"] != "Legacy" {
		t.Fatal("legacy candidate missing")
	}
	run([]string{"task", "reject", task, "--workspace", ws}, 0)
	plan = filepath.Join(dir, "v2-plan.json")
	operations := []any{makeOp("title", "Title", "New < & >"), makeOp("creator", "Writer", "Next Writer"), makeOp("language", "en", "fr-CA"), makeOp("identifier", "urn:uuid:12345678-1234-1234-1234-123456789012", "urn:uuid:22345678-1234-1234-1234-123456789012"), makeOp("meta", "2026-10-04T00:00:00Z", "2024-02-29T12:34:56Z")}
	coreMetadataJSON(t, ops, map[string]any{"schemaVersion": 8, "operations": operations})
	planned := run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", plan}, 0)["data"].(map[string]any)
	if planned["schemaVersion"] != float64(8) {
		t.Fatal(planned)
	}
	execution := run([]string{"apply", "--workspace", ws, "--plan", plan}, 0)["data"].(map[string]any)
	task = execution["taskId"].(string)
	if execution["reviewRequired"] != true || execution["conformance"] != "not_run" {
		t.Fatal(execution)
	}
	review := run([]string{"task", "diff", task, "--workspace", ws}, 0)["data"].(map[string]any)
	actual := review["operations"].([]any)
	if review["matchesExecution"] != true || len(actual) != len(operations) {
		t.Fatal(review)
	}
	for i, op := range operations {
		p := op.(map[string]any)["params"].(map[string]any)
		got := actual[i].(map[string]any)
		if got["oldValue"] != p["expectedOldValue"] || got["newValue"] != p["newValue"] || got["plannedValue"] != p["newValue"] {
			t.Fatal("actual candidate diff", got)
		}
	}
	if !reflect.DeepEqual(read(), baseline) {
		t.Fatal("candidate leaked into accepted read")
	}
	// The expected entire OPF is computed solely from fixture literals, never
	// candidate bytes, product spans, WriteSet, or the exported result.
	want := bytes.Clone(files["EPUB/package.opf"])
	for _, replacement := range [][2]string{{">Title</dc:title>", ">New &lt; &amp; &gt;</dc:title>"}, {">Writer</dc:creator>", ">Next Writer</dc:creator>"}, {">en</dc:language>", ">fr-CA</dc:language>"}, {">urn:uuid:12345678-1234-1234-1234-123456789012</dc:identifier>", ">urn:uuid:22345678-1234-1234-1234-123456789012</dc:identifier>"}, {">2026-10-04T00:00:00Z</meta>", ">2024-02-29T12:34:56Z</meta>"}} {
		if bytes.Count(want, []byte(replacement[0])) != 1 {
			t.Fatal("oracle anchor not unique", replacement)
		}
		want = bytes.Replace(want, []byte(replacement[0]), []byte(replacement[1]), 1)
	}
	files["EPUB/package.opf"] = want
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("Plan/Apply/diff passed; formal acceptance requires pinned EPUBCheck 5.3.0")
	}
	accepted := run([]string{"task", "accept", task, "--workspace", ws}, 0)["data"].(map[string]any)
	if accepted["status"] != "accepted" || accepted["validation"].(map[string]any)["status"] != "pass" {
		t.Fatal(accepted)
	}
	current := read() // Fresh child process must reopen accepted history.
	if current["revisionId"] != accepted["revisionId"] || current["revisionId"] == baseline["revisionId"] || current["resourceSha256"] != fmt.Sprintf("%x", sha256.Sum256(want)) {
		t.Fatal(current)
	}
	output := filepath.Join(dir, "final.epub")
	exported := run([]string{"workspace", "export", ws, "--output", output}, 0)["data"].(map[string]any)
	if exported["verified"] != true || exported["revisionId"] != accepted["revisionId"] {
		t.Fatal(exported)
	}
	checkExport(t, output, files)
	coreMetadataZIP(t, output, files)
	reopened := filepath.Join(dir, "reopened")
	run([]string{"workspace", "open", output, "--output", reopened}, 0)
	fresh := run([]string{"workspace", "metadata", "--workspace", reopened}, 0)["data"].(map[string]any)
	if !reflect.DeepEqual(fresh["value"], current["value"]) || fresh["resourceSha256"] != current["resourceSha256"] || fresh["revisionId"] != "initial" {
		t.Fatal(fresh, current)
	}
	if !native {
		after, err := os.ReadFile(book)
		if err != nil || !bytes.Equal(after, original) {
			t.Fatal("original EPUB changed", err)
		}
	}
}

func coreMetadataJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func coreMetadataCapabilities(t *testing.T, capabilities []any) {
	t.Helper()
	found := map[int]bool{}
	for _, entry := range capabilities {
		c := entry.(map[string]any)
		if c["operationId"] != "metadata.set" {
			continue
		}
		version := int(c["operationVersion"].(float64))
		found[version] = true
		schema := c["inputSchema"].(map[string]any)
		properties := schema["properties"].(map[string]any)
		if schema["additionalProperties"] != false || c["implementationStatus"] != "available" {
			t.Fatal(c)
		}
		if version == 1 {
			if len(properties) != 5 || !reflect.DeepEqual(schema["required"], []any{"namespace", "localName", "expectedOldValue", "newValue"}) {
				t.Fatal("legacy shape changed", c)
			}
		} else if version == 2 {
			if len(properties) != 9 || !reflect.DeepEqual(schema["required"], []any{"bookPath", "revisionId", "resourceSha256", "namespace", "localName", "expectedOldValue", "newValue"}) {
				t.Fatal("v2 shape", c)
			}
		} else {
			t.Fatal("unexpected metadata version", c)
		}
	}
	if !found[1] || !found[2] {
		t.Fatal("missing versioned capabilities", found)
	}
}

func coreMetadataZIP(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	z, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	want := map[string][]byte{"EPUB/": {}, "META-INF/": {}}
	for name, raw := range files {
		want[name] = raw
	}
	seen := map[string]bool{}
	for _, f := range z.File {
		if seen[f.Name] {
			t.Fatal("duplicate ZIP entry", f.Name)
		}
		seen[f.Name] = true
		expected, ok := want[f.Name]
		if !ok {
			t.Fatal("unexpected ZIP entry", f.Name)
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(r)
		r.Close()
		if err != nil || !bytes.Equal(raw, expected) || f.FileInfo().IsDir() != strings.HasSuffix(f.Name, "/") {
			t.Fatal("ZIP oracle mismatch", f.Name, err)
		}
	}
	if len(seen) != len(want) {
		t.Fatal("ZIP inventory mismatch", seen)
	}
}

func coreMetadataNativeBytes() map[string][]byte {
	return map[string][]byte{
		"mimetype":               []byte("application/epub+zip"),
		"META-INF/container.xml": []byte(`<?xml version="1.0" encoding="UTF-8"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>` + "\n"),
		"EPUB/package.opf":       []byte(`<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid" xml:lang="en"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:uuid:12345678-1234-1234-1234-123456789012</dc:identifier><dc:title id="title">Title</dc:title><dc:creator id="creator">Writer</dc:creator><dc:language>en</dc:language><meta property="dcterms:modified">2026-10-04T00:00:00Z</meta></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="css" href="style.css" media-type="text/css"/><item id="chapter-001" href="chapter-001.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="chapter-001"/></spine></package>` + "\n"),
		"EPUB/style.css":         []byte("body { font-family: serif; writing-mode: horizontal-tb; }\nh1 { text-align: start; }\np { text-indent: 2em; }\n"),
		"EPUB/nav.xhtml":         []byte(`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="en" xml:lang="en"><head><title>Contents</title><link rel="stylesheet" type="text/css" href="style.css"/></head><body><nav epub:type="toc" id="toc"><h1>Contents</h1><ol><li><a href="chapter-001.xhtml#start">Chapter</a></li></ol></nav></body></html>` + "\n"),
		"EPUB/chapter-001.xhtml": []byte(`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml" lang="en" xml:lang="en"><head><title>Chapter</title><link rel="stylesheet" type="text/css" href="style.css"/></head><body><h1 id="start">Chapter</h1><p>Original &amp; precise.</p>` + "\n</body></html>\n"),
	}
}
