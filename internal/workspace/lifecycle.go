package workspace

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/metadata"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/validation"
	"github.com/LeviTK/Kepub/internal/xmltext"
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
	TaskID           string            `json:"taskId"`
	BaseRevision     string            `json:"baseRevision"`
	Diff             Diff              `json:"diff"`
	MatchesExecution bool              `json:"matchesExecution"`
	Metadata         MetadataReview    `json:"metadata"`
	Content          *ContentReview    `json:"content,omitempty"`
	Operations       []OperationReview `json:"operations,omitempty"`
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

// OperationReview reports one version 3/4 operation's planned target and the
// actual candidate value observed for it, in operation order. It is additive:
// schema 1/2 reviews keep their existing metadata/content fields.
type OperationReview struct {
	Index            int     `json:"index"`
	OperationID      string  `json:"operationId"`
	OperationVersion int     `json:"operationVersion"`
	BookPath         string  `json:"bookPath,omitempty"`
	LocatorVersion   int     `json:"locatorVersion,omitempty"`
	Locator          string  `json:"locator,omitempty"`
	Namespace        string  `json:"namespace,omitempty"`
	LocalName        string  `json:"localName,omitempty"`
	ID               string  `json:"id,omitempty"`
	OldValue         string  `json:"oldValue"`
	PlannedValue     string  `json:"plannedValue"`
	NewValue         *string `json:"newValue"`
	Unavailable      string  `json:"unavailable,omitempty"`

	Attribute *AttributeReview `json:"attribute,omitempty"`
	Element   *ElementReview   `json:"element,omitempty"`
}

// AttributeReview reports one attribute operation's actual candidate state.
type AttributeReview struct {
	Namespace    string  `json:"namespace,omitempty"`
	Name         string  `json:"name"`
	OldValue     *string `json:"oldValue,omitempty"`
	PlannedValue *string `json:"plannedValue,omitempty"`
	NewValue     *string `json:"newValue"`
	Unavailable  string  `json:"unavailable,omitempty"`
}

// ElementReview reports one element operation's planned shape and what could be
// observed in the candidate. The file diff remains the authoritative view.
type ElementReview struct {
	Action      string `json:"action"`
	Anchor      string `json:"anchor,omitempty"`
	Position    string `json:"position,omitempty"`
	Candidate   string `json:"candidate,omitempty"`
	Unavailable string `json:"unavailable,omitempty"`
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

type TaskStatus struct {
	WorkspaceID      string     `json:"workspaceId"`
	TaskID           string     `json:"taskId"`
	BaseRevision     string     `json:"baseRevision"`
	CurrentRevision  string     `json:"currentRevision"`
	Status           string     `json:"status"`
	ExecutionStatus  string     `json:"executionStatus,omitempty"`
	ReviewRequired   bool       `json:"reviewRequired"`
	MatchesExecution bool       `json:"matchesExecution"`
	Decision         *Decision  `json:"decision,omitempty"`
	Checks           []Decision `json:"checks"`
}

// Status is exact-ID and source-verified, including settled historical tasks.
// It cannot confer approval or substitute another (or the latest) task.
func (w *Workspace) TaskStatus(id string) (TaskStatus, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return TaskStatus{}, err
	}
	if err := w.verifyBaseline(); err != nil {
		return TaskStatus{}, err
	}
	r := TaskStatus{WorkspaceID: w.id, TaskID: id, CurrentRevision: w.current, Checks: []Decision{}}
	if exists(w.root, "tasks/active") {
		active, err := w.taskID()
		if err != nil {
			return TaskStatus{}, err
		}
		if id == active {
			r.BaseRevision = w.current
			r.Status = "pending"
			if exists(w.root, "tasks/active/edit-intent.json") {
				e, err := w.execution()
				if err != nil && !errors.Is(err, ErrCandidateDrift) {
					return TaskStatus{}, err
				}
				r.Status = e.Status
				r.ExecutionStatus = e.Status
				r.MatchesExecution = err == nil
				r.ReviewRequired = e.ReviewRequired && err == nil
				if err != nil {
					r.Status = "candidate_drift"
				}
			}
			r.Checks, err = w.taskChecks("tasks/active", id, r.BaseRevision)
			return r, err
		}
	}
	if !validID(id) || !exists(w.root, "tasks/"+id) {
		return TaskStatus{}, ErrTaskConflict
	}
	dir := "tasks/" + id
	var d Decision
	if err := readEditJSON(w.root, dir+"/decision.json", &d); err != nil {
		return TaskStatus{}, err
	}
	if d.Version != 1 || d.TaskID != id || (d.Status != "accepted" && d.Status != "rejected") {
		return TaskStatus{}, fmt.Errorf("invalid historical decision")
	}
	if _, err := w.readRevision(d.BaseRevision); err != nil {
		return TaskStatus{}, err
	}
	if d.Status == "accepted" {
		rev, err := w.readRevision(d.RevisionID)
		if err != nil {
			return TaskStatus{}, err
		}
		if rev.TaskID != id {
			return TaskStatus{}, fmt.Errorf("historical task/revision mismatch")
		}
	} else if d.RevisionID != "" || d.Validation != nil {
		return TaskStatus{}, fmt.Errorf("invalid rejection")
	}
	j := settlement{Version: 1, WorkspaceID: w.id, Decision: d}
	if err := w.taskDigests(dir, &j); err != nil {
		return TaskStatus{}, err
	}
	r.BaseRevision = d.BaseRevision
	r.Status = d.Status
	r.Decision = &d
	if exists(w.root, dir+"/edit-result.json") {
		var e Execution
		if err := readEditJSON(w.root, dir+"/edit-result.json", &e); err != nil {
			return TaskStatus{}, err
		}
		var stored Plan
		var used planUse
		if err := readEditJSON(w.root, "plans/"+e.Plan.ID+".json", &stored); err != nil {
			return TaskStatus{}, err
		}
		if err := readEditJSON(w.root, "plans/"+e.Plan.ID+".used.json", &used); err != nil {
			return TaskStatus{}, err
		}
		if digest(stored) != digest(e.Plan) || used.Version != 1 || used.TaskID != id || used.PlanSHA256 != digest(e.Plan) {
			return TaskStatus{}, fmt.Errorf("historical plan consumption mismatch")
		}
		r.ExecutionStatus = e.Status
	}
	checks, err := w.taskChecks(dir, id, r.BaseRevision)
	r.Checks = checks
	return r, err
}

