package publication

import (
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/testfixture"
)

func TestT1BNavigationLocalStructureCertainty(t *testing.T) {
	for _, tc := range []struct {
		name, version, body string
		errors              int
		blocked             bool
	}{
		{"ncx unknown children", "2.0", `<navMap><navPoint><navLabel><text>A</text></navLabel>&children;</navPoint></navMap>`, 0, false},
		{"ncx known missing", "2.0", `<navMap><navPoint><navLabel><text>A</text></navLabel></navPoint></navMap>`, 1, false},
		{"ncx unrelated unknown", "2.0", `<navMap><navPoint><navLabel><text>A</text></navLabel>&children;</navPoint><navPoint><navLabel><text>B</text></navLabel></navPoint></navMap>`, 1, false},
		{"ncx unknown label known link", "2.0", `<navMap><navPoint><navLabel><text>&label;</text></navLabel><content src="Text/-first.xhtml#start"/></navPoint></navMap>`, 0, false},
		{"ncx unknown text child", "2.0", `<navMap><navPoint><navLabel>&text;</navLabel><content src="Text/-first.xhtml#start"/></navPoint></navMap>`, 0, false},
		{"ncx duplicate plus unknown", "2.0", `<navMap><navPoint>&children;<navLabel><text>A</text></navLabel><content src="Text/-first.xhtml"/><content src="Text/-first.xhtml"/></navPoint></navMap>`, 1, false},
		{"ncx missing map unknown", "2.0", `&map;`, 0, true},
		{"ncx empty map unknown", "2.0", `<navMap>&points;</navMap>`, 0, false},
		{"nav unknown items", "3.0", `<nav e:type="toc"><ol>&items;</ol></nav>`, 0, false},
		{"nav unknown label child", "3.0", `<nav e:type="toc"><ol><li>&children;</li></ol></nav>`, 0, false},
		{"nav known text beside unknown", "3.0", `<nav e:type="toc"><ol>bare &items;</ol></nav>`, 1, false},
		{"nav whitespace beside unknown", "3.0", `<nav e:type="toc"><ol>&#10;&items;&#32;</ol></nav>`, 0, false},
		{"nav duplicate plus unknown", "3.0", `<nav e:type="toc"><ol><li>&children;<a href="Text/-first.xhtml">A</a><span>B</span></li></ol></nav>`, 1, false},
		{"nav known bad child", "3.0", `<nav e:type="toc"><ol>&items;<div/></ol></nav>`, 1, false},
		{"nav unknown label known link", "3.0", `<nav e:type="toc"><ol><li><a href="Text/-first.xhtml#start">&label;</a></li></ol></nav>`, 0, false},
		{"nav unknown type", "3.0", `<nav e:type="&type;"><ol/></nav>`, 0, true},
		{"nav missing list unknown", "3.0", `<nav e:type="toc">&list;</nav>`, 0, true},
		{"nav duplicate list unknown", "3.0", `<nav e:type="toc">&list;<ol/><ol/></nav>`, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := testfixture.NavigationEPUB(tc.version)
			path := "书/nav.xhtml"
			raw := `<!DOCTYPE html [%unread;]><html xmlns="http://www.w3.org/1999/xhtml" xmlns:e="http://www.idpf.org/2007/ops"><body>` + tc.body + `</body></html>`
			if tc.version == "2.0" {
				path = "书/toc.ncx"
				raw = `<!DOCTYPE ncx [%unread;]><ncx xmlns="http://www.daisy.org/z3986/2005/ncx/">` + tc.body + `</ncx>`
			}
			for i := range entries {
				if entries[i].Name == path {
					entries[i].Data = []byte(raw)
				}
			}
			n := navigationFixture(t, entries)
			count, unresolved := 0, false
			for _, d := range n.Diagnostics {
				if d.Code == "NAVIGATION_STRUCTURE" {
					count++
				} else if d.Code == "XML_ENTITY_UNRESOLVED" {
					unresolved = true
				} else {
					t.Fatal("unexpected diagnostic", d)
				}
			}
			if count != tc.errors || !unresolved || (n.Status == "blocked") != tc.blocked || n.XMLCoverage == nil {
				t.Fatalf("structure certainty: %+v", n)
			}
			if strings.Contains(tc.name, "known link") && (len(n.Entries) != 1 || n.Entries[0].Target == nil || n.Entries[0].Exists == nil || !*n.Entries[0].Exists) {
				t.Fatal("unknown label hid known target", n)
			}
		})
	}
}
