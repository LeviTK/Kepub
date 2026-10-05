package workspace

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/metadata"
	"github.com/LeviTK/Kepub/internal/publication"
)

var ErrStalePlan = errors.New("stale or altered plan")
var ErrCandidateConflict = errors.New("workspace already has a candidate")
var ErrCandidateDrift = errors.New("candidate changed since execution; review actual diff or reject")

type Operation struct {
	ID      string `json:"operationId"`
	Version int    `json:"operationVersion"`
	Params  any    `json:"params"` // exactly metadata.Set or publication.TextSet
}

func (o *Operation) UnmarshalJSON(b []byte) error {
	var wire struct {
		ID      string          `json:"operationId"`
		Version int             `json:"operationVersion"`
		Params  json.RawMessage `json:"params"`
	}
	if err := decodeStrict(b, &wire); err != nil {
		return err
	}
	o.ID, o.Version = wire.ID, wire.Version
	switch wire.ID {
	case "metadata.set":
		var p metadata.Set
		if err := decodeStrict(wire.Params, &p); err != nil {
			return err
		}
		o.Params = p
	case "content.text.set":
		var p publication.TextSet
		if err := decodeStrict(wire.Params, &p); err != nil {
			return err
		}
		o.Params = p
	default:
		return fmt.Errorf("unsupported operation")
	}
	return nil
}

type Request struct {
	SchemaVersion int         `json:"schemaVersion"`
	Operations    []Operation `json:"operations"`
}
type Plan struct {
	SchemaVersion      int         `json:"schemaVersion"`
	ID                 string      `json:"planId"`
	WorkspaceID        string      `json:"workspaceId"`
	WorkspacePath      string      `json:"workspacePath"`
	BaseRevision       string      `json:"baseRevision"`
	InputTreeSHA256    string      `json:"inputTreeSha256"`
	OperationSetSHA256 string      `json:"operationSetSha256"`
	PolicySHA256       string      `json:"policySha256"`
	Rootfile           string      `json:"rootfile"`
	Operations         []Operation `json:"operations"`
	WriteSet           []string    `json:"writeSet"`
	Applicable         bool        `json:"applicable"`
}
type Change struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Before *Entry `json:"before"`
	After  *Entry `json:"after"`
}
type Diff struct {
	BeforeSHA256 string   `json:"beforeSha256"`
	AfterSHA256  string   `json:"afterSha256"`
	Changed      bool     `json:"changed"`
	Changes      []Change `json:"changes"`
}
type Execution struct {
	Version        int    `json:"version"`
	TaskID         string `json:"taskId,omitempty"`
	Plan           Plan   `json:"plan"`
	Checkpoint     string `json:"checkpoint"`
	Status         string `json:"status"`
	ReviewRequired bool   `json:"reviewRequired"`
	Conformance    string `json:"conformance"`
	Diff           Diff   `json:"diff"`
	Failure        string `json:"failure,omitempty"`
}

