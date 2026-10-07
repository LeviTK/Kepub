package publication

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
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

// TestVerifyStructureRejectsMisplacedBlock keeps the independent verifier from
// accepting a block that is byte-identical but placed before trailing text.
func TestVerifyStructureRejectsMisplacedBlock(t *testing.T) {
	input := `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>T</title></head><body><p>before<em>x</em>after</p></body></html>`
	doc, err := ParseStructureDocument([]byte(input), "EPUB/a.xhtml", xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
	if err != nil {
		t.Fatal(err)
	}
	e, err := doc.ElementInsertEdit(ElementInsert{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/p[1]", Position: "last-child", Fragment: `<strong>N</strong>`})
	if err != nil {
		t.Fatal(err)
	}
	edits := []*StructureEdit{e}
	good := ApplyEdits([]byte(input), edits)
	if err := VerifyStructure(doc, edits, good); err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(good), `after<strong>N</strong>`, `<strong>N</strong>after`, 1)
	if bad == string(good) {
		t.Fatal("bad fixture did not change")
	}
	if err := VerifyStructure(doc, edits, []byte(bad)); err == nil {
		t.Fatal("accepted a last-child insertion placed before trailing text")
	}
	// A first-child block must stay immediately after the start tag.
	first, err := doc.ElementInsertEdit(ElementInsert{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/p[1]", Position: "first-child", Fragment: `<strong>F</strong>`})
	if err != nil {
		t.Fatal(err)
	}
	firstGood := ApplyEdits([]byte(input), []*StructureEdit{first})
	if err := VerifyStructure(doc, []*StructureEdit{first}, firstGood); err != nil {
		t.Fatal(err)
	}
	firstBad := strings.Replace(string(firstGood), `<p><strong>F</strong>before`, `<p>before<strong>F</strong>`, 1)
	if firstBad == string(firstGood) {
		t.Fatal("bad first-child fixture did not change")
	}
	if err := VerifyStructure(doc, []*StructureEdit{first}, []byte(firstBad)); err == nil {
		t.Fatal("accepted a first-child insertion after leading text")
	}
}

// TestVerifyStructureRejectsMisplacedReplace keeps the replaced element's
// surrounding character data part of the verification.
func TestVerifyStructureRejectsMisplacedReplace(t *testing.T) {
	input := `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>T</title></head><body><div>head<p id="t">T</p>tail</div></body></html>`
	doc, err := ParseStructureDocument([]byte(input), "EPUB/a.xhtml", xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
	if err != nil {
		t.Fatal(err)
	}
	e, err := doc.ElementReplaceEdit(ElementReplace{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/div[1]/p[1]", Fragment: `<blockquote id="q">Q</blockquote>`})
	if err != nil {
		t.Fatal(err)
	}
	edits := []*StructureEdit{e}
	good := ApplyEdits([]byte(input), edits)
	if err := VerifyStructure(doc, edits, good); err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(good), `head<blockquote id="q">Q</blockquote>tail`, `headtail<blockquote id="q">Q</blockquote>`, 1)
	if bad == string(good) {
		t.Fatal("bad replace fixture did not change")
	}
	if err := VerifyStructure(doc, edits, []byte(bad)); err == nil {
		t.Fatal("accepted a replacement block outside its character-data context")
	}
}

// TestAttributeInsertionSeparators covers the supported positive cases that must
// not corrupt the start tag: empty values, apostrophes in single-quoted values,
// a first operations-namespace declaration, a reused prefix and a taken prefix.
func TestAttributeInsertionSeparators(t *testing.T) {
	profile := xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"}
	for _, tc := range []struct {
		name, input, locator, namespace, attr, old, value, want string
	}{
		{"empty-value", `<html xmlns="http://www.w3.org/1999/xhtml"><body><p title="">x</p></body></html>`, "/html[1]/body[1]/p[1]", "", "title", "", "new", `<p title="new">`},
		{"single-quote", `<html xmlns="http://www.w3.org/1999/xhtml"><body><p title='old'>x</p></body></html>`, "/html[1]/body[1]/p[1]", "", "title", "old", "reader's note", `<p title='reader&#39;s note'>`},
		{"double-quote", `<html xmlns="http://www.w3.org/1999/xhtml"><body><p title="old">x</p></body></html>`, "/html[1]/body[1]/p[1]", "", "title", "old", `say "hi"`, `<p title="say &quot;hi&quot;">`},
		{"new-namespace", `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>x</p></body></html>`, "/html[1]/body[1]/p[1]", OpsNamespace, "type", "", "footnote", `<p xmlns:epub="http://www.idpf.org/2007/ops" epub:type="footnote">`},
		{"reused-prefix", `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:ops="http://www.idpf.org/2007/ops"><body><p>x</p></body></html>`, "/html[1]/body[1]/p[1]", OpsNamespace, "type", "", "footnote", `<p ops:type="footnote">`},
		{"taken-prefix", `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://example.invalid/other"><body><p>x</p></body></html>`, "/html[1]/body[1]/p[1]", OpsNamespace, "type", "", "footnote", `<p xmlns:epub2="http://www.idpf.org/2007/ops" epub2:type="footnote">`},
		{"xml-namespace", `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>x</p></body></html>`, "/html[1]/body[1]/p[1]", XMLNamespace, "lang", "", "zh", `<p xml:lang="zh">`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := ParseStructureDocument([]byte(tc.input), "EPUB/a.xhtml", profile)
			if err != nil {
				t.Fatal(err)
			}
			op := AttributeSet{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: tc.locator, Namespace: tc.namespace, Name: tc.attr, Value: tc.value}
			if tc.old != "" || tc.name == "empty-value" {
				op.ExpectedOldValue = &tc.old
			}
			edit, err := doc.AttributeSetEdit(op)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			out := ApplyEdits([]byte(tc.input), []*StructureEdit{edit})
			if err := VerifyStructure(doc, []*StructureEdit{edit}, out); err != nil {
				t.Fatalf("verify: %v", err)
			}
			if !strings.Contains(string(out), tc.want) {
				t.Fatalf("spliced tag %q missing %q", out, tc.want)
			}
		})
	}
	// A fragment identity must be a legal XML name.
	doc, err := ParseStructureDocument([]byte(structureInput), "EPUB/a.xhtml", profile)
	if err != nil {
		t.Fatal(err)
	}
	scope := doc.Doc.Root.NamespaceScope()
	for _, fragment := range []string{`<p id="bad id">x</p>`, `<p id="">x</p>`, `<p xml:id="1bad">x</p>`} {
		if _, err := ParseFragment(fragment, scope, doc); err == nil {
			t.Fatalf("accepted fragment with invalid id: %s", fragment)
		}
	}
}

// TestFragmentDynamicContentRefused keeps the batch from writing dynamic
// content or URL semantics it cannot maintain: scriptable inline documents,
// nested browsing contexts, plugin content and unsupported URL attributes.
func TestFragmentDynamicContentRefused(t *testing.T) {
	doc := structureFixture(t)
	scope := doc.Doc.Root.NamespaceScope()
	for _, fragment := range []string{
		`<iframe srcdoc="&lt;script>bad()&lt;/script>"></iframe>`,
		`<iframe src="https://example.invalid/frame"></iframe>`,
		`<object data="https://example.invalid/x"></object>`,
		`<embed src="https://example.invalid/x"/>`,
		`<applet code="x"></applet>`,
		`<form action="https://example.invalid/post"></form>`,
		`<meta http-equiv="refresh" content="0;url=https://example.invalid/"/>`,
		`<link rel="stylesheet" href="https://example.invalid/x.css"/>`,
		`<p srcdoc="x">y</p>`,
		`<p data="https://example.invalid/x">y</p>`,
		`<p action="https://example.invalid/post">y</p>`,
		`<p poster="https://example.invalid/x.png">y</p>`,
		`<p ping="https://example.invalid/p">y</p>`,
		`<p cite="https://example.invalid/c">y</p>`,
		`<p usemap="#m">y</p>`,
		`<p itemid="https://example.invalid/i">y</p>`,
		`<p classid="https://example.invalid/c">y</p>`,
	} {
		if _, err := ParseFragment(fragment, scope, doc); err == nil {
			t.Fatalf("accepted dynamic content: %s", fragment)
		}
	}
	// Static, gated references stay writable.
	if _, err := ParseFragment(`<p><a href="https://example.invalid/ok">ok</a></p>`, scope, doc); err != nil {
		t.Fatalf("refused a gated static reference: %v", err)
	}
}

// fuzzStructureInput is the fixed document the structural fuzz edits. It carries
// mixed content, a single-quoted attribute, simple text and an unreferenced id.
const fuzzStructureInput = `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>T</title></head><body><h1 id="start">One</h1><p id="mixed">A <em>em</em> B</p><p title='old'>Plain.</p><p id="last">Last.</p></body></html>`

// FuzzStructureEditRoundTrip drives valid structural edits over UTF-8 and
// BOM-marked UTF-16 documents. Any accepted edit must be deterministic, must
// verify against the independent tree simulation and must keep the resource's
// encoding intact, so a model or encoding inconsistency fails the fuzz instead
// of only producing a rejection.
func FuzzStructureEditRoundTrip(f *testing.F) {
	f.Add("utf8", "insert", "before", "<p>x</p>")
	f.Add("utf8", "insert", "last-child", `<p>reader's "note"</p>`)
	f.Add("utf16le", "insert", "first-child", "<em>e</em>")
	f.Add("utf16be", "attribute-new", "after", "reader's note")
	f.Add("utf8", "attribute-old", "before", `say "hi" & <ok>`)
	f.Add("utf16le", "replace", "", "<p>r</p>")
	f.Add("utf8", "text", "", "updated")
	f.Add("utf16be", "move", "after", "")
	f.Add("utf8", "delete", "", "")
	f.Fuzz(func(t *testing.T, enc, kind, position, payload string) {
		if len(payload) > 2048 || len(position) > 32 {
			return
		}
		input, err := encodeFuzzDocument(fuzzStructureInput, enc)
		if err != nil {
			return
		}
		doc, err := ParseStructureDocument(input, "EPUB/a.xhtml", xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
		if err != nil {
			t.Skip()
		}
		base := AttributeSet{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1}
		var edit *StructureEdit
		switch kind {
		case "insert":
			base.Locator = "/html[1]/body[1]/p[1]"
			edit, err = doc.ElementInsertEdit(ElementInsert{BookPath: base.BookPath, RevisionID: base.RevisionID, ResourceSHA256: base.ResourceSHA256, LocatorVersion: 1, Locator: base.Locator, Position: position, Fragment: payload})
		case "replace":
			base.Locator = "/html[1]/body[1]/p[2]"
			edit, err = doc.ElementReplaceEdit(ElementReplace{BookPath: base.BookPath, RevisionID: base.RevisionID, ResourceSHA256: base.ResourceSHA256, LocatorVersion: 1, Locator: base.Locator, Fragment: payload})
		case "attribute-new":
			base.Locator = "/html[1]/body[1]/p[2]"
			base.Name, base.Value = "lang", payload
			edit, err = doc.AttributeSetEdit(base)
		case "attribute-old":
			base.Locator = "/html[1]/body[1]/p[2]"
			old := "old"
			base.Name, base.Value, base.ExpectedOldValue = "title", payload, &old
			edit, err = doc.AttributeSetEdit(base)
		case "text":
			base.Locator = "/html[1]/body[1]/p[3]"
			edit, err = doc.TextSetEdit(TextSet{BookPath: base.BookPath, RevisionID: base.RevisionID, ResourceSHA256: base.ResourceSHA256, LocatorVersion: 1, Locator: base.Locator, ExpectedOldValue: "Last.", NewValue: payload}, payload)
		case "move":
			base.Locator = "/html[1]/body[1]/p[3]"
			edit, err = doc.ElementMoveEdit(ElementMove{BookPath: base.BookPath, RevisionID: base.RevisionID, ResourceSHA256: base.ResourceSHA256, LocatorVersion: 1, Locator: base.Locator, Anchor: "/html[1]/body[1]/h1[1]", Position: position})
		case "delete":
			base.Locator = "/html[1]/body[1]/p[3]"
			edit, err = doc.ElementDeleteEdit(ElementDelete{BookPath: base.BookPath, RevisionID: base.RevisionID, ResourceSHA256: base.ResourceSHA256, LocatorVersion: 1, Locator: base.Locator})
		default:
			return
		}
		if err != nil {
			return // refusals are expected for invalid positions, values and fragments
		}
		edit.OpIndex = 0
		edits := []*StructureEdit{edit}
		if err := ValidateEdits(edits); err != nil {
			t.Fatalf("%s %q %q: single edit refused: %v", kind, position, payload, err)
		}
		out := ApplyEdits(input, edits)
		if again := ApplyEdits(input, edits); !bytes.Equal(out, again) {
			t.Fatalf("%s: nondeterministic splice", kind)
		}
		if err := VerifyStructure(doc, edits, out); err != nil {
			t.Fatalf("%s %q %q: model inconsistency: %v", kind, position, payload, err)
		}
		if _, err := xmltext.Parse(out); err != nil {
			t.Fatalf("%s: spliced output is not well formed: %v", kind, err)
		}
		switch enc {
		case "utf16le", "utf16be":
			if len(out)%2 != 0 || !bytes.Equal(out[:2], input[:2]) {
				t.Fatalf("%s: UTF-16 boundary broken (%d bytes)", kind, len(out))
			}
		}
	})
}

// encodeFuzzDocument renders the fixed document in one supported encoding.
func encodeFuzzDocument(text, enc string) ([]byte, error) {
	switch enc {
	case "utf8":
		return []byte(text), nil
	case "utf16le", "utf16be":
		units := utf16.Encode([]rune(text))
		out := make([]byte, 0, 2+len(units)*2)
		if enc == "utf16le" {
			out = append(out, 0xFF, 0xFE)
		} else {
			out = append(out, 0xFE, 0xFF)
		}
		for _, u := range units {
			if enc == "utf16le" {
				out = binary.LittleEndian.AppendUint16(out, u)
			} else {
				out = binary.BigEndian.AppendUint16(out, u)
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported encoding %q", enc)
	}
}

// TestVerifyStructureRejectsMisplacedReplaceWithContext keeps replacement blocks
// at their planned offset: neither an insertion into the surrounding gap nor a
// second identical context elsewhere in the document may hide a misplaced block.
func TestVerifyStructureRejectsMisplacedReplaceWithContext(t *testing.T) {
	profile := xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"}
	for _, mode := range []string{"gap-insertion", "duplicate-context"} {
		t.Run(mode, func(t *testing.T) {
			body := `<div>head<p>T</p>tail<em>E</em></div>`
			if mode == "duplicate-context" {
				body = `<div>head<p>T</p>tail</div><div>head<blockquote>Q</blockquote>tail</div>`
			}
			input := `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>T</title></head><body>` + body + `</body></html>`
			doc, err := ParseStructureDocument([]byte(input), "EPUB/a.xhtml", profile)
			if err != nil {
				t.Fatal(err)
			}
			r, err := doc.ElementReplaceEdit(ElementReplace{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/div[1]/p[1]", Fragment: `<blockquote>Q</blockquote>`})
			if err != nil {
				t.Fatal(err)
			}
			edits := []*StructureEdit{r}
			if mode == "gap-insertion" {
				i, err := doc.ElementInsertEdit(ElementInsert{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/div[1]/em[1]", Position: "before", Fragment: `<strong>N</strong>`})
				if err != nil {
					t.Fatal(err)
				}
				i.OpIndex = 1
				edits = append(edits, i)
			}
			if err := ValidateEdits(edits); err != nil {
				t.Fatal(err)
			}
			good := ApplyEdits([]byte(input), edits)
			if err := VerifyStructure(doc, edits, good); err != nil {
				t.Fatalf("positive control: %v", err)
			}
			bad := strings.Replace(string(good), `head<blockquote>Q</blockquote>tail`, `headtail<blockquote>Q</blockquote>`, 1)
			if bad == string(good) {
				t.Fatal("fixture unchanged")
			}
			if err := VerifyStructure(doc, edits, []byte(bad)); err == nil {
				t.Fatalf("accepted a replacement after trailing text: %s", bad)
			}
		})
	}
	// Adjacent insertions around a replaced element stay legal and exact.
	input := `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>T</title></head><body><div>head<p id="t">T</p>tail</div></body></html>`
	doc, err := ParseStructureDocument([]byte(input), "EPUB/a.xhtml", profile)
	if err != nil {
		t.Fatal(err)
	}
	r, err := doc.ElementReplaceEdit(ElementReplace{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/div[1]/p[1]", Fragment: `<blockquote>Q</blockquote>`})
	if err != nil {
		t.Fatal(err)
	}
	before, err := doc.ElementInsertEdit(ElementInsert{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/div[1]/p[1]", Position: "before", Fragment: `<strong>B</strong>`})
	if err != nil {
		t.Fatal(err)
	}
	after, err := doc.ElementInsertEdit(ElementInsert{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/div[1]/p[1]", Position: "after", Fragment: `<strong>A</strong>`})
	if err != nil {
		t.Fatal(err)
	}
	// "before"/"after" the replaced element share its boundaries, which the
	// frozen edit model refuses rather than ordering silently.
	if err := ValidateEdits([]*StructureEdit{r, before, after}); err == nil {
		t.Fatal("boundary insertions around a replaced element were accepted")
	}
	// Adjacent insertions at distinct offsets around an untouched element pass.
	other, err := doc.ElementInsertEdit(ElementInsert{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/div[1]/p[1]", Position: "first-child", Fragment: `<strong>F</strong>`})
	if err != nil {
		t.Fatal(err)
	}
	first, err := doc.ElementInsertEdit(ElementInsert{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/div[1]/p[1]", Position: "last-child", Fragment: `<strong>L</strong>`})
	if err != nil {
		t.Fatal(err)
	}
	other.OpIndex, first.OpIndex = 0, 1
	adjacent := []*StructureEdit{other, first}
	if err := ValidateEdits(adjacent); err != nil {
		t.Fatal(err)
	}
	out := ApplyEdits([]byte(input), adjacent)
	if err := VerifyStructure(doc, adjacent, out); err != nil {
		t.Fatalf("adjacent insertions: %v", err)
	}
	if !strings.Contains(string(out), `<p id="t"><strong>F</strong>T<strong>L</strong></p>`) {
		t.Fatalf("adjacent insertions output: %s", out)
	}
}

// TestAttributePositiveByteControlsAcrossEncodings keeps the byte-exact positive
// controls for every supported encoding: an empty single-quoted value and a new
// value with quotes, an apostrophe, CJK and a surrogate pair, plus an operations
// namespace prefix that is already taken.
func TestAttributePositiveByteControlsAcrossEncodings(t *testing.T) {
	profile := xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"}
	base := `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>T</title></head><body><p title=''>x</p></body></html>`
	for _, enc := range []string{"utf8", "utf16le", "utf16be"} {
		t.Run("empty-single-quoted-"+enc, func(t *testing.T) {
			input, err := encodeFuzzDocument(base, enc)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := ParseStructureDocument(input, "EPUB/a.xhtml", profile)
			if err != nil {
				t.Fatal(err)
			}
			old := ""
			edit, err := doc.AttributeSetEdit(AttributeSet{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/p[1]", Name: "title", ExpectedOldValue: &old, Value: `reader's "note" 中文 😀`})
			if err != nil {
				t.Fatal(err)
			}
			out := ApplyEdits(input, []*StructureEdit{edit})
			if err := VerifyStructure(doc, []*StructureEdit{edit}, out); err != nil {
				t.Fatal(err)
			}
			decoded, err := decodeFuzzDocument(out, enc)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(decoded, `title='reader&#39;s &quot;note&quot; 中文 😀'`) {
				t.Fatalf("candidate: %q", decoded)
			}
		})
		t.Run("taken-prefix-"+enc, func(t *testing.T) {
			input, err := encodeFuzzDocument(strings.Replace(base, `<html xmlns="http://www.w3.org/1999/xhtml"`, `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://example.invalid/other"`, 1), enc)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := ParseStructureDocument(input, "EPUB/a.xhtml", profile)
			if err != nil {
				t.Fatal(err)
			}
			edit, err := doc.AttributeSetEdit(AttributeSet{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/p[1]", Namespace: OpsNamespace, Name: "type", Value: "footnote"})
			if err != nil {
				t.Fatal(err)
			}
			out := ApplyEdits(input, []*StructureEdit{edit})
			if err := VerifyStructure(doc, []*StructureEdit{edit}, out); err != nil {
				t.Fatal(err)
			}
			decoded, err := decodeFuzzDocument(out, enc)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(decoded, `<p title='' xmlns:epub2="http://www.idpf.org/2007/ops" epub2:type="footnote">`) {
				t.Fatalf("candidate: %q", decoded)
			}
		})
	}
}

// decodeFuzzDocument reverses encodeFuzzDocument so assertions read as text.
func decodeFuzzDocument(data []byte, enc string) (string, error) {
	if enc == "utf8" {
		return string(data), nil
	}
	if len(data) < 2 || len(data)%2 != 0 {
		return "", fmt.Errorf("not UTF-16")
	}
	units := make([]uint16, 0, len(data)/2)
	for i := 2; i+1 < len(data); i += 2 {
		if enc == "utf16le" {
			units = append(units, binary.LittleEndian.Uint16(data[i:]))
		} else {
			units = append(units, binary.BigEndian.Uint16(data[i:]))
		}
	}
	return string(utf16.Decode(units)), nil
}
