package publication

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
)

const opfNS = "http://www.idpf.org/2007/opf"
const containerNS = "urn:oasis:names:tc:opendocument:xmlns:container"
const XMLLimit = 8 << 20

type Element struct {
	Name       xml.Name   `json:"name"`
	Attributes []xml.Attr `json:"attributes"`
	Text       string     `json:"text"`
	Children   []*Element `json:"children"`
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

// Only UTF-8 and XML's built-in entities are accepted. No CharsetReader,
// custom entity map, DTD, resolver or network client is installed.
func parseXML(b []byte) (*Element, error) {
	if !utf8.Valid(b) {
		return nil, fault.New(3, "UNSUPPORTED_XML_ENCODING", "M1-A supports UTF-8 XML only")
	}
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	d := xml.NewDecoder(bytes.NewReader(b))
	stack := []*Element{}
	var root *Element
	tokens := 0
	for {
		t, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fault.New(1, "XML_NOT_WELL_FORMED", "XML: %v", err)
		}
		tokens++
		if tokens > 200000 {
			return nil, fault.New(1, "XML_LIMIT", "too many XML tokens")
		}
		switch t := t.(type) {
		case xml.Directive:
			return nil, fault.New(1, "XML_DTD_FORBIDDEN", "DTD/directives are not supported")
		case xml.StartElement:
			if len(stack) >= 128 {
				return nil, fault.New(1, "XML_LIMIT", "XML depth exceeds 128")
			}
			seen := map[xml.Name]bool{}
			for _, a := range t.Attr {
				if seen[a.Name] {
					return nil, fault.New(1, "XML_NOT_WELL_FORMED", "duplicate attribute")
				}
				seen[a.Name] = true
				if a.Name.Space == "http://www.w3.org/XML/1998/namespace" && a.Name.Local == "base" {
					return nil, fault.New(3, "UNSUPPORTED_XML_BASE", "xml:base is not supported in M1-A")
				}
			}
			e := &Element{Name: t.Name, Attributes: t.Attr, Children: []*Element{}}
			if len(stack) == 0 {
				if root != nil {
					return nil, fault.New(1, "XML_NOT_WELL_FORMED", "multiple XML roots")
				}
				root = e
			} else {
				p := stack[len(stack)-1]
				p.Children = append(p.Children, e)
			}
			stack = append(stack, e)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 {
				if strings.TrimSpace(string(t)) != "" {
					return nil, fault.New(1, "XML_NOT_WELL_FORMED", "text outside XML root")
				}
			} else {
				stack[len(stack)-1].Text += string(t)
			}
		}
	}
	if root == nil || len(stack) != 0 {
		return nil, fault.New(1, "XML_NOT_WELL_FORMED", "missing/unfinished XML root")
	}
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

func Load(a *archive.Archive, selected string) (*Publication, error) {
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
	p := &Publication{Rootfiles: []Rootfile{}, Manifest: []Item{}, Spine: []Itemref{}, Limitations: []Limitation{{"READ_ONLY_PARTIAL", "Metadata/manifest/spine only; no conformance, navigation, references, rendering or editing verification"}}}
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
		_, exists := a.Files[bp]
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
	if _, ok := a.Files["META-INF/encryption.xml"]; ok {
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
	if _, ok := a.Files["META-INF/signatures.xml"]; ok {
		p.Limitations = append(p.Limitations, Limitation{"SIGNATURES", "Signatures preserved but not verified"})
	}
	return p, nil
}
