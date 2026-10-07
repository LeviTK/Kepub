package publication

import (
	"bytes"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

// Versioned XHTML structural operations (request/plan/execution schema 4). All
// of them bind the frozen accepted resource bytes and an exact structural
// locator; none of them expand content.text.set v1.
const (
	StructureLocatorVersion = 1
	XMLNamespace            = "http://www.w3.org/XML/1998/namespace"
	OpsNamespace            = "http://www.idpf.org/2007/ops"
	AttributeValueLimit     = ContentTextLimit
	FragmentLimit           = XMLLimit
)

// Element names that must not be structurally targeted: the document skeleton
// is maintained by explicit publication operations, not by deleting it.
var structureTargetDenied = []string{"html", "head", "body"}

// Attribute writes that belong to other batches, open dynamic content, or carry
// URL semantics this batch cannot maintain. The URL set mirrors the reference
// index's unsupported URL-bearing attributes.
var structureAttributeDenied = []string{
	"style", "srcset", "imagesrcset", "http-equiv", "srcdoc",
	"poster", "data", "action", "formaction", "ping", "longdesc", "archive",
	"codebase", "background", "profile", "cite", "usemap", "manifest",
	"itemid", "itemtype", "classid",
}

// Fragment elements that are never inserted by this batch: script/style/base,
// nested browsing contexts, plugin content and head-only elements.
var fragmentElementDenied = []string{"script", "style", "base", "iframe", "frame", "frameset", "object", "embed", "applet", "portal", "link", "meta", "title"}

// idrefAttributes are XHTML/SVG attributes whose value is one IDREF or a
// whitespace-separated IDREF list resolved inside the same document. The
// reference index and the structural edit facts share this one set, so a new
// write and an existing document cannot disagree about what is a reference.
var idrefAttributes = map[string]bool{
	"headers": true, "for": true, "list": true, "form": true, "itemref": true,
	"aria-activedescendant": true, "aria-controls": true, "aria-describedby": true,
	"aria-details": true, "aria-errormessage": true, "aria-flowto": true,
	"aria-labelledby": true, "aria-owns": true,
}

// IsIDREFAttribute reports whether an unprefixed attribute name carries one
// IDREF or an IDREF list in the same document.
func IsIDREFAttribute(name string) bool { return idrefAttributes[name] }

// IDREFs splits one IDREF attribute value into its tokens.
func IDREFs(value string) []string { return strings.Fields(value) }

func validateBinding(bp bookpath.BookPath, revision, sha string, locatorVersion int, locator string) error {
	if _, err := bookpath.Parse(string(bp)); err != nil {
		return err
	}
	if revision == "" || !utf8.ValidString(revision) {
		return fmt.Errorf("revisionId is required")
	}
	h, err := hex.DecodeString(sha)
	if err != nil || len(h) != 32 || hex.EncodeToString(h) != sha {
		return fmt.Errorf("resourceSha256 must be lowercase SHA-256")
	}
	if locatorVersion != StructureLocatorVersion || locator == "" || len(locator) > 4096 || !utf8.ValidString(locator) {
		return fmt.Errorf("unsupported or invalid locator")
	}
	return nil
}

func validStructurePosition(position string) bool {
	switch position {
	case "before", "after", "first-child", "last-child":
		return true
	}
	return false
}

// validAttributeName rejects namespace declarations, xml:base and writes that
// belong to the CSS/script batches.
func validAttributeName(namespace, name string) error {
	switch namespace {
	case "", XMLNamespace, OpsNamespace:
	default:
		return fmt.Errorf("unsupported attribute namespace %q", namespace)
	}
	if !xmltext.ValidNCName(name) {
		return fmt.Errorf("invalid attribute name %q", name)
	}
	if name == "xmlns" || namespace == XMLNamespace && name == "base" {
		return fmt.Errorf("namespace declarations are not editable attributes")
	}
	if namespace == "" {
		if slices.Contains(structureAttributeDenied, name) || strings.HasPrefix(name, "on") {
			return fmt.Errorf("attribute %q is not editable in this batch", name)
		}
	}
	return nil
}

func validAttributeValue(value string) error {
	if len(value) > AttributeValueLimit || !utf8.ValidString(value) {
		return fmt.Errorf("attribute value exceeds 1 MiB or is not UTF-8")
	}
	for _, r := range value {
		if !validXMLRune(r) {
			return fmt.Errorf("attribute value is not XML text")
		}
	}
	return nil
}

func validXMLRune(r rune) bool {
	return r == 9 || r == 10 || r == 13 || r >= 0x20 && r <= 0xD7FF || r >= 0xE000 && r <= 0xFFFD || r >= 0x10000 && r <= 0x10FFFF
}

func validateFragment(fragment string) error {
	if fragment == "" || len(fragment) > FragmentLimit || !utf8.ValidString(fragment) {
		return fmt.Errorf("fragment must be 1 byte to 8 MiB of UTF-8 markup")
	}
	if strings.Contains(fragment, "<!") || strings.Contains(fragment, "<?") {
		return fmt.Errorf("fragment comments, CDATA, declarations and processing instructions are not supported")
	}
	return nil
}

// Field order is the schema 4 canonical JSON encoding.
type AttributeSet struct {
	BookPath         bookpath.BookPath `json:"bookPath"`
	RevisionID       string            `json:"revisionId"`
	ResourceSHA256   string            `json:"resourceSha256"`
	LocatorVersion   int               `json:"locatorVersion"`
	Locator          string            `json:"locator"`
	Namespace        string            `json:"namespace,omitempty"`
	Name             string            `json:"name"`
	ExpectedOldValue *string           `json:"expectedOldValue,omitempty"`
	Value            string            `json:"value"`
}

func (s AttributeSet) Validate() error {
	if err := validateBinding(s.BookPath, s.RevisionID, s.ResourceSHA256, s.LocatorVersion, s.Locator); err != nil {
		return err
	}
	if err := validAttributeName(s.Namespace, s.Name); err != nil {
		return err
	}
	if err := validAttributeValue(s.Value); err != nil {
		return err
	}
	if s.ExpectedOldValue != nil {
		if err := validAttributeValue(*s.ExpectedOldValue); err != nil {
			return fmt.Errorf("expectedOldValue: %v", err)
		}
	}
	return nil
}

type AttributeRemove struct {
	BookPath         bookpath.BookPath `json:"bookPath"`
	RevisionID       string            `json:"revisionId"`
	ResourceSHA256   string            `json:"resourceSha256"`
	LocatorVersion   int               `json:"locatorVersion"`
	Locator          string            `json:"locator"`
	Namespace        string            `json:"namespace,omitempty"`
	Name             string            `json:"name"`
	ExpectedOldValue string            `json:"expectedOldValue"`
}

func (s AttributeRemove) Validate() error {
	if err := validateBinding(s.BookPath, s.RevisionID, s.ResourceSHA256, s.LocatorVersion, s.Locator); err != nil {
		return err
	}
	if err := validAttributeName(s.Namespace, s.Name); err != nil {
		return err
	}
	return validAttributeValue(s.ExpectedOldValue)
}

type ElementDelete struct {
	BookPath       bookpath.BookPath `json:"bookPath"`
	RevisionID     string            `json:"revisionId"`
	ResourceSHA256 string            `json:"resourceSha256"`
	LocatorVersion int               `json:"locatorVersion"`
	Locator        string            `json:"locator"`
}

func (s ElementDelete) Validate() error {
	return validateBinding(s.BookPath, s.RevisionID, s.ResourceSHA256, s.LocatorVersion, s.Locator)
}

type ElementInsert struct {
	BookPath       bookpath.BookPath `json:"bookPath"`
	RevisionID     string            `json:"revisionId"`
	ResourceSHA256 string            `json:"resourceSha256"`
	LocatorVersion int               `json:"locatorVersion"`
	Locator        string            `json:"locator"`
	Position       string            `json:"position"`
	Fragment       string            `json:"fragment"`
}

func (s ElementInsert) Validate() error {
	if err := validateBinding(s.BookPath, s.RevisionID, s.ResourceSHA256, s.LocatorVersion, s.Locator); err != nil {
		return err
	}
	if !validStructurePosition(s.Position) {
		return fmt.Errorf("unsupported insert position %q", s.Position)
	}
	return validateFragment(s.Fragment)
}

type ElementReplace struct {
	BookPath       bookpath.BookPath `json:"bookPath"`
	RevisionID     string            `json:"revisionId"`
	ResourceSHA256 string            `json:"resourceSha256"`
	LocatorVersion int               `json:"locatorVersion"`
	Locator        string            `json:"locator"`
	Fragment       string            `json:"fragment"`
}

func (s ElementReplace) Validate() error {
	if err := validateBinding(s.BookPath, s.RevisionID, s.ResourceSHA256, s.LocatorVersion, s.Locator); err != nil {
		return err
	}
	return validateFragment(s.Fragment)
}

type ElementMove struct {
	BookPath       bookpath.BookPath `json:"bookPath"`
	RevisionID     string            `json:"revisionId"`
	ResourceSHA256 string            `json:"resourceSha256"`
	LocatorVersion int               `json:"locatorVersion"`
	Locator        string            `json:"locator"`
	Anchor         string            `json:"anchor"`
	Position       string            `json:"position"`
}

func (s ElementMove) Validate() error {
	if err := validateBinding(s.BookPath, s.RevisionID, s.ResourceSHA256, s.LocatorVersion, s.Locator); err != nil {
		return err
	}
	if s.Anchor == "" || len(s.Anchor) > 4096 || !utf8.ValidString(s.Anchor) {
		return fmt.Errorf("anchor is required")
	}
	if !validStructurePosition(s.Position) {
		return fmt.Errorf("unsupported move position %q", s.Position)
	}
	return nil
}

// StructureDocument is a frozen XHTML resource prepared for structural edits.
type StructureDocument struct {
	Path    bookpath.BookPath
	Input   []byte
	Doc     *xmltext.Document
	profile xmltext.Profile
	byLoc   map[string]*xmltext.Element
}

// ParseStructureDocument requires a complete, profile-clean XHTML document with
// exactly one direct body, so byte edits cannot silently depend on unresolved
// entities or a second content root.
func ParseStructureDocument(input []byte, path bookpath.BookPath, profile xmltext.Profile) (*StructureDocument, error) {
	if _, err := bookpath.Parse(string(path)); err != nil {
		return nil, err
	}
	doc, err := xmltext.Parse(input)
	if err != nil {
		return nil, err
	}
	if err := doc.CheckProfile(profile); err != nil {
		return nil, err
	}
	if err := doc.RequireComplete(); err != nil {
		return nil, err
	}
	if doc.Root.Name != (xml.Name{Space: XHTMLNamespace, Local: "html"}) {
		return nil, fmt.Errorf("expected XHTML html root")
	}
	bodies, nested := 0, 0
	for _, e := range doc.Elements {
		if e.Name == (xml.Name{Space: XHTMLNamespace, Local: "body"}) {
			if e.Parent == doc.Root {
				bodies++
			} else {
				nested++
			}
		}
	}
	if bodies != 1 || nested != 0 {
		return nil, fmt.Errorf("expected one direct XHTML body and no nested body")
	}
	d := &StructureDocument{Path: path, Input: input, Doc: doc, profile: profile, byLoc: map[string]*xmltext.Element{}}
	for _, e := range doc.Elements {
		d.byLoc[e.Location] = e
	}
	return d, nil
}

func (d *StructureDocument) Locate(locator string) (*xmltext.Element, error) {
	e, ok := d.byLoc[locator]
	if !ok {
		return nil, fault.New(2, "INVALID_OPERATIONS", "locator %q does not match an element", locator)
	}
	return e, nil
}

// IDs counts unprefixed id and xml:id values in the frozen document, matching
// the reference index's identity rules.
func (d *StructureDocument) IDs() map[string]int {
	out := map[string]int{}
	for _, e := range d.Doc.Elements {
		for _, a := range e.Attributes {
			if (a.Name.Space == "" || a.Name.Space == XMLNamespace) && a.Name.Local == "id" {
				out[a.Value]++
			}
		}
	}
	return out
}

// Encode encodes literal markup or text for the document's original encoding.
func (d *StructureDocument) Encode(text string) ([]byte, error) {
	return d.Doc.Root.EncodeMarkup(text)
}

// EditSpan replaces the original byte interval [Start, End) with Bytes.
type EditSpan struct {
	Start, End int
	Bytes      []byte
}

// EditPoint inserts Bytes at the original offset At.
type EditPoint struct {
	At    int
	Bytes []byte
}

// StructureLink is a new or changed URL-bearing attribute value that the
// dependency gate must resolve before the plan can run.
type StructureLink struct {
	Locator string
	Name    string
	Value   string
}

// StructureIDREF is one new same-document IDREF token written by an operation.
// It is validated against the transaction's final identity state, exactly like
// an existing IDREF attribute is validated against the frozen index.
type StructureIDREF struct {
	Locator string
	Name    string
	Value   string
}

// StructureEdit is one operation's complete byte-level effect on the frozen
// resource plus the facts the dependency gate and verifier need.
type StructureEdit struct {
	OpIndex    int
	Spans      []EditSpan
	Points     []EditPoint
	RemovedIDs []string
	AddedIDs   []string
	Links      []StructureLink
	IDREFs     []StructureIDREF
	Change     StructureChange
}

// StructureChange describes one operation's tree-level effect for independent
// verification of the spliced result.
type StructureChange struct {
	Kind     string
	Locator  string
	Anchor   string
	Position string
	Attr     *xml.Attr // attribute-set value; nil for removal
	Remove   bool
	Text     string
	Fragment *Fragment
	Block    []byte // exact inserted or moved bytes, checked at the destination
	// At is the frozen insertion point or replaced-range start of the block.
	At int
	// Decls are namespace declarations written together with a new attribute,
	// in tag order before it.
	Decls []xml.Attr
}

// Fragment is a validated insertion block. Bytes are encoded for the target
// document; Nodes mirror the parsed top-level elements.
type Fragment struct {
	Raw   string
	Bytes []byte
	Nodes []*FragmentNode
}

// FragmentNode is one parsed top-level fragment element.
type FragmentNode struct {
	Name     xml.Name
	Attrs    []xml.Attr
	Direct   string
	Text     string
	Children []*FragmentNode
}

// ParseFragment validates author markup in the insertion context and keeps the
// authored bytes. The context scope comes from the frozen document, so prefixes
// resolve exactly as they would at the insertion point.
func ParseFragment(raw string, scope map[string]string, d *StructureDocument) (*Fragment, error) {
	if err := validateFragment(raw); err != nil {
		return nil, err
	}
	var wrapper strings.Builder
	wrapper.WriteString("<kepub-fragment")
	for _, prefix := range sortedKeys(scope) {
		uri := scope[prefix]
		if prefix == "xml" || prefix == "" && uri == "" {
			continue
		}
		if prefix == "" {
			wrapper.WriteString(` xmlns="` + escapeAttributeValue(uri) + `"`)
		} else {
			wrapper.WriteString(` xmlns:` + prefix + `="` + escapeAttributeValue(uri) + `"`)
		}
	}
	wrapper.WriteString(">")
	wrapper.WriteString(raw)
	wrapper.WriteString("</kepub-fragment>")
	doc, err := xmltext.Parse([]byte(wrapper.String()))
	if err != nil {
		return nil, fmt.Errorf("fragment: %v", err)
	}
	if err := doc.CheckProfile(d.profile); err != nil {
		return nil, fmt.Errorf("fragment: %v", err)
	}
	if err := doc.RequireComplete(); err != nil {
		return nil, fmt.Errorf("fragment: %v", err)
	}
	if len(doc.ProcessingInstructions) != 0 {
		return nil, fmt.Errorf("fragment processing instructions are not supported")
	}
	root := doc.Root
	if root.Name.Local != "kepub-fragment" {
		return nil, fmt.Errorf("fragment wrapper mismatch")
	}
	// Top-level text is not modelled as an inserted node, so it cannot be
	// verified against the spliced bytes: require elements only.
	if len(root.Children) == 0 || root.DirectText != "" {
		return nil, fmt.Errorf("fragment must contain only elements with no top-level text")
	}
	f := &Fragment{Raw: raw}
	if f.Bytes, err = d.Encode(raw); err != nil {
		return nil, err
	}
	for _, child := range root.Children {
		if err := validateFragmentElement(child); err != nil {
			return nil, err
		}
		f.Nodes = append(f.Nodes, fragmentNode(child))
	}
	return f, nil
}

func validateFragmentElement(e *xmltext.Element) error {
	if e.Name.Space != XHTMLNamespace {
		return fmt.Errorf("fragment element %q is outside the XHTML namespace", e.Name.Local)
	}
	if slices.Contains(fragmentElementDenied, e.Name.Local) {
		return fmt.Errorf("fragment element %q is not insertable", e.Name.Local)
	}
	for _, a := range e.Attributes {
		if a.Name.Space == "xmlns" || a.Name == (xml.Name{Local: "xmlns"}) {
			continue
		}
		if err := validAttributeName(a.Name.Space, a.Name.Local); err != nil {
			return err
		}
		if isIDAttribute(a.Name) && !xmltext.ValidNCName(a.Value) {
			return fmt.Errorf("fragment id %q is not a legal XML name", a.Value)
		}
	}
	for _, c := range e.Children {
		if err := validateFragmentElement(c); err != nil {
			return err
		}
	}
	return nil
}

func fragmentNode(e *xmltext.Element) *FragmentNode {
	n := &FragmentNode{Name: e.Name, Attrs: e.Attributes, Direct: e.DirectText, Text: e.Text}
	for _, c := range e.Children {
		n.Children = append(n.Children, fragmentNode(c))
	}
	return n
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func escapeAttributeValue(v string) string {
	var b strings.Builder
	for _, r := range v {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&#39;")
		case '\t':
			b.WriteString("&#x9;")
		case '\n':
			b.WriteString("&#xA;")
		case '\r':
			b.WriteString("&#xD;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// encodeAttributeValue escapes and encodes one new attribute value.
func (d *StructureDocument) encodeAttributeValue(v string) ([]byte, error) {
	return d.Encode(escapeAttributeValue(v))
}

// attributeName renders one resolved attribute name with the prefix to use. An
// unprefixed name is in no namespace; xml: is predeclared.
func attributeName(name xml.Name, prefix string) (string, bool) {
	switch name.Space {
	case "":
		return name.Local, true
	case XMLNamespace:
		return "xml:" + name.Local, true
	case OpsNamespace:
		if prefix == "" {
			return "", false
		}
		return prefix + ":" + name.Local, true
	}
	return "", false
}

// opsPrefix returns an in-scope non-empty prefix bound to the operations
// namespace, so an existing declaration is reused instead of duplicated.
func opsPrefix(scope map[string]string) string {
	for _, prefix := range sortedKeys(scope) {
		if prefix != "" && prefix != "xml" && scope[prefix] == OpsNamespace {
			return prefix
		}
	}
	return ""
}

// freePrefix returns an unused prefix for a new operations namespace
// declaration.
func freePrefix(scope map[string]string) string {
	if _, taken := scope["epub"]; !taken {
		return "epub"
	}
	for i := 2; ; i++ {
		prefix := "epub" + strconv.Itoa(i)
		if _, taken := scope[prefix]; !taken {
			return prefix
		}
	}
}

// attributeInsertion builds the bytes that declare (when needed) and write one
// new attribute, separated correctly from the existing tag content.
func (d *StructureDocument) attributeInsertion(e *xmltext.Element, name xml.Name, value string) ([]byte, []xml.Attr, error) {
	scope := e.NamespaceScope()
	prefix := ""
	decls := []xml.Attr{}
	parts := []string{}
	if name.Space == OpsNamespace {
		if existing := opsPrefix(scope); existing != "" {
			prefix = existing
		} else {
			prefix = freePrefix(scope)
			parts = append(parts, `xmlns:`+prefix+`="`+OpsNamespace+`"`)
			decls = append(decls, xml.Attr{Name: xml.Name{Space: "xmlns", Local: prefix}, Value: OpsNamespace})
		}
	}
	rendered, ok := attributeName(name, prefix)
	if !ok {
		return nil, nil, fault.New(2, "INVALID_OPERATIONS", "unsupported attribute namespace")
	}
	parts = append(parts, rendered+`="`+escapeAttributeValue(value)+`"`)
	inserted, err := d.Encode(" " + strings.Join(parts, " "))
	if err != nil {
		return nil, nil, err
	}
	return inserted, decls, nil
}

// subtreeIDs collects unprefixed id and xml:id values in one subtree.
func subtreeIDs(e *xmltext.Element) []string {
	out := []string{}
	var walk func(*xmltext.Element)
	walk = func(e *xmltext.Element) {
		for _, a := range e.Attributes {
			if (a.Name.Space == "" || a.Name.Space == XMLNamespace) && a.Name.Local == "id" {
				out = append(out, a.Value)
			}
		}
		for _, c := range e.Children {
			walk(c)
		}
	}
	walk(e)
	return out
}

func (d *StructureDocument) elementLinkFacts(e *xmltext.Element) []StructureLink {
	out := []StructureLink{}
	for _, a := range e.Attributes {
		if e.Name.Space == XHTMLNamespace && a.Name.Space == "" && (a.Name.Local == "href" || a.Name.Local == "src") {
			out = append(out, StructureLink{Locator: e.Location, Name: a.Name.Local, Value: a.Value})
		}
	}
	return out
}

func fragmentFacts(nodes []*FragmentNode) ([]string, []StructureLink, []StructureIDREF) {
	ids := []string{}
	links := []StructureLink{}
	refs := []StructureIDREF{}
	var walk func(*FragmentNode)
	walk = func(n *FragmentNode) {
		for _, a := range n.Attrs {
			if isIDAttribute(a.Name) {
				ids = append(ids, a.Value)
			}
			if n.Name.Space == XHTMLNamespace && a.Name.Space == "" && (a.Name.Local == "href" || a.Name.Local == "src") {
				links = append(links, StructureLink{Name: a.Name.Local, Value: a.Value})
			}
			if n.Name.Space == XHTMLNamespace && a.Name.Space == "" && IsIDREFAttribute(a.Name.Local) {
				for _, token := range IDREFs(a.Value) {
					refs = append(refs, StructureIDREF{Name: a.Name.Local, Value: token})
				}
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, n := range nodes {
		walk(n)
	}
	return ids, links, refs
}

func targetRefused(e *xmltext.Element) error {
	if slices.Contains(structureTargetDenied, e.Name.Local) && e.Name.Space == XHTMLNamespace {
		return fmt.Errorf("element %q is not a structural edit target", e.Name.Local)
	}
	if e.Parent == nil {
		return fmt.Errorf("document root is not a structural edit target")
	}
	return nil
}

func (d *StructureDocument) AttributeSetEdit(op AttributeSet) (*StructureEdit, error) {
	if err := op.Validate(); err != nil {
		return nil, err
	}
	e, err := d.Locate(op.Locator)
	if err != nil {
		return nil, err
	}
	if e.Name.Space != XHTMLNamespace {
		return nil, fmt.Errorf("attribute edits require an XHTML element")
	}
	name := xml.Name{Space: op.Namespace, Local: op.Name}
	current, present := elementAttribute(e, name)
	if op.ExpectedOldValue != nil {
		if !present || current != *op.ExpectedOldValue {
			return nil, fault.New(2, "INVALID_OPERATIONS", "attribute %s old value mismatch", op.Name)
		}
	} else if present {
		return nil, fault.New(2, "INVALID_OPERATIONS", "attribute %s already exists", op.Name)
	}
	edit := &StructureEdit{Change: StructureChange{Kind: "attribute-set", Locator: op.Locator, Attr: &xml.Attr{Name: name, Value: op.Value}}}
	if isIDAttribute(name) {
		if op.Value == "" || !xmltext.ValidNCName(op.Value) {
			return nil, fault.New(2, "INVALID_OPERATIONS", "new id must be a non-empty XML name")
		}
		if present && current != op.Value {
			edit.RemovedIDs = append(edit.RemovedIDs, current)
		}
		if !present || current != op.Value {
			edit.AddedIDs = append(edit.AddedIDs, op.Value)
		}
	}
	if e.Name.Space == XHTMLNamespace && name.Space == "" && (name.Local == "href" || name.Local == "src") {
		edit.Links = append(edit.Links, StructureLink{Locator: op.Locator, Name: name.Local, Value: op.Value})
	}
	if e.Name.Space == XHTMLNamespace && name.Space == "" && IsIDREFAttribute(name.Local) {
		for _, token := range IDREFs(op.Value) {
			edit.IDREFs = append(edit.IDREFs, StructureIDREF{Locator: op.Locator, Name: name.Local, Value: token})
		}
	}
	value, err := d.encodeAttributeValue(op.Value)
	if err != nil {
		return nil, err
	}
	if present {
		markup, ok := e.AttributeBytes(name)
		if !ok {
			return nil, fault.New(2, "INVALID_OPERATIONS", "attribute %s has no writable source interval", op.Name)
		}
		if current == op.Value {
			return edit, nil // no-op
		}
		edit.Spans = append(edit.Spans, EditSpan{Start: markup.ValueStart, End: markup.ValueEnd, Bytes: value})
		return edit, nil
	}
	at, ok := e.TagEnd()
	if !ok {
		return nil, fault.New(2, "INVALID_OPERATIONS", "element %s has no literal start tag", op.Locator)
	}
	inserted, decls, err := d.attributeInsertion(e, name, op.Value)
	if err != nil {
		return nil, err
	}
	edit.Change.Decls = decls
	edit.Points = append(edit.Points, EditPoint{At: at, Bytes: inserted})
	return edit, nil
}

func (d *StructureDocument) AttributeRemoveEdit(op AttributeRemove) (*StructureEdit, error) {
	if err := op.Validate(); err != nil {
		return nil, err
	}
	e, err := d.Locate(op.Locator)
	if err != nil {
		return nil, err
	}
	if e.Name.Space != XHTMLNamespace {
		return nil, fmt.Errorf("attribute edits require an XHTML element")
	}
	name := xml.Name{Space: op.Namespace, Local: op.Name}
	current, present := elementAttribute(e, name)
	if !present || current != op.ExpectedOldValue {
		return nil, fault.New(2, "INVALID_OPERATIONS", "attribute %s old value mismatch", op.Name)
	}
	markup, ok := e.AttributeBytes(name)
	if !ok {
		return nil, fault.New(2, "INVALID_OPERATIONS", "attribute %s has no writable source interval", op.Name)
	}
	edit := &StructureEdit{Change: StructureChange{Kind: "attribute-remove", Locator: op.Locator, Remove: true, Attr: &xml.Attr{Name: name}}}
	if isIDAttribute(name) {
		edit.RemovedIDs = append(edit.RemovedIDs, current)
	}
	edit.Spans = append(edit.Spans, EditSpan{Start: markup.Start, End: markup.End})
	return edit, nil
}

func (d *StructureDocument) TextSetEdit(op TextSet, newText string) (*StructureEdit, error) {
	if err := op.Validate(); err != nil {
		return nil, err
	}
	e, err := simpleTextElementIn(d.Doc, op.Locator)
	if err != nil {
		return nil, err
	}
	if e.Text != op.ExpectedOldValue {
		return nil, fault.New(2, "INVALID_OPERATIONS", "text old value mismatch")
	}
	edit := &StructureEdit{Change: StructureChange{Kind: "text-set", Locator: op.Locator, Text: newText}}
	if newText == op.ExpectedOldValue {
		return edit, nil
	}
	start, end, ok := e.PhysicalContent()
	if !ok {
		return nil, fault.New(2, "INVALID_OPERATIONS", "target has no literal content interval")
	}
	value, err := e.ReplaceBytes(newText)
	if err != nil {
		return nil, err
	}
	edit.Spans = append(edit.Spans, EditSpan{Start: start, End: end, Bytes: value})
	return edit, nil
}

func (d *StructureDocument) ElementDeleteEdit(op ElementDelete) (*StructureEdit, error) {
	if err := op.Validate(); err != nil {
		return nil, err
	}
	e, err := d.Locate(op.Locator)
	if err != nil {
		return nil, err
	}
	if err := targetRefused(e); err != nil {
		return nil, err
	}
	start, end, ok := e.PhysicalMarkup()
	if !ok {
		return nil, fault.New(2, "INVALID_OPERATIONS", "element %s has no literal markup interval", op.Locator)
	}
	edit := &StructureEdit{RemovedIDs: subtreeIDs(e), Change: StructureChange{Kind: "element-delete", Locator: op.Locator}}
	edit.Spans = append(edit.Spans, EditSpan{Start: start, End: end})
	return edit, nil
}

// insertion resolves the destination of an insert/replace/move block: the
// parent, the byte position in the frozen resource, and the namespace scope.
func (d *StructureDocument) insertion(anchor *xmltext.Element, position string) (parent *xmltext.Element, at int, scope map[string]string, err error) {
	switch position {
	case "before", "after":
		parent = anchor.Parent
		if parent == nil {
			return nil, 0, nil, fault.New(2, "INVALID_OPERATIONS", "anchor has no parent element")
		}
		if parent.Name == (xml.Name{Space: XHTMLNamespace, Local: "html"}) {
			return nil, 0, nil, fault.New(2, "INVALID_OPERATIONS", "insertion between head and body is not supported")
		}
		if position == "before" {
			if anchor.OpenStart < 0 {
				return nil, 0, nil, fault.New(2, "INVALID_OPERATIONS", "anchor start tag has no literal interval")
			}
			at = anchor.OpenStart
		} else {
			if anchor.CloseEnd < 0 {
				return nil, 0, nil, fault.New(2, "INVALID_OPERATIONS", "anchor end tag has no literal interval")
			}
			at = anchor.CloseEnd
		}
		return parent, at, parent.NamespaceScope(), nil
	case "first-child", "last-child":
		if anchor.Name.Space == XHTMLNamespace && (anchor.Name.Local == "html" || anchor.Name.Local == "head") {
			return nil, 0, nil, fault.New(2, "INVALID_OPERATIONS", "child insertion into %s is not supported", anchor.Name.Local)
		}
		start, end, ok := anchor.PhysicalContent()
		if !ok {
			return nil, 0, nil, fault.New(2, "INVALID_OPERATIONS", "anchor has no literal content interval")
		}
		if position == "first-child" {
			at = start
		} else {
			at = end
		}
		return anchor, at, anchor.NamespaceScope(), nil
	}
	return nil, 0, nil, fault.New(2, "INVALID_OPERATIONS", "unsupported position")
}

func (d *StructureDocument) ElementInsertEdit(op ElementInsert) (*StructureEdit, error) {
	if err := op.Validate(); err != nil {
		return nil, err
	}
	anchor, err := d.Locate(op.Locator)
	if err != nil {
		return nil, err
	}
	_, at, scope, err := d.insertion(anchor, op.Position)
	if err != nil {
		return nil, err
	}
	fragment, err := ParseFragment(op.Fragment, scope, d)
	if err != nil {
		return nil, err
	}
	ids, links, refs := fragmentFacts(fragment.Nodes)
	edit := &StructureEdit{AddedIDs: ids, Links: links, IDREFs: refs, Change: StructureChange{Kind: "element-insert", Locator: op.Locator, Position: op.Position, Fragment: fragment, Block: fragment.Bytes, At: at}}
	edit.Points = append(edit.Points, EditPoint{At: at, Bytes: fragment.Bytes})
	return edit, nil
}

func (d *StructureDocument) ElementReplaceEdit(op ElementReplace) (*StructureEdit, error) {
	if err := op.Validate(); err != nil {
		return nil, err
	}
	target, err := d.Locate(op.Locator)
	if err != nil {
		return nil, err
	}
	if err := targetRefused(target); err != nil {
		return nil, err
	}
	start, end, ok := target.PhysicalMarkup()
	if !ok {
		return nil, fault.New(2, "INVALID_OPERATIONS", "element %s has no literal markup interval", op.Locator)
	}
	fragment, err := ParseFragment(op.Fragment, target.Parent.NamespaceScope(), d)
	if err != nil {
		return nil, err
	}
	ids, links, refs := fragmentFacts(fragment.Nodes)
	edit := &StructureEdit{RemovedIDs: subtreeIDs(target), AddedIDs: ids, Links: links, IDREFs: refs, Change: StructureChange{Kind: "element-replace", Locator: op.Locator, Fragment: fragment, Block: fragment.Bytes, At: start}}
	edit.Spans = append(edit.Spans, EditSpan{Start: start, End: end, Bytes: fragment.Bytes})
	return edit, nil
}

func (d *StructureDocument) ElementMoveEdit(op ElementMove) (*StructureEdit, error) {
	if err := op.Validate(); err != nil {
		return nil, err
	}
	moved, err := d.Locate(op.Locator)
	if err != nil {
		return nil, err
	}
	if err := targetRefused(moved); err != nil {
		return nil, err
	}
	anchor, err := d.Locate(op.Anchor)
	if err != nil {
		return nil, err
	}
	if anchor == moved {
		return nil, fault.New(2, "INVALID_OPERATIONS", "cannot move an element relative to itself")
	}
	start, end, ok := moved.PhysicalMarkup()
	if !ok {
		return nil, fault.New(2, "INVALID_OPERATIONS", "element %s has no literal markup interval", op.Locator)
	}
	for p := anchor; p != nil; p = p.Parent {
		if p == moved {
			return nil, fault.New(2, "INVALID_OPERATIONS", "cannot move an element inside itself")
		}
	}
	_, at, _, err := d.insertion(anchor, op.Position)
	if err != nil {
		return nil, err
	}
	if at > start && at < end {
		return nil, fault.New(2, "INVALID_OPERATIONS", "move destination is inside the moved element")
	}
	edit := &StructureEdit{Change: StructureChange{Kind: "element-move", Locator: op.Locator, Anchor: op.Anchor, Position: op.Position, Block: bytes.Clone(d.Input[start:end]), At: at}}
	edit.Spans = append(edit.Spans, EditSpan{Start: start, End: end})
	edit.Points = append(edit.Points, EditPoint{At: at, Bytes: bytes.Clone(d.Input[start:end])})
	return edit, nil
}

func elementAttribute(e *xmltext.Element, name xml.Name) (string, bool) {
	for _, a := range e.Attributes {
		if a.Name == name {
			return a.Value, true
		}
	}
	return "", false
}

// isIDAttribute reports whether a resolved attribute name carries document
// identity for the reference index: unprefixed id and xml:id.
func isIDAttribute(name xml.Name) bool {
	return (name.Space == "" || name.Space == XMLNamespace) && name.Local == "id"
}

// OverlapError reports an edit conflict inside one resource.
type OverlapError struct{ Message string }

func (e *OverlapError) Error() string { return e.Message }

// ValidateEdits rejects duplicate, aliased and overlapping targets before any
// byte is written: spans must not overlap, insert points must stay strictly
// outside every replaced range and must not coincide with each other. One
// insertion point per offset keeps byte order and the independent tree
// simulation in exact agreement.
func ValidateEdits(edits []*StructureEdit) error {
	type item struct {
		opIndex    int
		start, end int
		point      bool
	}
	items := []item{}
	for _, e := range edits {
		for _, s := range e.Spans {
			items = append(items, item{e.OpIndex, s.Start, s.End, false})
		}
		for _, p := range e.Points {
			items = append(items, item{e.OpIndex, p.At, p.At, true})
		}
	}
	for i := range items {
		for j := i + 1; j < len(items); j++ {
			a, b := items[i], items[j]
			if a.point && b.point {
				if a.start == b.start {
					return &OverlapError{fmt.Sprintf("operations %d and %d insert at the same point", a.opIndex, b.opIndex)}
				}
				continue
			}
			if !a.point && !b.point {
				if a.start < b.end && b.start < a.end {
					return &OverlapError{fmt.Sprintf("operations %d and %d have overlapping target ranges", a.opIndex, b.opIndex)}
				}
				continue
			}
			point, span := a, b
			if !a.point {
				point, span = b, a
			}
			if point.start >= span.start && point.start <= span.end {
				return &OverlapError{fmt.Sprintf("operation %d inserts at or inside the target range of operation %d", point.opIndex, span.opIndex)}
			}
		}
	}
	return nil
}

// ApplyEdits applies every span and point to the frozen bytes. Larger offsets
// are applied first, so earlier coordinates stay valid. ValidateEdits has
// already refused insertions at or inside a replaced range, so the remaining
// tie at one offset is defensive: a removal is applied before an insertion.
func ApplyEdits(input []byte, edits []*StructureEdit) []byte {
	type item struct {
		opIndex    int
		start, end int
		bytes      []byte
		point      bool
	}
	items := []item{}
	for _, e := range edits {
		for _, s := range e.Spans {
			items = append(items, item{e.OpIndex, s.Start, s.End, s.Bytes, false})
		}
		for _, p := range e.Points {
			items = append(items, item{e.OpIndex, p.At, p.At, p.Bytes, true})
		}
	}
	slices.SortStableFunc(items, func(a, b item) int {
		if a.start != b.start {
			return b.start - a.start
		}
		if a.point != b.point {
			if a.point {
				return 1
			}
			return -1
		}
		if a.point && b.point {
			return b.opIndex - a.opIndex
		}
		return 0
	})
	out := bytes.Clone(input)
	for _, it := range items {
		next := make([]byte, 0, len(out)-(it.end-it.start)+len(it.bytes))
		next = append(next, out[:it.start]...)
		next = append(next, it.bytes...)
		next = append(next, out[it.end:]...)
		out = next
	}
	return out
}

// ---- independent verification of a spliced result ----

type simNode struct {
	name       xml.Name
	attrs      []xml.Attr
	direct     string
	children   []*simNode
	parent     *simNode
	loc        string
	block      []byte
	blockNodes int
	position   string
	anchor     string
	blockAt    int
}

// blockLocator names a block node that has no frozen locator, so verification
// still checks its exact bytes at its final position.
func blockLocator(kind string, opIndex int) string {
	return "@" + kind + ":" + strconv.Itoa(opIndex)
}

// expectedBlockOffset maps a frozen insertion point or replaced-range start to
// its byte offset in the spliced output. It recomputes the accumulated size
// change of every earlier edit instead of reusing the splice implementation, so
// a block cannot be accepted at another occurrence of the same bytes.
func expectedBlockOffset(offset int, edits []*StructureEdit) int {
	delta := 0
	for _, e := range edits {
		for _, span := range e.Spans {
			if span.Start >= offset {
				continue
			}
			delta += len(span.Bytes) - (span.End - span.Start)
		}
		for _, p := range e.Points {
			if p.At >= offset {
				continue
			}
			delta += len(p.Bytes)
		}
	}
	return offset + delta
}

func buildSim(e *xmltext.Element) *simNode {
	n := &simNode{name: e.Name, attrs: e.Attributes, direct: e.DirectText, loc: e.Location}
	for _, c := range e.Children {
		child := buildSim(c)
		child.parent = n
		n.children = append(n.children, child)
	}
	return n
}

func convertFragmentNodes(nodes []*FragmentNode) []*simNode {
	out := []*simNode{}
	for _, f := range nodes {
		n := &simNode{name: f.Name, attrs: f.Attrs, direct: f.Direct}
		for _, c := range f.Children {
			child := convertFragmentNodes([]*FragmentNode{c})[0]
			child.parent = n
			n.children = append(n.children, child)
		}
		out = append(out, n)
	}
	return out
}

func setAttribute(attrs []xml.Attr, attr xml.Attr) []xml.Attr {
	out := slices.Clone(attrs)
	for i, a := range out {
		if a.Name == attr.Name {
			out[i] = attr
			return out
		}
	}
	return append(out, attr)
}

func removeAttribute(attrs []xml.Attr, name xml.Name) []xml.Attr {
	out := make([]xml.Attr, 0, len(attrs))
	for _, a := range attrs {
		if a.Name != name {
			out = append(out, a)
		}
	}
	return out
}

func (n *simNode) detach() {
	if n.parent == nil {
		return
	}
	parent := n.parent
	children := make([]*simNode, 0, len(parent.children)-1)
	for _, c := range parent.children {
		if c != n {
			children = append(children, c)
		}
	}
	parent.children = children
	n.parent = nil
}

func insertRelative(anchor *simNode, position string, nodes []*simNode, block []byte) error {
	parent := anchor.parent
	if parent == nil {
		return fmt.Errorf("insertion anchor has no parent")
	}
	index := 0
	for index < len(parent.children) && parent.children[index] != anchor {
		index++
	}
	if index == len(parent.children) {
		return fmt.Errorf("insertion anchor is not a child of its parent")
	}
	switch position {
	case "before":
		parent.children = slices.Insert(parent.children, index, nodes...)
	case "after":
		parent.children = slices.Insert(parent.children, index+1, nodes...)
	case "first-child":
		parent = anchor
		parent.children = slices.Insert(parent.children, 0, nodes...)
	case "last-child":
		parent = anchor
		parent.children = append(parent.children, nodes...)
	default:
		return fmt.Errorf("unsupported position %q", position)
	}
	for _, n := range nodes {
		n.parent = parent
	}
	if len(nodes) > 0 {
		nodes[0].block, nodes[0].blockNodes, nodes[0].position, nodes[0].anchor = block, len(nodes), position, anchor.loc
	}
	return nil
}

// SimulateStructure applies the operations to a tree-domain copy of the frozen
// document. It is a second, independent expression of the operation semantics,
// used to verify the byte-spliced result.
func SimulateStructure(doc *StructureDocument, edits []*StructureEdit) (*simNode, map[string]*simNode, error) {
	root := buildSim(doc.Doc.Root)
	byLoc := map[string]*simNode{}
	var index func(*simNode)
	index = func(n *simNode) {
		if n.loc != "" {
			byLoc[n.loc] = n
		}
		for _, c := range n.children {
			index(c)
		}
	}
	index(root)
	for _, e := range edits {
		ch := e.Change
		switch ch.Kind {
		case "attribute-set":
			n := byLoc[ch.Locator]
			if n == nil {
				return nil, nil, fmt.Errorf("simulation lost target %s", ch.Locator)
			}
			for _, decl := range ch.Decls {
				n.attrs = setAttribute(n.attrs, decl)
			}
			n.attrs = setAttribute(n.attrs, *ch.Attr)
		case "attribute-remove":
			n := byLoc[ch.Locator]
			if n == nil {
				return nil, nil, fmt.Errorf("simulation lost target %s", ch.Locator)
			}
			n.attrs = removeAttribute(n.attrs, ch.Attr.Name)
		case "text-set":
			n := byLoc[ch.Locator]
			if n == nil {
				return nil, nil, fmt.Errorf("simulation lost target %s", ch.Locator)
			}
			n.direct = ch.Text
		case "element-delete":
			n := byLoc[ch.Locator]
			if n == nil {
				return nil, nil, fmt.Errorf("simulation lost target %s", ch.Locator)
			}
			n.detach()
		case "element-insert":
			anchor := byLoc[ch.Locator]
			if anchor == nil {
				return nil, nil, fmt.Errorf("simulation lost anchor %s", ch.Locator)
			}
			nodes := convertFragmentNodes(ch.Fragment.Nodes)
			if err := insertRelative(anchor, ch.Position, nodes, ch.Block); err != nil {
				return nil, nil, err
			}
			if len(nodes) > 0 {
				nodes[0].blockAt = ch.At
				nodes[0].loc = blockLocator("insert", e.OpIndex)
				byLoc[nodes[0].loc] = nodes[0]
			}
		case "element-replace":
			target := byLoc[ch.Locator]
			if target == nil {
				return nil, nil, fmt.Errorf("simulation lost target %s", ch.Locator)
			}
			nodes := convertFragmentNodes(ch.Fragment.Nodes)
			parent := target.parent
			if parent == nil {
				return nil, nil, fmt.Errorf("replacement target has no parent")
			}
			for i, c := range parent.children {
				if c == target {
					parent.children = slices.Replace(parent.children, i, i+1, nodes...)
					break
				}
			}
			for _, n := range nodes {
				n.parent = parent
			}
			if len(nodes) > 0 {
				nodes[0].block, nodes[0].blockNodes, nodes[0].position = ch.Block, len(nodes), "replace"
				nodes[0].blockAt = ch.At
				nodes[0].loc = blockLocator("replace", e.OpIndex)
				byLoc[nodes[0].loc] = nodes[0]
			}
		case "element-move":
			moved := byLoc[ch.Locator]
			anchor := byLoc[ch.Anchor]
			if moved == nil || anchor == nil {
				return nil, nil, fmt.Errorf("simulation lost move source or anchor")
			}
			moved.detach()
			if err := insertRelative(anchor, ch.Position, []*simNode{moved}, ch.Block); err != nil {
				return nil, nil, err
			}
			moved.blockAt = ch.At
		default:
			return nil, nil, fmt.Errorf("unsupported change %q", ch.Kind)
		}
	}
	return root, byLoc, nil
}

// VerifyStructure re-parses the spliced output and requires it to match the
// simulated tree, including every inserted block's exact bytes and destination.
func VerifyStructure(doc *StructureDocument, edits []*StructureEdit, output []byte) error {
	expected, expectedByLoc, err := SimulateStructure(doc, edits)
	if err != nil {
		return err
	}
	check, err := xmltext.Parse(output)
	if err != nil {
		return fmt.Errorf("spliced resource is not well-formed: %v", err)
	}
	if err := check.CheckProfile(doc.profile); err != nil {
		return fmt.Errorf("spliced resource profile: %v", err)
	}
	if err := check.RequireComplete(); err != nil {
		return fmt.Errorf("spliced resource is incomplete: %v", err)
	}
	actualByLoc := map[string]*xmltext.Element{}
	if err := compareStructure(expected, check.Root, output, actualByLoc); err != nil {
		return err
	}
	checked := 0
	for loc, exp := range expectedByLoc {
		if exp.block == nil {
			continue
		}
		act := actualByLoc[loc]
		if act == nil {
			return fmt.Errorf("inserted element %s is missing", loc)
		}
		if err := checkBlockBytes(output, exp, expectedBlockOffset(exp.blockAt, edits)); err != nil {
			return err
		}
		if err := checkBlockPlacement(output, exp, act, actualByLoc); err != nil {
			return err
		}
		checked++
	}
	// Every operation that produced a block must be checked: a block without a
	// locator would silently skip its byte placement check.
	want := 0
	for _, e := range edits {
		if e.Change.Block != nil {
			want++
		}
	}
	if checked != want {
		return fmt.Errorf("verified %d of %d inserted blocks", checked, want)
	}
	return nil
}

func compareStructure(expected *simNode, actual *xmltext.Element, output []byte, byLoc map[string]*xmltext.Element) error {
	if expected.name != actual.Name {
		return fmt.Errorf("element %s changed name from %s to %s", expected.loc, expected.name.Local, actual.Name.Local)
	}
	if len(expected.attrs) != len(actual.Attributes) {
		return fmt.Errorf("element %s attribute count changed", expected.loc)
	}
	for i := range expected.attrs {
		if expected.attrs[i].Name != actual.Attributes[i].Name || expected.attrs[i].Value != actual.Attributes[i].Value {
			return fmt.Errorf("element %s attribute %s changed", expected.loc, expected.attrs[i].Name.Local)
		}
	}
	if expected.direct != actual.DirectText {
		return fmt.Errorf("element %s direct text changed", expected.loc)
	}
	if len(expected.children) != len(actual.Children) {
		return fmt.Errorf("element %s child count changed", expected.loc)
	}
	if expected.loc != "" {
		byLoc[expected.loc] = actual
	}
	for i, child := range expected.children {
		if err := compareStructure(child, actual.Children[i], output, byLoc); err != nil {
			return err
		}
	}
	return nil
}

// checkBlockBytes requires the exact block bytes at the offset the frozen edit
// facts imply, so a block cannot be accepted at another occurrence or after
// surrounding text it does not belong to.
func checkBlockBytes(output []byte, exp *simNode, offset int) error {
	if len(exp.block) == 0 {
		return nil
	}
	if offset < 0 || offset+len(exp.block) > len(output) || !bytes.Equal(output[offset:offset+len(exp.block)], exp.block) {
		return fmt.Errorf("block bytes are not at the planned position")
	}
	return nil
}

func checkBlockPlacement(output []byte, exp *simNode, act *xmltext.Element, byLoc map[string]*xmltext.Element) error {
	if len(exp.block) == 0 {
		return nil
	}
	parent := act.Parent
	if parent == nil {
		return fmt.Errorf("inserted block has no parent")
	}
	switch exp.position {
	case "first-child":
		if !bytes.HasPrefix(output[parent.OpenEnd:], exp.block) {
			return fmt.Errorf("inserted block is not the first child content")
		}
	case "last-child":
		if !bytes.HasSuffix(output[:parent.CloseStart], exp.block) {
			return fmt.Errorf("inserted block is not the last child content")
		}
	case "before":
		anchor := byLoc[exp.anchor]
		if anchor == nil || !bytes.HasSuffix(output[:anchor.OpenStart], exp.block) {
			return fmt.Errorf("inserted block is not immediately before its anchor")
		}
	case "after":
		anchor := byLoc[exp.anchor]
		if anchor == nil || !bytes.HasPrefix(output[anchor.CloseEnd:], exp.block) {
			return fmt.Errorf("inserted block is not immediately after its anchor")
		}
	case "replace":
		// checkBlockBytes already requires the block at the replaced range's
		// mapped offset; the structural comparison pins its parent and order.
		if !bytes.Contains(output, exp.block) {
			return fmt.Errorf("replacement block bytes are absent")
		}
	default:
		return fmt.Errorf("unsupported block position %q", exp.position)
	}
	return nil
}

// PlannedTarget returns the child-index path of the element that the simulated
// edits place at locator, so a caller can follow the same path in the candidate
// bytes. The path starts at the document root's children; verification proves
// the candidate has the same child structure.
func PlannedTarget(doc *StructureDocument, edits []*StructureEdit, locator string) ([]int, error) {
	_, byLoc, err := SimulateStructure(doc, edits)
	if err != nil {
		return nil, err
	}
	target := byLoc[locator]
	if target == nil {
		return nil, fmt.Errorf("locator %q is not part of the planned structure", locator)
	}
	path := []int{}
	for n := target; n.parent != nil; n = n.parent {
		index := -1
		for i, c := range n.parent.children {
			if c == n {
				index = i
				break
			}
		}
		if index < 0 {
			return nil, fmt.Errorf("planned target lost its parent link")
		}
		path = append([]int{index}, path...)
	}
	return path, nil
}

// countBlockNodes counts the top-level fragment nodes emitted by one block.
func countBlockNodes(n *simNode) int { return n.blockNodes }
