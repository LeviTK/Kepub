package testfixture

import "strings"

// NavigationEPUB deliberately differs from spine and manifest order. The first
// tree has depth three, the second is a leaf; nav never appears in the spine.
func NavigationEPUB(version string) []Entry {
	entries := EPUB(version, false)
	for i := range entries {
		switch entries[i].Name {
		case "书/Text/-first.xhtml":
			entries[i].Data = []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="start">First</p><a href="第二%20章.xhtml?mode=read#note">Next</a><a href="#start">Self</a></body></html>`)
		case "书/nav.xhtml":
			entries[i].Data = []byte(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:e="http://www.idpf.org/2007/ops"><body><nav e:type="landmarks"><ol><li><a href="Text/-first.xhtml">Landmark, not TOC</a></li></ol></nav><nav e:type="toc"><h1>Contents</h1><ol><li><span>Part A</span><ol><li><a href="Text/第二%20章.xhtml?from=toc#note">Second <em>&amp;</em> final</a><ol><li><a href="Text/-first.xhtml#start">Return</a></li></ol></li></ol></li><li><a href="Text/-first.xhtml#start">First</a></li></ol></nav></body></html>`)
		case "书/toc.ncx":
			entries[i].Data = []byte(`<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head/><docTitle><text>Fixture</text></docTitle><navMap><navPoint id="n2" playOrder="1"><navLabel><text>Second &amp; final</text></navLabel><content src="Text/第二%20章.xhtml?from=toc#note"/><navPoint id="n3" playOrder="2"><navLabel><text>Return</text></navLabel><content src="Text/-first.xhtml#start"/><navPoint id="n4" playOrder="3"><navLabel><text>Deep</text></navLabel><content src="Text/第二%20章.xhtml#note"/></navPoint></navPoint></navPoint><navPoint id="n1" playOrder="4"><navLabel><text>First</text></navLabel><content src="Text/-first.xhtml#start"/></navPoint></navMap></ncx>`)
		}
	}
	return entries
}

// ReferenceEPUB includes static supported syntax and declared unknown syntax.
// Missing links are intentional diagnostics, not a supposedly valid EPUB.
func ReferenceEPUB() []Entry {
	entries := NavigationEPUB("3.0")
	items := `<item id="css" href="../Styles/main.css" media-type="text/css"/><item id="svg" href="../Images/cover.svg" media-type="image/svg+xml"/><item id="smil" href="../overlay.smil" media-type="application/smil+xml"/><item id="js" href="../book.js" media-type="application/javascript"/><item id="missing" href="../missing.xhtml" media-type="application/xhtml+xml"/>`
	entries[2].Data = []byte(strings.Replace(string(entries[2].Data), "</manifest>", items+"</manifest>", 1))
	entries[3].Data = []byte(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:s="http://www.w3.org/2000/svg"><head><link href="../Styles/main.css" rel="stylesheet"/><style>p { background: url('../Images/cover.svg#shape'); }</style></head><body><p id="note" style="background:url(../Images/cover.svg#shape)">Second</p><img src="../Images/cover.svg" srcset="a.png 1x, b.png 2x"/><a href="-first.xhtml#absent">Missing fragment</a><a href="../missing.xhtml#lost">Missing file</a><a href="https://example.invalid/external#remote">External</a><a href="../../../escape.xhtml">Unsafe</a><s:svg><s:use href="../Images/cover.svg#shape"/></s:svg><script>location.href = 'hidden.xhtml';</script></body></html>`)
	return append(entries,
		Entry{Name: "书/Styles/main.css", Data: []byte(`/* url(ignored.svg) */ @import "theme.css"; @import url("../other.css"); p { background: URL('../Images/cover.svg#shape'); content: "url(not-a-reference.svg)"; image: image-set("unhandled.png" 1x); }`)},
		Entry{Name: "书/Images/cover.svg", Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><g id="shape"/><use xlink:href="#shape"/><image href="../Text/100%25.xhtml"/></svg>`)},
		Entry{Name: "书/overlay.smil", Data: []byte(`<smil><text src="hidden.xhtml"/></smil>`)},
		Entry{Name: "书/book.js", Data: []byte(`fetch("hidden.xhtml")`)},
		Entry{Name: "书/Text/100%.xhtml", Data: []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body/></html>`)},
	)
}
