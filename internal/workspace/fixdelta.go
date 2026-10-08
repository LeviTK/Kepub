package workspace

import (
	"fmt"
	"path/filepath"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/fix"
)

// DeltaSnapshot is one verified private snapshot side of a validation delta.
// Revision and task sides carry their own real identity; no preview generation
// or Git identity is invented.
type DeltaSnapshot struct {
	Dir             string
	Archive         *archive.Archive
	Tree            archive.Tree
	Snapshot        fix.Snapshot
	RevisionID      string
	TaskID          string
	ExecutionSHA256 string
	TreeSHA256      string
}

// Close removes the private frozen snapshot of one delta side.
func (ds DeltaSnapshot) Close() {
	if ds.Archive != nil {
		ds.Archive.Close()
	}
}

// freezeDelta captures one verified tree into a private archive so the native
// facts and the checker read the same bytes, never the live path.
func (w *Workspace) freezeDelta(dir string, wantTree, base string) (DeltaSnapshot, error) {
	a, tree, err := archive.SnapshotDirectoryContext(w.resources.ctx, dir, w.resources.limits)
	if err != nil {
		return DeltaSnapshot{}, err
	}
	if tree.SHA256 != wantTree {
		a.Close()
		return DeltaSnapshot{}, ErrStalePlan
	}
	s, err := w.fixSnapshotOfArchive(a, tree, base)
	if err != nil {
		a.Close()
		return DeltaSnapshot{}, err
	}
	return DeltaSnapshot{Dir: dir, Archive: a, Tree: tree, Snapshot: s, TreeSHA256: tree.SHA256}, nil
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
		ds, err := w.freezeDelta(dir, r.Tree.SHA256, revision)
		if err != nil {
			return DeltaSnapshot{}, err
		}
		ds.RevisionID = revision
		return ds, nil
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
	t, err := w.hashAt(candidate)
	if err != nil {
		return DeltaSnapshot{}, err
	}
	ds, err := w.freezeDelta(dir, t.SHA256, e.Plan.BaseRevision)
	if err != nil {
		return DeltaSnapshot{}, err
	}
	ds.TaskID = task
	ds.ExecutionSHA256 = digest(e)
	return ds, nil
}
