package workspace

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/metadata"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/validation"
)

var ErrTaskConflict = errors.New("explicit task is not the active task")

// Revision is immutable publication provenance. Initial has no Validation;
// acceptance never mutates the import State or confers approval on initial.
type Revision struct {
	Version         int                `json:"version"`
	ID              string             `json:"revisionId"`
	WorkspaceID     string             `json:"workspaceId"`
	Parent          string             `json:"parentRevision"`
	TaskID          string             `json:"taskId"`
	ExecutionSHA256 string             `json:"executionSha256"`
	Rootfile        string             `json:"rootfile"`
	Tree            Tree               `json:"tree"`
	Validation      *validation.Report `json:"validation,omitempty"`
}
type acceptedPointer struct {
	Version     int    `json:"version"`
	WorkspaceID string `json:"workspaceId"`
	RevisionID  string `json:"revisionId"`
}
type Decision struct {
	Version      int                `json:"version"`
	TaskID       string             `json:"taskId"`
	Status       string             `json:"status"`
	BaseRevision string             `json:"baseRevision"`
	RevisionID   string             `json:"revisionId"`
	TreeSHA256   string             `json:"treeSha256"`
	Validation   *validation.Report `json:"validation,omitempty"`
}
type MetadataReview struct {
	Namespace    string  `json:"namespace"`
	LocalName    string  `json:"localName"`
	ID           string  `json:"id,omitempty"`
	OldValue     string  `json:"oldValue"`
	PlannedValue string  `json:"plannedValue"`
	NewValue     *string `json:"newValue"`
	Unavailable  string  `json:"unavailable,omitempty"`
}
type Review struct {
	TaskID           string         `json:"taskId"`
	BaseRevision     string         `json:"baseRevision"`
	Diff             Diff           `json:"diff"`
	MatchesExecution bool           `json:"matchesExecution"`
	Metadata         MetadataReview `json:"metadata"`
	Content          *ContentReview `json:"content,omitempty"`
}

type ContentReview struct {
	BookPath       string  `json:"bookPath"`
	LocatorVersion int     `json:"locatorVersion"`
	Locator        string  `json:"locator"`
	OldValue       string  `json:"oldValue"`
	PlannedValue   string  `json:"plannedValue"`
	NewValue       *string `json:"newValue"`
	Unavailable    string  `json:"unavailable,omitempty"`
}

func revisionPath(id string) string { return "revisions/" + id + "/pub" }

func approval(r validation.Report, treeSHA string) error {
	if r.Draft || r.Status != "pass" || r.InputTreeSHA256 != treeSHA || len(r.ArchiveSHA256) != 64 || len(r.ConfigSHA256) != 64 {
		return fmt.Errorf("complete formal validation bound to the frozen tree is required")
	}
	seen := map[string]bool{}
	for _, c := range r.Checks {
		if seen[c.ID] {
			return fmt.Errorf("duplicate check")
		}
		seen[c.ID] = true
		if c.InputSHA256 != r.ArchiveSHA256 || c.ConfigSHA256 != r.ConfigSHA256 {
			return fmt.Errorf("check evidence mismatch")
		}
		if c.Required && c.Status != "passed" {
			return fmt.Errorf("required check did not pass")
		}
		if c.ID == "archive" || c.ID == "parse.structure" || c.ID == "epubcheck" {
			if !c.Required || c.Status != "passed" {
				return fmt.Errorf("missing formal check")
			}
		}
		if c.ID == "epubcheck" && (c.ToolSHA256 != validation.ToolSHA256 || c.Version != validation.Version) {
			return fmt.Errorf("checker identity mismatch")
		}
	}
	for _, id := range []string{"archive", "parse.structure", "epubcheck"} {
		if !seen[id] {
			return fmt.Errorf("missing check %s", id)
		}
	}
	for _, d := range r.Diagnostics {
		if d.Severity == "error" || d.Severity == "fatal" {
			return fmt.Errorf("blocking validation diagnostic")
		}
	}
	return nil
}

