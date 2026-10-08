package workspace

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
)

func TestN1TransactionBudget(t *testing.T) {
	for _, scope := range []string{"nodes", "operations", "resources"} {
		for _, kind := range []string{"hits", "expanded"} {
			t.Run(scope+"/"+kind, func(t *testing.T) {
				w, dir, original := structureWorkspace(t, nil)
				defer w.Close()
				a := structureBinding(t, w, "EPUB/chapter1.xhtml")
				b := structureBinding(t, w, "EPUB/chapter2.xhtml")
				ops := []Operation{a.replace(a.locatorID(t, "dir"), "literal", "RTL", "xxxxx", 1), a.replace(a.locatorID(t, "last"), "literal", "Last", "xxxxx", 1)}
				switch scope {
				case "nodes":
					ops = []Operation{a.replace("/html[1]/body[1]", "regex", "RTL|Last", "xxxxx", 2)}
				case "resources":
					ops[1] = b.replace(b.locatorID(t, "start2"), "literal", "Two", "xxxxx", 1)
				}
				budget := publication.NewReplaceBudget()
				if kind == "hits" {
					budget.Hits = 1
				} else {
					budget.ReplacementBytes = 9 // two individually legal 5-byte replacements
				}
				w.replaceBudget = budget
				before := n1Tree(t, dir)
				originalBytes := readResource(t, original)
				replaceRefused(t, w, ops, "RESOURCE_LIMIT")
				if after := n1Tree(t, dir); !reflect.DeepEqual(after, before) {
					t.Fatal("over-budget Plan created a plan/intent or changed workspace bytes")
				}
				assertBytes(t, original, originalBytes)
				budget.Hits, budget.ReplacementBytes = 2, 10
				p := replacePlan(t, w, ops)
				p2 := replacePlan(t, w, ops)
				// Plan IDs are intentionally random; the frozen payload is not.
				p2.ID = p.ID
				if !reflect.DeepEqual(p, p2) || budget.Hits != 2 || budget.ReplacementBytes != 10 {
					t.Fatalf("budget not fresh/deterministic: %+v %+v %+v", p, p2, budget)
				}
				e := applyPlan(t, w, p)
				// The oracle names every fixture resource independently of WriteSet.
				for _, path := range []string{"mimetype", "META-INF/container.xml", "EPUB/package.opf", "EPUB/chapter1.xhtml", "EPUB/chapter2.xhtml", "EPUB/nav.xhtml", "EPUB/style.css"} {
					frozen := readResource(t, filepath.Join(dir, revision, path))
					want := bytes.Clone(frozen)
					if path == "EPUB/chapter1.xhtml" {
						want = bytes.Replace(want, []byte(">RTL text<"), []byte(">xxxxx text<"), 1)
						if scope != "resources" {
							want = bytes.Replace(want, []byte(">Last.<"), []byte(">xxxxx.<"), 1)
						}
					}
					if path == "EPUB/chapter2.xhtml" && scope == "resources" {
						want = bytes.Replace(want, []byte(`id="start2">Two</h1>`), []byte(`id="start2">xxxxx</h1>`), 1)
					}
					assertBytes(t, filepath.Join(dir, candidate, path), want)
					assertBytes(t, filepath.Join(dir, revision, path), frozen)
				}
				if _, err := w.Reject(e.TaskID); err != nil {
					t.Fatal(err)
				}
				assertBytes(t, original, originalBytes)
			})
		}
	}
}