// Attempts are reported individually, not called latest or mistaken for a
// settled decision. Random audit filenames do not provide chronological order.
func (w *Workspace) taskChecks(dir, id, base string) ([]Decision, error) {
	out := []Decision{}
	if !exists(w.root, dir+"/checks") {
		return out, nil
	}
	entries, err := fs.ReadDir(w.root.FS(), dir+"/checks")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if len(name) != 37 || name[32:] != ".json" || !validID(name[:32]) {
			return nil, fmt.Errorf("invalid check audit filename")
		}
		var d Decision
		if err := readEditJSON(w.root, dir+"/checks/"+name, &d); err != nil {
			return nil, err
		}
		if d.Version != 1 || d.TaskID != id || d.BaseRevision != base || d.RevisionID != "" || (d.Status != "checks_passed" && d.Status != "checks_failed") || d.Validation == nil || d.Validation.InputTreeSHA256 != d.TreeSHA256 {
			return nil, fmt.Errorf("invalid check audit provenance")
		}
		out = append(out, d)
	}
	return out, nil
}

// TaskDiff always observes actual candidate bytes; tampering does not prevent
// review/rejection and cannot inherit the execution's review status.
func (w *Workspace) TaskDiff(id string) (Review, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.taskDiff(id)
}

func (w *Workspace) taskDiff(id string) (Review, error) {
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
	if e.Plan.SchemaVersion >= 3 {
		r.Operations = plannedReviews(e.Plan.Operations)
	} else if param, ok := e.Plan.Operations[0].Params.(publication.TextSet); ok {
		r.Content = &ContentReview{BookPath: string(param.BookPath), LocatorVersion: param.LocatorVersion, Locator: param.Locator, OldValue: param.ExpectedOldValue, PlannedValue: param.NewValue}
	} else {
		param := e.Plan.Operations[0].Params.(metadata.Set)
		r.Metadata = MetadataReview{Namespace: param.Namespace, LocalName: param.LocalName, ID: param.ID, OldValue: param.ExpectedOldValue, PlannedValue: param.NewValue}
	}
	a, frozen, err := archive.SnapshotDirectory(filepath.Join(w.dir, filepath.FromSlash(candidate)), archive.DefaultLimits)
	if err != nil {
		unavailableReview(&r, err.Error())
		return r, nil
	}
	defer a.Close()
	if frozen.SHA256 != t.SHA256 {
		return r, ErrCandidateDrift
	}
	p, err := publication.Load(a, w.state.Rootfile)
	if err != nil {
		unavailableReview(&r, err.Error())
		return r, nil
	}
	if r.Operations != nil {
		var base publicationRoot
		if br, err := subdir(w.root, revisionPath(w.current)); err == nil {
			base = publicationRoot{br}
			defer br.Close()
		}
		cands := map[string][]byte{}
		bases := map[string][]byte{}
		docs := map[string]*publication.StructureDocument{}
		var planned map[string][]*publication.StructureEdit
		if e.Plan.SchemaVersion == 4 {
			if d, derr := w.recomputeAt(e.Plan.Operations, revisionPath(w.current), w.current); derr == nil {
				planned = d.edits
			}
		}
		for i, op := range e.Plan.Operations {
			switch param := op.Params.(type) {
			case publication.TextSet:
				value, err := candidateContentValue(a, p, param)
				if err != nil {
					r.Operations[i].Unavailable = err.Error()
				} else {
					r.Operations[i].NewValue = &value
				}
			case metadata.Set:
				value, err := candidateMetadataValue(p, param)
				if err != nil {
					r.Operations[i].Unavailable = err.Error()
				} else {
					r.Operations[i].NewValue = &value
				}
			case publication.AttributeSet:
				value, err := candidateAttribute(a, p, cands, base, bases, docs, planned, param.BookPath, param.Locator, xml.Name{Space: param.Namespace, Local: param.Name})
				if err != nil {
					r.Operations[i].Unavailable = err.Error()
					r.Operations[i].Attribute.Unavailable = err.Error()
				} else {
					r.Operations[i].NewValue = value
					r.Operations[i].Attribute.NewValue = value
				}
			case publication.AttributeRemove:
				value, err := candidateAttribute(a, p, cands, base, bases, docs, planned, param.BookPath, param.Locator, xml.Name{Space: param.Namespace, Local: param.Name})
				if err != nil {
					r.Operations[i].Unavailable = err.Error()
					r.Operations[i].Attribute.Unavailable = err.Error()
				} else {
					r.Operations[i].NewValue = value
					r.Operations[i].Attribute.NewValue = value
				}
			case publication.ElementDelete:
				value, err := candidateElementEffect(a, p, base, bases, cands, docs, param.BookPath, op)
				if err != nil {
					r.Operations[i].Unavailable = err.Error()
					r.Operations[i].Element.Unavailable = err.Error()
				} else {
					r.Operations[i].Element.Candidate = value
				}
			case publication.ElementInsert:
				value, err := candidateElementEffect(a, p, base, bases, cands, docs, param.BookPath, op)
				if err != nil {
					r.Operations[i].Unavailable = err.Error()
					r.Operations[i].Element.Unavailable = err.Error()
				} else {
					r.Operations[i].Element.Candidate = value
				}
			case publication.ElementReplace:
				value, err := candidateElementEffect(a, p, base, bases, cands, docs, param.BookPath, op)
				if err != nil {
					r.Operations[i].Unavailable = err.Error()
					r.Operations[i].Element.Unavailable = err.Error()
				} else {
					r.Operations[i].Element.Candidate = value
				}
			case publication.ElementMove:
				value, err := candidateElementEffect(a, p, base, bases, cands, docs, param.BookPath, op)
				if err != nil {
					r.Operations[i].Unavailable = err.Error()
					r.Operations[i].Element.Unavailable = err.Error()
				} else {
					r.Operations[i].Element.Candidate = value
				}
			}
		}
		return r, nil
	}
	if r.Content != nil {
		param := e.Plan.Operations[0].Params.(publication.TextSet)
		value, err := candidateContentValue(a, p, param)
		if err != nil {
			r.Content.Unavailable = err.Error()
		} else {
			r.Content.NewValue = &value
		}
		return r, nil
	}
	param := e.Plan.Operations[0].Params.(metadata.Set)
	value, err := candidateMetadataValue(p, param)
	if err != nil {
		r.Metadata.Unavailable = err.Error()
	} else {
		r.Metadata.NewValue = &value
	}
	return r, nil
}

