package workspace

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
)

type Snapshot struct {
	Version      int    `json:"version"`
	ID           string `json:"id"`
	BaseRevision string `json:"baseRevision"`
	Tree         Tree   `json:"tree"`
}

// Checkpoint captures all candidate bytes, including malformed content and new
// user files. It reports no validation status. Caller must quiesce all writers.
func (w *Workspace) Checkpoint() (_ Snapshot, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.checkpoint()
}

func (w *Workspace) checkpoint() (_ Snapshot, err error) {
	return w.checkpointFrom(candidate)
}

func (w *Workspace) checkpointFrom(source string) (_ Snapshot, err error) {
	if err := w.ready(); err != nil {
		return Snapshot{}, err
	}
	if err := w.verifyTask(); err != nil {
		return Snapshot{}, err
	}
	before, err := hashAt(w.root, source)
	if err != nil {
		return Snapshot{}, err
	}
	id := randomID()
	stage := "staging/" + id
	if err := w.root.Mkdir(stage, 0700); err != nil {
		return Snapshot{}, err
	}
	defer func() { err = errors.Join(err, w.root.RemoveAll(stage)) }()
	tree, err := copyTree(w.root, source, stage+"/pub")
	if err != nil {
		return Snapshot{}, err
	}
	after, err := hashAt(w.root, source)
	if err != nil {
		return Snapshot{}, err
	}
	if tree.SHA256 != before.SHA256 || tree.SHA256 != after.SHA256 {
		return Snapshot{}, fmt.Errorf("candidate changed during checkpoint")
	}
	s := Snapshot{1, id, w.current, tree}
	if err := writeJSON(w.root, stage+"/checkpoint.json", s); err != nil {
		return Snapshot{}, err
	}
	if err := syncDir(w.root, stage); err != nil {
		return Snapshot{}, err
	}
	if err := publish(w.root, stage, checkpointDir(id)); err != nil {
		return Snapshot{}, err
	}
	if err := syncDir(w.root, "tasks/active/checkpoints"); err != nil {
		w.recovery = true
		return Snapshot{}, err
	}
	return s, nil
}

// Checkpoints lists and verifies persisted snapshots, including after reopening.
func (w *Workspace) Checkpoints() ([]Snapshot, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return nil, err
	}
	return w.checkpoints()
}

