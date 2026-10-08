package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/testfixture"
	"github.com/LeviTK/Kepub/internal/validation"
	"github.com/LeviTK/Kepub/internal/workspace"
)

func TestPlanAndAcceptRefusalsAreNotIOErrors(t *testing.T) {
	for _, scenario := range []string{"unknown-plan", "unreadable-plan", "failed", "legacy", "draft", "rootfile"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			book, ws := filepath.Join(dir, "book.epub"), filepath.Join(dir, "ws")
			testfixture.ZIP(t, book, testfixture.EPUB("3.0", false))
			if _, err := OpenWorkspace(book, ws, ""); err != nil {
				t.Fatal(err)
			}
			req, plan := filepath.Join(dir, "ops.json"), filepath.Join(dir, "plan.json")
			if err := os.WriteFile(req, []byte(`{"schemaVersion":1,"operations":[{"operationId":"metadata.set","operationVersion":1,"params":{"namespace":"http://purl.org/dc/elements/1.1/","localName":"title","id":"t","expectedOldValue":"测试 & Space","newValue":"New"}}]}`), 0600); err != nil {
				t.Fatal(err)
			}
			p, err := PlanWorkspace(ws, req, plan)
			if err != nil {
				t.Fatal(err)
			}
			wantExit, wantCode := 4, "TASK_CONFLICT"
			if scenario == "unknown-plan" || scenario == "unreadable-plan" {
				if scenario == "unknown-plan" {
					p.ID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					b, err := json.Marshal(p)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(plan, b, 0600); err != nil {
						t.Fatal(err)
					}
					wantCode = "INPUT_DRIFT"
				} else {
					if os.Geteuid() == 0 {
						t.Skip("requires non-root permissions")
					}
					stored := filepath.Join(ws, "plans", p.ID+".json")
					if err := os.Chmod(stored, 0000); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { os.Chmod(stored, 0600) })
					wantExit, wantCode = 6, "IO_ERROR"
				}
				_, err = ApplyWorkspace(ws, plan)
			} else if scenario == "legacy" {
				w, openErr := workspace.Open(ws)
				if openErr != nil {
					t.Fatal(openErr)
				}
				_, createErr := w.NewCandidate()
				w.Close()
				if createErr != nil {
					t.Fatal(createErr)
				}
				if err := os.WriteFile(filepath.Join(ws, "tasks/active/task.json"), []byte(`{"version":1,"baseRevision":"initial"}`), 0600); err != nil {
					t.Fatal(err)
				}
				_, err = WorkspaceTask(context.Background(), ws, "active", "accept", validation.Options{}, false)
			} else {
				var e workspace.Execution
				e, err = ApplyWorkspace(ws, plan)
				if err != nil {
					t.Fatal(err)
				}
				o := validation.Options{}
				switch scenario {
				case "failed":
					if err := os.Remove(filepath.Join(ws, "tasks/active/edit-result.json")); err != nil {
						t.Fatal(err)
					}
				case "draft":
					o.Draft = true
				case "rootfile":
					o.Rootfile = "EPUB/other.opf"
				}
				if scenario == "draft" || scenario == "rootfile" {
					wantExit, wantCode = 2, "INVALID_ARGUMENT"
				}
				_, err = WorkspaceTask(context.Background(), ws, e.TaskID, "accept", o, false)
			}
			var f *fault.Error
			if !errors.As(err, &f) || f.Exit != wantExit || f.Code != wantCode {
				t.Fatalf("%s: got %v; want %d %s", scenario, err, wantExit, wantCode)
			}
		})
	}
}

func TestPlanApplyFilesystemFailuresAreIOErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission failure requires non-root user")
	}
	for _, action := range []string{"plan", "apply", "publish"} {
		t.Run(action, func(t *testing.T) {
			dir := t.TempDir()
			book, ws := filepath.Join(dir, "book.epub"), filepath.Join(dir, "workspace")
			testfixture.ZIP(t, book, testfixture.EPUB("3.0", false))
			if _, err := OpenWorkspace(book, ws, ""); err != nil {
				t.Fatal(err)
			}
			req := filepath.Join(dir, "operations.json")
			if err := os.WriteFile(req, []byte(`{"schemaVersion":1,"operations":[{"operationId":"metadata.set","operationVersion":1,"params":{"namespace":"http://purl.org/dc/elements/1.1/","localName":"title","id":"t","expectedOldValue":"测试 & Space","newValue":"New"}}]}`), 0600); err != nil {
				t.Fatal(err)
			}
			plan := filepath.Join(dir, "plan.json")
			if _, err := PlanWorkspace(ws, req, plan); err != nil {
				t.Fatal(err)
			}
			blocked := filepath.Join(ws, "plans")
			if action == "apply" {
				blocked = filepath.Join(ws, "staging")
			} else if action == "publish" {
				blocked = filepath.Join(ws, "tasks")
			}
			if err := os.Chmod(blocked, 0500); err != nil {
				t.Fatal(err)
			}
			defer os.Chmod(blocked, 0700)
			var err error
			if action == "plan" {
				_, err = PlanWorkspace(ws, req, filepath.Join(dir, "other.json"))
			} else {
				_, err = ApplyWorkspace(ws, plan)
			}
			var f *fault.Error
			if !errors.As(err, &f) || f.Exit != 6 || f.Code != "IO_ERROR" {
				t.Fatal("filesystem error misreported as bad input", err)
			}
		})
	}
}

func TestEditArgumentErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		err  error
		exit int
		code string
	}{
		{fmt.Errorf("write: %w", &os.PathError{Op: "open", Path: "plans/x", Err: os.ErrPermission}), 6, "IO_ERROR"},
		{fmt.Errorf("publish: %w", &os.LinkError{Op: "rename", Old: "a", New: "b", Err: os.ErrPermission}), 6, "IO_ERROR"},
		{fmt.Errorf("sync: %w", os.NewSyscallError("fsync", os.ErrPermission)), 6, "IO_ERROR"},
		{fmt.Errorf("publish: %w", syscall.EACCES), 6, "IO_ERROR"},
		{errors.New("invalid locator"), 2, "INVALID_OPERATIONS"},
		{fault.New(3, "UNSUPPORTED_INPUT", "unsupported"), 3, "UNSUPPORTED_INPUT"},
		{fmt.Errorf("metadata: %w", fault.New(1, "WORKSPACE_JSON_LIMIT", "too large")), 1, "WORKSPACE_JSON_LIMIT"},
		{errors.Join(workspace.ErrStalePlan, &os.PathError{Op: "open", Err: os.ErrNotExist}), 4, "INPUT_DRIFT"},
		{workspace.ErrCandidateConflict, 4, "TASK_CONFLICT"},
	} {
		var f *fault.Error
		err := editArgumentError("INVALID_OPERATIONS", tc.err)
		if !errors.As(err, &f) || f.Exit != tc.exit || f.Code != tc.code {
			t.Fatal("wrong error classification", tc, err)
		}
	}
}