// plannedReviews lists every version 3/4 operation's planned target in order.
// It reports no actual candidate value; callers fill those from real bytes.
func plannedReviews(ops []Operation) []OperationReview {
	out := make([]OperationReview, 0, len(ops))
	for i, op := range ops {
		or := OperationReview{Index: i, OperationID: op.ID, OperationVersion: op.Version}
		switch param := op.Params.(type) {
		case publication.TextSet:
			or.BookPath, or.LocatorVersion, or.Locator = string(param.BookPath), param.LocatorVersion, param.Locator
			or.OldValue, or.PlannedValue = param.ExpectedOldValue, param.NewValue
		case metadata.Set:
			or.Namespace, or.LocalName, or.ID = param.Namespace, param.LocalName, param.ID
			or.OldValue, or.PlannedValue = param.ExpectedOldValue, param.NewValue
		case publication.AttributeSet:
			or.BookPath, or.LocatorVersion, or.Locator = string(param.BookPath), param.LocatorVersion, param.Locator
			or.OldValue, or.PlannedValue = derefString(param.ExpectedOldValue), param.Value
			or.Attribute = &AttributeReview{Namespace: param.Namespace, Name: param.Name, OldValue: param.ExpectedOldValue, PlannedValue: &param.Value}
		case publication.AttributeRemove:
			or.BookPath, or.LocatorVersion, or.Locator = string(param.BookPath), param.LocatorVersion, param.Locator
			or.OldValue = param.ExpectedOldValue
			or.Attribute = &AttributeReview{Namespace: param.Namespace, Name: param.Name, OldValue: &param.ExpectedOldValue}
		case publication.ElementDelete:
			or.BookPath, or.LocatorVersion, or.Locator = string(param.BookPath), param.LocatorVersion, param.Locator
			or.Element = &ElementReview{Action: "delete"}
		case publication.ElementInsert:
			or.BookPath, or.LocatorVersion, or.Locator = string(param.BookPath), param.LocatorVersion, param.Locator
			or.Element = &ElementReview{Action: "insert", Anchor: param.Locator, Position: param.Position}
		case publication.ElementReplace:
			or.BookPath, or.LocatorVersion, or.Locator = string(param.BookPath), param.LocatorVersion, param.Locator
			or.Element = &ElementReview{Action: "replace"}
		case publication.ElementMove:
			or.BookPath, or.LocatorVersion, or.Locator = string(param.BookPath), param.LocatorVersion, param.Locator
			or.Element = &ElementReview{Action: "move", Anchor: param.Anchor, Position: param.Position}
		}
		out = append(out, or)
	}
	return out
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func unavailableReview(r *Review, reason string) {
	switch {
	case r.Operations != nil:
		for i := range r.Operations {
			r.Operations[i].Unavailable = reason
		}
	case r.Content != nil:
		r.Content.Unavailable = reason
	default:
		r.Metadata.Unavailable = reason
	}
}

// candidateContentValue observes the actual candidate simple-text target. It
// never substitutes the planned value for unreadable candidate content.
func candidateContentValue(a *archive.Archive, p *publication.Publication, param publication.TextSet) (string, error) {
	media := ""
	for _, item := range p.Manifest {
		if item.Path == param.BookPath {
			media = item.MediaType
		}
	}
	if media != "application/xhtml+xml" {
		return "", fmt.Errorf("candidate target is not manifest XHTML")
	}
	b, err := a.Read(param.BookPath, publication.XMLLimit)
	if err != nil {
		return "", err
	}
	return publication.ContentText(b, param.Locator, xmltext.Profile{Version: p.Version, MediaType: media})
}

// candidateMetadataValue observes the actual candidate metadata target.
// Ambiguity or complex content is reported, never guessed.
func candidateMetadataValue(p *publication.Publication, param metadata.Set) (string, error) {
	found := false
	value := ""
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
		if found || len(m.Children) != 0 {
			return "", fmt.Errorf("ambiguous or complex candidate metadata")
		}
		found = true
		value = m.Text
	}
	if !found {
		return "", fmt.Errorf("selected metadata is absent")
	}
	return value, nil
}

