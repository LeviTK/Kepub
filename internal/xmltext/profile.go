package xmltext

import (
	"encoding/xml"
	"strings"

	"github.com/LeviTK/Kepub/internal/fault"
)

// Profile comes from the effective OPF version and manifest media type, never
// from a DOCTYPE or filename. Empty version is only the bootstrap XML pass.
type Profile struct {
	Version   string
	MediaType string
}

func (d *Document) CheckProfile(p Profile) error {
	if p.Version == "" {
		return nil
	}
	if p.Version != "3.0" && p.Version != "2.0" {
		return fault.New(3, "UNSUPPORTED_PACKAGE_VERSION", "unsupported package version")
	}
	if d.dtd.externalEntity {
		if p.Version == "3.0" {
			return fault.New(1, "XML_POLICY", "EPUB3 prohibits external ENTITY declarations")
		}
		return fault.New(3, "UNSUPPORTED_XML_DTD", "EPUB2 external entities are outside the offline processing profile")
	}
	if d.dtd.externalSubset {
		if p.Version == "2.0" {
			return fault.New(3, "UNSUPPORTED_XML_DTD", "EPUB2 external DOCTYPE is outside the non-migration processing profile")
		}
		pub, sys := "", ""
		switch p.MediaType {
		case "application/x-dtbncx+xml":
			pub, sys = "-//NISO//DTD ncx 2005-1//EN", "http://www.daisy.org/z3986/2005/ncx-2005-1.dtd"
		case "image/svg+xml":
			pub, sys = "-//W3C//DTD SVG 1.1//EN", "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd"
		case "application/mathml+xml", "application/mathml-presentation+xml", "application/mathml-content+xml":
			pub, sys = "-//W3C//DTD MathML 3.0//EN", "http://www.w3.org/Math/DTD/mathml3/mathml3.dtd"
		}
		// XML §4.2.2 normalizes a legal PubidLiteral before matching, not
		// its stored/application-visible spelling or the system identifier.
		publicID := strings.Join(strings.FieldsFunc(d.dtd.publicID, func(r rune) bool {
			return r == ' ' || r == '\r' || r == '\n'
		}), " ")
		if pub == "" || publicID != pub || d.dtd.systemID != sys {
			return fault.New(1, "XML_POLICY", "DOCTYPE external identifier is not allowed for this EPUB3 manifest media type")
		}
	}
	for _, e := range d.Elements {
		if e.Name.Space == "http://www.w3.org/2001/XInclude" {
			if p.Version == "3.0" {
				return fault.New(1, "XML_POLICY", "EPUB3 prohibits XInclude")
			}
			return fault.New(3, "UNSUPPORTED_XINCLUDE", "XInclude is outside the offline processing profile")
		}
	}
	for _, e := range d.Elements {
		e.profile = p
	}
	return nil
}

func (d *Document) PackageProfile() (Profile, error) {
	if d.Root.uncertainAttributes[xml.Name{Local: "version"}] {
		return Profile{}, fault.New(3, "XML_ENTITY_UNRESOLVED", "package identity depends on unresolved XML declarations or references")
	}
	if d.Root.Name != (xml.Name{Space: "http://www.idpf.org/2007/opf", Local: "package"}) {
		if d.UnknownDefaults {
			return Profile{}, fault.New(3, "XML_ENTITY_UNRESOLVED", "package namespace is unknown after unread declarations")
		}
		return Profile{}, fault.New(1, "INVALID_OPF", "unexpected OPF package root/namespace")
	}
	p := Profile{MediaType: "application/oebps-package+xml"}
	for _, a := range d.Root.Attributes {
		if a.Name == (xml.Name{Local: "version"}) {
			p.Version = a.Value
		}
	}
	if p.Version == "" && (d.dtd.unreadPE || d.dtd.externalSubset) {
		return p, fault.New(3, "XML_ENTITY_UNRESOLVED", "package version is unknown after unread XML declarations")
	}
	if p.Version != "2.0" && p.Version != "3.0" {
		return p, fault.New(3, "UNSUPPORTED_EPUB_VERSION", "unsupported package version %q", p.Version)
	}
	return p, d.CheckProfile(p)
}

func (d *Document) RequireComplete() error {
	if len(d.Unresolved) != 0 {
		return fault.New(3, "XML_ENTITY_UNRESOLVED", "XML text or attributes depend on unresolved entities/declarations")
	}
	return nil
}
