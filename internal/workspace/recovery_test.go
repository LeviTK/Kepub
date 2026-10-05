package workspace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/LeviTK/Kepub/internal/validation"
)

func TestUnstartedRecoveryPreservesDriftAcrossResultInterruption(t *testing.T) {
	for _, operation := range []string{"metadata", "content"} {
		t.Run(operation, func(t *testing.T) {
			w, dir, _ := legalWorkspace(t, "3.0")
			p := fieldPlan(t, w, "title", "title", "Title", "New")
			if operation == "content" {
				p = contentPlan(t, w, "Changed")
			}
			_, err := w.createCandidate(&p)
			if err != nil {
				t.Fatal(err)
			}
			id, err := w.taskID()
			if err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(dir, candidate, "user-notes.txt"), []byte("outside changes"))
			before := treeAt(t, filepath.Join(dir, candidate)).SHA256
			w.Close()
			// Use the actual synthesized start, then materialize interruption
			// before its failed result publication. Repeated retries must not
			// reinterpret that record as authorization to undo outside changes.
			for attempt := 0; attempt < 3; attempt++ {
				w, err = Open(dir)
				if err != nil {
					t.Fatal(err)
				}
				e, err := w.Execution()
				if !errors.Is(err, ErrCandidateDrift) || e.TaskID != id || e.Status != "failed" || e.ReviewRequired {
					t.Fatalf("outside changes no longer reviewable: %+v, %v", e, err)
				}
				if treeAt(t, filepath.Join(dir, candidate)).SHA256 != before {
					t.Fatal("synthesized recovery destroyed outside changes")
				}
				if _, err := w.TaskDiff(id); err != nil {
					t.Fatal(err)
				}
				if _, err := w.Accept(context.Background(), id, validation.Options{}); !errors.Is(err, ErrCandidateDrift) {
					t.Fatalf("drift accepted: %v", err)
				}
				if attempt < 2 {
					if err := w.root.Remove("tasks/active/edit-result.json"); err != nil {
						t.Fatal(err)
					}
				} else if _, err := w.Reject(id); err != nil {
					t.Fatal(err)
				}
				w.Close()
			}
			w, err = Open(dir)
			if err != nil {
				t.Fatal("rejected synthetic history", err)
			}
			if treeAt(t, filepath.Join(dir, revisionPath(w.current))).SHA256 != p.InputTreeSHA256 {
				t.Fatal("accepted publication changed")
			}
			w.Close()
		})
	}
}

func TestInterruptedApplyWithPreJournalRestoreLeftovers(t *testing.T) {
	for _, operation := range []string{"metadata", "content"} {
		for _, phase := range []string{"partial-copy", "unpublished-journal"} {
			t.Run(operation+"/"+phase, func(t *testing.T) {
				var w *Workspace
				var dir string
				var p Plan
				if operation == "metadata" {
					w, dir = makeWorkspace(t)
					p = planTitle(t, w, "New")
				} else {
					w, dir, _ = legalWorkspace(t, "3.0")
					p = contentPlan(t, w, "Changed body")
				}
				e, out := prepareExecution(t, w, p)
				put(t, filepath.Join(dir, candidate, p.WriteSet[0]), out)
				if phase == "partial-copy" {
					put(t, filepath.Join(dir, restoreNew, "partial"), []byte("unfinished restore copy"))
				} else {
					s, err := w.snapshot(e.Checkpoint)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := copyTree(w.root, checkpointDir(s.ID)+"/pub", restoreNew); err != nil {
						t.Fatal(err)
					}
					if err := writeJSON(w.root, "staging/restore.json", restoreRecord{1, s.ID, s.Tree.SHA256}); err != nil {
						t.Fatal(err)
					}
				}
				put(t, filepath.Join(dir, "user-file"), []byte("outside reserved staging"))
				w.Close()
				for attempt := 0; attempt < 3; attempt++ {
					var err error
					w, err = Open(dir)
					if err != nil {
						t.Fatalf("recovery retry %d: %v", attempt, err)
					}
					got, err := w.Execution()
					if err != nil || got.TaskID != e.TaskID || got.Status != "failed" || got.Failure != "interrupted apply" || got.ReviewRequired {
						t.Fatalf("recovered execution: %+v, %v", got, err)
					}
					if treeAt(t, filepath.Join(dir, candidate)).SHA256 != p.InputTreeSHA256 {
						t.Fatal("rollback did not restore exact baseline")
					}
					if _, err := w.TaskDiff(e.TaskID); err != nil {
						t.Fatal(err)
					}
					if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{}); !errors.Is(err, ErrTaskConflict) {
						t.Fatalf("failed accepted: %v", err)
					}
					assertEmpty(t, filepath.Join(dir, "staging"))
					assertEmpty(t, filepath.Join(dir, "journal"))
					assertBytes(t, filepath.Join(dir, "user-file"), []byte("outside reserved staging"))
					if attempt == 2 {
						if _, err := w.Reject(e.TaskID); err != nil {
							t.Fatal(err)
						}
					}
					w.Close()
				}
			})
		}
	}
}