func (w *Workspace) checkpoints() ([]Snapshot, error) {
	if err := w.verifyTask(); err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(w.root.FS(), "tasks/active/checkpoints")
	if err != nil {
		return nil, err
	}
	out := []Snapshot{}
	for _, e := range entries {
		s, err := w.snapshot(e.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func validID(id string) bool {
	b, err := hex.DecodeString(id)
	return err == nil && len(b) == 16 && hex.EncodeToString(b) == id
}

func checkpointDir(id string) string { return "tasks/active/checkpoints/" + id }

func (w *Workspace) snapshot(id string) (Snapshot, error) {
	var s Snapshot
	if !validID(id) {
		return s, fmt.Errorf("invalid checkpoint ID")
	}
	if err := readJSON(w.root, checkpointDir(id)+"/checkpoint.json", &s); err != nil {
		return s, err
	}
	if s.Version != 1 || s.ID != id || s.BaseRevision != w.current {
		return s, fmt.Errorf("invalid checkpoint provenance")
	}
	tree, err := hashAt(w.root, checkpointDir(id)+"/pub")
	if err != nil {
		return s, err
	}
	if tree.SHA256 != s.Tree.SHA256 || !slices.Equal(tree.Entries, s.Tree.Entries) {
		return s, fmt.Errorf("checkpoint manifest mismatch")
	}
	return s, nil
}

const restoreNew = "staging/restore-new"
const restoreOld = "staging/restore-old"
const restoreJournal = "journal/restore.json"

type restoreRecord struct {
	Version    int    `json:"version"`
	Checkpoint string `json:"checkpoint"`
	SHA256     string `json:"sha256"`
}

// Restore replaces the candidate publication tree, not work/ or the workspace.
// After a persisted journal exists, errors leave recoverable data in place and
// the handle refuses further work until Close/Open. Open rolls this restore
// forward. There is a brief missing-pub interval; readers/writers must be stopped.
func (w *Workspace) Restore(id string) (err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.restore(id)
}

func (w *Workspace) restore(id string) (err error) {
	if err := w.ready(); err != nil {
		return err
	}
	if err := w.verifyTask(); err != nil {
		return err
	}
	s, err := w.snapshot(id)
	if err != nil {
		return err
	}
	// Refuse unknown leftovers rather than overwriting files on a live handle.
	for _, name := range []string{restoreNew, restoreOld, restoreJournal, "staging/restore.json"} {
		if exists(w.root, name) {
			return fmt.Errorf("restore path already exists: %s", name)
		}
	}
	if r, err := subdir(w.root, candidate); err != nil {
		return err
	} else {
		r.Close()
	}
	defer func() {
		if !w.recovery {
			err = errors.Join(err, w.root.RemoveAll(restoreNew), w.root.RemoveAll("staging/restore.json"))
		}
	}()
	tree, err := copyTree(w.root, checkpointDir(id)+"/pub", restoreNew)
	if err != nil {
		return err
	}
	if tree.SHA256 != s.Tree.SHA256 {
		return fmt.Errorf("checkpoint changed during restore copy")
	}
	if err := syncDir(w.root, "staging"); err != nil {
		return err
	}
	if err := writeJSON(w.root, "staging/restore.json", restoreRecord{1, id, tree.SHA256}); err != nil {
		return err
	}
	if err := publish(w.root, "staging/restore.json", restoreJournal); err != nil {
		return err
	}
	w.recovery = true
	if err := syncDir(w.root, "journal"); err != nil {
		return err
	}
	if err := w.recoverRestore(); err != nil {
		return err
	}
	w.recovery = false
	return nil
}

func (w *Workspace) recoverRestore() error {
	if !exists(w.root, restoreJournal) {
		return nil
	}
	if err := w.verifyTask(); err != nil {
		return err
	}
	var j restoreRecord
	if err := readJSON(w.root, restoreJournal, &j); err != nil {
		return err
	}
	if j.Version != 1 {
		return fmt.Errorf("unsupported restore journal")
	}
	s, err := w.snapshot(j.Checkpoint)
	if err != nil {
		return err
	}
	if s.Tree.SHA256 != j.SHA256 {
		return fmt.Errorf("restore checkpoint hash mismatch")
	}
	if exists(w.root, restoreNew) {
		tree, err := hashAt(w.root, restoreNew)
		if err != nil {
			return err
		}
		if tree.SHA256 != j.SHA256 {
			return fmt.Errorf("restore staging hash mismatch")
		}
		if exists(w.root, candidate) {
			if err := publish(w.root, candidate, restoreOld); err != nil {
				return err
			}
		} else if !exists(w.root, restoreOld) {
			return fmt.Errorf("restore lost candidate and backup")
		}
		if err := syncRestoreDirs(w.root); err != nil {
			return err
		}
		if err := publish(w.root, restoreNew, candidate); err != nil {
			return err
		}
	}
	// This also validates the post-publish and post-backup-cleanup crash states.
	tree, err := hashAt(w.root, candidate)
	if err != nil {
		return err
	}
	if tree.SHA256 != j.SHA256 {
		return fmt.Errorf("restored candidate hash mismatch")
	}
	if err := syncRestoreDirs(w.root); err != nil {
		return err
	}
	if err := w.root.RemoveAll(restoreOld); err != nil {
		return err
	}
	if err := syncDir(w.root, "staging"); err != nil {
		return err
	}
	if err := w.root.Remove(restoreJournal); err != nil {
		return err
	}
	return syncDir(w.root, "journal")
}

func syncRestoreDirs(r *os.Root) error {
	return errors.Join(syncDir(r, "staging"), syncDir(r, "tasks/active/work"))
}