func (w *Workspace) readRevision(id string) (Revision, error) {
	if id == "initial" {
		tree := w.state.Tree
		tree.Entries = slices.Clone(tree.Entries)
		return Revision{Version: 1, ID: id, WorkspaceID: w.id, Rootfile: w.state.Rootfile, Tree: tree}, nil
	}
	seen := map[string]bool{}
	var selected Revision
	expectedParent := ""
	for id != "initial" {
		if !validID(id) || seen[id] {
			return selected, fmt.Errorf("invalid revision chain")
		}
		seen[id] = true
		var r Revision
		if err := readEditJSON(w.root, "revisions/"+id+"/revision.json", &r); err != nil {
			return selected, err
		}
		if r.Version != 1 || r.ID != id || !validID(r.TaskID) || r.Rootfile != w.state.Rootfile || r.WorkspaceID != w.id || r.Validation == nil {
			return selected, fmt.Errorf("invalid revision provenance")
		}
		t, err := hashAt(w.root, revisionPath(id))
		if err != nil {
			return selected, err
		}
		if digest(t) != digest(r.Tree) {
			return selected, fmt.Errorf("revision manifest mismatch")
		}
		if err := approval(*r.Validation, t.SHA256); err != nil {
			return selected, err
		}
		if expectedParent != "" && expectedParent != t.SHA256 {
			return selected, fmt.Errorf("revision parent tree mismatch")
		}
		inputHash, err := w.revisionSource(r)
		if err != nil {
			return selected, err
		}
		expectedParent = inputHash
		if selected.ID == "" {
			selected = r
		}
		id = r.Parent
	}
	if expectedParent != w.state.Tree.SHA256 {
		return selected, fmt.Errorf("revision chain does not reach initial tree")
	}
	return selected, nil
}

func (w *Workspace) revisionSource(r Revision) (string, error) {
	dir := "tasks/" + r.TaskID
	var d Decision
	if exists(w.root, dir+"/decision.json") {
		if err := readEditJSON(w.root, dir+"/decision.json", &d); err != nil {
			return "", err
		}
	} else {
		var j settlement
		if err := readEditJSON(w.root, settlementJournal, &j); err != nil {
			return "", err
		}
		if j.WorkspaceID != w.id || j.Decision.RevisionID != r.ID {
			return "", fmt.Errorf("missing revision source")
		}
		d = j.Decision
		if exists(w.root, "tasks/active") {
			dir = "tasks/active"
		}
	}
	if d.Version != 1 || d.Status != "accepted" || d.TaskID != r.TaskID || d.BaseRevision != r.Parent || d.RevisionID != r.ID || d.TreeSHA256 != r.Tree.SHA256 || digest(d.Validation) != digest(r.Validation) {
		return "", fmt.Errorf("revision decision mismatch")
	}
	j := settlement{Version: 1, WorkspaceID: w.id, Decision: d}
	if err := w.taskDigests(dir, &j); err != nil {
		return "", err
	}
	var e Execution
	if err := readEditJSON(w.root, dir+"/edit-result.json", &e); err != nil {
		return "", err
	}
	var stored Plan
	if err := readEditJSON(w.root, "plans/"+e.Plan.ID+".json", &stored); err != nil {
		return "", err
	}
	var used planUse
	if err := readEditJSON(w.root, "plans/"+e.Plan.ID+".used.json", &used); err != nil {
		return "", err
	}
	if digest(e) != r.ExecutionSHA256 || e.Status != "review_required" || !e.ReviewRequired || e.Conformance != "not_run" || e.TaskID != r.TaskID || e.Diff.AfterSHA256 != r.Tree.SHA256 || digest(stored) != digest(e.Plan) || used.Version != 1 || used.TaskID != r.TaskID || used.PlanSHA256 != digest(e.Plan) {
		return "", fmt.Errorf("revision execution mismatch")
	}
	return e.Plan.InputTreeSHA256, nil
}

func (w *Workspace) loadCurrent() error {
	id := "initial"
	if exists(w.root, "accepted.json") {
		if err := w.ensureIdentity(); err != nil {
			return err
		}
		var p acceptedPointer
		if err := readEditJSON(w.root, "accepted.json", &p); err != nil {
			return err
		}
		if p.Version != 1 || p.WorkspaceID != w.id || (p.RevisionID != "initial" && !validID(p.RevisionID)) {
			return fmt.Errorf("invalid accepted pointer")
		}
		id = p.RevisionID
	} else {
		entries, err := fs.ReadDir(w.root.FS(), "revisions")
		if err != nil {
			return err
		}
		if len(entries) != 1 && !exists(w.root, settlementJournal) {
			return fmt.Errorf("missing accepted pointer")
		}
	}
	if w.current != "" && w.current != id {
		return ErrStalePlan
	}
	r, err := w.readRevision(id)
	if err != nil {
		return err
	}
	w.current, w.base = id, r.Tree
	return nil
}

