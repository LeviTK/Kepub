package publication

import (
	"encoding/xml"
	"errors"
	"slices"
	"strings"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

const XHTMLNamespace = "http://www.w3.org/1999/xhtml"
const NCXNamespace = "http://www.daisy.org/z3986/2005/ncx/"

// Diagnostic is a scoped structural observation, never a conformance verdict.
type Diagnostic struct {
	Source   string            `json:"source"`
	Code     string            `json:"code"`
	Severity string            `json:"severity"`
	BookPath bookpath.BookPath `json:"bookPath"`
	Location string            `json:"location"`
	Line     *int              `json:"line"`
	Column   *int              `json:"column"`
	Message  string            `json:"message"`
}

func DiagnosticFor(err error, p bookpath.BookPath, location string) Diagnostic {
	code := "IO_ERROR"
	var fe *fault.Error
	if errors.As(err, &fe) {
		code = fe.Code
	}
	return Diagnostic{Source: "kepub", Code: code, Severity: "error", BookPath: p, Location: location, Message: err.Error()}
}

type NavigationNode struct {
	Label    string              `json:"label"`
	Href     *bookpath.Href      `json:"href"` // nil for a grouping span, not an empty link.
	Target   *bookpath.Reference `json:"target"`
	Exists   *bool               `json:"exists"` // nil for groups/external/invalid URLs.
	Location string              `json:"location"`
	Children []NavigationNode    `json:"children"`
}

type Navigation struct {
	Format        string            `json:"format"`
	Source        bookpath.BookPath `json:"source"`
	Status        string            `json:"status"` // complete/partial/blocked within navigation only.
	ParserVersion int               `json:"parserVersion"`
	Entries       []NavigationNode  `json:"entries"`
	Diagnostics   []Diagnostic      `json:"diagnostics"`
	XMLCoverage   *XMLCoverage      `json:"xmlCoverage,omitempty"`
}

// LoadNavigation uses EPUB3's manifest nav property or EPUB2's spine toc ID.
// It neither synthesizes a TOC from spine nor changes the reading order.
func LoadNavigation(a *archive.Archive, p *Publication) Navigation {
	n := Navigation{Status: "complete", ParserVersion: 1, Entries: []NavigationNode{}, Diagnostics: []Diagnostic{}, XMLCoverage: p.XMLCoverage}
	var candidates []Item
	if p.Version == "3.0" {
		n.Format = "epub3-nav"
		for _, item := range p.Manifest {
			if slices.Contains(strings.Fields(item.Properties), "nav") {
				candidates = append(candidates, item)
			}
		}
	} else {
		n.Format = "epub2-ncx"
		toc := ""
		for _, attr := range p.SpineAttributes {
			if attr.Name == (xml.Name{Local: "toc"}) {
				toc = attr.Value
			}
		}
		for _, item := range p.Manifest {
			if toc != "" && item.ID == toc {
				candidates = append(candidates, item)
			}
		}
	}
	if len(candidates) != 1 {
		n.problem(p.Rootfile, "", "NAVIGATION_SELECTION", "expected exactly one declared navigation resource")
		n.Status = "blocked"
		return n
	}
	item := candidates[0]
	n.Source = item.Path
	expected := "application/xhtml+xml"
	if n.Format == "epub2-ncx" {
		expected = "application/x-dtbncx+xml"
	}
	if item.MediaType != expected {
		n.problem(item.Path, "", "NAVIGATION_MEDIA_TYPE", "declared navigation media type does not match its format")
		n.Status = "blocked"
		return n
	}
	root, err := ReadXML(a, item.Path, xmltext.Profile{Version: p.Version, MediaType: item.MediaType})
	if err != nil {
		n.Diagnostics = append(n.Diagnostics, DiagnosticFor(err, item.Path, ""))
		n.Status = "blocked"
		return n
	}
	n.XMLCoverage = n.XMLCoverage.Merge(root.XMLCoverage)
	if err := root.XMLDocument.RequireComplete(); err != nil {
		n.Diagnostics = append(n.Diagnostics, DiagnosticFor(err, item.Path, ""))
		n.Status = "partial"
	}
	if n.Format == "epub3-nav" {
		if root.Name != (xml.Name{Space: XHTMLNamespace, Local: "html"}) {
			n.problem(n.Source, root.Location, "NAVIGATION_STRUCTURE", "expected XHTML html root")
			n.Status = "blocked"
			return n
		}
		var tocs []*Element
		base := false
		var walk func(*Element)
		walk = func(e *Element) {
			types, _ := e.Attribute("http://www.idpf.org/2007/ops", "type")
			if e.Name.Space == XHTMLNamespace {
				if e.Name.Local == "nav" && slices.Contains(strings.Fields(types), "toc") {
					tocs = append(tocs, e)
				}
				if e.Name.Local == "base" {
					base = true
				}
			}
			for _, c := range e.Children {
				walk(c)
			}
		}
		walk(root)
		if base || len(tocs) != 1 {
			n.problem(n.Source, root.Location, "NAVIGATION_STRUCTURE", "expected one epub:type=toc nav and no unsupported HTML base element")
			n.Status = "blocked"
			return n
		}
		lists := tocs[0].children(XHTMLNamespace, "ol")
		if len(lists) != 1 {
			n.problem(n.Source, tocs[0].Location, "NAVIGATION_STRUCTURE", "TOC must contain one ordered list")
			n.Status = "blocked"
			return n
		}
		n.Entries = n.navList(a, lists[0])
	} else {
		if root.Name != (xml.Name{Space: NCXNamespace, Local: "ncx"}) || len(root.children(NCXNamespace, "navMap")) != 1 {
			n.problem(n.Source, root.Location, "NAVIGATION_STRUCTURE", "expected NCX root with one navMap")
			n.Status = "blocked"
			return n
		}
		n.Entries = n.ncxPoints(a, root.children(NCXNamespace, "navMap")[0])
	}
	return n
}

func (n *Navigation) problem(p bookpath.BookPath, location, code, message string) {
	n.Status = "partial"
	n.Diagnostics = append(n.Diagnostics, Diagnostic{Source: "kepub", Code: code, Severity: "error", BookPath: p, Location: location, Message: message})
}

func (n *Navigation) link(a *archive.Archive, node *NavigationNode, href string) {
	h := bookpath.Href(href)
	node.Href = &h
	resolvedHref := h
	if n.Format == "epub3-nav" {
		// Preserve the attribute while applying HTML's peripheral ASCII space rule.
		resolvedHref = bookpath.Href(strings.Trim(href, " \t\n\r\f"))
	}
	r, err := bookpath.ResolveReference(n.Source, resolvedHref)
	if err != nil {
		n.problem(n.Source, node.Location, "INVALID_NAVIGATION_HREF", err.Error())
		return
	}
	node.Target = &r
	if !r.External {
		_, exists := a.Files[r.Path]
		node.Exists = &exists
		if !exists {
			n.problem(n.Source, node.Location, "MISSING_NAVIGATION_TARGET", string(r.Path))
		}
	}
}

func (n *Navigation) navList(a *archive.Archive, ol *Element) []NavigationNode {
	nodes := []NavigationNode{}
	if len(ol.Children) == 0 || strings.TrimSpace(ol.Text) != "" {
		n.problem(n.Source, ol.Location, "NAVIGATION_STRUCTURE", "TOC list must contain items, not bare text")
	}
	for _, li := range ol.Children {
		if li.Name != (xml.Name{Space: XHTMLNamespace, Local: "li"}) {
			n.problem(n.Source, li.Location, "NAVIGATION_STRUCTURE", "unsupported child in TOC list")
			continue
		}
		node := NavigationNode{Location: li.Location, Children: []NavigationNode{}}
		labels := 0
		lists := 0
		unknownTarget := false
		for _, c := range li.Children {
			if c.Name.Space == XHTMLNamespace && (c.Name.Local == "a" || c.Name.Local == "span") {
				labels++
				if labels == 1 {
					node.Label = strings.Join(strings.Fields(c.Content), " ")
					if c.Name.Local == "a" {
						h, ok := c.Attribute("", "href")
						unknownTarget = !c.source.AttributeKnown(xml.Name{Local: "href"})
						// Coverage records unknown targets; neither a definite URL
						// nor a missing-link error can be derived from them.
						if !unknownTarget {
							if ok {
								n.link(a, &node, h)
							} else {
								n.problem(n.Source, c.Location, "NAVIGATION_STRUCTURE", "navigation anchor has no href")
							}
						}
					}
				}
			} else if c.Name == (xml.Name{Space: XHTMLNamespace, Local: "ol"}) {
				lists++
				node.Children = append(node.Children, n.navList(a, c)...)
			} else {
				n.problem(n.Source, c.Location, "NAVIGATION_STRUCTURE", "unsupported child in TOC item")
			}
		}
		if labels != 1 || lists > 1 || node.Label == "" || strings.TrimSpace(li.Text) != "" || (node.Href == nil && lists == 0 && !unknownTarget) {
			n.problem(n.Source, li.Location, "NAVIGATION_STRUCTURE", "TOC item requires one nonempty label and at most one nested list")
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func (n *Navigation) ncxPoints(a *archive.Archive, parent *Element) []NavigationNode {
	nodes := []NavigationNode{}
	for _, child := range parent.Children {
		if child.Name.Space != NCXNamespace || !slices.Contains([]string{"navPoint", "navLabel", "navInfo", "content"}, child.Name.Local) {
			n.problem(n.Source, child.Location, "NAVIGATION_STRUCTURE", "unsupported child in NCX navigation")
		}
	}
	if parent.Name.Local == "navMap" && len(parent.children(NCXNamespace, "navPoint")) == 0 {
		n.problem(n.Source, parent.Location, "NAVIGATION_STRUCTURE", "NCX navMap has no navPoint")
	}
	for _, point := range parent.children(NCXNamespace, "navPoint") {
		node := NavigationNode{Location: point.Location, Children: n.ncxPoints(a, point)}
		labels := point.children(NCXNamespace, "navLabel")
		links := point.children(NCXNamespace, "content")
		if len(labels) == 1 && len(labels[0].children(NCXNamespace, "text")) == 1 {
			node.Label = strings.Join(strings.Fields(labels[0].children(NCXNamespace, "text")[0].Content), " ")
		}
		if node.Label == "" || len(links) != 1 {
			n.problem(n.Source, point.Location, "NAVIGATION_STRUCTURE", "NCX navPoint requires one text label and content target")
		}
		if len(links) == 1 {
			if links[0].source.AttributeKnown(xml.Name{Local: "src"}) {
				if h, ok := links[0].Attribute("", "src"); ok {
					n.link(a, &node, h)
				} else {
					n.problem(n.Source, links[0].Location, "NAVIGATION_STRUCTURE", "NCX content has no src")
				}
			}
		}
		nodes = append(nodes, node)
	}
	return nodes
}
