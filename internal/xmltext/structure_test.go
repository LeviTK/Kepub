package xmltext

import (
	"encoding/xml"
	"strings"
	"testing"
	"unicode/utf16"
)

func elementAt(t *testing.T, doc *Document, location string) *Element {
	t.Helper()
	for _, e := range doc.Elements {
		if e.Location == location {
			return e
		}
	}
	t.Fatalf("missing element %s", location)
	return nil
}

func TestPhysicalMarkupRanges(t *testing.T) {
	input := []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="a">x<em/>y</p><br/></body></html>`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	p := elementAt(t, doc, "/html[1]/body[1]/p[1]")
	start, end, ok := p.PhysicalMarkup()
	if !ok || string(input[start:end]) != `<p id="a">x<em/>y</p>` {
		t.Fatalf("p markup %d..%d ok=%t: %q", start, end, ok, input[start:end])
	}
	cs, ce, ok := p.PhysicalContent()
	if !ok || string(input[cs:ce]) != `x<em/>y` {
		t.Fatalf("p content %d..%d ok=%t: %q", cs, ce, ok, input[cs:ce])
	}
	em := elementAt(t, doc, "/html[1]/body[1]/p[1]/em[1]")
	if !em.SelfClosing {
		t.Fatal("em not self-closing")
	}
	if start, end, ok := em.PhysicalMarkup(); !ok || string(input[start:end]) != `<em/>` || start != end-5 {
		t.Fatalf("em markup %d..%d ok=%t", start, end, ok)
	}
	if _, _, ok := em.PhysicalContent(); ok {
		t.Fatal("empty-element tag exposed a content interval")
	}
	br := elementAt(t, doc, "/html[1]/body[1]/br[1]")
	if start, end, ok := br.PhysicalMarkup(); !ok || string(input[start:end]) != `<br/>` {
		t.Fatalf("br markup %d..%d ok=%t", start, end, ok)
	}
	if tagEnd, ok := br.TagEnd(); !ok || string(input[tagEnd:br.OpenEnd]) != "/>" {
		t.Fatalf("br tag end %d ok=%t", tagEnd, ok)
	}
	if tagEnd, ok := p.TagEnd(); !ok || string(input[tagEnd:p.OpenEnd]) != ">" {
		t.Fatalf("p tag end %d ok=%t", tagEnd, ok)
	}
}

func TestAttributeMarkup(t *testing.T) {
	input := []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="a" class='x'>y</p></body></html>`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	p := elementAt(t, doc, "/html[1]/body[1]/p[1]")
	id, ok := p.AttributeBytes(xml.Name{Local: "id"})
	if !ok || string(input[id.Start:id.End]) != ` id="a"` || string(input[id.ValueStart:id.ValueEnd]) != "a" {
		t.Fatalf("id markup %+v: %q", id, input[id.Start:id.End])
	}
	class, ok := p.AttributeBytes(xml.Name{Local: "class"})
	if !ok || string(input[class.Start:class.End]) != ` class='x'` || string(input[class.ValueStart:class.ValueEnd]) != "x" {
		t.Fatalf("class markup %+v", class)
	}
	if _, ok := p.AttributeBytes(xml.Name{Local: "missing"}); ok {
		t.Fatal("unknown attribute reported a source interval")
	}
}

