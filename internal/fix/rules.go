package fix

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"sort"
	"strings"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

// Snapshot is the frozen baseline a proposal is derived from. Every value comes
// from the workspace's accepted revision, never from Git metadata.
type Snapshot struct {
	Workspace WorkspaceRef
	Version   string     // package document version, e.g. "3.0"
	Container []byte     // META-INF/container.xml
	Package   []byte     // the selected package document
	Resources []Resource // manifest application/xhtml+xml entries, sorted by path
	Inventory []string   // every container path, sorted
}

// Resource is one frozen XHTML resource of the manifest.
type Resource struct {
	Path  bookpath.BookPath
	Bytes []byte
}

// resourceHash returns the frozen resource SHA-256.
func resourceHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// hasPath reports whether the frozen container holds one path.
func (s Snapshot) hasPath(p bookpath.BookPath) bool {
	i := sort.SearchStrings(s.Inventory, string(p))
	return i < len(s.Inventory) && s.Inventory[i] == string(p)
}

// resource returns one parsed manifest resource.
func (s Snapshot) resource(p bookpath.BookPath) (Resource, bool) {
	for _, r := range s.Resources {
		if r.Path == p {
			return r, true
		}
	}
	return Resource{}, false
}

// profile is the XHTML profile of the frozen package version.
func (s Snapshot) profile() xmltext.Profile {
	return xmltext.Profile{Version: s.Version, MediaType: "application/xhtml+xml"}
}

// epub3 reports whether the frozen package is an EPUB 3 publication. The two
// rules never apply the EPUB 3 specification to EPUB 2 inputs.
func (s Snapshot) epub3() bool {
	return strings.HasPrefix(s.Version, "3")
}

// metadataContentElements is the frozen HTML §3.2.5.2.1 metadata content set
// plus the head element itself (EPUB 3.3 §6.1.3.1).
var metadataContentElements = map[string]bool{
	"head": true, "base": true, "link": true, "meta": true,
	"noscript": true, "script": true, "style": true, "template": true, "title": true,
}

// urlAttribute returns the no-namespace URL attribute the frozen FR-2 context
// covers for one element, if any.
func urlAttribute(local string) string {
	switch local {
	case "a", "area", "link":
		return "href"
	case "img":
		return "src"
	}
	return ""
}

// ruleRefs of the two first-batch rules.
var (
	ruleEpubType    = RuleRef{RuleEpubTypeProhibited, 1}
	ruleRelativeURL = RuleRef{RuleRelativeURLQuery, 1}
)

func epubTypeSpec() SpecRef {
	return SpecRef{SpecVersion: SpecVersion, Section: "6.1.3.1", NormativeLevel: "MUST NOT"}
}

func relativeURLSpec() SpecRef {
	return SpecRef{SpecVersion: SpecVersion, Section: "4.2.5", NormativeLevel: "MUST"}
}

// contextReadSet is the minimum read set every repair of one resource depends
// on: the container, the selected package document and the source resource.
func contextReadSet(s Snapshot, source bookpath.BookPath) []string {
	return sortedUnique([]string{"META-INF/container.xml", s.Workspace.Rootfile, string(source)})
}

