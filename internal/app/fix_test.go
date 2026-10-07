package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/fix"
	"github.com/LeviTK/Kepub/internal/metadata"
	"github.com/LeviTK/Kepub/internal/validation"
	"github.com/LeviTK/Kepub/internal/workspace"
)

// fixDeltaChapter1 carries both native rule facts: an epub:type attribute on a
// metadata element (FR-1) and a relative URL query (FR-2).
const fixDeltaChapter1 = `<?xml version="1.0" encoding="utf-8"?>` + "\n" +
	`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head epub:type="secrecy"><title>One</title></head><body><p id="start">One.</p><a href="chapter2.xhtml?q=1#start2">Two</a></body></html>` + "\n"

const fixDeltaChapter2 = `<?xml version="1.0" encoding="utf-8"?>` + "\n" +
	`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Two</title></head><body><h1 id="start2">Two</h1><p>Beta &amp; two.</p></body></html>` + "\n"

const fixDeltaOPF = `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:uuid:12345678-1234-1234-1234-123456789012</dc:identifier><dc:title id="title">Title</dc:title><dc:creator id="creator">Writer</dc:creator><dc:language>en</dc:language><meta property="dcterms:modified">2026-10-04T00:00:00Z</meta></metadata><manifest><item id="c1" href="chapter1.xhtml" media-type="application/xhtml+xml"/><item id="c2" href="chapter2.xhtml" media-type="application/xhtml+xml"/><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>` + "\n"

const fixDeltaNav = `<?xml version="1.0" encoding="utf-8"?>` + "\n" +
	`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="chapter1.xhtml#start">One</a></li><li><a href="chapter2.xhtml#start2">Two</a></li></ol></nav></body></html>` + "\n"

