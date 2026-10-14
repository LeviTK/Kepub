package metadata_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/metadata"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

const opfV2 = "http://www.idpf.org/2007/opf"
const modifiedV2 = `<meta id="m" property="dcterms:modified">old-invalid-date</meta>`

func requestV2(field string) metadata.SetV2 {
	r := metadata.SetV2{BookPath: "EPUB/package.opf", RevisionID: "accepted", ResourceSHA256: strings.Repeat("0", 64), Namespace: metadata.DC, LocalName: field, ExpectedOldValue: "deliberately not checked here", NewValue: "new"}
	if field == "language" {
		r.NewValue = "zz"
	}
	if field == "meta" {
		r.Namespace = opfV2
		r.Property = "dcterms:modified"
		r.NewValue = "2024-02-29T23:59:59Z"
	}
	return r
}
func packageV2(attrs, content, other string) string {
	return `<package xmlns="` + opfV2 + `" xmlns:dc="` + metadata.DC + `" version="3.0" unique-identifier="primary" ` + attrs + `><metadata>` + content + `</metadata>` + other + `</package>`
}
func checkV2(t *testing.T, input string, r metadata.SetV2, code string) {
	t.Helper()
	e, err := metadata.SelectV2([]byte(input), r)
	if code == "" {
		if err != nil || e == nil {
			t.Fatalf("selection: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatal("unexpected selection success")
	}
	var f *fault.Error
	if code == "invalid" {
		if errors.As(err, &f) {
			t.Fatalf("expected invalid operation, got %+v", f)
		}
		return
	}
	if !errors.As(err, &f) || f.Code != code || f.Exit != 3 {
		t.Fatalf("expected %s/3, got %v", code, err)
	}
}

func TestSelectV2ModifiedSubjectsAndVocabulary(t *testing.T) {
	r := requestV2("meta")
	r.ID = "m"
	for _, extra := range []string{``, `<meta name="cover" content="cover.png"/>`, `<meta property="dcterms:modified" refines="#primary">another subject</meta>`, `<meta property="schema:accessMode">textual</meta>`} {
		checkV2(t, packageV2("", modifiedV2+extra, `<collection role="series"><metadata><meta property="missing:term" refines="">untouched</meta><meta property="dcterms:modified">same or different</meta></metadata></collection>`), r, "")
	}
	for _, tc := range []struct{ name, attrs, body string }{
		{"duplicate", "", modifiedV2 + `<meta property="dcterms:modified">second</meta>`},
		{"spaced", "", `<meta id="m" property=" dcterms:modified ">old</meta>`},
		{"alias-only", `prefix="alias: http://purl.org/dc/terms/"`, `<meta property="alias:modified">old</meta>`},
		{"alias-duplicate", `prefix="alias: http://purl.org/dc/terms/"`, modifiedV2 + `<meta property="alias:modified">old</meta>`},
		{"redeclaration", `prefix="dcterms: http://purl.org/dc/terms/"`, modifiedV2},
		{"unused-alias", `prefix="alias: http://purl.org/dc/terms/"`, modifiedV2},
		{"bad-prefix", `prefix="foo:http://example.org/"`, modifiedV2},
		{"empty-prefix", `prefix=""`, modifiedV2},
		{"undeclared", "", modifiedV2 + `<meta property="missing:term">x</meta>`},
		{"nested", "", modifiedV2 + `<wrapper><meta property="dcterms:modified">x</meta></wrapper>`},
		{"empty-refines", "", modifiedV2 + `<meta property="schema:name" refines=" &#x9; ">x</meta>`},
	} {
		t.Run(tc.name, func(t *testing.T) { checkV2(t, packageV2(tc.attrs, tc.body, ""), r, "invalid") })
	}
	checkV2(t, packageV2(`prefix="foo: https://example.org/"`, modifiedV2+`<meta property="foo:term">x</meta>`, ""), r, "")
	for _, value := range []string{"old-invalid-date", "different"} {
		input := packageV2("", modifiedV2+`<meta name="cover" content="x"/>`, `<collection><metadata><meta property="dcterms:modified">`+value+`</meta></metadata></collection>`)
		e, err := metadata.SelectV2([]byte(input), r)
		if err != nil {
			t.Fatal(err)
		}
		out, _, err := xmltext.Replace([]byte(input), e, e.Text, r.NewValue)
		want := strings.Replace(input, modifiedV2, `<meta id="m" property="dcterms:modified">2024-02-29T23:59:59Z</meta>`, 1)
		if err != nil || string(out) != want {
			t.Fatalf("subject byte preservation: %v", err)
		}
	}
}

func TestSelectV2NewValuesOnly(t *testing.T) {
	for _, value := range []string{"zz", "abcd", "xx-YY", "en-123", "en-GB-oed", "en-a-aaa-a-bbb", "X-a-9", "I-KLINGON", "sgn-BE-FR", "abc-def-ghi-jkl-Latn-123-1996-a-abc-x-z"} {
		r := requestV2("language")
		r.NewValue = value
		checkV2(t, packageV2("", `<dc:language>old invalid language</dc:language>`, ""), r, "")
	}
	for _, value := range []string{"", "en_US", " en", "en ", "en--US", "x-aaaaaaaaa", "en-a", "i-ami i-bnn", "en-KK", "en-ſſ", "en-abc-def-ghi-jkl"} {
		r := requestV2("language")
		r.NewValue = value
		checkV2(t, packageV2("", `<dc:language>`+value+`</dc:language>`, ""), r, "invalid")
	}
	for _, value := range []string{"0001-01-01T00:00:00Z", "9999-12-31T23:59:59Z", "2000-02-29T00:00:00Z"} {
		r := requestV2("meta")
		r.NewValue = value
		checkV2(t, packageV2("", modifiedV2, ""), r, "")
	}
	for _, value := range []string{"0000-01-01T00:00:00Z", "2024-02-30T00:00:00Z", "1900-02-29T00:00:00Z", "2024-01-01T00:00:60Z", "2024-01-01t00:00:00Z", "2024-01-01T00:00:00.0Z", "2024-01-01T00:00:00+00:00"} {
		r := requestV2("meta")
		r.NewValue = value
		checkV2(t, packageV2("", modifiedV2, ""), r, "invalid")
	}
}

func TestSelectV2IdentifierDependencies(t *testing.T) {
	body := `<dc:identifier id="primary">old</dc:identifier><dc:identifier id="secondary">old</dc:identifier>`
	for _, dep := range []string{`<manifest><item media-type="application/x-dtbncx+xml"/></manifest>`, `<spine toc=""/>`} {
		r := requestV2("identifier")
		r.NewValue = "old"
		r.ID = "primary"
		checkV2(t, packageV2("", body, dep), r, "UNSUPPORTED_INPUT")
		r.ID = "secondary"
		checkV2(t, packageV2("", body, dep), r, "")
	}
	r := requestV2("identifier")
	r.ID = "primary"
	checkV2(t, packageV2("", body, ""), r, "")
	r.ID = "secondary"
	for _, ref := range []string{"", "missing"} {
		checkV2(t, strings.Replace(packageV2("", body, ""), `unique-identifier="primary"`, `unique-identifier="`+ref+`"`, 1), r, "invalid")
	}
	checkV2(t, packageV2("", `<dc:identifier id="secondary">old</dc:identifier>`, `<collection><metadata><dc:identifier id="primary">other</dc:identifier></metadata></collection>`), r, "invalid")
	checkV2(t, packageV2("", body+`<dc:title id="primary">duplicate</dc:title>`, ""), r, "invalid")
	checkV2(t, `<!DOCTYPE package [<!ATTLIST item media-type CDATA "application/xhtml+xml">]>`+packageV2("", body, `<manifest><item/></manifest>`), r, "UNSUPPORTED_INPUT")
}

func TestSelectV2ProvenanceAndV1Compatibility(t *testing.T) {
	r := requestV2("title")
	r.NewValue = "old"
	for _, body := range []string{`<dc:title/>`, `<dc:title><![CDATA[old]]></dc:title>`, `<dc:title><b>old</b></dc:title>`} {
		checkV2(t, packageV2("", body, ""), r, "UNSUPPORTED_INPUT")
	}
	for _, decl := range []string{`<!ATTLIST package version CDATA "3.0">`, `<!ATTLIST package prefix CDATA "foo: https://example.org/">`, `<!ATTLIST package unique-identifier CDATA "primary">`, `<!ATTLIST dc:title id CDATA "title">`} {
		input := packageV2("", `<dc:title>old</dc:title>`, "")
		if strings.Contains(decl, "version") {
			input = strings.Replace(input, ` version="3.0"`, "", 1)
		}
		if strings.Contains(decl, "unique-identifier") {
			input = strings.Replace(input, ` unique-identifier="primary"`, "", 1)
		}
		checkV2(t, `<!DOCTYPE package [`+decl+`]>`+input, r, "UNSUPPORTED_INPUT")
	}
	for _, attr := range []string{"property", "refines"} {
		input := modifiedV2
		if attr == "property" {
			input = `<meta id="m">old</meta>`
		}
		checkV2(t, `<!DOCTYPE package [<!ATTLIST meta `+attr+` CDATA "dcterms:modified">]>`+packageV2("", input, ""), requestV2("meta"), "UNSUPPORTED_INPUT")
	}
	checkV2(t, `<!DOCTYPE package [<!ENTITY old "old">]>`+packageV2("", `<dc:title>&old;</dc:title>`, ""), r, "UNSUPPORTED_INPUT")
	checkV2(t, `<!DOCTYPE package [<!ENTITY empty "">]>`+packageV2("", `<dc:title>old&empty;</dc:title>`, ""), r, "UNSUPPORTED_INPUT")
	checkV2(t, `<!DOCTYPE package [<!ENTITY % unread SYSTEM "file:///not-read">%unread;]>`+packageV2("", `<dc:title>old</dc:title>`, ""), r, "XML_ENTITY_UNRESOLVED")
	input := strings.Replace(packageV2("", `<dc:title>old</dc:title>`, ""), `version="3.0"`, `version="2.0"`, 1)
	checkV2(t, input, r, "UNSUPPORTED_INPUT")
	out, changed, err := metadata.Apply([]byte(input), metadata.Set{Namespace: metadata.DC, LocalName: "title", ExpectedOldValue: "old", NewValue: ""})
	if err != nil || !changed || string(out) != strings.Replace(input, ">old</dc:title>", "></dc:title>", 1) {
		t.Fatalf("v1 changed: %s %v", out, err)
	}
}

func TestSelectV2ExplicitEntityAttributes(t *testing.T) {
	const decl = `<!DOCTYPE package [<!ENTITY empty ""><!ENTITY id "ti&empty;tle"><!ENTITY version "3.0"><!ENTITY uid "primary"><!ENTITY prefix "foo: https://example.org/"><!ENTITY property "dcterms:modified"><!ENTITY refines "#title">]>`
	for _, id := range []string{`title`, `ti&#116;le`, `&id;`} {
		input := decl + packageV2(`prefix="&prefix;"`, `<dc:title id="`+id+`">old</dc:title>`+modifiedV2+`<meta property="schema:name" refines="&refines;">keep</meta>`, "")
		input = strings.Replace(input, `version="3.0"`, `version="&version;"`, 1)
		input = strings.Replace(input, `unique-identifier="primary"`, `unique-identifier="&uid;"`, 1)
		input = strings.Replace(input, `property="dcterms:modified"`, `property="&property;"`, 1)
		input = strings.Replace(input, "old-invalid-date", "2024-02-29T23:59:59Z", 1)
		for _, field := range []string{"title", "meta"} {
			r := requestV2(field)
			old := "2024-02-29T23:59:59Z"
			if field == "title" {
				r.ID = "title"
				old = "old"
			} else {
				r.NewValue = "2025-01-02T03:04:05Z"
			}
			for _, noop := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/noop=%v", id, field, noop), func(t *testing.T) {
					request := r
					if noop {
						request.NewValue = old
					}
					e, err := metadata.SelectV2([]byte(input), request)
					if err != nil {
						t.Fatal(err)
					}
					out, _, err := xmltext.Replace([]byte(input), e, old, request.NewValue)
					want := strings.Replace(input, ">"+old+"</", ">"+request.NewValue+"</", 1)
					if err != nil || string(out) != want {
						t.Fatalf("explicit attribute byte oracle: %v", err)
					}
				})
			}
		}
	}
}

