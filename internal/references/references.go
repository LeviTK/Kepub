// Package references builds a read-only, explicitly scoped reference index.
// Coverage describes extraction, not EPUB conformance or permission to edit.
package references

import (
	"encoding/xml"
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
)

const ParserVersion = 1
const opfNS = "http://www.idpf.org/2007/opf"
const svgNS = "http://www.w3.org/2000/svg"
const xlinkNS = "http://www.w3.org/1999/xlink"

type Edge struct {
	Source         bookpath.BookPath   `json:"source"`
	Location       string              `json:"location"`
	Syntax         string              `json:"syntax"`
	Href           bookpath.Href       `json:"href"`
	Target         *bookpath.Reference `json:"target"`
	Status         string              `json:"status"` // resolved/missing/invalid/external.
	FragmentStatus string              `json:"fragmentStatus"`
	ParserVersion  int                 `json:"parserVersion"`
}

type Coverage struct {
	Resource      bookpath.BookPath `json:"resource"`
	Syntax        string            `json:"syntax"`
	Status        string            `json:"status"` // complete/partial/blocked.
	Reasons       []string          `json:"reasons"`
	ParserVersion int               `json:"parserVersion"`
}

type Graph struct {
	Status        string                   `json:"status"`
	Scope         string                   `json:"scope"`
	Resource      bookpath.BookPath        `json:"resource"`
	Direction     string                   `json:"direction"`
	Edges         []Edge                   `json:"edges"`
	Coverage      []Coverage               `json:"coverage"`
	Diagnostics   []publication.Diagnostic `json:"diagnostics"`
	ParserVersion int                      `json:"parserVersion"`
}

type builder struct {
	a       *archive.Archive
	p       *publication.Publication
	g       Graph
	covered map[string]int
	ids     map[bookpath.BookPath]map[string]int
	items   map[string]publication.Item
}