func (w *Workspace) taskID() (string, error) {
	if err := w.verifyTask(); err != nil {
		return "", err
	}
	var t taskRecord
	if err := readJSON(w.root, "tasks/active/task.json", &t); err != nil {
		return "", err
	}
	if t.Version == 1 {
		return "active", nil
	} // explicit legacy ID, never latest
	return t.ID, nil
}
func (w *Workspace) requireTask(id string) error {
	if err := w.ready(); err != nil {
		return err
	}
	if err := w.verifyBaseline(); err != nil {
		return err
	}
	actual, err := w.taskID()
	if err != nil || actual != id {
		return errors.Join(ErrTaskConflict, err)
	}
	return nil
}

// TaskID returns the persisted explicit ID, not a global/latest-task lookup.
func (w *Workspace) TaskID() (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return "", err
	}
	return w.taskID()
}

// TaskDiff always observes actual candidate bytes; tampering does not prevent
// review/rejection and cannot inherit the execution's review status.
func (w *Workspace) TaskDiff(id string) (Review, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.requireTask(id); err != nil {
		return Review{}, err
	}
	if !exists(w.root, "tasks/active/edit-intent.json") {
		t, err := hashAt(w.root, candidate)
		return Review{TaskID: id, BaseRevision: w.current, Diff: compareTrees(w.base, t), Metadata: MetadataReview{Unavailable: "manual candidate has no registered metadata execution"}}, err
	}
	e, err := w.execution()
	if err != nil && !errors.Is(err, ErrCandidateDrift) {
		return Review{}, err
	}
	t, err2 := hashAt(w.root, candidate)
	if err2 != nil {
		return Review{}, err2
	}
	r := Review{TaskID: id, BaseRevision: w.current, Diff: compareTrees(w.base, t), MatchesExecution: err == nil}
	if param, ok := e.Plan.Operations[0].Params.(publication.TextSet); ok {
		r.Content = &ContentReview{BookPath: string(param.BookPath), LocatorVersion: param.LocatorVersion, Locator: param.Locator, OldValue: param.ExpectedOldValue, PlannedValue: param.NewValue}
	} else {
		param := e.Plan.Operations[0].Params.(metadata.Set)
		r.Metadata = MetadataReview{Namespace: param.Namespace, LocalName: param.LocalName, ID: param.ID, OldValue: param.ExpectedOldValue, PlannedValue: param.NewValue}
	}
	a, frozen, err := archive.SnapshotDirectory(filepath.Join(w.dir, filepath.FromSlash(candidate)), archive.DefaultLimits)
	if err != nil {
		return r, err
	}
	defer a.Close()
	if frozen.SHA256 != t.SHA256 {
		return r, ErrCandidateDrift
	}
	if r.Content != nil {
		param := e.Plan.Operations[0].Params.(publication.TextSet)
		b, err := a.Read(param.BookPath, publication.XMLLimit)
		if err != nil {
			r.Content.Unavailable = err.Error()
			return r, nil
		}
		value, err := publication.ContentText(b, param.Locator)
		if err != nil {
			r.Content.Unavailable = err.Error()
		} else {
			r.Content.NewValue = &value
		}
		return r, nil
	}
	param := e.Plan.Operations[0].Params.(metadata.Set)
	p, err := publication.Load(a, w.state.Rootfile)
	if err != nil {
		r.Metadata.Unavailable = err.Error()
		return r, nil
	}
	for _, m := range p.Metadata {
		if m.Name.Space != param.Namespace || m.Name.Local != param.LocalName {
			continue
		}
		mid := ""
		for _, attr := range m.Attributes {
			if attr.Name.Space == "" && attr.Name.Local == "id" {
				mid = attr.Value
			}
		}
		if param.ID != "" && mid != param.ID {
			continue
		}
		if r.Metadata.NewValue != nil || len(m.Children) != 0 {
			r.Metadata.NewValue = nil
			r.Metadata.Unavailable = "ambiguous or complex candidate metadata"
			return r, nil
		}
		value := m.Text
		r.Metadata.NewValue = &value
	}
	if r.Metadata.NewValue == nil {
		r.Metadata.Unavailable = "selected metadata is absent"
	}
	return r, nil
}

