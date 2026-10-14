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

// Selector owns one parsed frozen resource for this call. Selection rules are
// still checked for every operation; the index is never a cross-call cache.
type Selector struct {
	input []byte
	doc   *xmltext.Document
}

func NewSelector(input []byte) (*Selector, error) {
	doc, err := xmltext.Parse(input)
	if err != nil {
		return nil, err
	}
	return &Selector{input: input, doc: doc}, nil
}

// Apply selects a unique direct metadata child. The shared strict byte index
// rejects complex/self-closing targets, even for no-op, and verifies replacement.
func Apply(input []byte, request Set) ([]byte, bool, error) {
	if err := validate(request); err != nil {
		return nil, false, err
	}
	e, err := selectTarget(input, request)
	if err != nil {
		return nil, false, err
	}
	return xmltext.Replace(input, e, request.ExpectedOldValue, request.NewValue)
}

// Select returns the unique target element's structural location in the frozen
// input without changing it. It applies the same selection rules as Apply, so a
// multi-operation plan can detect duplicate and aliased targets.
func Select(input []byte, request Set) (string, error) {
	if err := validate(request); err != nil {
		return "", err
	}
	e, err := selectTarget(input, request)
	if err != nil {
		return "", err
	}
	return e.Location, nil
}

func validate(request Set) error {
	if request.Namespace != DC || (request.LocalName != "title" && request.LocalName != "creator") {
		return fmt.Errorf("unsupported metadata field")
	}
	if len(request.NewValue) > xmltext.Limit || !utf8.ValidString(request.NewValue) {
		return fmt.Errorf("XML size/UTF-8 limit")
	}
	return nil
}

func selectTarget(input []byte, request Set) (*xmltext.Element, error) {
	s, err := NewSelector(input)
	if err != nil {
		return nil, err
	}
	return s.Select(request)
}

// Select applies v1's unchanged authorization to this frozen resource.
func (s *Selector) Select(request Set) (*xmltext.Element, error) {
	if err := validate(request); err != nil {
		return nil, err
	}
	doc := s.doc
	if _, err := doc.PackageProfile(); err != nil {
		return nil, err
	}
	if err := doc.RequireComplete(); err != nil {
		return nil, err
	}
	root := doc.Root
	metas := 0
	var selected []*xmltext.Element
	ids := map[string]int{}
	for _, e := range doc.Elements {
		if e.ID != "" {
			ids[e.ID]++
			if ids[e.ID] > 1 {
				return nil, fmt.Errorf("duplicate id %q", e.ID)
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
		return nil, fmt.Errorf("missing or ambiguous metadata target")
	}
	return selected[0], nil
}
