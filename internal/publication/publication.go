package publication

import (
	"encoding/xml"
	"errors"
	"strings"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

const opfNS = "http://www.idpf.org/2007/opf"
const containerNS = "urn:oasis:names:tc:opendocument:xmlns:container"
const XMLLimit = xmltext.Limit

type Element struct {
	Name                   xml.Name   `json:"name"`
	Attributes             []xml.Attr `json:"attributes"`
	Text                   string     `json:"text"`
	Children               []*Element `json:"children"`
	Content                string     `json:"-"` // Mixed text in document order, for navigation labels.
	Location               string     `json:"-"` // Structural position, not a writable byte offset.
	ProcessingInstructions []string   `json:"-"` // Document-level unknown reference syntax.
}

func (e *Element) Attribute(ns, name string) (string, bool) {
	for _, a := range e.Attributes {
		if a.Name.Space == ns && a.Name.Local == name {
			return a.Value, true
		}
	}
	return "", false
}

func (e *Element) attr(name string) string {
	for _, a := range e.Attributes {
		if a.Name.Space == "" && a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}
func (e *Element) children(ns, name string) []*Element {
	out := []*Element{}
	for _, c := range e.Children {
		if c.Name.Space == ns && c.Name.Local == name {
			out = append(out, c)
		}
	}
	return out
}
func one(e *Element, ns, name string) (*Element, error) {
	list := e.children(ns, name)
	if len(list) != 1 {
		return nil, fault.New(1, "INVALID_STRUCTURE", "expected exactly one %s", name)
	}
	return list[0], nil
}

// ReadXML shares the bounded, non-networked parser with read-only indexes.
func ReadXML(a *archive.Archive, p bookpath.BookPath) (*Element, error) {
	b, err := a.Read(p, XMLLimit)
	if err != nil {
		return nil, err
	}
	return parseXML(b)
}

// Read indexes and local edits share strict lexical/encoding validation.
// Preserve direct text in the old summary and aggregate text for content.
func parseXML(b []byte) (*Element, error) {
	doc, err := xmltext.Parse(b)
	if err != nil {
		var f *fault.Error
		if errors.As(err, &f) {
			return nil, err
		}
		return nil, fault.New(1, "XML_NOT_WELL_FORMED", "XML: %v", err)
	}
	var convert func(*xmltext.Element) *Element
	convert = func(e *xmltext.Element) *Element {
		out := &Element{Name: e.Name, Attributes: e.Attributes, Text: e.DirectText, Content: e.Text, Location: e.Location, Children: []*Element{}}
		for _, c := range e.Children {
			out.Children = append(out.Children, convert(c))
		}
		return out
	}
	root := convert(doc.Root)
	root.ProcessingInstructions = doc.ProcessingInstructions
	return root, nil
}

type Rootfile struct {
	Path      bookpath.BookPath `json:"bookPath"`
	MediaType string            `json:"mediaType"`
}
type Item struct {
	ID         string            `json:"id"`
	Href       bookpath.Href     `json:"href"`
	Path       bookpath.BookPath `json:"bookPath"`
	MediaType  string            `json:"mediaType"`
	Properties string            `json:"properties"`
	Attributes []xml.Attr        `json:"attributes"`
	Exists     bool              `json:"exists"`
}
type Itemref struct {
	IDRef      string     `json:"idref"`
	Linear     string     `json:"linear"`
	Attributes []xml.Attr `json:"attributes"`
}
type Limitation struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Publication struct {
	Rootfiles         []Rootfile        `json:"rootfiles"`
	Rootfile          bookpath.BookPath `json:"rootfile"`
	Version           string            `json:"version"`
	UniqueIdentifier  string            `json:"uniqueIdentifier"`
	PackageAttributes []xml.Attr        `json:"packageAttributes"`
	Metadata          []*Element        `json:"metadata"`
	Manifest          []Item            `json:"manifest"`
	Spine             []Itemref         `json:"spine"`
	SpineAttributes   []xml.Attr        `json:"spineAttributes"`
	Limitations       []Limitation      `json:"limitations"`
}

// ResourceReader supplies bounded bytes and the approved file inventory. Locked
// workspace history can use its root directly without copying an entire book.
type ResourceReader interface {
	Read(bookpath.BookPath, int64) ([]byte, error)
	HasFile(bookpath.BookPath) bool
}

func Load(a ResourceReader, selected string) (*Publication, error) {
	b, err := a.Read("META-INF/container.xml", XMLLimit)
	if err != nil {
		return nil, err
	}
	c, err := parseXML(b)
	if err != nil {
		return nil, err
	}
	if c.Name != (xml.Name{Space: containerNS, Local: "container"}) {
		return nil, fault.New(1, "INVALID_CONTAINER", "unexpected container root/namespace")
	}
	roots, err := one(c, containerNS, "rootfiles")
	if err != nil {
		return nil, err
	}
	p := &Publication{Rootfiles: []Rootfile{}, Manifest: []Item{}, Spine: []Itemref{}, Limitations: []Limitation{{"READ_ONLY_PARTIAL", "Read-only structure indexes; navigation/references require their inspect sections and coverage; no EPUB conformance, rendering or editing verification"}}}
	seen := map[bookpath.BookPath]bool{}
	for _, r := range roots.children(containerNS, "rootfile") {
		bp, e := bookpath.Parse(r.attr("full-path"))
		if e != nil {
			return nil, e
		}
		if seen[bp] {
			return nil, fault.New(1, "INVALID_CONTAINER", "duplicate rootfile")
		}
		seen[bp] = true
		p.Rootfiles = append(p.Rootfiles, Rootfile{bp, r.attr("media-type")})
	}
	if len(p.Rootfiles) == 0 {
		return nil, fault.New(1, "INVALID_CONTAINER", "no rootfile")
	}
	if selected == "" {
		if len(p.Rootfiles) != 1 {
			return nil, fault.New(2, "ROOTFILE_REQUIRED", "multiple rootfiles; specify --rootfile")
		}
		selected = string(p.Rootfiles[0].Path)
	}
	found := false
	for _, r := range p.Rootfiles {
		if string(r.Path) == selected {
			found = true
			p.Rootfile = r.Path
			if r.MediaType != "application/oebps-package+xml" {
				return nil, fault.New(3, "UNSUPPORTED_ROOTFILE", "rootfile media type unsupported")
			}
		}
	}
	if !found {
		return nil, fault.New(2, "INVALID_ROOTFILE", "selected rootfile is not in container.xml")
	}
	b, err = a.Read(p.Rootfile, XMLLimit)
	if err != nil {
		return nil, err
	}
	pkg, err := parseXML(b)
	if err != nil {
		return nil, err
	}
	if pkg.Name != (xml.Name{Space: opfNS, Local: "package"}) {
		return nil, fault.New(1, "INVALID_OPF", "unexpected package root/namespace")
	}
	p.Version = pkg.attr("version")
	if p.Version != "2.0" && p.Version != "3.0" {
		return nil, fault.New(3, "UNSUPPORTED_EPUB_VERSION", "unsupported package version %q", p.Version)
	}
	p.UniqueIdentifier = pkg.attr("unique-identifier")
	p.PackageAttributes = pkg.Attributes
	m, err := one(pkg, opfNS, "metadata")
	if err != nil {
		return nil, err
	}
	p.Metadata = m.Children
	manifest, err := one(pkg, opfNS, "manifest")
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, i := range manifest.children(opfNS, "item") {
		id := i.attr("id")
		if id == "" || ids[id] {
			return nil, fault.New(1, "INVALID_MANIFEST", "missing or duplicate manifest id")
		}
		ids[id] = true
		bp, e := bookpath.Resolve(p.Rootfile, bookpath.Href(i.attr("href")))
		if e != nil {
			return nil, e
		}
		exists := a.HasFile(bp)
		p.Manifest = append(p.Manifest, Item{id, bookpath.Href(i.attr("href")), bp, i.attr("media-type"), i.attr("properties"), i.Attributes, exists})
		if !exists {
			p.Limitations = append(p.Limitations, Limitation{"MISSING_MANIFEST_RESOURCE", string(bp)})
		}
	}
	spine, err := one(pkg, opfNS, "spine")
	if err != nil {
		return nil, err
	}
	p.SpineAttributes = spine.Attributes
	for _, i := range spine.children(opfNS, "itemref") {
		id := i.attr("idref")
		if !ids[id] {
			return nil, fault.New(1, "INVALID_SPINE", "unknown spine idref %q", id)
		}
		linear := i.attr("linear")
		if linear == "" {
			linear = "yes"
		}
		if linear != "yes" && linear != "no" {
			return nil, fault.New(1, "INVALID_SPINE", "invalid linear value")
		}
		p.Spine = append(p.Spine, Itemref{id, linear, i.Attributes})
	}
	// Report unimplemented structures without rewriting or discarding any resource.
	for _, i := range p.Manifest {
		if strings.Contains(i.Properties, "scripted") {
			p.Limitations = append(p.Limitations, Limitation{"SCRIPTED", "Book scripts are not executed or analyzed"})
		}
		if i.MediaType == "application/smil+xml" {
			p.Limitations = append(p.Limitations, Limitation{"MEDIA_OVERLAYS", "SMIL/media overlays are not interpreted"})
		}
		if strings.HasPrefix(i.MediaType, "audio/") || strings.HasPrefix(i.MediaType, "video/") {
			p.Limitations = append(p.Limitations, Limitation{"AUDIO_VIDEO", "Media bytes preserved; playback is not supported"})
		}
	}
	var scan func(*Element)
	scan = func(e *Element) {
		if e.Name.Local == "meta" && e.attr("property") == "rendition:layout" && strings.TrimSpace(e.Text) == "pre-paginated" {
			p.Limitations = append(p.Limitations, Limitation{"FIXED_LAYOUT", "Fixed layout is not rendered"})
		}
		for _, c := range e.Children {
			scan(c)
		}
	}
	scan(pkg)
	if a.HasFile("META-INF/encryption.xml") {
		b, e := a.Read("META-INF/encryption.xml", XMLLimit)
		if e != nil {
			return nil, e
		}
		enc, e := parseXML(b)
		if e != nil {
			return nil, e
		}
		p.Limitations = append(p.Limitations, Limitation{"ENCRYPTION_DECLARED", "Encryption/obfuscation declarations retained; no decryption or editing supported; presence alone does not establish DRM"})
		var algorithms func(*Element)
		algorithms = func(e *Element) {
			if e.Name.Local == "EncryptionMethod" {
				p.Limitations = append(p.Limitations, Limitation{"ENCRYPTION_ALGORITHM", e.attr("Algorithm")})
			}
			for _, c := range e.Children {
				algorithms(c)
			}
		}
		algorithms(enc)
	}
	if a.HasFile("META-INF/signatures.xml") {
		p.Limitations = append(p.Limitations, Limitation{"SIGNATURES", "Signatures preserved but not verified"})
	}
	return p, nil
}
