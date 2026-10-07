package publication

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

// MoveEndpoint is one frozen resource binding of a cross-resource move: the
// source subtree or the destination anchor with the exact resource hash the
// bytes were read from.
type MoveEndpoint struct {
	BookPath       bookpath.BookPath `json:"bookPath"`
	RevisionID     string            `json:"revisionId"`
	ResourceSHA256 string            `json:"resourceSha256"`
	LocatorVersion int               `json:"locatorVersion"`
	Locator        string            `json:"locator"`
}

// ElementMoveCross is xhtml.element.move v2: move one explicit XHTML subtree
// from one manifest resource to an explicit position in another manifest
// resource. The v1 same-resource ElementMove keeps its frozen shape and meaning.
type ElementMoveCross struct {
	Source      MoveEndpoint `json:"source"`
	Destination MoveEndpoint `json:"destination"`
	Position    string       `json:"position"`
}

func (s ElementMoveCross) Validate() error {
	if err := validateBinding(s.Source.BookPath, s.Source.RevisionID, s.Source.ResourceSHA256, s.Source.LocatorVersion, s.Source.Locator); err != nil {
		return err
	}
	if err := validateBinding(s.Destination.BookPath, s.Destination.RevisionID, s.Destination.ResourceSHA256, s.Destination.LocatorVersion, s.Destination.Locator); err != nil {
		return err
	}
	if s.Source.BookPath == s.Destination.BookPath {
		return fmt.Errorf("cross-resource move requires two different resources")
	}
	if !validStructurePosition(s.Position) {
		return fmt.Errorf("unsupported move position %q", s.Position)
	}
	return nil
}

// MoveRewrite is one attribute value rewritten inside a moved block. The byte
// interval stays internal; review reports the locator, name and both values.
type MoveRewrite struct {
	Locator string `json:"locator"`
	Name    string `json:"name"`
	Old     string `json:"oldValue"`
	New     string `json:"newValue"`
}

// CrossMoveEdit is one cross-resource move derived against two frozen documents:
// the source removal, the rewritten destination block and the block's rebased
// outgoing link facts. Incoming reference synchronization belongs to the
// workspace gate, which owns the frozen reference index.
type CrossMoveEdit struct {
	SourcePath      bookpath.BookPath
	DestinationPath bookpath.BookPath
	SourceLocator   string
	Anchor          string
	Position        string
	MovedIDs        []string
	SourceSpan      EditSpan
	InsertAt        int
	Fragment        *Fragment
	Block           []byte
	Links           []StructureLink
	IDREFs          []StructureIDREF
	Rewrites        []MoveRewrite
	valueSpans      []moveValueSpan
}

// moveValueSpan is one rewritten attribute value interval in the frozen source
// bytes, applied to the block before it is re-encoded for the destination.
type moveValueSpan struct {
	Start, End int
	Bytes      []byte
}

// ElementMoveCrossEdit derives one cross-resource move. The block keeps every
// authored byte except the URL attribute values this batch rebases, and it must
// resolve to the same names and attributes in the destination insertion scope.
func (d *StructureDocument) ElementMoveCrossEdit(op ElementMoveCross, dest *StructureDocument) (*CrossMoveEdit, error) {
	if err := op.Validate(); err != nil {
		return nil, err
	}
	if dest == d {
		return nil, fault.New(2, "INVALID_OPERATIONS", "cross-resource move requires two resources")
	}
	moved, err := d.Locate(op.Source.Locator)
	if err != nil {
		return nil, err
	}
	if err := targetRefused(moved); err != nil {
		return nil, fault.New(2, "INVALID_OPERATIONS", "move source: %v", err)
	}
	start, end, ok := moved.PhysicalMarkup()
	if !ok {
		return nil, fault.New(2, "INVALID_OPERATIONS", "element %s has no literal markup interval", op.Source.Locator)
	}
	anchor, err := dest.Locate(op.Destination.Locator)
	if err != nil {
		return nil, err
	}
	_, at, scope, err := dest.insertion(anchor, op.Position)
	if err != nil {
		return nil, err
	}
	valueSpans, rewrites, links, refs, err := d.rebaseMoveBlock(moved, op.Source.BookPath, op.Destination.BookPath)
	if err != nil {
		return nil, err
	}
	block := bytes.Clone(d.Input[start:end])
	for i := len(valueSpans) - 1; i >= 0; i-- {
		span := valueSpans[i]
		lo, hi := span.Start-start, span.End-start
		if lo < 0 || hi > len(block) || lo > hi {
			return nil, fault.New(2, "INVALID_OPERATIONS", "move block rewrite interval is outside the moved element")
		}
		next := make([]byte, 0, len(block)+len(span.Bytes)-(hi-lo))
		next = append(next, block[:lo]...)
		next = append(next, span.Bytes...)
		next = append(next, block[hi:]...)
		block = next
	}
	raw, err := d.DecodeMarkup(block)
	if err != nil {
		return nil, fault.New(2, "INVALID_OPERATIONS", "move block cannot be decoded from the source encoding: %v", err)
	}
	fragment, err := ParseFragment(raw, scope, dest)
	if err != nil {
		return nil, fault.New(2, "INVALID_OPERATIONS", "move block is not insertable at the destination: %v", err)
	}
	expected := moveExpectedNode(moved, rewrites)
	if err := compareMoveNode(expected, fragment.Nodes[0]); err != nil {
		return nil, fault.New(2, "INVALID_OPERATIONS", "move block namespace context differs at the destination: %v", err)
	}
	return &CrossMoveEdit{
		SourcePath:      op.Source.BookPath,
		DestinationPath: op.Destination.BookPath,
		SourceLocator:   op.Source.Locator,
		Anchor:          op.Destination.Locator,
		Position:        op.Position,
		MovedIDs:        subtreeIDs(moved),
		SourceSpan:      EditSpan{Start: start, End: end},
		InsertAt:        at,
		Fragment:        fragment,
		Block:           fragment.Bytes,
		Links:           links,
		IDREFs:          refs,
		Rewrites:        rewrites,
		valueSpans:      valueSpans,
	}, nil
}