// deriveEpubType derives every FR-1 fact of the frozen snapshot.
func deriveEpubType(s Snapshot) ([]Repair, []Limitation) {
	repairs := []Repair{}
	limits := []Limitation{}
	if !s.epub3() {
		return repairs, []Limitation{{ruleEpubType, "", "", "the frozen package is not an EPUB 3 publication; the EPUB 3 rule is not applied"}}
	}
	for _, res := range s.Resources {
		doc, err := publication.ParseStructureDocument(res.Bytes, res.Path, s.profile())
		if err != nil {
			limits = append(limits, Limitation{ruleEpubType, string(res.Path), "", "resource is not a complete XHTML document; the rule fact could not be derived"})
			continue
		}
		hash := resourceHash(res.Bytes)
		for _, e := range doc.Doc.Elements {
			opsAttr := xml.Name{Space: publication.OpsNamespace, Local: "type"}
			if e.Name.Space != publication.XHTMLNamespace {
				if _, ok := elementAttribute(e, opsAttr); ok {
					limits = append(limits, Limitation{ruleEpubType, string(res.Path), e.Location, "the ops type attribute is on a foreign-namespace element; the rule does not expand namespace coverage"})
				}
				continue
			}
			if !metadataContentElements[e.Name.Local] {
				continue
			}
			value, present := elementAttribute(e, opsAttr)
			if !present {
				continue
			}
			target := Target{
				BookPath: string(res.Path), ResourceSHA256: hash, LocatorVersion: 1,
				Locator: e.Location, Element: e.Name.Local,
				Attribute:        AttributeRef{Namespace: publication.OpsNamespace, Name: "type"},
				ExpectedOldValue: value,
			}
			repair := Repair{
				RepairID: RepairID(ruleEpubType.ID, ruleEpubType.Version, res.Path, e.Location),
				Rule:     ruleEpubType,
				Basis:    Basis{Checker, CheckVersion, FactEpubTypeProhibited, epubTypeSpec()},
				Target:   target,
				Risk:     RiskEpubTypeProhibited,
				ReadSet:  contextReadSet(s, res.Path),
				WriteSet: []string{string(res.Path)},
			}
			if _, ok := e.AttributeBytes(opsAttr); !ok {
				repair.Status = StatusUnfixable
				repair.UnfixableReason = "the attribute source interval is generated or unavailable; no byte edit is generated"
				repair.WriteSet = []string{}
			} else {
				repair.Status = StatusFixable
				repair.Operation = &Operation{ID: "xhtml.attribute.remove", Version: 1, Params: publication.AttributeRemove{
					BookPath:         res.Path,
					RevisionID:       s.Workspace.BaseRevision,
					ResourceSHA256:   hash,
					LocatorVersion:   1,
					Locator:          e.Location,
					Namespace:        publication.OpsNamespace,
					Name:             "type",
					ExpectedOldValue: value,
				}}
			}
			repairs = append(repairs, repair)
		}
	}
	return repairs, limits
}

// documentBase reports whether the document carries a base element with href or
// any xml:base attribute; both change URL resolution and make FR-2 repairs
// unprovable without re-basing.
func documentBase(doc *publication.StructureDocument) bool {
	for _, e := range doc.Doc.Elements {
		if e.Name.Space == publication.XHTMLNamespace && e.Name.Local == "base" {
			if _, ok := elementAttribute(e, xml.Name{Local: "href"}); ok {
				return true
			}
		}
		if _, ok := elementAttribute(e, xml.Name{Space: publication.XMLNamespace, Local: "base"}); ok {
			return true
		}
	}
	return false
}

// removeQuery strips one real query component (including an empty "?") from a
// decoded attribute value while preserving outer ASCII whitespace, path
// spelling, RawFragment and an explicit empty "#". A "?" inside the fragment or
// a "%3F" escape is not a query.
func removeQuery(value string) (string, bool) {
	trimmed := strings.Trim(value, " \t\n\r\f")
	if trimmed == "" {
		return "", false
	}
	head, tail := trimmed, ""
	if i := strings.IndexByte(trimmed, '#'); i >= 0 {
		head, tail = trimmed[:i], trimmed[i:]
	}
	i := strings.IndexByte(head, '?')
	if i < 0 {
		return "", false
	}
	stripped := head[:i] + tail
	if stripped == "" || stripped == trimmed {
		return "", false
	}
	lead := len(value) - len(strings.TrimLeft(value, " \t\n\r\f"))
	trail := len(value) - len(strings.TrimRight(value, " \t\n\r\f"))
	return value[:lead] + stripped + value[len(value)-trail:], true
}

