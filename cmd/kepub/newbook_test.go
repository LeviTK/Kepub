package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestNativeBookBinaryLifecycle(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("real pinned EPUBCheck required")
	}
	bin := workspaceBinary(t)
	dir := t.TempDir()
	ws, definition := filepath.Join(dir, "工作区 空格"), filepath.Join(dir, "definition.json")
	input := `{"schemaVersion":1,"title":"新书 & 😀","creator":"作者","language":"zh-Hans","identifier":"urn:uuid:93b80bc0-a0d1-4a7b-a121-32b4478db8ec","modified":"2026-10-13T00:00:00Z","chapters":[{"title":"第三章","paragraphs":["原文 < & > 😀"]},{"title":"第一章","paragraphs":["正文甲\t\r\n\"'"]},{"title":"第一章","paragraphs":["正文乙"]}]}`
	if err := os.WriteFile(definition, []byte(input), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(args []string, want int) map[string]any { return processJSON(t, bin, args, want, 60*time.Second) }
	create := []string{"workspace", "create", "--definition", definition, "--output", ws}
	created := run(create, 0)["data"].(map[string]any)
	state := created["state"].(map[string]any)
	if created["conformance"] != "pass" || state["version"] != float64(2) || state["originalSHA256"] != "" || state["source"].(map[string]any)["kind"] != "native-epub3" {
		t.Fatal(created)
	}
	if run(create, 2)["error"].(map[string]any)["code"] != "OUTPUT_EXISTS" {
		t.Fatal("wrong refusal")
	}
	if _, err := os.Stat(filepath.Join(ws, "original")); !os.IsNotExist(err) {
		t.Fatal("fake original")
	}

	// Standard-library ZIP/XML oracle, independent of Kepub's tree/renderer.
	initialZIP := filepath.Join(dir, "initial.epub")
	run([]string{"workspace", "export", ws, "--output", initialZIP}, 0)
	z, err := zip.OpenReader(initialZIP)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	files := map[string][]byte{}
	names := []string{}
	for _, f := range z.File {
		names = append(names, f.Name)
		if f.FileInfo().IsDir() {
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[f.Name] = b
	}
	sort.Strings(names)
	wantNames := []string{"EPUB/", "EPUB/chapter-001.xhtml", "EPUB/chapter-002.xhtml", "EPUB/chapter-003.xhtml", "EPUB/nav.xhtml", "EPUB/package.opf", "EPUB/style.css", "META-INF/", "META-INF/container.xml", "mimetype"}
	if !reflect.DeepEqual(names, wantNames) || z.File[0].Name != "mimetype" || z.File[0].Method != zip.Store || string(files["mimetype"]) != "application/epub+zip" {
		t.Fatalf("ZIP layout %v", names)
	}
	var opf struct {
		Version  string `xml:"version,attr"`
		Metadata struct {
			Title      string `xml:"title"`
			Creator    string `xml:"creator"`
			Language   string `xml:"language"`
			Identifier string `xml:"identifier"`
		} `xml:"metadata"`
		Spine struct {
			Items []struct {
				ID string `xml:"idref,attr"`
			} `xml:"itemref"`
		} `xml:"spine"`
		Manifest struct {
			Items []struct {
				ID   string `xml:"id,attr"`
				Href string `xml:"href,attr"`
			} `xml:"item"`
		} `xml:"manifest"`
	}
	if err := xml.Unmarshal(files["EPUB/package.opf"], &opf); err != nil {
		t.Fatal(err)
	}
	if opf.Version != "3.0" || opf.Metadata.Title != "新书 & 😀" || opf.Metadata.Creator != "作者" || opf.Metadata.Language != "zh-Hans" || opf.Metadata.Identifier != "urn:uuid:93b80bc0-a0d1-4a7b-a121-32b4478db8ec" {
		t.Fatal(opf)
	}
	ids := []string{}
	for _, item := range opf.Spine.Items {
		ids = append(ids, item.ID)
	}
	if !reflect.DeepEqual(ids, []string{"chapter-001", "chapter-002", "chapter-003"}) {
		t.Fatal(ids)
	}
	manifest := map[string]string{}
	for _, item := range opf.Manifest.Items {
		manifest[item.ID] = item.Href
	}
	if !reflect.DeepEqual(manifest, map[string]string{"nav": "nav.xhtml", "css": "style.css", "chapter-001": "chapter-001.xhtml", "chapter-002": "chapter-002.xhtml", "chapter-003": "chapter-003.xhtml"}) {
		t.Fatal(manifest)
	}
	var nav struct {
		Body struct {
			Nav struct {
				List struct {
					Items []struct {
						Link struct {
							Href string `xml:"href,attr"`
							Text string `xml:",chardata"`
						} `xml:"a"`
					} `xml:"li"`
				} `xml:"ol"`
			} `xml:"nav"`
		} `xml:"body"`
	}
	if err := xml.Unmarshal(files["EPUB/nav.xhtml"], &nav); err != nil {
		t.Fatal(err)
	}
	gotLinks := []string{}
	for _, item := range nav.Body.Nav.List.Items {
		gotLinks = append(gotLinks, item.Link.Href+":"+item.Link.Text)
	}
	if !reflect.DeepEqual(gotLinks, []string{"chapter-001.xhtml#start:第三章", "chapter-002.xhtml#start:第一章", "chapter-003.xhtml#start:第一章"}) {
		t.Fatal(gotLinks)
	}
	first := `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml" lang="zh-Hans" xml:lang="zh-Hans"><head><title>第三章</title><link rel="stylesheet" type="text/css" href="style.css"/></head><body><h1 id="start">第三章</h1><p>原文 &lt; &amp; &gt; 😀</p>` + "\n</body></html>\n"
	if string(files["EPUB/chapter-001.xhtml"]) != first {
		t.Fatal("incorrect generated text bytes", string(files["EPUB/chapter-001.xhtml"]))
	}
	// Template 1 is persisted provenance, so even formatting changes require a
	// new template version. Freeze complete bytes, not only equivalent XML.
	wantTemplate := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<?xml version="1.0" encoding="UTF-8"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>` + "\n",
		"EPUB/style.css":         "body { font-family: serif; writing-mode: horizontal-tb; }\nh1 { text-align: start; }\np { text-indent: 2em; }\n",
		"EPUB/package.opf":       `<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid" xml:lang="zh-Hans"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:uuid:93b80bc0-a0d1-4a7b-a121-32b4478db8ec</dc:identifier><dc:title id="title">新书 &amp; 😀</dc:title><dc:creator id="creator">作者</dc:creator><dc:language>zh-Hans</dc:language><meta property="dcterms:modified">2026-10-13T00:00:00Z</meta></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="css" href="style.css" media-type="text/css"/><item id="chapter-001" href="chapter-001.xhtml" media-type="application/xhtml+xml"/><item id="chapter-002" href="chapter-002.xhtml" media-type="application/xhtml+xml"/><item id="chapter-003" href="chapter-003.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="chapter-001"/><itemref idref="chapter-002"/><itemref idref="chapter-003"/></spine></package>` + "\n",
		"EPUB/nav.xhtml":         `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" lang="zh-Hans" xml:lang="zh-Hans"><head><title>Contents</title><link rel="stylesheet" type="text/css" href="style.css"/></head><body><nav epub:type="toc" id="toc"><h1>Contents</h1><ol><li><a href="chapter-001.xhtml#start">第三章</a></li><li><a href="chapter-002.xhtml#start">第一章</a></li><li><a href="chapter-003.xhtml#start">第一章</a></li></ol></nav></body></html>` + "\n",
		"EPUB/chapter-001.xhtml": first,
		"EPUB/chapter-002.xhtml": `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml" lang="zh-Hans" xml:lang="zh-Hans"><head><title>第一章</title><link rel="stylesheet" type="text/css" href="style.css"/></head><body><h1 id="start">第一章</h1><p>正文甲&#x9;&#xD;&#xA;&#34;&#39;</p>` + "\n</body></html>\n",
		"EPUB/chapter-003.xhtml": `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml" lang="zh-Hans" xml:lang="zh-Hans"><head><title>第一章</title><link rel="stylesheet" type="text/css" href="style.css"/></head><body><h1 id="start">第一章</h1><p>正文乙</p>` + "\n</body></html>\n",
	}
	if len(files) != len(wantTemplate) {
		t.Fatal("template 1 inventory changed")
	}
	for name, want := range wantTemplate {
		if string(files[name]) != want {
			t.Fatalf("template 1 bytes changed: %s", name)
		}
	}

	read := func() map[string]any {
		return run([]string{"content", "--workspace", ws, "--resource", "EPUB/chapter-001.xhtml", "--query", "原文"}, 0)["data"].(map[string]any)
	}
	initial := read()
	ops := filepath.Join(dir, "ops.json")
	newText := "新正文 < & > 😀"
	node := initial["nodes"].([]any)[0].(map[string]any)
	req := map[string]any{"schemaVersion": 2, "operations": []any{map[string]any{"operationId": "content.text.set", "operationVersion": 1, "params": map[string]any{"bookPath": initial["bookPath"], "revisionId": initial["revisionId"], "resourceSha256": initial["resourceSha256"], "locatorVersion": initial["locatorVersion"], "locator": node["locator"], "expectedOldValue": "原文 < & > 😀", "newValue": newText}}}}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ops, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for i, action := range []string{"reject", "accept"} {
		plan := filepath.Join(dir, action+".json")
		run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", plan}, 0)
		e := run([]string{"apply", "--workspace", ws, "--plan", plan}, 0)["data"].(map[string]any)
		task := e["taskId"].(string)
		if !reflect.DeepEqual(read(), initial) {
			t.Fatal("candidate leaked")
		}
		review := run([]string{"task", "diff", task, "--workspace", ws}, 0)["data"].(map[string]any)
		if review["matchesExecution"] != true || review["content"].(map[string]any)["newValue"] != newText {
			t.Fatal(review)
		}
		run([]string{"task", action, task, "--workspace", ws}, 0)
		if i == 0 && !reflect.DeepEqual(read(), initial) {
			t.Fatal("rejected text leaked")
		}
	}
	run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", filepath.Join(dir, "stale.json")}, 4)
	operationFile(t, ops, "title", "新书 & 😀", "出版标题")
	plan := filepath.Join(dir, "metadata.json")
	run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", plan}, 0)
	task := run([]string{"apply", "--workspace", ws, "--plan", plan}, 0)["data"].(map[string]any)["taskId"].(string)
	run([]string{"task", "diff", task, "--workspace", ws}, 0)
	run([]string{"task", "accept", task, "--workspace", ws}, 0)
	final := filepath.Join(dir, "final.epub")
	result := run([]string{"workspace", "export", ws, "--output", final}, 0)["data"].(map[string]any)
	if result["verified"] != true {
		t.Fatal(result)
	}
	files["EPUB/chapter-001.xhtml"] = []byte(strings.Replace(first, "原文 &lt; &amp; &gt; 😀", "新正文 &lt; &amp; &gt; 😀", 1))
	files["EPUB/package.opf"] = bytes.Replace(files["EPUB/package.opf"], []byte(">新书 &amp; 😀</dc:title>"), []byte(">出版标题</dc:title>"), 1)
	checkExport(t, final, files)
	reopened := filepath.Join(dir, "reopened")
	run([]string{"workspace", "open", final, "--output", reopened}, 0)
	content := run([]string{"content", "--workspace", reopened, "--resource", "EPUB/chapter-001.xhtml", "--query", "新正文"}, 0)["data"].(map[string]any)
	if content["nodes"].([]any)[0].(map[string]any)["text"] != newText {
		t.Fatal(content)
	}
	if raw, err := os.ReadFile(definition); err != nil || string(raw) != input {
		t.Fatal("definition changed", err)
	}
}

