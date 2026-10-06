package publication

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

func navigationFixture(t *testing.T, entries []testfixture.Entry) Navigation {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "navigation.epub")
	testfixture.ZIP(t, filename, entries)
	a, err := archive.Open(filename, archive.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	p, err := Load(a, "")
	if err != nil {
		t.Fatal(err)
	}
	spine := append([]Itemref{}, p.Spine...)
	n := LoadNavigation(a, p)
	if !reflect.DeepEqual(spine, p.Spine) || p.Spine[0].IDRef != "c1" {
		t.Fatal("navigation changed spine")
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name, "/") {
			data, err := a.Read(bookpath.BookPath(e.Name), 256<<20)
			if err != nil || !bytes.Equal(data, e.Data) {
				t.Fatal("read changed resource", e.Name, err)
			}
		}
	}
	return n
}

func TestNavigationHierarchyAndSpineSeparation(t *testing.T) {
	for _, version := range []string{"3.0", "2.0"} {
		t.Run(version, func(t *testing.T) {
			n := navigationFixture(t, testfixture.NavigationEPUB(version))
			if n.Status != "complete" || len(n.Diagnostics) != 0 || len(n.Entries) != 2 || n.Entries[1].Label != "First" || len(n.Entries[1].Children) != 0 {
				t.Fatalf("wrong tree: %+v", n)
			}
			second := n.Entries[0]
			if version == "3.0" {
				if n.Format != "epub3-nav" || n.Source != "书/nav.xhtml" || second.Label != "Part A" || second.Href != nil || second.Target != nil || len(second.Children) != 1 {
					t.Fatalf("group/nav selection: %+v", n)
				}
				second = second.Children[0]
			} else if n.Format != "epub2-ncx" || n.Source != "书/toc.ncx" || len(second.Children[0].Children) != 1 || second.Children[0].Children[0].Label != "Deep" {
				t.Fatalf("NCX selection/depth: %+v", n)
			}
			if second.Label != "Second & final" || second.Href == nil || *second.Href != "Text/第二%20章.xhtml?from=toc#note" || second.Target.Path != "书/Text/第二 章.xhtml" || second.Target.Fragment != "note" || second.Target.Query != "from=toc" || second.Exists == nil || !*second.Exists || len(second.Children) != 1 || second.Children[0].Label != "Return" {
				t.Fatalf("lost href/label/hierarchy: %+v", second)
			}
		})
	}
}

func TestNavigationDiagnostics(t *testing.T) {
	for _, tc := range []struct{ name, old, replacement, status, code string }{
		{"missing target", "Text/-first.xhtml#start", "absent.xhtml#no", "partial", "MISSING_NAVIGATION_TARGET"},
		{"traversal", "Text/-first.xhtml#start", "../../escape.xhtml", "partial", "INVALID_NAVIGATION_HREF"},
		{"wrong type namespace", `e:type="toc"`, `type="toc"`, "blocked", "NAVIGATION_STRUCTURE"},
		{"wrong token", `e:type="toc"`, `e:type="toc-other"`, "blocked", "NAVIGATION_STRUCTURE"},
		{"base", "<body>", `<head><base href="elsewhere/"/></head><body>`, "blocked", "NAVIGATION_STRUCTURE"},
		{"bad XML", "</body>", "</broken>", "blocked", "XML_NOT_WELL_FORMED"},
		{"DTD", "<html", `<!DOCTYPE html SYSTEM "https://example.invalid/remote.dtd"><html`, "blocked", "XML_POLICY"},
		{"xml base", "<body>", `<body xml:base="Text/">`, "blocked", "UNSUPPORTED_XML_BASE"},
		{"empty label", ">Part A</span>", "></span>", "partial", "NAVIGATION_STRUCTURE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := testfixture.NavigationEPUB("3.0")
			entries[5].Data = []byte(strings.Replace(string(entries[5].Data), tc.old, tc.replacement, 1))
			n := navigationFixture(t, entries)
			if n.Status != tc.status {
				t.Fatalf("status %s: %+v", n.Status, n)
			}
			found := false
			for _, d := range n.Diagnostics {
				if d.Code == tc.code && d.BookPath == "书/nav.xhtml" {
					found = true
				}
			}
			if !found {
				t.Fatal("missing scoped diagnostic", n.Diagnostics)
			}
			if n.Status == "blocked" && len(n.Entries) != 0 {
				t.Fatal("blocked parse leaked partial tree", n)
			}
		})
	}
	for _, version := range []string{"3.0", "2.0"} {
		entries := testfixture.NavigationEPUB(version)
		if version == "3.0" {
			entries[2].Data = []byte(strings.Replace(string(entries[2].Data), `properties="nav"`, `properties="nav-other"`, 1))
		} else {
			entries[2].Data = []byte(strings.Replace(string(entries[2].Data), `toc="ncx"`, `toc="not-the-ncx"`, 1))
		}
		if n := navigationFixture(t, entries); n.Status != "blocked" || n.Diagnostics[0].Code != "NAVIGATION_SELECTION" {
			t.Fatalf("silently selected first navigation/spine: %+v", n)
		}
	}
}