// Strict JSON rejects unknown/missing fields, null in place of values, duplicate
// object keys and trailing values. Typed JSON is the canonical digest encoding:
// struct field order, no whitespace, Go JSON string escaping, optional id absent.
func decodeStrict(b []byte, out any) error {
	if len(b) > 32<<20 || !utf8.Valid(b) {
		return fmt.Errorf("edit JSON limit")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	depth := 0
	var walk func() error
	walk = func() error {
		depth++
		defer func() { depth-- }()
		if depth > 128 {
			return fmt.Errorf("edit JSON depth limit")
		}
		t, err := d.Token()
		if err != nil {
			return err
		}
		if delim, ok := t.(json.Delim); ok {
			seen := map[string]bool{}
			for d.More() {
				if delim == '{' {
					k, err := d.Token()
					if err != nil {
						return err
					}
					key := k.(string)
					if seen[key] {
						return fmt.Errorf("duplicate JSON key")
					}
					seen[key] = true
				}
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		}
		return nil
	}
	if err := walk(); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("trailing JSON")
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	var raw, typed any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(encoded, &typed); err != nil {
		return err
	}
	rb, _ := json.Marshal(raw)
	tb, _ := json.Marshal(typed)
	if !bytes.Equal(rb, tb) {
		return fmt.Errorf("missing, null, or noncanonical optional fields")
	}
	return nil
}
func digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

const legacyEditPolicy = "kepub-metadata-v1:initial-only;single-set;simple-text;no-timestamp;review-required;conformance-not-run"
const editPolicy = "kepub-metadata-v2:accepted-baseline;single-set;simple-text;no-timestamp;review-required;conformance-not-run"
const contentEditPolicy = "kepub-content-text-v1:accepted-baseline;single-set;locator-v1;simple-text;no-timestamp;review-required;conformance-not-run"

func operationSchema(ops []Operation) (int, error) {
	if len(ops) != 1 || ops[0].Version != 1 {
		return 0, fmt.Errorf("requires one supported version 1 operation")
	}
	switch ops[0].ID {
	case "metadata.set":
		if _, ok := ops[0].Params.(metadata.Set); ok {
			return 1, nil
		}
	case "content.text.set":
		if p, ok := ops[0].Params.(publication.TextSet); ok {
			if err := p.Validate(); err != nil {
				return 0, fmt.Errorf("content params: %v", err)
			}
			if p.RevisionID != "initial" && !validID(p.RevisionID) {
				return 0, fmt.Errorf("invalid revisionId")
			}
			return 2, nil
		}
	}
	return 0, fmt.Errorf("unsupported operation params")
}

func policyFor(version int) string {
	if version == 2 {
		return contentEditPolicy
	}
	return editPolicy
}

func validPlanOperation(p Plan) bool {
	v, err := operationSchema(p.Operations)
	return err == nil && p.SchemaVersion == v && (p.PolicySHA256 == digest(policyFor(v)) || v == 1 && p.BaseRevision == "initial" && p.PolicySHA256 == digest(legacyEditPolicy))
}

type identity struct {
	Version int    `json:"version"`
	ID      string `json:"workspaceId"`
}
type planUse struct {
	Version    int    `json:"version"`
	TaskID     string `json:"taskId"`
	PlanSHA256 string `json:"planSha256"`
}

// ID persists a random identity for legacy M2-A workspaces on first use. State
// v1 remains unchanged. An identity cannot be regenerated once plans exist.
func (w *Workspace) ID() (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return "", err
	}
	if err := w.ensureIdentity(); err != nil {
		return "", err
	}
	return w.id, nil
}
func (w *Workspace) ensureIdentity() error {
	if !exists(w.root, "identity.json") {
		if w.id != "" || exists(w.root, "plans") || exists(w.root, "tasks/active/edit-intent.json") {
			return fmt.Errorf("missing workspace identity")
		}
		if err := writeJSON(w.root, "identity.json", identity{1, randomID()}); err != nil {
			return err
		}
		if err := syncDir(w.root, "."); err != nil {
			return err
		}
	}
	var record identity
	if err := readEditJSON(w.root, "identity.json", &record); err != nil {
		return err
	}
	if record.Version != 1 || !validID(record.ID) || (w.id != "" && record.ID != w.id) {
		return fmt.Errorf("invalid workspace identity")
	}
	w.id = record.ID
	return nil
}

func readEditJSON(r *os.Root, name string, out any) error {
	p, err := subdir(r, path.Dir(name))
	if err != nil {
		return err
	}
	defer p.Close()
	f, err := openRegular(p, path.Base(name))
	if err != nil {
		return err
	}
	b, err := io.ReadAll(io.LimitReader(f, (32<<20)+1))
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	return decodeStrict(b, out)
}

// ReadEditFile rejects symlink ancestors/leaves, special files and hard links.
// openRegular uses O_NONBLOCK and inode rechecking, so FIFO/device replacement
// cannot block before the regular-file test. Host paths are explicit, not found.
func ReadEditFile(file string) ([]byte, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, err
	}
	r, err := openDir(filepath.Dir(abs))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	f, err := openRegular(r, filepath.Base(abs))
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(io.LimitReader(f, (32<<20)+1))
	err = errors.Join(err, f.Close())
	if len(b) > 32<<20 {
		return nil, fmt.Errorf("edit file exceeds 32 MiB")
	}
	return b, err
}

func (w *Workspace) recompute(ops []Operation) ([]byte, []string, error) {
	return w.recomputeAt(ops, revisionPath(w.current), w.current)
}

