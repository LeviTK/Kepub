// Package workspace provides owned snapshots, bounded deterministic candidate
// editing and audited acceptance using real formal validation. One handle owns
// the writer lock until Close. External writers must stop before all operations;
// flock is advisory, not a
// sandbox against another process with the same user's filesystem permissions.
package workspace

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/publication"
)

var (
	ErrBusy     = errors.New("workspace already has an owner")
	ErrClosed   = errors.New("workspace is closed")
	ErrReadOnly = errors.New("workspace input is restricted to read-only")
	ErrRecovery = errors.New("workspace requires reopening for recovery")
)

const revision = "revisions/initial/pub"
const candidate = "tasks/active/work/pub"

type Options struct {
	Rootfile string
}

// State is import provenance, not an accepted/validated publication status.
// InitialRevision names an immutable library snapshot, not an approval.
type State struct {
	Version         int      `json:"version"`
	InitialRevision string   `json:"initialRevision"`
	OriginalSHA256  string   `json:"originalSHA256"`
	Rootfile        string   `json:"rootfile"`
	Tree            Tree     `json:"tree"`
	ReadOnlyReasons []string `json:"readOnlyReasons"`
}

type Workspace struct {
	mu       sync.Mutex
	root     *os.Root
	owner    *os.File
	dir      string
	state    State
	current  string
	base     Tree
	id       string
	closed   bool
	recovery bool
}

type taskRecord struct {
	Version      int    `json:"version"`
	BaseRevision string `json:"baseRevision"`
	ID           string `json:"id,omitempty"`
}

// Create requires an absent destination and an existing real parent directory.
// It snapshots the source archive first, parses that private copy through M1-A,
// then publishes the entire workspace with an atomic, no-replace rename. Existing
// files (including empty directories and symlinks) are never replaced.
func Create(dir, source string, opts Options) (_ *Workspace, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	parent, err := openDir(filepath.Dir(abs))
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	if _, err := parent.Lstat(filepath.Base(abs)); err == nil {
		return nil, os.ErrExist
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	stage := ".kepub-create-" + randomID()
	if err := parent.Mkdir(stage, 0700); err != nil {
		return nil, err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, parent.RemoveAll(stage))
		}
	}()
	r, err := parent.OpenRoot(stage)
	if err != nil {
		return nil, err
	}
	w := &Workspace{root: r, dir: abs}
	defer func() {
		if err != nil {
			w.Close()
		}
	}()
	w.owner, err = lock(r)
	if err != nil {
		return nil, err
	}
	for _, d := range []string{"original", "revisions", "revisions/initial", "tasks", "staging", "journal"} {
		if err := r.Mkdir(d, 0700); err != nil {
			return nil, err
		}
	}
	originalHash, err := copyOriginal(r, source)
	if err != nil {
		return nil, err
	}
	stagePath := filepath.Join(filepath.Dir(abs), stage)
	a, err := archive.Open(filepath.Join(stagePath, "original/book.epub"), archive.DefaultLimits)
	if err != nil {
		return nil, err
	}
	defer a.Close()
	p, err := publication.Load(a, opts.Rootfile)
	if err != nil {
		return nil, err
	}
	if err := a.Unpack(filepath.Join(stagePath, filepath.FromSlash(revision))); err != nil {
		return nil, err
	}
	tree, err := hashAt(r, revision)
	if err != nil {
		return nil, err
	}
	reasons := []string{}
	for _, l := range p.Limitations {
		if l.Code != "READ_ONLY_PARTIAL" {
			reasons = append(reasons, l.Code)
		}
	}
	// Preserve hostile-looking resources in original/revision without ever
	// putting them into an executable Agent working directory in this tranche.
	for _, e := range tree.Entries {
		for _, component := range strings.Split(e.Path, "/") {
			switch strings.ToLower(component) {
			case "agents.md", "claude.md", ".amp", ".claude", ".mcp.json", "mcp.json", ".vscode", ".git":
				reasons = append(reasons, "UNTRUSTED_AGENT_CONFIG:"+e.Path)
			}
		}
	}
	slices.Sort(reasons)
	w.state = State{1, "initial", originalHash, string(p.Rootfile), tree, slices.Compact(reasons)}
	if err := writeJSON(r, "state.json", w.state); err != nil {
		return nil, err
	}
	if err := w.ensureIdentity(); err != nil {
		return nil, err
	}
	// Unpack does not promise fsync; sync the complete initial snapshot before
	// publishing the root. This is not a claim of power-loss durability.
	if err := syncTree(r); err != nil {
		return nil, err
	}
	if err := publish(parent, stage, filepath.Base(abs)); err != nil {
		return nil, err
	}
	published = true
	if err := syncDir(parent, "."); err != nil {
		// The destination is complete but publication durability is uncertain.
		// Do not remove it or retry blindly; Open can verify and acquire it.
		return nil, err
	}
	return w, nil
}

