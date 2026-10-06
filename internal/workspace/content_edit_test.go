package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/validation"
)

func contentRequest(t *testing.T, w *Workspace, bp, new string) Request {
	t.Helper()
	a, _, r, err := w.AcceptedSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	p, err := publication.Load(a, w.state.Rootfile)
	if err != nil {
		t.Fatal(err)
	}
	c, err := publication.ReadContent(a, p, bookpath.BookPath(bp), publication.ContentOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range c.Nodes {
		if n.LocalName == "p" {
			return Request{2, []Operation{{"content.text.set", 1, publication.TextSet{BookPath: c.BookPath, RevisionID: r.ID, ResourceSHA256: c.ResourceSHA256, LocatorVersion: c.LocatorVersion, Locator: n.Locator, ExpectedOldValue: n.Text, NewValue: new}}}}
		}
	}
	t.Fatal("missing p")
	return Request{}
}

func contentPlan(t *testing.T, w *Workspace, new string) Plan {
	t.Helper()
	p, err := w.Plan(editJSON(t, contentRequest(t, w, "EPUB/chapter.xhtml", new)))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestContentEditRequestStrictAndCanonical(t *testing.T) {
	w, _, _ := legalWorkspace(t, "3.0")
	defer w.Close()
	r := contentRequest(t, w, "EPUB/chapter.xhtml", "new")
	b := editJSON(t, r)
	// Independent canonical key order and digest of the frozen policy string.
	var p publication.TextSet = r.Operations[0].Params.(publication.TextSet)
	want := `{"bookPath":"EPUB/chapter.xhtml","revisionId":"initial","resourceSha256":"` + p.ResourceSHA256 + `","locatorVersion":1,"locator":"/html[1]/body[1]/p[1]","expectedOldValue":"Original \u0026 precise.","newValue":"new"}`
	if string(editJSON(t, p)) != want {
		t.Fatalf("canonical params: %s", editJSON(t, p))
	}
	if digest(contentEditPolicy) != "8989bb59b95abd9a0434922f62ae7592009a139043a7e0e41ddae8be793d48fd" {
		t.Fatal("frozen content policy digest changed")
	}
	for _, field := range []string{"bookPath", "revisionId", "resourceSha256", "locatorVersion", "locator", "expectedOldValue", "newValue"} {
		var raw map[string]any
		json.Unmarshal(b, &raw)
		params := raw["operations"].([]any)[0].(map[string]any)["params"].(map[string]any)
		delete(params, field)
		missing, _ := json.Marshal(raw)
		if _, err := w.Plan(missing); err == nil {
			t.Fatal("missing field allowed", field)
		}
		params[field] = nil
		null, _ := json.Marshal(raw)
		if _, err := w.Plan(null); err == nil {
			t.Fatal("null allowed", field)
		}
	}
	for _, bad := range []string{
		strings.Replace(string(b), `"newValue":"new"`, `"newValue":"new","newValue":"new"`, 1),
		strings.Replace(string(b), `"newValue":"new"`, `"newValue":"new","id":"t"`, 1),
		strings.Replace(string(b), `"newValue":"new"`, `"newValue":"new","offset":42`, 1),
		strings.Replace(string(b), `"schemaVersion":2`, `"schemaVersion":1`, 1),
		strings.Replace(string(b), `"operationVersion":1`, `"operationVersion":2`, 1),
		strings.Replace(string(b), `"content.text.set"`, `"metadata.set"`, 1),
		strings.Replace(string(editJSON(t, setRequest("New"))), `"schemaVersion":1`, `"schemaVersion":2`, 1),
		string(b) + string(b), string(b) + string([]byte{255}),
	} {
		if _, err := w.Plan([]byte(bad)); err == nil {
			t.Fatal("invalid request accepted", bad)
		}
	}
	r.Operations = append(r.Operations, r.Operations[0])
	if _, err := w.Plan(editJSON(t, r)); err == nil {
		t.Fatal("two operations accepted")
	}
	for _, change := range []func(*publication.TextSet){func(p *publication.TextSet) { p.RevisionID = randomID() }, func(p *publication.TextSet) { p.ResourceSHA256 = strings.Repeat("0", 64) }} {
		p := r.Operations[0].Params.(publication.TextSet)
		change(&p)
		req := Request{2, []Operation{{"content.text.set", 1, p}}}
		_, err := w.Plan(editJSON(t, req))
		var f *fault.Error
		if !errors.Is(err, ErrStalePlan) && (!errors.As(err, &f) || f.Code != "INPUT_DRIFT") {
			t.Fatal("stale content binding", err)
		}
	}
	if exists(w.root, "tasks/active") {
		t.Fatal("invalid plan created candidate")
	}
}

func TestContentEditUnicodeTargetAndNoOp(t *testing.T) {
	for _, new := range []string{"新 < & > 😀\r\n", "第二章"} {
		w, dir := makeWorkspace(t)
		req := contentRequest(t, w, "书/Text/第二 章.xhtml", new)
		p, err := w.Plan(editJSON(t, req))
		if err != nil {
			t.Fatal(err)
		}
		if p.SchemaVersion != 2 || p.Rootfile != "书/Deep/package.opf" || !slices.Equal(p.WriteSet, func() []string {
			if new == "第二章" {
				return []string{}
			}
			return []string{"书/Text/第二 章.xhtml"}
		}()) {
			t.Fatal("plan path/version", p)
		}
		e := applyPlan(t, w, p)
		if e.Version != 2 || e.Diff.Changed != (new != "第二章") {
			t.Fatal("execution", e)
		}
		d, err := w.TaskDiff(e.TaskID)
		if err != nil || d.Content == nil || d.Content.NewValue == nil || *d.Content.NewValue != new || d.Content.OldValue != "第二章" || !d.MatchesExecution {
			t.Fatal("review", d, err)
		}
		bp := "书/Text/第二 章.xhtml"
		before, err := os.ReadFile(filepath.Join(dir, revision, bp))
		if err != nil {
			t.Fatal(err)
		}
		want := before
		if new != "第二章" {
			want = bytes.Replace(before, []byte(`>第二章</p>`), []byte(`>新 &lt; &amp; &gt; 😀&#xD;&#xA;</p>`), 1)
		}
		assertBytes(t, filepath.Join(dir, candidate, bp), want)
		w.Close()
		w, err = Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Execution(); err != nil {
			t.Fatal("reopen", err)
		}
		put(t, filepath.Join(dir, candidate, bp), bytes.Replace(want, []byte(`</p>`), []byte(`<em>tamper</em></p>`), 1))
		d, err = w.TaskDiff(e.TaskID)
		if err != nil || d.MatchesExecution || d.Content.NewValue != nil || d.Content.Unavailable == "" {
			t.Fatal("actual unavailable review", d, err)
		}
		if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{}); !errors.Is(err, ErrCandidateDrift) {
			t.Fatal("drift accepted", err)
		}
		if _, err := w.Reject(e.TaskID); err != nil {
			t.Fatal("drift not rejectable", err)
		}
		w.Close()
	}
}

func TestContentEditPlanExecutionTamperAndRollback(t *testing.T) {
	for _, scenario := range []string{"policy", "schema", "writeSet", "params", "execution-version", "execution-hash", "wrong-content", "other-file", "interrupted", "missing-start"} {
		t.Run(scenario, func(t *testing.T) {
			w, dir, _ := legalWorkspace(t, "3.0")
			p := contentPlan(t, w, "New & 😀")
			defer func() { w.Close() }()
			if scenario == "policy" || scenario == "schema" || scenario == "writeSet" || scenario == "params" {
				switch scenario {
				case "policy":
					p.PolicySHA256 = digest(editPolicy)
				case "schema":
					p.SchemaVersion = 1
				case "writeSet":
					p.WriteSet = []string{p.Rootfile}
				case "params":
					s := p.Operations[0].Params.(publication.TextSet)
					s.ExpectedOldValue = "Forged"
					p.Operations[0].Params = s
				}
				// Alter the persisted plan as well: rejection must be derived, not
				// merely equality between the caller and disk copy.
				p.OperationSetSHA256 = digest(p.Operations)
				put(t, filepath.Join(dir, "plans", p.ID+".json"), editJSON(t, p))
				if _, err := w.Apply(editJSON(t, p)); err == nil {
					t.Fatal("forged plan accepted")
				}
				return
			}
			if scenario == "execution-version" || scenario == "execution-hash" {
				e := applyPlan(t, w, p)
				if scenario == "execution-version" {
					e.Version = 1
				} else {
					e.Diff.Changes[0].After.SHA256 = strings.Repeat("0", 64)
				}
				put(t, filepath.Join(dir, "tasks/active/edit-result.json"), editJSON(t, e))
				w.Close()
				opened, err := Open(dir)
				if err == nil {
					opened.Close()
					t.Fatal("invalid execution reopened")
				}
				return
			}
			if scenario == "missing-start" {
				if _, err := w.createCandidate(&p); err != nil {
					t.Fatal(err)
				}
			} else {
				e, outputs := prepareExecution(t, w, p)
				if scenario == "interrupted" {
					putOutputs(t, dir, outputs)
				} else {
					e, err := w.execute(e, outputs, func() error {
						if scenario == "wrong-content" {
							put(t, filepath.Join(dir, candidate, "EPUB/chapter.xhtml"), []byte("wrong bytes"))
						} else {
							put(t, filepath.Join(dir, candidate, "EPUB/style.css"), []byte("other file"))
						}
						return nil
					})
					if err == nil || e.Status != "failed" || e.ReviewRequired {
						t.Fatal("write check failed", e, err)
					}
				}
			}
			w.Close()
			var err error
			w, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			e, err := w.Execution()
			if err != nil || e.Version != 2 || e.Status != "failed" || treeAt(t, filepath.Join(dir, candidate)).SHA256 != w.base.SHA256 {
				t.Fatal("v2 rollback/recovery", e, err)
			}
			if _, err := w.Reject(e.TaskID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestContentEditAcceptedHistoryAndRecovery(t *testing.T) {
	requireChecker(t)
	w, _, source := legalWorkspace(t, "3.0")
	e := applyPlan(t, w, contentPlan(t, w, "Reviewed"))
	rp, err := validation.Validate(context.Background(), filepath.Join(w.dir, candidate), validation.Options{})
	if err != nil || rp.Status != "pass" {
		t.Fatal("real check", err)
	}
	w.Close()
	for _, phase := range []string{"intent", "pointer", "archive", "bad-schema", "bad-operation", "bad-policy", "bad-execution", "bad-content-hash", "recovery-bad-schema", "recovery-bad-operation", "recovery-bad-policy", "recovery-bad-execution", "recovery-bad-content-hash"} {
		t.Run(phase, func(t *testing.T) {
			bad := strings.Contains(phase, "bad-")
			recovering := strings.HasPrefix(phase, "recovery-")
			dir := filepath.Join(t.TempDir(), "ws")
			w, err := Create(dir, source, Options{})
			if err != nil {
				t.Fatal(err)
			}
			e = applyPlan(t, w, contentPlan(t, w, "Reviewed"))
			id := randomID()
			stage := "staging/accept-" + id
			if err := w.root.Mkdir(stage, 0700); err != nil {
				t.Fatal(err)
			}
			tree, err := copyTree(w.root, candidate, stage+"/pub")
			if err != nil || tree.SHA256 != rp.InputTreeSHA256 {
				t.Fatal("fixture tree", err)
			}
			r := Revision{Version: 1, ID: id, WorkspaceID: w.id, Parent: "initial", TaskID: e.TaskID, ExecutionSHA256: digest(e), Rootfile: w.state.Rootfile, Tree: tree, Validation: &rp}
			put(t, filepath.Join(dir, stage, "revision.json"), editJSON(t, r))
			d := Decision{Version: 1, TaskID: e.TaskID, Status: "accepted", BaseRevision: "initial", RevisionID: id, TreeSHA256: tree.SHA256, Validation: &rp}
			j := settlement{Version: 1, WorkspaceID: w.id, Decision: d}
			if err := w.taskDigests("tasks/active", &j); err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(dir, settlementJournal), editJSON(t, j))
			if phase != "intent" && !recovering {
				if err := publish(w.root, stage, "revisions/"+id); err != nil {
					t.Fatal(err)
				}
				put(t, filepath.Join(dir, "accepted.json"), editJSON(t, acceptedPointer{1, w.id, id}))
			}
			if phase == "archive" || strings.HasPrefix(phase, "bad-") {
				put(t, filepath.Join(dir, "tasks/active/decision.json"), editJSON(t, d))
				if err := publish(w.root, "tasks/active", "tasks/"+e.TaskID); err != nil {
					t.Fatal(err)
				}
				if err := w.root.Remove(settlementJournal); err != nil {
					t.Fatal(err)
				}
			}
			if bad {
				dirTask := "tasks/" + e.TaskID
				if recovering {
					dirTask = "tasks/active"
				}
				var start Execution
				if err := readEditJSON(w.root, dirTask+"/edit-start.json", &start); err != nil {
					t.Fatal(err)
				}
				switch strings.TrimPrefix(phase, "recovery-") {
				case "bad-schema":
					e.Plan.SchemaVersion = 1
				case "bad-operation":
					e.Plan.Operations = setRequest("New").Operations
				case "bad-policy":
					e.Plan.PolicySHA256 = digest(editPolicy)
				case "bad-execution":
					e.Version = 1
					start.Version = 1
				case "bad-content-hash":
					e.Diff.Changes[0].After.SHA256 = strings.Repeat("0", 64)
				}
				e.Plan.OperationSetSHA256 = digest(e.Plan.Operations)
				start.Plan = e.Plan
				// Also update enclosing digest evidence, isolating semantic rederivation.
				put(t, filepath.Join(dir, dirTask, "edit-intent.json"), editJSON(t, e.Plan))
				put(t, filepath.Join(dir, dirTask, "edit-start.json"), editJSON(t, start))
				put(t, filepath.Join(dir, dirTask, "edit-result.json"), editJSON(t, e))
				put(t, filepath.Join(dir, "plans", e.Plan.ID+".json"), editJSON(t, e.Plan))
				put(t, filepath.Join(dir, "plans", e.Plan.ID+".used.json"), editJSON(t, planUse{1, e.TaskID, digest(e.Plan)}))
				r.ExecutionSHA256 = digest(e)
				if recovering {
					j.IntentSHA256 = digest(e.Plan)
					j.StartSHA256 = digest(start)
					j.ResultSHA256 = digest(e)
					put(t, filepath.Join(dir, settlementJournal), editJSON(t, j))
					put(t, filepath.Join(dir, stage, "revision.json"), editJSON(t, r))
				} else {
					put(t, filepath.Join(dir, "revisions", id, "revision.json"), editJSON(t, r))
				}
			}
			w.Close()
			w, err = Open(dir)
			if bad {
				if err == nil {
					w.Close()
					t.Fatal("invalid accepted provenance reopened")
				}
				return
			}
			if err != nil {
				t.Fatal("content settlement/revision reopen", err)
			}
			defer w.Close()
			if w.current != id || exists(w.root, "tasks/active") {
				t.Fatal("recovery did not accept")
			}
			// Current is now the new revision; historical replay must still use
			// the task's initial checkpoint and revision binding.
			p := contentPlan(t, w, "Reviewed again")
			if p.BaseRevision != id || p.SchemaVersion != 2 {
				t.Fatal("advanced baseline", p)
			}
			stale := p.Operations[0].Params.(publication.TextSet)
			stale.RevisionID = "initial"
			if _, err := w.Plan(editJSON(t, Request{2, []Operation{{"content.text.set", 1, stale}}})); !errors.Is(err, ErrStalePlan) {
				t.Fatal("stale revision", err)
			}
		})
	}
}