// AcceptedSnapshot freezes only the current accepted tree. Initial is a usable
// baseline, not passed conformance; the export boundary must check the final ZIP.
func (w *Workspace) AcceptedSnapshot() (*archive.Archive, archive.Tree, Revision, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return nil, archive.Tree{}, Revision{}, err
	}
	if err := w.verifyBaseline(); err != nil {
		return nil, archive.Tree{}, Revision{}, err
	}
	r, err := w.readRevision(w.current)
	if err != nil {
		return nil, archive.Tree{}, r, err
	}
	a, t, err := archive.SnapshotDirectory(filepath.Join(w.dir, filepath.FromSlash(revisionPath(w.current))), archive.DefaultLimits)
	if err != nil {
		return nil, t, r, err
	}
	if t.SHA256 != w.base.SHA256 {
		a.Close()
		return nil, t, r, ErrStalePlan
	}
	return a, t, r, nil
}

const settlementJournal = "journal/settlement.json"

type settlement struct {
	Version      int      `json:"version"`
	WorkspaceID  string   `json:"workspaceId"`
	Decision     Decision `json:"decision"`
	TaskSHA256   string   `json:"taskSha256"`
	IntentSHA256 string   `json:"intentSha256"`
	StartSHA256  string   `json:"startSha256"`
	ResultSHA256 string   `json:"resultSha256"`
}

// Accept has no passed flag or checker callback: it runs real validation against
// an independent frozen revision before journaling any accepted-pointer update.
func (w *Workspace) Accept(ctx context.Context, id string, o validation.Options) (d Decision, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.requireTask(id); err != nil {
		return d, err
	}
	if id == "active" {
		return d, fmt.Errorf("legacy task must be rejected and replanned before acceptance")
	}
	if o.Draft || o.Rootfile != "" && o.Rootfile != w.state.Rootfile {
		return d, fmt.Errorf("accept requires formal checks of the selected rootfile")
	}
	e, err := w.execution()
	if err != nil {
		return d, err
	}
	if e.Status != "review_required" {
		return d, fmt.Errorf("only a completed review task can be accepted")
	}
	idRevision := randomID()
	stage := "staging/accept-" + idRevision
	if err := w.root.Mkdir(stage, 0700); err != nil {
		return d, err
	}
	defer func() {
		if !w.recovery {
			err = errors.Join(err, w.root.RemoveAll(stage))
		}
	}()
	t, err := copyTree(w.root, candidate, stage+"/pub")
	if err != nil {
		return d, err
	}
	if t.SHA256 != e.Diff.AfterSHA256 {
		return d, ErrCandidateDrift
	}
	o.Rootfile = w.state.Rootfile
	report, checkErr := validation.Validate(ctx, filepath.Join(w.dir, filepath.FromSlash(stage+"/pub")), o)
	if checkErr == nil {
		checkErr = approval(report, t.SHA256)
	}
	d = Decision{Version: 1, TaskID: id, Status: "checks_failed", BaseRevision: w.current, TreeSHA256: t.SHA256, Validation: &report}
	if checkErr == nil {
		d.Status = "checks_passed"
	}
	if !exists(w.root, "tasks/active/checks") {
		if err := w.root.Mkdir("tasks/active/checks", 0700); err != nil {
			return d, err
		}
	}
	if err := writeJSON(w.root, "tasks/active/checks/"+randomID()+".json", d); err != nil {
		return d, err
	}
	if err := syncDir(w.root, "tasks/active/checks"); err != nil {
		return d, err
	}
	if checkErr != nil {
		return d, checkErr
	}
	if _, err := w.execution(); err != nil {
		return d, err
	}
	again, err := hashAt(w.root, stage+"/pub")
	if err != nil {
		return d, err
	}
	if digest(again) != digest(t) {
		return d, ErrCandidateDrift
	}
	if err := ctx.Err(); err != nil {
		return d, err
	}
	r := Revision{Version: 1, ID: idRevision, WorkspaceID: w.id, Parent: w.current, TaskID: id, ExecutionSHA256: digest(e), Rootfile: w.state.Rootfile, Tree: t, Validation: &report}
	if err := writeJSON(w.root, stage+"/revision.json", r); err != nil {
		return d, err
	}
	if err := syncDir(w.root, stage); err != nil {
		return d, err
	}
	d.Status, d.RevisionID = "accepted", idRevision
	settled, err := w.settle(ctx, d)
	if err != nil {
		if w.recovery {
			settled.Status = "accept_pending"
		} else {
			settled.Status = "checks_passed"
			settled.RevisionID = ""
		}
	}
	return settled, err
}