// Build inspects the entire safe archive, including unmanifested resources.
// No remote target is fetched and no input resource is modified.
func Build(a *archive.Archive, p *publication.Publication) Graph {
	b := builder{a: a, p: p, covered: map[string]int{}, ids: map[bookpath.BookPath]map[string]int{}, items: map[string]publication.Item{}, g: Graph{
		Status: "complete", Scope: "archive resources; selected rootfile only; extraction is not conformance validation",
		Direction: "both", Edges: []Edge{}, Coverage: []Coverage{}, Diagnostics: []publication.Diagnostic{}, ParserVersion: ParserVersion,
	}}
	media := map[bookpath.BookPath]string{}
	for _, item := range p.Manifest {
		b.items[item.ID] = item
		if previous, ok := media[item.Path]; ok && previous != item.MediaType {
			b.cover(item.Path, "media-type", "blocked", "conflicting manifest media types")
		}
		media[item.Path] = item.MediaType
		if !item.Exists {
			b.cover(item.Path, "resource", "blocked", "manifest resource is missing")
		}
		if slices.Contains(strings.Fields(item.Properties), "scripted") {
			b.cover(item.Path, "script", "blocked", "manifest declares scripted content; dynamic references are not analyzed")
		}
	}
	paths := make([]bookpath.BookPath, 0, len(a.Files))
	for bp := range a.Files {
		paths = append(paths, bp)
	}
	slices.Sort(paths)
	for _, bp := range paths {
		if bp == "mimetype" {
			b.cover(bp, "container.mimetype", "complete", "fixed non-reference content")
			continue
		}
		if bp == "META-INF/container.xml" {
			for i, r := range p.Rootfiles {
				b.addTarget(bp, fmt.Sprintf("/container/rootfiles/rootfile[%d]/@full-path", i+1), "container.rootfile", bookpath.Href(r.Path), bookpath.Reference{Path: r.Path})
			}
			b.cover(bp, "container.rootfile", "complete", "rootfile full-path values (BookPaths, not relative URLs)")
			continue
		}
		kind := media[bp]
		if bp == p.Rootfile {
			kind = "application/oebps-package+xml"
		}
		if kind == "" {
			switch strings.ToLower(path.Ext(string(bp))) {
			case ".xhtml", ".html", ".htm":
				kind = "application/xhtml+xml"
			case ".svg":
				kind = "image/svg+xml"
			case ".css":
				kind = "text/css"
			case ".ncx":
				kind = "application/x-dtbncx+xml"
			}
			b.cover(bp, "unmanifested-resource", "partial", "media type is not declared in the selected manifest")
		}
		switch kind {
		case "application/xhtml+xml", "image/svg+xml", "application/x-dtbncx+xml", "application/oebps-package+xml":
			if kind == "application/oebps-package+xml" && bp != p.Rootfile {
				b.cover(bp, "opf", "blocked", "other package documents require explicit rootfile selection")
				continue
			}
			b.scanXML(bp, kind)
		case "text/css":
			data, err := a.Read(bp, publication.XMLLimit)
			if err != nil {
				b.block(bp, []string{"css.url", "css.import"}, err)
			} else {
				b.css(bp, "stylesheet", string(data))
			}
		case "image/png", "image/jpeg", "image/gif", "font/otf", "font/ttf", "font/woff", "font/woff2", "application/vnd.ms-opentype", "application/font-sfnt":
			b.cover(bp, "opaque-media", "complete", "no publication URL syntax in the declared leaf media type; binary validity is not checked")
		case "application/smil+xml":
			b.cover(bp, "smil", "blocked", "SMIL references are not implemented")
		case "application/javascript", "text/javascript":
			b.cover(bp, "script", "blocked", "dynamic script references are not analyzed or executed")
		default:
			b.cover(bp, "unknown-resource", "blocked", "reference syntax for this resource is not implemented")
		}
	}
	b.fragments()
	sort.Slice(b.g.Coverage, func(i, j int) bool {
		x, y := b.g.Coverage[i], b.g.Coverage[j]
		if x.Resource != y.Resource {
			return x.Resource < y.Resource
		}
		return x.Syntax < y.Syntax
	})
	for _, c := range b.g.Coverage {
		if c.Status != "complete" {
			b.g.Status = "partial"
		}
	}
	return b.g
}

// Filter limits edges only. Global coverage/diagnostics remain visible because
// unsupported sources can contain incoming links to any requested resource.
func (g Graph) Filter(resource, direction string) (Graph, error) {
	if direction != "" && direction != "incoming" && direction != "outgoing" {
		return Graph{}, fault.New(2, "INVALID_ARGUMENT", "direction must be incoming or outgoing")
	}
	if resource == "" {
		if direction != "" {
			return Graph{}, fault.New(2, "INVALID_ARGUMENT", "--direction requires --resource")
		}
		return g, nil
	}
	bp, err := bookpath.Parse(resource)
	if err != nil {
		return Graph{}, fault.New(2, "INVALID_ARGUMENT", "resource must be a canonical BookPath: %v", err)
	}
	g.Resource = bp
	if direction != "" {
		g.Direction = direction
	}
	edges := []Edge{}
	for _, edge := range g.Edges {
		out := edge.Source == bp
		in := edge.Target != nil && !edge.Target.External && edge.Target.Path == bp
		if (direction != "incoming" && out) || (direction != "outgoing" && in) {
			edges = append(edges, edge)
		}
	}
	g.Edges = edges
	return g, nil
}

func (b *builder) cover(bp bookpath.BookPath, syntax, status, reason string) {
	key := string(bp) + "\x00" + syntax
	i, ok := b.covered[key]
	if !ok {
		i = len(b.g.Coverage)
		b.covered[key] = i
		b.g.Coverage = append(b.g.Coverage, Coverage{Resource: bp, Syntax: syntax, Status: status, Reasons: []string{}, ParserVersion: ParserVersion})
	}
	c := &b.g.Coverage[i]
	if status == "blocked" || (status == "partial" && c.Status == "complete") {
		c.Status = status
	}
	if reason != "" && !slices.Contains(c.Reasons, reason) {
		c.Reasons = append(c.Reasons, reason)
	}
}

