package references

import (
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/testfixture"
)

func TestHTMLURLPeripheralWhitespace(t *testing.T) {
	for _, tc := range []struct{ name, encoded, raw, status, fragment string }{
		{"ASCII", " &#9;&#10;&#13;#start&#13;&#10;&#9; ", " \t\n\r#start\r\n\t ", "resolved", "start"},
		{"encoded space", "#start%20", "#start%20", "resolved", "start "},
		{"NBSP", "&#160;#start&#160;", "\u00a0#start\u00a0", "missing", "start\u00a0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := testfixture.NavigationEPUB("3.0")
			for i := range entries {
				if entries[i].Name == "书/Text/-first.xhtml" {
					entries[i].Data = []byte(strings.Replace(string(entries[i].Data), `href="#start"`, `href="`+tc.encoded+`"`, 1))
				}
			}
			g := graphFixture(t, entries)
			for _, e := range g.Edges {
				if e.Source == "书/Text/-first.xhtml" && string(e.Href) == tc.raw {
					if e.Status != tc.status || e.Target == nil || e.Target.Fragment != tc.fragment || (tc.name == "ASCII" && e.FragmentStatus != "resolved") || (tc.name == "encoded space" && e.FragmentStatus != "missing") {
						t.Fatalf("wrong URL: %+v", e)
					}
					return
				}
			}
			t.Fatal("lost original href")
		})
	}
	entries := testfixture.ReferenceEPUB()
	for i := range entries {
		if entries[i].Name == "书/Text/第二 章.xhtml" {
			entries[i].Data = []byte(strings.ReplaceAll(string(entries[i].Data), `="../Images/cover.svg"`, `=" &#9;../Images/cover.svg&#13; "`))
		}
	}
	g := graphFixture(t, entries)
	found := false
	for _, e := range g.Edges {
		if e.Syntax == "xhtml.src" && string(e.Href) == " \t../Images/cover.svg\r " {
			found = true
			if e.Status != "resolved" || e.Target.Path != "书/Images/cover.svg" {
				t.Fatalf("src: %+v", e)
			}
		}
	}
	if !found {
		t.Fatal("missing src")
	}
}