// Reject preserves all task/checkpoint/check evidence and never removes revisions.
func (w *Workspace) Reject(id string) (Decision, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.requireTask(id); err != nil {
		return Decision{}, err
	}
	if exists(w.root, "tasks/active/edit-intent.json") {
		if _, err := w.execution(); err != nil && !errors.Is(err, ErrCandidateDrift) {
			return Decision{}, err
		}
	} else if _, err := w.checkpoints(); err != nil {
		return Decision{}, err
	}
	t, err := hashAt(w.root, candidate)
	if err != nil {
		return Decision{}, err
	}
	return w.settle(context.Background(), Decision{Version: 1, TaskID: id, Status: "rejected", BaseRevision: w.current, TreeSHA256: t.SHA256})
}

func (w *Workspace) settle(ctx context.Context, d Decision) (decision Decision, err error) {
	decision = d
	prepared := false
	defer func() {
		if prepared && !w.recovery {
			err = errors.Join(err, w.root.RemoveAll("staging/settlement.json"))
		}
	}()
	if err := w.ensureIdentity(); err != nil {
		return d, err
	}
	j := settlement{Version: 1, WorkspaceID: w.id, Decision: d}
	if err := w.taskDigests("tasks/active", &j); err != nil {
		return d, err
	}
	if err := w.verifyBaseline(); err != nil {
		return d, err
	}
	if d.TaskID == "active" && j.IntentSHA256 != "" {
		var p Plan
		if err := readEditJSON(w.root, "tasks/active/edit-intent.json", &p); err != nil {
			return d, err
		}
		name := "plans/" + p.ID + ".used.json"
		if !exists(w.root, name) {
			if err := writeJSON(w.root, name, planUse{1, "active", digest(p)}); err != nil {
				return d, err
			}
			if err := syncDir(w.root, "plans"); err != nil {
				return d, err
			}
		}
	}
	if exists(w.root, settlementJournal) {
		return d, ErrRecovery
	}
	if err := writeJSON(w.root, "staging/settlement.json", j); err != nil {
		return d, err
	}
	prepared = true
	// All expensive provenance/tree scans and record preparation are finished.
	// Publishing the journal commits irrevocable intent; cancellation before this
	// point leaves the baseline unchanged. Afterwards recovery always rolls forward.
	if err := ctx.Err(); err != nil {
		return d, err
	}
	if err := publish(w.root, "staging/settlement.json", settlementJournal); err != nil {
		return d, err
	}
	w.recovery = true
	if err := syncDir(w.root, "journal"); err != nil {
		return d, err
	}
	if err := w.recoverSettlement(); err != nil {
		return d, err
	}
	w.recovery = false
	return d, nil
}
func (w *Workspace) taskDigests(dir string, j *settlement) error {
	var t taskRecord
	var p Plan
	var s, e Execution
	if err := readEditJSON(w.root, dir+"/task.json", &t); err != nil {
		return err
	}
	id := t.ID
	if t.Version == 1 {
		id = "active"
	}
	if (t.Version != 1 && t.Version != 2) || id != j.Decision.TaskID || t.BaseRevision != j.Decision.BaseRevision || (t.Version == 1 && (t.ID != "" || t.BaseRevision != "initial")) || (t.Version == 2 && !validID(t.ID)) {
		return fmt.Errorf("settlement task identity mismatch")
	}
	if !exists(w.root, dir+"/edit-intent.json") {
		if j.Decision.Status != "rejected" || exists(w.root, dir+"/edit-start.json") || exists(w.root, dir+"/edit-result.json") {
			return fmt.Errorf("manual candidate cannot be accepted")
		}
		tree, err := hashAt(w.root, dir+"/work/pub")
		if err != nil {
			return err
		}
		if tree.SHA256 != j.Decision.TreeSHA256 {
			return ErrCandidateDrift
		}
		entries, err := fs.ReadDir(w.root.FS(), dir+"/checkpoints")
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !validID(entry.Name()) {
				return fmt.Errorf("invalid manual checkpoint ID")
			}
			var snap Snapshot
			name := dir + "/checkpoints/" + entry.Name()
			if err := readEditJSON(w.root, name+"/checkpoint.json", &snap); err != nil {
				return err
			}
			actual, err := hashAt(w.root, name+"/pub")
			if err != nil {
				return err
			}
			if snap.Version != 1 || snap.ID != entry.Name() || snap.BaseRevision != t.BaseRevision || digest(actual) != digest(snap.Tree) {
				return fmt.Errorf("manual checkpoint mismatch")
			}
		}
		j.TaskSHA256 = digest(t)
		j.IntentSHA256 = ""
		j.StartSHA256 = ""
		j.ResultSHA256 = ""
		return nil
	}
	for _, item := range []struct {
		name string
		out  any
	}{{"edit-intent.json", &p}, {"edit-start.json", &s}, {"edit-result.json", &e}} {
		if err := readEditJSON(w.root, dir+"/"+item.name, item.out); err != nil {
			return err
		}
	}
	if id != j.Decision.TaskID || t.BaseRevision != j.Decision.BaseRevision || p.BaseRevision != t.BaseRevision || p.WorkspaceID != j.WorkspaceID || p.WorkspacePath != w.dir || digest(s.Plan) != digest(p) || digest(e.Plan) != digest(p) || e.Checkpoint != s.Checkpoint {
		return fmt.Errorf("settlement task provenance mismatch")
	}
	var snap Snapshot
	if !validID(s.Checkpoint) {
		return fmt.Errorf("invalid settlement checkpoint")
	}
	if err := readEditJSON(w.root, dir+"/checkpoints/"+s.Checkpoint+"/checkpoint.json", &snap); err != nil {
		return err
	}
	tree, err := hashAt(w.root, dir+"/checkpoints/"+s.Checkpoint+"/pub")
	if err != nil {
		return err
	}
	if snap.Version != 1 || snap.ID != s.Checkpoint || snap.BaseRevision != t.BaseRevision || digest(snap.Tree) != digest(tree) || tree.SHA256 != p.InputTreeSHA256 {
		return fmt.Errorf("settlement checkpoint mismatch")
	}
	if !validPlanOperation(p) || p.Rootfile != w.state.Rootfile || p.OperationSetSHA256 != digest(p.Operations) || s.Version != p.SchemaVersion || e.Version != s.Version || s.TaskID != e.TaskID || s.TaskID != id && !(id == "active" && s.TaskID == "") || s.Status != "running" || s.ReviewRequired || s.Conformance != "not_run" || digest(s.Diff) != digest(compareTrees(tree, tree)) || e.Conformance != "not_run" || e.Diff.Changes == nil {
		return fmt.Errorf("settlement operation/execution version mismatch")
	}
	out, writes, err := w.recomputeAt(p.Operations, dir+"/checkpoints/"+s.Checkpoint+"/pub", p.BaseRevision)
	if err != nil {
		return err
	}
	if !p.Applicable || !slices.Equal(writes, p.WriteSet) {
		return fmt.Errorf("settlement write set mismatch")
	}
	if e.Status == "review_required" {
		if !e.ReviewRequired || e.Failure != "" || digest(e.Diff) != digest(expectedDiff(tree, writes, out)) {
			return fmt.Errorf("settlement content mismatch")
		}
	} else if e.Status != "failed" || e.ReviewRequired || e.Failure == "" || j.Decision.Status == "accepted" {
		return fmt.Errorf("invalid settlement execution status")
	}
	actual, err := hashAt(w.root, dir+"/work/pub")
	if err != nil {
		return err
	}
	if actual.SHA256 != j.Decision.TreeSHA256 {
		return ErrCandidateDrift
	}
	if j.Decision.Status == "accepted" && (digest(e.Diff) != digest(compareTrees(tree, actual)) || !e.ReviewRequired) {
		return ErrCandidateDrift
	}
	j.TaskSHA256, j.IntentSHA256, j.StartSHA256, j.ResultSHA256 = digest(t), digest(p), digest(s), digest(e)
	return nil
}

