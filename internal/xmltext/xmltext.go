// Package xmltext owns the strict byte index used for local simple-text edits.
// Locations are derived from parsing, never caller-supplied writable offsets.
package xmltext

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/fault"
)

const Limit = 8 << 20

var htmlDoctype = regexp.MustCompile(`^DOCTYPE[\x20\x09\x0d\x0a]+html[\x20\x09\x0d\x0a]*$`)

type Element struct {
	Name       xml.Name
	Attributes []xml.Attr
	ID         string
	Start, End int
	Text       string
	DirectText string
	Complex    bool
	Location   string
	Parent     *Element
	Children   []*Element
	rawName    xml.Name
	text       strings.Builder
	direct     strings.Builder
	order      binary.ByteOrder
	ns         map[string]string
	counts     map[string]int
}

type Document struct {
	Root                   *Element
	Elements               []*Element // document order
	ProcessingInstructions []string
}

// Parse is the former metadata RawToken index: lexical namespace validation,
// byte intervals, simple-text classification and aggregate budgets are shared.
func Parse(input []byte) (*Document, error) {
	s, err := decode(input)
	if err != nil {
		return nil, err
	}
	d := xml.NewDecoder(bytes.NewReader(s.text))
	// Physical encoding and declaration have already been checked. RawToken
	// must not decode the transcoded UTF-8 a second time.
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	stack := []*Element{}
	doc := &Document{}
	doctype := false
	tokens, indexed := 0, 0
	for {
		before := int(d.InputOffset())
		t, err := d.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid XML: %w", err)
		}
		tokens++
		if tokens > 200000 {
			return nil, fault.New(1, "XML_LIMIT", "XML token limit")
		}
		switch t := t.(type) {
		case xml.Directive:
			if !htmlDoctype.Match(t) {
				return nil, fault.New(3, "UNSUPPORTED_XML_DTD", "DTD is outside the T1a offline subset")
			}
			if doctype || doc.Root != nil {
				return nil, fmt.Errorf("duplicate or misplaced DOCTYPE")
			}
			doctype = true
		case xml.StartElement:
			if len(stack) >= 128 {
				return nil, fault.New(1, "XML_LIMIT", "XML depth limit")
			}
			ns := map[string]string{"xml": "http://www.w3.org/XML/1998/namespace"}
			var parent *Element
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
				parent.Complex = true
			}
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" {
					if strings.Contains(a.Name.Local, ":") || a.Name.Local == "xmlns" || a.Value == "http://www.w3.org/2000/xmlns/" || (a.Name.Local == "xml" && a.Value != "http://www.w3.org/XML/1998/namespace") || (a.Name.Local != "xml" && a.Value == "http://www.w3.org/XML/1998/namespace") || a.Value == "" {
						return nil, fmt.Errorf("invalid namespace declaration")
					}
					ns[a.Name.Local] = a.Value
				}
				if a.Name.Space == "" && a.Name.Local == "xmlns" {
					if a.Value == "http://www.w3.org/XML/1998/namespace" || a.Value == "http://www.w3.org/2000/xmlns/" {
						return nil, fmt.Errorf("reserved default namespace")
					}
					ns[""] = a.Value
				}
			}
			e := &Element{rawName: t.Name, Start: s.offset(int(d.InputOffset())), ns: ns, counts: map[string]int{}, Parent: parent, order: s.order}
			var resolveErr error
			t.Name, resolveErr = resolveName(t.Name, e, true)
			if resolveErr != nil {
				return nil, resolveErr
			}
			e.Name = t.Name
			if bytes.HasSuffix(s.text[before:int(d.InputOffset())], []byte("/>")) {
				e.Complex = true
			}
			seen := map[xml.Name]bool{}
			for i, a := range t.Attr {
				if a.Name.Space != "xmlns" {
					a.Name, resolveErr = resolveName(a.Name, e, false)
					if resolveErr != nil {
						return nil, resolveErr
					}
				}
				t.Attr[i] = a
				if seen[a.Name] {
					return nil, fmt.Errorf("duplicate attribute")
				}
				seen[a.Name] = true
				if a.Name.Space == "http://www.w3.org/XML/1998/namespace" && a.Name.Local == "base" {
					return nil, fault.New(3, "UNSUPPORTED_XML_BASE", "xml:base unsupported")
				}
				if a.Name.Space == "" && a.Name.Local == "id" {
					e.ID = a.Value
				}
			}
			e.Attributes = t.Attr
			if parent == nil {
				if doc.Root != nil {
					return nil, fmt.Errorf("multiple XML roots")
				}
				doc.Root = e
				e.Location = "/" + e.Name.Local + "[1]"
			} else {
				parent.counts[e.Name.Local]++
				e.Location = fmt.Sprintf("%s/%s[%d]", parent.Location, e.Name.Local, parent.counts[e.Name.Local])
				parent.Children = append(parent.Children, e)
			}
			indexed += len(e.Location)
			doc.Elements = append(doc.Elements, e)
			stack = append(stack, e)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("unexpected XML end element")
			}
			e := stack[len(stack)-1]
			e.End = s.offset(before)
			name, err := resolveName(t.Name, e, true)
			if err != nil || name != e.Name || t.Name != e.rawName {
				return nil, fmt.Errorf("mismatched XML end element")
			}
			if e.End < e.Start {
				e.Complex = true
			}
			e.Text = e.text.String()
			e.DirectText = e.direct.String()
			if len(stack) > 1 {
				indexed += len(e.Text)
				stack[len(stack)-2].text.WriteString(e.Text)
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 {
				if strings.TrimSpace(string(t)) != "" {
					return nil, fmt.Errorf("text outside root")
				}
			} else {
				indexed += 2 * len(t)
				e := stack[len(stack)-1]
				e.text.Write(t)
				e.direct.Write(t)
				if bytes.HasPrefix(s.text[before:], []byte("<![CDATA[")) {
					e.Complex = true
				}
			}
		case xml.ProcInst:
			if strings.EqualFold(t.Target, "xml") && (t.Target != "xml" || before != 0 || !declaration.Match(s.text)) {
				return nil, fmt.Errorf("misplaced XML declaration")
			}
			if t.Target != "xml" {
				doc.ProcessingInstructions = append(doc.ProcessingInstructions, t.Target)
			}
			if len(stack) > 0 {
				stack[len(stack)-1].Complex = true
			}
		case xml.Comment:
			if len(stack) > 0 {
				stack[len(stack)-1].Complex = true
			}
		}
		if indexed > 32<<20 {
			return nil, fault.New(1, "XML_LIMIT", "XML text/location limit")
		}
	}
	if doc.Root == nil || len(stack) != 0 {
		return nil, fmt.Errorf("missing or unclosed XML root")
	}
	return doc, nil
}