// deriveRelativeURLQuery derives every FR-2 fact of the frozen snapshot.
func deriveRelativeURLQuery(s Snapshot) ([]Repair, []Limitation) {
	repairs := []Repair{}
	limits := []Limitation{}
	if !s.epub3() {
		return repairs, []Limitation{{ruleRelativeURL, "", "", "the frozen package is not an EPUB 3 publication; the EPUB 3 rule is not applied"}}
	}
	for _, res := range s.Resources {
		doc, err := publication.ParseStructureDocument(res.Bytes, res.Path, s.profile())
		if err != nil {
			limits = append(limits, Limitation{ruleRelativeURL, string(res.Path), "", "resource is not a complete XHTML document; the rule fact could not be derived"})
			continue
		}
		if documentBase(doc) {
			limits = append(limits, Limitation{ruleRelativeURL, string(res.Path), "", "a base element or xml:base changes URL resolution; no executable repair is generated"})
			continue
		}
		hash := resourceHash(res.Bytes)
		for _, e := range doc.Doc.Elements {
			if e.Name.Space != publication.XHTMLNamespace {
				continue
			}
			name := urlAttribute(e.Name.Local)
			if name == "" {
				continue
			}
			attrName := xml.Name{Local: name}
			value, present := elementAttribute(e, attrName)
			if !present {
				continue
			}
			// Only a real relative container URL is in scope: absolute,
			// network-path and path-absolute references are not applicable.
			if ref, err := bookpath.ResolveReference(res.Path, bookpath.Href(strings.Trim(value, " \t\n\r\f"))); err != nil || ref.External {
				continue
			}
			stripped, ok := removeQuery(value)
			if !ok {
				continue
			}
			target := Target{
				BookPath: string(res.Path), ResourceSHA256: hash, LocatorVersion: 1,
				Locator: e.Location, Element: e.Name.Local,
				Attribute:        AttributeRef{Namespace: "", Name: name},
				ExpectedOldValue: value,
			}
			repair := Repair{
				RepairID: RepairID(ruleRelativeURL.ID, ruleRelativeURL.Version, res.Path, e.Location),
				Rule:     ruleRelativeURL,
				Basis:    Basis{Checker, CheckVersion, FactRelativeURLQuery, relativeURLSpec()},
				Target:   target,
				Risk:     RiskRelativeURLQuery,
				ReadSet:  contextReadSet(s, res.Path),
				WriteSet: []string{string(res.Path)},
			}
			if _, ok := e.AttributeBytes(attrName); !ok {
				repair.Status = StatusUnfixable
				repair.UnfixableReason = "the attribute source interval is generated or unavailable; no byte edit is generated"
				repair.WriteSet = []string{}
				repairs = append(repairs, repair)
				continue
			}
			// The new value must resolve inside the frozen container with a
			// unique fragment; otherwise the repair is not generated.
			ref, err := bookpath.ResolveReference(res.Path, bookpath.Href(strings.Trim(stripped, " \t\n\r\f")))
			if err != nil || ref.External {
				repair.Status = StatusUnfixable
				repair.UnfixableReason = "the URL after query removal does not resolve to a container resource"
				repair.WriteSet = []string{}
				repairs = append(repairs, repair)
				continue
			}
			if !s.hasPath(ref.Path) {
				repair.Status = StatusUnfixable
				repair.UnfixableReason = "the target after query removal is not present in the frozen container"
				repair.WriteSet = []string{}
				repairs = append(repairs, repair)
				continue
			}
			if ref.Fragment != "" {
				targetRes, ok := s.resource(ref.Path)
				if !ok {
					repair.Status = StatusUnfixable
					repair.UnfixableReason = "the fragment target is not an indexable XHTML resource"
					repair.WriteSet = []string{}
					repairs = append(repairs, repair)
					continue
				}
				targetDoc, err := publication.ParseStructureDocument(targetRes.Bytes, targetRes.Path, s.profile())
				if err != nil || targetDoc.IDs()[ref.Fragment] != 1 {
					repair.Status = StatusUnfixable
					repair.UnfixableReason = "the fragment after query removal is not uniquely resolvable"
					repair.WriteSet = []string{}
					repairs = append(repairs, repair)
					continue
				}
				repair.ReadSet = sortedUnique(append(repair.ReadSet, string(ref.Path)))
			}
			repair.Status = StatusFixable
			repair.Operation = &Operation{ID: "xhtml.attribute.set", Version: 1, Params: publication.AttributeSet{
				BookPath:         res.Path,
				RevisionID:       s.Workspace.BaseRevision,
				ResourceSHA256:   hash,
				LocatorVersion:   1,
				Locator:          e.Location,
				Namespace:        "",
				Name:             name,
				ExpectedOldValue: &value,
				Value:            stripped,
			}}
			repairs = append(repairs, repair)
		}
	}
	return repairs, limits
}

// elementAttribute returns one attribute value of a parsed element.
func elementAttribute(e *xmltext.Element, name xml.Name) (string, bool) {
	for _, a := range e.Attributes {
		if a.Name == name {
			return a.Value, true
		}
	}
	return "", false
}