// candidateStructure parses one candidate XHTML resource for review. Bytes are
// cached per resource; the parse reuses the same strictness as planning.
func candidateStructure(a *archive.Archive, p *publication.Publication, cands map[string][]byte, bp bookpath.BookPath) (*publication.StructureDocument, error) {
	media, err := xhtmlMediaType(p, bp)
	if err != nil {
		return nil, err
	}
	b, ok := cands[string(bp)]
	if !ok {
		var readErr error
		b, readErr = a.Read(bp, publication.XMLLimit)
		if readErr != nil {
			return nil, readErr
		}
		cands[string(bp)] = b
	}
	return publication.ParseStructureDocument(b, bp, xmltext.Profile{Version: p.Version, MediaType: media})
}

// candidateAttribute observes one attribute operation's actual candidate value.
// A nil value means the attribute is absent; an error means the candidate could
// not be observed and must never be replaced by the planned value.
func candidateAttribute(a *archive.Archive, p *publication.Publication, cands map[string][]byte, base publicationRoot, bases map[string][]byte, docs map[string]*publication.StructureDocument, planned map[string][]*publication.StructureEdit, bp bookpath.BookPath, locator string, name xml.Name) (*string, error) {
	candDoc, err := candidateStructure(a, p, cands, bp)
	if err != nil {
		return nil, err
	}
	baseDoc, _, err := candidateBaseStructure(base, p, bp, bases, docs)
	if err != nil {
		return nil, err
	}
	e, err := candidateTargetElement(baseDoc, planned[string(bp)], candDoc, locator)
	if err != nil {
		return nil, err
	}
	for _, attr := range e.Attributes {
		if attr.Name == name {
			value := attr.Value
			return &value, nil
		}
	}
	return nil, nil
}