func resolveName(name xml.Name, scope *Element, defaultNS bool) (xml.Name, error) {
	if strings.Contains(name.Local, ":") || name.Space == "xmlns" {
		return name, fmt.Errorf("invalid XML qualified name")
	}
	if name.Space == "" && !defaultNS {
		return name, nil
	}
	for current := scope; current != nil; current = current.Parent {
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

// Replace is only for an element from Parse of this input. Escaping and reparsing
// prove the decoded new text; this does not decide which elements may be edited.
func Replace(input []byte, e *Element, old, new string) ([]byte, bool, error) {
	if e.Complex || e.Text != old {
		return nil, false, fmt.Errorf("complex target or old value mismatch")
	}
	if len(new) > Limit || !utf8.ValidString(new) {
		return nil, false, fmt.Errorf("XML size/UTF-8 limit")
	}
	if new == old {
		return bytes.Clone(input), false, nil
	}
	var escaped bytes.Buffer
	if err := xml.EscapeText(&escaped, []byte(new)); err != nil {
		return nil, false, err
	}
	replacement := encode(escaped.Bytes(), e.order)
	out := make([]byte, 0, len(input)+escaped.Len())
	out = append(out, input[:e.Start]...)
	out = append(out, replacement...)
	out = append(out, input[e.End:]...)
	check, err := Parse(out)
	if err != nil {
		return nil, false, err
	}
	for _, c := range check.Elements {
		if c.Location == e.Location && c.Name == e.Name && !c.Complex && c.Text == new {
			return out, true, nil
		}
	}
	return nil, false, fmt.Errorf("replacement text mismatch")
}