func (b *builder) block(bp bookpath.BookPath, syntaxes []string, err error) {
	for _, syntax := range syntaxes {
		b.cover(bp, syntax, "blocked", err.Error())
	}
	b.g.Diagnostics = append(b.g.Diagnostics, publication.DiagnosticFor(err, bp, ""))
}

func (b *builder) diagnostic(bp bookpath.BookPath, location, code, message string) {
	b.g.Diagnostics = append(b.g.Diagnostics, publication.Diagnostic{Source: "kepub", Code: code, Severity: "error", BookPath: bp, Location: location, Message: message})
}

func (b *builder) add(bp bookpath.BookPath, location, syntax, href string) {
	if strings.HasPrefix(strings.ToLower(href), "data:") {
		b.cover(bp, "embedded-data", "partial", "data URL payloads are not recursively analyzed")
	}
	r, err := bookpath.ResolveReference(bp, bookpath.Href(href))
	if err != nil {
		b.g.Edges = append(b.g.Edges, Edge{Source: bp, Location: location, Syntax: syntax, Href: bookpath.Href(href), Status: "invalid", FragmentStatus: "blocked", ParserVersion: ParserVersion})
		b.diagnostic(bp, location, "INVALID_REFERENCE", err.Error())
		return
	}
	b.addTarget(bp, location, syntax, bookpath.Href(href), r)
}

func (b *builder) addTarget(bp bookpath.BookPath, location, syntax string, href bookpath.Href, r bookpath.Reference) {
	e := Edge{Source: bp, Location: location, Syntax: syntax, Href: href, Target: &r, Status: "resolved", FragmentStatus: "not_applicable", ParserVersion: ParserVersion}
	if r.External {
		e.Status = "external"
		e.FragmentStatus = "not_checked"
	} else if _, ok := b.a.Files[r.Path]; !ok {
		e.Status = "missing"
		e.FragmentStatus = "blocked"
		b.diagnostic(bp, location, "MISSING_REFERENCE_TARGET", string(r.Path))
	} else if r.Fragment != "" {
		e.FragmentStatus = "not_checked"
	}
	b.g.Edges = append(b.g.Edges, e)
}

func (b *builder) fragments() {
	for i := range b.g.Edges {
		e := &b.g.Edges[i]
		if e.Status != "resolved" || e.Target.Fragment == "" {
			continue
		}
		ids, parsed := b.ids[e.Target.Path]
		if !parsed || strings.ContainsAny(e.Target.Fragment, "()") {
			b.cover(e.Source, "fragment", "partial", "target ID index unavailable or non-ID fragment syntax unsupported")
			continue
		}
		b.cover(e.Source, "fragment", "complete", "plain XML id/xml:id lookup; not CFI, XPointer or media-fragment validation")
		switch ids[e.Target.Fragment] {
		case 0:
			e.FragmentStatus = "missing"
			b.diagnostic(e.Source, e.Location, "MISSING_FRAGMENT", string(e.Target.Path)+"#"+e.Target.Fragment)
		case 1:
			e.FragmentStatus = "resolved"
		default:
			e.FragmentStatus = "ambiguous"
			b.diagnostic(e.Source, e.Location, "AMBIGUOUS_FRAGMENT", string(e.Target.Path)+"#"+e.Target.Fragment)
		}
	}
}