// candidateTargetElement follows the planned child path of a target into the
// parsed candidate, so a locator shift caused by another operation in the same
// transaction cannot be mistaken for the target itself.
func candidateTargetElement(base *publication.StructureDocument, edits []*publication.StructureEdit, cand *publication.StructureDocument, locator string) (*xmltext.Element, error) {
	if base == nil || edits == nil {
		return nil, fmt.Errorf("planned structure is unavailable")
	}
	path, err := publication.PlannedTarget(base, edits, locator)
	if err != nil {
		return nil, err
	}
	n := cand.Doc.Root
	for _, index := range path {
		if index >= len(n.Children) {
			return nil, fmt.Errorf("candidate structure is shorter than planned")
		}
		n = n.Children[index]
	}
	return n, nil
}

// candidateBaseStructure parses one frozen base resource for review, caching the
// bytes and the parsed document.
func candidateBaseStructure(base publicationRoot, p *publication.Publication, bp bookpath.BookPath, bases map[string][]byte, docs map[string]*publication.StructureDocument) (*publication.StructureDocument, []byte, error) {
	if doc, ok := docs[string(bp)]; ok {
		return doc, bases[string(bp)], nil
	}
	if base.root == nil {
		return nil, nil, fmt.Errorf("base revision is unavailable")
	}
	media, err := xhtmlMediaType(p, bp)
	if err != nil {
		return nil, nil, err
	}
	b, err := base.Read(bp, publication.XMLLimit)
	if err != nil {
		return nil, nil, err
	}
	doc, err := publication.ParseStructureDocument(b, bp, xmltext.Profile{Version: p.Version, MediaType: media})
	if err != nil {
		return nil, nil, err
	}
	bases[string(bp)] = b
	docs[string(bp)] = doc
	return doc, b, nil
}

func xhtmlMediaType(p *publication.Publication, bp bookpath.BookPath) (string, error) {
	for _, item := range p.Manifest {
		if item.Path == bp {
			if item.MediaType != "application/xhtml+xml" {
				return "", fmt.Errorf("candidate target is not manifest XHTML")
			}
			return item.MediaType, nil
		}
	}
	return "", fmt.Errorf("candidate target is not manifest XHTML")
}

// candidateElementEffect observes an element operation in the candidate. Exact
// block bytes are counted against the frozen base, and a deletion's ids must be
// absent. Placement is never claimed here; the file diff is authoritative.
func candidateElementEffect(a *archive.Archive, p *publication.Publication, base publicationRoot, bases, cands map[string][]byte, docs map[string]*publication.StructureDocument, bp bookpath.BookPath, op Operation) (string, error) {
	baseDoc, baseBytes, err := candidateBaseStructure(base, p, bp, bases, docs)
	if err != nil {
		return "", err
	}
	switch param := op.Params.(type) {
	case publication.ElementDelete:
		e, err := baseDoc.Locate(param.Locator)
		if err != nil {
			return "", err
		}
		ids := structureSubtreeIDs(e)
		if len(ids) == 0 {
			return "", fmt.Errorf("removed element has no id; review the file diff")
		}
		candDoc, err := candidateStructure(a, p, cands, bp)
		if err != nil {
			return "", err
		}
		candIDs := candDoc.IDs()
		for _, id := range ids {
			if candIDs[id] > 0 {
				return fmt.Sprintf("removed id %q is still present", id), nil
			}
		}
		return fmt.Sprintf("removed ids absent: %s", strings.Join(ids, ", ")), nil
	case publication.ElementInsert:
		block, err := baseDoc.Encode(param.Fragment)
		if err != nil {
			return "", err
		}
		return countBlockBytes(a, cands, bp, baseBytes, block, "present", "not observed verbatim")
	case publication.ElementReplace:
		block, err := baseDoc.Encode(param.Fragment)
		if err != nil {
			return "", err
		}
		return countBlockBytes(a, cands, bp, baseBytes, block, "present", "not observed verbatim")
	case publication.ElementMove:
		e, err := baseDoc.Locate(param.Locator)
		if err != nil {
			return "", err
		}
		start, end, ok := e.PhysicalMarkup()
		if !ok {
			return "", fmt.Errorf("moved element has no literal markup interval")
		}
		return countBlockBytes(a, cands, bp, baseBytes, baseBytes[start:end], "preserved", "missing")
	}
	return "", fmt.Errorf("unsupported operation params")
}

