// Package testfixture generates small reproducible test archives; no downloaded books.
package testfixture

import (
	"archive/zip"
	"fmt"
	"os"
	"strings"
	"testing"
)

type Entry struct {
	Name string
	Data []byte
	Mode os.FileMode
}

func ZIP(t testing.TB, filename string, entries []Entry) {
	t.Helper()
	f, e := os.Create(filename)
	if e != nil {
		t.Fatal(e)
	}
	z := zip.NewWriter(f)
	for _, entry := range entries {
		h := &zip.FileHeader{Name: entry.Name, Method: zip.Deflate}
		if entry.Name == "mimetype" {
			h.Method = zip.Store
		}
		if entry.Mode != 0 {
			h.SetMode(entry.Mode)
		}
		w, e := z.CreateHeader(h)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = w.Write(entry.Data); e != nil {
			t.Fatal(e)
		}
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
}

func EPUB(version string, multi bool) []Entry {
	roots := `<rootfile full-path="书/Deep/package.opf" media-type="application/oebps-package+xml"/>`
	if multi {
		roots += `<rootfile full-path="alternate.opf" media-type="application/oebps-package+xml"/>`
	}
	container := `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles>` + roots + `</rootfiles></container>`
	opf := fmt.Sprintf(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/" version="%s" unique-identifier="uid"><metadata><dc:identifier id="uid">urn:test:123</dc:identifier><dc:title id="t">测试 &amp; Space</dc:title><dc:language>zh</dc:language><meta property="title-type" refines="#t">main</meta></metadata><manifest><item id="c2" href="../Text/第二%%20章.xhtml?x=1#note" media-type="application/xhtml+xml"/><item id="c1" href="../Text/-first.xhtml" media-type="application/xhtml+xml"/><item id="nav" href="../nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine page-progression-direction="rtl"><itemref idref="c1"/><itemref idref="c2" linear="no"/></spine></package>`, version)
	entries := []Entry{{"mimetype", []byte("application/epub+zip"), 0}, {"META-INF/container.xml", []byte(container), 0}, {"书/Deep/package.opf", []byte(opf), 0}, {"书/Text/第二 章.xhtml", []byte("<?xml version=\"1.0\"?><html xmlns=\"http://www.w3.org/1999/xhtml\"><body><p id=\"note\">第二章</p></body></html>\r\n"), 0}, {"书/Text/-first.xhtml", []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body>First</body></html>`), 0}, {"书/nav.xhtml", []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body/></html>`), 0}, {"unlisted.bin", []byte{0, 255, 13, 10, 42}, 0}, {"empty/", nil, os.ModeDir | 0700}}
	if version == "2.0" {
		opf = strings.Replace(opf, `<meta property="title-type" refines="#t">main</meta>`, `<meta name="generator" content="kepub-fixture"/>`, 1)
		opf = strings.Replace(opf, ` properties="nav"`, "", 1)
		opf = strings.Replace(opf, `</manifest>`, `<item id="ncx" href="../toc.ncx" media-type="application/x-dtbncx+xml"/></manifest>`, 1)
		opf = strings.Replace(opf, `<spine page-progression-direction="rtl">`, `<spine toc="ncx">`, 1)
		entries[2].Data = []byte(opf)
		entries = append(entries, Entry{"书/toc.ncx", []byte(`<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head/><docTitle><text>Fixture</text></docTitle><navMap><navPoint id="n1" playOrder="1"><navLabel><text>First</text></navLabel><content src="Text/-first.xhtml"/></navPoint></navMap></ncx>`), 0})
	}
	if multi {
		entries = append(entries, Entry{"alternate.opf", []byte(`<package xmlns="http://www.idpf.org/2007/opf" version="2.0"><metadata/><manifest/><spine/></package>`), 0})
	}
	return entries
}
