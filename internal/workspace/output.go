package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// OutputPath requires an absent output outside the entire workspace, with a
// real, already existing parent. Rejecting symlink ancestors also rejects aliases
// into protected trees; same-user concurrent filesystem writers remain excluded.
func (w *Workspace) OutputPath(output string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return "", err
	}
	return w.outputPath(output)
}
func (w *Workspace) outputPath(output string) (string, error) {
	abs, err := filepath.Abs(output)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(w.dir, abs)
	if err != nil {
		return "", err
	}
	if rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("output must be outside workspace root")
	}
	r, err := openDir(filepath.Dir(abs))
	if err != nil {
		return "", err
	}
	defer r.Close()
	if _, err := r.Lstat(filepath.Base(abs)); !os.IsNotExist(err) {
		if err == nil {
			err = os.ErrExist
		}
		return "", err
	}
	return abs, nil
}

// WritePlanReport publishes exactly the verified plan without replacement.
// Internal plan provenance is retained independently of the user's report path.
func (w *Workspace) WritePlanReport(p Plan, output string) (err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return err
	}
	if _, err := w.verifyPlan(p); err != nil {
		return err
	}
	abs, err := w.outputPath(output)
	if err != nil {
		return err
	}
	r, err := openDir(filepath.Dir(abs))
	if err != nil {
		return err
	}
	defer r.Close()
	tmp := ".kepub-plan-" + randomID()
	defer func() { err = errors.Join(err, r.RemoveAll(tmp)) }()
	if err := writeJSON(r, tmp, p); err != nil {
		return err
	}
	if err := publish(r, tmp, filepath.Base(abs)); err != nil {
		return err
	}
	return syncDir(r, ".")
}
