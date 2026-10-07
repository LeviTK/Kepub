package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/fix"
	"github.com/LeviTK/Kepub/internal/validation"
	"github.com/LeviTK/Kepub/internal/workspace"
)

// RawDocument is a JSON document written verbatim to stdout by document
// commands. It is never wrapped in the human summary.
type RawDocument struct{ Bytes []byte }

// FixPropose derives a complete FixProposal v1 (or its schema 7 request) from
// the frozen accepted baseline. It is read-only: it never writes a candidate,
// accepted revision or history.
func FixPropose(dir, selectValue string, emitRequest bool, output string, machine bool) (any, error) {
	w, err := workspace.Open(dir)
	if err != nil {
		return nil, WorkspaceError(err)
	}
	defer w.Close()
	if output != "" {
		if _, err := w.OutputPath(output); err != nil {
			return nil, fault.New(2, "INVALID_OUTPUT", "%v", err)
		}
	}
	s, err := w.FixSnapshot()
	if err != nil {
		return nil, WorkspaceError(err)
	}
	sel := fix.Selection{Mode: fix.ModeAll, RepairIDs: []string{}}
	if selectValue != "" {
		parsed, err := fix.ParseSelection(selectValue)
		if err != nil {
			return nil, fault.New(2, "SELECTION_INVALID", "%v", err)
		}
		sel = parsed
	}
	p, err := fix.Propose(s, sel)
	if err != nil {
		if errors.Is(err, fix.ErrSelection) {
			return nil, fault.New(2, "SELECTION_INVALID", "%v", err)
		}
		return nil, editArgumentError("INVALID_OPERATIONS", err)
	}
	doc := any(p)
	kind := "proposal"
	if emitRequest {
		if err := fix.RequestAllowed(p); err != nil {
			return nil, fault.New(2, "SELECTION_INVALID", "%v", err)
		}
		if len(p.Derived.Operations) == 0 {
			return nil, fault.New(2, "INVALID_OPERATIONS", "proposal has no executable operations")
		}
		doc = map[string]any{"schemaVersion": 7, "proposal": p, "operations": p.Derived.Operations}
		kind = "request"
	}
	summary := map[string]any{
		"document": kind, "proposalId": p.ProposalID, "proposalSha256": p.ProposalSHA256,
		"repairs": len(p.Repairs), "operations": len(p.Derived.Operations), "limitations": len(p.Limitations),
	}
	if output != "" {
		b, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return nil, editArgumentError("INVALID_OPERATIONS", err)
		}
		if err := w.WriteDocument(append(b, '\n'), output); err != nil {
			return nil, WorkspaceError(err)
		}
		summary["output"] = output
		return summary, nil
	}
	if machine {
		return doc, nil
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, editArgumentError("INVALID_OPERATIONS", err)
	}
	return RawDocument{Bytes: append(b, '\n')}, nil
}

// FixDelta runs the pinned validation on both verified private snapshots and
// classifies the diagnostic difference. A normal compliance failure still
// produces the delta; checker or execution failures keep their fault.
func FixDelta(ctx context.Context, dir, before, afterRevision, afterTask, output string, o validation.Options, machine bool) (any, error) {
	w, err := workspace.Open(dir)
	if err != nil {
		return nil, WorkspaceError(err)
	}
	defer w.Close()
	if output != "" {
		if _, err := w.OutputPath(output); err != nil {
			return nil, fault.New(2, "INVALID_OUTPUT", "%v", err)
		}
	}
	beforeSnap, err := w.FixDeltaSnapshot(before, "")
	if err != nil {
		return nil, editArgumentError("INPUT_DRIFT", err)
	}
	afterSnap, err := w.FixDeltaSnapshot(afterRevision, afterTask)
	if err != nil {
		return nil, editArgumentError("INPUT_DRIFT", err)
	}
	beforeSide, beforeErr := buildDeltaSide(ctx, beforeSnap, o)
	afterSide, afterErr := buildDeltaSide(ctx, afterSnap, o)
	d := fix.Delta{
		DeltaVersion: fix.DeltaVersion,
		Before:       beforeSide,
		After:        afterSide,
		Entries:      fix.ClassifyDelta(beforeSide, afterSide),
		Limitations:  append(append([]fix.Limitation{}, nativeLimitations(beforeSide)...), nativeLimitations(afterSide)...),
	}
	doc := any(d)
	summary := map[string]any{"deltaVersion": fix.DeltaVersion, "entries": len(d.Entries), "limitations": len(d.Limitations)}
	if output != "" {
		b, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return nil, editArgumentError("INVALID_OPERATIONS", err)
		}
		if err := w.WriteDocument(append(b, '\n'), output); err != nil {
			return nil, WorkspaceError(err)
		}
		summary["output"] = output
		return summary, mergeDeltaErrors(beforeErr, afterErr)
	}
	if machine {
		return doc, mergeDeltaErrors(beforeErr, afterErr)
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, editArgumentError("INVALID_OPERATIONS", err)
	}
	return RawDocument{Bytes: append(b, '\n')}, mergeDeltaErrors(beforeErr, afterErr)
}

