package workspace

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/fix"
	"github.com/LeviTK/Kepub/internal/validation"
)

// fixSourceProposal derives the complete explicit proposal of the shared
// fixture; the independent expected bytes stay the frozen literal edits.
func fixSourceProposal(t *testing.T, w *Workspace) fix.Proposal {
	t.Helper()
	s, err := w.FixSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	all, err := fix.Propose(s, fix.Selection{Mode: fix.ModeAll, RepairIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, r := range all.Repairs {
		ids = append(ids, r.RepairID)
	}
	p, err := fix.Propose(s, fix.Selection{Mode: fix.ModeExplicit, RepairIDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func fixSourceWant(src string) []byte {
	want := strings.Replace(src, ` epub:type="secrecy"`, "", 1)
	return []byte(strings.Replace(want, "?q=1#start2", "#start2", 1))
}

// fixSourceSVG is the same fixture with an inline SVG so the shared structure
// package document, which declares properties="svg", stays conformance-positive
// for the acceptance path.
func fixSourceSVG() string {
	return `<?xml version="1.0" encoding="utf-8"?>` + "\n" + strings.Replace(fixSource,
		`<p id="start">One.</p>`,
		`<p id="start">One.</p><p>Text <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg> tail.</p>`, 1)
}

// TestFixSavedSourceRevalidation pins the frozen rule that a stored schema 7
// source is re-derived from its own baseline on every consumption path: a
// re-signed risk or a truncated operation list with a recomputed digest is
// refused without publishing a candidate, while the untampered control applies
// with the exact expected bytes.
func TestFixSavedSourceRevalidation(t *testing.T) {
	forge := func(t *testing.T, w *Workspace, dir string, kind string) {
		t.Helper()
		plan, err := w.Plan(fixSchema7JSON(t, fixSourceProposal(t, w)))
		if err != nil {
			t.Fatal(err)
		}
		switch kind {
		case "resigned-risk":
			plan.Proposal.Repairs[0].Risk = "forged approved safe repair"
			if err := plan.Proposal.Sign(); err != nil {
				t.Fatal(err)
			}
		case "operation-subset":
			plan.Operations = plan.Operations[:1]
			plan.OperationSetSHA256 = digest(plan.Operations)
		}
		put(t, filepath.Join(dir, "plans", plan.ID+".json"), editJSON(t, plan))
		if _, err := w.Apply(editJSON(t, plan)); err == nil {
			t.Fatal("forged schema7 source accepted by Apply")
		}
		if exists(w.root, "tasks/active") {
			t.Fatal("refused source published a candidate")
		}
		if _, err := os.Lstat(filepath.Join(dir, candidate)); !os.IsNotExist(err) {
			t.Fatal("refused source created candidate bytes")
		}
	}
	t.Run("control", func(t *testing.T) {
		w, dir, _ := structureWorkspace(t, map[string]string{"EPUB/chapter1.xhtml": fixSource})
		defer w.Close()
		plan, err := w.Plan(fixSchema7JSON(t, fixSourceProposal(t, w)))
		if err != nil {
			t.Fatal(err)
		}
		e := applyPlan(t, w, plan)
		if got := readResource(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml")); !bytes.Equal(got, fixSourceWant(fixSource)) {
			t.Fatalf("control bytes %q", got)
		}
		if _, err := w.Reject(e.TaskID); err != nil {
			t.Fatal(err)
		}
	})
	for _, kind := range []string{"resigned-risk", "operation-subset"} {
		t.Run(kind, func(t *testing.T) {
			w, dir, _ := structureWorkspace(t, map[string]string{"EPUB/chapter1.xhtml": fixSource})
			defer w.Close()
			forge(t, w, dir, kind)
		})
	}
	// The schema 7 task review carries the complete re-derived source with the
	// actual old values and planned new values; old schemas keep their shape.
	t.Run("task-diff-source", func(t *testing.T) {
		w, _, _ := structureWorkspace(t, map[string]string{"EPUB/chapter1.xhtml": fixSource})
		defer w.Close()
		p := fixSourceProposal(t, w)
		plan, err := w.Plan(fixSchema7JSON(t, p))
		if err != nil {
			t.Fatal(err)
		}
		e := applyPlan(t, w, plan)
		r, err := w.TaskDiff(e.TaskID)
		if err != nil {
			t.Fatal(err)
		}
		if r.Proposal == nil || !fix.CanonicalEqual(*r.Proposal, &p) {
			t.Fatalf("task diff source: %+v", r.Proposal)
		}
		old, err := w.TaskDiff(e.TaskID)
		if err != nil {
			t.Fatal(err)
		}
		if old.Proposal == nil || len(old.Proposal.Repairs) != 2 {
			t.Fatalf("task diff repairs: %+v", old.Proposal)
		}
		if _, err := w.Reject(e.TaskID); err != nil {
			t.Fatal(err)
		}
	})
	// A plan bound to an earlier revision still re-derives from that frozen
	// revision, never from the current accepted tree.
	t.Run("frozen-baseline", func(t *testing.T) {
		if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
			t.Skip("real pinned EPUBCheck required; set KEPUB_EPUBCHECK_JAR")
		}
		w, _, _ := structureWorkspace(t, map[string]string{"EPUB/chapter1.xhtml": fixSourceSVG()})
		defer w.Close()
		plan, err := w.Plan(fixSchema7JSON(t, fixSourceProposal(t, w)))
		if err != nil {
			t.Fatal(err)
		}
		e := applyPlan(t, w, plan)
		if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{Timeout: 2 * time.Minute}); err != nil {
			t.Fatal(err)
		}
		if w.current == plan.BaseRevision {
			t.Fatal("accept did not advance the accepted revision")
		}
		if err := w.verifyFixPlan(plan); err != nil {
			t.Fatalf("plan bound to its own earlier revision: %v", err)
		}
		forged := plan
		forged.Proposal = plan.Proposal
		forged.Proposal.Repairs = append([]fix.Repair{}, plan.Proposal.Repairs...)
		forged.Proposal.Repairs[0].Risk = "forged approved safe repair"
		if err := forged.Proposal.Sign(); err != nil {
			t.Fatal(err)
		}
		if err := w.verifyFixPlan(forged); err == nil {
			t.Fatal("forged source passed the frozen-baseline re-derivation")
		}
	})
}

// Re-sign all stored copies and journal digests, leaving operations and legal
// bytes untouched. Test the real consumers, not just verifyFixPlan directly.
func TestFixRejectedSourceRevalidation(t *testing.T) {
	for _, boundary := range []string{"history", "history-advanced", "recovery", "recovery-archived"} {
		for _, forged := range []bool{false, true} {
			name := "legal"
			if forged {
				name = "resigned-risk"
			}
			t.Run(boundary+"/"+name, func(t *testing.T) {
				if boundary == "history-advanced" && os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
					t.Skip("real pinned EPUBCheck required to advance accepted")
				}
				src := fixSourceSVG()
				w, dir, _ := structureWorkspace(t, map[string]string{"EPUB/chapter1.xhtml": src})
				plan, err := w.Plan(fixSchema7JSON(t, fixSourceProposal(t, w)))
				if err != nil {
					t.Fatal(err)
				}
				e := applyPlan(t, w, plan)
				taskDir := "tasks/active"
				j := settlement{Version: 1, WorkspaceID: w.id, Decision: Decision{Version: 1, TaskID: e.TaskID, Status: "rejected", BaseRevision: plan.BaseRevision, TreeSHA256: e.Diff.AfterSHA256}}
				if err := w.taskDigests(taskDir, &j); err != nil {
					t.Fatal(err)
				}
				if strings.HasPrefix(boundary, "history") {
					if _, err := w.Reject(e.TaskID); err != nil {
						t.Fatal(err)
					}
					taskDir = "tasks/" + e.TaskID
					if boundary == "history-advanced" {
						next, err := w.Plan(fixSchema7JSON(t, fixSourceProposal(t, w)))
						if err != nil {
							t.Fatal(err)
						}
						applied := applyPlan(t, w, next)
						if _, err := w.Accept(context.Background(), applied.TaskID, validation.Options{Timeout: 2 * time.Minute}); err != nil {
							t.Fatal(err)
						}
						if w.current == plan.BaseRevision {
							t.Fatal("accepted did not advance")
						}
					}
				}
				if forged {
					plan.Proposal.Repairs[0].Risk = "forged safe approved repair"
					if err := plan.Proposal.Sign(); err != nil {
						t.Fatal(err)
					}
					var start, result Execution
					if err := readEditJSON(w.root, taskDir+"/edit-start.json", &start); err != nil {
						t.Fatal(err)
					}
					if err := readEditJSON(w.root, taskDir+"/edit-result.json", &result); err != nil {
						t.Fatal(err)
					}
					start.Plan, result.Plan = plan, plan
					for path, record := range map[string]any{
						taskDir + "/edit-intent.json": plan, taskDir + "/edit-start.json": start, taskDir + "/edit-result.json": result,
						"plans/" + plan.ID + ".json": plan, "plans/" + plan.ID + ".used.json": planUse{1, e.TaskID, digest(plan)},
					} {
						put(t, filepath.Join(dir, filepath.FromSlash(path)), editJSON(t, record))
					}
					j.IntentSHA256, j.StartSHA256, j.ResultSHA256 = digest(plan), digest(start), digest(result)
				}
				if strings.HasPrefix(boundary, "recovery") {
					if err := writeJSON(w.root, settlementJournal, j); err != nil {
						t.Fatal(err)
					}
					if boundary == "recovery-archived" {
						if err := w.root.Rename("tasks/active", "tasks/"+e.TaskID); err != nil {
							t.Fatal(err)
						}
					}
				}
				w.Close()
				w, err = Open(dir)
				if err == nil {
					defer w.Close()
					var status TaskStatus
					status, err = w.TaskStatus(e.TaskID)
					if !forged && (err != nil || status.Status != "rejected" || status.BaseRevision != plan.BaseRevision) {
						t.Fatalf("legal historical consumer: %+v %v", status, err)
					}
				} else if !forged {
					t.Fatalf("legal recovery: %v", err)
				}
				if forged && !errors.Is(err, ErrStalePlan) {
					t.Fatalf("complete-source forgery must fail re-derivation, got %v", err)
				}
				if got := readResource(t, filepath.Join(dir, revision, "EPUB/chapter1.xhtml")); !bytes.Equal(got, []byte(src)) {
					t.Fatal("frozen baseline changed")
				}
				if forged && strings.HasPrefix(boundary, "recovery") {
					if _, err := os.Stat(filepath.Join(dir, settlementJournal)); err != nil {
						t.Fatal("rejected provenance must not consume the recovery journal")
					}
				}
			})
		}
	}
}

func TestN2LegacySchemas(t *testing.T) {
	for _, version := range []int{4, 5, 6, 7} {
		t.Run(map[int]string{4: "structure", 5: "replace", 6: "move", 7: "proposal"}[version], func(t *testing.T) {
			files := map[string]string{}
			if version == 6 {
				files = moveCrossFiles()
			} else if version == 7 {
				files["EPUB/chapter1.xhtml"] = fixSourceSVG()
			}
			w, dir, original := structureWorkspace(t, files)
			defer func() { w.Close() }()
			b := structureBinding(t, w, "EPUB/chapter1.xhtml")
			var ops []Operation
			var proposal *fix.Proposal
			switch version {
			case 4:
				ops = []Operation{b.attrSet(b.locatorID(t, "dir"), "class", nil, "n2")}
			case 5:
				ops = []Operation{b.replace(b.locatorID(t, "dir"), "literal", "RTL", "LTR", 1)}
			case 6:
				dst := structureBinding(t, w, "EPUB/text/chapter3.xhtml")
				ops = []Operation{b.elemMoveCross(dst, b.locatorID(t, "moveblock"), dst.locatorID(t, "three"), "after")}
			case 7:
				p := fixSourceProposal(t, w)
				fresh, err := w.Plan(fixSchema7JSON(t, p))
				if err != nil {
					t.Fatal(err)
				}
				ops, proposal = fresh.Operations, fresh.Proposal
			}
			p, derived := n2LegacyPlan(t, w, ops)
			p.SchemaVersion, p.Proposal = version, proposal
			p.PolicySHA256 = digest(legacyPolicyFor(version))
			put(t, filepath.Join(dir, "plans", p.ID+".json"), editJSON(t, p))
			originalBytes := readResource(t, original)
			if _, err := w.Apply(editJSON(t, p)); !errors.Is(err, ErrStalePlan) {
				t.Fatalf("legacy schema authorized new Apply: %v", err)
			}
			if _, err := w.createCandidate(&p); err != nil {
				t.Fatal(err)
			}
			e, err := w.startExecution(p)
			if err != nil {
				t.Fatal(err)
			}
			e, err = w.execute(e, derived.outputs, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Reject(e.TaskID); err != nil {
				t.Fatal(err)
			}
			w.Close()
			w, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			status, err := w.TaskStatus(e.TaskID)
			if err != nil || status.Status != "rejected" {
				t.Fatalf("legacy schema history: %+v %v", status, err)
			}
			var fresh Plan
			if version == 7 {
				fresh, err = w.Plan(fixSchema7JSON(t, *proposal))
			} else {
				fresh, err = w.Plan(editJSON(t, Request{version, ops}))
			}
			wantPolicy := legacyPolicyFor(version) + ";reference-parser-v2"
			if version == 7 {
				wantPolicy += ";file-target-v2"
			}
			if err != nil || fresh.SchemaVersion != version || fresh.PolicySHA256 != digest(wantPolicy) || fresh.PolicySHA256 == p.PolicySHA256 {
				t.Fatalf("new schema lost explicit parser binding: %+v %v", fresh, err)
			}
			assertBytes(t, original, originalBytes)
		})
	}
}

func TestN2LegacyAcceptedJournal(t *testing.T) {
	requireChecker(t)
	w, dir, original := structureWorkspace(t, nil)
	defer func() { w.Close() }()
	b := structureBinding(t, w, "EPUB/chapter1.xhtml")
	p, derived := n2LegacyPlan(t, w, []Operation{b.attrSet(b.locatorID(t, "dir"), "class", nil, "legacy")})
	if _, err := w.createCandidate(&p); err != nil {
		t.Fatal(err)
	}
	e, err := w.startExecution(p)
	if err != nil {
		t.Fatal(err)
	}
	e, err = w.execute(e, derived.outputs, nil)
	if err != nil {
		t.Fatal(err)
	}
	rp, err := validation.Validate(context.Background(), filepath.Join(dir, candidate), validation.Options{})
	if err != nil || rp.Status != "pass" || len(rp.Checks) < 4 || rp.Checks[3].ID != "epubcheck" || !rp.Checks[3].Required || rp.Checks[3].Status != "passed" || rp.Checks[3].Version != "5.3.0" {
		t.Fatalf("legacy journal requires real formal checks: %+v %v", rp, err)
	}
	id := randomID()
	stage := "staging/accept-" + id
	if err := w.root.Mkdir(stage, 0700); err != nil {
		t.Fatal(err)
	}
	tree, err := copyTree(w.root, candidate, stage+"/pub")
	if err != nil || tree.SHA256 != rp.InputTreeSHA256 {
		t.Fatalf("validation source: %v", err)
	}
	r := Revision{Version: 1, ID: id, WorkspaceID: w.id, Parent: "initial", TaskID: e.TaskID, ExecutionSHA256: digest(e), Rootfile: w.state.Rootfile, Tree: tree, Validation: &rp}
	if err := writeJSON(w.root, stage+"/revision.json", r); err != nil {
		t.Fatal(err)
	}
	j := settlement{Version: 1, WorkspaceID: w.id, Decision: Decision{Version: 1, TaskID: e.TaskID, Status: "accepted", BaseRevision: "initial", RevisionID: id, TreeSHA256: tree.SHA256, Validation: &rp}}
	if err := w.taskDigests("tasks/active", &j); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(w.root, settlementJournal, j); err != nil {
		t.Fatal(err)
	}
	frozen := readResource(t, filepath.Join(dir, "plans", p.ID+".json"))
	originalBytes := readResource(t, original)
	w.Close()
	w, err = Open(dir)
	if err != nil || w.current != id {
		t.Fatalf("durable old decision did not roll forward: %v", err)
	}
	status, err := w.TaskStatus(e.TaskID)
	if err != nil || status.Status != "accepted" || status.Decision.Validation.Status != "pass" {
		t.Fatalf("old accepted history: %+v %v", status, err)
	}
	assertBytes(t, filepath.Join(dir, "plans", p.ID+".json"), frozen)
	assertBytes(t, original, originalBytes)
}
