package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
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

// Observe the real builder without adding a production injection API. The
// cancellation/deadline is real, but its timing is an owner-window injection.
type r4GraphContext struct {
	context.Context
	interrupt func() context.Context
	reached   bool
}

func (c *r4GraphContext) Err() error {
	if !c.reached {
		pcs := make([]uintptr, 32)
		n := runtime.Callers(2, pcs)
		frames := runtime.CallersFrames(pcs[:n])
		for {
			frame, more := frames.Next()
			if strings.Contains(frame.Function, "/internal/references.") {
				c.reached = true
				c.Context = c.interrupt()
				break
			}
			if !more {
				break
			}
		}
	}
	return c.Context.Err()
}

func TestR4RunningGraphCancellationRecovery(t *testing.T) {
	for _, mode := range []string{"control", "cancel", "deadline"} {
		for _, state := range []string{"running", "no-start", "unstarted", "completed", "bad-used", "bad-start", "bad-checkpoint"} {
			t.Run(mode+"/"+state, func(t *testing.T) {
				w, dir, original := structureWorkspace(t, nil)
				a := structureBinding(t, w, "EPUB/chapter1.xhtml")
				p := structurePlan(t, w, []Operation{a.elemDelete(a.locatorID(t, "unreferenced"))})
				start, _ := prepareExecution(t, w, p)
				frozen := n1Tree(t, filepath.Join(dir, revision))
				frozenBytes := map[string][]byte{}
				for _, bp := range []string{"mimetype", "META-INF/container.xml", "EPUB/package.opf", "EPUB/chapter1.xhtml", "EPUB/chapter2.xhtml", "EPUB/nav.xhtml", "EPUB/style.css"} {
					frozenBytes[bp] = readResource(t, filepath.Join(dir, revision, bp))
				}
				source := readResource(t, original)
				stateBytes := readResource(t, filepath.Join(dir, "state.json"))
				chapter := filepath.Join(dir, candidate, "EPUB/chapter1.xhtml")
				// Independent partial-write oracle, not the product's derived output.
				put(t, chapter, bytes.Replace(readResource(t, chapter), []byte(`<p id="unreferenced">Plain.</p>`), nil, 1))
				switch state {
				case "no-start":
					if err := w.root.Remove("tasks/active/edit-start.json"); err != nil {
						t.Fatal(err)
					}
				case "unstarted":
					start.Status = "unstarted"
					put(t, filepath.Join(dir, "tasks/active/edit-start.json"), editJSON(t, start))
				case "completed":
					if _, err := w.Execution(); err != nil {
						t.Fatal(err)
					}
					put(t, chapter, []byte("safe candidate drift after failed execution"))
				case "bad-used":
					put(t, filepath.Join(dir, "plans/"+p.ID+".used.json"), editJSON(t, planUse{Version: 1, TaskID: "wrong", PlanSHA256: digest(p)}))
				case "bad-start":
					start.TaskID = "wrong"
					put(t, filepath.Join(dir, "tasks/active/edit-start.json"), editJSON(t, start))
				case "bad-checkpoint":
					start.Checkpoint = "missing"
					put(t, filepath.Join(dir, "tasks/active/edit-start.json"), editJSON(t, start))
				}
				w.Close()
				before := n1Tree(t, dir)
				ctx := &r4GraphContext{Context: context.Background(), interrupt: func() context.Context {
					if mode == "cancel" {
						c, cancel := context.WithCancel(context.Background())
						cancel()
						return c
					}
					if mode == "deadline" {
						c, cancel := context.WithTimeout(context.Background(), time.Millisecond)
						defer cancel()
						<-c.Done()
						return c
					}
					return context.Background()
				}}
				opened, err := OpenContext(ctx, dir)
				if opened != nil {
					opened.Close()
				}
				if !ctx.reached {
					t.Fatal("references builder observation not reached")
				}
				if mode != "control" && state != "bad-used" && state != "bad-start" && state != "bad-checkpoint" {
					want := context.Canceled
					if mode == "deadline" {
						want = context.DeadlineExceeded
					}
					if !errors.Is(err, want) {
						t.Fatalf("want %v, got %v", want, err)
					}
				}
				if state == "running" {
					if mode == "control" && err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(frozen, n1Tree(t, filepath.Join(dir, candidate))) {
						t.Error("this Open did not restore the complete candidate")
					}
					for bp, want := range frozenBytes {
						if got := readResource(t, filepath.Join(dir, candidate, bp)); !bytes.Equal(got, want) {
							t.Errorf("this Open left different candidate bytes: %s", bp)
						}
					}
					var result Execution
					data, readErr := os.ReadFile(filepath.Join(dir, "tasks/active/edit-result.json"))
					if readErr != nil {
						t.Errorf("this Open did not persist failed result: %v", readErr)
					} else {
						if err := json.Unmarshal(data, &result); err != nil {
							t.Fatal(err)
						}
						if result.Status != "failed" || result.ReviewRequired || !result.Diff.Changed || result.Checkpoint != start.Checkpoint {
							t.Errorf("lost failed provenance: %+v", result)
						}
					}
				} else if mode != "control" || strings.HasPrefix(state, "bad-") {
					if err == nil {
						t.Error("invalid or canceled recovery succeeded")
					}
					if !reflect.DeepEqual(before, n1Tree(t, dir)) {
						t.Error("non-running or invalid provenance was rewritten")
					}
				}
				if !reflect.DeepEqual(frozen, n1Tree(t, filepath.Join(dir, revision))) {
					t.Error("accepted changed")
				}
				for bp, want := range frozenBytes {
					assertBytes(t, filepath.Join(dir, revision, bp), want)
				}
				assertBytes(t, original, source)
				assertBytes(t, filepath.Join(dir, "state.json"), stateBytes)
				if !strings.HasPrefix(state, "bad-") {
					reopened, err := Open(dir)
					if err != nil {
						t.Fatal("lock release and reopen", err)
					}
					defer reopened.Close()
					if _, err := Open(dir); !errors.Is(err, ErrBusy) {
						t.Fatal("reopen lost lock", err)
					}
				}
			})
		}
	}
}

