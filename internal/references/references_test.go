package references

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

func graphFixture(t *testing.T, entries []testfixture.Entry) Graph {
	t.Helper()
	file := filepath.Join(t.TempDir(), "references.epub")
	testfixture.ZIP(t, file, entries)
	a, err := archive.Open(file, archive.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	p, err := publication.Load(a, "")
	if err != nil {
		t.Fatal(err)
	}
	g := Build(a, p)
	first, _ := json.Marshal(g)
	second, _ := json.Marshal(Build(a, p))
	if !bytes.Equal(first, second) {
		t.Fatal("nondeterministic graph")
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name, "/") {
			data, err := a.Read(bookpath.BookPath(entry.Name), 256<<20)
			if err != nil || !bytes.Equal(data, entry.Data) {
				t.Fatal("query modified resource", entry.Name, err)
			}
		}
	}
	return g
}

func requireCoverage(t *testing.T, g Graph, resource, syntax, status string) {
	t.Helper()
	for _, c := range g.Coverage {
		if string(c.Resource) == resource && c.Syntax == syntax {
			if c.Status != status || c.ParserVersion != ParserVersion || (status != "complete" && len(c.Reasons) == 0) {
				t.Fatalf("wrong coverage: %+v", c)
			}
			return
		}
	}
	t.Fatal("missing coverage", resource, syntax)
}

func TestReferenceSyntaxAndTargets(t *testing.T) {
	g := graphFixture(t, testfixture.ReferenceEPUB())
	for _, tc := range []struct{ source, syntax, href, target, fragment, query, status, fragmentStatus string }{
		{"书/Deep/package.opf", "opf.manifest", "../Text/第二%20章.xhtml?x=1#note", "书/Text/第二 章.xhtml", "note", "x=1", "resolved", "resolved"},
		{"书/Deep/package.opf", "opf.spine", "c1", "书/Text/-first.xhtml", "", "", "resolved", "not_applicable"},
		{"书/Deep/package.opf", "opf.href", "#t", "书/Deep/package.opf", "t", "", "resolved", "resolved"},
		{"书/Text/-first.xhtml", "xhtml.href", "第二%20章.xhtml?mode=read#note", "书/Text/第二 章.xhtml", "note", "mode=read", "resolved", "resolved"},
		{"书/Text/-first.xhtml", "xhtml.href", "#start", "书/Text/-first.xhtml", "start", "", "resolved", "resolved"},
		{"书/Text/第二 章.xhtml", "xhtml.href", "-first.xhtml#absent", "书/Text/-first.xhtml", "absent", "", "resolved", "missing"},
		{"书/Text/第二 章.xhtml", "xhtml.href", "../missing.xhtml#lost", "书/missing.xhtml", "lost", "", "missing", "blocked"},
		{"书/Text/第二 章.xhtml", "xhtml.href", "https://example.invalid/external#remote", "", "remote", "", "external", "not_checked"},
		{"书/Text/第二 章.xhtml", "xhtml.src", "../Images/cover.svg", "书/Images/cover.svg", "", "", "resolved", "not_applicable"},
		{"书/nav.xhtml", "nav.href", "Text/第二%20章.xhtml?from=toc#note", "书/Text/第二 章.xhtml", "note", "from=toc", "resolved", "resolved"},
		{"书/Images/cover.svg", "svg.href", "#shape", "书/Images/cover.svg", "shape", "", "resolved", "resolved"},
		{"书/Images/cover.svg", "svg.href", "../Text/100%25.xhtml", "书/Text/100%.xhtml", "", "", "resolved", "not_applicable"},
		{"书/Styles/main.css", "css.url", "../Images/cover.svg#shape", "书/Images/cover.svg", "shape", "", "resolved", "resolved"},
		{"书/Styles/main.css", "css.import", "theme.css", "书/Styles/theme.css", "", "", "missing", "blocked"},
		{"书/Styles/main.css", "css.import", "../other.css", "书/other.css", "", "", "missing", "blocked"},
	} {
		found := false
		for _, e := range g.Edges {
			if string(e.Source) == tc.source && e.Syntax == tc.syntax && string(e.Href) == tc.href {
				found = true
				if e.Target == nil || string(e.Target.Path) != tc.target || e.Target.Fragment != tc.fragment || e.Target.Query != tc.query || e.Status != tc.status || e.FragmentStatus != tc.fragmentStatus || e.Location == "" || e.ParserVersion != 1 {
					t.Fatalf("wrong edge for %+v: %+v", tc, e)
				}
			}
		}
		if !found {
			t.Fatal("missing edge", tc)
		}
	}
	inline, stylesheet, unsafe := 0, 0, 0
	for _, e := range g.Edges {
		if strings.Contains(string(e.Href), "ignored") || strings.Contains(string(e.Href), "not-a-reference") {
			t.Fatal("comment or string became URL", e)
		}
		if e.Source == "书/Text/第二 章.xhtml" && e.Syntax == "css.url" {
			inline++
		}
		if e.Source == "书/Styles/main.css" {
			stylesheet++
		}
		if e.Href == "../../../escape.xhtml" {
			unsafe++
			if e.Target != nil || e.Status != "invalid" {
				t.Fatal("unsafe reference resolved", e)
			}
		}
	}
	if inline != 2 || stylesheet != 3 || unsafe != 1 {
		t.Fatal("wrong extraction counts", inline, stylesheet, unsafe)
	}
	for _, code := range []string{"MISSING_REFERENCE_TARGET", "MISSING_FRAGMENT", "INVALID_REFERENCE"} {
		found := false
		for _, d := range g.Diagnostics {
			if d.Code == code && d.BookPath != "" && d.Location != "" {
				found = true
			}
		}
		if !found {
			t.Fatal("missing diagnostic", code, g.Diagnostics)
		}
	}
}

