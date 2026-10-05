package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/testfixture"
	"github.com/LeviTK/Kepub/internal/workspace"
)

const cliContentPath = "书/Text/第二 章.xhtml"

func cliContentWorkspace(t *testing.T, text string) (dir, book, id, revision string) {
	t.Helper()
	entries := testfixture.EPUB("3.0", false)
	entries[3].Data = []byte(text)
	book = filepath.Join(t.TempDir(), "含 空格.epub")
	testfixture.ZIP(t, book, entries)
	dir = filepath.Join(t.TempDir(), "工作区 空格")
	w, err := workspace.Create(dir, book, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	id, err = w.ID()
	if err != nil {
		t.Fatal(err)
	}
	a, _, r, err := w.AcceptedSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	a.Close()
	return dir, book, id, r.ID
}

func contentTreeBytes(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[strings.TrimPrefix(path, dir+string(os.PathSeparator))] = data
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func TestContentCLIAcceptedIsolationAndLiteralQueries(t *testing.T) {
	text := "\xef\xbb\xbf" + `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>hidden</title></head><body><p id="a">Same &amp; 😀` + "\r\n" + `</p><p>Same &amp; &#x1f600;` + "\r\n" + `</p><p>Alpha <em>emphasis</em> tail</p><div>outer<script>secret</script><p>safe</p></div></body></html>`
	dir, book, id, revision := cliContentWorkspace(t, text)
	w, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := w.NewCandidate()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(candidate, cliContentPath), []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p>candidate only</p></body></html>`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(candidate, "书/Deep/package.opf"), []byte("candidate-invalid-opf"), 0600); err != nil {
		t.Fatal(err)
	}
	w.Close()
	before := contentTreeBytes(t, dir)
	original, err := os.ReadFile(book)
	if err != nil {
		t.Fatal(err)
	}
	// No Java/checker/Amp is needed for a read, even with an active candidate.
	t.Setenv("PATH", t.TempDir())
	t.Setenv("KEPUB_EPUBCHECK_JAR", "missing-checker")
	args := []string{"content", "--workspace", dir, "--resource", cliContentPath, "--json"}
	r := invoke(t, args, 0)["data"].(map[string]any)
	if r["workspaceId"] != id || r["revisionId"] != revision || r["rootfile"] != "书/Deep/package.opf" || r["bookPath"] != cliContentPath || r["resourceSha256"] != fmt.Sprintf("%x", sha256.Sum256([]byte(text))) || r["locatorVersion"] != float64(1) || r["matchedCount"] != float64(5) || r["returnedCount"] != float64(5) || r["truncated"] != false {
		t.Fatal(r)
	}
	want := []any{
		map[string]any{"namespace": publication.XHTMLNamespace, "localName": "p", "id": "a", "locator": "/html[1]/body[1]/p[1]", "text": "Same & 😀\n", "hasChildElements": false},
		map[string]any{"namespace": publication.XHTMLNamespace, "localName": "p", "locator": "/html[1]/body[1]/p[2]", "text": "Same & 😀\n", "hasChildElements": false},
		map[string]any{"namespace": publication.XHTMLNamespace, "localName": "p", "locator": "/html[1]/body[1]/p[3]", "text": "Alpha emphasis tail", "hasChildElements": true},
		map[string]any{"namespace": publication.XHTMLNamespace, "localName": "em", "locator": "/html[1]/body[1]/p[3]/em[1]", "text": "emphasis", "hasChildElements": false},
		map[string]any{"namespace": publication.XHTMLNamespace, "localName": "p", "locator": "/html[1]/body[1]/div[1]/p[1]", "text": "safe", "hasChildElements": false},
	}
	if !reflect.DeepEqual(r["nodes"], want) {
		t.Fatal("wrong full nodes", r)
	}
	for _, tc := range []struct {
		query string
		count int
	}{
		{"Same & 😀\n", 2}, {"Alpha emphasis", 1}, {"emphasis", 2}, {"emphasis tail", 1}, {"same", 0}, {"😀\nSame", 0}, {"candidate only", 0}, {"secret", 0}, {"hidden", 0}, {strings.Repeat("😀", 1024), 0},
	} {
		data := invoke(t, append(args, "--query", tc.query), 0)["data"].(map[string]any)
		if data["matchedCount"] != float64(tc.count) || data["returnedCount"] != float64(tc.count) || data["truncated"] != false || data["nodes"] == nil {
			t.Fatal(tc, data)
		}
	}
	r = invoke(t, append(args, "--query", "Same", "--limit", "1"), 0)["data"].(map[string]any)
	if r["matchedCount"] != float64(2) || r["returnedCount"] != float64(1) || r["truncated"] != true || !reflect.DeepEqual(r["nodes"], want[:1]) {
		t.Fatal("bounded ambiguity", r)
	}
	if !reflect.DeepEqual(contentTreeBytes(t, dir), before) {
		t.Fatal("content changed workspace files, revisions, candidate or pointers")
	}
	after, err := os.ReadFile(book)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatal("content changed original ZIP", err)
	}
}

func TestContentCLIArgumentErrorsAndOptionStorage(t *testing.T) {
	base := []string{"content", "--workspace", "nonexistent", "--resource", cliContentPath}
	invalid := string([]byte{0xff})
	cases := [][]string{
		{"content"}, {"content", "--workspace", "nonexistent"}, {"content", "--resource", cliContentPath},
		append(base, "--unknown"), append(base, "--resource", cliContentPath), append(base, "--workspace", "other"),
		append(base, "--query", "a", "--query", "b"), append(base, "--limit", "1", "--limit", "2"),
		append(base, "--query="), append(base, "--query", ""),
		append(base, "--query", invalid), append(base, "--query", strings.Repeat("😀", 1024)+"a"),
		append(base, "--limit", "0"), append(base, "--limit", "201"), append(base, "--limit", "-1"),
		append(base, "--limit", "1.5"), append(base, "--limit", "99999999999999999999999"),
		append(base, "--rootfile", "other.opf"), append(base, "--task", "x"), append(base, "--timeout", "1"),
		append(base, "BOOK.epub"), append(base, "--output", "out"),
		{"content", "--workspace", "nonexistent", "--resource", "../escape"},
		{"content", "--workspace", "nonexistent", "--resource", invalid},
		append(base, "--help", "--query", strings.Repeat("a", 4097)), append(base, "--help", "--limit", "0"),
	}
	for _, args := range cases {
		r := invoke(t, append(args, "--json"), 2)
		if r["command"] != "content" || r["error"] == nil {
			t.Fatal(r)
		}
	}
	// Put --json before a truly missing trailing value: the existing parser
	// treats the next argv as a literal value, even when it starts with '-'.
	for _, option := range []string{"--query", "--limit", "--workspace", "--resource"} {
		invoke(t, []string{"content", "--json", option}, 2)
	}
	query := strings.Repeat("😀", 1024)
	o, err := parse(append(base, "--query="+query, "--limit=200"))
	if err != nil || o.content.Query == nil || *o.content.Query != query || o.content.Limit == nil || *o.content.Limit != 200 || o.output != "" {
		t.Fatal("option storage", o, err)
	}
	if _, err := validateCommand(o); err != nil {
		t.Fatal("boundary should validate", err)
	}
	o, err = parse(base)
	if err != nil || o.content.Query != nil || o.content.Limit != nil {
		t.Fatal("omitted options", o, err)
	}
}

func TestContentCLIResourceErrorsAndCountBoundaries(t *testing.T) {
	text := `<html xmlns="http://www.w3.org/1999/xhtml"><body>` + strings.Repeat(`<p>same</p>`, 201) + `</body></html>`
	dir, _, _, _ := cliContentWorkspace(t, text)
	base := []string{"content", "--workspace", dir, "--resource", cliContentPath, "--json"}
	for _, tc := range []struct {
		limit    string
		returned int
	}{{"", 50}, {"1", 1}, {"200", 200}} {
		args := append([]string{}, base...)
		if tc.limit != "" {
			args = append(args, "--limit", tc.limit)
		}
		r := invoke(t, args, 0)["data"].(map[string]any)
		if r["matchedCount"] != float64(201) || r["returnedCount"] != float64(tc.returned) || r["truncated"] != true {
			t.Fatal(tc, r)
		}
	}
	for _, tc := range []struct {
		resource string
		exit     int
		code     string
	}{
		{"书/Text/第二%20章.xhtml", 2, "CONTENT_RESOURCE_NOT_DECLARED"}, {"书/Text/第二 章.xhtml#note", 2, "CONTENT_RESOURCE_NOT_DECLARED"}, {"书/text/第二 章.xhtml", 2, "CONTENT_RESOURCE_NOT_DECLARED"}, {"unlisted.bin", 2, "CONTENT_RESOURCE_NOT_DECLARED"},
	} {
		r := invoke(t, []string{"content", "--workspace", dir, "--resource", tc.resource, "--json"}, tc.exit)
		if r["error"].(map[string]any)["code"] != tc.code {
			t.Fatal(r)
		}
	}
	for _, tc := range []struct {
		name, replacement, code string
		exit                    int
	}{
		{"wrong MIME", `media-type="text/html"`, "UNSUPPORTED_CONTENT_TYPE", 3},
		{"missing", `media-type="application/xhtml+xml"`, "MISSING_RESOURCE", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := testfixture.EPUB("3.0", false)
			entries[2].Data = []byte(strings.Replace(string(entries[2].Data), `media-type="application/xhtml+xml"`, tc.replacement, 1))
			if tc.name == "missing" {
				entries = append(entries[:3], entries[4:]...)
			}
			book := filepath.Join(t.TempDir(), "book.epub")
			testfixture.ZIP(t, book, entries)
			ws := filepath.Join(t.TempDir(), "ws")
			w, err := workspace.Create(ws, book, workspace.Options{})
			if err != nil {
				t.Fatal(err)
			}
			w.Close()
			r := invoke(t, []string{"content", "--workspace", ws, "--resource", cliContentPath, "--json"}, tc.exit)
			if r["error"].(map[string]any)["code"] != tc.code {
				t.Fatal(r)
			}
		})
	}
	dir, _, _, _ = cliContentWorkspace(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>`+strings.Repeat("a", (1<<20)+1)+`</p></body></html>`)
	r := invoke(t, []string{"content", "--workspace", dir, "--resource", cliContentPath, "--json"}, 1)
	if r["error"].(map[string]any)["code"] != "CONTENT_LIMIT" {
		t.Fatal(r)
	}
}

func TestContentCLIPersistedRootfile(t *testing.T) {
	entries := testfixture.EPUB("3.0", true)
	entries[len(entries)-1].Data = []byte(`<package xmlns="http://www.idpf.org/2007/opf" version="2.0"><metadata/><manifest><item id="selected" href="书/Text/第二%20章.xhtml" media-type="application/xhtml+xml"/></manifest><spine/></package>`)
	book := filepath.Join(t.TempDir(), "multi.epub")
	testfixture.ZIP(t, book, entries)
	dir := filepath.Join(t.TempDir(), "ws")
	w, err := workspace.Create(dir, book, workspace.Options{Rootfile: "alternate.opf"})
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	r := invoke(t, []string{"content", "--workspace", dir, "--resource", cliContentPath, "--json"}, 0)["data"].(map[string]any)
	if r["rootfile"] != "alternate.opf" || r["matchedCount"] != float64(1) || r["nodes"].([]any)[0].(map[string]any)["text"] != "第二章" {
		t.Fatal(r)
	}
	// The other publication declares this resource, but the selected one does not.
	r = invoke(t, []string{"content", "--workspace", dir, "--resource", "书/nav.xhtml", "--json"}, 2)
	if r["error"].(map[string]any)["code"] != "CONTENT_RESOURCE_NOT_DECLARED" {
		t.Fatal(r)
	}
}

func TestContentBinarySmokeAndBusy(t *testing.T) {
	binary := workspaceBinary(t)
	text := `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>Binary &amp; 😀</p><p>Binary &amp; 😀</p></body></html>`
	dir, _, id, revision := cliContentWorkspace(t, text)
	args := []string{"content", "--workspace", dir, "--resource", cliContentPath, "--query", "Binary & 😀", "--limit", "1"}
	w, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := processJSON(t, binary, args, 4, 5*time.Second)
	if r["error"].(map[string]any)["code"] != "WORKSPACE_BUSY" {
		t.Fatal(r)
	}
	w.Close()
	t.Setenv("PATH", t.TempDir())
	t.Setenv("KEPUB_EPUBCHECK_JAR", "missing-checker")
	r = processJSON(t, binary, args, 0, 5*time.Second)
	data := r["data"].(map[string]any)
	if data["workspaceId"] != id || data["revisionId"] != revision || data["resourceSha256"] != fmt.Sprintf("%x", sha256.Sum256([]byte(text))) || data["matchedCount"] != float64(2) || data["returnedCount"] != float64(1) || data["truncated"] != true {
		t.Fatal(data)
	}
	nodes := data["nodes"].([]any)
	if len(nodes) != 1 || nodes[0].(map[string]any)["text"] != "Binary & 😀" || nodes[0].(map[string]any)["locator"] != "/html[1]/body[1]/p[1]" {
		t.Fatal(nodes)
	}
	processJSON(t, binary, append(args, "--query", "duplicate"), 2, 5*time.Second)
	processJSON(t, binary, []string{"content", "--workspace", dir, "--resource", cliContentPath, "--query", string([]byte{0xff})}, 2, 5*time.Second)
}

func TestContentCapabilityContract(t *testing.T) {
	capabilities := invoke(t, []string{"capabilities", "--json"}, 0)["data"].([]any)
	found := false
	for _, value := range capabilities {
		c := value.(map[string]any)
		if c["operationId"] != "publication.content" {
			continue
		}
		found = true
		if c["implementationStatus"] != "available" || c["risk"] != "read_only" || c["mutatesPublication"] != false || c["requiresGUI"] != false || c["requiresModel"] != false || c["requiresNetwork"] != false {
			t.Fatal(c)
		}
		in := c["inputSchema"].(map[string]any)
		if !reflect.DeepEqual(in["required"], []any{"workspace", "resource"}) {
			t.Fatal(in)
		}
		properties := in["properties"].(map[string]any)
		if len(properties) != 4 || properties["book"] != nil || properties["rootfile"] != nil {
			t.Fatal(properties)
		}
		q := properties["query"].(map[string]any)
		limit := properties["limit"].(map[string]any)
		if q["minLength"] != float64(1) || q["x-maxUtf8Bytes"] != float64(4096) || limit["minimum"] != float64(1) || limit["maximum"] != float64(200) || limit["default"] != float64(50) {
			t.Fatal(properties)
		}
	}
	if !found {
		t.Fatal("publication.content capability absent")
	}
}
