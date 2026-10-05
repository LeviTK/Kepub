// Package metadata implements a bounded byte-preserving metadata.set v1.
package metadata

import (
	"encoding/xml"
	"fmt"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/xmltext"
)

const DC = "http://purl.org/dc/elements/1.1/"
const opf = "http://www.idpf.org/2007/opf"

type Set struct {
	Namespace        string `json:"namespace"`
	LocalName        string `json:"localName"`
	ID               string `json:"id,omitempty"`
	ExpectedOldValue string `json:"expectedOldValue"`
	NewValue         string `json:"newValue"`
}

// Apply selects a unique direct metadata child. The shared strict byte index
// rejects complex/self-closing targets, even for no-op, and verifies replacement.
func Apply(input []byte, request Set) ([]byte, bool, error) {
	if request.Namespace != DC || (request.LocalName != "title" && request.LocalName != "creator") {
		return nil, false, fmt.Errorf("unsupported metadata field")
	}
	if len(request.NewValue) > xmltext.Limit || !utf8.ValidString(request.NewValue) {
		return nil, false, fmt.Errorf("XML size/UTF-8 limit")
	}
	doc, err := xmltext.Parse(input)
	if err != nil {
		return nil, false, err
	}
	root := doc.Root
	if root.Name != (xml.Name{Space: opf, Local: "package"}) {
		return nil, false, fmt.Errorf("expected single OPF package")
	}
	version := ""
	for _, a := range root.Attributes {
		if a.Name == (xml.Name{Local: "version"}) {
			version = a.Value
		}
	}
	if version != "2.0" && version != "3.0" {
		return nil, false, fmt.Errorf("unsupported package version")
	}
	metas := 0
	var selected []*xmltext.Element
	ids := map[string]int{}
	for _, e := range doc.Elements {
		if e.ID != "" {
			ids[e.ID]++
			if ids[e.ID] > 1 {
				return nil, false, fmt.Errorf("duplicate id %q", e.ID)
			}
		}
		if e.Parent == root && e.Name == (xml.Name{Space: opf, Local: "metadata"}) {
			metas++
		}
		if e.Parent != nil && e.Parent.Parent == root && e.Parent.Name == (xml.Name{Space: opf, Local: "metadata"}) && e.Name == (xml.Name{Space: request.Namespace, Local: request.LocalName}) && (request.ID == "" || request.ID == e.ID) {
			selected = append(selected, e)
		}
	}
	if metas != 1 || len(selected) != 1 {
		return nil, false, fmt.Errorf("missing or ambiguous metadata target")
	}
	return xmltext.Replace(input, selected[0], request.ExpectedOldValue, request.NewValue)
}
