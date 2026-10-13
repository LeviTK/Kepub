package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Public OpenContext covers pre-cancellation; the existing per-open I/O hook
// makes cancellation inside baseline verification deterministic (not a timer).
func TestOpenDurableJournalCancellation(t *testing.T) {
	for _, kind := range []string{"restore", "settlement"} {
		for _, when := range []string{"before-open", "baseline"} {
			t.Run(kind+"/"+when, func(t *testing.T) {
				w, dir := makeWorkspace(t)
				var pub string
				var err error
				if kind == "settlement" {
					w.Close()
					w, dir, _ = legalWorkspace(t, "3.0")
					applyPlan(t, w, fieldPlan(t, w, "title", "title", "Title", "Reviewed"))
					pub, err = w.Candidate()
				} else {
					pub, err = w.NewCandidate()
				}
				if err != nil {
					t.Fatal(err)
				}
				s, err := w.Checkpoint()
				if err != nil {
					t.Fatal(err)
				}
				frozen := n1Tree(t, pub)
				immutable := n1Tree(t, filepath.Join(dir, "revisions"))
				book := readResource(t, filepath.Join(dir, "original/book.epub"))
				journal := restoreJournal
				archiveDir := ""
				if kind == "restore" {
					put(t, filepath.Join(pub, "outside.txt"), []byte("uncheckpointed"))
					if _, err := w.copyTree(checkpointDir(s.ID)+"/pub", restoreNew); err != nil {
						t.Fatal(err)
					}
					if err := writeJSON(w.root, journal, restoreRecord{1, s.ID, s.Tree.SHA256}); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := w.ensureIdentity(); err != nil {
						t.Fatal(err)
					}
					var task taskRecord
					if err := readJSON(w.root, "tasks/active/task.json", &task); err != nil {
						t.Fatal(err)
					}
					j := settlement{Version: 1, WorkspaceID: w.id, Decision: Decision{Version: 1, TaskID: task.ID, Status: "rejected", BaseRevision: "initial", TreeSHA256: s.Tree.SHA256}}
					if err := w.taskDigests("tasks/active", &j); err != nil {
						t.Fatal(err)
					}
					journal = settlementJournal
					archiveDir = "tasks/" + task.ID
					if err := writeJSON(w.root, journal, j); err != nil {
						t.Fatal(err)
					}
					frozen = n1Tree(t, pub)
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				var opened *Workspace
				if when == "before-open" {
					cancel()
					opened, err = OpenContext(ctx, dir)
				} else {
					policy := defaultResourceIO(ctx)
					triggered := false
					policy.onOriginalRead = func() { triggered = true; cancel() }
					opened, err = openWithIO(dir, policy)
					if !triggered {
						t.Fatal("baseline cancellation hook not reached")
					}
				}
				if opened != nil {
					opened.Close()
					t.Fatal("cancelled open returned workspace")
				}
				if !errors.Is(err, context.Canceled) {
					t.Fatal("request cancellation not honored", err)
				}
				if _, err := os.Stat(filepath.Join(dir, journal)); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("durable journal retained", err)
				}
				if archiveDir != "" {
					pub = filepath.Join(dir, archiveDir, "work/pub")
					if _, err := os.Stat(filepath.Join(dir, "tasks/active")); !errors.Is(err, os.ErrNotExist) {
						t.Fatal("settlement not archived", err)
					}
					var d Decision
					r, err := openDir(dir)
					if err != nil {
						t.Fatal(err)
					}
					err = readJSON(r, archiveDir+"/decision.json", &d)
					r.Close()
					if err != nil || d.Status != "rejected" {
						t.Fatal("durable decision missing", d, err)
					}
				}
				if !reflect.DeepEqual(frozen, n1Tree(t, pub)) {
					t.Fatal("recovery bytes differ")
				}
				if !reflect.DeepEqual(immutable, n1Tree(t, filepath.Join(dir, "revisions"))) {
					t.Fatal("accepted changed")
				}
				assertBytes(t, filepath.Join(dir, "original/book.epub"), book)
				retry, err := Open(dir)
				if err != nil {
					t.Fatal("retry/lock release", err)
				}
				defer retry.Close()
			})
		}
	}
}

func TestOpenCancelledJournalControls(t *testing.T) {
	for _, kind := range []string{"none", "none-baseline", "malformed-restore", "false-restore", "malformed-settlement", "false-settlement"} {
		t.Run(kind, func(t *testing.T) {
			w, dir := makeWorkspace(t)
			if _, err := w.NewCandidate(); err != nil {
				t.Fatal(err)
			}
			s, err := w.Checkpoint()
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "malformed-restore":
				put(t, filepath.Join(dir, restoreJournal), []byte("{"))
			case "false-restore":
				if err := writeJSON(w.root, restoreJournal, restoreRecord{1, s.ID, "false"}); err != nil {
					t.Fatal(err)
				}
			case "malformed-settlement":
				put(t, filepath.Join(dir, settlementJournal), []byte("{"))
			case "false-settlement":
				if err := writeJSON(w.root, settlementJournal, settlement{Version: 1, WorkspaceID: "false"}); err != nil {
					t.Fatal(err)
				}
			}
			w.Close()
			before := n1Tree(t, dir)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var opened *Workspace
			if kind == "none-baseline" {
				policy := defaultResourceIO(ctx)
				triggered := false
				policy.onOriginalRead = func() { triggered = true; cancel() }
				opened, err = openWithIO(dir, policy)
				if !triggered {
					t.Fatal("baseline cancellation hook not reached")
				}
			} else {
				cancel()
				opened, err = OpenContext(ctx, dir)
			}
			if opened != nil {
				opened.Close()
				t.Fatal("cancelled/invalid open succeeded")
			}
			if err == nil || (kind == "none" || kind == "none-baseline") != errors.Is(err, context.Canceled) {
				t.Fatal("wrong cancellation/validation classification", err)
			}
			if !reflect.DeepEqual(before, n1Tree(t, dir)) {
				t.Fatal("refusal changed evidence")
			}
		})
	}
}