func TestCoverageAndFiltersCannotHideUnknownIncoming(t *testing.T) {
	g := graphFixture(t, testfixture.ReferenceEPUB())
	if g.Status != "partial" {
		t.Fatal(g.Status)
	}
	for _, c := range []struct{ resource, syntax, status string }{
		{"书/Deep/package.opf", "opf.manifest", "complete"},
		{"书/Text/-first.xhtml", "xhtml.href", "complete"},
		{"书/Styles/main.css", "css.url", "complete"},
		{"书/Styles/main.css", "css.grammar", "partial"},
		{"书/Text/第二 章.xhtml", "srcset", "blocked"},
		{"书/Text/第二 章.xhtml", "script", "blocked"},
		{"书/overlay.smil", "smil", "blocked"},
		{"书/book.js", "script", "blocked"},
		{"unlisted.bin", "unknown-resource", "blocked"},
		{"书/missing.xhtml", "resource", "blocked"},
	} {
		requireCoverage(t, g, c.resource, c.syntax, c.status)
	}
	for _, direction := range []string{"incoming", "outgoing", ""} {
		filtered, err := g.Filter("书/Text/-first.xhtml", direction)
		if err != nil || filtered.Status != "partial" || !reflect.DeepEqual(filtered.Coverage, g.Coverage) || !reflect.DeepEqual(filtered.Diagnostics, g.Diagnostics) {
			t.Fatal("filter hid uncertainty", filtered, err)
		}
		// Outgoing has the Next and Self links; incoming has OPF manifest/spine,
		// three nav links, the missing-fragment link, and Self. Both dedups Self.
		want := map[string]int{"outgoing": 2, "incoming": 7, "": 8}[direction]
		if len(filtered.Edges) != want {
			t.Fatalf("%s count = %d, want %d: %+v", direction, len(filtered.Edges), want, filtered.Edges)
		}
		for _, e := range filtered.Edges {
			if direction == "incoming" && (e.Target == nil || e.Target.Path != "书/Text/-first.xhtml") || direction == "outgoing" && e.Source != "书/Text/-first.xhtml" {
				t.Fatal("wrong direction", e)
			}
		}
	}
	empty, err := g.Filter("not-in-book.xhtml", "incoming")
	if err != nil || len(empty.Edges) != 0 || empty.Status != "partial" || len(empty.Coverage) == 0 {
		t.Fatal("empty means unknown, not no references", empty, err)
	}
	for _, tc := range [][2]string{{"", "incoming"}, {"../bad", "outgoing"}, {"x", "both"}, {"x", "other"}} {
		if _, err := g.Filter(tc[0], tc[1]); err == nil {
			t.Fatal("accepted invalid query", tc)
		}
	}
}

