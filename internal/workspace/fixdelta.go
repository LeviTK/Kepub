package workspace

import (
	"fmt"
	"path/filepath"

	"github.com/LeviTK/Kepub/internal/fix"
)

// DeltaSnapshot is one verified private snapshot side of a validation delta.
// Revision and task sides carry their own real identity; no preview generation
// or Git identity is invented.
type DeltaSnapshot struct {
	Dir             string
	Snapshot        fix.Snapshot
	RevisionID      string
	TaskID          string
	ExecutionSHA256 string
	TreeSHA256      string
}

// FixDeltaSnapshot resolves exactly one delta side: a frozen revision of this
// workspace, or the stable candidate of a successful apply task. Drifted,
// failed or unknown identities are refused.
func (w *Workspace) FixDeltaSnapshot(revision, task string) (DeltaSnapshot, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return DeltaSnapshot{}, err
	}
	if err := w.verifyBaseline(); err != nil {
		return DeltaSnapshot{}, err
	}
	if err := w.ensureIdentity(); err != nil {
		return DeltaSnapshot{}, err
	}
	if revision != "" {
		r, err := w.readRevision(revision)
		if err != nil {
			return DeltaSnapshot{}, err
		}
		dir := filepath.Join(w.dir, filepath.FromSlash(revisionPath(revision)))
		t, err := HashTree(dir)
		if err != nil {
			return DeltaSnapshot{}, err
		}
		if t.SHA256 != r.Tree.SHA256 {
			return DeltaSnapshot{}, ErrStalePlan
		}
		s, err := w.fixSnapshotAt(revisionPath(revision))
		if err != nil {
			return DeltaSnapshot{}, err
		}
		s.Workspace.BaseRevision = revision
		return DeltaSnapshot{Dir: dir, Snapshot: s, RevisionID: revision, TreeSHA256: t.SHA256}, nil
	}
	active, err := w.taskID()
	if err != nil {
		return DeltaSnapshot{}, err
	}
	if task != active {
		return DeltaSnapshot{}, fmt.Errorf("task %s is not the active candidate", task)
	}
	e, err := w.execution()
	if err != nil {
		return DeltaSnapshot{}, err
	}
	if e.TaskID != task || e.Status != "review_required" {
		return DeltaSnapshot{}, fmt.Errorf("task %s is not a stable applied candidate", task)
	}
	dir := filepath.Join(w.dir, filepath.FromSlash(candidate))
	t, err := HashTree(dir)
	if err != nil {
		return DeltaSnapshot{}, err
	}
	s, err := w.fixSnapshotAt(candidate)
	if err != nil {
		return DeltaSnapshot{}, err
	}
	return DeltaSnapshot{Dir: dir, Snapshot: s, TaskID: task, ExecutionSHA256: digest(e), TreeSHA256: t.SHA256}, nil
}