func (b *builder) scanXML(bp bookpath.BookPath, kind string) {
	syntaxes := []string{"xml.ids"}
	expected := xml.Name{}
	switch kind {
	case "application/xhtml+xml":
		syntaxes = append(syntaxes, "xhtml.href", "xhtml.src", "nav.href", "inline-style")
		expected = xml.Name{Space: publication.XHTMLNamespace, Local: "html"}
	case "image/svg+xml":
		syntaxes = append(syntaxes, "svg.href", "inline-style")
		expected = xml.Name{Space: svgNS, Local: "svg"}
	case "application/x-dtbncx+xml":
		syntaxes = append(syntaxes, "ncx.src")
		expected = xml.Name{Space: publication.NCXNamespace, Local: "ncx"}
	case "application/oebps-package+xml":
		syntaxes = append(syntaxes, "opf.manifest", "opf.spine", "opf.href", "opf.idref")
		expected = xml.Name{Space: opfNS, Local: "package"}
	}
	root, err := publication.ReadXML(b.a, bp)
	if err != nil {
		b.block(bp, syntaxes, err)
		return
	}
	if root.Name != expected {
		b.block(bp, syntaxes, fault.New(1, "REFERENCE_XML_ROOT", "root/namespace does not match declared media type"))
		return
	}
	// HTML base changes all relative URLs, including URLs occurring before it.
	var hasBase func(*publication.Element) bool
	hasBase = func(e *publication.Element) bool {
		if e.Name == (xml.Name{Space: publication.XHTMLNamespace, Local: "base"}) {
			return true
		}
		for _, c := range e.Children {
			if hasBase(c) {
				return true
			}
		}
		return false
	}
	if hasBase(root) {
		b.block(bp, syntaxes, fault.New(3, "UNSUPPORTED_HTML_BASE", "HTML base URL resolution is not implemented"))
		return
	}
	if len(root.ProcessingInstructions) > 0 {
		b.cover(bp, "xml.processing-instruction", "blocked", "processing instruction references are not interpreted")
	}
	for _, s := range syntaxes {
		b.cover(bp, s, "complete", "")
	}
	b.ids[bp] = map[string]int{}
	b.walkXML(bp, root, false)
}