// countBlockBytes reports how the exact block bytes occur in the candidate
// relative to the frozen base. It is evidence, not proof of placement.
func countBlockBytes(a *archive.Archive, cands map[string][]byte, bp bookpath.BookPath, baseBytes, block []byte, increased, equal string) (string, error) {
	cand, ok := cands[string(bp)]
	if !ok {
		var err error
		cand, err = a.Read(bp, publication.XMLLimit)
		if err != nil {
			return "", err
		}
		cands[string(bp)] = cand
	}
	before, after := bytes.Count(baseBytes, block), bytes.Count(cand, block)
	switch {
	case after > before:
		return fmt.Sprintf("block bytes %s (occurrences %d→%d)", increased, before, after), nil
	case after == before:
		return fmt.Sprintf("block bytes %s (occurrences %d→%d)", equal, before, after), nil
	default:
		return fmt.Sprintf("block bytes decreased (occurrences %d→%d)", before, after), nil
	}
}

// structureSubtreeIDs collects unprefixed id and xml:id values in one subtree.
func structureSubtreeIDs(e *xmltext.Element) []string {
	out := []string{}
	var walk func(*xmltext.Element)
	walk = func(e *xmltext.Element) {
		for _, a := range e.Attributes {
			if (a.Name.Space == "" || a.Name.Space == publication.XMLNamespace) && a.Name.Local == "id" {
				out = append(out, a.Value)
			}
		}
		for _, c := range e.Children {
			walk(c)
		}
	}
	walk(e)
	return out
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
		return d, fmt.Errorf("%w: legacy task must be rejected and replanned before acceptance", ErrTaskConflict)
	}
	if o.Draft || o.Rootfile != "" && o.Rootfile != w.state.Rootfile {
		return d, fault.New(2, "INVALID_ARGUMENT", "accept requires formal checks of the selected rootfile")
	}
	e, err := w.execution()
	if err != nil {
		return d, err
	}
	if e.Status != "review_required" {
		return d, fmt.Errorf("%w: only a completed review task can be accepted", ErrTaskConflict)
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
	// Consumed plans remain bound to identity, stored plan and exact bytes,
	// not their former host directory, including durable pending settlements.
	if id != j.Decision.TaskID || t.BaseRevision != j.Decision.BaseRevision || p.BaseRevision != t.BaseRevision || p.WorkspaceID != j.WorkspaceID || digest(s.Plan) != digest(p) || digest(e.Plan) != digest(p) || e.Checkpoint != s.Checkpoint {
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
	if !validPlanOperation(p) || p.Rootfile != w.state.Rootfile || p.OperationSetSHA256 != digest(p.Operations) || s.Version != p.SchemaVersion || e.Version != s.Version || s.TaskID != e.TaskID || s.TaskID != id && !(id == "active" && s.TaskID == "") || (s.Status != "running" && s.Status != "unstarted") || (s.Status == "unstarted" && e.Status != "failed") || s.ReviewRequired || s.Conformance != "not_run" || digest(s.Diff) != digest(compareTrees(tree, tree)) || e.Conformance != "not_run" || e.Diff.Changes == nil {
		return fmt.Errorf("settlement operation/execution version mismatch")
	}
	d, err := w.recomputeAt(p.Operations, dir+"/checkpoints/"+s.Checkpoint+"/pub", p.BaseRevision)
	if err != nil {
		return err
	}
	if !p.Applicable || !slices.Equal(d.writes, p.WriteSet) {
		return fmt.Errorf("settlement write set mismatch")
	}
	if e.Status == "review_required" {
		if !e.ReviewRequired || e.Failure != "" || digest(e.Diff) != digest(expectedDiff(tree, d.outputs)) {
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
