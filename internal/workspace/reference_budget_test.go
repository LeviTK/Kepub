package workspace

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/references"
	"github.com/LeviTK/Kepub/internal/validation"
)

func r4WorkspaceLimit(t *testing.T, err error) {
	t.Helper()
	var f *fault.Error
	if !errors.As(err, &f) || f.Code != "REFERENCE_LIMIT" || f.Exit != 1 {
		t.Fatalf("want REFERENCE_LIMIT/1, got %v", err)
	}
}

func TestR4DependencyPlansRequireCompleteGraph(t *testing.T) {
	for _, kind := range []string{"delete", "rename", "move"} {
		t.Run(kind, func(t *testing.T) {
			w, dir, original := structureWorkspace(t, nil)
			defer w.Close()
			a := structureBinding(t, w, "EPUB/chapter1.xhtml")
			b := structureBinding(t, w, "EPUB/chapter2.xhtml")
			ops := []Operation{a.elemDelete(a.locatorID(t, "unreferenced"))}
			schema := 4
			if kind == "rename" {
				ops = []Operation{a.attrSet(a.locatorID(t, "unreferenced"), "id", strPtr("unreferenced"), "newid")}
			} else if kind == "move" {
				ops = []Operation{a.elemMoveCross(b, a.locatorID(t, "unreferenced"), b.locatorID(t, "start2"), "after")}
				schema = 6
			}
			before := n1Tree(t, dir)
			source := readResource(t, original)
			limits := references.DefaultGraphLimits
			limits.Edges = 1 // OPF, nav and chapters share this one cap.
			w.graphLimits = &limits
			_, err := w.Plan(editJSON(t, Request{schema, ops}))
			r4WorkspaceLimit(t, err)
			if !reflect.DeepEqual(before, n1Tree(t, dir)) {
				t.Fatal("failed plan wrote workspace")
			}
			assertBytes(t, original, source)
			w.graphLimits = nil
			plan, err := w.Plan(editJSON(t, Request{schema, ops}))
			if err != nil {
				t.Fatal("legal positive control", err)
			}
			w.graphLimits = &limits
			before = n1Tree(t, dir)
			_, err = w.Apply(editJSON(t, plan))
			r4WorkspaceLimit(t, err)
			if !reflect.DeepEqual(before, n1Tree(t, dir)) {
				t.Fatal("Apply published before rechecking graph")
			}
			w.graphLimits = nil
			e := applyPlan(t, w, plan)
			for _, bp := range []string{"mimetype", "META-INF/container.xml", "EPUB/package.opf", "EPUB/chapter1.xhtml", "EPUB/chapter2.xhtml", "EPUB/nav.xhtml", "EPUB/style.css"} {
				want := readResource(t, filepath.Join(dir, revision, bp))
				if bp == "EPUB/chapter1.xhtml" {
					if kind == "rename" {
						want = bytes.Replace(want, []byte(`id="unreferenced"`), []byte(`id="newid"`), 1)
					} else {
						want = bytes.Replace(want, []byte(`<p id="unreferenced">Plain.</p>`), nil, 1)
					}
				}
				if kind == "move" && bp == "EPUB/chapter2.xhtml" {
					want = bytes.Replace(want, []byte(`<h1 id="start2">Two</h1>`), []byte(`<h1 id="start2">Two</h1><p id="unreferenced">Plain.</p>`), 1)
				}
				assertBytes(t, filepath.Join(dir, candidate, bp), want)
			}
			w.graphLimits = &limits
			_, err = w.TaskDiff(e.TaskID)
			r4WorkspaceLimit(t, err)
			w.graphLimits = nil
			if _, err := w.Reject(e.TaskID); err != nil {
				t.Fatal(err)
			}
			before = n1Tree(t, dir)
			w.graphLimits = &limits
			_, err = w.TaskStatus(e.TaskID)
			r4WorkspaceLimit(t, err)
			if !reflect.DeepEqual(before, n1Tree(t, dir)) {
				t.Fatal("history failure rewrote source")
			}
			w.graphLimits = nil
			if _, err := w.TaskStatus(e.TaskID); err != nil {
				t.Fatal("complete historical reproof", err)
			}
			assertBytes(t, original, source)
		})
	}
}

func TestR4InterruptedGraphBudgetRollsBack(t *testing.T) {
	w, dir, original := structureWorkspace(t, nil)
	defer w.Close()
	a := structureBinding(t, w, "EPUB/chapter1.xhtml")
	p := structurePlan(t, w, []Operation{a.elemDelete(a.locatorID(t, "unreferenced"))})
	start, outputs := prepareExecution(t, w, p)
	source := readResource(t, original)
	if err := w.writeCandidateFile("EPUB/chapter1.xhtml", outputs["EPUB/chapter1.xhtml"]); err != nil {
		t.Fatal(err)
	}
	limits := references.DefaultGraphLimits
	limits.Edges = 1
	w.graphLimits = &limits
	e, err := w.Execution()
	r4WorkspaceLimit(t, err)
	for _, bp := range []string{"mimetype", "META-INF/container.xml", "EPUB/package.opf", "EPUB/chapter1.xhtml", "EPUB/chapter2.xhtml", "EPUB/nav.xhtml", "EPUB/style.css"} {
		assertBytes(t, filepath.Join(dir, candidate, bp), readResource(t, filepath.Join(dir, revision, bp)))
	}
	if e.Status != "failed" || e.Checkpoint != start.Checkpoint || !e.Diff.Changed || e.ReviewRequired {
		t.Fatal("lost interrupted audit", e)
	}
	w.graphLimits = nil
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
	assertBytes(t, original, source)
}

func TestR4AcceptRequiresGraphBudgetAndRealChecker(t *testing.T) {
	w, dir, original := legalWorkspace(t, "3.0")
	defer w.Close()
	source := readResource(t, original)
	e := applyPlan(t, w, fieldPlan(t, w, "title", "title", "Title", "New title"))
	accepted := n1Tree(t, filepath.Join(dir, revision))
	candidateTree := n1Tree(t, filepath.Join(dir, candidate))
	limits := references.DefaultGraphLimits
	limits.Edges = 1
	d, err := w.Accept(context.Background(), e.TaskID, validation.Options{GraphLimits: &limits})
	r4WorkspaceLimit(t, err)
	if w.current != "initial" || d.Validation.Status == "pass" || d.Validation.Checks[2].Status != "blocked" || d.Validation.Checks[3].Status != "blocked" {
		t.Fatal("budget became accepted evidence", d)
	}
	if !reflect.DeepEqual(accepted, n1Tree(t, filepath.Join(dir, revision))) || !reflect.DeepEqual(candidateTree, n1Tree(t, filepath.Join(dir, candidate))) {
		t.Fatal("failed accept changed content")
	}
	if _, err := Open(dir); !errors.Is(err, ErrBusy) {
		t.Fatal("lost writer lock", err)
	}
	d, err = w.Accept(context.Background(), e.TaskID, validation.Options{})
	if err != nil || d.Status != "accepted" || d.Validation.Status != "pass" || d.Validation.Checks[3].Status != "passed" || d.Validation.Checks[3].ToolSHA256 != validation.ToolSHA256 {
		t.Fatal("real checker positive control", d, err)
	}
	assertBytes(t, original, source)
}
