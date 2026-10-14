package metadata

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

// SetV2 binds selection to an accepted OPF. The transaction owner verifies the
// revision, resource digest, rootfile and expected old value against its snapshot.
type SetV2 struct {
	BookPath         bookpath.BookPath `json:"bookPath"`
	RevisionID       string            `json:"revisionId"`
	ResourceSHA256   string            `json:"resourceSha256"`
	Namespace        string            `json:"namespace"`
	LocalName        string            `json:"localName"`
	ID               string            `json:"id,omitempty"`
	Property         string            `json:"property,omitempty"`
	ExpectedOldValue string            `json:"expectedOldValue"`
	NewValue         string            `json:"newValue"`
}

var digestV2 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// RFC 5646 section 2.1, deliberately without registry or singleton-uniqueness
// constraints. RFC 5234 literals are ASCII case insensitive.
var languageV2 = regexp.MustCompile(`(?i)^(?:[a-z]{2,3}(?:-[a-z]{3}){0,3}|[a-z]{4}|[a-z]{5,8})(?:-[a-z]{4})?(?:-[a-z]{2}|-[0-9]{3})?(?:-[a-z0-9]{5,8}|-[0-9][a-z0-9]{3})*(?:-[0-9a-wy-z](?:-[a-z0-9]{2,8})+)*(?:-x(?:-[a-z0-9]{1,8})+)?$|(?i)^x(?:-[a-z0-9]{1,8})+$`)

const grandfatheredV2 = "en-gb-oed i-ami i-bnn i-default i-enochian i-hak i-klingon i-lux i-mingo i-navajo i-pwn i-tao i-tay i-tsu sgn-be-fr sgn-be-nl sgn-ch-de art-lojban cel-gaulish no-bok no-nyn zh-guoyu zh-hakka zh-min zh-min-nan zh-xiang"

// Validate checks operation syntax and only the new value's field constraints.
func (r SetV2) Validate() error {
	if _, err := bookpath.Parse(string(r.BookPath)); err != nil {
		return fmt.Errorf("invalid metadata bookPath: %v", err)
	}
	if r.RevisionID == "" || !digestV2.MatchString(r.ResourceSHA256) {
		return fmt.Errorf("invalid metadata revision/hash")
	}
	modified := r.Namespace == opf && r.LocalName == "meta" && r.Property == "dcterms:modified"
	if !modified && (r.Namespace != DC || r.Property != "" || (r.LocalName != "title" && r.LocalName != "creator" && r.LocalName != "language" && r.LocalName != "identifier")) {
		return fmt.Errorf("unsupported metadata field")
	}
	if len(r.NewValue) > xmltext.Limit || !validXMLV2(r.NewValue) {
		return fmt.Errorf("invalid XML text")
	}
	if modified {
		const layout = "2006-01-02T15:04:05Z"
		t, err := time.Parse(layout, r.NewValue)
		if err != nil || t.Year() < 1 || t.Year() > 9999 || t.Format(layout) != r.NewValue {
			return fmt.Errorf("invalid modified UTC date")
		}
	} else if r.LocalName == "language" {
		for _, c := range r.NewValue {
			if c > 127 || c == ' ' {
				return fmt.Errorf("invalid language tag")
			}
		}
		if !languageV2.MatchString(r.NewValue) && !strings.Contains(" "+grandfatheredV2+" ", " "+strings.ToLower(r.NewValue)+" ") {
			return fmt.Errorf("invalid language tag")
		}
	} else if strings.TrimSpace(r.NewValue) == "" {
		return fmt.Errorf("empty metadata value")
	}
	return nil
}

func validXMLV2(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, c := range s {
		if !(c == 9 || c == 10 || c == 13 || c >= 0x20 && c <= 0xd7ff || c >= 0xe000 && c <= 0xfffd || c >= 0x10000 && c <= 0x10ffff) {
			return false
		}
	}
	return true
}

// selectionAttributeV2 distinguishes proven absence, literal effective values,
// unread declarations and known defaults. Read-only selection is not write authority.
func selectionAttributeV2(e *xmltext.Element, name string) (string, bool, error) {
	n := xml.Name{Local: name}
	if !e.AttributeKnown(n) {
		return "", false, fault.New(3, "XML_ENTITY_UNRESOLVED", "unknown selection attribute %s", name)
	}
	for _, a := range e.Attributes {
		if a.Name == n {
			if _, ok := e.AttributeBytes(n); !ok {
				return "", false, fault.New(3, "UNSUPPORTED_INPUT", "nonliteral selection attribute %s", name)
			}
			return a.Value, true, nil
		}
	}
	return "", false, nil
}