func TestSelectV2EncodedPhysicalReplacement(t *testing.T) {
	for _, encoding := range []string{"UTF-8", "UTF-16LE", "UTF-16BE"} {
		t.Run(encoding, func(t *testing.T) {
			original := `<?xml version="1.0" encoding="` + encoding + `"?><!DOCTYPE package [<!ENTITY target "a">]>` + packageV2("", `<!--keep--><dc:title id="&target;">A&amp;😀</dc:title><dc:title id="b">second</dc:title>`, "")
			encode := func(s string) []byte {
				if encoding == "UTF-8" {
					return []byte(s)
				}
				var order binary.ByteOrder = binary.LittleEndian
				b := []byte{255, 254}
				if encoding == "UTF-16BE" {
					order = binary.BigEndian
					b = []byte{254, 255}
				}
				for _, u := range utf16.Encode([]rune(s)) {
					var pair [2]byte
					order.PutUint16(pair[:], u)
					b = append(b, pair[:]...)
				}
				return b
			}
			r := requestV2("title")
			r.ID = "a"
			r.NewValue = "新😀 & text"
			input := encode(original)
			e, err := metadata.SelectV2(input, r)
			if err != nil || e.Text != "A&😀" {
				t.Fatalf("select: %v", err)
			}
			out, _, err := xmltext.Replace(input, e, e.Text, r.NewValue)
			if err != nil {
				t.Fatal(err)
			}
			want := encode(strings.Replace(original, "A&amp;😀", "新😀 &amp; text", 1))
			if !bytes.Equal(out, want) {
				t.Fatal("full independent byte oracle mismatch")
			}
			r.ID = ""
			if _, err := metadata.SelectV2(input, r); err == nil {
				t.Fatal("ambiguous titles accepted")
			}
		})
	}
}

func TestSetV2ValidationAndCanonicalOrder(t *testing.T) {
	r := requestV2("title")
	r.ID = "a"
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"bookPath":"EPUB/package.opf","revisionId":"accepted","resourceSha256":"` + strings.Repeat("0", 64) + `","namespace":"` + metadata.DC + `","localName":"title","id":"a","expectedOldValue":"deliberately not checked here","newValue":"new"}`
	if string(b) != want {
		t.Fatalf("canonical order: %s", b)
	}
	for _, change := range []func(*metadata.SetV2){func(r *metadata.SetV2) { r.BookPath = "a/../b" }, func(r *metadata.SetV2) { r.ResourceSHA256 = strings.Repeat("A", 64) }, func(r *metadata.SetV2) { r.Property = "dcterms:modified" }, func(r *metadata.SetV2) { r.NewValue = " \t" }, func(r *metadata.SetV2) { r.NewValue = "\x00" }} {
		bad := r
		change(&bad)
		if bad.Validate() == nil {
			t.Fatalf("invalid params accepted: %+v", bad)
		}
	}
}