func TestXMLMixedContentAndIndexBudget(t *testing.T) {
	e, err := parseXML([]byte(`<x>Before <b>bold <i>inner</i> end</b> after<a/>tail<a/></x>`))
	if err != nil || e.Text != "Before  aftertail" || e.Content != "Before bold inner end aftertail" || e.Children[2].Location != "/x[1]/a[2]" {
		t.Fatalf("mixed content/positions: %+v %v", e, err)
	}
	if _, err := parseXML([]byte(strings.Repeat("<x>", 40) + strings.Repeat("a", 1<<20) + strings.Repeat("</x>", 40))); err == nil {
		t.Fatal("aggregate index budget ignored")
	}
}

func TestNavigationMissingAmbiguousAndEmpty(t *testing.T) {
	for _, tc := range []struct{ name, status, code string }{
		{"missing", "blocked", "MISSING_RESOURCE"},
		{"ambiguous", "blocked", "NAVIGATION_SELECTION"},
		{"empty", "partial", "NAVIGATION_STRUCTURE"},
		{"wrong media", "blocked", "NAVIGATION_MEDIA_TYPE"},
	} {
		entries := testfixture.NavigationEPUB("3.0")
		switch tc.name {
		case "missing":
			entries = append(entries[:5], entries[6:]...)
		case "ambiguous":
			entries[2].Data = []byte(strings.Replace(string(entries[2].Data), `id="c1"`, `id="c1" properties="nav"`, 1))
		case "empty":
			entries[5].Data = []byte(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:e="http://www.idpf.org/2007/ops"><body><nav e:type="toc"><ol/></nav></body></html>`)
		case "wrong media":
			entries[2].Data = []byte(strings.Replace(string(entries[2].Data), `media-type="application/xhtml+xml" properties="nav"`, `media-type="application/x-dtbncx+xml" properties="nav"`, 1))
		}
		n := navigationFixture(t, entries)
		if n.Status != tc.status || len(n.Diagnostics) == 0 || n.Diagnostics[0].Code != tc.code {
			t.Fatalf("%s: %+v", tc.name, n)
		}
	}
}

func TestT1BNavigationPartialDoesNotEraseRawLabelsOrUnblockStructure(t *testing.T) {
	for _, bad := range []bool{false, true} {
		entries := testfixture.NavigationEPUB("3.0")
		raw := strings.Replace(string(entries[5].Data), ">First</a>", ">&unknown;</a>", 1)
		if bad {
			raw = strings.Replace(raw, `e:type="toc"`, `e:type="other"`, 1)
		}
		entries[5].Data = []byte(`<!DOCTYPE html [%unread;]>` + raw)
		n := navigationFixture(t, entries)
		if n.XMLCoverage == nil || len(n.XMLCoverage.Resources) != 1 || n.XMLCoverage.Resources[0].Status != "partial" || len(n.XMLCoverage.Resources[0].Unresolved) != 2 {
			t.Fatal("source uncertainty was suppressed", n)
		}
		if bad {
			if n.Status != "blocked" || len(n.Entries) != 0 {
				t.Fatal("XML partial must not unblock missing TOC structure", n)
			}
		} else if n.Status != "partial" || len(n.Entries) != 2 || n.Entries[1].Label != "&unknown;" {
			t.Fatal("unknown label must not become empty or a complete tree", n)
		}
	}
}