// fixDeltaFixture builds a conformance-positive EPUB 3 whose first chapter
// carries the two native rule facts.
func fixDeltaFixture(t *testing.T) (*workspace.Workspace, string) {
	t.Helper()
	dir := t.TempDir()
	pub := filepath.Join(dir, "pub")
	files := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"EPUB/package.opf":       fixDeltaOPF,
		"EPUB/chapter1.xhtml":    fixDeltaChapter1,
		"EPUB/chapter2.xhtml":    fixDeltaChapter2,
		"EPUB/nav.xhtml":         fixDeltaNav,
	}
	for name, b := range files {
		abs := filepath.Join(pub, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(b), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a, tree, err := archive.SnapshotDirectory(pub, archive.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	source := filepath.Join(dir, "original.epub")
	f, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.WriteZIP(f, tree); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(dir, "workspace")
	w, err := workspace.Create(ws, source, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return w, ws
}

// fixDeltaPropose derives one proposal of the frozen fixture.
func fixDeltaPropose(t *testing.T, w *workspace.Workspace, mode string, ids ...string) fix.Proposal {
	t.Helper()
	s, err := w.FixSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	p, err := fix.Propose(s, fix.Selection{Mode: mode, RepairIDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// fixDeltaRequest renders the frozen schema 7 request wire.
func fixDeltaRequest(t *testing.T, p fix.Proposal, ops []fix.Operation) []byte {
	t.Helper()
	b, err := json.Marshal(map[string]any{"schemaVersion": 7, "proposal": p, "operations": ops})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func fixDeltaApply(t *testing.T, w *workspace.Workspace, p fix.Proposal) workspace.Execution {
	t.Helper()
	plan, err := w.Plan(fixDeltaRequest(t, p, p.Derived.Operations))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	e, err := w.Apply(b)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// fixDeltaWant is the independent byte expectation of the two literal edits.
func fixDeltaWant() []byte {
	want := strings.Replace(fixDeltaChapter1, ` epub:type="secrecy"`, "", 1)
	return []byte(strings.Replace(want, "?q=1#start2", "#start2", 1))
}

func fixDeltaCandidate(t *testing.T, w *workspace.Workspace) string {
	t.Helper()
	c, err := w.Candidate()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func fixDeltaFault(t *testing.T, err error, code string) {
	t.Helper()
	var fe *fault.Error
	if !errors.As(err, &fe) || fe.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}

// TestFixDeltaPinnedChecker runs the clean delta success path with the pinned
// EPUBCheck: the before side is a real accepted revision, the after side is a
// stable candidate task, and both sides ran the fixed checker.
func TestFixDeltaPinnedChecker(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("real pinned EPUBCheck required; set KEPUB_EPUBCHECK_JAR")
	}
	w, ws := fixDeltaFixture(t)
	all := fixDeltaPropose(t, w, fix.ModeAll)
	if len(all.Repairs) != 2 {
		t.Fatalf("repairs: %+v", all.Repairs)
	}
	p := fixDeltaPropose(t, w, fix.ModeExplicit, all.Repairs[0].RepairID, all.Repairs[1].RepairID)
	e := fixDeltaApply(t, w, p)
	first, err := w.Accept(context.Background(), e.TaskID, validation.Options{Timeout: 2 * time.Minute})
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	rename, err := json.Marshal(workspace.Request{SchemaVersion: 1, Operations: []workspace.Operation{{
		ID: "metadata.set", Version: 1,
		Params: metadata.Set{Namespace: metadata.DC, LocalName: "title", ID: "title", ExpectedOldValue: "Title", NewValue: "Renamed"},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := w.Plan(rename)
	if err != nil {
		t.Fatalf("clean plan: %v", err)
	}
	b, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	second, err := w.Apply(b)
	if err != nil {
		t.Fatalf("clean apply: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	doc, err := FixDelta(context.Background(), ws, first.RevisionID, "", second.TaskID, "", validation.Options{Timeout: 2 * time.Minute}, true)
	if err != nil {
		t.Fatalf("delta success path: %v", err)
	}
	d, ok := doc.(fix.Delta)
	if !ok {
		t.Fatalf("delta document: %T", doc)
	}
	if d.Before.Snapshot.RevisionID != first.RevisionID || d.Before.Snapshot.TaskID != "" {
		t.Fatalf("before identity: %+v", d.Before.Snapshot)
	}
	if d.After.Snapshot.TaskID != second.TaskID || d.After.Snapshot.ExecutionSHA256 == "" {
		t.Fatalf("after identity: %+v", d.After.Snapshot)
	}
	if d.Before.Snapshot.TreeSHA256 == "" || d.After.Snapshot.TreeSHA256 == d.Before.Snapshot.TreeSHA256 {
		t.Fatalf("tree identity: %+v %+v", d.Before.Snapshot, d.After.Snapshot)
	}
	if d.Before.ReportHash == "" || d.After.ReportHash == "" || d.Before.ReportHash == d.After.ReportHash {
		t.Fatalf("report hashes: %q %q", d.Before.ReportHash, d.After.ReportHash)
	}
	if d.Before.ReportHash != fix.ReportHash(d.Before.Report) {
		t.Fatal("before report hash does not bind the complete report")
	}
	if len(d.Entries) != 0 || len(d.Limitations) != 0 {
		t.Fatalf("clean delta entries: %+v limitations %+v", d.Entries, d.Limitations)
	}
	for _, side := range []fix.DeltaSide{d.Before, d.After} {
		if len(side.NativeDiagnostics) != 0 || len(side.Upstream) != 0 {
			t.Fatalf("clean side facts: %+v %+v", side.NativeDiagnostics, side.Upstream)
		}
		checker, native := false, false
		for _, c := range side.Checks {
			switch c.CheckerID {
			case "epubcheck":
				checker = true
				if c.RunStatus != "completed" || c.ToolVersion != validation.Version {
					t.Fatalf("checker ran: %+v", c)
				}
			case fix.Checker:
				native = true
				if cov, ok := c.Coverage.(fix.NativeCoverage); !ok || cov.Status != "complete" {
					t.Fatalf("native coverage: %+v", c.Coverage)
				}
			}
		}
		if !checker || !native {
			t.Fatalf("missing checks: %+v", side.Checks)
		}
	}
}

// TestFixDeltaComplianceFailure keeps a normal compliance FAIL distinct from an
// execution fault: the complete report still produces the classified delta with
// exit 0, and the failed side keeps the completed run status.
func TestFixDeltaComplianceFailure(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("real pinned EPUBCheck required; set KEPUB_EPUBCHECK_JAR")
	}
	w, ws := fixDeltaFixture(t)
	all := fixDeltaPropose(t, w, fix.ModeAll)
	p := fixDeltaPropose(t, w, fix.ModeExplicit, all.Repairs[0].RepairID, all.Repairs[1].RepairID)
	e := fixDeltaApply(t, w, p)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	doc, err := FixDelta(context.Background(), ws, "initial", "", e.TaskID, "", validation.Options{Timeout: 2 * time.Minute}, true)
	if err != nil {
		t.Fatalf("complete compliance FAIL must still explain the delta: %v", err)
	}
	d, ok := doc.(fix.Delta)
	if !ok {
		t.Fatalf("delta document: %T", doc)
	}
	before, ok := d.Before.Report.(validation.Report)
	if !ok || before.Status != "fail" {
		t.Fatalf("before report: %+v", d.Before.Report)
	}
	for _, side := range []fix.DeltaSide{d.Before, d.After} {
		for _, c := range side.Checks {
			if c.CheckerID == "epubcheck" && c.RunStatus != "completed" {
				t.Fatalf("compliance FAIL run status: %+v", c)
			}
		}
	}
	if len(d.Before.NativeDiagnostics) != 2 || len(d.After.NativeDiagnostics) != 0 {
		t.Fatalf("native facts: %d / %d", len(d.Before.NativeDiagnostics), len(d.After.NativeDiagnostics))
	}
	if len(d.Before.Upstream) < 2 || len(d.After.Upstream) != 0 {
		t.Fatalf("upstream facts: %d / %d", len(d.Before.Upstream), len(d.After.Upstream))
	}
	resolved := map[string]bool{}
	queryResolved := false
	for _, en := range d.Entries {
		if en.Classification == "" || en.Source == "" {
			t.Fatalf("unclassified entry: %+v", en)
		}
		for _, v := range append(append([]any{}, en.Before...), en.After...) {
			switch inst := v.(type) {
			case fix.UpstreamDiagnostic:
				if en.Classification == "resolved" && (inst.Code == "RSC-005" || inst.Code == "RSC-016" || inst.Code == "RSC-017") {
					t.Fatalf("generic schema code claimed resolved: %+v", en)
				}
				if strings.Contains(inst.Message, "query component") {
					if en.Classification != "resolved" {
						t.Fatalf("specific query diagnostic not resolved: %+v", en)
					}
					queryResolved = true
				}
			case fix.NativeDiagnostic:
				if en.Classification == "resolved" {
					resolved[inst.Rule.ID] = true
				}
			}
		}
	}
	if !resolved[fix.RuleEpubTypeProhibited] || !resolved[fix.RuleRelativeURLQuery] {
		t.Fatalf("native repairs not resolved: %+v", resolved)
	}
	if !queryResolved {
		t.Fatal("missing upstream query diagnostic entry")
	}
}

// TestFixDeltaMissingChecker keeps the dependency fault and still returns the
// delta data, with the checker status unavailable and no resolved claim.
func TestFixDeltaMissingChecker(t *testing.T) {
	w, ws := fixDeltaFixture(t)
	all := fixDeltaPropose(t, w, fix.ModeAll)
	p := fixDeltaPropose(t, w, fix.ModeExplicit, all.Repairs[0].RepairID)
	e := fixDeltaApply(t, w, p)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	doc, err := FixDelta(context.Background(), ws, "initial", "", e.TaskID, "",
		validation.Options{JAR: filepath.Join(t.TempDir(), "absent.jar"), Timeout: time.Minute}, true)
	fixDeltaFault(t, err, "DEPENDENCY_UNAVAILABLE")
	d, ok := doc.(fix.Delta)
	if !ok {
		t.Fatalf("delta document: %T", doc)
	}
	for _, side := range []fix.DeltaSide{d.Before, d.After} {
		for _, c := range side.Checks {
			if c.CheckerID != "epubcheck" {
				continue
			}
			if c.RunStatus != "unavailable" {
				t.Fatalf("missing checker status: %+v", c)
			}
		}
	}
	for _, en := range d.Entries {
		if en.Source == "epubcheck" && en.Classification == "resolved" {
			t.Fatalf("missing checker claimed resolved: %+v", en)
		}
	}
}

// TestFixDeltaExecutionFailure keeps the execution fault when the checker
// process fails, with the failed status on both sides.
func TestFixDeltaExecutionFailure(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("real pinned EPUBCheck required; set KEPUB_EPUBCHECK_JAR")
	}
	w, ws := fixDeltaFixture(t)
	all := fixDeltaPropose(t, w, fix.ModeAll)
	p := fixDeltaPropose(t, w, fix.ModeExplicit, all.Repairs[0].RepairID)
	e := fixDeltaApply(t, w, p)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(t.TempDir(), "java")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	doc, err := FixDelta(context.Background(), ws, "initial", "", e.TaskID, "",
		validation.Options{Java: fake, JAR: os.Getenv("KEPUB_EPUBCHECK_JAR"), Timeout: time.Minute}, true)
	fixDeltaFault(t, err, "CHECKER_EXECUTION_FAILED")
	d, ok := doc.(fix.Delta)
	if !ok {
		t.Fatalf("delta document: %T", doc)
	}
	for _, side := range []fix.DeltaSide{d.Before, d.After} {
		for _, c := range side.Checks {
			if c.CheckerID == "epubcheck" && c.RunStatus != "failed" {
				t.Fatalf("execution failure status: %+v", c)
			}
		}
	}
	for _, en := range d.Entries {
		if en.Source == "epubcheck" && en.Classification == "resolved" {
			t.Fatalf("failed checker claimed resolved: %+v", en)
		}
	}
}

// TestFixProposalTamperMatrix re-signs each mutated proposal source field and
// requires the whole-content re-derivation to refuse it.
func TestFixProposalTamperMatrix(t *testing.T) {
	w, _ := fixDeltaFixture(t)
	defer w.Close()
	all := fixDeltaPropose(t, w, fix.ModeAll)
	base := fixDeltaPropose(t, w, fix.ModeExplicit, all.Repairs[0].RepairID, all.Repairs[1].RepairID)
	mutations := []struct {
		name   string
		mutate func(p *fix.Proposal)
	}{
		{"basis-fact", func(p *fix.Proposal) { p.Repairs[0].Basis.Fact = "tampered fact" }},
		{"risk", func(p *fix.Proposal) { p.Repairs[0].Risk = "re-signed tamper" }},
		{"old-value", func(p *fix.Proposal) { p.Repairs[0].Target.ExpectedOldValue = "other" }},
		{"read-set", func(p *fix.Proposal) { p.Derived.ReadSet = append(p.Derived.ReadSet, "EPUB/other.xhtml") }},
		{"write-set", func(p *fix.Proposal) { p.Derived.WriteSet = []string{"EPUB/other.xhtml"} }},
		{"selection-subset", func(p *fix.Proposal) { p.Selection.RepairIDs = p.Selection.RepairIDs[:1] }},
		{"selection-mode", func(p *fix.Proposal) { p.Selection.Mode = fix.ModeAll }},
		{"workspace-id", func(p *fix.Proposal) { p.Workspace.WorkspaceID = "other" }},
		{"workspace-revision", func(p *fix.Proposal) { p.Workspace.BaseRevision = "00000000" }},
		{"rules", func(p *fix.Proposal) { p.Rules[0].Version++ }},
		{"limitations", func(p *fix.Proposal) { p.Limitations = append(p.Limitations, fix.Limitation{}) }},
	}
	for _, m := range mutations {
		p := base
		p.Repairs = append([]fix.Repair{}, base.Repairs...)
		p.Derived.ReadSet = append([]string{}, base.Derived.ReadSet...)
		p.Derived.WriteSet = append([]string{}, base.Derived.WriteSet...)
		p.Selection.RepairIDs = append([]string{}, base.Selection.RepairIDs...)
		p.Rules = append([]fix.RuleRef{}, base.Rules...)
		m.mutate(&p)
		if err := p.Sign(); err != nil {
			t.Fatal(err)
		}
		if _, err := w.Plan(fixDeltaRequest(t, p, p.Derived.Operations)); err == nil {
			t.Fatalf("%s: re-signed tamper accepted", m.name)
		} else {
			var fe *fault.Error
			if !errors.As(err, &fe) || fe.Code != "PROPOSAL_DRIFT" {
				t.Fatalf("%s: %v", m.name, err)
			}
		}
	}
	hashTamper := base
	hashTamper.ProposalSHA256 = strings.Repeat("0", 64)
	if _, err := w.Plan(fixDeltaRequest(t, hashTamper, hashTamper.Derived.Operations)); err == nil {
		t.Fatal("stale proposal hash accepted")
	} else {
		var fe *fault.Error
		if !errors.As(err, &fe) || fe.Code != "PROPOSAL_DRIFT" {
			t.Fatalf("stale hash: %v", err)
		}
	}
}

// TestFixProposalOperationPermutationAndBytes permits a legal operation
// permutation and compares both orders against the independent byte oracle.
func TestFixProposalOperationPermutationAndBytes(t *testing.T) {
	w, _ := fixDeltaFixture(t)
	defer w.Close()
	all := fixDeltaPropose(t, w, fix.ModeAll)
	p := fixDeltaPropose(t, w, fix.ModeExplicit, all.Repairs[0].RepairID, all.Repairs[1].RepairID)
	if len(p.Derived.Operations) != 2 {
		t.Fatalf("operations: %+v", p.Derived.Operations)
	}
	want := fixDeltaWant()
	var first []byte
	for i, ops := range [][]fix.Operation{p.Derived.Operations, {p.Derived.Operations[1], p.Derived.Operations[0]}} {
		plan, err := w.Plan(fixDeltaRequest(t, p, ops))
		if err != nil {
			t.Fatalf("order %d refused: %v", i, err)
		}
		b, err := json.Marshal(plan)
		if err != nil {
			t.Fatal(err)
		}
		e, err := w.Apply(b)
		if err != nil {
			t.Fatalf("order %d apply: %v", i, err)
		}
		got, err := os.ReadFile(filepath.Join(fixDeltaCandidate(t, w), "EPUB/chapter1.xhtml"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("order %d candidate bytes:\n got %q\nwant %q", i, got, want)
		}
		if i == 0 {
			first = append([]byte{}, got...)
		} else if !bytes.Equal(first, got) {
			t.Fatal("both orders produced different bytes")
		}
		if _, err := w.Reject(e.TaskID); err != nil {
			t.Fatal(err)
		}
	}
}

// TestFixExternalFileLifecycle plans from an external request file, deletes it,
// and requires diff, accept and a reopened workspace to recompute the same
// source and accepted tree.
func TestFixExternalFileLifecycle(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("real pinned EPUBCheck required; set KEPUB_EPUBCHECK_JAR")
	}
	w, ws := fixDeltaFixture(t)
	all := fixDeltaPropose(t, w, fix.ModeAll)
	p := fixDeltaPropose(t, w, fix.ModeExplicit, all.Repairs[0].RepairID, all.Repairs[1].RepairID)
	external := filepath.Join(t.TempDir(), "request.json")
	if err := os.WriteFile(external, fixDeltaRequest(t, p, p.Derived.Operations), 0o600); err != nil {
		t.Fatal(err)
	}
	req, err := os.ReadFile(external)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := w.Plan(req)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	e, err := w.Apply(b)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(external); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(external); !os.IsNotExist(err) {
		t.Fatal("external request file still exists")
	}
	d, err := w.Diff()
	if err != nil {
		t.Fatal(err)
	}
	if !d.Changed || len(d.Changes) == 0 {
		t.Fatalf("diff after deletion: %+v", d)
	}
	decision, err := w.Accept(context.Background(), e.TaskID, validation.Options{Timeout: 2 * time.Minute})
	if err != nil {
		t.Fatalf("accept after deletion: %v", err)
	}
	if decision.RevisionID == "" || decision.TreeSHA256 == "" {
		t.Fatalf("decision: %+v", decision)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := workspace.Open(ws)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	a, tree, rev, err := reopened.AcceptedSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if rev.ID != decision.RevisionID || tree.SHA256 != decision.TreeSHA256 {
		t.Fatalf("reopened accepted revision: %+v %s vs %+v", rev, tree.SHA256, decision)
	}
	got, err := a.Read("EPUB/chapter1.xhtml", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, fixDeltaWant()) {
		t.Fatalf("accepted bytes:\n got %q\nwant %q", got, fixDeltaWant())
	}
	if _, err := reopened.FixDeltaSnapshot("initial", ""); err != nil {
		t.Fatalf("reopened before snapshot: %v", err)
	}
	if _, err := reopened.Diff(); err == nil {
		t.Fatal("no active candidate after accept")
	}
}
