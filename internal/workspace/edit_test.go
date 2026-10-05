package workspace

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/LeviTK/Kepub/internal/metadata"
)

func editJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func setRequest(new string) Request {
	return Request{1, []Operation{{"metadata.set", 1, metadata.Set{Namespace: metadata.DC, LocalName: "title", ID: "t", ExpectedOldValue: "测试 & Space", NewValue: new}}}}
}
func planTitle(t *testing.T, w *Workspace, new string) Plan {
	t.Helper()
	p, err := w.Plan(editJSON(t, setRequest(new)))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMetadataV1CanonicalBaselineEvidence(t *testing.T) {
	// Recorded with the unmodified 06474154 baseline binary, not regenerated
	// from the implementation under test. Optional id is absent, order fixed.
	const encoded = `[{"operationId":"metadata.set","operationVersion":1,"params":{"namespace":"http://purl.org/dc/elements/1.1/","localName":"title","expectedOldValue":"Title","newValue":"Legacy Title"}}]`
	ops := []Operation{{"metadata.set", 1, metadata.Set{Namespace: metadata.DC, LocalName: "title", ExpectedOldValue: "Title", NewValue: "Legacy Title"}}}
	if string(editJSON(t, ops)) != encoded || digest(ops) != "9b544b3cf576ef62d5b671ed32118c7edc4192121e08ef86bf891f4d22a35891" || digest(editPolicy) != "305b43d37304f964e79396386309a865e10a0f4516873c961d47015e45612935" {
		t.Fatal("legacy canonical encoding/policy changed")
	}
}

func TestPlanApplyDiffReopenAndIsolation(t *testing.T) {
	for _, new := range []string{"新 < & >", "测试 & Space"} {
		t.Run(new, func(t *testing.T) {
			w, dir := makeWorkspace(t)
			initial := w.State().Tree
			before, err := os.ReadFile(filepath.Join(dir, revision, w.state.Rootfile))
			if err != nil {
				t.Fatal(err)
			}
			p := planTitle(t, w, new)
			if exists(w.root, "tasks/active") || treeAt(t, filepath.Join(dir, revision)).SHA256 != initial.SHA256 {
				t.Fatal("plan mutated publication")
			}
			id, err := w.ID()
			if err != nil || p.WorkspaceID != id || p.WorkspacePath != dir {
				t.Fatalf("identity %s: %v", id, err)
			}
			if _, err := Open(dir); !errors.Is(err, ErrBusy) {
				t.Fatalf("writer lock: %v", err)
			}
			w.Close()
			w, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			e, err := w.Apply(editJSON(t, p))
			if err != nil {
				t.Fatal(err)
			}
			wantChanged := new != "测试 & Space"
			if e.Status != "review_required" || !e.ReviewRequired || e.Conformance != "not_run" || e.Diff.Changed != wantChanged {
				t.Fatalf("result %+v", e)
			}
			d, err := w.Diff()
			if err != nil || !reflect.DeepEqual(d, e.Diff) {
				t.Fatalf("diff: %+v %v", d, err)
			}
			if wantChanged {
				if len(d.Changes) != 1 || d.Changes[0].Path != p.Rootfile || d.Changes[0].Kind != "modified" {
					t.Fatalf("write set: %+v", d)
				}
				want := bytes.Replace(before, []byte(`>测试 &amp; Space</dc:title>`), []byte(`>新 &lt; &amp; &gt;</dc:title>`), 1)
				assertBytes(t, filepath.Join(dir, candidate, p.Rootfile), want)
			} else {
				assertBytes(t, filepath.Join(dir, candidate, p.Rootfile), before)
			}
			assertBytes(t, filepath.Join(dir, revision, p.Rootfile), before)
			assertBytes(t, filepath.Join(dir, checkpointDir(e.Checkpoint), "pub", p.Rootfile), before)
			if err := w.verifyBaseline(); err != nil {
				t.Fatal(err)
			}
			if _, err := w.Apply(editJSON(t, p)); !errors.Is(err, ErrCandidateConflict) {
				t.Fatalf("reapply: %v", err)
			}
			w.Close()
			w, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			got, err := w.Execution()
			if err != nil || !reflect.DeepEqual(got, e) {
				t.Fatalf("reopened: %+v %v", got, err)
			}
			// Ordinary truncation must not affect initial/checkpoint/original.
			put(t, filepath.Join(dir, candidate, p.Rootfile), []byte("candidate-only"))
			assertBytes(t, filepath.Join(dir, revision, p.Rootfile), before)
			assertBytes(t, filepath.Join(dir, checkpointDir(e.Checkpoint), "pub", p.Rootfile), before)
			if err := w.verifyBaseline(); err != nil {
				t.Fatal(err)
			}
			if _, err := w.Execution(); err == nil {
				t.Fatal("changed candidate inherited review record")
			}
			w.Close()
			opened, err := Open(dir)
			if err != nil {
				t.Fatalf("changed candidate must remain reviewable: %v", err)
			}
			defer opened.Close()
			if _, err := opened.Execution(); !errors.Is(err, ErrCandidateDrift) {
				t.Fatalf("reopened candidate inherited review record: %v", err)
			}
		})
	}
}

func TestRequestsAndPlansRejectTampering(t *testing.T) {
	w, dir := makeWorkspace(t)
	valid := string(editJSON(t, setRequest("New")))
	for _, b := range []string{
		strings.Replace(valid, `"schemaVersion":1`, `"schemaVersion":2`, 1),
		strings.Replace(valid, `"operationVersion":1`, `"operationVersion":2`, 1),
		strings.Replace(valid, `metadata.set`, `resource.rename`, 1),
		strings.Replace(valid, `"newValue":"New"`, `"newValue":"New","extra":1`, 1),
		strings.Replace(valid, `"expectedOldValue":"测试 \u0026 Space",`, ``, 1),
		strings.Replace(valid, `"schemaVersion":1`, `"schemaVersion":1,"schemaVersion":1`, 1),
		strings.Replace(valid, `"newValue":"New"`, `"newValue":null`, 1),
		valid + ` {}`, `{"schemaVersion":1,"operations":[]}`,
		string(editJSON(t, Request{1, append(setRequest("New").Operations, setRequest("Other").Operations...)})),
	} {
		if _, err := w.Plan([]byte(b)); err == nil {
			t.Fatalf("accepted bad request: %s", b)
		}
	}
	p := planTitle(t, w, "New")
	changes := []func(*Plan){
		func(p *Plan) { p.SchemaVersion = 2 }, func(p *Plan) { p.WorkspaceID = randomID() }, func(p *Plan) { p.WorkspacePath += "/else" }, func(p *Plan) { p.BaseRevision = "later" }, func(p *Plan) { p.InputTreeSHA256 = digest("wrong") }, func(p *Plan) { p.PolicySHA256 = digest("wrong") }, func(p *Plan) { p.Rootfile = "mimetype" }, func(p *Plan) { p.WriteSet = []string{} }, func(p *Plan) { p.Applicable = false }, func(p *Plan) {
			p.Operations = slices.Clone(p.Operations)
			param := p.Operations[0].Params.(metadata.Set)
			param.NewValue = "Altered"
			p.Operations[0].Params = param
			p.OperationSetSHA256 = digest(p.Operations)
		},
	}
	for i, change := range changes {
		altered := p
		change(&altered)
		if _, err := w.Apply(editJSON(t, altered)); err == nil {
			t.Fatalf("accepted alteration %d", i)
		}
		if exists(w.root, "tasks/active") {
			t.Fatal("rejected plan created candidate")
		}
	}
	// Even rewriting the stored writeSet/applicable cannot override recomputation.
	altered := p
	altered.WriteSet = []string{"mimetype"}
	put(t, filepath.Join(dir, "plans", p.ID+".json"), editJSON(t, altered))
	if _, err := w.Apply(editJSON(t, altered)); err == nil {
		t.Fatal("trusted stored forged write set")
	}
	put(t, filepath.Join(dir, "plans", p.ID+".json"), editJSON(t, p))
	put(t, filepath.Join(dir, revision, "unlisted.bin"), []byte("baseline drift"))
	if _, err := w.Apply(editJSON(t, p)); !errors.Is(err, ErrStalePlan) {
		t.Fatalf("baseline drift: %v", err)
	}
}
func prepareExecution(t *testing.T, w *Workspace, p Plan) (Execution, []byte) {
	t.Helper()
	out, err := w.verifyPlan(p, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.createCandidate(&p); err != nil {
		t.Fatal(err)
	}
	s, err := w.checkpoint()
	if err != nil {
		t.Fatal(err)
	}
	task, err := w.TaskID()
	if err != nil {
		t.Fatal(err)
	}
	e := Execution{Version: p.SchemaVersion, TaskID: task, Plan: p, Checkpoint: s.ID, Status: "running", Conformance: "not_run", Diff: compareTrees(s.Tree, s.Tree)}
	if err := writeJSON(w.root, "tasks/active/edit-start.json", e); err != nil {
		t.Fatal(err)
	}
	return e, out
}

func TestPublishedTaskStartupFailureRetainsID(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission failure requires non-root user")
	}
	for _, blocked := range []string{"tasks/active/checkpoints", "tasks/active"} {
		t.Run(blocked, func(t *testing.T) {
			w, dir := makeWorkspace(t)
			p := planTitle(t, w, "startup failure")
			if _, err := w.createCandidate(&p); err != nil {
				t.Fatal(err)
			}
			id, err := w.TaskID()
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, blocked)
			if err := os.Chmod(path, 0500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(path, 0700) })
			e, err := w.startExecution(p)
			if err == nil || e.TaskID != id || digest(e.Plan) != digest(p) {
				t.Fatalf("startup result: %+v, %v; durable ID %s", e, err, id)
			}
			if err := os.Chmod(path, 0700); err != nil {
				t.Fatal(err)
			}
			w.Close()
			w, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			d, err := w.TaskDiff(e.TaskID)
			if err != nil || d.Diff.Changed {
				t.Fatalf("recovery diff: %+v, %v", d, err)
			}
			if _, err := w.Reject(e.TaskID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestActualWriteSetFailureRollbackAndReopen(t *testing.T) {
	for _, scenario := range []string{"add", "delete", "type", "wrong-opf", "io"} {
		t.Run(scenario, func(t *testing.T) {
			w, dir := makeWorkspace(t)
			p := planTitle(t, w, "New")
			e, out := prepareExecution(t, w, p)
			e, err := w.execute(e, out, func() error {
				switch scenario {
				case "add":
					put(t, filepath.Join(dir, candidate, "new-file.txt"), []byte("unexpected"))
				case "delete":
					return os.Remove(filepath.Join(dir, candidate, "unlisted.bin"))
				case "type":
					if err := os.Remove(filepath.Join(dir, candidate, "unlisted.bin")); err != nil {
						return err
					}
					return os.Mkdir(filepath.Join(dir, candidate, "unlisted.bin"), 0700)
				case "wrong-opf":
					put(t, filepath.Join(dir, candidate, p.Rootfile), []byte("same filename wrong bytes"))
				case "io":
					return errors.New("injected write failure")
				}
				return nil
			})
			if err == nil || e.Status != "failed" || e.ReviewRequired || e.Failure == "" {
				t.Fatalf("failed apply: %+v %v", e, err)
			}
			if treeAt(t, filepath.Join(dir, candidate)).SHA256 != w.state.Tree.SHA256 {
				t.Fatal("rollback lost baseline")
			}
			if err := w.verifyBaseline(); err != nil {
				t.Fatal(err)
			}
			w.Close()
			w, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			got, err := w.Execution()
			if err != nil || got.Status != "failed" || got.Failure != e.Failure {
				t.Fatalf("failure record lost: %+v %v", got, err)
			}
			if _, err := w.Apply(editJSON(t, p)); !errors.Is(err, ErrCandidateConflict) {
				t.Fatalf("rerun failed task: %v", err)
			}
		})
	}
}

func TestInterruptedApplyRollsBackNotRerun(t *testing.T) {
	for _, phase := range []string{"intent", "after-checkpoint", "after-write"} {
		t.Run(phase, func(t *testing.T) {
			w, dir := makeWorkspace(t)
			p := planTitle(t, w, "New")
			if phase == "intent" {
				if _, err := w.createCandidate(&p); err != nil {
					t.Fatal(err)
				}
			} else {
				_, out := prepareExecution(t, w, p)
				if phase == "after-write" {
					put(t, filepath.Join(dir, candidate, p.Rootfile), out)
					put(t, filepath.Join(dir, candidate, "extra"), []byte("unplanned"))
				}
			}
			w.Close()
			w, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			e, err := w.Execution()
			if err != nil || e.Status != "failed" || e.Failure != "interrupted apply" || e.ReviewRequired {
				t.Fatalf("interrupted: %+v %v", e, err)
			}
			if treeAt(t, filepath.Join(dir, candidate)).SHA256 != w.state.Tree.SHA256 {
				t.Fatal("interrupted mutation survived")
			}
			if _, err := w.Apply(editJSON(t, p)); !errors.Is(err, ErrCandidateConflict) {
				t.Fatalf("retry: %v", err)
			}
			w.Close()
			w, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
		})
	}
}

func TestDiffObservesAddsDeletesAndTypeChanges(t *testing.T) {
	w, dir := makeWorkspace(t)
	pub, err := w.NewCandidate()
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(pub, "new.txt"), []byte("unique-added-bytes"))
	if err := os.Remove(filepath.Join(pub, "mimetype")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(pub, "unlisted.bin")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(pub, "unlisted.bin"), 0700); err != nil {
		t.Fatal(err)
	}
	d, err := w.Diff()
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Changes) != 3 || d.Changes[0].Path != "mimetype" || d.Changes[0].Kind != "deleted" || d.Changes[1].Path != "new.txt" || d.Changes[1].Kind != "added" || d.Changes[1].After.Size != 18 || d.Changes[2].Path != "unlisted.bin" || d.Changes[2].Before.Type != "file" || d.Changes[2].After.Type != "directory" {
		t.Fatalf("actual diff: %+v", d)
	}
	if treeAt(t, filepath.Join(dir, revision)).SHA256 != w.state.Tree.SHA256 {
		t.Fatal("diff changed baseline")
	}
}

func TestConcurrentApplySingleCandidate(t *testing.T) {
	w, _ := makeWorkspace(t)
	p := planTitle(t, w, "New")
	b := editJSON(t, p)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() { _, err := w.Apply(b); results <- err })
	}
	wg.Wait()
	close(results)
	ok, conflict := 0, 0
	for err := range results {
		if err == nil {
			ok++
		} else if errors.Is(err, ErrCandidateConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if ok != 1 || conflict != 1 {
		t.Fatalf("race outcomes: %d %d", ok, conflict)
	}
}

func TestIdentityLegacyMigrationAndPlanDirectoryBinding(t *testing.T) {
	w, dir := makeWorkspace(t)
	w.Close()
	if err := os.Remove(filepath.Join(dir, "identity.json")); err != nil {
		t.Fatal(err)
	}
	w, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if exists(w.root, "identity.json") {
		t.Fatal("legacy Open silently migrated identity")
	}
	p := planTitle(t, w, "New")
	id, err := w.ID()
	if err != nil || id != p.WorkspaceID || !validID(id) {
		t.Fatalf("migration: %s %v", id, err)
	}
	w.Close()
	moved := dir + "-moved"
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	w, err = Open(moved)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if _, err := w.Apply(editJSON(t, p)); !errors.Is(err, ErrStalePlan) {
		t.Fatalf("moved directory: %v", err)
	}
	if err := os.Remove(filepath.Join(moved, "identity.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.ID(); err == nil {
		t.Fatal("lost identity regenerated")
	}
}

func TestForgedExecutionCannotClaimReview(t *testing.T) {
	for _, scenario := range []string{"extra-file", "wrong-opf"} {
		t.Run(scenario, func(t *testing.T) {
			w, dir := makeWorkspace(t)
			p := planTitle(t, w, "New")
			e, err := w.Apply(editJSON(t, p))
			if err != nil {
				t.Fatal(err)
			}
			// Forge a self-consistent diff, even retaining the correct changed path.
			if scenario == "extra-file" {
				put(t, filepath.Join(dir, candidate, "extra"), []byte("forged"))
			} else {
				put(t, filepath.Join(dir, candidate, p.Rootfile), []byte("forged OPF bytes"))
			}
			e.Diff, err = w.Diff()
			if err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(dir, "tasks/active/edit-result.json"), editJSON(t, e))
			w.Close()
			if opened, err := Open(dir); err == nil {
				opened.Close()
				t.Fatal("trusted forged review report")
			}
		})
	}
}

func TestReportFailureAlsoRollsBack(t *testing.T) {
	w, dir := makeWorkspace(t)
	p := planTitle(t, w, "New")
	e, out := prepareExecution(t, w, p)
	collision := filepath.Join(dir, "tasks/active/edit-result.json")
	if err := os.Mkdir(collision, 0700); err != nil {
		t.Fatal(err)
	}
	e, err := w.execute(e, out, nil)
	if err == nil || e.Status != "failed" || e.ReviewRequired || e.Failure == "" {
		t.Fatalf("report failure: %+v %v", e, err)
	}
	if treeAt(t, filepath.Join(dir, candidate)).SHA256 != w.state.Tree.SHA256 {
		t.Fatal("recording failure left candidate edits")
	}
	w.Close()
	if opened, err := Open(dir); err == nil {
		opened.Close()
		t.Fatal("untrusted collision record accepted")
	}
	// Only the test removes its own injected collision. The library never does.
	if err := os.Remove(collision); err != nil {
		t.Fatal(err)
	}
	w, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	e, err = w.Execution()
	if err != nil || e.Status != "failed" {
		t.Fatalf("recovery after record failure: %+v %v", e, err)
	}
}

func TestDiffDetectsSameSizeDifferentSHA(t *testing.T) {
	w, _ := makeWorkspace(t)
	pub, err := w.NewCandidate()
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(pub, "unlisted.bin"), []byte{1, 255, 13, 10, 42})
	d, err := w.Diff()
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Changes) != 1 || d.Changes[0].Kind != "modified" || d.Changes[0].Before.Size != 5 || d.Changes[0].After.Size != 5 || d.Changes[0].Before.SHA256 == d.Changes[0].After.SHA256 {
		t.Fatalf("same-size changed bytes missed: %+v", d)
	}
}

func TestNoOpNullWriteSetRejected(t *testing.T) {
	w, dir := makeWorkspace(t)
	p := planTitle(t, w, "测试 & Space")
	p.WriteSet = nil
	put(t, filepath.Join(dir, "plans", p.ID+".json"), editJSON(t, p))
	if _, err := w.Apply(editJSON(t, p)); !errors.Is(err, ErrStalePlan) {
		t.Fatalf("null write set accepted as empty array: %v", err)
	}
}