func TestRecoveryRejectsAlteredIdentityBeforeWriting(t *testing.T) {
	for _, operation := range []string{"metadata", "content"} {
		for _, phase := range []string{"intent", "start"} {
			for _, tamper := range []string{"missing-use", "malformed-use", "wrong-use-task", "wrong-use-digest", "wrong-use-version", "downgraded-task", "wrong-start"} {
				if phase == "intent" && tamper == "wrong-start" {
					continue
				}
				t.Run(operation+"/"+phase+"/"+tamper, func(t *testing.T) {
					w, dir, _ := legalWorkspace(t, "3.0")
					p := fieldPlan(t, w, "title", "title", "Title", "New")
					if operation == "content" {
						p = contentPlan(t, w, "Changed")
					}
					var e Execution
					if phase == "intent" {
						if _, err := w.createCandidate(&p); err != nil {
							t.Fatal(err)
						}
						put(t, filepath.Join(dir, candidate, "external.txt"), []byte("outside drift"))
					} else {
						var out []byte
						e, out = prepareExecution(t, w, p)
						put(t, filepath.Join(dir, candidate, p.WriteSet[0]), out)
					}
					usedName := "plans/" + p.ID + ".used.json"
					var used planUse
					if err := readEditJSON(w.root, usedName, &used); err != nil {
						t.Fatal(err)
					}
					switch tamper {
					case "missing-use":
						if err := w.root.Remove(usedName); err != nil {
							t.Fatal(err)
						}
					case "malformed-use":
						put(t, filepath.Join(dir, usedName), []byte("{"))
					case "downgraded-task":
						put(t, filepath.Join(dir, "tasks/active/task.json"), editJSON(t, taskRecord{Version: 1, BaseRevision: "initial"}))
					case "wrong-start":
						e.TaskID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						put(t, filepath.Join(dir, "tasks/active/edit-start.json"), editJSON(t, e))
					default:
						switch tamper {
						case "wrong-use-task":
							used.TaskID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						case "wrong-use-digest":
							used.PlanSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						case "wrong-use-version":
							used.Version = 2
						}
						put(t, filepath.Join(dir, usedName), editJSON(t, used))
					}
					w.Close()
					before := treeAt(t, dir).SHA256
					for attempt := 0; attempt < 2; attempt++ {
						opened, err := Open(dir)
						if opened != nil {
							opened.Close()
						}
						if err == nil {
							t.Fatal("tampered recovery accepted")
						}
						if after := treeAt(t, dir).SHA256; after != before {
							t.Fatalf("recovery changed bytes/records before refusing: %s → %s, %v", before, after, err)
						}
					}
				})
			}
		}
	}
}

