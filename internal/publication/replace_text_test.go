package publication

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/xmltext"
)

func replaceDoc(t *testing.T, input string) *StructureDocument {
	t.Helper()
	doc, err := ParseStructureDocument([]byte(input), "EPUB/a.xhtml", xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func applyReplace(t *testing.T, doc *StructureDocument, op TextReplace) ([]byte, ReplaceFacts) {
	t.Helper()
	edits, facts, err := doc.ReplaceTextEdits(op)
	if err != nil {
		t.Fatalf("replace refused: %v", err)
	}
	out := ApplyEdits(doc.Input, edits)
	if err := VerifyStructure(doc, edits, out); err != nil {
		t.Fatalf("verification: %v", err)
	}
	return out, facts
}

func replaceRefused(t *testing.T, doc *StructureDocument, op TextReplace) error {
	t.Helper()
	_, _, err := doc.ReplaceTextEdits(op)
	if err == nil {
		t.Fatal("replace accepted")
	}
	return err
}

func literalReplace(locator, pattern, replacement string, hits int) TextReplace {
	return TextReplace{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: locator, Mode: "literal", Pattern: pattern, Replacement: replacement, ExpectedHits: hits}
}

// TestReplaceLiteralAndMixedContent keeps the batch replace on literal runs:
// every occurrence in one element's own text is replaced, matches never cross a
// child element or an entity boundary, and untouched bytes stay.
func TestReplaceLiteralAndMixedContent(t *testing.T) {
	input := `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>colour and colour</p><p>foo<em>x</em>bar</p></body></html>`
	doc := replaceDoc(t, input)
	out, facts := applyReplace(t, doc, literalReplace("/html[1]/body[1]/p[1]", "colour", "color", 2))
	if !bytes.Contains(out, []byte(`<p>color and color</p>`)) {
		t.Fatalf("literal output: %s", out)
	}
	if facts.Hits != 2 || len(facts.Nodes) != 1 || facts.Nodes[0].Before != "colour and colour" || facts.Nodes[0].After != "color and color" {
		t.Fatalf("literal facts: %+v", facts)
	}
	out, facts = applyReplace(t, doc, literalReplace("/html[1]/body[1]/p[2]", "o", "0", 2))
	if !bytes.Contains(out, []byte(`<p>f00<em>x</em>bar</p>`)) {
		t.Fatalf("mixed output: %s", out)
	}
	if facts.Hits != 2 || len(facts.Nodes) != 1 || facts.Nodes[0].After != "f00bar" {
		t.Fatalf("mixed facts: %+v", facts)
	}
}

func TestReplaceRefusesCrossRunAndUnwritable(t *testing.T) {
	mixed := replaceDoc(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>foo<em>x</em>bar</p></body></html>`)
	if err := replaceRefused(t, mixed, literalReplace("/html[1]/body[1]/p[1]", "oob", "X", 1)); !strings.Contains(err.Error(), "non-writable") {
		t.Fatalf("cross-run error: %v", err)
	}
	entity := replaceDoc(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>a&amp;b</p></body></html>`)
	if err := replaceRefused(t, entity, literalReplace("/html[1]/body[1]/p[1]", "a&b", "X", 1)); err == nil {
		t.Fatal("entity match accepted")
	}
	cdata := replaceDoc(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><p><![CDATA[cdata]]></p></body></html>`)
	if err := replaceRefused(t, cdata, literalReplace("/html[1]/body[1]/p[1]", "cdata", "X", 1)); err == nil {
		t.Fatal("cdata match accepted")
	}
	if err := replaceRefused(t, cdata, literalReplace("/html[1]/body[1]/p[1]", "cdata", "X", 0)); err == nil {
		t.Fatal("cdata match accepted with zero expectation")
	}
}

func TestReplaceHitCountAndEmptyMatch(t *testing.T) {
	doc := replaceDoc(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>ab ab</p></body></html>`)
	if err := replaceRefused(t, doc, literalReplace("/html[1]/body[1]/p[1]", "ab", "x", 1)); !strings.Contains(err.Error(), "expected 1 hits, found 2") {
		t.Fatalf("hit count error: %v", err)
	}
	if err := replaceRefused(t, doc, literalReplace("/html[1]/body[1]/p[1]", "", "x", 0)); err == nil {
		t.Fatal("empty literal accepted")
	}
	empty := literalReplace("/html[1]/body[1]/p[1]", "a*", "x", 0)
	empty.Mode = "regex"
	if err := replaceRefused(t, doc, empty); !strings.Contains(err.Error(), "empty string") {
		t.Fatalf("empty regex error: %v", err)
	}
	noop := literalReplace("/html[1]/body[1]/p[1]", "zz", "x", 0)
	out, facts := applyReplace(t, doc, noop)
	if !bytes.Equal(out, []byte(doc.Input)) || facts.Hits != 0 || len(facts.Nodes) != 0 {
		t.Fatalf("no-op output changed: %s %+v", out, facts)
	}
}

func TestReplaceRegexCaptureExpansion(t *testing.T) {
	doc := replaceDoc(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>first.last and one.two</p></body></html>`)
	op := TextReplace{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: "/html[1]/body[1]/p[1]", Mode: "regex", Pattern: `([a-z]+)\.([a-z]+)`, Replacement: "$2 $1", ExpectedHits: 2}
	out, facts := applyReplace(t, doc, op)
	if !bytes.Contains(out, []byte(`<p>last first and two one</p>`)) {
		t.Fatalf("regex output: %s", out)
	}
	if facts.Hits != 2 || facts.Nodes[0].After != "last first and two one" {
		t.Fatalf("regex facts: %+v", facts)
	}
}

func TestReplaceScopeBoundaries(t *testing.T) {
	doc := replaceDoc(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="a">text<svg xmlns="http://www.w3.org/2000/svg"><text>svgtext</text></svg></p></body></html>`)
	out, facts := applyReplace(t, doc, literalReplace("/html[1]/body[1]/p[1]", "text", "T", 1))
	if !bytes.Contains(out, []byte(`>T<svg`)) || !bytes.Contains(out, []byte(`<text>svgtext</text>`)) {
		t.Fatalf("foreign scope output: %s", out)
	}
	if facts.Skipped != 1 {
		t.Fatalf("skipped facts: %+v", facts)
	}
	if err := replaceRefused(t, doc, literalReplace("/html[1]/body[1]/p[1]", "svgtext", "X", 1)); err == nil {
		t.Fatal("foreign text replaced")
	}
	if err := replaceRefused(t, doc, literalReplace("/html[1]/head[1]", "t", "x", 0)); err == nil {
		t.Fatal("head target accepted")
	}
	if err := replaceRefused(t, doc, literalReplace("/html[1]", "t", "x", 0)); err == nil {
		t.Fatal("html target accepted")
	}
}

// TestReplaceCRLFAndEncodingFidelity keeps the physical encoding: a CRLF pair
// is one decoded LF and a match beside it replaces only the matched bytes.
func TestReplaceCRLFAndEncodingFidelity(t *testing.T) {
	input := "<html xmlns=\"http://www.w3.org/1999/xhtml\"><body><p>one\r\ntwo</p></body></html>"
	doc := replaceDoc(t, input)
	out, _ := applyReplace(t, doc, literalReplace("/html[1]/body[1]/p[1]", "two", "TWO", 1))
	if !bytes.Contains(out, []byte("one\r\nTWO")) {
		t.Fatalf("crlf output: %q", out)
	}
	utf16 := []byte{0xff, 0xfe}
	for _, r := range "<html xmlns=\"http://www.w3.org/1999/xhtml\"><body><p>colour</p></body></html>" {
		if r <= 0xffff {
			utf16 = append(utf16, byte(r), byte(r>>8))
		}
	}
	doc16, err := ParseStructureDocument(utf16, "EPUB/b.xhtml", xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
	if err != nil {
		t.Fatal(err)
	}
	op := literalReplace("/html[1]/body[1]/p[1]", "colour", "color", 1)
	out16, facts := applyReplace(t, doc16, op)
	if facts.Hits != 1 {
		t.Fatalf("utf16 facts: %+v", facts)
	}
	check, err := xmltext.Parse(out16)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range check.Elements {
		if e.Name.Local == "p" && e.DirectText != "color" {
			t.Fatalf("utf16 text: %q", e.DirectText)
		}
	}
	if out16[0] != 0xff || out16[1] != 0xfe {
		t.Fatalf("utf16 BOM lost: %x", out16[:2])
	}
}

// FuzzReplaceTextEdits compares the batch replace with an independent standard
// library oracle whenever the engine accepts: the candidate direct text must
// equal the stdlib replacement, untouched elements must keep their text, and
// the spliced bytes must still verify.
func FuzzReplaceTextEdits(f *testing.F) {
	f.Add(uint8(0), "colour", "color", uint8(0))
	f.Add(uint8(1), `([a-z]+)`, "$1!", uint8(1))
	f.Add(uint8(0), "zz", "x", uint8(2))
	f.Add(uint8(0), "o", "", uint8(4))
	f.Fuzz(func(t *testing.T, mode uint8, pattern, replacement string, target uint8) {
		if len(pattern) > 64 || len(replacement) > 64 || !utf8.ValidString(pattern) || !utf8.ValidString(replacement) || pattern == "" {
			return
		}
		input := []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="a">alpha beta gamma</p><p id="b">one two</p><p id="c">x<em>mid</em>y</p><p id="d">a&amp;b</p><p id="e">crlf` + "\r\n" + `tail</p></body></html>`)
		doc, err := ParseStructureDocument(input, "EPUB/a.xhtml", xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
		if err != nil {
			t.Fatal(err)
		}
		locators := []string{"/html[1]/body[1]/p[1]", "/html[1]/body[1]/p[2]", "/html[1]/body[1]/p[3]", "/html[1]/body[1]/p[4]", "/html[1]/body[1]/p[5]"}
		modeName := "literal"
		if mode%2 == 1 {
			modeName = "regex"
		}
		op := TextReplace{BookPath: "EPUB/a.xhtml", RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: locators[int(target)%len(locators)], Mode: modeName, Pattern: pattern, Replacement: replacement}
		frozen, err := doc.Locate(op.Locator)
		if err != nil {
			t.Fatal(err)
		}
		text := frozen.DirectText
		want := ""
		hits := 0
		if modeName == "literal" {
			hits = strings.Count(text, pattern)
			want = strings.ReplaceAll(text, pattern, replacement)
		} else {
			re, err := regexp.Compile(pattern)
			if err != nil || re.MatchString("") {
				return
			}
			hits = len(re.FindAllStringIndex(text, -1))
			want = re.ReplaceAllString(text, replacement)
		}
		op.ExpectedHits = hits
		edits, facts, err := doc.ReplaceTextEdits(op)
		if err != nil {
			return
		}
		out := ApplyEdits(input, edits)
		if err := VerifyStructure(doc, edits, out); err != nil {
			t.Fatalf("verification: %v", err)
		}
		check, err := xmltext.Parse(out)
		if err != nil {
			t.Fatal(err)
		}
		byLoc := map[string]*xmltext.Element{}
		for _, e := range check.Elements {
			byLoc[e.Location] = e
		}
		got := byLoc[op.Locator]
		if got == nil || got.DirectText != want {
			text := ""
			if got != nil {
				text = got.DirectText
			}
			t.Fatalf("candidate direct text %q, oracle %q (facts %+v)", text, want, facts)
		}
		for _, e := range doc.Doc.Elements {
			if e.Location == op.Locator {
				continue
			}
			other := byLoc[e.Location]
			if other == nil || other.DirectText != e.DirectText {
				t.Fatalf("untouched element %s changed: %+v", e.Location, other)
			}
		}
	})
}