// SelectV2 selects from original bytes, including all provenance and no-op
// gates, but intentionally does not compare ExpectedOldValue or snapshot bindings.
func SelectV2(input []byte, r SetV2) (*xmltext.Element, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	d, err := xmltext.Parse(input)
	if err != nil {
		return nil, err
	}
	attrs := map[string]string{}
	for _, name := range []string{"version", "prefix", "unique-identifier"} {
		v, present, err := selectionAttributeV2(d.Root, name)
		if err != nil {
			return nil, err
		}
		if r.Namespace == opf && name == "prefix" && present && v == "" {
			return nil, fmt.Errorf("empty package prefix")
		}
		attrs[name] = v
	}
	if attrs["version"] != "3.0" {
		return nil, fault.New(3, "UNSUPPORTED_INPUT", "metadata v2 requires EPUB3")
	}
	if _, err := d.PackageProfile(); err != nil {
		return nil, err
	}
	if err := d.RequireComplete(); err != nil {
		return nil, err
	}
	var meta *xmltext.Element
	ids := map[string]bool{}
	for _, e := range d.Elements {
		id, _, err := selectionAttributeV2(e, "id")
		if err != nil {
			return nil, err
		}
		if id != "" {
			if ids[id] {
				return nil, fmt.Errorf("duplicate id %q", id)
			}
			ids[id] = true
		}
		if e.Parent == d.Root && e.Name == (xml.Name{Space: opf, Local: "metadata"}) {
			if meta != nil {
				return nil, fmt.Errorf("ambiguous publication metadata")
			}
			meta = e
		}
	}
	if meta == nil {
		return nil, fmt.Errorf("missing publication metadata")
	}
	var selected []*xmltext.Element
	if r.Namespace == opf {
		selected, err = modifiedTargetsV2(d, meta, attrs["prefix"])
		if err != nil {
			return nil, err
		}
		if len(selected) == 1 && r.ID != "" && selected[0].ID != r.ID {
			return nil, fmt.Errorf("modified id mismatch")
		}
	} else {
		for _, e := range meta.Children {
			if e.Name == (xml.Name{Space: r.Namespace, Local: r.LocalName}) && (r.ID == "" || r.ID == e.ID) {
				selected = append(selected, e)
			}
		}
	}
	if len(selected) != 1 {
		return nil, fmt.Errorf("missing or ambiguous metadata target")
	}
	e := selected[0]
	if r.LocalName == "identifier" {
		var primary *xmltext.Element
		for _, c := range meta.Children {
			if c.Name == (xml.Name{Space: DC, Local: "identifier"}) && attrs["unique-identifier"] != "" && c.ID == attrs["unique-identifier"] {
				primary = c
			}
		}
		if primary == nil {
			return nil, fmt.Errorf("invalid primary identifier reference")
		}
		ncx := false
		for _, c := range d.Elements {
			name := ""
			if c.Parent == d.Root && c.Name == (xml.Name{Space: opf, Local: "spine"}) {
				name = "toc"
			}
			if c.Parent != nil && c.Parent.Parent == d.Root && c.Parent.Name == (xml.Name{Space: opf, Local: "manifest"}) && c.Name == (xml.Name{Space: opf, Local: "item"}) {
				name = "media-type"
			}
			if name != "" {
				v, present, err := selectionAttributeV2(c, name)
				if err != nil {
					return nil, fault.New(3, "UNSUPPORTED_INPUT", "unknown NCX dependency: %v", err)
				}
				ncx = ncx || name == "toc" && present || name == "media-type" && v == "application/x-dtbncx+xml"
			}
		}
		if ncx && primary == e {
			return nil, fault.New(3, "UNSUPPORTED_INPUT", "NCX primary identifier is not writable")
		}
	}
	if e.Complex || e.SelfClosing || e.ContentUnknown {
		return nil, fault.New(3, "UNSUPPORTED_INPUT", "metadata target is not writable simple text")
	}
	if _, _, ok := e.PhysicalContent(); !ok {
		return nil, fault.New(3, "UNSUPPORTED_INPUT", "metadata target lacks physical content")
	}
	// A simple element can still contain general-entity expansion, including
	// zero-width generated text. Inspect original syntax, not decoded equality.
	literal, err := e.DecodeMarkup(input[e.Start:e.End])
	if err != nil {
		return nil, err
	}
	for rest := literal; ; {
		i := strings.IndexByte(rest, '&')
		if i < 0 {
			break
		}
		rest = rest[i+1:]
		end := strings.IndexByte(rest, ';')
		if end < 0 {
			return nil, fault.New(3, "UNSUPPORTED_INPUT", "nonliteral metadata text")
		}
		name := rest[:end]
		if name != "amp" && name != "lt" && name != "gt" && name != "apos" && name != "quot" && !strings.HasPrefix(name, "#") {
			return nil, fault.New(3, "UNSUPPORTED_INPUT", "generated metadata text")
		}
		rest = rest[end+1:]
	}
	return e, nil
}

