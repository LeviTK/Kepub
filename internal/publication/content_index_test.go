package publication

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

func r7SearchFixture(t testing.TB, count int) (*archive.Archive, *Publication) {
	t.Helper()
	entries := testfixture.EPUB("3.0", false)[:3]
	var manifest strings.Builder
	for i := count - 1; i >= 0; i-- {
		name := fmt.Sprintf("page-%05d.xhtml", i)
		fmt.Fprintf(&manifest, `<item id="p%d" href="%s" media-type="application/xhtml+xml"/>`, i, name)
		entries = append(entries, testfixture.Entry{Name: "书/Deep/" + name, Data: []byte(contentHTML(fmt.Sprintf(`<p id="p%d">needle %05d 中文</p>`, i, i)))})
	}
	opf := string(entries[2].Data)
	start, end := strings.Index(opf, "<manifest>"), strings.Index(opf, "</spine>")+len("</spine>")
	entries[2].Data = []byte(opf[:start] + "<manifest>" + manifest.String() + `</manifest><spine><itemref idref="p0"/></spine>` + opf[end:])
	filename := filepath.Join(t.TempDir(), "search.epub")
	testfixture.ZIP(t, filename, entries)
	a, err := archive.Open(filename, archive.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	p, err := Load(a, "")
	if err != nil {
		t.Fatal(err)
	}
	return a, p
}

func TestR7ManifestLookupScaling(t *testing.T) {
	for _, count := range []int{100, 1000, 10000} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			a, p := r7SearchFixture(t, count)
			query, limit, lookups := "needle", 1, 0
			r, err := SearchContent(a, p, ContentOptions{Query: &query, Limit: &limit, onManifestLookup: func() { lookups++ }})
			if err != nil || r.MatchedCount != count || r.ReturnedCount != 1 || !r.Truncated {
				t.Fatalf("search/count changed: %+v %v", r, err)
			}
			wantPath := bookpath.BookPath(fmt.Sprintf("书/Deep/page-%05d.xhtml", count-1))
			wantData := contentHTML(fmt.Sprintf(`<p id="p%d">needle %05d 中文</p>`, count-1, count-1))
			if r.Results[0].BookPath != wantPath || r.Results[0].ResourceSHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(wantData))) || r.Results[0].Locator != "/html[1]/body[1]/p[1]" || r.Results[0].Text != fmt.Sprintf("needle %05d 中文", count-1) {
				t.Fatalf("order/hash/locator changed: %+v", r.Results)
			}
			t.Logf("manifest=%d XHTML=%d membership_work=%d", count, count, lookups)
			if lookups != 2*count {
				t.Fatalf("repeated manifest scans: got %d membership checks, want M+X=%d", lookups, 2*count)
			}
		})
	}
}

func BenchmarkR7ManifestSearch(b *testing.B) {
	for _, count := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			a, p := r7SearchFixture(b, count)
			query, limit, lookups := "needle", 1, 0
			if _, err := SearchContent(a, p, ContentOptions{Query: &query, Limit: &limit, onManifestLookup: func() { lookups++ }}); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				r, err := SearchContent(a, p, ContentOptions{Query: &query, Limit: &limit})
				if err != nil || r.MatchedCount != count || r.ReturnedCount != 1 {
					b.Fatal(r, err)
				}
			}
			b.ReportMetric(float64(lookups), "membership/op")
		})
	}
}

