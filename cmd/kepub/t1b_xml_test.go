package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/testfixture"
	"github.com/LeviTK/Kepub/internal/workspace"
)

func t1bBook(t *testing.T, files map[string][]byte) string {
	t.Helper()
	names := []string{}
	for name := range files {
		if name != "mimetype" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	entries := []testfixture.Entry{{Name: "mimetype", Data: files["mimetype"]}}
	for _, name := range names {
		entries = append(entries, testfixture.Entry{Name: name, Data: files[name]})
	}
	book := filepath.Join(t.TempDir(), "subset.epub")
	testfixture.ZIP(t, book, entries)
	return book
}

func TestT1BCLICoverageScopeAndUTF16ExpansionOrigin(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("KEPUB_EPUBCHECK_JAR", "deliberately-missing")
	for _, be := range []bool{false, true} {
		_, files := legalCLI(t, "3.0")
		files["META-INF/container.xml"] = append([]byte(`<!DOCTYPE container [<!NOTATION image PUBLIC "image-id" "file:///never-open">]>`), files["META-INF/container.xml"]...)
		s := strings.TrimPrefix(string(files["EPUB/package.opf"]), `<?xml version="1.0"?>`)
		s = strings.Replace(s, ">Title</dc:title>", ">&outer;</dc:title>", 1)
		s = `<?xml version="1.0" encoding="UTF-16"?><!DOCTYPE package [<!ENTITY outer "&missing;">%unread;]>` + s
		files["EPUB/package.opf"] = testfixture.UTF16(s, be, true)
		// info must not parse unrelated content to manufacture global coverage.
		files["EPUB/chapter.xhtml"] = []byte("<malformed>")
		book := t1bBook(t, files)
		info := invoke(t, []string{"info", book, "--json"}, 0)["data"].(map[string]any)
		coverage := info["xmlCoverage"].(map[string]any)["resources"].([]any)
		if len(coverage) != 2 || coverage[0].(map[string]any)["bookPath"] != "EPUB/package.opf" || coverage[1].(map[string]any)["bookPath"] != "META-INF/container.xml" {
			t.Fatal("actual read scope/BookPath order", coverage)
		}
		for _, entry := range coverage {
			r := entry.(map[string]any)
			bp := r["bookPath"].(string)
			if r["resourceSha256"] != fmt.Sprintf("%x", sha256.Sum256(files[bp])) {
				t.Fatal("coverage not bound to original encoded bytes", r)
			}
		}
		r := coverage[0].(map[string]any)
		unresolved := r["unresolved"].([]any)
		if r["status"] != "partial" || len(unresolved) != 2 {
			t.Fatal("partial source records", r)
		}
		for i, tc := range []struct{ name, kind, origin, token string }{
			{"unread", "parameter", "reference", "%unread;"},
			{"missing", "general", "expansion", "&outer;"},
		} {
			u := unresolved[i].(map[string]any)
			pos := strings.Index(s, tc.token)
			start := len(testfixture.UTF16(s[:pos], be, true))
			end := len(testfixture.UTF16(s[:pos+len(tc.token)], be, true))
			if u["name"] != tc.name || u["kind"] != tc.kind || u["origin"] != tc.origin || u["startByte"] != float64(start) || u["endByte"] != float64(end) {
				t.Fatal("not a BOM-inclusive real raw reference", u, start, end)
			}
		}
		metadata := info["metadata"].([]any)
		if metadata[1].(map[string]any)["text"] != "&missing;" {
			t.Fatal("unknown metadata text was erased", metadata)
		}
		inspect := invoke(t, []string{"inspect", book, "--section", "metadata", "--json"}, 0)["data"].(map[string]any)
		if !reflect.DeepEqual(inspect["xmlCoverage"], info["xmlCoverage"]) || !reflect.DeepEqual(inspect["value"], info["metadata"]) {
			t.Fatal("inspect coverage must be inside data alongside unchanged value")
		}
	}
}

func TestT1BCLIPartialSearchAndZeroExternalHTTP(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, "forbidden external data")
	}))
	defer server.Close()
	t.Setenv("KEPUB_EPUBCHECK_JAR", "deliberately-missing")
	_, files := legalCLI(t, "3.0")
	files["EPUB/nav.xhtml"] = append([]byte(`<!DOCTYPE html [%unread;]>`), bytes.Replace(files["EPUB/nav.xhtml"], []byte(">Chapter</a>"), []byte(">&missing;</a>"), 1)...)
	book := t1bBook(t, files)
	ws := filepath.Join(t.TempDir(), "ws")
	invoke(t, []string{"workspace", "open", book, "--output", ws, "--json"}, 0)
	read := invoke(t, []string{"content", "--workspace", ws, "--resource", "EPUB/chapter.xhtml", "--query", "precise", "--json"}, 0)["data"].(map[string]any)
	if read["matchedCount"] != float64(1) {
		t.Fatal("complete resource positive control")
	}
	failed := invoke(t, []string{"search", "--workspace", ws, "--query", "precise", "--limit", "1", "--json"}, 3)
	// The existing envelope can carry a zero-valued result on error; it must
	// not carry the earlier complete resource's match as a successful count.
	failedData := failed["data"].(map[string]any)
	if failed["error"].(map[string]any)["code"] != "XML_ENTITY_UNRESOLVED" || failedData["matchedCount"] != float64(0) || failedData["results"] != nil || failedData["revisionId"] != "" {
		t.Fatal("late incomplete resource must not publish an incomplete search count", failed)
	}
	data := invoke(t, []string{"inspect", book, "--section", "navigation", "--json"}, 0)["data"].(map[string]any)
	value := data["value"].(map[string]any)
	if value["status"] != "partial" || !reflect.DeepEqual(value["xmlCoverage"], data["xmlCoverage"]) || value["entries"].([]any)[0].(map[string]any)["label"] != "&missing;" {
		t.Fatal("navigation partial/value/outer coverage disagrees", data)
	}
	for _, declaration := range []string{
		`<!DOCTYPE html SYSTEM "` + server.URL + `/dtd">`,
		`<!DOCTYPE html [<!ENTITY external SYSTEM "` + server.URL + `/entity">]>`,
		`<!DOCTYPE html [<!NOTATION format SYSTEM "` + server.URL + `/notation">]>`,
	} {
		files["EPUB/chapter.xhtml"] = []byte(declaration + `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>literal</p></body></html>`)
		r := invoke(t, []string{"inspect", t1bBook(t, files), "--section", "references", "--json"}, 0)["data"].(map[string]any)["value"].(map[string]any)
		if r["status"] != "partial" {
			t.Fatal("other existing uncertainty cannot be hidden", r)
		}
	}
	if requests.Load() != 0 {
		t.Fatal("XML declarations/notation identifiers triggered external HTTP", requests.Load())
	}
}

