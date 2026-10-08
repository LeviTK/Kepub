package publication

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"
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
	edits, facts, err := doc.ReplaceTextEdits(op, NewReplaceBudget())
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
	_, _, err := doc.ReplaceTextEdits(op, NewReplaceBudget())
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

// replaceEncode returns a fixture in one of the three physical encodings the
// product accepts, so the byte-level oracle can compare each encoding.
func replaceEncode(text, enc string) []byte {
	if enc == "utf8" {
		return []byte(text)
	}
	out := []byte{0xff, 0xfe}
	if enc == "utf16be" {
		out = []byte{0xfe, 0xff}
	}
	put := func(u rune) {
		if enc == "utf16le" {
			out = append(out, byte(u), byte(u>>8))
		} else {
			out = append(out, byte(u>>8), byte(u))
		}
	}
	for _, r := range text {
		if r <= 0xffff {
			put(r)
			continue
		}
		hi, lo := utf16.EncodeRune(r)
		put(hi)
		put(lo)
	}
	return out
}

// TestReplaceGeneratedDecodedNeighbors keeps the second rejected boundary: a
// generated entity whose replacement text spells a predefined reference (or
// nests one) decodes to a different string than its own source spelling, and a
// long numeric reference is still one reference. The generated piece stays
// non-writable, but its decoded fragment must align exactly so the authored
// literals beside it keep their byte intervals.
func TestReplaceGeneratedDecodedNeighbors(t *testing.T) {
	cases := []struct{ name, dtd, text string }{
		{"predefined", `<!ENTITY word "&amp;">`, "left&word;right"},
		{"nested", `<!ENTITY inner "&amp;"><!ENTITY word "&inner;">`, "left&word;right"},
		{"long-numeric", "", "left&#" + strings.Repeat("0", 61) + "65;right"},
	}
	for _, enc := range []string{"utf8", "utf16le", "utf16be"} {
		for _, c := range cases {
			for _, pattern := range []string{"left", "right"} {
				t.Run(c.name+"/"+enc+"/"+pattern, func(t *testing.T) {
					input := `<!DOCTYPE html [` + c.dtd + `]><html xmlns="http://www.w3.org/1999/xhtml"><body><p>` + c.text + `</p></body></html>`
					doc, err := ParseStructureDocument(replaceEncode(input, enc), "EPUB/a.xhtml", xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
					if err != nil {
						t.Fatal(err)
					}
					edits, facts, err := doc.ReplaceTextEdits(literalReplace("/html[1]/body[1]/p[1]", pattern, "CHANGED", 1), NewReplaceBudget())
					if err != nil {
						t.Fatalf("legal literal neighbor refused: %v", err)
					}
					if facts.Hits != 1 {
						t.Fatalf("facts: %+v", facts)
					}
					out := ApplyEdits(doc.Input, edits)
					want := strings.Replace(input, c.text, strings.Replace(c.text, pattern, "CHANGED", 1), 1)
					if !bytes.Equal(out, replaceEncode(want, enc)) {
						t.Fatalf("bytes: got=%q want=%q", out, replaceEncode(want, enc))
					}
					if err := VerifyStructure(doc, edits, out); err != nil {
						t.Fatalf("verification: %v", err)
					}
				})
			}
		}
	}
	// The generated decoded text itself is never writable source.
	doc := replaceDoc(t, `<!DOCTYPE html [<!ENTITY word "&amp;">]><html xmlns="http://www.w3.org/1999/xhtml"><body><p>left&word;right</p></body></html>`)
	replaceRefused(t, doc, literalReplace("/html[1]/body[1]/p[1]", "&", "X", 1))
	replaceRefused(t, doc, literalReplace("/html[1]/body[1]/p[1]", "left&right", "X", 1))
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
	f.Add(uint8(0), "left", "L", uint8(5))
	f.Add(uint8(0), "right", "R", uint8(5))
	f.Add(uint8(0), "&", "amp", uint8(5))
	f.Add(uint8(0), "right", "R", uint8(6))
	f.Fuzz(func(t *testing.T, mode uint8, pattern, replacement string, target uint8) {
		if len(pattern) > 64 || len(replacement) > 64 || !utf8.ValidString(pattern) || !utf8.ValidString(replacement) || pattern == "" {
			return
		}
		input := []byte(`<!DOCTYPE html [<!ENTITY word "&amp;">]><html xmlns="http://www.w3.org/1999/xhtml"><body><p id="a">alpha beta gamma</p><p id="b">one two</p><p id="c">x<em>mid</em>y</p><p id="d">a&amp;b</p><p id="e">crlf` + "\r\n" + `tail</p><p id="f">left&word;right</p><p id="g">left&#` + strings.Repeat("0", 61) + `65;right</p></body></html>`)
		doc, err := ParseStructureDocument(input, "EPUB/a.xhtml", xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
		if err != nil {
			t.Fatal(err)
		}
		locators := []string{"/html[1]/body[1]/p[1]", "/html[1]/body[1]/p[2]", "/html[1]/body[1]/p[3]", "/html[1]/body[1]/p[4]", "/html[1]/body[1]/p[5]", "/html[1]/body[1]/p[6]", "/html[1]/body[1]/p[7]"}
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
		edits, facts, err := doc.ReplaceTextEdits(op, NewReplaceBudget())
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

// TestReplaceZeroWidthAndProvenance keeps the rejected boundaries: a real
// zero-width match refuses instead of being dropped, a locator cannot enter an
// excluded ancestor, literals beside entity references stay writable, and
// entity-generated or entity-reference spellings are never literal source.
func TestReplaceZeroWidthAndProvenance(t *testing.T) {
	t.Run("zero-width", func(t *testing.T) {
		doc := replaceDoc(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>alpha beta</p></body></html>`)
		for _, pattern := range []string{`\b`, `alpha|\b`, `\b|beta`} {
			op := literalReplace("/html[1]/body[1]/p[1]", pattern, "X", 0)
			op.Mode = "regex"
			if err := replaceRefused(t, doc, op); !strings.Contains(err.Error(), "zero-width") {
				t.Fatalf("%s error: %v", pattern, err)
			}
		}
		anchored := literalReplace("/html[1]/body[1]/p[1]", `^alpha beta$`, "X", 1)
		anchored.Mode = "regex"
		out, _ := applyReplace(t, doc, anchored)
		if !bytes.Contains(out, []byte(`<p>X</p>`)) {
			t.Fatalf("anchored positive: %s", out)
		}
	})
	t.Run("excluded-ancestor", func(t *testing.T) {
		for _, c := range []struct{ body, locator string }{
			{`<script><p>alpha</p></script>`, "/html[1]/body[1]/script[1]/p[1]"},
			{`<style><p>alpha</p></style>`, "/html[1]/body[1]/style[1]/p[1]"},
			{`<svg xmlns="http://www.w3.org/2000/svg"><foreignObject><p xmlns="http://www.w3.org/1999/xhtml">alpha</p></foreignObject></svg>`, "/html[1]/body[1]/svg[1]/foreignObject[1]/p[1]"},
		} {
			doc := replaceDoc(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body>`+c.body+`</body></html>`)
			replaceRefused(t, doc, literalReplace(c.locator, "alpha", "CHANGED", 1))
		}
		ok := replaceDoc(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><section><p>alpha</p></section></body></html>`)
		out, _ := applyReplace(t, ok, literalReplace("/html[1]/body[1]/section[1]/p[1]", "alpha", "CHANGED", 1))
		if !bytes.Contains(out, []byte(`<p>CHANGED</p>`)) {
			t.Fatalf("ordinary descendant: %s", out)
		}
	})
	t.Run("literals-beside-entities", func(t *testing.T) {
		for _, c := range []struct{ text, pattern, replacement, want string }{
			{`alpha &amp; beta`, "alpha", "ALPHA", `ALPHA &amp; beta`},
			{`alpha &amp; beta`, "beta", "BETA", `alpha &amp; BETA`},
			{`alpha &#x1F600; beta`, "beta", "文😀", `alpha &#x1F600; 文😀`},
		} {
			input := `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>` + c.text + `</p></body></html>`
			doc := replaceDoc(t, input)
			out, facts := applyReplace(t, doc, literalReplace("/html[1]/body[1]/p[1]", c.pattern, c.replacement, 1))
			want := strings.Replace(input, c.text, c.want, 1)
			if facts.Hits != 1 || string(out) != want {
				t.Fatalf("entity neighbor %q: %+v %s", c.pattern, facts, out)
			}
		}
		for _, pattern := range []string{"&", "a&b"} {
			doc := replaceDoc(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>a&amp;b</p></body></html>`)
			replaceRefused(t, doc, literalReplace("/html[1]/body[1]/p[1]", pattern, "X", 1))
		}
	})
	t.Run("internal-entity-generated", func(t *testing.T) {
		input := `<!DOCTYPE html [<!ENTITY word "word">]><html xmlns="http://www.w3.org/1999/xhtml"><body><p>&word;</p></body></html>`
		doc := replaceDoc(t, input)
		replaceRefused(t, doc, literalReplace("/html[1]/body[1]/p[1]", "word", "changed", 1))
	})
	t.Run("zero-length-entity-boundary", func(t *testing.T) {
		input := `<!DOCTYPE html [<!ENTITY empty "">]><html xmlns="http://www.w3.org/1999/xhtml"><body><p>foo&empty;bar</p></body></html>`
		doc := replaceDoc(t, input)
		out, facts := applyReplace(t, doc, literalReplace("/html[1]/body[1]/p[1]", "bar", "X", 1))
		if facts.Hits != 1 || !strings.Contains(string(out), "foo&empty;X") {
			t.Fatalf("empty entity suffix: %+v %s", facts, out)
		}
		replaceRefused(t, doc, literalReplace("/html[1]/body[1]/p[1]", "foobar", "X", 1))
	})
}