// The same derivation is used for current plans and historical checkpoint
// provenance during acceptance/recovery. Neither trusts a persisted write set.
func (w *Workspace) recomputeAt(ops []Operation, baseDir, revision string) ([]byte, []string, error) {
	version, err := operationSchema(ops)
	if err != nil {
		return nil, nil, err
	}
	r, err := subdir(w.root, baseDir)
	if err != nil {
		return nil, nil, err
	}
	defer r.Close()
	a := publicationRoot{r}
	if version == 2 {
		param := ops[0].Params.(publication.TextSet)
		if param.RevisionID != revision {
			return nil, nil, ErrStalePlan
		}
		p, err := publication.Load(a, w.state.Rootfile)
		if err != nil {
			return nil, nil, err
		}
		out, changed, err := publication.ApplyText(a, p, param)
		if err != nil {
			return nil, nil, err
		}
		writes := []string{}
		if changed {
			writes = append(writes, string(param.BookPath))
		}
		return out, writes, nil
	}
	b, err := a.Read(bookpath.BookPath(w.state.Rootfile), publication.XMLLimit)
	if err != nil {
		return nil, nil, err
	}
	out, changed, err := metadata.Apply(b, ops[0].Params.(metadata.Set))
	if err != nil {
		return nil, nil, err
	}
	writeSet := []string{}
	if changed {
		writeSet = append(writeSet, w.state.Rootfile)
	}
	return out, writeSet, nil
}

// Plan reads the immutable accepted snapshot and persists only a plan report.
// No candidate mutation or conformance approval occurs at this stage.
func (w *Workspace) Plan(requestJSON []byte) (Plan, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return Plan{}, err
	}
	if len(w.state.ReadOnlyReasons) != 0 {
		return Plan{}, ErrReadOnly
	}
	if err := w.verifyBaseline(); err != nil {
		return Plan{}, err
	}
	if err := w.ensureIdentity(); err != nil {
		return Plan{}, err
	}
	var request Request
	if err := decodeStrict(requestJSON, &request); err != nil {
		return Plan{}, err
	}
	version, err := operationSchema(request.Operations)
	if err != nil {
		return Plan{}, err
	}
	if request.SchemaVersion != version {
		return Plan{}, fmt.Errorf("unsupported request version")
	}
	_, writes, err := w.recompute(request.Operations)
	if err != nil {
		return Plan{}, err
	}
	p := Plan{version, randomID(), w.id, w.dir, w.current, w.base.SHA256, digest(request.Operations), digest(policyFor(version)), w.state.Rootfile, request.Operations, writes, true}
	if !exists(w.root, "plans") {
		if err := w.root.Mkdir("plans", 0700); err != nil {
			return Plan{}, err
		}
	}
	if r, err := subdir(w.root, "plans"); err != nil {
		return Plan{}, err
	} else {
		r.Close()
	}
	if err := writeJSON(w.root, "plans/"+p.ID+".json", p); err != nil {
		return Plan{}, err
	}
	return p, syncDir(w.root, "plans")
}

func (w *Workspace) verifyPlan(p Plan, bindPath bool) ([]byte, error) {
	if err := w.ensureIdentity(); err != nil {
		return nil, err
	}
	if err := w.verifyBaseline(); err != nil {
		return nil, errors.Join(ErrStalePlan, err)
	}
	if !validPlanOperation(p) || !validID(p.ID) || p.WriteSet == nil || p.WorkspaceID != w.id || bindPath && p.WorkspacePath != w.dir || p.BaseRevision != w.current || p.InputTreeSHA256 != w.base.SHA256 || p.Rootfile != w.state.Rootfile || p.OperationSetSHA256 != digest(p.Operations) {
		return nil, ErrStalePlan
	}
	var stored Plan
	if err := readEditJSON(w.root, "plans/"+p.ID+".json", &stored); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrStalePlan
		}
		return nil, err
	}
	if digest(stored) != digest(p) {
		return nil, ErrStalePlan
	}
	if err := w.verifyBaseline(); err != nil {
		return nil, errors.Join(ErrStalePlan, err)
	}
	out, writes, err := w.recompute(p.Operations)
	if err != nil {
		return nil, err
	}
	if !p.Applicable || !slices.Equal(writes, p.WriteSet) {
		return nil, ErrStalePlan
	}
	return out, nil
}

