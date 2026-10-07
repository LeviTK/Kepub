package fix

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrSelection marks an invalid selection (empty, unknown or duplicate IDs).
var ErrSelection = errors.New("invalid selection")

// ErrNoOperations marks an emit-request attempt without executable operations.
var ErrNoOperations = errors.New("proposal has no executable operations")

// ParseSelection parses one --select value: comma-separated repair IDs.
func ParseSelection(value string) (Selection, error) {
	if strings.TrimSpace(value) == "" {
		return Selection{}, fmt.Errorf("%w: empty selection", ErrSelection)
	}
	seen := map[string]bool{}
	ids := []string{}
	for _, part := range strings.Split(value, ",") {
		id := strings.TrimSpace(part)
		if id == "" || seen[id] {
			return Selection{}, fmt.Errorf("%w: empty or duplicate repair id", ErrSelection)
		}
		seen[id] = true
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return Selection{Mode: ModeExplicit, RepairIDs: ids}, nil
}

// facts is the derived rule output of one snapshot.
type facts struct {
	repairs     []Repair
	limitations []Limitation
}

// derive runs both rules on the frozen snapshot.
func derive(s Snapshot) facts {
	repairs := []Repair{}
	limits := []Limitation{}
	r1, l1 := deriveEpubType(s)
	r2, l2 := deriveRelativeURLQuery(s)
	repairs = append(repairs, r1...)
	repairs = append(repairs, r2...)
	limits = append(limits, l1...)
	limits = append(limits, l2...)
	sort.SliceStable(repairs, func(i, j int) bool {
		a, b := repairs[i], repairs[j]
		if a.Rule.ID != b.Rule.ID {
			return a.Rule.ID < b.Rule.ID
		}
		if a.Rule.Version != b.Rule.Version {
			return a.Rule.Version < b.Rule.Version
		}
		if a.Target.BookPath != b.Target.BookPath {
			return a.Target.BookPath < b.Target.BookPath
		}
		return a.Target.Locator < b.Target.Locator
	})
	sort.SliceStable(limits, func(i, j int) bool {
		a, b := limits[i], limits[j]
		if a.Rule.ID != b.Rule.ID {
			return a.Rule.ID < b.Rule.ID
		}
		if a.Rule.Version != b.Rule.Version {
			return a.Rule.Version < b.Rule.Version
		}
		if a.BookPath != b.BookPath {
			return a.BookPath < b.BookPath
		}
		if a.Locator != b.Locator {
			return a.Locator < b.Locator
		}
		return a.Reason < b.Reason
	})
	limits = dedupeLimitations(limits)
	return facts{repairs, limits}
}

func dedupeLimitations(in []Limitation) []Limitation {
	out := in[:0]
	for i, l := range in {
		if i == 0 || l != in[i-1] {
			out = append(out, l)
		}
	}
	return out
}

// Propose derives a complete FixProposal v1 for one frozen snapshot and one
// selection. An explicit selection must reference repair IDs of the all
// proposal of the same baseline; unknown, duplicate or empty selections fail.
func Propose(s Snapshot, sel Selection) (Proposal, error) {
	f := derive(s)
	switch sel.Mode {
	case ModeAll:
		return assemble(s, f, Selection{Mode: ModeAll, RepairIDs: []string{}}, nil)
	case ModeExplicit:
		if len(sel.RepairIDs) == 0 {
			return Proposal{}, fmt.Errorf("%w: empty selection", ErrSelection)
		}
		byID := map[string]Repair{}
		for _, r := range f.repairs {
			byID[r.RepairID] = r
		}
		selected := []Repair{}
		for _, id := range sel.RepairIDs {
			r, ok := byID[id]
			if !ok {
				return Proposal{}, fmt.Errorf("%w: unknown repair id %s", ErrSelection, id)
			}
			selected = append(selected, r)
		}
		all, err := assemble(s, f, Selection{Mode: ModeAll, RepairIDs: []string{}}, nil)
		if err != nil {
			return Proposal{}, err
		}
		from := &DerivedFrom{ProposalID: all.ProposalID, ProposalSHA256: all.ProposalSHA256}
		return assemble(s, facts{f.repairs, f.limitations}, Selection{Mode: ModeExplicit, RepairIDs: sortedUnique(sel.RepairIDs)}, from, selected...)
	}
	return Proposal{}, fmt.Errorf("%w: unknown selection mode", ErrSelection)
}

// assemble builds one proposal from the derived facts.
func assemble(s Snapshot, f facts, sel Selection, from *DerivedFrom, selected ...Repair) (Proposal, error) {
	repairs := []Repair{}
	if sel.Mode == ModeExplicit {
		repairs = append(repairs, selected...)
		sort.SliceStable(repairs, func(i, j int) bool {
			a, b := repairs[i], repairs[j]
			if a.Rule.ID != b.Rule.ID {
				return a.Rule.ID < b.Rule.ID
			}
			if a.Target.BookPath != b.Target.BookPath {
				return a.Target.BookPath < b.Target.BookPath
			}
			return a.Target.Locator < b.Target.Locator
		})
	} else {
		repairs = append(repairs, f.repairs...)
	}
	rules := []RuleRef{}
	ops := []Operation{}
	read := []string{}
	write := []string{}
	seenRule := map[RuleRef]bool{}
	for _, r := range repairs {
		if !seenRule[r.Rule] {
			seenRule[r.Rule] = true
			rules = append(rules, r.Rule)
		}
		read = append(read, r.ReadSet...)
		if r.Status == StatusFixable && r.Operation != nil {
			ops = append(ops, *r.Operation)
			write = append(write, r.WriteSet...)
		}
	}
	sort.SliceStable(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	sortOperations(ops)
	p := Proposal{
		ProposalVersion: ProposalVersion,
		Workspace:       s.Workspace,
		Rules:           rules,
		Selection:       sel,
		DerivedFrom:     from,
		Repairs:         repairs,
		Limitations:     f.limitations,
		Derived:         Derived{Operations: ops, ReadSet: sortedUnique(read), WriteSet: sortedUnique(write)},
	}
	if p.Selection.RepairIDs == nil {
		p.Selection.RepairIDs = []string{}
	}
	if err := p.Sign(); err != nil {
		return Proposal{}, err
	}
	return p, nil
}

// HasFR2Operations reports whether a proposal would execute FR-2 operations.
// Such a proposal may be displayed, but emit-request and schema 7 require an
// explicit selection; an all selection never authorizes this rule.
func HasFR2Operations(p Proposal) bool {
	for _, r := range p.Repairs {
		if r.Rule.ID == RuleRelativeURLQuery && r.Status == StatusFixable {
			return true
		}
	}
	return false
}

// RequestAllowed applies the frozen selection limits to one proposal before it
// may become an executable schema 7 request.
func RequestAllowed(p Proposal) error {
	if p.Selection.Mode != ModeAll {
		return nil
	}
	if HasFR2Operations(p) {
		return fmt.Errorf("%w: the relative URL query rule requires an explicit --select", ErrSelection)
	}
	return nil
}

// Validate re-derives the proposal from the frozen snapshot and compares the
// complete canonical content, so a re-signed tamper of any field is refused.
// The workspace binding is checked separately by the caller.
func Validate(s Snapshot, p Proposal) error {
	if p.ProposalVersion != ProposalVersion {
		return fmt.Errorf("unsupported proposal version")
	}
	if p.Selection.Mode != ModeAll && p.Selection.Mode != ModeExplicit {
		return fmt.Errorf("unsupported selection mode")
	}
	if err := p.VerifyHash(); err != nil {
		return err
	}
	recomputed, err := Propose(s, p.Selection)
	if err != nil {
		return err
	}
	if !CanonicalEqual(p, recomputed) {
		return fmt.Errorf("proposal does not match the frozen baseline re-derivation")
	}
	return nil
}
