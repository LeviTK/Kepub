package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/fix"
	"github.com/LeviTK/Kepub/internal/publication"
)

// fixRequest is the schema 7 request wire: the complete proposal source plus the
// derived operations. The proposal field is required, so a schema 7 request
// without a proposal is rejected by the strict decoder.
type fixRequest struct {
	SchemaVersion int           `json:"schemaVersion"`
	Proposal      *fix.Proposal `json:"proposal"`
	Operations    []Operation   `json:"operations"`
}

// fixPolicy is the frozen schema 7 policy string.
const fixPolicy = "kepub-fix-proposal-v1:accepted-baseline;multi-operation;frozen-baseline;complete-source;locator-v1;reference-gate;explicit-selection;no-timestamp;review-required;conformance-not-run"

// fixSnapshot builds the frozen proposal source of the current accepted
// revision. It never reads Git metadata and never mutates workspace state.
func (w *Workspace) fixSnapshot() (fix.Snapshot, error) {
	return w.fixSnapshotAt(revisionPath(w.current))
}

// fixSnapshotAt builds the frozen proposal source of one publication tree inside
// the workspace root.
func (w *Workspace) fixSnapshotAt(baseDir string) (fix.Snapshot, error) {
	r, err := subdir(w.root, baseDir)
	if err != nil {
		return fix.Snapshot{}, err
	}
	defer r.Close()
	a := publicationRoot{r}
	pub, err := publication.Load(a, w.state.Rootfile)
	if err != nil {
		return fix.Snapshot{}, err
	}
	s := fix.Snapshot{
		Workspace: fix.WorkspaceRef{
			WorkspaceID:     w.id,
			Rootfile:        w.state.Rootfile,
			BaseRevision:    w.current,
			InputTreeSHA256: w.base.SHA256,
		},
		Version: pub.Version,
	}
	if s.Container, err = a.Read("META-INF/container.xml", publication.XMLLimit); err != nil {
		return fix.Snapshot{}, err
	}
	if s.Package, err = a.Read(bookpath.BookPath(w.state.Rootfile), publication.XMLLimit); err != nil {
		return fix.Snapshot{}, err
	}
	for _, item := range pub.Manifest {
		if item.MediaType != "application/xhtml+xml" || !item.Exists {
			continue
		}
		b, err := a.Read(item.Path, publication.XMLLimit)
		if err != nil {
			return fix.Snapshot{}, err
		}
		s.Resources = append(s.Resources, fix.Resource{Path: item.Path, Bytes: b})
	}
	sort.Slice(s.Resources, func(i, j int) bool { return s.Resources[i].Path < s.Resources[j].Path })
	if t, err := HashTree(filepath.Join(w.dir, filepath.FromSlash(baseDir))); err == nil {
		for _, e := range t.Entries {
			s.Inventory = append(s.Inventory, e.Path)
		}
	}
	sort.Strings(s.Inventory)
	return s, nil
}

// FixSnapshot exposes the frozen proposal source for read-only proposal
// commands. It holds the workspace lock only while reading the baseline.
func (w *Workspace) FixSnapshot() (fix.Snapshot, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return fix.Snapshot{}, err
	}
	if err := w.verifyBaseline(); err != nil {
		return fix.Snapshot{}, err
	}
	if err := w.ensureIdentity(); err != nil {
		return fix.Snapshot{}, err
	}
	return w.fixSnapshot()
}

// WriteDocument publishes one external document without replacement. It is used
// by the read-only document commands and never touches workspace state.
func (w *Workspace) WriteDocument(b []byte, output string) (err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return err
	}
	abs, err := w.outputPath(output)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(abs, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if os.IsExist(err) {
			return fault.New(2, "OUTPUT_EXISTS", "output already exists")
		}
		return err
	}
	published := false
	defer func() {
		if !published {
			f.Close()
			os.Remove(abs)
		}
	}()
	if _, err := f.Write(b); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	published = true
	return nil
}

// validateFixProposal applies the frozen schema 7 priority: self-consistency and
// workspace binding, whole-content re-derivation, the explicit-selection limit
// of the relative URL rule, and finally the operation multiset.
func (w *Workspace) validateFixProposal(p *fix.Proposal) error {
	s, err := w.fixSnapshot()
	if err != nil {
		return err
	}
	if p.Workspace != s.Workspace {
		return fault.New(4, "PROPOSAL_DRIFT", "proposal workspace binding does not match the accepted baseline")
	}
	if err := fix.Validate(s, *p); err != nil {
		return fault.New(4, "PROPOSAL_DRIFT", "%v", err)
	}
	if err := fix.RequestAllowed(*p); err != nil {
		if errors.Is(err, fix.ErrSelection) {
			return fault.New(2, "SELECTION_INVALID", "%v", err)
		}
		return fault.New(2, "INVALID_OPERATIONS", "%v", err)
	}
	return nil
}

// fixOperations converts workspace operations into the proposal wire shape for
// the multiset comparison.
func fixOperations(ops []Operation) []fix.Operation {
	out := make([]fix.Operation, 0, len(ops))
	for _, op := range ops {
		out = append(out, fix.Operation{ID: op.ID, Version: op.Version, Params: op.Params})
	}
	return out
}
