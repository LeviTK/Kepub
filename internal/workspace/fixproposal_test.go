package workspace

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/fix"
)

const fixSource = `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head epub:type="secrecy"><title>One</title></head><body><p id="start">One.</p><a href="chapter2.xhtml?q=1#start2">Two</a></body></html>`

// fixSchema7JSON builds the frozen schema 7 request wire for one proposal.
func fixSchema7JSON(t *testing.T, p fix.Proposal) []byte {
	t.Helper()
	return editJSON(t, map[string]any{"schemaVersion": 7, "proposal": p, "operations": p.Derived.Operations})
}

// TestFixProposalSchema7Lifecycle runs the frozen proposal to plan/apply path:
// the derived repairs bind the frozen baseline, a schema 7 plan re-derives the
// whole proposal, and apply removes only the bound attribute bytes.
func TestFixProposalSchema7Lifecycle(t *testing.T) {
	w, dir, _ := structureWorkspace(t, map[string]string{"EPUB/chapter1.xhtml": fixSource})
	defer w.Close()
	s, err := w.FixSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	all, err := fix.Propose(s, fix.Selection{Mode: fix.ModeAll, RepairIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Repairs) != 2 || len(all.Limitations) != 0 {
		t.Fatalf("repairs %d limitations %d: %+v", len(all.Repairs), len(all.Limitations), all)
	}
	if all.Repairs[0].Rule.ID != fix.RuleEpubTypeProhibited || all.Repairs[1].Rule.ID != fix.RuleRelativeURLQuery {
		t.Fatalf("repair rules: %+v", all.Repairs)
	}
	if all.Repairs[0].Status != fix.StatusFixable || all.Repairs[1].Status != fix.StatusFixable {
		t.Fatalf("repair status: %+v", all.Repairs)
	}
	again, err := fix.Propose(s, fix.Selection{Mode: fix.ModeAll, RepairIDs: []string{}})
	if err != nil || !fix.CanonicalEqual(all, again) {
		t.Fatalf("proposal is not stable: %v", err)
	}
	if err := fix.RequestAllowed(all); err == nil {
		t.Fatal("an all selection authorized the relative URL query rule")
	}
	if _, err := w.Plan(fixSchema7JSON(t, all)); err == nil {
		t.Fatal("an all selection became an executable schema 7 request")
	}

	sel := fix.Selection{Mode: fix.ModeExplicit, RepairIDs: []string{all.Repairs[0].RepairID, all.Repairs[1].RepairID}}
	p, err := fix.Propose(s, sel)
	if err != nil {
		t.Fatal(err)
	}
	if p.DerivedFrom == nil || p.DerivedFrom.ProposalSHA256 != all.ProposalSHA256 {
		t.Fatalf("derivedFrom: %+v", p.DerivedFrom)
	}
	if err := fix.RequestAllowed(p); err != nil {
		t.Fatal(err)
	}
	plan, err := w.Plan(fixSchema7JSON(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if plan.SchemaVersion != 7 || plan.Proposal == nil || plan.Proposal.ProposalSHA256 != p.ProposalSHA256 {
		t.Fatalf("plan: %+v", plan)
	}
	e := applyPlan(t, w, plan)
	c1 := readResource(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"))
	if bytes.Contains(c1, []byte("secrecy")) || bytes.Contains(c1, []byte("?q=1")) {
		t.Fatalf("candidate still carries the violations: %s", c1)
	}
	if !bytes.Contains(c1, []byte(`href="chapter2.xhtml#start2"`)) {
		t.Fatalf("candidate lost the fragment: %s", c1)
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}

// TestFixProposalSchema7Refusals pins the frozen priority: self-consistency and
// re-derivation before the operation multiset, and the explicit selection limit
// of the relative URL rule.
func TestFixProposalSchema7Refusals(t *testing.T) {
	w, _, _ := structureWorkspace(t, map[string]string{"EPUB/chapter1.xhtml": fixSource})
	defer w.Close()
	s, err := w.FixSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	all, err := fix.Propose(s, fix.Selection{Mode: fix.ModeAll, RepairIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	sel := fix.Selection{Mode: fix.ModeExplicit, RepairIDs: []string{all.Repairs[0].RepairID}}
	p, err := fix.Propose(s, sel)
	if err != nil {
		t.Fatal(err)
	}

	tampered := p
	tampered.Repairs = append([]fix.Repair{}, p.Repairs...)
	tampered.Repairs[0].Risk = "re-signed tamper"
	if err := tampered.Sign(); err != nil {
		t.Fatal(err)
	}
	_, err = w.Plan(fixSchema7JSON(t, tampered))
	var fe *fault.Error
	if !errors.As(err, &fe) || fe.Code != "PROPOSAL_DRIFT" {
		t.Fatalf("re-signed tamper: %v", err)
	}

	drift := p
	drift.Workspace.BaseRevision = "00000000"
	if err := drift.Sign(); err != nil {
		t.Fatal(err)
	}
	_, err = w.Plan(fixSchema7JSON(t, drift))
	if !errors.As(err, &fe) || fe.Code != "PROPOSAL_DRIFT" {
		t.Fatalf("workspace drift: %v", err)
	}

	missing := map[string]any{"schemaVersion": 7, "proposal": p, "operations": []fix.Operation{}}
	_, err = w.Plan(editJSON(t, missing))
	if !errors.As(err, &fe) || fe.Code != "INVALID_OPERATIONS" {
		t.Fatalf("missing operations: %v", err)
	}
	extra := p.Derived.Operations[0]
	_, err = w.Plan(editJSON(t, map[string]any{"schemaVersion": 7, "proposal": p, "operations": []fix.Operation{extra, extra}}))
	if !errors.As(err, &fe) || fe.Code != "INVALID_OPERATIONS" {
		t.Fatalf("duplicate operations: %v", err)
	}
	_, err = w.Plan(fixSchema7JSON(t, all))
	if !errors.As(err, &fe) || fe.Code != "SELECTION_INVALID" {
		t.Fatalf("all selection: %v", err)
	}
}

// TestFixProposalSelectionLimits pins the CLI-side selection errors.
func TestFixProposalSelectionLimits(t *testing.T) {
	if _, err := fix.ParseSelection(""); err == nil {
		t.Fatal("empty selection accepted")
	}
	if _, err := fix.ParseSelection("sha256:a,sha256:a"); err == nil {
		t.Fatal("duplicate selection accepted")
	}
	sel, err := fix.ParseSelection("sha256:b,sha256:a")
	if err != nil || sel.Mode != fix.ModeExplicit || strings.Join(sel.RepairIDs, ",") != "sha256:a,sha256:b" {
		t.Fatalf("selection: %+v %v", sel, err)
	}
	w, _, _ := structureWorkspace(t, map[string]string{"EPUB/chapter1.xhtml": fixSource})
	defer w.Close()
	s, err := w.FixSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fix.Propose(s, fix.Selection{Mode: fix.ModeExplicit, RepairIDs: []string{"sha256:missing"}}); err == nil {
		t.Fatal("unknown repair id accepted")
	}
}