// Apply strictly reads a plan, re-derives its effects, then creates the sole
// independent candidate. Repeated application is a conflict, never a new task.
func (w *Workspace) Apply(planJSON []byte) (e Execution, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return Execution{}, err
	}
	var p Plan
	if err := decodeStrict(planJSON, &p); err != nil {
		return Execution{}, err
	}
	out, err := w.verifyPlan(p, true)
	if err != nil {
		return Execution{}, err
	}
	if exists(w.root, "tasks/active") {
		return Execution{}, ErrCandidateConflict
	}
	if exists(w.root, "plans/"+p.ID+".used.json") {
		return Execution{}, ErrTaskConflict
	}
	defer func() {
		if err == nil {
			return
		}
		// Publication can succeed before a later sync/startup error. Expose
		// only this call's durable, identity- and plan-bound task, never latest.
		if e.TaskID == "" && exists(w.root, "tasks/active") {
			id, idErr := w.taskID()
			var intent Plan
			var used planUse
			if idErr == nil && readEditJSON(w.root, "tasks/active/edit-intent.json", &intent) == nil && digest(intent) == digest(p) && readEditJSON(w.root, "plans/"+p.ID+".used.json", &used) == nil && used.Version == 1 && used.TaskID == id && used.PlanSHA256 == digest(p) {
				e = Execution{Version: p.SchemaVersion, TaskID: id, Plan: p, Conformance: "not_run"}
			}
		}
		if e.TaskID != "" {
			e.Status, e.Failure, e.ReviewRequired = "failed", err.Error(), false
		}
	}()
	if _, err := w.createCandidate(&p); err != nil {
		return Execution{}, err
	}
	e, err = w.startExecution(p)
	if err != nil {
		return e, err
	}
	return w.execute(e, out, nil)
}

// startExecution records the pre-mutation checkpoint and durable start. Even
// when startup fails, its return value identifies the already published task.
func (w *Workspace) startExecution(p Plan) (e Execution, err error) {
	task, err := w.taskID()
	if err != nil {
		return e, err
	}
	e = Execution{Version: p.SchemaVersion, TaskID: task, Plan: p, Status: "running", Conformance: "not_run"}
	s, err := w.checkpoint()
	if err != nil {
		w.recovery = true
		return e, err
	}
	e.Checkpoint, e.Diff = s.ID, compareTrees(s.Tree, s.Tree)
	if err := writeJSON(w.root, "tasks/active/edit-start.json", e); err != nil {
		w.recovery = true
		return e, err
	}
	if err := syncDir(w.root, "tasks/active"); err != nil {
		w.recovery = true
		return e, err
	}
	return e, nil
}

// execute's hook is used only by package tests to simulate a stopped external
// writer or an I/O failure at the mutation boundary; no production bypass API.
func (w *Workspace) execute(e Execution, out []byte, hook func() error) (Execution, error) {
	var err error
	if len(e.Plan.WriteSet) > 0 {
		target := e.Plan.WriteSet[0]
		r, e2 := subdir(w.root, candidate+"/"+path.Dir(target))
		if e2 != nil {
			err = e2
		} else {
			f, e2 := openRegular(r, path.Base(target))
			if e2 != nil {
				err = e2
			} else {
				err = f.Close()
			}
			if err == nil {
				// Anchor both temporary write and replacement to the opened target
				// parent, never resolve a workspace-relative publication path again.
				name := ".kepub-edit-" + randomID()
				f, e2 = r.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
				if e2 != nil {
					err = e2
				} else {
					_, err = f.Write(out)
					err = errors.Join(err, f.Sync(), f.Close())
					if err == nil {
						err = r.Rename(name, path.Base(target))
					}
					err = errors.Join(err, r.RemoveAll(name), syncDir(r, "."))
				}
			}
			r.Close()
		}
	}
	if err == nil && hook != nil {
		err = hook()
	}
	if err == nil {
		err = w.verifyBaseline()
	}
	if err == nil {
		var actual Tree
		actual, err = hashAt(w.root, candidate)
		if err == nil {
			e.Diff = compareTrees(w.base, actual)
			paths := []string{}
			for _, c := range e.Diff.Changes {
				paths = append(paths, c.Path)
			}
			if !slices.Equal(paths, e.Plan.WriteSet) {
				err = fmt.Errorf("actual writes exceed plan")
			}
			// Verify exact content, not merely the set of changed filenames.
			if err == nil && len(paths) > 0 {
				h := sha256.Sum256(out)
				for _, entry := range actual.Entries {
					if entry.Path == e.Plan.WriteSet[0] && (entry.Type != "file" || entry.Size != int64(len(out)) || entry.SHA256 != hex.EncodeToString(h[:])) {
						err = fmt.Errorf("candidate content mismatch")
					}
				}
			}
		}
	}
	if err != nil {
		e.Status = "failed"
		e.Failure = err.Error()
		e.ReviewRequired = false
		err = errors.Join(err, w.restore(e.Checkpoint))
	} else {
		e.Status = "review_required"
		e.ReviewRequired = true
	}
	// Publish only complete JSON. If recording fails, the operation fails too:
	// restore before returning rather than leaving an unrecorded successful edit.
	reportStage := "staging/" + randomID()
	recordErr := writeJSON(w.root, reportStage, e)
	if recordErr == nil {
		recordErr = publish(w.root, reportStage, "tasks/active/edit-result.json")
	}
	// A successful no-replace publish gives this invocation ownership.
	published := recordErr == nil
	if recordErr == nil {
		recordErr = errors.Join(syncDir(w.root, "tasks/active"), syncDir(w.root, "staging"))
	}
	if recordErr != nil {
		if e.Status != "failed" {
			err = errors.Join(err, w.restore(e.Checkpoint))
		}
		e.Status = "failed"
		e.ReviewRequired = false
		e.Failure = errors.Join(err, recordErr).Error()
		if published {
			recordErr = errors.Join(recordErr, w.root.Remove("tasks/active/edit-result.json"))
		}
		// The intact start/checkpoint also gives Open an interrupted-apply
		// failure record if the filesystem still cannot persist this report.
		failureStage := "staging/" + randomID()
		failureErr := writeJSON(w.root, failureStage, e)
		if failureErr == nil {
			failureErr = publish(w.root, failureStage, "tasks/active/edit-result.json")
		}
		err = errors.Join(err, recordErr, failureErr, w.root.RemoveAll(failureStage))
		w.recovery = true
	}
	err = errors.Join(err, w.root.RemoveAll(reportStage))
	return e, err
}