func mergeDeltaErrors(errs ...error) error {
	var out error
	for _, err := range errs {
		if err != nil {
			out = errors.Join(out, err)
		}
	}
	return out
}

// nativeLimitations returns the native coverage limitations of one side.
func nativeLimitations(s fix.DeltaSide) []fix.Limitation {
	for _, c := range s.Checks {
		if c.CheckerID != fix.Checker {
			continue
		}
		if cov, ok := c.Coverage.(fix.NativeCoverage); ok {
			return cov.Limitations
		}
	}
	return nil
}

// buildDeltaSide runs the pinned validation on one private snapshot and collects
// the native and upstream diagnostic facts separately.
func buildDeltaSide(ctx context.Context, ds workspace.DeltaSnapshot, o validation.Options) (fix.DeltaSide, error) {
	report, verr := validation.Validate(ctx, ds.Dir, o)
	native := fix.NativeDiagnostics(ds.Snapshot)
	coverage := fix.NativeCoverageOf(ds.Snapshot)
	nativeHash := fix.NativeReportHash(coverage, native)
	side := fix.DeltaSide{
		Snapshot: fix.DeltaRef{
			WorkspaceID:     ds.Snapshot.Workspace.WorkspaceID,
			Rootfile:        ds.Snapshot.Workspace.Rootfile,
			BaseRevision:    ds.Snapshot.Workspace.BaseRevision,
			InputTreeSHA256: ds.Snapshot.Workspace.InputTreeSHA256,
			RevisionID:      ds.RevisionID,
			TaskID:          ds.TaskID,
			ExecutionSHA256: ds.ExecutionSHA256,
			TreeSHA256:      ds.TreeSHA256,
		},
		Report:            report,
		ReportHash:        fix.ReportHash(report),
		NativeDiagnostics: native,
		Upstream:          upstreamDiagnostics(report),
	}
	side.Checks = deltaChecks(report, side.ReportHash, nativeHash, coverage, ds.TreeSHA256)
	return side, verr
}

// upstreamDiagnostics preserves the checker diagnostics exactly as reported.
func upstreamDiagnostics(report validation.Report) []fix.UpstreamDiagnostic {
	out := []fix.UpstreamDiagnostic{}
	for _, d := range report.Diagnostics {
		if d.Source != "epubcheck" {
			continue
		}
		code := d.UpstreamCode
		if code == "" {
			code = d.Code
		}
		line, column := 0, 0
		if d.Line != nil {
			line = *d.Line
		}
		if d.Column != nil {
			column = *d.Column
		}
		out = append(out, fix.UpstreamDiagnostic{Code: code, Severity: d.Severity, BookPath: d.BookPath, Line: line, Column: column, Message: d.Message})
	}
	return out
}

// deltaChecks maps the actual validation checks plus the native check identity.
func deltaChecks(report validation.Report, reportHash, nativeHash string, coverage fix.NativeCoverage, tree string) []fix.CheckMeta {
	out := []fix.CheckMeta{}
	for _, c := range report.Checks {
		out = append(out, fix.CheckMeta{
			CheckerID: c.ID, ToolVersion: c.Version, Ruleset: c.Rules,
			SpecBaseline: "unknown", Profile: "", Flags: "",
			InputHash: c.InputSHA256, ConfigHash: c.ConfigSHA256, ReportHash: reportHash,
			RunStatus: c.Status, Coverage: c.Coverage,
		})
	}
	out = append(out, fix.CheckMeta{
		CheckerID: fix.Checker, ToolVersion: fmt.Sprint(fix.CheckVersion), Ruleset: "kepub-fix-native-v1",
		SpecBaseline: fix.SpecVersion, Profile: "", Flags: "",
		InputHash: tree, ConfigHash: "", ReportHash: nativeHash,
		RunStatus: "completed", Coverage: coverage,
	})
	return out
}
