package publication

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/LeviTK/Kepub/internal/xmltext"
)

const structureInput = `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>T</title></head><body><h1 id="start">One</h1><p id="mixed">A <em>em</em> B</p><p id="plain">Plain.</p><div id="box"><p>inner</p></div></body></html>`

func structureFixture(t *testing.T) *StructureDocument {
	t.Helper()
	doc, err := ParseStructureDocument([]byte(structureInput), "EPUB/a.xhtml", xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func editFor(t *testing.T, doc *StructureDocument, index int, build func(*xmltext.Element) (*StructureEdit, error)) *StructureEdit {
	t.Helper()
	e, err := doc.Locate(doc.Doc.Elements[index].Location)
	if err != nil {
		t.Fatal(err)
	}
	edit, err := build(e)
	if err != nil {
		t.Fatal(err)
	}
	return edit
}

// TestValidateEditsBoundaries proves that disjoint targets are accepted while
// overlaps, points inside or at a replaced range and coincident insertions are
// refused before any byte is written.
func TestValidateEditsBoundaries(t *testing.T) {
	span := func(op, start, end int) *StructureEdit {
		return &StructureEdit{OpIndex: op, Spans: []EditSpan{{Start: start, End: end}}}
	}
	point := func(op, at int) *StructureEdit {
		return &StructureEdit{OpIndex: op, Points: []EditPoint{{At: at, Bytes: []byte("x")}}}
	}
	for _, tc := range []struct {
		name  string
		edits []*StructureEdit
		ok    bool
	}{
		{"disjoint", []*StructureEdit{span(0, 10, 20), span(1, 30, 40), point(2, 25)}, true},
		{"adjacent-spans", []*StructureEdit{span(0, 10, 20), span(1, 20, 30)}, true},
		{"overlap", []*StructureEdit{span(0, 10, 25), span(1, 20, 30)}, false},
		{"point-inside", []*StructureEdit{span(0, 10, 30), point(1, 20)}, false},
		{"point-at-start", []*StructureEdit{span(0, 10, 30), point(1, 10)}, false},
		{"point-at-end", []*StructureEdit{span(0, 10, 30), point(1, 30)}, false},
		{"point-after", []*StructureEdit{span(0, 10, 30), point(1, 31)}, true},
		{"coincident-points", []*StructureEdit{point(0, 10), point(1, 10)}, false},
		{"distinct-points", []*StructureEdit{point(0, 10), point(1, 11)}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateEdits(tc.edits)
			if tc.ok && err != nil {
				t.Fatalf("refused: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("accepted a conflicting edit set")
			}
		})
	}
}

// TestApplyEditsOrdering applies larger offsets first and keeps a removal ahead
// of an insertion sharing its start, so every frozen coordinate stays valid.
func TestApplyEditsOrdering(t *testing.T) {
	input := []byte("0123456789ABCDEFGHIJ")
	edits := []*StructureEdit{
		{OpIndex: 0, Points: []EditPoint{{At: 10, Bytes: []byte("XX")}}},
		{OpIndex: 1, Spans: []EditSpan{{Start: 10, End: 20}}},
		{OpIndex: 2, Spans: []EditSpan{{Start: 2, End: 4, Bytes: []byte("zz")}}},
	}
	if got := string(ApplyEdits(input, edits)); got != "01zz456789XX" {
		t.Fatalf("apply order: %q", got)
	}
}

// TestStructureEditsExactBytes exercises every operation kind on one frozen
// document and requires the spliced bytes and the independent verification to
// agree, including mixed content and a UTF-16 resource.
func TestStructureEditsExactBytes(t *testing.T) {
	doc := structureFixture(t)
	attr, err := doc.AttributeSetEdit(AttributeSet{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/p[1]", Name: "dir", Value: "rtl"})
	if err != nil {
		t.Fatal(err)
	}
	remove, err := doc.AttributeRemoveEdit(AttributeRemove{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/div[1]", Name: "id", ExpectedOldValue: "box"})
	if err != nil {
		t.Fatal(err)
	}
	insert, err := doc.ElementInsertEdit(ElementInsert{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/h1[1]", Position: "before", Fragment: `<p class="lead">Lead &amp; more</p>`})
	if err != nil {
		t.Fatal(err)
	}
	move, err := doc.ElementMoveEdit(ElementMove{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/p[2]", Anchor: "/html[1]/body[1]/div[1]", Position: "after"})
	if err != nil {
		t.Fatal(err)
	}
	replace, err := doc.ElementReplaceEdit(ElementReplace{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/div[1]/p[1]", Fragment: `<blockquote id="q"><p>Q</p></blockquote>`})
	if err != nil {
		t.Fatal(err)
	}
	deleted, err := doc.ElementDeleteEdit(ElementDelete{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/p[1]/em[1]"})
	if err != nil {
		t.Fatal(err)
	}
	edits := []*StructureEdit{attr, remove, insert, move, replace, deleted}
	for i, e := range edits {
		e.OpIndex = i
	}
	if err := ValidateEdits(edits); err != nil {
		t.Fatal(err)
	}
	out := ApplyEdits([]byte(structureInput), edits)
	if err := VerifyStructure(doc, edits, out); err != nil {
		t.Fatalf("verify: %v", err)
	}
	want := `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>T</title></head><body><p class="lead">Lead &amp; more</p><h1 id="start">One</h1><p id="mixed" dir="rtl">A  B</p><div><blockquote id="q"><p>Q</p></blockquote></div><p id="plain">Plain.</p></body></html>`
	if string(out) != want {
		t.Fatalf("spliced bytes:\n got %q\nwant %q", out, want)
	}
	// Verification is independent: a single tampered byte in the spliced
	// output is refused instead of being reported as a match.
	tampered := bytes.Replace(out, []byte(`class="lead"`), []byte(`class="lead2"`), 1)
	if err := VerifyStructure(doc, edits, tampered); err == nil {
		t.Fatal("tampered splice verified")
	}
	// A UTF-16 resource is edited and re-encoded in its own encoding.
	utf16Input := utf16Bytes(structureInput)
	udoc, err := ParseStructureDocument(utf16Input, "EPUB/u.xhtml", xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
	if err != nil {
		t.Fatal(err)
	}
	uattr, err := udoc.AttributeSetEdit(AttributeSet{BookPath: "EPUB/u.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/p[1]", Name: "lang", Value: "zh"})
	if err != nil {
		t.Fatal(err)
	}
	uattr.OpIndex = 0
	uout := ApplyEdits(utf16Input, []*StructureEdit{uattr})
	if err := VerifyStructure(udoc, []*StructureEdit{uattr}, uout); err != nil {
		t.Fatalf("utf-16 verify: %v", err)
	}
	if !bytes.Contains(uout, utf16Bytes(`lang="zh"`)[2:]) || len(uout)%2 != 0 {
		t.Fatalf("utf-16 splice is not encoded text: %q", uout)
	}
}

// TestParseFragmentContext validates author markup in the insertion context and
// inserts the authored bytes, refusing vocabulary that this batch does not own.
func TestParseFragmentContext(t *testing.T) {
	doc := structureFixture(t)
	scope := doc.Doc.Root.NamespaceScope()
	frag, err := ParseFragment(`<p epub:type="footnote">Note &amp; more</p>`, scope, doc)
	if err != nil {
		t.Fatal(err)
	}
	if string(frag.Bytes) != `<p epub:type="footnote">Note &amp; more</p>` {
		t.Fatalf("fragment bytes: %q", frag.Bytes)
	}
	if len(frag.Nodes) != 1 || frag.Nodes[0].Name.Local != "p" {
		t.Fatalf("fragment nodes: %+v", frag.Nodes)
	}
	for _, tc := range []struct{ name, raw string }{
		{"script", `<script>x()</script>`},
		{"top-level-text", `text<p>x</p>`},
		{"whitespace-between", "<p>x</p>\n<p>y</p>"},
		{"text-only", `hello`},
		{"foreign", `<svg xmlns="http://www.w3.org/2000/svg"/>`},
		{"handler", `<p onclick="x()">x</p>`},
		{"style", `<p style="color: red">x</p>`},
		{"unclosed", `<p>x`},
		{"comment", `<!--x--><p>x</p>`},
		{"declaration", `<?xml version="1.0"?><p>x</p>`},
		{"empty", ``},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseFragment(tc.raw, scope, doc); err == nil {
				t.Fatalf("accepted %q", tc.raw)
			}
		})
	}
	// Several elements without top-level text are one verified block.
	multi, err := ParseFragment(`<p>a</p><p>b</p>`, scope, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(multi.Nodes) != 2 || string(multi.Bytes) != `<p>a</p><p>b</p>` {
		t.Fatalf("multi-element fragment: %+v", multi)
	}
	// A prefix that is not in scope at the insertion point is not invented.
	if _, err := ParseFragment(`<p kepub:type="x">y</p>`, scope, doc); err == nil {
		t.Fatal("accepted an undeclared prefix")
	}
}

// utf16Bytes encodes text as a BOM-marked UTF-16LE XML document.
func utf16Bytes(text string) []byte {
	units := utf16.Encode([]rune(text))
	out := make([]byte, 0, 2+len(units)*2)
	out = append(out, 0xFF, 0xFE)
	for _, u := range units {
		out = binary.LittleEndian.AppendUint16(out, u)
	}
	return out
}

// TestPlannedTargetFollowsInsertions proves review can stay on the planned
// element when an earlier sibling is inserted in the same transaction: the
// frozen locator would otherwise address the inserted element.
func TestPlannedTargetFollowsInsertions(t *testing.T) {
	doc := structureFixture(t)
	insert, err := doc.ElementInsertEdit(ElementInsert{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/p[1]", Position: "before", Fragment: `<p id="new">new</p>`})
	if err != nil {
		t.Fatal(err)
	}
	insert.OpIndex = 0
	path, err := PlannedTarget(doc, []*StructureEdit{insert}, "/html[1]/body[1]/p[1]")
	if err != nil {
		t.Fatal(err)
	}
	out := ApplyEdits([]byte(structureInput), []*StructureEdit{insert})
	cand, err := ParseStructureDocument(out, "EPUB/a.xhtml", xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
	if err != nil {
		t.Fatal(err)
	}
	n := cand.Doc.Root
	for _, index := range path {
		if index >= len(n.Children) {
			t.Fatal("planned path left the candidate tree")
		}
		n = n.Children[index]
	}
	if id, ok := elementAttribute(n, xml.Name{Local: "id"}); !ok || id != "mixed" {
		t.Fatalf("planned path resolved to %+v", n.Name)
	}
	// The candidate addresses the same element under a shifted locator: the
	// frozen locator alone would have matched the inserted sibling.
	if n.Location != "/html[1]/body[1]/p[2]" {
		t.Fatalf("candidate location %q", n.Location)
	}
}