func expectedDiff(base Tree, writes []string, out []byte) Diff {
	after := Tree{Entries: slices.Clone(base.Entries)}
	if len(writes) > 0 {
		h := sha256.Sum256(out)
		for i, entry := range after.Entries {
			if entry.Path == writes[0] {
				after.Entries[i] = Entry{entry.Path, "file", int64(len(out)), hex.EncodeToString(h[:])}
			}
		}
	}
	after.SHA256 = hashEntries(after.Entries)
	return compareTrees(base, after)
}

func compareTrees(before, after Tree) Diff {
	d := Diff{before.SHA256, after.SHA256, false, []Change{}}
	i, j := 0, 0
	for i < len(before.Entries) || j < len(after.Entries) {
		var b, a *Entry
		if j == len(after.Entries) || (i < len(before.Entries) && before.Entries[i].Path < after.Entries[j].Path) {
			v := before.Entries[i]
			b = &v
			i++
		} else if i == len(before.Entries) || after.Entries[j].Path < before.Entries[i].Path {
			v := after.Entries[j]
			a = &v
			j++
		} else {
			bv, av := before.Entries[i], after.Entries[j]
			b, a = &bv, &av
			i++
			j++
			if *b == *a {
				continue
			}
		}
		c := Change{Before: b, After: a, Kind: "modified"}
		if b == nil {
			c.Path = a.Path
			c.Kind = "added"
		} else {
			c.Path = b.Path
			if a == nil {
				c.Kind = "deleted"
			}
		}
		d.Changes = append(d.Changes, c)
	}
	d.Changed = len(d.Changes) > 0
	return d
}

// Diff scans both actual trees, including new/unlisted files and directories.
// It does not trust a planned write set or a prior execution report.
func (w *Workspace) Diff() (Diff, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return Diff{}, err
	}
	if err := w.verifyBaseline(); err != nil {
		return Diff{}, err
	}
	if err := w.verifyTask(); err != nil {
		return Diff{}, err
	}
	a, err := hashAt(w.root, candidate)
	if err != nil {
		return Diff{}, err
	}
	return compareTrees(w.base, a), nil
}