// Open exclusively owns and verifies an existing workspace. It never reparses
// the source path or silently imports changed baseline bytes. It completes an
// interrupted restore and removes only the reserved internal staging contents.
// Unrecognized user files elsewhere are left alone.
func Open(dir string) (_ *Workspace, err error) {
	r, err := openDir(dir)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		r.Close()
		return nil, err
	}
	w := &Workspace{root: r, dir: abs}
	defer func() {
		if err != nil {
			w.Close()
		}
	}()
	// Check provenance exists before creating a lock in an unrelated directory.
	if err := readJSON(r, "state.json", &w.state); err != nil {
		return nil, err
	}
	w.owner, err = lock(r)
	if err != nil {
		return nil, err
	}
	if w.state.Version != 1 || w.state.InitialRevision != "initial" {
		return nil, fmt.Errorf("unsupported workspace state")
	}
	for _, d := range []string{"original", "revisions/initial", "tasks", "staging", "journal"} {
		rd, err := subdir(r, d)
		if err != nil {
			return nil, err
		}
		rd.Close()
	}
	if err := w.verifyBaseline(); err != nil {
		return nil, err
	}
	if exists(r, "identity.json") {
		if err := w.ensureIdentity(); err != nil {
			return nil, err
		}
	}
	if err := w.recoverSettlement(); err != nil {
		return nil, err
	}
	if exists(r, "tasks/active") {
		if _, err := w.checkpoints(); err != nil {
			return nil, err
		}
	}
	if err := w.recoverRestore(); err != nil {
		return nil, err
	}
	if exists(r, "tasks/active") {
		// Malformed XHTML is permitted; filesystem escapes are not. No parsing
		// success/validation label is attached to an externally edited candidate.
		if _, err := hashAt(r, candidate); err != nil {
			return nil, err
		}
		if exists(r, "tasks/active/edit-intent.json") {
			if _, err := w.execution(); err != nil && !errors.Is(err, ErrCandidateDrift) {
				return nil, err
			}
		} else if exists(r, "tasks/active/edit-start.json") || exists(r, "tasks/active/edit-result.json") {
			return nil, fmt.Errorf("execution records without intent")
		}
	}
	if err := clearStaging(r); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Workspace) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	var err error
	if w.owner != nil {
		err = w.owner.Close() // Kernel releases flock, also on process termination.
	}
	return errors.Join(err, w.root.Close())
}

func (w *Workspace) State() State {
	w.mu.Lock()
	defer w.mu.Unlock()
	s := w.state
	s.ReadOnlyReasons = slices.Clone(s.ReadOnlyReasons)
	s.Tree.Entries = slices.Clone(s.Tree.Entries)
	return s
}

func (w *Workspace) ready() error {
	if w.closed {
		return ErrClosed
	}
	if w.recovery {
		return ErrRecovery
	}
	return nil
}

// NewCandidate publishes one independent writable copy. A second candidate is
// refused until the active task is settled.
func (w *Workspace) NewCandidate() (_ string, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.createCandidate(nil)
}

func (w *Workspace) createCandidate(plan *Plan) (_ string, err error) {
	if err := w.ready(); err != nil {
		return "", err
	}
	if len(w.state.ReadOnlyReasons) != 0 {
		return "", ErrReadOnly
	}
	if err := w.verifyBaseline(); err != nil {
		return "", err
	}
	stage := "staging/" + randomID()
	if err := w.root.Mkdir(stage, 0700); err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, w.root.RemoveAll(stage)) }()
	for _, d := range []string{"work", "checkpoints"} {
		if err := w.root.Mkdir(stage+"/"+d, 0700); err != nil {
			return "", err
		}
	}
	tree, err := copyTree(w.root, revisionPath(w.current), stage+"/work/pub")
	if err != nil {
		return "", err
	}
	if tree.SHA256 != w.base.SHA256 {
		return "", fmt.Errorf("baseline changed during copy")
	}
	task := taskRecord{Version: 2, BaseRevision: w.current, ID: randomID()}
	if err := writeJSON(w.root, stage+"/task.json", task); err != nil {
		return "", err
	}
	if plan != nil {
		if err := writeJSON(w.root, stage+"/edit-intent.json", plan); err != nil {
			return "", err
		}
	}
	if err := syncDir(w.root, stage+"/work"); err != nil {
		return "", err
	}
	if err := syncDir(w.root, stage); err != nil {
		return "", err
	}
	if plan != nil {
		if err := writeJSON(w.root, "plans/"+plan.ID+".used.json", planUse{1, task.ID, digest(plan)}); err != nil {
			return "", err
		}
		if err := syncDir(w.root, "plans"); err != nil {
			return "", err
		}
	}
	if err := publish(w.root, stage, "tasks/active"); err != nil {
		return "", err
	}
	if err := syncDir(w.root, "tasks"); err != nil {
		w.recovery = true
		return "", err
	}
	return filepath.Join(w.dir, filepath.FromSlash(candidate)), nil
}