func TestNativeDefinitionByteBoundary(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("real pinned EPUBCheck required")
	}
	bin := workspaceBinary(t)
	input, err := os.ReadFile("../../docs/examples/newbook-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	definition, ws := filepath.Join(dir, "definition.json"), filepath.Join(dir, "ws")
	for _, tc := range []struct{ size, exit int }{{1 << 20, 0}, {(1 << 20) + 1, 2}} {
		padded := append(bytes.Clone(input), bytes.Repeat([]byte(" "), tc.size-len(input))...)
		if err := os.WriteFile(definition, padded, 0600); err != nil {
			t.Fatal(err)
		}
		result := processJSON(t, bin, []string{"workspace", "create", "--definition", definition, "--output", ws}, tc.exit, 60*time.Second)
		if tc.exit == 0 {
			if result["data"].(map[string]any)["conformance"] != "pass" {
				t.Fatal(result)
			}
			if err := os.Rename(ws, filepath.Join(dir, "completed")); err != nil {
				t.Fatal(err)
			}
		} else {
			if result["error"].(map[string]any)["code"] != "INVALID_ARGUMENT" {
				t.Fatal(result)
			}
			if _, err := os.Lstat(ws); !os.IsNotExist(err) {
				t.Fatal("oversized input published workspace", err)
			}
			stages, err := filepath.Glob(filepath.Join(dir, ".kepub-create-*"))
			if err != nil || len(stages) != 0 {
				t.Fatal("oversized input staged workspace", stages, err)
			}
		}
	}
}