func TestBadXMLBlocksOnlyAffectedResource(t *testing.T) {
	for _, content := range []string{`<html><a href="ghost"/></oops>`, `<!DOCTYPE html><html/>`, `<html xmlns="http://www.w3.org/1999/xhtml"><base href="/"/></html>`, `<html xmlns="http://www.w3.org/1999/xhtml" xml:base="elsewhere/"/>`} {
		entries := testfixture.NavigationEPUB("3.0")
		entries[3].Data = []byte(content)
		g := graphFixture(t, entries)
		requireCoverage(t, g, "书/Text/第二 章.xhtml", "xhtml.href", "blocked")
		requireCoverage(t, g, "书/Text/-first.xhtml", "xhtml.href", "complete")
		for _, e := range g.Edges {
			if e.Source == "书/Text/第二 章.xhtml" {
				t.Fatal("blocked document leaked misleading partial references", e)
			}
			if e.Source == "书/Text/-first.xhtml" && e.Target.Path == "书/Text/第二 章.xhtml" && e.FragmentStatus != "not_checked" {
				t.Fatal("fragment checked against unparsed document", e)
			}
		}
		if len(g.Diagnostics) == 0 || g.Status != "partial" {
			t.Fatal("hidden parse failure", g)
		}
	}
}

func TestNCXAndDuplicateIDs(t *testing.T) {
	entries := testfixture.NavigationEPUB("2.0")
	entries[3].Data = []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="note"/><p id="note"/></body></html>`)
	g := graphFixture(t, entries)
	count, ambiguous := 0, 0
	for _, e := range g.Edges {
		if e.Syntax == "ncx.src" {
			count++
			if e.Target.Fragment == "note" && e.FragmentStatus == "ambiguous" {
				ambiguous++
			}
		}
	}
	if count != 4 || ambiguous != 2 {
		t.Fatal("NCX or ambiguous ID handling", count, ambiguous)
	}
}

func TestCompleteExtractionStillReportsContentErrors(t *testing.T) {
	entries := testfixture.NavigationEPUB("3.0")
	entries = append(entries[:6], entries[7:]...) // remove the unknown unlisted binary.
	entries[4].Data = []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="start"/><a href="absent.xhtml#x">Missing</a></body></html>`)
	g := graphFixture(t, entries)
	if g.Status != "complete" || len(g.Diagnostics) != 1 || g.Diagnostics[0].Code != "MISSING_REFERENCE_TARGET" {
		t.Fatalf("extraction completeness conflated with conformance: %+v", g)
	}
}

func TestUnimplementedSyntaxNeverLooksLikeNoReferences(t *testing.T) {
	for _, tc := range []struct{ body, syntax, status string }{
		{`<extra xmlns="urn:unknown" target="hidden.xhtml"/>`, "unknown-xml", "partial"},
		{`<a xmlns:x="http://www.w3.org/1999/xlink" x:href="hidden.xhtml"/>`, "unknown-url-attribute", "blocked"},
		{`<meta http-equiv="refresh" content="0;url=hidden.xhtml"/>`, "xhtml.refresh", "blocked"},
		{`<p onclick="location='hidden.xhtml'"/>`, "script", "blocked"},
		{`<a href="javascript:location='hidden.xhtml'">Run</a>`, "script", "blocked"},
		{`<?xml-stylesheet href="hidden.css"?><p/>`, "xml.processing-instruction", "blocked"},
		{`<style type="text/unknown">hidden.xhtml</style>`, "inline-style", "blocked"},
		{`<style>p{image:image-set('hidden.png' 1x)}</style>`, "inline-style", "partial"},
		{`<svg xmlns="http://www.w3.org/2000/svg"><animate attributeName="href" values="hidden.xhtml"/></svg>`, "svg.animation", "blocked"},
		{`<img src="data:image/svg+xml,hidden"/>`, "embedded-data", "partial"},
		{`<video poster="hidden.png"/>`, "other-url-attribute", "blocked"},
		{`<object classid="payload.bin" type="application/x-test"/>`, "other-url-attribute", "blocked"},
		{`<blockquote cite="hidden.xhtml#quote">Quote</blockquote>`, "other-url-attribute", "blocked"},
		{`<img usemap="#map"/>`, "other-url-attribute", "blocked"},
		{`<html manifest="hidden.appcache"/>`, "other-url-attribute", "blocked"},
		{`<div itemscope="" itemid="hidden.xhtml" itemtype="https://example.invalid/type"/>`, "other-url-attribute", "blocked"},
		{`<a href="#epubcfi(/6/2)">CFI</a>`, "fragment", "partial"},
	} {
		entries := testfixture.NavigationEPUB("3.0")
		entries[3].Data = []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="note"/>` + tc.body + `</body></html>`)
		g := graphFixture(t, entries)
		requireCoverage(t, g, "书/Text/第二 章.xhtml", tc.syntax, tc.status)
		if g.Status != "partial" {
			t.Fatal("unknown syntax hidden", tc)
		}
		filtered, err := g.Filter("书/Text/第二 章.xhtml", "incoming")
		if err != nil || filtered.Status != "partial" || !reflect.DeepEqual(filtered.Coverage, g.Coverage) {
			t.Fatal("incoming filter hid global uncertainty", tc, filtered, err)
		}
	}
	entries := testfixture.NavigationEPUB("3.0")
	entries[2].Data = []byte(strings.Replace(string(entries[2].Data), `properties="nav"`, `properties="nav scripted"`, 1))
	requireCoverage(t, graphFixture(t, entries), "书/nav.xhtml", "script", "blocked")
}

func TestSameElementIDsAndExactCase(t *testing.T) {
	entries := testfixture.NavigationEPUB("3.0")
	entries[3].Data = []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="note" xml:id="note"/><a href="-First.xhtml#start">Case mismatch</a><a href="#%6eote">Encoded</a></body></html>`)
	g := graphFixture(t, entries)
	for _, e := range g.Edges {
		if e.Href == "#%6eote" && (e.Target.Fragment != "note" || e.FragmentStatus != "resolved") {
			t.Fatal("same element IDs or percent fragment mishandled", e)
		}
		if e.Href == "-First.xhtml#start" && e.Status != "missing" {
			t.Fatal("case folded lookup", e)
		}
	}
	for _, d := range g.Diagnostics {
		if d.Code == "DUPLICATE_ID" || d.Code == "AMBIGUOUS_FRAGMENT" {
			t.Fatal("two attributes on one element are not two targets", d)
		}
	}
}

