package publication

import (
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

const contentPath bookpath.BookPath = "书/Text/第二 章.xhtml"

func contentFixture(t testing.TB, data string) (*archive.Archive, *Publication) {
	t.Helper()
	entries := testfixture.EPUB("3.0", false)
	entries[3].Data = []byte(data)
	filename := filepath.Join(t.TempDir(), "book.epub")
	testfixture.ZIP(t, filename, entries)
	original, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	a, err := archive.Open(filename, archive.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer a.Close()
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name, "/") {
				continue
			}
			got, err := a.Read(bookpath.BookPath(entry.Name), 256<<20)
			if err != nil || !bytes.Equal(got, entry.Data) {
				t.Fatalf("resource changed: %s %v", entry.Name, err)
			}
		}
		got, err := os.ReadFile(filename)
		if err != nil || !bytes.Equal(got, original) {
			t.Fatal("whole archive changed", err)
		}
	})
	p, err := Load(a, "")
	if err != nil {
		t.Fatal(err)
	}
	return a, p
}

func contentHTML(body string) string {
	return `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Hidden</title></head><body>` + body + `</body></html>`
}

func contentError(t testing.TB, err error, code string) {
	t.Helper()
	var fe *fault.Error
	if !errors.As(err, &fe) || fe.Code != code {
		t.Fatalf("want %s, got %v", code, err)
	}
}

func TestContentDecodedOrderSelectionAndHash(t *testing.T) {
	raw := "\xef\xbb\xbf" + contentHTML("<div>\r\n <p id=\"same\"> A&amp;&#x1F600;\r\nZ </p><p id=\"same\">A&amp;😀\nZ</p><p>Before <b>bold</b> after</p><section> <i/> </section><p> \t</p></div>")
	a, p := contentFixture(t, raw)
	r, err := ReadContent(a, p, contentPath, ContentOptions{})
	if err != nil {
		t.Fatal(err)
	}
	id := "same"
	want := []ContentNode{
		{XHTMLNamespace, "p", &id, "/html[1]/body[1]/div[1]/p[1]", " A&😀\nZ ", false},
		{XHTMLNamespace, "p", &id, "/html[1]/body[1]/div[1]/p[2]", "A&😀\nZ", false},
		{XHTMLNamespace, "p", nil, "/html[1]/body[1]/div[1]/p[3]", "Before bold after", true},
		{XHTMLNamespace, "b", nil, "/html[1]/body[1]/div[1]/p[3]/b[1]", "bold", false},
		{XHTMLNamespace, "i", nil, "/html[1]/body[1]/div[1]/section[1]/i[1]", "", false},
		{XHTMLNamespace, "p", nil, "/html[1]/body[1]/div[1]/p[4]", " \t", false},
	}
	if !reflect.DeepEqual(r.Nodes, want) || r.MatchedCount != 6 || r.ReturnedCount != 6 || r.Truncated {
		t.Fatalf("selection/order: %+v", r)
	}
	if r.ResourceSHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(raw))) || r.BookPath != contentPath || r.LocatorVersion != 1 {
		t.Fatalf("raw hash/binding: %+v", r)
	}
	for _, tc := range []struct {
		query string
		count int
	}{
		{"A&😀\nZ", 2}, {"bold", 2}, {"Before bold", 1}, {"Beforebold", 0}, {"ZBefore", 0}, {"BOLD", 0}, {".*", 0}, {"Hidden", 0}, {" \t", 1}, {"A&amp;", 0},
	} {
		r, err := ReadContent(a, p, contentPath, ContentOptions{Query: &tc.query})
		if err != nil || r.MatchedCount != tc.count || r.ReturnedCount != tc.count || r.Nodes == nil {
			t.Fatalf("query %q: %+v %v", tc.query, r, err)
		}
	}
}

func TestContentEmptyAndNamespacePrefix(t *testing.T) {
	a, p := contentFixture(t, contentHTML("body-only text"))
	r, err := ReadContent(a, p, contentPath, ContentOptions{})
	if err != nil || r.Nodes == nil || r.ReturnedCount != 0 || r.MatchedCount != 0 || r.Truncated {
		t.Fatalf("body is not a result: %+v %v", r, err)
	}
	a, p = contentFixture(t, `<h:html xmlns:h="http://www.w3.org/1999/xhtml"><h:body><h:p><![CDATA[é  next]]></h:p></h:body></h:html>`)
	for _, tc := range []struct {
		query string
		count int
	}{{"é  next", 1}, {"é", 0}, {"é next", 0}} {
		r, err := ReadContent(a, p, contentPath, ContentOptions{Query: &tc.query})
		if err != nil || r.MatchedCount != tc.count {
			t.Fatalf("prefix/normalization %q: %+v %v", tc.query, r, err)
		}
		if tc.count == 1 && (r.Nodes[0].Text != "é  next" || r.Nodes[0].Namespace != XHTMLNamespace || r.Nodes[0].Locator != "/html[1]/body[1]/p[1]") {
			t.Fatalf("prefix/text: %+v", r)
		}
	}
}

