package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

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