func TestT1BXMLPartialAndBusinessBlocksAreIndependent(t *testing.T) {
	for _, tc := range []struct{ body, syntax string }{
		{`<script/>&unknown;<a href="#known">raw</a>`, "script"},
		{`<base href="elsewhere/"/>&unknown;`, "xhtml.href"},
		{`<a onclick="known" href="#known">&unknown;</a>`, "script"},
	} {
		entries := testfixture.NavigationEPUB("3.0")
		entries[3].Data = []byte(`<!DOCTYPE html [%unread;]><html xmlns="http://www.w3.org/1999/xhtml"><body><p id="known"/>` + tc.body + `</body></html>`)
		g := graphFixture(t, entries)
		requireCoverage(t, g, "书/Text/第二 章.xhtml", tc.syntax, "blocked")
		if g.XMLCoverage == nil || len(g.XMLCoverage.Resources) != 1 || g.XMLCoverage.Resources[0].Status != "partial" {
			t.Fatal("blocked business extraction must retain partial XML provenance", g)
		}
		for _, edge := range g.Edges {
			if edge.Source == "书/Text/第二 章.xhtml" {
				t.Fatal("partial XML must not invent complete URL/ID extraction", edge)
			}
		}
	}
}

// TestCertainIncomingCoverage proves the dependency gate's blocker rules: a
// clean stylesheet's documented literal-form grammar entry blocks nothing
// because css.url/css.import report the extraction completeness, an escape that
// could hide a url() blocks, and a scripted resource blocks. The target id has
// no known incoming edge in every case.
func TestCertainIncomingCoverage(t *testing.T) {
	target := bookpath.BookPath("书/Text/第二 章.xhtml")
	chapter := func(script bool) []byte {
		data := `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><body><p id="note">第二章</p><p id="other">Other</p></body></html>`
		if script {
			data = strings.Replace(data, "</body>", `<script>var x = 1;</script></body>`, 1)
		}
		return []byte(data)
	}
	for _, tc := range []struct {
		name    string
		css     string
		script  bool
		blocked bool
	}{
		{"clean-stylesheet", `p { background: url(../Images/cover.svg#shape); }`, false, false},
		{"escaped-function", `p { background: u\72l(hidden.xhtml#shape); }`, false, true},
		{"scripted-resource", `p { color: red; }`, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := testfixture.EPUB("3.0", false)
			kept := entries[:0]
			for _, entry := range entries {
				if entry.Name != "unlisted.bin" {
					kept = append(kept, entry)
				}
			}
			entries = kept
			entries[2].Data = []byte(strings.Replace(string(entries[2].Data), "</manifest>", `<item id="css" href="../Styles/main.css" media-type="text/css"/></manifest>`, 1))
			entries[3].Data = chapter(tc.script)
			entries = append(entries, testfixture.Entry{Name: "书/Styles/main.css", Data: []byte(tc.css)})
			g := graphFixture(t, entries)
			edges, blockers := g.CertainIncoming(target, "other")
			if len(edges) != 0 {
				t.Fatalf("unexpected edges: %+v", edges)
			}
			if (len(blockers) > 0) != tc.blocked {
				t.Fatalf("blockers=%+v want blocked=%t", blockers, tc.blocked)
			}
		})
	}
}