func TestT1BFormalDefaultProfileAndEntityTextEditLifecycle(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("actual pinned EPUBCheck required; skip is not formal acceptance")
	}
	_, files := legalCLI(t, "3.0")
	opf := strings.TrimPrefix(string(files["EPUB/package.opf"]), `<?xml version="1.0"?>`)
	opf = strings.Replace(opf, ` xmlns="http://www.idpf.org/2007/opf" version="3.0"`, "", 1)
	opf = strings.Replace(opf, ">Title</dc:title>", ">&title;</dc:title>", 1)
	files["EPUB/package.opf"] = []byte(`<!DOCTYPE package [<!ATTLIST package xmlns CDATA "http://www.idpf.org/2007/opf" version CDATA "3.0"><!ENTITY title "Title">]>` + opf)
	chapter := strings.Replace(string(files["EPUB/chapter.xhtml"]), "</body>", `<div>&markup;</div><div>&word;</div></body>`, 1)
	chapter = strings.Replace(chapter, ">Original &amp; precise.</p>", ">&empty;&word;&empty;</p>", 1)
	files["EPUB/chapter.xhtml"] = []byte(`<!DOCTYPE html [<!ENTITY empty ""><!ENTITY markup "&#60;em>Generated&#60;/em>"><!ENTITY % attrs '<!ATTLIST p id CDATA "paragraph"><!ENTITY word "Original &amp; precise.">'>%attrs;]>` + chapter)
	book := t1bBook(t, files)
	invoke(t, []string{"validate", book, "--json"}, 0)
	ws, ops, plan := filepath.Join(t.TempDir(), "ws"), filepath.Join(t.TempDir(), "ops.json"), filepath.Join(t.TempDir(), "plan.json")
	invoke(t, []string{"workspace", "open", book, "--output", ws, "--json"}, 0)
	read := invoke(t, []string{"content", "--workspace", ws, "--resource", "EPUB/chapter.xhtml", "--query", "precise", "--json"}, 0)["data"].(map[string]any)
	n := read["nodes"].([]any)[0].(map[string]any)
	request := workspace.Request{SchemaVersion: 2, Operations: []workspace.Operation{{ID: "content.text.set", Version: 1, Params: publication.TextSet{BookPath: bookpath.BookPath("EPUB/chapter.xhtml"), RevisionID: read["revisionId"].(string), ResourceSHA256: read["resourceSha256"].(string), LocatorVersion: 1, Locator: n["locator"].(string), ExpectedOldValue: n["text"].(string), NewValue: "Reviewed & literal."}}}}
	b, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ops, b, 0600); err != nil {
		t.Fatal(err)
	}
	invoke(t, []string{"plan", "--workspace", ws, "--operations", ops, "--output", plan, "--json"}, 0)
	task := invoke(t, []string{"apply", "--workspace", ws, "--plan", plan, "--json"}, 0)["data"].(map[string]any)["taskId"].(string)
	if invoke(t, []string{"task", "diff", task, "--workspace", ws, "--json"}, 0)["data"].(map[string]any)["matchesExecution"] != true {
		t.Fatal("history source recomputation did not use effective default profile")
	}
	invoke(t, []string{"task", "accept", task, "--workspace", ws, "--json"}, 0)
	output := filepath.Join(t.TempDir(), "formal.epub")
	invoke(t, []string{"workspace", "export", ws, "--output", output, "--json"}, 0)
	files["EPUB/chapter.xhtml"] = bytes.Replace(files["EPUB/chapter.xhtml"], []byte(">&empty;&word;&empty;</p>"), []byte(">Reviewed &amp; literal.</p>"), 1)
	assertExportFiles(t, output, files)
	// metadata.Apply must bootstrap the actual defaulted OPF version itself,
	// including persisted source reconstruction, not trust a ReadXML caller.
	operationFile(t, ops, "title", "Title", "Reviewed metadata")
	plan = filepath.Join(t.TempDir(), "metadata-plan.json")
	invoke(t, []string{"plan", "--workspace", ws, "--operations", ops, "--output", plan, "--json"}, 0)
	task = invoke(t, []string{"apply", "--workspace", ws, "--plan", plan, "--json"}, 0)["data"].(map[string]any)["taskId"].(string)
	if invoke(t, []string{"task", "diff", task, "--workspace", ws, "--json"}, 0)["data"].(map[string]any)["matchesExecution"] != true {
		t.Fatal("metadata default profile/history source drift")
	}
	invoke(t, []string{"task", "accept", task, "--workspace", ws, "--json"}, 0)
	output = filepath.Join(t.TempDir(), "metadata-formal.epub")
	invoke(t, []string{"workspace", "export", ws, "--output", output, "--json"}, 0)
	files["EPUB/package.opf"] = bytes.Replace(files["EPUB/package.opf"], []byte(">&title;</dc:title>"), []byte(">Reviewed metadata</dc:title>"), 1)
	assertExportFiles(t, output, files)
}

func TestT1BCLITrailingMiscAndRealChecker(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("actual pinned checker required for the independent XML control")
	}
	for _, extra := range []string{"\u00a0", `<![CDATA[ ]]>`, `&#10;`, `&empty;`} {
		_, files := legalCLI(t, "3.0")
		opf := strings.TrimPrefix(string(files["EPUB/package.opf"]), `<?xml version="1.0"?>`)
		files["EPUB/package.opf"] = []byte(`<!DOCTYPE package [<!ENTITY empty "">]>` + opf + extra)
		book := t1bBook(t, files)
		failed := invoke(t, []string{"info", book, "--json"}, 1)
		if failed["error"].(map[string]any)["code"] != "XML_NOT_WELL_FORMED" {
			t.Fatal("parser must reject lexical trailing Misc independently of checker", failed)
		}
		check := invoke(t, []string{"validate", book, "--json"}, 1)
		b, err := json.Marshal(check)
		if err != nil || !bytes.Contains(b, []byte("RSC-016")) {
			t.Fatal("real checker must retain its independent XML failure", extra, err, string(b))
		}
	}
}