func TestContentExcludedSubtreesAndAncestors(t *testing.T) {
	a, p := contentFixture(t, contentHTML(`<div>outer<section>direct<p>visible</p><script>secret<b>script child</b></script></section><p>allowed</p><svg xmlns="urn:svg">secret<p xmlns="http://www.w3.org/1999/xhtml">reentry</p></svg><style>secret</style><head><p>secret</p></head><p xmlns="">secret</p><x:p xmlns:x="urn:x">secret</x:p><p>last</p></div>`))
	r, err := ReadContent(a, p, contentPath, ContentOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := []ContentNode{
		{XHTMLNamespace, "p", nil, "/html[1]/body[1]/div[1]/section[1]/p[1]", "visible", false},
		{XHTMLNamespace, "p", nil, "/html[1]/body[1]/div[1]/p[1]", "allowed", false},
		{XHTMLNamespace, "p", nil, "/html[1]/body[1]/div[1]/p[4]", "last", false},
	}
	if !reflect.DeepEqual(r.Nodes, want) {
		t.Fatalf("excluded text leaked or supported child lost: %+v", r)
	}
	for _, query := range []string{"secret", "reentry", "outer", "script child"} {
		r, err := ReadContent(a, p, contentPath, ContentOptions{Query: &query})
		if err != nil || r.MatchedCount != 0 {
			t.Fatalf("excluded query %q: %+v %v", query, r, err)
		}
	}
}

func TestContentExactManifestAndStructure(t *testing.T) {
	a, p := contentFixture(t, contentHTML(`<p>ok</p>`))
	for _, path := range []bookpath.BookPath{"Text/第二 章.xhtml", "书/Text/第二%20章.xhtml", "书/Text/第二 章.xhtml#id", "书/text/第二 章.xhtml", "unlisted.bin"} {
		_, err := ReadContent(a, p, path, ContentOptions{})
		contentError(t, err, "CONTENT_RESOURCE_NOT_DECLARED")
	}
	other := &Publication{Manifest: p.Manifest[1:]}
	_, err := ReadContent(a, other, contentPath, ContentOptions{})
	contentError(t, err, "CONTENT_RESOURCE_NOT_DECLARED")
	p.Manifest[0].MediaType = "text/html"
	_, err = ReadContent(a, p, contentPath, ContentOptions{})
	contentError(t, err, "UNSUPPORTED_CONTENT_TYPE")
	p.Manifest[0].MediaType = "application/xhtml+xml"
	p.Manifest[0].Path = "missing.xhtml"
	_, err = ReadContent(a, p, "missing.xhtml", ContentOptions{})
	contentError(t, err, "MISSING_RESOURCE")
	for _, body := range []string{`<body/><body/>`, `<div><body/></div>`, `<body><body><p>nested</p></body></body>`, `<head/>`} {
		a, p := contentFixture(t, `<html xmlns="http://www.w3.org/1999/xhtml">`+body+`</html>`)
		_, err := ReadContent(a, p, contentPath, ContentOptions{})
		contentError(t, err, "CONTENT_STRUCTURE")
	}
	for _, tc := range []struct{ data, code string }{
		{`<html><body><p>wrong namespace</p></body></html>`, "CONTENT_STRUCTURE"},
		{contentHTML(`<p>&unknown;</p>`), "XML_NOT_WELL_FORMED"},
		{`<!DOCTYPE html>` + contentHTML(`<p/>`), "XML_DTD_FORBIDDEN"},
		{contentHTML(`<p xml:base="x">x</p>`), "UNSUPPORTED_XML_BASE"},
		{contentHTML(strings.Repeat("<div>", 129) + strings.Repeat("</div>", 129)), "XML_LIMIT"},
		{contentHTML(strings.Repeat(`<p/>`, 100001)), "XML_LIMIT"},
		{contentHTML(strings.Repeat("<div>", 40) + strings.Repeat("a", 1<<20) + strings.Repeat("</div>", 40)), "XML_LIMIT"},
		{string([]byte{0xff}), "UNSUPPORTED_XML_ENCODING"},
		{strings.Repeat("x", (8<<20)+1), "RESOURCE_LIMIT"},
	} {
		a, p := contentFixture(t, tc.data)
		_, err := ReadContent(a, p, contentPath, ContentOptions{})
		contentError(t, err, tc.code)
	}
}

func TestContentOptionsAndCountLimits(t *testing.T) {
	for _, q := range []string{"", strings.Repeat("a", 4097), strings.Repeat("😀", 1024) + "a", string([]byte{0xff})} {
		_, err := ReadContent(nil, nil, contentPath, ContentOptions{Query: &q})
		contentError(t, err, "INVALID_CONTENT_QUERY")
	}
	for _, n := range []int{-1, 0, 201} {
		_, err := ReadContent(nil, nil, contentPath, ContentOptions{Limit: &n})
		contentError(t, err, "INVALID_CONTENT_LIMIT")
	}
	for _, q := range []string{"a", " ", strings.Repeat("a", 4096), strings.Repeat("😀", 1024)} {
		a, p := contentFixture(t, contentHTML("<p>"+q+"</p>"))
		r, err := ReadContent(a, p, contentPath, ContentOptions{Query: &q})
		if err != nil || r.MatchedCount != 1 {
			t.Fatalf("valid boundary: %+v %v", r, err)
		}
	}
	for _, tc := range []struct {
		total, limit, returned int
		truncated              bool
	}{
		{49, 0, 49, false}, {50, 0, 50, false}, {51, 0, 50, true}, {2, 1, 1, true}, {199, 200, 199, false}, {200, 200, 200, false}, {201, 200, 200, true},
	} {
		var body strings.Builder
		for i := 0; i < tc.total; i++ {
			fmt.Fprintf(&body, "<p>%d</p>", i)
		}
		a, p := contentFixture(t, contentHTML(body.String()))
		o := ContentOptions{}
		if tc.limit != 0 {
			o.Limit = &tc.limit
		}
		r, err := ReadContent(a, p, contentPath, o)
		if err != nil || r.MatchedCount != tc.total || r.ReturnedCount != tc.returned || r.Truncated != tc.truncated || r.Nodes[len(r.Nodes)-1].Text != fmt.Sprint(tc.returned-1) {
			t.Fatalf("counts %+v: %+v %v", tc, r, err)
		}
	}
}

func TestContentReturnedTextBudget(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		limit      int
		fail       bool
		count      int
	}{
		{"exact", "<p>" + strings.Repeat("😀", (1<<20)/4) + "</p>", 50, false, 1},
		{"over single", "<p>" + strings.Repeat("a", (1<<20)+1) + "</p>", 50, true, 0},
		{"over cumulative", "<p>" + strings.Repeat("a", 1<<19) + "</p><p>" + strings.Repeat("b", (1<<19)+1) + "</p>", 50, true, 0},
		{"overlap counts twice", "<p>x<b>" + strings.Repeat("a", 1<<19) + "</b></p>", 50, true, 0},
		{"not returned", "<p>small</p><p>" + strings.Repeat("a", (1<<20)+1) + "</p>", 1, false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, p := contentFixture(t, contentHTML(tc.body))
			r, err := ReadContent(a, p, contentPath, ContentOptions{Limit: &tc.limit})
			if tc.fail {
				contentError(t, err, "CONTENT_LIMIT")
				if len(r.Nodes) != 0 {
					t.Fatal("partial result on error")
				}
				return
			}
			if err != nil || r.MatchedCount != tc.count {
				t.Fatalf("budget: %+v %v", r, err)
			}
		})
	}
}

