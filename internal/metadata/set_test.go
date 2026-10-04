package metadata

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/publication"
)

func opfDocument(content string) string {
	return `<?xml version="1.0" encoding="UTF-8"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:d="http://purl.org/dc/elements/1.1/" version="3.0"><metadata>` + content + `</metadata><manifest/><spine/></package>`
}

func TestPreservesExactBytesAndSelectsID(t *testing.T) {
	prefix := "\xef\xbb\xbf" + `<?xml version="1.0" encoding="UTF-8"?>` + "\r\n" + `<o:package xmlns:o="http://www.idpf.org/2007/opf" xmlns:d="http://purl.org/dc/elements/1.1/" version='3.0' strange="untouched"><o:metadata><!-- <d:title>Old &amp; Same</d:title> --><d:title id="other">Old &amp; Same</d:title><d:title id='chosen' xml:lang="en">`
	suffix := `</d:title><o:meta refines="#chosen" property="title-type">main</o:meta><d:creator>Old &amp; Same</d:creator></o:metadata><o:manifest/><o:spine/></o:package>` + "\r\n"
	input := []byte(prefix + "Old &amp; Same" + suffix)
	r := Set{DC, "title", "chosen", "Old & Same", "新 < & > \" '"}
	out, changed, err := Apply(input, r)
	// Independently spelled expected escaping, rather than using EscapeText.
	want := []byte(prefix + "新 &lt; &amp; &gt; &#34; &#39;" + suffix)
	if err != nil || !changed || !bytes.Equal(out, want) {
		t.Fatalf("got %q, changed %v, err %v; want %q", out, changed, err, want)
	}
	r.NewValue = r.ExpectedOldValue
	out, changed, err = Apply(input, r)
	if err != nil || changed || !bytes.Equal(out, input) {
		t.Fatalf("no-op changed input: %v %v", changed, err)
	}
}

func TestCreatorEntitiesAndEmptyExplicitText(t *testing.T) {
	for _, version := range []string{"2.0", "3.0"} {
		b := []byte(strings.Replace(opfDocument(`<d:creator>A&#x26;B&#13;C</d:creator>`), `version="3.0"`, `version="`+version+`"`, 1))
		out, changed, err := Apply(b, Set{DC, "creator", "", "A&B\rC", ""})
		if err != nil || !changed || !bytes.Contains(out, []byte(`<d:creator></d:creator>`)) {
			t.Fatalf("creator: %s %v %v", out, changed, err)
		}
		out, changed, err = Apply(out, Set{DC, "creator", "", "", "\r\n\t&"})
		if err != nil || !changed || !bytes.Contains(out, []byte(`<d:creator>&#xD;&#xA;&#x9;&amp;</d:creator>`)) {
			t.Fatalf("empty creator: %s %v %v", out, changed, err)
		}
	}
}