func TestN1ResourceBudget(t *testing.T) {
	for _, kind := range []string{"input", "resource", "output"} {
		t.Run(kind, func(t *testing.T) {
			w, dir, _ := structureWorkspace(t, nil)
			defer w.Close()
			a := structureBinding(t, w, "EPUB/chapter1.xhtml")
			b := structureBinding(t, w, "EPUB/chapter2.xhtml")
			ops := []Operation{a.replace(a.locatorID(t, "dir"), "literal", "RTL", "xxxxx", 1), b.replace(b.locatorID(t, "start2"), "literal", "Two", "xxxxx", 1)}
			x := readResource(t, filepath.Join(dir, revision, "EPUB/chapter1.xhtml"))
			y := readResource(t, filepath.Join(dir, revision, "EPUB/chapter2.xhtml"))
			budget := publication.NewReplaceBudget()
			var limit *int64
			var exact int64
			switch kind {
			case "input":
				limit, exact = &budget.InputBytes, int64(len(x)+len(y))
			case "resource":
				limit, exact = &budget.ResourceBytes, int64(len(x)+2)
			case "output":
				limit, exact = &budget.OutputBytes, int64(len(x)+len(y)+4)
			}
			*limit = exact - 1
			w.replaceBudget = budget
			before := n1Tree(t, dir)
			replaceRefused(t, w, ops, "RESOURCE_LIMIT")
			if after := n1Tree(t, dir); !reflect.DeepEqual(after, before) {
				t.Fatal("resource budget failure changed workspace")
			}
			*limit = exact
			p := replacePlan(t, w, ops)
			e := applyPlan(t, w, p)
			assertBytes(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"), bytes.Replace(x, []byte(">RTL text<"), []byte(">xxxxx text<"), 1))
			assertBytes(t, filepath.Join(dir, candidate, "EPUB/chapter2.xhtml"), bytes.Replace(y, []byte(`id="start2">Two</h1>`), []byte(`id="start2">xxxxx</h1>`), 1))
			if _, err := w.Reject(e.TaskID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestN1ApplyBudgetRevalidation(t *testing.T) {
	w, dir, original := structureWorkspace(t, nil)
	defer w.Close()
	a := structureBinding(t, w, "EPUB/chapter1.xhtml")
	ops := []Operation{a.replace(a.locatorID(t, "dir"), "literal", "RTL", "xxxxx", 1), a.replace(a.locatorID(t, "last"), "literal", "Last", "xxxxx", 1)}
	w.replaceBudget = publication.NewReplaceBudget()
	w.replaceBudget.ReplacementBytes = 10
	p := replacePlan(t, w, ops)
	before := n1Tree(t, dir)
	originalBytes := readResource(t, original)
	w.replaceBudget.ReplacementBytes = 9
	_, err := w.Apply(editJSON(t, p))
	var f *fault.Error
	if !errors.As(err, &f) || f.Code != "RESOURCE_LIMIT" || f.Exit != 1 {
		t.Fatalf("Apply did not recompute the shared budget: %v", err)
	}
	if after := n1Tree(t, dir); !reflect.DeepEqual(after, before) {
		t.Fatal("failed Apply published an intent/checkpoint or changed accepted")
	}
	assertBytes(t, original, originalBytes)
	if _, err := Open(dir); !errors.Is(err, ErrBusy) {
		t.Fatalf("writer lock changed: %v", err)
	}
	w.replaceBudget.ReplacementBytes = 10
	e := applyPlan(t, w, p)
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	w, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	status, err := w.TaskStatus(e.TaskID)
	if err != nil || !strings.Contains(status.Status, "reject") {
		t.Fatalf("legal old-schema history did not reopen: %+v %v", status, err)
	}
	assertBytes(t, original, originalBytes)
}

func TestN1PostStartBudgetRecovery(t *testing.T) {
	for _, lower := range []bool{false, true} {
		t.Run(map[bool]string{false: "control", true: "lower-budget"}[lower], func(t *testing.T) {
			w, dir, original := structureWorkspace(t, nil)
			defer w.Close()
			a := structureBinding(t, w, "EPUB/chapter1.xhtml")
			b := structureBinding(t, w, "EPUB/chapter2.xhtml")
			w.replaceBudget = publication.NewReplaceBudget()
			w.replaceBudget.ReplacementBytes = 10
			p := replacePlan(t, w, []Operation{a.replace(a.locatorID(t, "dir"), "literal", "RTL", "xxxxx", 1), b.replace(b.locatorID(t, "start2"), "literal", "Two", "xxxxx", 1)})
			start, outputs := prepareExecution(t, w, p)
			paths := []string{"original/book.epub", "plans/" + p.ID + ".json", "plans/" + p.ID + ".used.json", "tasks/active/edit-intent.json", "tasks/active/edit-start.json", checkpointDir(start.Checkpoint) + "/checkpoint.json"}
			audit := make(map[string][]byte)
			for _, path := range paths {
				audit[path] = readResource(t, filepath.Join(dir, path))
			}
			accepted := n1Tree(t, filepath.Join(dir, revision))
			originalBytes := readResource(t, original)
			if err := w.writeCandidateFile("EPUB/chapter1.xhtml", outputs["EPUB/chapter1.xhtml"]); err != nil {
				t.Fatal(err)
			}
			frozenChapter := readResource(t, filepath.Join(dir, revision, "EPUB/chapter1.xhtml"))
			partialChapter := bytes.Replace(frozenChapter, []byte(">RTL text<"), []byte(">xxxxx text<"), 1)
			assertBytes(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"), partialChapter)
			if lower {
				w.replaceBudget.ReplacementBytes = 9
			}
			e, err := w.Execution()
			for _, path := range []string{"mimetype", "META-INF/container.xml", "EPUB/package.opf", "EPUB/chapter1.xhtml", "EPUB/chapter2.xhtml", "EPUB/nav.xhtml", "EPUB/style.css"} {
				frozen := readResource(t, filepath.Join(dir, revision, path))
				actual := readResource(t, filepath.Join(dir, candidate, path))
				if !bytes.Equal(actual, frozen) {
					t.Fatalf("post-start re-derivation error blocked checkpoint rollback for %s: err=%v", path, err)
				}
			}
			if lower {
				var f *fault.Error
				if !errors.As(err, &f) || f.Code != "RESOURCE_LIMIT" || f.Exit != 1 || errors.Is(err, ErrCandidateDrift) {
					t.Fatalf("budget recovery must remain a resource failure, not tolerated drift: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if e.Status != "failed" || e.ReviewRequired || e.Conformance != "not_run" || e.Failure == "" || !e.Diff.Changed || len(e.Diff.Changes) != 1 || e.Diff.Changes[0].Path != "EPUB/chapter1.xhtml" {
				t.Fatalf("lost failed audit or observed pre-rollback diff: %+v", e)
			}
			change := e.Diff.Changes[0]
			beforeHash, afterHash := sha256.Sum256(frozenChapter), sha256.Sum256(partialChapter)
			if change.Before == nil || change.After == nil || change.Before.Size != int64(len(frozenChapter)) || change.After.Size != int64(len(partialChapter)) || change.Before.SHA256 != hex.EncodeToString(beforeHash[:]) || change.After.SHA256 != hex.EncodeToString(afterHash[:]) {
				t.Fatalf("failed diff lost actual pre-rollback bytes: %+v", change)
			}
			result := readResource(t, filepath.Join(dir, "tasks/active/edit-result.json"))
			if lower {
				_, retryErr := w.Execution()
				var f *fault.Error
				if !errors.As(retryErr, &f) || f.Code != "RESOURCE_LIMIT" {
					t.Fatalf("failed status bypassed re-derivation budget: %v", retryErr)
				}
				if _, err := w.Reject(e.TaskID); !errors.As(err, &f) || f.Code != "RESOURCE_LIMIT" {
					t.Fatalf("rejection bypassed incomplete effect verification: %v", err)
				}
				assertBytes(t, filepath.Join(dir, "tasks/active/edit-result.json"), result)
			}
			for path, frozen := range audit {
				assertBytes(t, filepath.Join(dir, path), frozen)
			}
			if !reflect.DeepEqual(n1Tree(t, filepath.Join(dir, revision)), accepted) {
				t.Fatal("recovery changed accepted")
			}
			assertBytes(t, original, originalBytes)
			w.replaceBudget.ReplacementBytes = 10
			if _, err := w.Execution(); err != nil {
				t.Fatal(err)
			}
			if _, err := w.Reject(e.TaskID); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			w, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			if _, err := w.TaskStatus(e.TaskID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestN1BudgetRecoveryBoundaries(t *testing.T) {
	for _, boundary := range []string{"no-start", "unstarted", "completed", "used", "start", "checkpoint", "plan", "baseline"} {
		t.Run(boundary, func(t *testing.T) {
			w, dir, original := structureWorkspace(t, nil)
			defer w.Close()
			a := structureBinding(t, w, "EPUB/chapter1.xhtml")
			b := structureBinding(t, w, "EPUB/chapter2.xhtml")
			w.replaceBudget = publication.NewReplaceBudget()
			w.replaceBudget.ReplacementBytes = 10
			p := replacePlan(t, w, []Operation{a.replace(a.locatorID(t, "dir"), "literal", "RTL", "xxxxx", 1), b.replace(b.locatorID(t, "start2"), "literal", "Two", "xxxxx", 1)})
			originalBytes := readResource(t, original)
			var start Execution
			if boundary == "no-start" {
				if _, err := w.createCandidate(&p); err != nil {
					t.Fatal(err)
				}
			} else if boundary == "completed" {
				applyPlan(t, w, p)
			} else {
				start, _ = prepareExecution(t, w, p)
			}
			if boundary != "completed" {
				frozen := readResource(t, filepath.Join(dir, revision, "EPUB/chapter1.xhtml"))
				put(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"), bytes.Replace(frozen, []byte(">RTL text<"), []byte(">xxxxx text<"), 1))
			}
			switch boundary {
			case "unstarted":
				start.Status = "unstarted"
				put(t, filepath.Join(dir, "tasks/active/edit-start.json"), editJSON(t, start))
			case "used":
				var used planUse
				if err := readEditJSON(w.root, "plans/"+p.ID+".used.json", &used); err != nil {
					t.Fatal(err)
				}
				used.PlanSHA256 = strings.Repeat("0", 64)
				put(t, filepath.Join(dir, "plans", p.ID+".used.json"), editJSON(t, used))
			case "start":
				start.TaskID = strings.Repeat("0", 32)
				put(t, filepath.Join(dir, "tasks/active/edit-start.json"), editJSON(t, start))
			case "checkpoint":
				put(t, filepath.Join(dir, checkpointDir(start.Checkpoint), "pub/EPUB/chapter2.xhtml"), []byte("forged checkpoint"))
			case "plan":
				p.Applicable = false
				put(t, filepath.Join(dir, "plans", p.ID+".json"), editJSON(t, p))
			case "baseline":
				put(t, filepath.Join(dir, revision, "EPUB/chapter2.xhtml"), []byte("baseline drift"))
			}
			before := n1Tree(t, dir)
			w.replaceBudget.ReplacementBytes = 9
			_, err := w.Execution()
			if err == nil || errors.Is(err, ErrCandidateDrift) {
				t.Fatalf("incomplete/forged recovery was tolerated: %v", err)
			}
			var f *fault.Error
			resourceError := errors.As(err, &f) && f.Code == "RESOURCE_LIMIT"
			if resourceError != (boundary == "no-start" || boundary == "unstarted" || boundary == "completed") {
				t.Fatalf("budget failure bypassed provenance check for %s: %v", boundary, err)
			}
			if !reflect.DeepEqual(n1Tree(t, dir), before) {
				t.Fatal("budget/provenance failure rewrote an unstarted/completed/forged task")
			}
			assertBytes(t, original, originalBytes)
		})
	}
}

func FuzzN1BudgetTransaction(f *testing.F) {
	for mode := uint8(0); mode < 2; mode++ {
		for replacement := uint8(0); replacement < 4; replacement++ {
			f.Add(mode, replacement)
		}
	}
	f.Fuzz(func(t *testing.T, mode, replacement uint8) {
		values := []struct{ decoded, escaped string }{
			{"", ""}, {"xxxxx", "xxxxx"}, {"&<>新🙂", "&amp;&lt;&gt;新🙂"}, {"RTL", "RTL"},
		}
		value := values[int(replacement)%len(values)]
		w, dir, original := structureWorkspace(t, nil)
		defer w.Close()
		a := structureBinding(t, w, "EPUB/chapter1.xhtml")
		b := structureBinding(t, w, "EPUB/chapter2.xhtml")
		matchMode := []string{"literal", "regex"}[mode%2]
		w.replaceBudget = publication.NewReplaceBudget()
		w.replaceBudget.Hits = 2
		w.replaceBudget.ReplacementBytes = int64(2 * len(value.decoded))
		p := replacePlan(t, w, []Operation{a.replace(a.locatorID(t, "dir"), matchMode, "RTL", value.decoded, 1), b.replace(b.locatorID(t, "start2"), matchMode, "Two", value.decoded, 1)})
		originalBytes := readResource(t, original)
		e := applyPlan(t, w, p) // Every generated transaction is legal; rejection is FAIL.
		for _, path := range []string{"mimetype", "META-INF/container.xml", "EPUB/package.opf", "EPUB/chapter1.xhtml", "EPUB/chapter2.xhtml", "EPUB/nav.xhtml", "EPUB/style.css"} {
			frozen := readResource(t, filepath.Join(dir, revision, path))
			want := bytes.Clone(frozen)
			if path == "EPUB/chapter1.xhtml" {
				want = bytes.Replace(want, []byte(">RTL text<"), []byte(">"+value.escaped+" text<"), 1)
			} else if path == "EPUB/chapter2.xhtml" {
				want = bytes.Replace(want, []byte(`id="start2">Two</h1>`), []byte(`id="start2">`+value.escaped+`</h1>`), 1)
			}
			assertBytes(t, filepath.Join(dir, candidate, path), want)
			assertBytes(t, filepath.Join(dir, revision, path), frozen)
		}
		if _, err := w.Reject(e.TaskID); err != nil {
			t.Fatal(err)
		}
		assertBytes(t, original, originalBytes)
	})
}

func n1Tree(t *testing.T, dir string) Tree {
	t.Helper()
	tree, err := HashTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}
