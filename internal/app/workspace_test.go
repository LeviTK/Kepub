package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/testfixture"
	"github.com/LeviTK/Kepub/internal/workspace"
)

func TestPlanApplyFilesystemFailuresAreIOErrors(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission failure requires non-root user")
	}
	for _, action := range []string{"plan", "apply"} {
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
		{errors.New("invalid locator"), 2, "INVALID_OPERATIONS"},
		{fault.New(3, "UNSUPPORTED_INPUT", "unsupported"), 3, "UNSUPPORTED_INPUT"},
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
