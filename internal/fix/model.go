// Package fix derives native, deterministic fix proposals from frozen workspace
// facts. Proposals are read-only sources: they never write a candidate, accepted
// revision or history, and they bind every repair to the exact frozen baseline.
package fix

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/publication"
)

// ProposalVersion is the FixProposal wire version frozen by the T2 batch-5
// contract.
const ProposalVersion = 1

// Checker is the native check identity; the two rules never impersonate an
// upstream checker code.
const Checker = "kepub-native"

// CheckVersion is the native check version.
const CheckVersion = 1

// Rule IDs, versions and fixed facts of the first batch.
const (
	RuleEpubTypeProhibited = "kepub.fix.epub-type-on-prohibited-element"
	RuleRelativeURLQuery   = "kepub.fix.relative-url-query-component"

	FactEpubTypeProhibited = "xhtml-metadata-content-epub-type"
	FactRelativeURLQuery   = "xhtml-relative-url-query"

	RiskEpubTypeProhibited = "removes an author semantic annotation that the frozen spec prohibits here"
	RiskRelativeURLQuery   = "removing the query component can discard author-defined URL semantics; explicit selection and review required"

	SpecVersion = "EPUB 3.3"
)

// Status values of a repair.
const (
	StatusFixable   = "fixable"
	StatusUnfixable = "unfixable"
)

// WorkspaceRef is the frozen workspace identity of a proposal. The values are
// workspace facts, never Git commit or tree IDs.
type WorkspaceRef struct {
	WorkspaceID     string `json:"workspaceId"`
	Rootfile        string `json:"rootfile"`
	BaseRevision    string `json:"baseRevision"`
	InputTreeSHA256 string `json:"inputTreeSha256"`
}

// RuleRef identifies one rule and its version.
type RuleRef struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

// SpecRef cites the frozen normative clause of a rule fact.
type SpecRef struct {
	SpecVersion    string `json:"specVersion"`
	Section        string `json:"section"`
	NormativeLevel string `json:"normativeLevel"`
}

// Basis is the native check basis of a repair.
type Basis struct {
	Checker      string  `json:"checker"`
	CheckVersion int     `json:"checkVersion"`
	Fact         string  `json:"fact"`
	Spec         SpecRef `json:"spec"`
}

// AttributeRef is an expanded attribute name.
type AttributeRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// Target is the exact frozen binding of one repair. The locator is the frozen
// structural locator; the expected old value is the frozen decoded value.
type Target struct {
	BookPath         string       `json:"bookPath"`
	ResourceSHA256   string       `json:"resourceSha256"`
	LocatorVersion   int          `json:"locatorVersion"`
	Locator          string       `json:"locator"`
	Element          string       `json:"element"`
	Attribute        AttributeRef `json:"attribute"`
	ExpectedOldValue string       `json:"expectedOldValue"`
}

// Operation is one existing v1 operation carried by a repair. It is the same
// wire shape the workspace already accepts.
type Operation struct {
	ID      string `json:"operationId"`
	Version int    `json:"operationVersion"`
	Params  any    `json:"params"`
}

// Repair is one applicable rule fact. Unfixable repairs carry a null operation
// and a deterministic reason; they are listed for review and never executed.
type Repair struct {
	RepairID        string     `json:"repairId"`
	Rule            RuleRef    `json:"rule"`
	Basis           Basis      `json:"basis"`
	Target          Target     `json:"target"`
	Status          string     `json:"status"`
	Operation       *Operation `json:"operation"`
	Risk            string     `json:"risk"`
	UnfixableReason string     `json:"unfixableReason,omitempty"`
	ReadSet         []string   `json:"readSet"`
	WriteSet        []string   `json:"writeSet"`
}

// Limitation records a covered-range boundary: a fact that could not be
// expressed as an executable repair is listed instead of being silently treated
// as absent.
type Limitation struct {
	Rule     RuleRef `json:"rule"`
	BookPath string  `json:"bookPath"`
	Locator  string  `json:"locator"`
	Reason   string  `json:"reason"`
}

// Selection is the frozen selection of a proposal.
type Selection struct {
	Mode      string   `json:"mode"`
	RepairIDs []string `json:"repairIds"`
}

// Selection modes.
const (
	ModeAll      = "all"
	ModeExplicit = "explicit"
)

// DerivedFrom references the complete all proposal a selection was derived from.
type DerivedFrom struct {
	ProposalID     string `json:"proposalId"`
	ProposalSHA256 string `json:"proposalSha256"`
}

// Derived is the merged dependency and write facts of the selected repairs.
type Derived struct {
	Operations []Operation `json:"operations"`
	ReadSet    []string    `json:"readSet"`
	WriteSet   []string    `json:"writeSet"`
}