func TestR7ManifestDeclarationSemantics(t *testing.T) {
	first, second := contentHTML(`<p id="a">needle A</p>`), contentHTML(`<p id="b">needle B</p>`)
	a, p := searchFixture(t, first, second)
	query := "needle"
	p.Manifest = append(p.Manifest, p.Manifest[0])
	r, err := SearchContent(a, p, ContentOptions{Query: &query})
	aID, bID := "a", "b"
	want := []SearchResult{
		{contentPath, fmt.Sprintf("%x", sha256.Sum256([]byte(first))), 1, ContentNode{XHTMLNamespace, "p", &aID, "/html[1]/body[1]/p[1]", "needle A", false}},
		{"书/Text/-first.xhtml", fmt.Sprintf("%x", sha256.Sum256([]byte(second))), 1, ContentNode{XHTMLNamespace, "p", &bID, "/html[1]/body[1]/p[1]", "needle B", false}},
	}
	want = append(want, want[0])
	if err != nil || r.MatchedCount != 3 || r.ReturnedCount != 3 || r.Truncated || !reflect.DeepEqual(r.Results, want) {
		t.Fatalf("duplicate/order/full result changed: %+v %v", r, err)
	}
	// The next call must observe changed declarations, not a persistent index.
	p.Manifest[len(p.Manifest)-1].MediaType = "text/html"
	_, err = SearchContent(a, p, ContentOptions{Query: &query})
	contentError(t, err, "UNSUPPORTED_CONTENT_TYPE")
	_, err = ReadContent(a, p, contentPath, ContentOptions{})
	contentError(t, err, "UNSUPPORTED_CONTENT_TYPE")
	for _, lastXHTML := range []bool{false, true} {
		t.Run(fmt.Sprint(lastXHTML), func(t *testing.T) {
			a, p := searchFixture(t, first, second)
			conflict, good := p.Manifest[0], p.Manifest[0]
			conflict.MediaType = "text/html"
			if lastXHTML {
				p.Manifest = append(p.Manifest, conflict, good)
			} else {
				p.Manifest = append(p.Manifest, good, conflict)
			}
			_, err := SearchContent(a, p, ContentOptions{Query: &query})
			contentError(t, err, "UNSUPPORTED_CONTENT_TYPE")
			_, err = ReadContent(a, p, contentPath, ContentOptions{})
			contentError(t, err, "UNSUPPORTED_CONTENT_TYPE")
		})
	}
	for _, path := range []bookpath.BookPath{"unlisted.bin", "书/text/第二 章.xhtml", "书/Text/第二%20章.xhtml"} {
		_, err := ReadContent(a, p, path, ContentOptions{})
		contentError(t, err, "CONTENT_RESOURCE_NOT_DECLARED")
	}
	// Only non-XHTML declarations are not searched (the old behavior).
	p.Manifest = p.Manifest[:1]
	p.Manifest[0].MediaType = "text/html"
	r, err = SearchContent(a, p, ContentOptions{Query: &query})
	if err != nil || r.MatchedCount != 0 || r.ReturnedCount != 0 || r.Truncated || r.Results == nil {
		t.Fatalf("non-XHTML traversal changed: %+v %v", r, err)
	}
}

func TestR7SearchLateFailureAfterLimit(t *testing.T) {
	for _, tc := range []struct{ name, data, code string }{
		{"malformed", "<html", "XML_NOT_WELL_FORMED"},
		{"entity", contentHTML(`<p>&unknown;</p>`), "XML_NOT_WELL_FORMED"},
		{"missing", contentHTML(`<p>needle later</p>`), "MISSING_RESOURCE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, p := searchFixture(t, contentHTML(`<p>needle first</p>`), tc.data)
			if tc.name == "missing" {
				p.Manifest[1].Path = "missing.xhtml"
			}
			query, limit := "needle", 1
			r, err := SearchContent(a, p, ContentOptions{Query: &query, Limit: &limit})
			contentError(t, err, tc.code)
			if !reflect.DeepEqual(r, Search{}) {
				t.Fatalf("partial result after late failure: %+v", r)
			}
		})
	}
}

func TestR7SmallScanBudget(t *testing.T) {
	a, p := r7SearchFixture(t, 2)
	query, limit := "needle", 1
	var total int64
	for i := 0; i < 2; i++ {
		total += int64(len(contentHTML(fmt.Sprintf(`<p id="p%d">needle %05d 中文</p>`, i, i))))
	}
	for _, extra := range []int64{-1, 0, 1} {
		t.Run(fmt.Sprint(extra), func(t *testing.T) {
			lookups := 0
			r, err := SearchContent(a, p, ContentOptions{Query: &query, Limit: &limit, scanLimit: total + extra, onManifestLookup: func() { lookups++ }})
			if extra < 0 {
				contentError(t, err, "CONTENT_LIMIT")
				if !reflect.DeepEqual(r, Search{}) || lookups != 3 {
					t.Fatalf("late over-budget resource read/partial result: %+v lookups=%d", r, lookups)
				}
			} else if err != nil || r.MatchedCount != 2 || r.ReturnedCount != 1 || !r.Truncated || lookups != 4 {
				t.Fatalf("small exact budget/control: %+v %v lookups=%d", r, err, lookups)
			}
		})
	}
	p.Manifest = append(p.Manifest, p.Manifest[0])
	_, err := SearchContent(a, p, ContentOptions{Query: &query, Limit: &limit, scanLimit: total})
	contentError(t, err, "CONTENT_LIMIT") // duplicate bytes still consume scan budget
}
