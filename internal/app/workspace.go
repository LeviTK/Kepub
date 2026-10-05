package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/validation"
	"github.com/LeviTK/Kepub/internal/workspace"
)

type WorkspaceResult struct {
	Directory   string          `json:"directory"`
	ID          string          `json:"workspaceId"`
	State       workspace.State `json:"state"`
	Conformance string          `json:"conformance"`
}

func OpenWorkspace(book, dir, rootfile string) (WorkspaceResult, error) {
	w, err := workspace.Create(dir, book, workspace.Options{Rootfile: rootfile})
	if err != nil {
		return WorkspaceResult{}, WorkspaceError(err)
	}
	defer w.Close()
	id, err := w.ID()
	if err != nil {
		return WorkspaceResult{}, WorkspaceError(err)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return WorkspaceResult{}, err
	}
	return WorkspaceResult{abs, id, w.State(), "not_run"}, nil
}

func readEditFile(file string) ([]byte, error) {
	b, err := workspace.ReadEditFile(file)
	var pathError *os.PathError
	if err != nil && !errors.As(err, &pathError) {
		return nil, fault.New(2, "INVALID_ARGUMENT", "%v", err)
	}
	return b, err
}
func PlanWorkspace(dir, operations, output string) (workspace.Plan, error) {
	w, err := workspace.Open(dir)
	if err != nil {
		return workspace.Plan{}, WorkspaceError(err)
	}
	defer w.Close()
	if _, err := w.OutputPath(output); err != nil {
		return workspace.Plan{}, fault.New(2, "INVALID_OUTPUT", "%v", err)
	}
	b, err := readEditFile(operations)
	if err != nil {
		return workspace.Plan{}, err
	}
	p, err := w.Plan(b)
	if err != nil {
		return p, editArgumentError("INVALID_OPERATIONS", err)
	}
	return p, WorkspaceError(w.WritePlanReport(p, output))
}
func ApplyWorkspace(dir, plan string) (workspace.Execution, error) {
	w, err := workspace.Open(dir)
	if err != nil {
		return workspace.Execution{}, WorkspaceError(err)
	}
	defer w.Close()
	b, err := readEditFile(plan)
	if err != nil {
		return workspace.Execution{}, err
	}
	e, err := w.Apply(b)
	if err != nil && e.Version == 0 {
		return e, editArgumentError("INVALID_PLAN", err)
	}
	return e, WorkspaceError(err)
}

func WorkspaceTask(ctx context.Context, dir, id, action string, o validation.Options) (any, error) {
	w, err := workspace.Open(dir)
	if err != nil {
		return nil, WorkspaceError(err)
	}
	defer w.Close()
	var result any
	switch action {
	case "diff":
		result, err = w.TaskDiff(id)
	case "accept":
		result, err = w.Accept(ctx, id, o)
	case "reject":
		result, err = w.Reject(id)
	default:
		return nil, fault.New(2, "INVALID_ARGUMENT", "unsupported task action")
	}
	return result, WorkspaceError(err)
}

type WorkspaceExportResult struct {
	RevisionID string `json:"revisionId"`
	PackResult
}

func ExportWorkspace(ctx context.Context, dir, output string, o validation.Options) (WorkspaceExportResult, error) {
	w, err := workspace.Open(dir)
	if err != nil {
		return WorkspaceExportResult{}, WorkspaceError(err)
	}
	defer w.Close()
	out, err := w.OutputPath(output)
	if err != nil {
		return WorkspaceExportResult{}, fault.New(2, "INVALID_OUTPUT", "%v", err)
	}
	a, t, r, err := w.AcceptedSnapshot()
	if err != nil {
		return WorkspaceExportResult{}, WorkspaceError(err)
	}
	defer a.Close()
	if o.Rootfile != "" && o.Rootfile != r.Rootfile {
		return WorkspaceExportResult{}, fault.New(2, "INVALID_ARGUMENT", "workspace export uses the persisted selected rootfile")
	}
	o.Rootfile = r.Rootfile
	packed, err := PackSnapshot(ctx, a, t, out, o)
	return WorkspaceExportResult{r.ID, packed}, WorkspaceError(err)
}

func editArgumentError(code string, err error) error {
	mapped := WorkspaceError(err)
	var f *fault.Error
	if errors.As(mapped, &f) {
		return mapped
	}
	return fault.New(2, code, "%v", err)
}
func WorkspaceError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, workspace.ErrBusy):
		return fault.New(4, "WORKSPACE_BUSY", "%v", err)
	case errors.Is(err, workspace.ErrStalePlan), errors.Is(err, workspace.ErrCandidateDrift):
		return fault.New(4, "INPUT_DRIFT", "%v", err)
	case errors.Is(err, workspace.ErrTaskConflict), errors.Is(err, workspace.ErrCandidateConflict):
		return fault.New(4, "TASK_CONFLICT", "%v", err)
	case errors.Is(err, workspace.ErrReadOnly):
		return fault.New(3, "UNSUPPORTED_INPUT", "%v", err)
	case errors.Is(err, workspace.ErrRecovery):
		return fault.New(4, "RECOVERY_REQUIRED", "%v", err)
	case errors.Is(err, context.DeadlineExceeded):
		return fault.New(5, "CHECKER_TIMEOUT", "request timed out before acceptance intent")
	case errors.Is(err, context.Canceled):
		return fault.New(130, "CANCELLED", "request cancelled before acceptance intent")
	default:
		return err
	}
}
