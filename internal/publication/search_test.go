package publication

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

func searchFixture(t *testing.T, first, second string) (*archive.Archive, *Publication) {
	t.Helper()
	entries := testfixture.EPUB("3.0", false)
	entries[3].Data = []byte(first)
	entries[4].Data = []byte(second)
	file := filepath.Join(t.TempDir(), "search.epub")
	testfixture.ZIP(t, file, entries)
	a, err := archive.Open(file, archive.DefaultLimits)
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

func TestSearchAllResourcesAndExclusions(t *testing.T) {
	a, p := searchFixture(t, contentHTML(`<p>Same</p><p>Same Same</p><div>head<script>Same</script><p>safe Same</p></div><p>Case</p>`), contentHTML(`<p>Same</p><p>prefix <em>Same</em> suffix</p>`))
	q, n := "Same", 1
	r, err := SearchContent(a, p, ContentOptions{Query: &q, Limit: &n})
	if err != nil || r.MatchedCount != 6 || r.ReturnedCount != 1 || !r.Truncated || r.Results[0].BookPath != contentPath || r.Results[0].Locator != "/html[1]/body[1]/p[1]" {
		t.Fatal(r, err)
	}
	n = 200
	r, err = SearchContent(a, p, ContentOptions{Query: &q, Limit: &n})
	if err != nil || len(r.Results) != 6 || r.Results[3].BookPath != "书/Text/-first.xhtml" || r.Results[0].Locator != r.Results[3].Locator || r.Results[0].ResourceSHA256 == r.Results[3].ResourceSHA256 {
		t.Fatal("resource identity/count conflation", r, err)
	}
	for _, query := range []string{"same", "Sameprefix", "head", "Hidden"} {
		q = query
		r, err = SearchContent(a, p, ContentOptions{Query: &q})
		if err != nil || r.MatchedCount != 0 {
			t.Fatal(query, r, err)
		}
	}
	for _, late := range []string{"<html", contentHTML(`<p>&undeclared;</p>`), contentHTML(`<body/>`)} {
		a, p := searchFixture(t, contentHTML(`<p>Same</p>`), late)
		q = "Same"
		n = 1
		if _, err := SearchContent(a, p, ContentOptions{Query: &q, Limit: &n}); err == nil {
			t.Fatal("bad late resource skipped after return limit")
		}
	}
	if _, err := SearchContent(a, p, ContentOptions{}); err == nil {
		t.Fatal("query optional")
	}
}

func TestSearchReturnedTextBudgetAcrossFiles(t *testing.T) {
	for _, extra := range []int{0, 1} {
		a, p := searchFixture(t, contentHTML(`<p>`+strings.Repeat("x", ContentTextLimit/2)+`</p>`), contentHTML(`<p>`+strings.Repeat("x", ContentTextLimit/2+extra)+`</p>`))
		q := "x"
		r, err := SearchContent(a, p, ContentOptions{Query: &q})
		if extra == 0 {
			if err != nil || r.MatchedCount != 2 || r.ReturnedCount != 2 {
				t.Fatal("exact returned budget", err)
			}
		} else {
			contentError(t, err, "CONTENT_LIMIT")
		}
		n := 1
		r, err = SearchContent(a, p, ContentOptions{Query: &q, Limit: &n})
		if err != nil || r.MatchedCount != 2 || r.ReturnedCount != 1 || !r.Truncated {
			t.Fatal("unreturned text counted in output budget", r, err)
		}
	}
}

func TestSearchRawScanBoundary(t *testing.T) {
	const prefix = `<html xmlns="http://www.w3.org/1999/xhtml"><body><!--`
	const suffix = `--><p>nothing</p></body></html>`
	data := []byte(prefix + strings.Repeat("x", XMLLimit-len(prefix)-len(suffix)) + suffix)
	entries := testfixture.EPUB("3.0", false)
	// Replace only the manifest/spine of the known publication. All 17 source
	// resources exist physically: neither metadata estimates nor fake readers.
	manifest := ""
	for i := 0; i < 17; i++ {
		name := fmt.Sprintf("page-%02d.xhtml", i)
		b := data
		if i == 16 {
			b = []byte("x")
		} // exactly one raw byte beyond 128 MiB
		entries = append(entries, testfixture.Entry{Name: "书/Deep/" + name, Data: b})
		manifest += fmt.Sprintf(`<item id="page%d" href="%s" media-type="application/xhtml+xml"/>`, i, name)
	}
	opf := string(entries[2].Data)
	start, end := strings.Index(opf, "<manifest>"), strings.Index(opf, "</spine>")+len("</spine>")
	entries[2].Data = []byte(opf[:start] + "<manifest>" + manifest + `</manifest><spine><itemref idref="page0"/></spine>` + opf[end:])
	file := filepath.Join(t.TempDir(), "scan.epub")
	testfixture.ZIP(t, file, entries)
	a, err := archive.Open(file, archive.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	p, err := Load(a, "")
	if err != nil {
		t.Fatal(err)
	}
	q := "absent"
	all := p.Manifest
	p.Manifest = all[:16]
	r, err := SearchContent(a, p, ContentOptions{Query: &q})
	if err != nil || r.MatchedCount != 0 || len(r.Results) != 0 {
		t.Fatal("128 MiB exact scan", err)
	}
	p.Manifest = all
	_, err = SearchContent(a, p, ContentOptions{Query: &q})
	contentError(t, err, "CONTENT_LIMIT")
}