func TestRejectUnsafeAmbiguousAndComplex(t *testing.T) {
	base := opfDocument(`<d:title id="t">Old</d:title>`)
	cases := map[string]string{
		"multiple":               opfDocument(`<d:title>Old</d:title><d:title>Old</d:title>`),
		"duplicate-id":           opfDocument(`<d:title id="t">Old</d:title><d:creator id="t">Other</d:creator>`),
		"comment":                opfDocument(`<d:title>Ol<!--comment-->d</d:title>`),
		"cdata":                  opfDocument(`<d:title>O<![CDATA[l]]>d</d:title>`),
		"child":                  opfDocument(`<d:title>O<b>l</b>d</d:title>`),
		"pi":                     opfDocument(`<d:title>Ol<?x data?>d</d:title>`),
		"self-closing":           opfDocument(`<d:title/>`),
		"malformed-after-target": base + `<broken`,
		"multiple-roots":         base + base,
		"text-outside":           base + `oops`,
		"dtd":                    `<!DOCTYPE package [<!ENTITY test "Old">]>` + base,
		"custom-entity":          strings.Replace(base, `>Old<`, `>&unknown;<`, 1),
		"utf8":                   base + string([]byte{255}),
		"utf16-declaration":      strings.Replace(base, `UTF-8`, `UTF-16`, 1),
		"xml-base":               strings.Replace(base, `<manifest/>`, `<manifest xml:base="elsewhere"/>`, 1),
		"bad-later-attribute":    strings.Replace(base, `<manifest/>`, `<manifest id="a" id="b"/>`, 1),
		"undeclared-prefix":      strings.Replace(base, `<manifest/>`, `<x:manifest/>`, 1),
		"wrong-close-alias":      strings.Replace(base, `</d:title>`, `</a:title>`, 1),
		"declared-close-alias":   strings.Replace(strings.Replace(base, `<metadata>`, `<metadata xmlns:a="`+DC+`">`, 1), `</d:title>`, `</a:title>`, 1),
		"misplaced-declaration":  base + `<?xml version="1.0"?>`,
		"reserved-namespace":     strings.Replace(base, `<manifest/>`, `<manifest xmlns:xml="urn:wrong"/>`, 1),
		"invalid-qname":          strings.Replace(base, `<manifest/>`, `<manifest xmlns:a:b="urn:wrong"/>`, 1),
		"aliased-attribute":      strings.Replace(base, `<manifest/>`, `<manifest xmlns:a="urn:test" xmlns:b="urn:test" a:attr="1" b:attr="2"/>`, 1),
		"wrong-namespace":        strings.Replace(base, DC, DC+"wrong", 1),
		"unclosed":               strings.TrimSuffix(base, `</package>`),
		"size":                   base + strings.Repeat(" ", publication.XMLLimit),
		"depth":                  opfDocument(`<d:title>Old</d:title>` + strings.Repeat(`<x>`, 128) + strings.Repeat(`</x>`, 128)),
		"tokens":                 opfDocument(`<d:title>Old</d:title>` + strings.Repeat(`<!--x-->`, 200001)),
		"text-index":             opfDocument(`<d:title>Old</d:title>` + strings.Repeat(`<x>`, 16) + strings.Repeat("X", 2<<20) + strings.Repeat(`</x>`, 16)),
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			r := Set{DC, "title", "", "Old", "New"}
			if name == "self-closing" {
				r.ExpectedOldValue = ""
			}
			for _, noOp := range []bool{false, true} {
				if noOp {
					r.NewValue = r.ExpectedOldValue
				}
				if out, changed, err := Apply([]byte(input), r); err == nil || changed || out != nil {
					t.Fatalf("unsafe accepted: %v %v", changed, err)
				}
			}
		})
	}
	for _, r := range []Set{{DC, "identifier", "t", "Old", "New"}, {DC, "language", "t", "Old", "New"}, {DC, "title", "absent", "Old", "New"}, {DC, "title", "t", "wrong", "New"}, {DC, "title", "t", "Old", "\x00"}} {
		if _, _, err := Apply([]byte(base), r); err == nil {
			t.Fatalf("bad selector/value accepted: %+v", r)
		}
	}
}

func TestNamespaceScopesAndXMLSizeBoundary(t *testing.T) {
	input := opfDocument(`<x xmlns:d="urn:other"><d:title>Other</d:title></x><d:title>Old</d:title>`)
	out, changed, err := Apply([]byte(input), Set{DC, "title", "", "Old", "New"})
	want := strings.Replace(input, `<d:title>Old</d:title>`, `<d:title>New</d:title>`, 1)
	if err != nil || !changed || string(out) != want {
		t.Fatalf("namespace scope: %s %v %v", out, changed, err)
	}
	input = opfDocument(`<d:title>Old</d:title>`)
	input += strings.Repeat(" ", publication.XMLLimit-len(input))
	out, changed, err = Apply([]byte(input), Set{DC, "title", "", "Old", "Old"})
	if err != nil || changed || string(out) != input {
		t.Fatalf("XML size boundary: %v %v", changed, err)
	}
	if _, _, err := Apply([]byte(input+" "), Set{DC, "title", "", "Old", "Old"}); err == nil {
		t.Fatal("over-size no-op accepted")
	}
}

func FuzzSimpleTextPreservation(f *testing.F) {
	f.Add("A & < B", "New 中文 &", false)
	f.Add("", "", true)
	f.Fuzz(func(t *testing.T, old, new string, bom bool) {
		// Restrict seeds to real supported inputs, not mostly parser rejection.
		valid := func(s string) bool {
			for _, r := range s {
				if r < 32 || r == 0xfffd || r == 0xfffe || r == 0xffff {
					return false
				}
			}
			return len(s) < 4096
		}
		if !valid(old) || !valid(new) {
			t.Skip()
		}
		escape := func(s string) string {
			r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&#34;", "'", "&#39;")
			return r.Replace(s)
		}
		prefix := `<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata><title xmlns="` + DC + `">`
		if bom {
			prefix = "\xef\xbb\xbf" + prefix
		}
		suffix := `</title></metadata><manifest/><spine/></package>`
		input := []byte(prefix + escape(old) + suffix)
		out, changed, err := Apply(input, Set{DC, "title", "", old, new})
		if err != nil || changed != (old != new) || !bytes.Equal(out, []byte(prefix+escape(new)+suffix)) {
			t.Fatalf("preservation: %v %v", changed, err)
		}
	})
}