func FuzzContentLiteralAndExclusion(f *testing.F) {
	f.Add("Hello", "ell")
	f.Add("😀 & < >\r\n", "😀")
	f.Add("same same", "SAME")
	f.Fuzz(func(t *testing.T, text, query string) {
		if len(text) > 4096 || len(query) > 4096 || query == "" || !utf8.ValidString(text) || !utf8.ValidString(query) {
			return
		}
		for _, r := range text {
			if !(r == 9 || r == 10 || r == 13 || r >= 0x20 && r <= 0xD7FF || r >= 0xE000 && r <= 0xFFFD || r >= 0x10000 && r <= 0x10FFFF) {
				return
			}
		}
		var escaped bytes.Buffer
		if err := xml.EscapeText(&escaped, []byte(text)); err != nil {
			t.Fatal(err)
		}
		a, p := contentFixture(t, contentHTML(`<div>ancestor<script>`+escaped.String()+`</script><p>`+escaped.String()+`</p></div>`))
		all, err := ReadContent(a, p, contentPath, ContentOptions{})
		if err != nil || all.MatchedCount != 1 || len(all.Nodes) != 1 || all.Nodes[0].Text != text {
			t.Fatalf("decoded/excluded: %+v %v", all, err)
		}
		r, err := ReadContent(a, p, contentPath, ContentOptions{Query: &query})
		// EscapeText encodes CR as an entity, so it survives XML line decoding.
		// Compare to the original, not to another parse of the generated XML.
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if strings.Contains(text, query) {
			want = 1
		}
		if r.MatchedCount != want || r.ReturnedCount != want {
			t.Fatalf("literal/exclusion: %+v", r)
		}
		if want == 1 && (r.Nodes[0].Text != text || r.Nodes[0].Locator != "/html[1]/body[1]/div[1]/p[1]") {
			t.Fatalf("text/locator: %+v", r)
		}
	})
}