// Proposal is the complete FixProposal v1 source.
type Proposal struct {
	ProposalVersion int          `json:"proposalVersion"`
	Workspace       WorkspaceRef `json:"workspace"`
	Rules           []RuleRef    `json:"rules"`
	Selection       Selection    `json:"selection"`
	DerivedFrom     *DerivedFrom `json:"derivedFrom,omitempty"`
	Repairs         []Repair     `json:"repairs"`
	Limitations     []Limitation `json:"limitations"`
	Derived         Derived      `json:"derived"`
	ProposalID      string       `json:"proposalId"`
	ProposalSHA256  string       `json:"proposalSha256"`
}

// Canonical renders the frozen canonical JSON: Go's default string encoding with
// recursively key-sorted objects, no whitespace. It reuses encoding/json rather
// than introducing another serializer.
func Canonical(doc any) ([]byte, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, err
	}
	return json.Marshal(tree)
}

// HashInput is the canonical bytes whose SHA-256 signs a proposal: the document
// without its two derived top-level fields.
func (p Proposal) HashInput() ([]byte, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var tree map[string]any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, err
	}
	delete(tree, "proposalId")
	delete(tree, "proposalSha256")
	return json.Marshal(tree)
}

// Sign fills proposalId and proposalSha256 from the canonical content.
func (p *Proposal) Sign() error {
	b, err := p.HashInput()
	if err != nil {
		return err
	}
	sum := sha256.Sum256(b)
	p.ProposalSHA256 = hex.EncodeToString(sum[:])
	p.ProposalID = "kepub-fix-proposal-v1:sha256:" + p.ProposalSHA256
	return nil
}

// VerifyHash checks that the two derived fields are self-consistent.
func (p Proposal) VerifyHash() error {
	want := p
	if err := want.Sign(); err != nil {
		return err
	}
	if p.ProposalSHA256 != want.ProposalSHA256 || p.ProposalID != want.ProposalID {
		return fmt.Errorf("proposal hash is not self-consistent")
	}
	return nil
}

// RepairID derives the deterministic repair identity. The locator never
// contains "|", so the tuple is unambiguous.
func RepairID(ruleID string, version int, bp bookpath.BookPath, locator string) string {
	sum := sha256.Sum256([]byte(ruleID + "|" + fmt.Sprint(version) + "|" + string(bp) + "|" + locator))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// CanonicalEqual reports whether two documents have identical canonical bytes.
func CanonicalEqual(a, b any) bool {
	ab, err := Canonical(a)
	if err != nil {
		return false
	}
	bb, err := Canonical(b)
	if err != nil {
		return false
	}
	return string(ab) == string(bb)
}

// sortedUnique returns the sorted, de-duplicated copy of values.
func sortedUnique(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return compact(out)
}

func compact(values []string) []string {
	out := values[:0]
	for i, v := range values {
		if i == 0 || v != values[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// paramBookPath returns the book path of an operation's params when known.
func paramBookPath(params any) string {
	switch p := params.(type) {
	case publication.AttributeRemove:
		return string(p.BookPath)
	case publication.AttributeSet:
		return string(p.BookPath)
	}
	return ""
}

// paramLocator returns the structural locator of an operation's params.
func paramLocator(params any) string {
	switch p := params.(type) {
	case publication.AttributeRemove:
		return p.Locator
	case publication.AttributeSet:
		return p.Locator
	}
	return ""
}

// sortOperations orders operations by (bookPath, operationId, locator) as the
// frozen contract requires; duplicates are kept, never dropped.
func sortOperations(ops []Operation) {
	sort.SliceStable(ops, func(i, j int) bool {
		a, b := ops[i], ops[j]
		if pa, pb := paramBookPath(a.Params), paramBookPath(b.Params); pa != pb {
			return pa < pb
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return paramLocator(a.Params) < paramLocator(b.Params)
	})
}

// OperationKey renders one operation for multiset comparison.
func OperationKey(op Operation) string {
	b, err := Canonical(op)
	if err != nil {
		return ""
	}
	return string(b)
}

// OperationMultiset compares two operation arrays as multisets: same length,
// same elements with the same multiplicity. Order is not significant.
func OperationMultiset(a, b []Operation) bool {
	if len(a) != len(b) {
		return false
	}
	left := make([]string, 0, len(a))
	right := make([]string, 0, len(b))
	for _, op := range a {
		left = append(left, OperationKey(op))
	}
	for _, op := range b {
		right = append(right, OperationKey(op))
	}
	sort.Strings(left)
	sort.Strings(right)
	return strings.Join(left, "\x00") == strings.Join(right, "\x00")
}
