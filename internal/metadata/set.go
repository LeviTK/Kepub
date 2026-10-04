// Package metadata implements a bounded byte-preserving metadata.set v1.
package metadata

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/publication"
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

type element struct {
	name       xml.Name
	rawName    xml.Name
	id         string
	start, end int
	text       strings.Builder
	complex    bool
	location   string
	ns         map[string]string
	counts     map[string]int
	parent     *element
}

// Apply validates the complete UTF-8 XML before selecting a direct OPF metadata
// child. Comments, CDATA, PIs and child elements within the target are deliberately
// outside v1's simple-text subset, including for no-op requests.
func Apply(input []byte, request Set) ([]byte, bool, error) {
	if request.Namespace != DC || (request.LocalName != "title" && request.LocalName != "creator") {
		return nil, false, fmt.Errorf("unsupported metadata field")
	}
	if len(input) > publication.XMLLimit || len(request.NewValue) > publication.XMLLimit || !utf8.Valid(input) || !utf8.ValidString(request.NewValue) {
		return nil, false, fmt.Errorf("XML size/UTF-8 limit")
	}
	bom := len(input) - len(bytes.TrimPrefix(input, []byte{239, 187, 191}))
	d := xml.NewDecoder(bytes.NewReader(input[bom:]))
	stack := []*element{}
	var selected []*element
	ids := map[string]int{}
	roots, metas, tokens, indexed := 0, 0, 0, 0
	for {
		before := int(d.InputOffset()) + bom
		t, err := d.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, false, fmt.Errorf("invalid XML: %w", err)
		}
		tokens++
		if tokens > 200000 {
			return nil, false, fmt.Errorf("XML token limit")
		}
		switch t := t.(type) {
		case xml.Directive:
			return nil, false, fmt.Errorf("XML directives/DTD forbidden")
		case xml.StartElement:
			if len(stack) >= 128 {
				return nil, false, fmt.Errorf("XML depth limit")
			}
			ns := map[string]string{"xml": "http://www.w3.org/XML/1998/namespace"}
			var parent *element
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
				parent.complex = true
			}
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" {
					if strings.Contains(a.Name.Local, ":") || a.Name.Local == "xmlns" || a.Value == "http://www.w3.org/2000/xmlns/" || (a.Name.Local == "xml" && a.Value != "http://www.w3.org/XML/1998/namespace") || (a.Name.Local != "xml" && a.Value == "http://www.w3.org/XML/1998/namespace") || a.Value == "" {
						return nil, false, fmt.Errorf("invalid namespace declaration")
					}
					ns[a.Name.Local] = a.Value
				}
				if a.Name.Space == "" && a.Name.Local == "xmlns" {
					if a.Value == "http://www.w3.org/XML/1998/namespace" || a.Value == "http://www.w3.org/2000/xmlns/" {
						return nil, false, fmt.Errorf("reserved default namespace")
					}
					ns[""] = a.Value
				}
			}
			rawName := t.Name
			e := &element{rawName: rawName, start: int(d.InputOffset()) + bom, ns: ns, counts: map[string]int{}, parent: parent}
			var resolveErr error
			t.Name, resolveErr = resolveName(t.Name, e, true)
			if resolveErr != nil {
				return nil, false, resolveErr
			}
			e.name = t.Name
			if bytes.HasSuffix(input[before:e.start], []byte("/>")) {
				e.complex = true
			}
			seen := map[xml.Name]bool{}
			for i, a := range t.Attr {
				if a.Name.Space != "xmlns" {
					a.Name, resolveErr = resolveName(a.Name, e, false)
					if resolveErr != nil {
						return nil, false, resolveErr
					}
				}
				t.Attr[i] = a
				if seen[a.Name] {
					return nil, false, fmt.Errorf("duplicate attribute")
				}
				seen[a.Name] = true
				if a.Name.Space == "http://www.w3.org/XML/1998/namespace" && a.Name.Local == "base" {
					return nil, false, fmt.Errorf("xml:base unsupported")
				}
				if a.Name.Space == "" && a.Name.Local == "id" {
					e.id = a.Value
					ids[a.Value]++
				}
			}
			if len(stack) == 0 {
				roots++
				if roots != 1 || t.Name != (xml.Name{Space: opf, Local: "package"}) {
					return nil, false, fmt.Errorf("expected single OPF package")
				}
				version := ""
				for _, a := range t.Attr {
					if a.Name == (xml.Name{Local: "version"}) {
						version = a.Value
					}
				}
				if version != "2.0" && version != "3.0" {
					return nil, false, fmt.Errorf("unsupported package version")
				}
				e.location = "/package[1]"
			} else {
				parent := stack[len(stack)-1]
				parent.counts[t.Name.Local]++
				e.location = fmt.Sprintf("%s/%s[%d]", parent.location, t.Name.Local, parent.counts[t.Name.Local])
			}
			indexed += len(e.location)
			if len(stack) == 1 && t.Name == (xml.Name{Space: opf, Local: "metadata"}) {
				metas++
			}
			if len(stack) == 2 && stack[1].name == (xml.Name{Space: opf, Local: "metadata"}) && t.Name == (xml.Name{Space: request.Namespace, Local: request.LocalName}) && (request.ID == "" || request.ID == e.id) {
				selected = append(selected, e)
			}
			stack = append(stack, e)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, false, fmt.Errorf("unexpected XML end element")
			}
			e := stack[len(stack)-1]
			e.end = before
			name, err := resolveName(t.Name, e, true)
			if err != nil || name != e.name || t.Name != e.rawName {
				return nil, false, fmt.Errorf("mismatched XML end element")
			}
			// A self-closing element has no independently replaceable text interval.
			if e.end < e.start {
				e.complex = true
			}
			if len(stack) > 1 {
				indexed += e.text.Len()
				stack[len(stack)-2].text.WriteString(e.text.String())
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 {
				if strings.TrimSpace(string(t)) != "" {
					return nil, false, fmt.Errorf("text outside root")
				}
			} else {
				indexed += 2 * len(t)
				e := stack[len(stack)-1]
				e.text.Write(t)
				if bytes.HasPrefix(input[before:], []byte("<![CDATA[")) {
					e.complex = true
				}
			}
		case xml.ProcInst:
			if strings.EqualFold(t.Target, "xml") && (t.Target != "xml" || before != bom) {
				return nil, false, fmt.Errorf("misplaced XML declaration")
			}
			if len(stack) > 0 {
				stack[len(stack)-1].complex = true
			}
		case xml.Comment:
			if len(stack) > 0 {
				stack[len(stack)-1].complex = true
			}
		}
		if indexed > 32<<20 {
			return nil, false, fmt.Errorf("XML text/location limit")
		}
	}
	if roots != 1 || metas != 1 || len(stack) != 0 || len(selected) != 1 {
		return nil, false, fmt.Errorf("missing or ambiguous metadata target")
	}
	for id, n := range ids {
		if id != "" && n > 1 {
			return nil, false, fmt.Errorf("duplicate id %q", id)
		}
	}
	e := selected[0]
	if e.complex {
		return nil, false, fmt.Errorf("complex metadata target unsupported")
	}
	if e.text.String() != request.ExpectedOldValue {
		return nil, false, fmt.Errorf("metadata old value mismatch")
	}
	if request.NewValue == request.ExpectedOldValue {
		return bytes.Clone(input), false, nil
	}
	var escaped bytes.Buffer
	if err := xml.EscapeText(&escaped, []byte(request.NewValue)); err != nil {
		return nil, false, err
	}
	result := make([]byte, 0, len(input)+escaped.Len())
	result = append(result, input[:e.start]...)
	result = append(result, escaped.Bytes()...)
	result = append(result, input[e.end:]...)
	// EscapeText does not reject every XML-forbidden character. Re-parse the
	// result and independently assert the decoded new value before returning it.
	check := request
	check.ExpectedOldValue = request.NewValue
	check.NewValue = request.NewValue
	if _, _, err := Apply(result, check); err != nil {
		return nil, false, err
	}
	return result, true, nil
}

func resolveName(name xml.Name, scope *element, defaultNS bool) (xml.Name, error) {
	if strings.Contains(name.Local, ":") || name.Space == "xmlns" {
		return name, fmt.Errorf("invalid XML qualified name")
	}
	if name.Space == "" && !defaultNS {
		return name, nil
	}
	// Keep lexical scopes linked, not duplicated at every depth: 8 MiB of
	// namespace declarations must not expand into 128 copies of the same map.
	for current := scope; current != nil; current = current.parent {
		if uri, ok := current.ns[name.Space]; ok {
			name.Space = uri
			return name, nil
		}
	}
	if name.Space != "" {
		return name, fmt.Errorf("undeclared namespace prefix")
	}
	return name, nil
}