const termsV2 = "http://purl.org/dc/terms/"

func modifiedTargetsV2(d *xmltext.Document, meta *xmltext.Element, prefix string) ([]*xmltext.Element, error) {
	vocab := map[string]string{"a11y": "http://www.idpf.org/epub/vocab/package/a11y/#", "dcterms": termsV2, "marc": "http://id.loc.gov/vocabulary/", "media": "http://www.idpf.org/epub/vocab/overlays/#", "onix": "http://www.editeur.org/ONIX/book/codelists/current.html#", "rendition": "http://www.idpf.org/vocab/rendition/#", "schema": "http://schema.org/", "xsd": "http://www.w3.org/2001/XMLSchema#"}
	seen := map[string]bool{}
	for prefix != "" {
		i := strings.IndexByte(prefix, ':')
		if i < 1 || !xmltext.ValidNCName(prefix[:i]) || i+1 >= len(prefix) || prefix[i+1] != ' ' {
			return nil, fmt.Errorf("malformed package prefix")
		}
		name := prefix[:i]
		rest := strings.TrimLeft(prefix[i+1:], " ")
		end := strings.IndexAny(rest, " \t\r\n")
		uri := rest
		prefix = ""
		if end >= 0 {
			uri = rest[:end]
			prefix = strings.TrimLeft(rest[end:], " \t\r\n")
			if prefix == "" {
				return nil, fmt.Errorf("trailing prefix whitespace")
			}
		}
		if uri == "" || name == "dcterms" || uri == termsV2 || seen[name] {
			return nil, fmt.Errorf("invalid or aliased vocabulary declaration")
		}
		if _, err := url.Parse(uri); err != nil {
			return nil, fmt.Errorf("invalid vocabulary URI")
		}
		seen[name] = true
		vocab[name] = uri
	}
	var targets []*xmltext.Element
	for _, e := range d.Elements {
		inside := false
		for p := e.Parent; p != nil; p = p.Parent {
			if p == meta {
				inside = true
				break
			}
		}
		if !inside || e.Name != (xml.Name{Space: opf, Local: "meta"}) {
			continue
		}
		property, present, err := selectionAttributeV2(e, "property")
		if err != nil {
			return nil, err
		}
		refines, refined, err := selectionAttributeV2(e, "refines")
		if err != nil {
			return nil, err
		}
		if refined && strings.Trim(refines, " \t\r\n") == "" {
			return nil, fmt.Errorf("empty refines")
		}
		if !present {
			continue
		}
		p := strings.Trim(property, " \t\r\n")
		base := "http://idpf.org/epub/vocab/package/meta/#"
		ref := p
		if i := strings.IndexByte(p, ':'); i >= 0 {
			var ok bool
			base, ok = vocab[p[:i]]
			if !ok {
				return nil, fmt.Errorf("undeclared property prefix")
			}
			ref = p[i+1:]
		}
		u, err := url.Parse(ref)
		if err != nil || ref == "" || strings.ContainsAny(ref, " \t\r\n") || u.IsAbs() || u.Host != "" || strings.HasPrefix(ref, "/") {
			return nil, fmt.Errorf("invalid metadata property")
		}
		if base+ref == termsV2+"modified" && !refined {
			if e.Parent != meta || property != "dcterms:modified" {
				return nil, fmt.Errorf("noncanonical publication modified")
			}
			targets = append(targets, e)
		}
	}
	return targets, nil
}