func (b *builder) walkXML(bp bookpath.BookPath, e *publication.Element, inNav bool) {
	ns, name := e.Name.Space, e.Name.Local
	if ns == publication.XHTMLNamespace && name == "nav" {
		inNav = true
	}
	if ns != publication.XHTMLNamespace && ns != svgNS && ns != opfNS && ns != publication.NCXNamespace && ns != "http://purl.org/dc/elements/1.1/" {
		b.cover(bp, "unknown-xml", "partial", "unknown XML vocabulary: "+ns)
	}
	if (ns == publication.XHTMLNamespace || ns == svgNS) && name == "script" {
		b.cover(bp, "script", "blocked", "script contents and dynamic references are not analyzed")
	}
	if ns == svgNS && (name == "animate" || name == "set" || name == "animateMotion" || name == "animateTransform") {
		b.cover(bp, "svg.animation", "blocked", "animated reference values are not analyzed")
	}
	if ns == publication.XHTMLNamespace && name == "meta" {
		if value, _ := e.Attribute("", "http-equiv"); strings.EqualFold(value, "refresh") {
			b.cover(bp, "xhtml.refresh", "blocked", "meta refresh target syntax is not implemented")
		}
	}
	if (ns == publication.XHTMLNamespace || ns == svgNS) && name == "style" {
		if typ, _ := e.Attribute("", "type"); typ != "" && typ != "text/css" {
			b.cover(bp, "inline-style", "blocked", "non-CSS style language")
		} else {
			b.css(bp, e.Location+"/text()", e.Content)
		}
	}
	elementIDs := map[string]bool{}
	for _, a := range e.Attributes {
		location := e.Location + "/@" + a.Name.Local
		if a.Name.Space == xlinkNS {
			location = e.Location + "/@xlink:" + a.Name.Local
		}
		if (a.Name.Space == "" || a.Name.Space == "http://www.w3.org/XML/1998/namespace") && a.Name.Local == "id" && !elementIDs[a.Value] {
			elementIDs[a.Value] = true
			b.ids[bp][a.Value]++
			if b.ids[bp][a.Value] > 1 {
				b.diagnostic(bp, location, "DUPLICATE_ID", a.Value)
			}
		}
		if a.Name.Space != "" && a.Name.Space != "xmlns" && a.Name.Space != "http://www.w3.org/XML/1998/namespace" && a.Name.Space != "http://www.idpf.org/2007/ops" && a.Name.Space != xlinkNS {
			b.cover(bp, "unknown-xml", "partial", "unknown namespaced attribute: "+a.Name.Space)
		}
		if ns == publication.XHTMLNamespace || ns == svgNS {
			if a.Name.Space == "" && (a.Name.Local == "srcset" || a.Name.Local == "imagesrcset") {
				b.cover(bp, "srcset", "blocked", "candidate URL lists are not implemented")
			}
			if a.Name.Space == "" && strings.HasPrefix(a.Name.Local, "on") {
				b.cover(bp, "script", "blocked", "event handler references are not analyzed")
			}
			if a.Name.Space == "" && a.Name.Local == "style" {
				b.css(bp, location, a.Value)
			}
			if ns == svgNS && a.Name.Space == "" && slices.Contains([]string{"fill", "stroke", "filter", "clip-path", "mask", "marker", "marker-start", "marker-mid", "marker-end", "cursor"}, a.Name.Local) {
				b.css(bp, location, a.Value)
			}
		}
		syntax := ""
		switch {
		case ns == publication.XHTMLNamespace && a.Name.Space == "" && a.Name.Local == "href":
			syntax = "xhtml.href"
			if inNav {
				syntax = "nav.href"
			}
		case ns == publication.XHTMLNamespace && a.Name.Space == "" && a.Name.Local == "src":
			syntax = "xhtml.src"
		case ns == svgNS && (a.Name.Space == "" || a.Name.Space == xlinkNS) && a.Name.Local == "href":
			syntax = "svg.href"
		case ns == publication.NCXNamespace && a.Name.Space == "" && a.Name.Local == "src":
			syntax = "ncx.src"
		case ns == opfNS && a.Name.Space == "" && a.Name.Local == "href":
			syntax = "opf.href"
			if name == "item" {
				syntax = "opf.manifest"
			}
		case ns == opfNS && a.Name.Space == "" && slices.Contains([]string{"idref", "toc", "fallback", "fallback-style", "media-overlay", "handler"}, a.Name.Local):
			syntax = "opf.idref"
			if name == "itemref" && a.Name.Local == "idref" {
				syntax = "opf.spine"
			}
			if item, found := b.items[a.Value]; found {
				r, _ := bookpath.ResolveReference(b.p.Rootfile, item.Href)
				b.addTarget(bp, location, syntax, bookpath.Href(a.Value), r)
			} else {
				b.g.Edges = append(b.g.Edges, Edge{Source: bp, Location: location, Syntax: syntax, Href: bookpath.Href(a.Value), Status: "invalid", FragmentStatus: "blocked", ParserVersion: ParserVersion})
				b.diagnostic(bp, location, "UNKNOWN_MANIFEST_ID", a.Value)
			}
			continue
		case ns == opfNS && a.Name.Space == "" && a.Name.Local == "refines":
			syntax = "opf.href"
		}
		if syntax != "" {
			b.cover(bp, syntax, "complete", "")
			b.add(bp, location, syntax, a.Value)
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(a.Value)), "javascript:") {
				b.cover(bp, "script", "blocked", "javascript URLs are not analyzed or executed")
			}
		} else if a.Name.Local == "href" || a.Name.Local == "src" {
			b.cover(bp, "unknown-url-attribute", "blocked", "href/src in an unsupported namespace or vocabulary")
		} else if a.Name.Space == "" && slices.Contains([]string{"poster", "data", "action", "formaction", "ping", "longdesc", "srcdoc", "archive", "codebase", "background", "profile"}, a.Name.Local) {
			b.cover(bp, "other-url-attribute", "blocked", "unsupported URL-bearing attribute: "+a.Name.Local)
		}
	}
	for _, child := range e.Children {
		b.walkXML(bp, child, inNav)
	}
}