func TestR4WorkspaceGraphReadGrowth(t *testing.T) {
	for _, tc := range []struct {
		growth, allowance int
		second            bool
	}{
		{0, 0, false}, // exact-budget control
		{1, 0, false}, // bounded reader overflows
		{32, 0, false},
		{1, 0, true}, // growth fits this read; extra charging blocks the next resource
		{1, 1, true}, // actual growth at the cumulative cap succeeds
	} {
		t.Run(fmt.Sprintf("growth=%d/allowance=%d/second=%t", tc.growth, tc.allowance, tc.second), func(t *testing.T) {
			dir := t.TempDir()
			const bp = bookpath.BookPath("chapter.xhtml")
			data := []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="target">Text</p></body></html>`)
			put(t, filepath.Join(dir, string(bp)), data)
			if tc.second {
				put(t, filepath.Join(dir, "second.xhtml"), data)
			}
			r, err := openDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			inv, err := inventory(r)
			if err != nil {
				t.Fatal(err)
			}
			p := &publication.Publication{Manifest: []publication.Item{{Path: bp, MediaType: "application/xhtml+xml", Exists: true}}}
			if tc.second {
				p.Manifest = append(p.Manifest, publication.Item{Path: "second.xhtml", MediaType: "application/xhtml+xml", Exists: true})
			}
			limits := references.DefaultGraphLimits
			limits.ScanBytes = int64(len(data) + tc.allowance)
			if tc.second {
				limits.ScanBytes += int64(len(data))
			}
			put(t, filepath.Join(dir, string(bp)), append(bytes.Clone(data), bytes.Repeat([]byte(" "), tc.growth)...))
			g, err := references.BuildSource(t.Context(), publicationRoot{r}, inv, p, limits)
			if tc.growth <= tc.allowance {
				if err != nil || g.Status != "complete" {
					t.Fatalf("exact-budget control: %+v %v", g, err)
				}
			} else {
				r4WorkspaceLimit(t, err)
				if !reflect.DeepEqual(g, references.Graph{}) {
					t.Fatalf("limit returned usable graph: %+v", g)
				}
			}
		})
	}
}

func TestR4WorkspaceGraphCumulativeOverflow(t *testing.T) {
	for _, tc := range []struct {
		name               string
		grown              int
		delta              int64
		missing, cancelled bool
	}{
		{"normal", 0, -1, false, false},
		{"one-resource-only", 1, -1, false, false},
		{"below", 2, -1, false, false},
		{"exact", 2, 0, false, false},
		{"above", 2, 1, false, false},
		{"real-io", 1, -1, true, false},
		{"cancelled", 2, -1, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tiny := []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p/></body></html>`)
			pub := &publication.Publication{}
			for _, bp := range []bookpath.BookPath{"a.xhtml", "b.xhtml"} {
				put(t, filepath.Join(dir, string(bp)), tiny)
				pub.Manifest = append(pub.Manifest, publication.Item{Path: bp, MediaType: "application/xhtml+xml", Exists: true})
			}
			r, err := openDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			inv, err := inventory(r)
			if err != nil {
				t.Fatal(err)
			}
			large := bytes.Repeat([]byte("x"), publication.XMLLimit+1)
			for i := 0; i < tc.grown; i++ {
				put(t, filepath.Join(dir, string(pub.Manifest[i].Path)), large)
			}
			if tc.missing {
				if err := os.Remove(filepath.Join(dir, "b.xhtml")); err != nil {
					t.Fatal(err)
				}
			}
			limits := references.DefaultGraphLimits
			limits.ScanBytes = 2*int64(publication.XMLLimit+1) + tc.delta
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			g, err := references.BuildSource(ctx, publicationRoot{r}, inv, pub, limits)
			if tc.cancelled || (tc.grown == 2 && tc.delta < 0) {
				if tc.cancelled {
					if !errors.Is(err, context.Canceled) {
						t.Fatal(err)
					}
				} else {
					r4WorkspaceLimit(t, err)
				}
				if !reflect.DeepEqual(g, references.Graph{}) {
					t.Fatalf("usable graph: %+v", g)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.grown == 0 {
				if g.Status != "complete" || len(g.Diagnostics) != 0 {
					t.Fatalf("normal: %+v", g)
				}
				return
			}
			want := tc.grown
			if tc.missing {
				want++
			}
			if g.Status != "partial" || len(g.Diagnostics) != want {
				t.Fatalf("partial: %+v", g)
			}
			for i, d := range g.Diagnostics {
				code := "RESOURCE_LIMIT"
				if tc.missing && i == 1 {
					code = "IO_ERROR"
				}
				if d.Code != code {
					t.Fatalf("want %s, got %+v", code, d)
				}
			}
		})
	}
}

func TestR4WorkspaceGraphEqualCaps(t *testing.T) {
	for _, tc := range []struct {
		delta         int64
		grow, missing bool
	}{{0, false, false}, {-1, true, false}, {0, true, false}, {1, true, false}, {0, false, true}} {
		t.Run(fmt.Sprintf("delta=%d/grow=%t/missing=%t", tc.delta, tc.grow, tc.missing), func(t *testing.T) {
			dir := t.TempDir()
			const bp = bookpath.BookPath("style.css")
			data := append([]byte("p{color:red}"), bytes.Repeat([]byte(" "), publication.XMLLimit-len("p{color:red}")-1)...)
			file := filepath.Join(dir, string(bp))
			put(t, file, data)
			r, err := openDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			inv, err := inventory(r)
			if err != nil {
				t.Fatal(err)
			}
			if tc.grow {
				put(t, file, append(bytes.Clone(data), ' ', ' '))
			} else if tc.missing {
				if err := os.Remove(file); err != nil {
					t.Fatal(err)
				}
			}
			pub := &publication.Publication{Manifest: []publication.Item{{Path: bp, MediaType: "text/css", Exists: true}}}
			limits := references.DefaultGraphLimits
			limits.ScanBytes = int64(publication.XMLLimit) + tc.delta
			g, err := references.BuildSource(t.Context(), publicationRoot{r}, inv, pub, limits)
			if tc.grow && tc.delta <= 0 {
				r4WorkspaceLimit(t, err)
				if !reflect.DeepEqual(g, references.Graph{}) {
					t.Fatalf("global cap returned usable graph: %+v", g)
				}
				return
			}
			// CSS coverage is partial even on a successful read. Only a global
			// limit fails the build; resource-only limits and I/O retain diagnostics.
			if err != nil || g.Status != "partial" || len(g.Coverage) == 0 {
				t.Fatalf("non-global control: %+v %v", g, err)
			}
			if tc.grow || tc.missing {
				code := "RESOURCE_LIMIT"
				if tc.missing {
					code = "IO_ERROR"
				}
				if len(g.Diagnostics) != 1 || g.Diagnostics[0].Code != code {
					t.Fatalf("lost %s classification: %+v", code, g.Diagnostics)
				}
			} else {
				for _, c := range g.Coverage {
					if c.Status == "blocked" {
						t.Fatalf("legal equal-cap read blocked: %+v", c)
					}
				}
			}
		})
	}
}
