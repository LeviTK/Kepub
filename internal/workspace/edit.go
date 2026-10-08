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
	"strings"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/fix"
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
	case "xhtml.attribute.set":
		var p publication.AttributeSet
		if err := decodeStrict(wire.Params, &p); err != nil {
			return err
		}
		o.Params = p
	case "xhtml.attribute.remove":
		var p publication.AttributeRemove
		if err := decodeStrict(wire.Params, &p); err != nil {
			return err
		}
		o.Params = p
	case "xhtml.element.delete":
		var p publication.ElementDelete
		if err := decodeStrict(wire.Params, &p); err != nil {
			return err
		}
		o.Params = p
	case "xhtml.element.insert":
		var p publication.ElementInsert
		if err := decodeStrict(wire.Params, &p); err != nil {
			return err
		}
		o.Params = p
	case "xhtml.element.replace":
		var p publication.ElementReplace
		if err := decodeStrict(wire.Params, &p); err != nil {
			return err
		}
		o.Params = p
	case "xhtml.element.move":
		if wire.Version == 2 {
			var p publication.ElementMoveCross
			if err := decodeStrict(wire.Params, &p); err != nil {
				return err
			}
			o.Params = p
			return nil
		}
		var p publication.ElementMove
		if err := decodeStrict(wire.Params, &p); err != nil {
			return err
		}
		o.Params = p
	case "content.text.replace":
		var p publication.TextReplace
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
	SchemaVersion      int           `json:"schemaVersion"`
	ID                 string        `json:"planId"`
	WorkspaceID        string        `json:"workspaceId"`
	WorkspacePath      string        `json:"workspacePath"`
	BaseRevision       string        `json:"baseRevision"`
	InputTreeSHA256    string        `json:"inputTreeSha256"`
	OperationSetSHA256 string        `json:"operationSetSha256"`
	PolicySHA256       string        `json:"policySha256"`
	Rootfile           string        `json:"rootfile"`
	Proposal           *fix.Proposal `json:"proposal,omitempty"`
	Operations         []Operation   `json:"operations"`
	WriteSet           []string      `json:"writeSet"`
	Applicable         bool          `json:"applicable"`
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
	if len(b) > maxJSONBytes || !utf8.Valid(b) {
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
const multiEditPolicy = "kepub-multi-v1:accepted-baseline;multi-operation;multi-resource;frozen-baseline;simple-text;no-timestamp;review-required;conformance-not-run"
const structurePolicy = "kepub-xhtml-structure-v1:accepted-baseline;multi-operation;frozen-baseline;locator-v1;reference-gate;no-timestamp;review-required;conformance-not-run"
const replacePolicy = "kepub-content-text-replace-v1:accepted-baseline;multi-operation;frozen-baseline;locator-v1;explicit-hits;no-timestamp;review-required;conformance-not-run"
const movePolicy = "kepub-xhtml-move-v1:accepted-baseline;multi-operation;frozen-baseline;locator-v1;cross-resource;reference-sync;no-timestamp;review-required;conformance-not-run"
const referencePolicy = ";reference-parser-v2"

// MaxPlanOperations bounds one version 3 or version 4 transaction. Each
// operation may parse its target resource, so the request file size alone is
// not a work bound.
// MaxPlanOperations is the inherited bound of one plan transaction. It is
// exported so output commands enforce the same budget before publishing.
const MaxPlanOperations = 256

// derivation is the complete deterministic effect of one plan against the frozen
// baseline: every changed BookPath with its exact final bytes. No caller may
// assume a single resource or a single output.
type derivation struct {
	outputs map[string][]byte
	writes  []string
	// edits carries each resource's planned structural edits, so review can
	// locate a planned target in the candidate without guessing.
	edits map[string][]*publication.StructureEdit
	// replaces carries each batch replace operation's facts by operation index.
	replaces map[int]publication.ReplaceFacts
	// moves carries each cross-resource move's derived facts by operation index.
	moves map[int]*publication.CrossMoveEdit
	// sync carries each cross-resource move's synchronized references by
	// operation index.
	sync map[int][]moveSyncRewrite
}

func singleDerivation(path string, out []byte, changed bool) derivation {
	if !changed {
		return derivation{outputs: map[string][]byte{}, writes: []string{}}
	}
	return derivation{outputs: map[string][]byte{path: out}, writes: []string{path}}
}

// structureOperation reports whether a request carries a versioned XHTML
// structural operation. Such requests use schema 4; single metadata/content
// requests keep their frozen schema 1/2/3 encodings.
func structureOperation(ops []Operation) bool {
	for _, op := range ops {
		if strings.HasPrefix(op.ID, "xhtml.") {
			return true
		}
	}
	return false
}

// replaceOperation reports whether a request carries the batch text replace
// operation. Such requests use schema 5, which may also carry the frozen v1
// operations; requests without it keep schema 1/2/3/4.
func replaceOperation(ops []Operation) bool {
	for _, op := range ops {
		if op.ID == "content.text.replace" {
			return true
		}
	}
	return false
}

func validateStructureOperation(op Operation) error {
	if op.Version != 1 && !(op.ID == "xhtml.element.move" && op.Version == 2) {
		return fmt.Errorf("structure operations require version 1")
	}
	switch p := op.Params.(type) {
	case metadata.Set:
		return nil
	case publication.TextSet:
		if err := p.Validate(); err != nil {
			return fmt.Errorf("content params: %v", err)
		}
		if p.RevisionID != "initial" && !validID(p.RevisionID) {
			return fmt.Errorf("invalid revisionId")
		}
		return nil
	case publication.AttributeSet:
		if err := p.Validate(); err != nil {
			return fmt.Errorf("xhtml params: %v", err)
		}
		if p.RevisionID != "initial" && !validID(p.RevisionID) {
			return fmt.Errorf("invalid revisionId")
		}
		return nil
	case publication.AttributeRemove:
		if err := p.Validate(); err != nil {
			return fmt.Errorf("xhtml params: %v", err)
		}
		if p.RevisionID != "initial" && !validID(p.RevisionID) {
			return fmt.Errorf("invalid revisionId")
		}
		return nil
	case publication.ElementDelete:
		if err := p.Validate(); err != nil {
			return fmt.Errorf("xhtml params: %v", err)
		}
		if p.RevisionID != "initial" && !validID(p.RevisionID) {
			return fmt.Errorf("invalid revisionId")
		}
		return nil
	case publication.ElementInsert:
		if err := p.Validate(); err != nil {
			return fmt.Errorf("xhtml params: %v", err)
		}
		if p.RevisionID != "initial" && !validID(p.RevisionID) {
			return fmt.Errorf("invalid revisionId")
		}
		return nil
	case publication.ElementReplace:
		if err := p.Validate(); err != nil {
			return fmt.Errorf("xhtml params: %v", err)
		}
		if p.RevisionID != "initial" && !validID(p.RevisionID) {
			return fmt.Errorf("invalid revisionId")
		}
		return nil
	case publication.ElementMove:
		if err := p.Validate(); err != nil {
			return fmt.Errorf("xhtml params: %v", err)
		}
		if p.RevisionID != "initial" && !validID(p.RevisionID) {
			return fmt.Errorf("invalid revisionId")
		}
		return nil
	case publication.ElementMoveCross:
		if err := p.Validate(); err != nil {
			return fmt.Errorf("xhtml params: %v", err)
		}
		for _, endpoint := range []publication.MoveEndpoint{p.Source, p.Destination} {
			if endpoint.RevisionID != "initial" && !validID(endpoint.RevisionID) {
				return fmt.Errorf("invalid revisionId")
			}
		}
		return nil
	case publication.TextReplace:
		if err := p.Validate(); err != nil {
			return fmt.Errorf("replace params: %v", err)
		}
		if p.RevisionID != "initial" && !validID(p.RevisionID) {
			return fmt.Errorf("invalid revisionId")
		}
		return nil
	}
	return fmt.Errorf("unsupported operation params")
}

// crossMoveOperation reports whether a request carries a cross-resource move.
// Such requests use schema 6, which may also carry the frozen v1 operations;
// requests without it keep schema 1/2/3/4/5.
func crossMoveOperation(ops []Operation) bool {
	for _, op := range ops {
		if op.ID == "xhtml.element.move" && op.Version == 2 {
			return true
		}
	}
	return false
}

func operationSchema(ops []Operation) (int, error) {
	if crossMoveOperation(ops) {
		if len(ops) == 0 || len(ops) > MaxPlanOperations {
			return 0, fmt.Errorf("operation count exceeds %d", MaxPlanOperations)
		}
		for _, op := range ops {
			if err := validateStructureOperation(op); err != nil {
				return 0, err
			}
		}
		return 6, nil
	}
	if replaceOperation(ops) {
		if len(ops) == 0 || len(ops) > MaxPlanOperations {
			return 0, fmt.Errorf("operation count exceeds %d", MaxPlanOperations)
		}
		for _, op := range ops {
			if err := validateStructureOperation(op); err != nil {
				return 0, err
			}
		}
		return 5, nil
	}
	if structureOperation(ops) {
		if len(ops) == 0 || len(ops) > MaxPlanOperations {
			return 0, fmt.Errorf("operation count exceeds %d", MaxPlanOperations)
		}
		for _, op := range ops {
			if err := validateStructureOperation(op); err != nil {
				return 0, err
			}
		}
		return 4, nil
	}
	if len(ops) == 1 {
		if ops[0].Version != 1 {
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
	if len(ops) > 1 {
		if len(ops) > MaxPlanOperations {
			return 0, fmt.Errorf("operation count exceeds %d", MaxPlanOperations)
		}
		for _, op := range ops {
			if op.Version != 1 {
				return 0, fmt.Errorf("multi-operation plans require version 1 operations")
			}
			switch op.ID {
			case "metadata.set":
				if _, ok := op.Params.(metadata.Set); !ok {
					return 0, fmt.Errorf("unsupported operation params")
				}
			case "content.text.set":
				p, ok := op.Params.(publication.TextSet)
				if !ok {
					return 0, fmt.Errorf("unsupported operation params")
				}
				if err := p.Validate(); err != nil {
					return 0, fmt.Errorf("content params: %v", err)
				}
				if p.RevisionID != "initial" && !validID(p.RevisionID) {
					return 0, fmt.Errorf("invalid revisionId")
				}
			default:
				return 0, fmt.Errorf("unsupported operation")
			}
		}
		return 3, nil
	}
	return 0, fmt.Errorf("requires one supported version 1 operation")
}

func policyFor(version int) string {
	policy := legacyPolicyFor(version)
	if version >= 4 && version <= 7 {
		policy += referencePolicy
	}
	if version == 7 {
		policy += fileTargetPolicy
	}
	return policy
}

func legacyPolicyFor(version int) string {
	switch version {
	case 2:
		return contentEditPolicy
	case 3:
		return multiEditPolicy
	case 4:
		return structurePolicy
	case 5:
		return replacePolicy
	case 6:
		return movePolicy
	case 7:
		return fixPolicy
	}
	return editPolicy
}

func planReferenceVersion(p Plan) int {
	if p.SchemaVersion >= 4 && p.SchemaVersion <= 7 && p.PolicySHA256 == digest(legacyPolicyFor(p.SchemaVersion)) {
		return 1
	}
	return publication.ReferenceParserVersion
}

func planTargetVersion(p Plan) int {
	if p.SchemaVersion == 7 && p.PolicySHA256 != digest(policyFor(7)) {
		return 1
	}
	return 2
}

func validPlanOperation(p Plan) bool {
	if p.SchemaVersion == 7 {
		return p.Proposal != nil && (p.PolicySHA256 == digest(policyFor(7)) || p.PolicySHA256 == digest(fixPolicy+referencePolicy) || p.PolicySHA256 == digest(fixPolicy))
	}
	if p.Proposal != nil {
		return false
	}
	v, err := operationSchema(p.Operations)
	return err == nil && p.SchemaVersion == v && (p.PolicySHA256 == digest(policyFor(v)) || p.PolicySHA256 == digest(legacyPolicyFor(v)) || v == 1 && p.BaseRevision == "initial" && p.PolicySHA256 == digest(legacyEditPolicy))
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
	return readEditJSONLimit(r, name, out, maxJSONBytes)
}

func readEditJSONLimit(r *os.Root, name string, out any, limit int64) (err error) {
	p, err := subdir(r, path.Dir(name))
	if err != nil {
		return err
	}
	defer p.Close()
	f, err := openRegular(p, path.Base(name))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if err := checkJSONSize(info.Size(), limit); err != nil {
		return err
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return err
	}
	if err := checkJSONSize(int64(len(b)), limit); err != nil {
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
	b, err := io.ReadAll(io.LimitReader(f, maxJSONBytes+1))
	err = errors.Join(err, f.Close())
	if len(b) > maxJSONBytes {
		return nil, fmt.Errorf("edit file exceeds 32 MiB")
	}
	return b, err
}

func (w *Workspace) recompute(ops []Operation) (derivation, error) {
	return w.recomputeAt(ops, revisionPath(w.current), w.current, publication.ReferenceParserVersion)
}

// The same derivation is used for current plans and historical checkpoint
// provenance during acceptance/recovery. Neither trusts a persisted write set.
func (w *Workspace) recomputeAt(ops []Operation, baseDir, revision string, referenceVersion int) (derivation, error) {
	version, err := operationSchema(ops)
	if err != nil {
		return derivation{}, err
	}
	r, err := subdir(w.root, baseDir)
	if err != nil {
		return derivation{}, err
	}
	defer r.Close()
	a := publicationRoot{r}
	if version == 4 || version == 5 || version == 6 {
		return w.recomputeStructure(a, ops, revision, referenceVersion)
	}
	if version == 3 {
		return w.recomputeMulti(a, ops, revision)
	}
	if version == 2 {
		param := ops[0].Params.(publication.TextSet)
		if param.RevisionID != revision {
			return derivation{}, ErrStalePlan
		}
		p, err := publication.Load(a, w.state.Rootfile)
		if err != nil {
			return derivation{}, err
		}
		out, changed, err := publication.ApplyText(a, p, param)
		if err != nil {
			return derivation{}, err
		}
		return singleDerivation(string(param.BookPath), out, changed), nil
	}
	b, err := a.Read(bookpath.BookPath(w.state.Rootfile), publication.XMLLimit)
	if err != nil {
		return derivation{}, err
	}
	out, changed, err := metadata.Apply(b, ops[0].Params.(metadata.Set))
	if err != nil {
		return derivation{}, err
	}
	return singleDerivation(w.state.Rootfile, out, changed), nil
}

// recomputeMulti derives a multi-operation plan against the frozen baseline
// before applying it in order. Every binding (revision, resource hash, expected
// old value) is checked against the frozen baseline, so an earlier operation can
// never rewrite a later operation's expectation. Duplicate and aliased targets
// are rejected instead of being applied twice.
func (w *Workspace) recomputeMulti(a publicationRoot, ops []Operation, revision string) (derivation, error) {
	var pub *publication.Publication
	base := map[string][]byte{}
	current := map[string][]byte{}
	touched := map[string]bool{}
	targets := map[string]bool{}
	loadBase := func(path string) ([]byte, error) {
		if b, ok := base[path]; ok {
			return b, nil
		}
		bp, err := bookpath.Parse(path)
		if err != nil {
			return nil, err
		}
		b, err := a.Read(bp, publication.XMLLimit)
		if err != nil {
			return nil, err
		}
		base[path] = b
		return b, nil
	}
	for _, op := range ops {
		switch op.ID {
		case "metadata.set":
			param := op.Params.(metadata.Set)
			b, err := loadBase(w.state.Rootfile)
			if err != nil {
				return derivation{}, err
			}
			location, err := metadata.Select(b, param)
			if err != nil {
				return derivation{}, err
			}
			key := "metadata\x00" + location
			if targets[key] {
				return derivation{}, fmt.Errorf("duplicate metadata target %s", location)
			}
			targets[key] = true
			if !touched[w.state.Rootfile] {
				out, _, err := metadata.Apply(b, param)
				if err != nil {
					return derivation{}, err
				}
				current[w.state.Rootfile] = out
				touched[w.state.Rootfile] = true
				continue
			}
			// Revalidate against the frozen baseline before applying to the
			// evolving bytes: an earlier operation must not relax this
			// operation's expected old value.
			if _, _, err := metadata.Apply(b, param); err != nil {
				return derivation{}, err
			}
			out, _, err := metadata.Apply(current[w.state.Rootfile], param)
			if err != nil {
				return derivation{}, err
			}
			current[w.state.Rootfile] = out
		case "content.text.set":
			param := op.Params.(publication.TextSet)
			if param.RevisionID != revision {
				return derivation{}, ErrStalePlan
			}
			if pub == nil {
				p, err := publication.Load(a, w.state.Rootfile)
				if err != nil {
					return derivation{}, err
				}
				pub = p
			}
			path := string(param.BookPath)
			b, err := loadBase(path)
			if err != nil {
				return derivation{}, err
			}
			if err := publication.CheckTextResourceHash(b, param); err != nil {
				return derivation{}, err
			}
			key := "content\x00" + path + "\x00" + param.Locator
			if targets[key] {
				return derivation{}, fmt.Errorf("duplicate content target %s in %s", param.Locator, path)
			}
			targets[key] = true
			if !touched[path] {
				out, _, err := publication.ApplyTextAt(b, pub, param)
				if err != nil {
					return derivation{}, err
				}
				current[path] = out
				touched[path] = true
				continue
			}
			if _, _, err := publication.ApplyTextAt(b, pub, param); err != nil {
				return derivation{}, err
			}
			out, _, err := publication.ApplyTextAt(current[path], pub, param)
			if err != nil {
				return derivation{}, err
			}
			current[path] = out
		default:
			return derivation{}, fmt.Errorf("unsupported operation")
		}
	}
	outputs := map[string][]byte{}
	for path, out := range current {
		if !bytes.Equal(out, base[path]) {
			outputs[path] = out
		}
	}
	writes := make([]string, 0, len(outputs))
	for path := range outputs {
		writes = append(writes, path)
	}
	slices.Sort(writes)
	return derivation{outputs: outputs, writes: writes}, nil
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
	var probe struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	_ = json.Unmarshal(requestJSON, &probe)
	version := 0
	var proposal *fix.Proposal
	var operations []Operation
	if probe.SchemaVersion == 7 {
		var request fixRequest
		if err := decodeStrict(requestJSON, &request); err != nil {
			return Plan{}, err
		}
		if err := w.validateFixProposal(request.Proposal); err != nil {
			return Plan{}, err
		}
		if !fix.OperationMultiset(fixOperations(request.Operations), request.Proposal.Derived.Operations) {
			return Plan{}, fault.New(2, "INVALID_OPERATIONS", "request operations do not match the proposal selection")
		}
		version, proposal, operations = 7, request.Proposal, request.Operations
	} else {
		var request Request
		if err := decodeStrict(requestJSON, &request); err != nil {
			return Plan{}, err
		}
		v, err := operationSchema(request.Operations)
		if err != nil {
			return Plan{}, err
		}
		if request.SchemaVersion != v {
			return Plan{}, fmt.Errorf("unsupported request version")
		}
		version, operations = v, request.Operations
	}
	d, err := w.recompute(operations)
	if err != nil {
		return Plan{}, err
	}
	p := Plan{
		SchemaVersion:      version,
		ID:                 randomID(),
		WorkspaceID:        w.id,
		WorkspacePath:      w.dir,
		BaseRevision:       w.current,
		InputTreeSHA256:    w.base.SHA256,
		OperationSetSHA256: digest(operations),
		PolicySHA256:       digest(policyFor(version)),
		Rootfile:           w.state.Rootfile,
		Proposal:           proposal,
		Operations:         operations,
		WriteSet:           d.writes,
		Applicable:         true,
	}
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

// verifyPlanEnvelope checks identity, frozen baseline and saved proposal source.
// It does not prove the operation effects; those still require recomputation.
func (w *Workspace) verifyPlanEnvelope(p Plan, bindPath bool) error {
	if err := w.ensureIdentity(); err != nil {
		return err
	}
	if err := w.verifyBaseline(); err != nil {
		return errors.Join(ErrStalePlan, err)
	}
	if !validPlanOperation(p) || !validID(p.ID) || p.WriteSet == nil || p.WorkspaceID != w.id || bindPath && p.WorkspacePath != w.dir || p.BaseRevision != w.current || p.InputTreeSHA256 != w.base.SHA256 || p.Rootfile != w.state.Rootfile || p.OperationSetSHA256 != digest(p.Operations) {
		return ErrStalePlan
	}
	var stored Plan
	if err := readEditJSON(w.root, "plans/"+p.ID+".json", &stored); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrStalePlan
		}
		return err
	}
	if digest(stored) != digest(p) {
		return ErrStalePlan
	}
	if err := w.verifyFixPlan(p); err != nil {
		return err
	}
	if err := w.verifyBaseline(); err != nil {
		return errors.Join(ErrStalePlan, err)
	}
	return nil
}

func (w *Workspace) verifyPlan(p Plan, bindPath bool) (derivation, error) {
	if planReferenceVersion(p) != publication.ReferenceParserVersion {
		return derivation{}, fmt.Errorf("%w: legacy reference policy requires a new plan", ErrStalePlan)
	}
	if planTargetVersion(p) != 2 {
		return derivation{}, fmt.Errorf("%w: legacy target policy requires a new plan", ErrStalePlan)
	}
	if err := w.verifyPlanEnvelope(p, bindPath); err != nil {
		return derivation{}, err
	}
	d, err := w.recompute(p.Operations)
	if err != nil {
		return derivation{}, err
	}
	if !p.Applicable || !slices.Equal(d.writes, p.WriteSet) {
		return derivation{}, ErrStalePlan
	}
	return d, nil
}

// verifyFixPlan re-derives the complete schema 7 proposal source from the
// plan's own frozen baseline and checks the request operation multiset. Every
// path that consumes a stored plan uses it, so a forged source cannot be
// trusted just because its digest matches a stored copy.
func (w *Workspace) verifyFixPlan(p Plan) error {
	if p.SchemaVersion != 7 {
		return nil
	}
	if p.Proposal == nil || p.Proposal.ProposalVersion != fix.ProposalVersion || p.Proposal.Workspace.BaseRevision != p.BaseRevision {
		return ErrStalePlan
	}
	tree, err := HashTree(filepath.Join(w.dir, filepath.FromSlash(revisionPath(p.BaseRevision))))
	if err != nil {
		return errors.Join(ErrStalePlan, err)
	}
	s, err := w.fixSnapshotAt(revisionPath(p.BaseRevision))
	if err != nil {
		return errors.Join(ErrStalePlan, err)
	}
	s.Workspace.BaseRevision = p.BaseRevision
	s.Workspace.InputTreeSHA256 = tree.SHA256
	s.TargetVersion = planTargetVersion(p)
	if err := fix.Validate(s, *p.Proposal); err != nil {
		return errors.Join(ErrStalePlan, err)
	}
	if err := fix.RequestAllowed(*p.Proposal); err != nil {
		return errors.Join(ErrStalePlan, err)
	}
	if !fix.OperationMultiset(fixOperations(p.Operations), p.Proposal.Derived.Operations) {
		return ErrStalePlan
	}
	return nil
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
	d, err := w.verifyPlan(p, true)
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
	return w.execute(e, d.outputs, nil)
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
// All planned resources are written in sorted order; any failure rolls the
// whole candidate back through its checkpoint, never leaving a partial book.
func (w *Workspace) execute(e Execution, outputs map[string][]byte, hook func() error) (Execution, error) {
	var err error
	if len(outputs) != len(e.Plan.WriteSet) {
		err = fmt.Errorf("derived output set does not match plan")
	}
	for _, target := range e.Plan.WriteSet {
		if err != nil {
			break
		}
		out, ok := outputs[target]
		if !ok {
			err = fmt.Errorf("missing derived output for %s", target)
			break
		}
		err = w.writeCandidateFile(target, out)
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
			// Verify exact content for every planned resource, not merely the
			// set of changed filenames.
			for _, target := range e.Plan.WriteSet {
				if err != nil {
					break
				}
				out := outputs[target]
				h := sha256.Sum256(out)
				seen := false
				for _, entry := range actual.Entries {
					if entry.Path != target {
						continue
					}
					seen = true
					if entry.Type != "file" || entry.Size != int64(len(out)) || entry.SHA256 != hex.EncodeToString(h[:]) {
						err = fmt.Errorf("candidate content mismatch")
					}
				}
				if err == nil && !seen {
					err = fmt.Errorf("candidate target %s is missing", target)
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

// writeCandidateFile replaces one candidate resource through a same-directory
// temporary file and rename. Both the temporary write and the replacement are
// anchored to the opened target parent, never resolved a second time.
func (w *Workspace) writeCandidateFile(target string, out []byte) error {
	r, err := subdir(w.root, candidate+"/"+path.Dir(target))
	if err != nil {
		return err
	}
	defer r.Close()
	f, err := openRegular(r, path.Base(target))
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	name := ".kepub-edit-" + randomID()
	f, err = r.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(out)
	err = errors.Join(err, f.Sync(), f.Close())
	if err == nil {
		err = r.Rename(name, path.Base(target))
	}
	return errors.Join(err, r.RemoveAll(name), syncDir(r, "."))
}

// expectedDiff predicts the candidate tree from the complete derived output set.
// It must describe every planned write, not only the first resource.
func expectedDiff(base Tree, outputs map[string][]byte) Diff {
	after := Tree{Entries: slices.Clone(base.Entries)}
	for i, entry := range after.Entries {
		out, ok := outputs[entry.Path]
		if !ok {
			continue
		}
		h := sha256.Sum256(out)
		after.Entries[i] = Entry{entry.Path, "file", int64(len(out)), hex.EncodeToString(h[:])}
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
	if err := w.verifyPlanEnvelope(intent, false); err != nil {
		return e, err
	}
	d, deriveErr := w.recomputeAt(intent.Operations, revisionPath(w.current), w.current, planReferenceVersion(intent))
	if deriveErr != nil {
		var f *fault.Error
		if !errors.As(deriveErr, &f) || f.Code != "RESOURCE_LIMIT" {
			return e, deriveErr
		}
	}
	if !intent.Applicable || deriveErr == nil && !slices.Equal(d.writes, intent.WriteSet) {
		return e, ErrStalePlan
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
		if deriveErr != nil {
			return e, deriveErr
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
	// Only a verified, interrupted mutation can be rolled back on a budget
	// failure. Never rewrite a completed result or discard unstarted drift.
	if deriveErr != nil && (start.Status != "running" || exists(w.root, "tasks/active/edit-result.json")) {
		return e, deriveErr
	}
	if !exists(w.root, "tasks/active/edit-result.json") {
		// Interrupted mutation is rolled back, not silently rerun as a new task.
		actual, err := hashAt(w.root, candidate)
		if err != nil {
			return e, errors.Join(deriveErr, err)
		}
		e = start
		e.Diff = compareTrees(w.base, actual)
		if start.Status == "running" {
			if err := w.restore(start.Checkpoint); err != nil {
				return e, errors.Join(deriveErr, err)
			}
		}
		e.Status = "failed"
		e.Failure = "interrupted apply"
		if deriveErr != nil {
			e.Failure = deriveErr.Error()
		}
		if err := writeJSON(w.root, "tasks/active/edit-result.json", e); err != nil {
			return e, errors.Join(deriveErr, err)
		}
		if err := syncDir(w.root, "tasks/active"); err != nil {
			return e, errors.Join(deriveErr, err)
		}
		if deriveErr != nil {
			// Restoration is not proof of effects. Later execution, rejection
			// and history consumption must still perform full recomputation.
			return e, deriveErr
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
		if digest(e.Diff) != digest(expectedDiff(w.base, d.outputs)) {
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