// Execution verifies persisted execution provenance. A returned review record is
// bound to its frozen hash; edits since completion must not inherit that status.
func (w *Workspace) Execution() (Execution, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ready(); err != nil {
		return Execution{}, err
	}
	return w.execution()
}
func (w *Workspace) execution() (Execution, error) {
	var start, e Execution
	if err := w.verifyTask(); err != nil {
		return e, err
	}
	var intent Plan
	if err := readEditJSON(w.root, "tasks/active/edit-intent.json", &intent); err != nil {
		return e, err
	}
	// A registered task has consumed its plan. Its stored identity and source
	// still bind execution after moving the workspace, not the former host path.
	out, err := w.verifyPlan(intent, false)
	if err != nil {
		return e, err
	}
	task, err := w.taskID()
	if err != nil {
		return e, err
	}
	// Legacy active tasks may predate consumption records; an existing record
	// must still bind the task, not be bypassed by changing its outer version.
	if task != "active" || exists(w.root, "plans/"+intent.ID+".used.json") {
		var used planUse
		if err := readEditJSON(w.root, "plans/"+intent.ID+".used.json", &used); err != nil {
			return e, err
		}
		if used.Version != 1 || used.TaskID != task || used.PlanSHA256 != digest(intent) {
			return e, fmt.Errorf("execution plan consumption mismatch")
		}
	}
	unstarted := !exists(w.root, "tasks/active/edit-start.json")
	if unstarted {
		if exists(w.root, "tasks/active/edit-result.json") {
			return e, fmt.Errorf("execution result without start")
		}
		// No registered mutation can precede edit-start. Capture the verified
		// baseline as provenance; preserve any outside writer's candidate drift.
		s, err := w.checkpointFrom(revisionPath(w.current))
		if err != nil {
			return e, err
		}
		// Persist the distinction: retrying after start but before result must
		// not mistake this synthetic record for a mutation that needs rollback.
		start = Execution{Version: intent.SchemaVersion, TaskID: task, Plan: intent, Checkpoint: s.ID, Status: "unstarted", Conformance: "not_run", Diff: compareTrees(s.Tree, s.Tree)}
		if err := writeJSON(w.root, "tasks/active/edit-start.json", start); err != nil {
			return e, err
		}
		if err := syncDir(w.root, "tasks/active"); err != nil {
			return e, err
		}
	} else if err := readEditJSON(w.root, "tasks/active/edit-start.json", &start); err != nil {
		return e, err
	}
	if digest(start.Plan) != digest(intent) {
		return e, fmt.Errorf("execution intent mismatch")
	}
	if start.TaskID != task && !(start.TaskID == "" && task == "active") {
		return e, fmt.Errorf("execution task identity mismatch")
	}
	s, err := w.snapshot(start.Checkpoint)
	if err != nil {
		return e, err
	}
	if start.Version != intent.SchemaVersion || (start.Status != "running" && start.Status != "unstarted") || start.ReviewRequired || start.Conformance != "not_run" || digest(start.Diff) != digest(compareTrees(s.Tree, s.Tree)) || s.Tree.SHA256 != w.base.SHA256 {
		return e, fmt.Errorf("invalid execution start")
	}
	if !exists(w.root, "tasks/active/edit-result.json") {
		// Interrupted mutation is rolled back, not silently rerun as a new task.
		actual, err := hashAt(w.root, candidate)
		if err != nil {
			return e, err
		}
		e = start
		e.Diff = compareTrees(w.base, actual)
		if start.Status == "running" {
			if err := w.restore(start.Checkpoint); err != nil {
				return e, err
			}
		}
		e.Status = "failed"
		e.Failure = "interrupted apply"
		if err := writeJSON(w.root, "tasks/active/edit-result.json", e); err != nil {
			return e, err
		}
		if err := syncDir(w.root, "tasks/active"); err != nil {
			return e, err
		}
	} else if err := readEditJSON(w.root, "tasks/active/edit-result.json", &e); err != nil {
		return e, err
	}
	if e.TaskID != start.TaskID {
		return e, fmt.Errorf("execution task identity mismatch")
	}
	if e.Version != start.Version || e.Diff.Changes == nil || digest(e.Plan) != digest(start.Plan) || e.Checkpoint != start.Checkpoint || e.Conformance != "not_run" || (start.Status == "unstarted" && e.Status != "failed") {
		return e, fmt.Errorf("invalid execution provenance")
	}
	tree, err := hashAt(w.root, candidate)
	if err != nil {
		return e, err
	}
	switch e.Status {
	case "failed":
		if e.ReviewRequired || e.Failure == "" {
			return e, fmt.Errorf("invalid failed execution")
		}
		if tree.SHA256 != s.Tree.SHA256 {
			return e, ErrCandidateDrift
		}
	case "review_required":
		if !e.ReviewRequired || e.Failure != "" {
			return e, fmt.Errorf("stale execution result")
		}
		if digest(e.Diff) != digest(expectedDiff(w.base, intent.WriteSet, out)) {
			return e, fmt.Errorf("execution write set/content mismatch")
		}
		if digest(e.Diff) != digest(compareTrees(w.base, tree)) {
			return e, ErrCandidateDrift
		}
	default:
		return e, fmt.Errorf("invalid execution state")
	}
	return e, nil
}
