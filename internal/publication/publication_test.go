package publication

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

func loadFixture(t *testing.T, entries []testfixture.Entry, root string) (*Publication, error) {
	t.Helper()
	f := filepath.Join(t.TempDir(), "book.epub")
	testfixture.ZIP(t, f, entries)
	a, e := archive.Open(f, archive.DefaultLimits)
	if e != nil {
		return nil, e
	}
	defer a.Close()
	p, e := Load(a, root)
	// Read parsing never modifies the archive's resource bytes.
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name, "/") {
			continue
		}
		b, re := a.Read(bookpath.BookPath(entry.Name), 256<<20)
		if re != nil || !bytes.Equal(b, entry.Data) {
			t.Fatalf("resource altered: %s %v", entry.Name, re)
		}
	}
	return p, e
}

func TestEPUB2And3(t *testing.T) {
	for _, version := range []string{"2.0", "3.0"} {
		t.Run(version, func(t *testing.T) {
			p, e := loadFixture(t, testfixture.EPUB(version, false), "")
			if e != nil {
				t.Fatal(e)
			}
			if p.Version != version || p.Rootfile != "书/Deep/package.opf" || p.UniqueIdentifier != "uid" {
				t.Fatalf("wrong package: %+v", p)
			}
			if len(p.Metadata) != 4 || p.Metadata[1].Name.Space != "http://purl.org/dc/elements/1.1/" || p.Metadata[1].Text != "测试 & Space" {
				t.Fatalf("metadata lost: %+v", p.Metadata)
			}
			expectedItems := 3
			if version == "2.0" {
				expectedItems = 4
				if p.Metadata[3].attr("content") != "kepub-fixture" || p.SpineAttributes[0].Value != "ncx" {
					t.Fatal("EPUB2 attributes lost")
				}
			} else {
				if p.Metadata[3].attr("refines") != "#t" {
					t.Fatal("EPUB3 refinement lost")
				}
			}
			if len(p.Manifest) != expectedItems || p.Manifest[0].Path != "书/Text/第二 章.xhtml" || p.Manifest[0].Href != "../Text/第二%20章.xhtml?x=1#note" || !p.Manifest[0].Exists {
				t.Fatal(p.Manifest)
			}
			if len(p.Spine) != 2 || p.Spine[0].IDRef != "c1" || p.Spine[1].IDRef != "c2" || p.Spine[1].Linear != "no" {
				t.Fatal(p.Spine)
			}
			for _, s := range p.Spine {
				if s.IDRef == "nav" {
					t.Fatal("nav incorrectly injected in spine")
				}
			}
		})
	}
}

func TestMultiRootfile(t *testing.T) {
	entries := testfixture.EPUB("3.0", true)
	_, e := loadFixture(t, entries, "")
	var fe *fault.Error
	if !errors.As(e, &fe) || fe.Exit != 2 || fe.Code != "ROOTFILE_REQUIRED" {
		t.Fatal(e)
	}
	p, e := loadFixture(t, entries, "alternate.opf")
	if e != nil || p.Version != "2.0" || p.Rootfile != "alternate.opf" || len(p.Rootfiles) != 2 {
		t.Fatalf("selection: %+v %v", p, e)
	}
	_, e = loadFixture(t, entries, "missing.opf")
	if !errors.As(e, &fe) || fe.Code != "INVALID_ROOTFILE" {
		t.Fatal(e)
	}
}

func TestXMLSafety(t *testing.T) {
	for _, tc := range []struct{ xml, code string }{
		{`<x><y></x>`, "XML_NOT_WELL_FORMED"},
		{`<x>&external;</x>`, "XML_NOT_WELL_FORMED"},
		{`<!DOCTYPE x SYSTEM "https://example.com/evil.dtd"><x/>`, "UNSUPPORTED_XML_DTD"},
		{`<!DOCTYPE x [<!ENTITY x "expanded">]><x>&x;</x>`, "UNSUPPORTED_XML_DTD"},
		{`<x/><y/>`, "XML_NOT_WELL_FORMED"},
		{`<x a="1" a="2"/>`, "XML_NOT_WELL_FORMED"},
		{`<x xml:base="../"/>`, "UNSUPPORTED_XML_BASE"},
		{strings.Repeat("<x>", 129) + strings.Repeat("</x>", 129), "XML_LIMIT"},
		{string([]byte{0xff, 0xfe, '<', 0, 'x', 0}), "XML_NOT_WELL_FORMED"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			_, e := parseXML([]byte(tc.xml))
			var fe *fault.Error
			if !errors.As(e, &fe) || fe.Code != tc.code {
				t.Fatalf("want %s got %v", tc.code, e)
			}
		})
	}
	e, err := parseXML([]byte("\xef\xbb\xbf<x>&amp;&#x4e2d;</x>"))
	if err != nil || e.Text != "&中" {
		t.Fatalf("UTF-8 BOM/entities: %+v %v", e, err)
	}
}

func TestRestrictedStructuresAndCaseMismatch(t *testing.T) {
	entries := testfixture.EPUB("3.0", false)
	opf := string(entries[2].Data)
	opf = strings.Replace(opf, `<metadata>`, `<metadata><meta property="rendition:layout">pre-paginated</meta>`, 1)
	opf = strings.Replace(opf, `properties="nav"`, `properties="nav scripted"`, 1)
	opf = strings.Replace(opf, `../Text/-first.xhtml`, `../text/-first.xhtml`, 1)
	entries[2].Data = []byte(opf)
	entries = append(entries, testfixture.Entry{Name: "META-INF/encryption.xml", Data: []byte(`<encryption><EncryptionMethod Algorithm="http://www.idpf.org/2008/embedding"/></encryption>`)}, testfixture.Entry{Name: "META-INF/signatures.xml", Data: []byte("uninterpreted")})
	p, e := loadFixture(t, entries, "")
	if e != nil {
		t.Fatal(e)
	}
	codes := map[string]bool{}
	for _, l := range p.Limitations {
		codes[l.Code] = true
	}
	for _, code := range []string{"FIXED_LAYOUT", "SCRIPTED", "MISSING_MANIFEST_RESOURCE", "ENCRYPTION_DECLARED", "ENCRYPTION_ALGORITHM", "SIGNATURES", "READ_ONLY_PARTIAL"} {
		if !codes[code] {
			t.Fatal("missing restriction", code)
		}
	}
	if p.Manifest[1].Exists {
		t.Fatal("case mismatch hidden")
	}
}

func FuzzXML(f *testing.F) {
	for _, s := range []string{`<x/>`, `<x><y></x>`, `<!DOCTYPE x><x/>`, `<x>&amp;</x>`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > XMLLimit {
			return
		}
		e, err := parseXML([]byte(s))
		if err == nil && e == nil {
			t.Fatal("successful parse with no root")
		}
	})
}
