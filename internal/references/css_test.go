package references

import (
	"reflect"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
)

func cssGraph(css string) Graph {
	b := builder{a: &archive.Archive{Files: map[bookpath.BookPath]int64{}}, covered: map[string]int{}}
	b.css("Styles/main.css", "stylesheet", css)
	return b.g
}

func TestCSSLexicalBoundaries(t *testing.T) {
	for _, tc := range []struct {
		css, status string
		want        []string
	}{
		{`/*url(no)*/ a { content:"url(no2)"; x:myurl(no3); background:url( '../img/a%20b.svg#shape' ); } @IMPORT/*x*/"print.css"; @import url(../other.css);`, "complete", []string{"../img/a%20b.svg#shape", "print.css", "../other.css"}},
		{`a {background:url(data:image/png;base64,AAAA); x: url(//example.invalid/a);}`, "complete", []string{"data:image/png;base64,AAAA", "//example.invalid/a"}},
		{`x { image: image-set("hidden.png" 1x); }`, "complete", []string{}},
		{`x { background: url("bad\20escape.svg"); }`, "partial", []string{}},
		{`x { background: u\72l(hidden.png); }`, "partial", []string{}},
		{`x { background: url(a b.svg); }`, "partial", []string{}},
		{`@import `, "partial", []string{}},
		{`url("unfinished`, "partial", []string{}},
		{`/* unfinished url(x)`, "partial", []string{}},
		{string([]byte{0xff, 0xfe, 'x'}), "blocked", []string{}},
	} {
		g := cssGraph(tc.css)
		hrefs := []string{}
		for _, e := range g.Edges {
			hrefs = append(hrefs, string(e.Href))
		}
		if !reflect.DeepEqual(hrefs, tc.want) {
			t.Fatalf("%q: %v want %v", tc.css, hrefs, tc.want)
		}
		requireCoverage(t, g, "Styles/main.css", "css.url", tc.status)
		requireCoverage(t, g, "Styles/main.css", "css.import", tc.status)
		requireCoverage(t, g, "Styles/main.css", "css.grammar", "partial")
	}
}

// Generate supported references, then put competing url-looking text inside
// comments/strings. The expected URL comes from input, not from the lexer.
func FuzzCSSLiteralURLs(f *testing.F) {
	for _, seed := range []string{"chapter", "中文", "a%20b", "100%25"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, seed string) {
		if len(seed) > 128 {
			return
		}
		name := strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
				return r
			}
			return 'x'
		}, seed)
		href := "../Text/" + name + "%20chapter.xhtml#id"
		g := cssGraph(`/*url(ignored)*/ p{content:"url(also-ignored)";background:url("` + href + `")} @import 'theme.css';`)
		if len(g.Edges) != 2 || string(g.Edges[0].Href) != href || g.Edges[1].Href != "theme.css" || g.Edges[0].Target.Path != bookpath.BookPath("Text/"+name+" chapter.xhtml") || g.Edges[0].Target.Fragment != "id" {
			t.Fatalf("literal lexer changed URL: %+v", g.Edges)
		}
	})
}