func TestLegacyRegisteredRecoveryAllowsEmptyStartID(t *testing.T) {
	for _, recordedUse := range []bool{false, true} {
		t.Run(fmt.Sprint(recordedUse), func(t *testing.T) {
			w, dir := makeWorkspace(t)
			p := planTitle(t, w, "Legacy edit")
			e, out := prepareExecution(t, w, p)
			put(t, filepath.Join(dir, candidate, p.WriteSet[0]), out)
			put(t, filepath.Join(dir, "tasks/active/task.json"), editJSON(t, taskRecord{Version: 1, BaseRevision: "initial"}))
			e.TaskID = ""
			put(t, filepath.Join(dir, "tasks/active/edit-start.json"), editJSON(t, e))
			if err := w.root.Remove("plans/" + p.ID + ".used.json"); err != nil {
				t.Fatal(err)
			}
			if recordedUse {
				if err := writeJSON(w.root, "plans/"+p.ID+".used.json", planUse{1, "active", digest(p)}); err != nil {
					t.Fatal(err)
				}
			}
			w.Close()
			w, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			got, err := w.Execution()
			if err != nil || got.TaskID != "" || got.Status != "failed" || got.ReviewRequired {
				t.Fatalf("legacy recovery: %+v, %v", got, err)
			}
			if treeAt(t, filepath.Join(dir, candidate)).SHA256 != p.InputTreeSHA256 {
				t.Fatal("legacy rollback bytes changed")
			}
			if _, err := w.Reject("active"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCommittedRestoreFinishesBeforeExecutionRefusal(t *testing.T) {
	for _, operation := range []string{"metadata", "content"} {
		for _, tamper := range []string{"used", "start"} {
			for _, journal := range []string{"valid", "valid-pub-missing", "wrong-hash", "malformed"} {
				t.Run(operation+"/"+tamper+"/"+journal, func(t *testing.T) {
					w, dir, _ := legalWorkspace(t, "3.0")
					p := fieldPlan(t, w, "title", "title", "Title", "New")
					if operation == "content" {
						p = contentPlan(t, w, "Changed")
					}
					e, out := prepareExecution(t, w, p)
					put(t, filepath.Join(dir, candidate, p.WriteSet[0]), out)
					s, err := w.snapshot(e.Checkpoint)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := copyTree(w.root, checkpointDir(s.ID)+"/pub", restoreNew); err != nil {
						t.Fatal(err)
					}
					j := restoreRecord{1, s.ID, s.Tree.SHA256}
					if journal == "wrong-hash" {
						j.SHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
					}
					if err := writeJSON(w.root, restoreJournal, j); err != nil {
						t.Fatal(err)
					}
					if journal == "malformed" {
						put(t, filepath.Join(dir, restoreJournal), []byte("{"))
					}
					if journal == "valid-pub-missing" {
						if err := publish(w.root, candidate, restoreOld); err != nil {
							t.Fatal(err)
						}
					}
					usedName := filepath.Join(dir, "plans", p.ID+".used.json")
					startName := filepath.Join(dir, "tasks/active/edit-start.json")
					if tamper == "used" {
						put(t, usedName, editJSON(t, planUse{1, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", digest(p)}))
					} else {
						e.TaskID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
						put(t, startName, editJSON(t, e))
					}
					put(t, filepath.Join(dir, "user-file"), []byte("keep"))
					w.Close()
					before := treeAt(t, dir).SHA256
					checkpoints := treeAt(t, filepath.Join(dir, "tasks/active/checkpoints")).SHA256
					original := treeAt(t, filepath.Join(dir, "original")).SHA256
					accepted := treeAt(t, filepath.Join(dir, "revisions")).SHA256
					usedBytes, err := os.ReadFile(usedName)
					if err != nil {
						t.Fatal(err)
					}
					startBytes, err := os.ReadFile(startName)
					if err != nil {
						t.Fatal(err)
					}
					opened, err := Open(dir)
					if opened != nil {
						opened.Close()
					}
					if err == nil {
						t.Fatal("damaged execution accepted")
					}
					if journal != "valid" && journal != "valid-pub-missing" {
						if treeAt(t, dir).SHA256 != before {
							t.Fatal("invalid committed journal changed evidence")
						}
					} else {
						if treeAt(t, filepath.Join(dir, candidate)).SHA256 != s.Tree.SHA256 {
							t.Fatal("committed exact rollback not completed")
						}
						assertEmpty(t, filepath.Join(dir, "journal"))
						assertEmpty(t, filepath.Join(dir, "staging"))
					}
					if treeAt(t, filepath.Join(dir, "tasks/active/checkpoints")).SHA256 != checkpoints || treeAt(t, filepath.Join(dir, "original")).SHA256 != original || treeAt(t, filepath.Join(dir, "revisions")).SHA256 != accepted {
						t.Fatal("recovery changed immutable sources/checkpoints")
					}
					assertBytes(t, usedName, usedBytes)
					assertBytes(t, startName, startBytes)
					assertBytes(t, filepath.Join(dir, "user-file"), []byte("keep"))
					if _, err := os.Stat(filepath.Join(dir, "tasks/active/edit-result.json")); !errors.Is(err, os.ErrNotExist) {
						t.Fatalf("new execution result published: %v", err)
					}
				})
			}
		}
	}
}

func TestRestoreInterruptionRecovery(t *testing.T) {
	for _, step := range []string{"before-journal", "journal", "old-moved", "new-published", "backup-removed", "done"} {
		t.Run(step, func(t *testing.T) {
			w, dir := makeWorkspace(t)
			pub, err := w.NewCandidate()
			if err != nil {
				t.Fatal(err)
			}
			s, err := w.Checkpoint()
			if err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(pub, "user-file"), []byte("uncheckpointed"))
			changed := treeAt(t, pub).SHA256
			put(t, filepath.Join(dir, "user-file"), []byte("not internal staging"))
			// Materialize each actual on-disk interruption boundary. No global
			// failure hooks, sleeps, or mock rename semantics are involved.
			if _, err := copyTree(w.root, checkpointDir(s.ID)+"/pub", restoreNew); err != nil {
				t.Fatal(err)
			}
			if step != "before-journal" {
				if err := writeJSON(w.root, restoreJournal, restoreRecord{1, s.ID, s.Tree.SHA256}); err != nil {
					t.Fatal(err)
				}
				if step != "journal" {
					if err := publish(w.root, candidate, restoreOld); err != nil {
						t.Fatal(err)
					}
					if step != "old-moved" {
						if err := publish(w.root, restoreNew, candidate); err != nil {
							t.Fatal(err)
						}
						if step != "new-published" {
							if err := w.root.RemoveAll(restoreOld); err != nil {
								t.Fatal(err)
							}
							if step == "done" {
								if err := w.root.Remove(restoreJournal); err != nil {
									t.Fatal(err)
								}
							}
						}
					}
				}
			}
			w.Close()
			other, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			want := s.Tree.SHA256
			if step == "before-journal" {
				want = changed
			}
			if treeAt(t, pub).SHA256 != want {
				t.Fatalf("wrong tree after %s recovery", step)
			}
			assertEmpty(t, filepath.Join(dir, "staging"))
			assertEmpty(t, filepath.Join(dir, "journal"))
			assertBytes(t, filepath.Join(dir, "user-file"), []byte("not internal staging"))
			if treeAt(t, filepath.Join(dir, revision)).SHA256 != s.Tree.SHA256 {
				t.Fatal("recovery changed revision")
			}
		})
	}
}

func TestFailurePreservesCandidateCheckpointAndEvidence(t *testing.T) {
	for _, kind := range []string{"checkpoint-corrupt", "candidate-link", "restore-stage-corrupt", "invalid-journal", "metadata-link"} {
		t.Run(kind, func(t *testing.T) {
			w, dir := makeWorkspace(t)
			pub, err := w.NewCandidate()
			if err != nil {
				t.Fatal(err)
			}
			s, err := w.Checkpoint()
			if err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(pub, "user-data"), []byte("keep on failure"))
			switch kind {
			case "checkpoint-corrupt":
				put(t, filepath.Join(dir, checkpointDir(s.ID), "pub/unlisted.bin"), []byte("bad"))
				if err := w.Restore(s.ID); err == nil {
					t.Fatal("corrupt checkpoint restored")
				}
			case "candidate-link":
				outside := filepath.Join(t.TempDir(), "outside")
				put(t, outside, []byte("outside"))
				if err := os.Symlink(outside, filepath.Join(pub, "escape")); err != nil {
					t.Fatal(err)
				}
				if _, err := w.Checkpoint(); err == nil {
					t.Fatal("symlink checkpoint succeeded")
				}
				assertEmpty(t, filepath.Join(dir, "staging"))
				assertBytes(t, outside, []byte("outside"))
			case "restore-stage-corrupt", "invalid-journal":
				if _, err := copyTree(w.root, checkpointDir(s.ID)+"/pub", restoreNew); err != nil {
					t.Fatal(err)
				}
				if err := writeJSON(w.root, restoreJournal, restoreRecord{1, s.ID, s.Tree.SHA256}); err != nil {
					t.Fatal(err)
				}
				if kind == "restore-stage-corrupt" {
					put(t, filepath.Join(dir, restoreNew, "extra"), []byte("corruption"))
				} else {
					put(t, filepath.Join(dir, restoreJournal), []byte("{"))
				}
			case "metadata-link":
				if err := os.Rename(filepath.Join(dir, "staging"), filepath.Join(dir, "saved-staging")); err != nil {
					t.Fatal(err)
				}
				outside := t.TempDir()
				put(t, filepath.Join(outside, "keep"), []byte("untouched"))
				if err := os.Symlink(outside, filepath.Join(dir, "staging")); err != nil {
					t.Fatal(err)
				}
				defer assertBytes(t, filepath.Join(outside, "keep"), []byte("untouched"))
			}
			assertBytes(t, filepath.Join(pub, "user-data"), []byte("keep on failure"))
			w.Close()
			if other, err := Open(dir); err == nil {
				other.Close()
				t.Fatal("unsafe/corrupt workspace reopened")
			}
			assertBytes(t, filepath.Join(pub, "user-data"), []byte("keep on failure"))
			if kind == "restore-stage-corrupt" || kind == "invalid-journal" {
				if _, err := os.Lstat(filepath.Join(dir, restoreJournal)); err != nil {
					t.Fatalf("failed recovery removed evidence: %v", err)
				}
			}
		})
	}
}

func TestCheckpointIDsNeverTraverse(t *testing.T) {
	w, dir := makeWorkspace(t)
	if _, err := w.NewCandidate(); err != nil {
		t.Fatal(err)
	}
	before := treeAt(t, filepath.Join(dir, candidate))
	for _, id := range []string{"", "../initial", "/tmp", "../../../../original", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"} {
		if err := w.Restore(id); err == nil {
			t.Fatalf("unsafe ID accepted: %q", id)
		}
	}
	if treeAt(t, filepath.Join(dir, candidate)).SHA256 != before.SHA256 {
		t.Fatal("invalid ID changed candidate")
	}
	assertEmpty(t, filepath.Join(dir, "staging"))
}