// Candidate returns the active publication directory; it does not create one.
func (w *Workspace) Candidate() (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return "", err
	}
	if err := w.verifyTask(); err != nil {
		return "", err
	}
	r, err := subdir(w.root, candidate)
	if err != nil {
		return "", err
	}
	r.Close()
	return filepath.Join(w.dir, filepath.FromSlash(candidate)), nil
}

func (w *Workspace) verifyTask() error {
	var t taskRecord
	if err := readJSON(w.root, "tasks/active/task.json", &t); err != nil {
		return err
	}
	if (t.Version != 1 && t.Version != 2) || t.BaseRevision != w.current || (t.Version == 1 && (t.ID != "" || t.BaseRevision != "initial")) || (t.Version == 2 && !validID(t.ID)) || len(w.state.ReadOnlyReasons) != 0 {
		return fmt.Errorf("invalid candidate provenance")
	}
	r, err := subdir(w.root, "tasks/active/checkpoints")
	if err == nil {
		r.Close()
	}
	return err
}

func (w *Workspace) verifyBaseline() error {
	f, err := openRegular(w.root, "original/book.epub")
	if err != nil {
		return err
	}
	h := sha256.New()
	_, err = io.Copy(h, f)
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != w.state.OriginalSHA256 {
		return fmt.Errorf("original archive hash mismatch")
	}
	tree, err := hashAt(w.root, revision)
	if err != nil {
		return err
	}
	if tree.SHA256 != w.state.Tree.SHA256 || !slices.Equal(tree.Entries, w.state.Tree.Entries) {
		return fmt.Errorf("initial revision manifest mismatch")
	}
	return w.loadCurrent()
}

func copyOriginal(dst *os.Root, source string) (string, error) {
	abs, err := filepath.Abs(source)
	if err != nil {
		return "", err
	}
	src, err := openDir(filepath.Dir(abs))
	if err != nil {
		return "", err
	}
	defer src.Close()
	f, err := openRegular(src, filepath.Base(abs))
	if err != nil {
		return "", err
	}
	defer f.Close()
	out, err := dst.OpenFile("original/book.epub", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(out, h), f)
	err = errors.Join(err, out.Sync(), out.Close())
	return hex.EncodeToString(h.Sum(nil)), err
}

func randomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read either succeeds or terminates.
	return hex.EncodeToString(b[:])
}

func writeJSON(r *os.Root, name string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	f, err := r.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return errors.Join(err, f.Sync(), f.Close())
}

func readJSON(r *os.Root, name string, value any) error {
	p, err := subdir(r, path.Dir(name))
	if err != nil {
		return err
	}
	defer p.Close()
	f, err := openRegular(p, path.Base(name))
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() > 32<<20 {
		return fmt.Errorf("workspace JSON exceeds 32 MiB")
	}
	d := json.NewDecoder(io.LimitReader(f, 32<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("trailing workspace JSON: %v", err)
	}
	return nil
}

func exists(r *os.Root, name string) bool {
	_, err := r.Lstat(name)
	// Access errors are handled by the subsequent operation, not as absence.
	return !os.IsNotExist(err)
}

func clearStaging(r *os.Root) error {
	f, err := r.Open("staging")
	if err != nil {
		return err
	}
	names, err := f.Readdirnames(-1)
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := r.RemoveAll("staging/" + name); err != nil {
			return err
		}
	}
	return syncDir(r, "staging")
}

func syncTree(r *os.Root) error {
	tree, err := scanTree(r, nil)
	if err != nil {
		return err
	}
	for i := len(tree.Entries) - 1; i >= 0; i-- {
		e := tree.Entries[i]
		f, err := r.Open(e.Path)
		if err != nil {
			return err
		}
		if err := errors.Join(f.Sync(), f.Close()); err != nil {
			return err
		}
	}
	return syncDir(r, ".")
}