func TestAttributeMarkupRejectsGeneratedValues(t *testing.T) {
	input := []byte(`<!DOCTYPE html [<!ENTITY v "expanded">]><html xmlns="http://www.w3.org/1999/xhtml"><body><p class="&v;" id="a">y</p></body></html>`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	p := elementAt(t, doc, "/html[1]/body[1]/p[1]")
	if _, ok := p.AttributeBytes(xml.Name{Local: "class"}); ok {
		t.Fatal("entity-expanded attribute value reported a writable interval")
	}
	if id, ok := p.AttributeBytes(xml.Name{Local: "id"}); !ok || string(input[id.ValueStart:id.ValueEnd]) != "a" {
		t.Fatal("literal attribute lost its source interval")
	}
	if start, end, ok := p.PhysicalMarkup(); !ok || string(input[start:end]) != `<p class="&v;" id="a">y</p>` {
		t.Fatalf("tag with an entity value lost its loose markup range: %d..%d %t", start, end, ok)
	}
}

func TestPhysicalMarkupRangesUTF16(t *testing.T) {
	text := `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>值</p></body></html>`
	units := utf16.Encode([]rune(text))
	input := []byte{0xff, 0xfe}
	for _, u := range units {
		input = append(input, byte(u), byte(u>>8))
	}
	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	p := elementAt(t, doc, "/html[1]/body[1]/p[1]")
	start, end, ok := p.PhysicalMarkup()
	if !ok {
		t.Fatal("UTF-16 markup range not physical")
	}
	want := utf16.Encode([]rune(`<p>值</p>`))
	got := make([]byte, 0, end-start)
	for i := start; i < end; i += 2 {
		got = append(got, input[i], input[i+1])
	}
	if len(got) != len(want)*2 {
		t.Fatalf("UTF-16 markup length %d", len(got))
	}
	for i, u := range want {
		if got[2*i] != byte(u) || got[2*i+1] != byte(u>>8) {
			t.Fatalf("UTF-16 markup byte %d", i)
		}
	}
}

func TestPhysicalMarkupRejectsGeneratedMarkup(t *testing.T) {
	input := []byte(`<!DOCTYPE html [<!ENTITY frag "<b>x</b>">]><html xmlns="http://www.w3.org/1999/xhtml"><body>&frag;</body></html>`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	body := elementAt(t, doc, "/html[1]/body[1]")
	if _, _, ok := body.PhysicalMarkup(); !ok {
		t.Fatal("literal body tags must stay physical")
	}
	b := elementAt(t, doc, "/html[1]/body[1]/b[1]")
	if _, _, ok := b.PhysicalMarkup(); ok {
		t.Fatal("entity-generated element reported a writable markup range")
	}
	if _, _, ok := b.PhysicalContent(); ok {
		t.Fatal("entity-generated element reported a writable content range")
	}
}

func TestNamespaceScope(t *testing.T) {
	input := []byte(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><p epub:type="note"><em>x</em></p></body></html>`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	em := elementAt(t, doc, "/html[1]/body[1]/p[1]/em[1]")
	scope := em.NamespaceScope()
	if scope[""] != "http://www.w3.org/1999/xhtml" || scope["epub"] != "http://www.idpf.org/2007/ops" || scope["xml"] == "" {
		t.Fatalf("scope %v", scope)
	}
	if _, ok := em.NamespaceScope()["missing"]; ok {
		t.Fatal("unexpected prefix")
	}
}

func TestValidNCName(t *testing.T) {
	for _, ok := range []string{"a", "p", "_x", "x-y.z", "数据", "aria-label"} {
		if !ValidNCName(ok) {
			t.Fatalf("rejected %q", ok)
		}
	}
	for _, bad := range []string{"", "a:b", "1a", "a b", "a<b", "a\u0000"} {
		if ValidNCName(bad) {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestReplaceBytesMatchesReplace(t *testing.T) {
	input := []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p>old</p></body></html>`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	p := elementAt(t, doc, "/html[1]/body[1]/p[1]")
	replacement, err := p.ReplaceBytes("新 < & >\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(replacement), "<") || strings.Contains(string(replacement), "& ") {
		t.Fatalf("replacement not escaped: %q", replacement)
	}
	out, changed, err := Replace(input, p, "old", "新 < & >\r\n")
	if err != nil || !changed {
		t.Fatal(err, changed)
	}
	if !strings.Contains(string(out), string(replacement)) {
		t.Fatal("Replace did not use the same encoded bytes")
	}
}
