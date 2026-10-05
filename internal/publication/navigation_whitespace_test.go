package publication

import (
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/testfixture"
)

func TestNavigationHTMLURLPeripheralWhitespace(t *testing.T) {
	for _, tc := range []struct {
		name, encoded, raw, path, fragment string
		exists                             bool
	}{
		{"ASCII", " &#9;&#10;&#13;Text/-first.xhtml#start&#13;&#10;&#9; ", " \t\n\rText/-first.xhtml#start\r\n\t ", "书/Text/-first.xhtml", "start", true},
		{"encoded space", "Text/-first.xhtml%20#start", "Text/-first.xhtml%20#start", "书/Text/-first.xhtml ", "start", false},
		{"NBSP", "&#160;Text/-first.xhtml#start&#160;", "\u00a0Text/-first.xhtml#start\u00a0", "书/\u00a0Text/-first.xhtml", "start\u00a0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := testfixture.NavigationEPUB("3.0")
			entries[5].Data = []byte(strings.ReplaceAll(string(entries[5].Data), "Text/-first.xhtml#start", tc.encoded))
			n := navigationFixture(t, entries)
			x := n.Entries[1]
			if x.Href == nil || string(*x.Href) != tc.raw || x.Target == nil || string(x.Target.Path) != tc.path || x.Target.Fragment != tc.fragment || x.Exists == nil || *x.Exists != tc.exists {
				t.Fatalf("nav: %+v", x)
			}
			if tc.exists && n.Status != "complete" {
				t.Fatalf("status: %+v", n)
			}
		})
	}
	entries := testfixture.NavigationEPUB("2.0")
	for i := range entries {
		if entries[i].Name == "书/toc.ncx" {
			entries[i].Data = []byte(strings.ReplaceAll(string(entries[i].Data), "Text/-first.xhtml#start", "Text/-first.xhtml "))
		}
	}
	n := navigationFixture(t, entries)
	x := n.Entries[1]
	if x.Href == nil || *x.Href != "Text/-first.xhtml " || x.Target == nil || x.Target.Path != "书/Text/-first.xhtml " || x.Exists == nil || *x.Exists {
		t.Fatalf("NCX URL space was trimmed: %+v", x)
	}
}