// rebaseMoveBlock rewrites the block's XHTML href/src values from the source
// resource directory to the destination directory and refuses block content
// that cannot be carried across resources: an IDREF that would leave the block
// is same-document by definition and has no cross-resource URL form.
func (d *StructureDocument) rebaseMoveBlock(moved *xmltext.Element, sourcePath, destPath bookpath.BookPath) ([]moveValueSpan, []MoveRewrite, []StructureLink, []StructureIDREF, error) {
	spans := []moveValueSpan{}
	rewrites := []MoveRewrite{}
	links := []StructureLink{}
	refs := []StructureIDREF{}
	var walk func(*xmltext.Element) error
	walk = func(e *xmltext.Element) error {
		for _, a := range e.Attributes {
			if a.Name.Space == "" && (a.Name.Local == "href" || a.Name.Local == "src") {
				value, changed, err := d.rebaseMoveValue(moved, a.Value, sourcePath, destPath)
				if err != nil {
					return err
				}
				if changed {
					markup, ok := e.AttributeBytes(a.Name)
					if !ok {
						return fault.New(2, "INVALID_OPERATIONS", "move block attribute %s has no writable source interval", a.Name.Local)
					}
					encoded, err := d.encodeAttributeValue(value)
					if err != nil {
						return err
					}
					spans = append(spans, moveValueSpan{Start: markup.ValueStart, End: markup.ValueEnd, Bytes: encoded})
					rewrites = append(rewrites, MoveRewrite{Locator: e.Location, Name: a.Name.Local, Old: a.Value, New: value})
				}
				// Every URL that moves with the block is validated by the shared
				// link gate in its final form, including values this batch does
				// not rewrite: a blocked scheme or an unprovable internal target
				// must refuse instead of arriving at the destination unverified.
				links = append(links, StructureLink{Locator: e.Location, Name: a.Name.Local, Value: value})
			}
			if a.Name.Space == "" && IsIDREFAttribute(a.Name.Local) {
				for _, token := range IDREFs(a.Value) {
					targets := d.identityElements(token)
					if len(targets) != 1 || !insideSubtree(targets[0], moved) {
						return fault.New(2, "INVALID_OPERATIONS", "IDREF %s=%q in the moved block is same-document and cannot be synchronized across resources", a.Name.Local, token)
					}
					refs = append(refs, StructureIDREF{Locator: e.Location, Name: a.Name.Local, Value: token})
				}
			}
		}
		for _, c := range e.Children {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(moved); err != nil {
		return nil, nil, nil, nil, err
	}
	return spans, rewrites, links, refs, nil
}

// rebaseMoveValue returns the destination-relative form of one block URL. A
// fragment that moves with the block stays local; a fragment that stays in the
// source resource is rewritten to the source resource; any other target is
// rebased from the destination directory.
func (d *StructureDocument) rebaseMoveValue(moved *xmltext.Element, value string, sourcePath, destPath bookpath.BookPath) (string, bool, error) {
	trimmed := strings.Trim(value, " \t\n\r\f")
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", false, fault.New(2, "INVALID_OPERATIONS", "move block URL %q cannot be parsed", value)
	}
	ref, err := bookpath.ResolveReference(sourcePath, bookpath.Href(trimmed))
	if err != nil {
		return "", false, fault.New(2, "INVALID_OPERATIONS", "move block URL %q cannot be resolved: %v", value, err)
	}
	if ref.External {
		return value, false, nil
	}
	if ref.Path != sourcePath {
		target, err := bookpath.RelativeHref(destPath, ref.Path)
		if err != nil {
			return "", false, err
		}
		return string(target) + urlSuffix(u), true, nil
	}
	// The reference targets the source resource itself.
	if ref.Fragment == "" {
		target, err := bookpath.RelativeHref(destPath, sourcePath)
		if err != nil {
			return "", false, err
		}
		return string(target) + urlSuffix(u), true, nil
	}
	targets := d.identityElements(ref.Fragment)
	if len(targets) != 1 {
		return "", false, fault.New(2, "INVALID_OPERATIONS", "move block fragment %q does not resolve uniquely in %s", ref.Fragment, sourcePath)
	}
	if insideSubtree(targets[0], moved) {
		if u.Path == "" {
			return value, false, nil
		}
		// An explicit path to the source document whose fragment moves with the
		// block becomes a local reference in the destination; its query stays.
		return urlSuffix(u), true, nil
	}
	target, err := bookpath.RelativeHref(destPath, sourcePath)
	if err != nil {
		return "", false, err
	}
	return string(target) + urlSuffix(u), true, nil
}

// urlSuffix renders the query and fragment of one parsed URL canonically. An
// empty query that was written as "?" keeps its "?" so the rewritten reference
// has the same meaning as the authored one.
func urlSuffix(u *url.URL) string {
	out := ""
	if u.RawQuery != "" || u.ForceQuery {
		out += "?" + u.RawQuery
	}
	if u.Fragment != "" {
		out += "#" + u.EscapedFragment()
	}
	return out
}

// identityElements returns every frozen element carrying one identity value.
func (d *StructureDocument) identityElements(value string) []*xmltext.Element {
	out := []*xmltext.Element{}
	for _, e := range d.Doc.Elements {
		for _, v := range ElementIdentities(e) {
			if v == value {
				out = append(out, e)
				break
			}
		}
	}
	return out
}

// insideSubtree reports whether one element is the subtree root or a descendant.
func insideSubtree(e, root *xmltext.Element) bool {
	for p := e; p != nil; p = p.Parent {
		if p == root {
			return true
		}
	}
	return false
}

// moveExpectedNode mirrors the moved subtree with the rewritten values, so the
// destination parse can be compared against the names and attributes the block
// resolved to in the source document.
func moveExpectedNode(e *xmltext.Element, rewrites []MoveRewrite) *FragmentNode {
	attrs := append([]xml.Attr(nil), e.Attributes...)
	for i := range attrs {
		for _, rw := range rewrites {
			if rw.Locator == e.Location && rw.Name == attrs[i].Name.Local && attrs[i].Name.Space == "" {
				attrs[i].Value = rw.New
				break
			}
		}
	}
	n := &FragmentNode{Name: e.Name, Attrs: attrs, Direct: e.DirectText}
	for _, c := range e.Children {
		n.Children = append(n.Children, moveExpectedNode(c, rewrites))
	}
	return n
}

// compareMoveNode refuses a block whose destination parse resolves a different
// element name, attribute name or attribute value than the source document. The
// batch never rewrites namespace prefixes, so a context mismatch is an error.
func compareMoveNode(expected, actual *FragmentNode) error {
	if expected.Name != actual.Name {
		return fmt.Errorf("element %q resolves to {%s}%s at the destination", expected.Name.Local, actual.Name.Space, actual.Name.Local)
	}
	if expected.Direct != actual.Direct {
		return fmt.Errorf("element %s direct text differs at the destination", expected.Name.Local)
	}
	expectedAttrs := comparableAttrs(expected.Attrs)
	actualAttrs := comparableAttrs(actual.Attrs)
	if len(expectedAttrs) != len(actualAttrs) {
		return fmt.Errorf("element %s attribute count differs at the destination", expected.Name.Local)
	}
	for i := range expectedAttrs {
		if expectedAttrs[i].Name != actualAttrs[i].Name || expectedAttrs[i].Value != actualAttrs[i].Value {
			return fmt.Errorf("element %s attribute %s differs at the destination", expected.Name.Local, expectedAttrs[i].Name.Local)
		}
	}
	if len(expected.Children) != len(actual.Children) {
		return fmt.Errorf("element %s child count differs at the destination", expected.Name.Local)
	}
	for i := range expected.Children {
		if err := compareMoveNode(expected.Children[i], actual.Children[i]); err != nil {
			return err
		}
	}
	return nil
}

// comparableAttrs drops namespace declarations, which the destination scope may
// legitimately supply instead of the block, and keeps resolved names in order.
func comparableAttrs(attrs []xml.Attr) []xml.Attr {
	out := []xml.Attr{}
	for _, a := range attrs {
		if a.Name.Space == "xmlns" || a.Name.Space == "" && a.Name.Local == "xmlns" {
			continue
		}
		out = append(out, a)
	}
	return out
}
