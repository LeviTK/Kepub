package publication

import (
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

func textSet(raw, locator, old, new string) TextSet {
	return TextSet{contentPath, "initial", fmt.Sprintf("%x", sha256.Sum256([]byte(raw))), 1, locator, old, new}
}

func TestTextSetExactBytesAndReadLocator(t *testing.T) {
	prefix := "\xef\xbb\xbf<?xml version='1.0' encoding='UTF-8'?>\r\n" + `<h:html xmlns:h="http://www.w3.org/1999/xhtml"><h:head><h:title>A&amp;😀</h:title></h:head><h:body><h:p id='same'>A&amp;😀` + "\r\nZ" + `</h:p><!-- untouched --><h:div><h:p id='same' xml:lang='en'>`
	suffix := `</h:p></h:div><h:p>A&amp;😀` + "\r\nZ" + `</h:p></h:body></h:html>` + "\r\n"
	raw := prefix + "A&#38;&#x1F600;\r\nZ" + suffix
	a, p := contentFixture(t, raw)
	r, err := ReadContent(a, p, contentPath, ContentOptions{})
	if err != nil || len(r.Nodes) != 3 || r.Nodes[1].Locator != "/html[1]/body[1]/div[1]/p[1]" || r.Nodes[1].Text != "A&😀\nZ" {
		t.Fatalf("read binding: %+v %v", r, err)
	}
	s := textSet(raw, r.Nodes[1].Locator, r.Nodes[1].Text, "新 < & > \" '\r\n\t😀")
	out, changed, err := ApplyText(a, p, s)
	want := prefix + "新 &lt; &amp; &gt; &#34; &#39;&#xD;&#xA;&#x9;😀" + suffix
	if err != nil || !changed || !bytes.Equal(out, []byte(want)) {
		t.Fatalf("replacement %q %v %v", out, changed, err)
	}
	s.NewValue = s.ExpectedOldValue
	out, changed, err = ApplyText(a, p, s)
	if err != nil || changed || string(out) != raw {
		t.Fatalf("no-op: %v %v", changed, err)
	}
}

func TestTextSetSimpleSubsetAndAncestors(t *testing.T) {
	for _, tc := range []struct{ body, locator, text string }{
		{`<p>text<b>child</b>tail</p>`, "/html[1]/body[1]/p[1]", "textchildtail"},
		{`<p><!--x-->text</p>`, "/html[1]/body[1]/p[1]", "text"},
		{`<p><![CDATA[text]]></p>`, "/html[1]/body[1]/p[1]", "text"},
		{`<p><?x y?>text</p>`, "/html[1]/body[1]/p[1]", "text"},
		{`<p/>`, "/html[1]/body[1]/p[1]", ""},
		{`<script><p>text</p></script>`, "/html[1]/body[1]/script[1]/p[1]", "text"},
		{`<style><p>text</p></style>`, "/html[1]/body[1]/style[1]/p[1]", "text"},
		{`<head><p>text</p></head>`, "/html[1]/body[1]/head[1]/p[1]", "text"},
		{`<svg xmlns="urn:svg"><p xmlns="http://www.w3.org/1999/xhtml">text</p></svg>`, "/html[1]/body[1]/svg[1]/p[1]", "text"},
		{`<p xmlns="">text</p>`, "/html[1]/body[1]/p[1]", "text"},
		{`<p>text</p>`, "/html[1]/head[1]/title[1]", "Hidden"},
		{`text`, "/html[1]/body[1]", "text"},
		{`<body><p>text</p></body>`, "/html[1]/body[1]/body[1]/p[1]", "text"},
	} {
		t.Run(tc.locator+tc.body, func(t *testing.T) {
			raw := contentHTML(tc.body)
			a, p := contentFixture(t, raw)
			for _, new := range []string{"new", tc.text} {
				out, changed, err := ApplyText(a, p, textSet(raw, tc.locator, tc.text, new))
				if err == nil || changed || out != nil {
					t.Fatal("unsupported target allowed", err)
				}
			}
		})
	}
	for _, raw := range []string{contentHTML(`<p></p>`), `<html xmlns="http://www.w3.org/1999/xhtml"><body><p></p></body></html>`} {
		a, p := contentFixture(t, raw)
		out, changed, err := ApplyText(a, p, textSet(raw, "/html[1]/body[1]/p[1]", "", "new"))
		if err != nil || !changed || !bytes.Contains(out, []byte(`<p>new</p>`)) {
			t.Fatal("explicit empty target", err)
		}
	}
	for _, raw := range []string{
		`<html xmlns="http://www.w3.org/1999/xhtml"><body><p>x</p></body><body/></html>`,
		`<html xmlns="http://www.w3.org/1999/xhtml"><div><body><p>x</p></body></div></html>`,
		contentHTML(`<p>x</p>`) + `<broken`,
		strings.Replace(contentHTML(`<p>x</p>`), `</p>`, `</q>`, 1),
	} {
		if _, err := ContentText([]byte(raw), "/html[1]/body[1]/p[1]", xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"}); err == nil {
			t.Fatal("invalid structure allowed")
		}
	}
}

func TestTextSetBindingsAndBoundaries(t *testing.T) {
	raw := contentHTML(`<p> old </p>`)
	a, p := contentFixture(t, raw)
	base := textSet(raw, "/html[1]/body[1]/p[1]", " old ", "new")
	for _, change := range []func(*TextSet){
		func(s *TextSet) { s.BookPath = "书/Text/第二%20章.xhtml" }, func(s *TextSet) { s.BookPath = "missing.xhtml" },
		func(s *TextSet) { s.Locator = "/html[1]/body[1]/p[2]" }, func(s *TextSet) { s.ExpectedOldValue = "old" },
		func(s *TextSet) { s.NewValue = "\x00" }, func(s *TextSet) { s.NewValue = string([]byte{255}) },
		func(s *TextSet) { s.ResourceSHA256 = strings.ToUpper(s.ResourceSHA256) }, func(s *TextSet) { s.LocatorVersion = 2 },
		func(s *TextSet) { s.ExpectedOldValue = strings.Repeat("a", ContentTextLimit+1) },
		func(s *TextSet) { s.NewValue = strings.Repeat("a", ContentTextLimit+1) },
		func(s *TextSet) { s.Locator = strings.Repeat("a", 4097) },
	} {
		s := base
		change(&s)
		if _, _, err := ApplyText(a, p, s); err == nil {
			t.Fatal("bad param accepted", s.LocatorVersion)
		}
	}
	s := base
	s.ResourceSHA256 = strings.Repeat("0", 64)
	_, _, err := ApplyText(a, p, s)
	contentError(t, err, "INPUT_DRIFT")
	p.Manifest[0].MediaType = "text/html"
	if _, _, err := ApplyText(a, p, base); err == nil {
		t.Fatal("wrong MIME accepted")
	}
	p.Manifest[0].MediaType = "application/xhtml+xml"
	p.Manifest[0].Path = bookpath.BookPath("missing.xhtml")
	s = base
	s.BookPath = p.Manifest[0].Path
	if _, _, err := ApplyText(a, p, s); err == nil {
		t.Fatal("missing resource accepted")
	}
	for _, n := range []int{ContentTextLimit - 1, ContentTextLimit} {
		raw := contentHTML(`<p>` + strings.Repeat("x", n) + `</p>`)
		a, p := contentFixture(t, raw)
		s := textSet(raw, "/html[1]/body[1]/p[1]", strings.Repeat("x", n), strings.Repeat("y", n))
		out, changed, err := ApplyText(a, p, s)
		if err != nil || !changed || string(out) != contentHTML(`<p>`+strings.Repeat("y", n)+`</p>`) {
			t.Fatal("text byte boundary", n, err)
		}
	}
	s = base
	s.Locator = strings.Repeat("a", 4096)
	if err := s.Validate(); err != nil {
		t.Fatal("locator byte boundary", err)
	}
}

func FuzzContentSimpleTextReplacement(f *testing.F) {
	for _, s := range []string{"", "A&😀\r\nZ", "<markup> & \" '\t", "é", "\x00"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 4096 {
			t.Skip()
		}
		// Map arbitrary fuzz input to legal XML text, exercising real successful
		// replacements instead of mostly testing input rejection.
		if !utf8.ValidString(value) {
			value = strings.ToValidUTF8(value, "😀")
		}
		value = strings.Map(func(r rune) rune {
			if r < 32 && r != 9 && r != 10 && r != 13 || r == 0xfffe || r == 0xffff {
				return 'X'
			}
			return r
		}, value)
		prefix := "\xef\xbb\xbf" + `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>Same&amp;😀</p><p id='chosen'>`
		suffix := `</p><p>Same&amp;😀</p></body></html>` + "\r\n"
		input := []byte(prefix + `Same&#38;😀` + suffix)
		e, err := simpleTextElement(input, "/html[1]/body[1]/p[2]", xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
		if err != nil {
			t.Fatal(err)
		}
		out, changed, err := xmltext.Replace(input, e, "Same&😀", value)
		if err != nil {
			t.Fatal(err)
		}
		if changed != (value != "Same&😀") || !bytes.HasPrefix(out, []byte(prefix)) || !bytes.HasSuffix(out, []byte(suffix)) {
			t.Fatal("non-target bytes changed")
		}
		// Publication now shares xmltext's parser. Use the standard decoder
		// directly for an independent decoded-text expectation instead.
		var doc struct {
			Body struct {
				P []string `xml:"p"`
			} `xml:"body"`
		}
		if err := xml.Unmarshal(out, &doc); err != nil {
			t.Fatal(err)
		}
		if len(doc.Body.P) != 3 || doc.Body.P[1] != value {
			t.Fatal("decoded replacement mismatch")
		}
	})
}