// Journal publication is the irreversible decision boundary. accepted.json's
// atomic rename subsequently installs the new visible baseline. Recovery validates
// frozen bytes and evidence across both boundaries, then archives the task.
// No validation is silently rerun, and cancellation cannot revoke durable intent.
func (w *Workspace) recoverSettlement() error {
	if !exists(w.root, settlementJournal) {
		return nil
	}
	if err := w.ensureIdentity(); err != nil {
		return err
	}
	var j settlement
	if err := readEditJSON(w.root, settlementJournal, &j); err != nil {
		return err
	}
	d := j.Decision
	if j.Version != 1 || j.WorkspaceID != w.id || d.Version != 1 || (d.TaskID != "active" && !validID(d.TaskID)) || (d.BaseRevision != "initial" && !validID(d.BaseRevision)) || (d.Status != "accepted" && d.Status != "rejected") {
		return fmt.Errorf("invalid settlement journal")
	}
	dir := "tasks/active"
	archiveDir := "tasks/" + d.TaskID
	if d.TaskID == "active" {
		archiveDir = "tasks/legacy-" + j.TaskSHA256
	}
	if !exists(w.root, dir) {
		dir = archiveDir
	} else if exists(w.root, archiveDir) {
		return ErrTaskConflict
	}
	actual := j
	if err := w.taskDigests(dir, &actual); err != nil {
		return err
	}
	if digest(actual) != digest(j) {
		return fmt.Errorf("settlement records changed")
	}
	if d.Status == "accepted" {
		if !validID(d.RevisionID) || !validID(d.TaskID) || d.Validation == nil {
			return fmt.Errorf("missing accepted evidence")
		}
		if err := approval(*d.Validation, d.TreeSHA256); err != nil {
			return err
		}
		stage := "staging/accept-" + d.RevisionID
		target := "revisions/" + d.RevisionID
		source := target
		if exists(w.root, stage) {
			source = stage
			if exists(w.root, target) {
				return fmt.Errorf("revision publication conflict")
			}
		}
		var r Revision
		if err := readEditJSON(w.root, source+"/revision.json", &r); err != nil {
			return err
		}
		t, err := hashAt(w.root, source+"/pub")
		if err != nil {
			return err
		}
		if r.ID != d.RevisionID || r.Parent != d.BaseRevision || r.TaskID != d.TaskID || r.ExecutionSHA256 != j.ResultSHA256 || r.WorkspaceID != w.id || r.Rootfile != w.state.Rootfile || r.Version != 1 || digest(r.Tree) != digest(t) || t.SHA256 != d.TreeSHA256 || digest(r.Validation) != digest(d.Validation) {
			return fmt.Errorf("frozen revision changed")
		}
		if w.current != d.BaseRevision && w.current != d.RevisionID {
			return ErrStalePlan
		}
		if source == stage {
			if err := publish(w.root, stage, target); err != nil {
				return err
			}
			if err := syncDir(w.root, "revisions"); err != nil {
				return err
			}
			if err := syncDir(w.root, "staging"); err != nil {
				return err
			}
		}
		if w.current != d.RevisionID {
			if exists(w.root, "staging/accepted.json") {
				if err := w.root.Remove("staging/accepted.json"); err != nil {
					return err
				}
			}
			if err := writeJSON(w.root, "staging/accepted.json", acceptedPointer{1, w.id, d.RevisionID}); err != nil {
				return err
			}
			if err := w.root.Rename("staging/accepted.json", "accepted.json"); err != nil {
				return err
			}
			if err := syncDir(w.root, "."); err != nil {
				return err
			}
			w.current, w.base = d.RevisionID, t
		}
	} else if d.RevisionID != "" || d.Validation != nil || w.current != d.BaseRevision {
		return fmt.Errorf("invalid rejection")
	}
	if exists(w.root, dir+"/decision.json") {
		var stored Decision
		if err := readEditJSON(w.root, dir+"/decision.json", &stored); err != nil {
			return err
		}
		if digest(stored) != digest(d) {
			return fmt.Errorf("decision changed")
		}
	} else if err := writeJSON(w.root, dir+"/decision.json", d); err != nil {
		return err
	}
	if err := syncDir(w.root, dir); err != nil {
		return err
	}
	if dir == "tasks/active" {
		if err := publish(w.root, dir, archiveDir); err != nil {
			return err
		}
	}
	if err := syncDir(w.root, "tasks"); err != nil {
		return err
	}
	if err := w.root.Remove(settlementJournal); err != nil {
		return err
	}
	return syncDir(w.root, "journal")
}
