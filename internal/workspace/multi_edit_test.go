package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/metadata"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/validation"
)

// FuzzMultiOperationRequest bounds the new version 3 request decoding: accepted
// input must be typed, version-checked, operation-count bounded and canonically
// stable under re-encoding.
func FuzzMultiOperationRequest(f *testing.F) {
	f.Add([]byte(`{"schemaVersion":3,"operations":[{"operationId":"metadata.set","operationVersion":1,"params":{"namespace":"http://purl.org/dc/elements/1.1/","localName":"title","expectedOldValue":"a","newValue":"b"}},{"operationId":"content.text.set","operationVersion":1,"params":{"bookPath":"EPUB/a.xhtml","revisionId":"initial","resourceSha256":"0000000000000000000000000000000000000000000000000000000000000000","locatorVersion":1,"locator":"/html[1]/body[1]/p[1]","expectedOldValue":"a","newValue":"b"}}]}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		var req Request
		if err := decodeStrict(b, &req); err != nil {
			return
		}
		v, err := operationSchema(req.Operations)
		if err != nil {
			return
		}
		if req.SchemaVersion != v {
			return
		}
		if len(req.Operations) == 0 || len(req.Operations) > maxPlanOperations {
			t.Fatalf("accepted request with %d operations", len(req.Operations))
		}
		for _, op := range req.Operations {
			if op.Version != 1 {
				t.Fatalf("accepted operation version %d", op.Version)
			}
			switch op.Params.(type) {
			case metadata.Set, publication.TextSet:
			default:
				t.Fatalf("accepted untyped params %T", op.Params)
			}
		}
		enc, err := json.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		var again Request
		if err := decodeStrict(enc, &again); err != nil {
			t.Fatal(err)
		}
		if digest(again.Operations) != digest(req.Operations) {
			t.Fatal("unstable canonical encoding")
		}
	})
}

// multiWorkspace is a conformance-positive EPUB3 with two XHTML chapters, so a
// single plan can write several resources. It mirrors legalWorkspace's shape.
func multiWorkspace(t *testing.T) (*Workspace, string, string) {
	t.Helper()
	dir := t.TempDir()
	pub := filepath.Join(dir, "pub")
	files := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"EPUB/package.opf":       `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:uuid:12345678-1234-1234-1234-123456789012</dc:identifier><dc:title id="title">Title</dc:title><dc:creator id="creator">Writer</dc:creator><dc:language>en</dc:language><meta property="dcterms:modified">2026-10-04T00:00:00Z</meta></metadata><manifest><item id="c1" href="chapter1.xhtml" media-type="application/xhtml+xml"/><item id="c2" href="chapter2.xhtml" media-type="application/xhtml+xml"/><item id="css" href="style.css" media-type="text/css"/><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>` + "\r\n",
		"EPUB/chapter1.xhtml":    `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>One</title><link rel="stylesheet" type="text/css" href="style.css"/></head><body><h1 id="start">One</h1><p>Alpha &amp; one.</p><p>Second node.</p></body></html>` + "\r\n",
		"EPUB/chapter2.xhtml":    `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Two</title></head><body><p>Beta &amp; two.</p></body></html>` + "\r\n",
		"EPUB/style.css":         "p { color: #123456; }\r\n",
		"EPUB/nav.xhtml":         `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="chapter1.xhtml#start">One</a></li><li><a href="chapter2.xhtml">Two</a></li></ol></nav></body></html>`,
	}
	for name, b := range files {
		put(t, filepath.Join(pub, filepath.FromSlash(name)), []byte(b))
	}
	if err := os.Mkdir(filepath.Join(pub, "empty"), 0700); err != nil {
		t.Fatal(err)
	}
	a, tree, err := archive.SnapshotDirectory(pub, archive.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	source := filepath.Join(dir, "original.epub")
	f, err := os.Create(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.WriteZIP(f, tree); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(dir, "workspace")
	w, err := Create(ws, source, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return w, ws, source
}

func metadataOp(field, id, old, new string) Operation {
	return Operation{"metadata.set", 1, metadata.Set{Namespace: metadata.DC, LocalName: field, ID: id, ExpectedOldValue: old, NewValue: new}}
}

// contentOpN binds the nth manifest XHTML body p element in document order.
func contentOpN(t *testing.T, w *Workspace, bp string, n int, new string) Operation {
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
	seen := 0
	for _, node := range c.Nodes {
		if node.LocalName != "p" {
			continue
		}
		if seen != n {
			seen++
			continue
		}
		return Operation{"content.text.set", 1, publication.TextSet{BookPath: c.BookPath, RevisionID: r.ID, ResourceSHA256: c.ResourceSHA256, LocatorVersion: c.LocatorVersion, Locator: node.Locator, ExpectedOldValue: node.Text, NewValue: new}}
	}
	t.Fatalf("missing p node %d in %s", n, bp)
	return Operation{}
}

// multiPlan is a two-operation version 3 transaction over legalWorkspace's
// single chapter: one metadata and one content operation.
func multiPlan(t *testing.T, w *Workspace) Plan {
	t.Helper()
	p, err := w.Plan(editJSON(t, Request{3, []Operation{
		metadataOp("title", "title", "Title", "Multi Title"),
		contentOpN(t, w, "EPUB/chapter.xhtml", 0, "Changed body"),
	}}))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// multiWritePlan changes three resources in one transaction: the OPF and both
// chapters of multiWorkspace.
func multiWritePlan(t *testing.T, w *Workspace) Plan {
	t.Helper()
	p, err := w.Plan(editJSON(t, Request{3, []Operation{
		metadataOp("title", "title", "Title", "Multi < & > Title"),
		contentOpN(t, w, "EPUB/chapter1.xhtml", 0, "First < & > 😀"),
		contentOpN(t, w, "EPUB/chapter2.xhtml", 0, "Second changed"),
	}}))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMultiOperationPlanApplyActualDiffAndProvenance(t *testing.T) {
	w, dir, _ := multiWorkspace(t)
	defer w.Close()
	initial := w.State()
	original, err := os.ReadFile(filepath.Join(dir, "original/book.epub"))
	if err != nil {
		t.Fatal(err)
	}
	ch1, err := os.ReadFile(filepath.Join(dir, revision, "EPUB/chapter1.xhtml"))
	if err != nil {
		t.Fatal(err)
	}
	ch2, err := os.ReadFile(filepath.Join(dir, revision, "EPUB/chapter2.xhtml"))
	if err != nil {
		t.Fatal(err)
	}
	opf, err := os.ReadFile(filepath.Join(dir, revision, "EPUB/package.opf"))
	if err != nil {
		t.Fatal(err)
	}
	p := multiWritePlan(t, w)
	if p.SchemaVersion != 3 || p.PolicySHA256 != digest(multiEditPolicy) || !slices.Equal(p.WriteSet, []string{"EPUB/chapter1.xhtml", "EPUB/chapter2.xhtml", "EPUB/package.opf"}) {
		t.Fatalf("multi plan: %+v", p)
	}
	if exists(w.root, "tasks/active") || treeAt(t, filepath.Join(dir, revision)).SHA256 != initial.Tree.SHA256 {
		t.Fatal("plan mutated publication")
	}
	e := applyPlan(t, w, p)
	if e.Version != 3 || e.Status != "review_required" || !e.ReviewRequired || e.Conformance != "not_run" || !e.Diff.Changed || len(e.Diff.Changes) != 3 {
		t.Fatalf("execution: %+v", e)
	}
	d, err := w.Diff()
	if err != nil || digest(d) != digest(e.Diff) {
		t.Fatalf("actual diff: %+v %v", d, err)
	}
	got, err := w.Execution()
	if err != nil || digest(got) != digest(e) {
		t.Fatalf("execution provenance: %+v %v", got, err)
	}
	// Every planned resource carries its exact derived bytes; every other
	// resource stays byte-identical to the accepted baseline.
	want1 := bytes.Replace(ch1, []byte(">Alpha &amp; one.</p>"), []byte(">First &lt; &amp; &gt; 😀</p>"), 1)
	want2 := bytes.Replace(ch2, []byte(">Beta &amp; two.</p>"), []byte(">Second changed</p>"), 1)
	wantOPF := bytes.Replace(opf, []byte(">Title</dc:title>"), []byte(">Multi &lt; &amp; &gt; Title</dc:title>"), 1)
	assertBytes(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"), want1)
	assertBytes(t, filepath.Join(dir, candidate, "EPUB/chapter2.xhtml"), want2)
	assertBytes(t, filepath.Join(dir, candidate, "EPUB/package.opf"), wantOPF)
	assertBytes(t, filepath.Join(dir, candidate, "EPUB/nav.xhtml"), []byte(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="chapter1.xhtml#start">One</a></li><li><a href="chapter2.xhtml">Two</a></li></ol></nav></body></html>`))
	assertBytes(t, filepath.Join(dir, candidate, "EPUB/style.css"), []byte("p { color: #123456; }\r\n"))
	assertBytes(t, filepath.Join(dir, revision, "EPUB/chapter1.xhtml"), ch1)
	assertBytes(t, filepath.Join(dir, revision, "EPUB/package.opf"), opf)
	assertBytes(t, filepath.Join(dir, "original/book.epub"), original)
	// Review reports every operation's actual candidate value, in order.
	r, err := w.TaskDiff(e.TaskID)
	if err != nil || !r.MatchesExecution || len(r.Operations) != 3 {
		t.Fatalf("review: %+v %v", r, err)
	}
	for i, want := range []string{"Multi < & > Title", "First < & > 😀", "Second changed"} {
		op := r.Operations[i]
		if op.Index != i || op.NewValue == nil || *op.NewValue != want || op.PlannedValue != want || op.Unavailable != "" {
			t.Fatalf("operation %d: %+v", i, op)
		}
	}
	if r.Operations[0].OldValue != "Title" || r.Operations[1].OldValue != "Alpha & one." || r.Operations[2].OldValue != "Beta & two." {
		t.Fatalf("operation old values: %+v", r.Operations)
	}
	// Reopening must re-verify the same multi-resource execution.
	w.Close()
	w, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	got, err = w.Execution()
	if err != nil || digest(got) != digest(e) {
		t.Fatalf("reopened execution: %+v %v", got, err)
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
	if treeAt(t, filepath.Join(dir, revision)).SHA256 != initial.Tree.SHA256 {
		t.Fatal("rejection changed accepted baseline")
	}
}

func TestMultiOperationSameFileTargetsAndNoOpMix(t *testing.T) {
	w, dir, _ := multiWorkspace(t)
	defer w.Close()
	ch1, err := os.ReadFile(filepath.Join(dir, revision, "EPUB/chapter1.xhtml"))
	if err != nil {
		t.Fatal(err)
	}
	opf, err := os.ReadFile(filepath.Join(dir, revision, "EPUB/package.opf"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := w.Plan(editJSON(t, Request{3, []Operation{
		contentOpN(t, w, "EPUB/chapter1.xhtml", 0, "First changed"),
		contentOpN(t, w, "EPUB/chapter1.xhtml", 1, "Second changed"),
		metadataOp("title", "title", "Title", "Title"),
	}}))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.WriteSet, []string{"EPUB/chapter1.xhtml"}) {
		t.Fatalf("same-file write set: %+v", p.WriteSet)
	}
	e := applyPlan(t, w, p)
	if len(e.Diff.Changes) != 1 || e.Diff.Changes[0].Path != "EPUB/chapter1.xhtml" {
		t.Fatalf("same-file diff: %+v", e.Diff)
	}
	want := bytes.Replace(ch1, []byte(">Alpha &amp; one.</p>"), []byte(">First changed</p>"), 1)
	want = bytes.Replace(want, []byte(">Second node.</p>"), []byte(">Second changed</p>"), 1)
	assertBytes(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"), want)
	assertBytes(t, filepath.Join(dir, candidate, "EPUB/package.opf"), opf)
	r, err := w.TaskDiff(e.TaskID)
	if err != nil || !r.MatchesExecution || len(r.Operations) != 3 {
		t.Fatalf("review: %+v %v", r, err)
	}
	for i, want := range []string{"First changed", "Second changed", "Title"} {
		if r.Operations[i].NewValue == nil || *r.Operations[i].NewValue != want {
			t.Fatalf("operation %d actual value: %+v", i, r.Operations[i])
		}
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
	// A transaction whose every operation is a no-op changes no bytes, has an
	// empty write set and still records a complete review.
	noop, err := w.Plan(editJSON(t, Request{3, []Operation{
		contentOpN(t, w, "EPUB/chapter1.xhtml", 0, "Alpha & one."),
		contentOpN(t, w, "EPUB/chapter2.xhtml", 0, "Beta & two."),
		metadataOp("title", "title", "Title", "Title"),
	}}))
	if err != nil {
		t.Fatal(err)
	}
	if len(noop.WriteSet) != 0 || noop.WriteSet == nil {
		t.Fatalf("no-op write set: %+v", noop.WriteSet)
	}
	ne := applyPlan(t, w, noop)
	if ne.Diff.Changed || !ne.ReviewRequired || ne.Conformance != "not_run" {
		t.Fatalf("no-op execution: %+v", ne)
	}
	if _, err := w.Execution(); err != nil {
		t.Fatal(err)
	}
	nr, err := w.TaskDiff(ne.TaskID)
	if err != nil || !nr.MatchesExecution || nr.Diff.Changed {
		t.Fatalf("no-op review: %+v %v", nr, err)
	}
	for i := range nr.Operations {
		if nr.Operations[i].NewValue == nil || *nr.Operations[i].NewValue != nr.Operations[i].OldValue {
			t.Fatalf("no-op operation %d: %+v", i, nr.Operations[i])
		}
	}
	if _, err := w.Reject(ne.TaskID); err != nil {
		t.Fatal(err)
	}
}

func TestMultiOperationRejectsStaleDuplicateAndUnsupportedTargets(t *testing.T) {
	w, _, _ := multiWorkspace(t)
	defer w.Close()
	first := contentOpN(t, w, "EPUB/chapter1.xhtml", 0, "changed")
	// The same exact locator twice is a duplicate, not two edits.
	if _, err := w.Plan(editJSON(t, Request{3, []Operation{first, first}})); err == nil {
		t.Fatal("duplicate content target accepted")
	}
	// An explicit id and an omitted id that both select the unique element are
	// an alias of one target and must not be applied twice.
	alias := Request{3, []Operation{metadataOp("title", "title", "Title", "A"), metadataOp("title", "", "Title", "B")}}
	if _, err := w.Plan(editJSON(t, alias)); err == nil {
		t.Fatal("aliased metadata target accepted")
	}
	duplicateMetadata := Request{3, []Operation{metadataOp("title", "title", "Title", "A"), metadataOp("title", "title", "Title", "B")}}
	if _, err := w.Plan(editJSON(t, duplicateMetadata)); err == nil {
		t.Fatal("duplicate metadata target accepted")
	}
	stale := first
	staleParam := stale.Params.(publication.TextSet)
	staleParam.RevisionID = randomID()
	stale.Params = staleParam
	if _, err := w.Plan(editJSON(t, Request{3, []Operation{stale, metadataOp("title", "title", "Title", "A")}})); !errors.Is(err, ErrStalePlan) {
		t.Fatalf("stale revision: %v", err)
	}
	drift := first
	driftParam := drift.Params.(publication.TextSet)
	driftParam.ResourceSHA256 = strings.Repeat("0", 64)
	drift.Params = driftParam
	_, err := w.Plan(editJSON(t, Request{3, []Operation{drift, metadataOp("title", "title", "Title", "A")}}))
	var f *fault.Error
	if !errors.Is(err, ErrStalePlan) && (!errors.As(err, &f) || f.Code != "INPUT_DRIFT") {
		t.Fatalf("resource hash drift: %v", err)
	}
	badVersion := metadataOp("title", "title", "Title", "A")
	badVersion.Version = 2
	if _, err := w.Plan(editJSON(t, Request{3, []Operation{badVersion, metadataOp("title", "title", "Title", "A")}})); err == nil {
		t.Fatal("operation version 2 accepted in a multi-operation plan")
	}
	// Schema 3 is only the multi-operation form; a single operation keeps its
	// frozen schema 1/2 encoding.
	if _, err := w.Plan(editJSON(t, Request{3, []Operation{metadataOp("title", "title", "Title", "A")}})); err == nil {
		t.Fatal("single-operation schema 3 accepted")
	}
	over := make([]Operation, 0, maxPlanOperations+1)
	for i := 0; i <= maxPlanOperations; i++ {
		over = append(over, metadataOp("title", "title", "Title", fmt.Sprintf("v%d", i)))
	}
	if _, err := w.Plan(editJSON(t, Request{3, over})); err == nil {
		t.Fatal("operation count limit not enforced")
	}
	raw := `{"schemaVersion":3,"operations":[{"operationId":"metadata.set","operationVersion":1,"params":{"namespace":"http://purl.org/dc/elements/1.1/","localName":"title","expectedOldValue":"Title","newValue":"A"}},{"operationId":"resource.rename","operationVersion":1,"params":{}}]}`
	if _, err := w.Plan([]byte(raw)); err == nil {
		t.Fatal("unsupported operation accepted")
	}
	if exists(w.root, "tasks/active") || exists(w.root, "plans") {
		t.Fatal("rejected request published a plan or candidate")
	}
}

func TestMultiOperationSecondResourceFailureRollsBack(t *testing.T) {
	w, dir, _ := multiWorkspace(t)
	defer w.Close()
	p := multiWritePlan(t, w)
	e, outputs := prepareExecution(t, w, p)
	// The second planned resource becomes a hard-linked file: the write must
	// fail there after the first resource was already replaced.
	outside := filepath.Join(dir, "outside.bin")
	put(t, outside, []byte("outside"))
	second := filepath.Join(dir, candidate, p.WriteSet[1])
	if err := os.Remove(second); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(outside, second); err != nil {
		t.Fatal(err)
	}
	failed, err := w.execute(e, outputs, nil)
	if err == nil || failed.Status != "failed" || failed.ReviewRequired || failed.Failure == "" {
		t.Fatalf("second-resource failure: %+v %v", failed, err)
	}
	if treeAt(t, filepath.Join(dir, candidate)).SHA256 != p.InputTreeSHA256 {
		t.Fatal("failed multi-resource write left a partial candidate")
	}
	assertBytes(t, outside, []byte("outside"))
	assertEmpty(t, filepath.Join(dir, "staging"))
	assertEmpty(t, filepath.Join(dir, "journal"))
	w.Close()
	w, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	got, err := w.Execution()
	if err != nil || got.Status != "failed" || got.Failure != failed.Failure {
		t.Fatalf("failed execution record: %+v %v", got, err)
	}
	if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{}); !errors.Is(err, ErrTaskConflict) {
		t.Fatalf("failed multi-operation task accepted: %v", err)
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}

func TestMultiOperationRollbackInterruptedByIOIsRetryable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission failure requires non-root user")
	}
	w, dir, _ := multiWorkspace(t)
	p := multiWritePlan(t, w)
	e, outputs := prepareExecution(t, w, p)
	// All planned resources of this fixture share one parent directory, so the
	// blocked directory fails the first planned write before any file is
	// replaced. This exercises a permission-induced write failure plus a
	// rollback whose cleanup itself needs a retry; the hard-link test above
	// covers failure at the second resource after the first was replaced.
	parent := filepath.Dir(p.WriteSet[0])
	for _, target := range p.WriteSet {
		if filepath.Dir(target) != parent {
			t.Fatalf("fixture targets no longer share a parent: %v", p.WriteSet)
		}
	}
	blocked := filepath.Join(dir, candidate, parent)
	if err := os.Chmod(blocked, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, name := range []string{
			filepath.Join(dir, candidate, parent),
			filepath.Join(dir, "staging/restore-old", parent),
		} {
			if _, err := os.Lstat(name); err == nil {
				os.Chmod(name, 0700)
			}
		}
	})
	failed, err := w.execute(e, outputs, nil)
	if err == nil || failed.Status != "failed" || failed.ReviewRequired {
		t.Fatalf("permission-induced write failure: %+v %v", failed, err)
	}
	if !strings.Contains(failed.Failure, "permission") {
		t.Fatalf("expected a permission failure, got %q", failed.Failure)
	}
	// The blocked parent prevents the first planned write, so no resource was
	// replaced. The visible candidate is already the exact baseline even though
	// removing the read-only backup needs a retry.
	if treeAt(t, filepath.Join(dir, candidate)).SHA256 != p.InputTreeSHA256 {
		t.Fatal("candidate is not the exact baseline after interrupted rollback")
	}
	restoreOldParent := filepath.Join(dir, "staging/restore-old", parent)
	if _, err := os.Lstat(restoreOldParent); err == nil {
		if err := os.Chmod(restoreOldParent, 0700); err != nil {
			t.Fatal(err)
		}
	} else if err := os.Chmod(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	w.Close()
	w, err = Open(dir)
	if err != nil {
		t.Fatal("rollback retry after fault removal:", err)
	}
	defer w.Close()
	got, err := w.Execution()
	if err != nil || got.Status != "failed" || got.Failure != failed.Failure {
		t.Fatalf("recovered failure record: %+v %v", got, err)
	}
	assertEmpty(t, filepath.Join(dir, "staging"))
	assertEmpty(t, filepath.Join(dir, "journal"))
	if treeAt(t, filepath.Join(dir, candidate)).SHA256 != p.InputTreeSHA256 {
		t.Fatal("retried rollback lost the baseline")
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}

func TestMultiOperationInterruptedApplyRollsBackWholeCandidate(t *testing.T) {
	w, dir, _ := multiWorkspace(t)
	p := multiWritePlan(t, w)
	e, outputs := prepareExecution(t, w, p)
	// Interruption after the first of several planned writes.
	first := p.WriteSet[0]
	put(t, filepath.Join(dir, candidate, first), outputs[first])
	w.Close()
	w, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	got, err := w.Execution()
	if err != nil || got.Status != "failed" || got.Failure != "interrupted apply" || got.ReviewRequired {
		t.Fatalf("interrupted multi-operation apply: %+v %v", got, err)
	}
	if treeAt(t, filepath.Join(dir, candidate)).SHA256 != p.InputTreeSHA256 {
		t.Fatal("partial multi-resource write survived recovery")
	}
	if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{}); !errors.Is(err, ErrTaskConflict) {
		t.Fatalf("interrupted task accepted: %v", err)
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}

func TestMultiOperationForgedExecutionCannotClaimReview(t *testing.T) {
	for _, scenario := range []string{"extra-file", "reverted-file", "wrong-bytes"} {
		t.Run(scenario, func(t *testing.T) {
			w, dir, _ := multiWorkspace(t)
			p := multiWritePlan(t, w)
			e := applyPlan(t, w, p)
			switch scenario {
			case "extra-file":
				put(t, filepath.Join(dir, candidate, "EPUB/extra.txt"), []byte("unplanned"))
			case "reverted-file":
				base, err := os.ReadFile(filepath.Join(dir, revision, "EPUB/chapter2.xhtml"))
				if err != nil {
					t.Fatal(err)
				}
				put(t, filepath.Join(dir, candidate, "EPUB/chapter2.xhtml"), base)
			case "wrong-bytes":
				put(t, filepath.Join(dir, candidate, "EPUB/chapter2.xhtml"), []byte("same filename wrong bytes"))
			}
			// Forge a self-consistent actual diff; it still cannot describe the
			// complete derived output set.
			var err error
			e.Diff, err = w.Diff()
			if err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(dir, "tasks/active/edit-result.json"), editJSON(t, e))
			w.Close()
			opened, err := Open(dir)
			if err == nil {
				opened.Close()
				t.Fatal("trusted forged multi-resource review")
			}
		})
	}
}

func TestMultiOperationSettlementFailureAndRetry(t *testing.T) {
	requireChecker(t)
	w, dir, _ := multiWorkspace(t)
	p := multiWritePlan(t, w)
	e := applyPlan(t, w, p)
	// Block the settlement journal name with a directory: the irreversible
	// acceptance intent cannot be published.
	blocked := filepath.Join(dir, settlementJournal)
	if err := os.MkdirAll(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{}); err == nil {
		t.Fatal("accept succeeded with blocked settlement journal")
	}
	if w.current != "initial" || exists(w.root, "accepted.json") || exists(w.root, "tasks/active/decision.json") {
		t.Fatal("blocked settlement committed acceptance")
	}
	if _, err := w.Execution(); err != nil {
		t.Fatalf("blocked settlement lost the review task: %v", err)
	}
	if err := os.RemoveAll(blocked); err != nil {
		t.Fatal(err)
	}
	// The same task can still be accepted after the fault is removed.
	d, err := w.Accept(context.Background(), e.TaskID, validation.Options{})
	if err != nil || d.Status != "accepted" || d.Validation == nil || d.Validation.Status != "pass" {
		t.Fatalf("retried accept: %+v %v", d, err)
	}
	if w.current != d.RevisionID || exists(w.root, settlementJournal) {
		t.Fatal("retried accept did not settle")
	}
}

func TestMultiOperationSettlementRollForward(t *testing.T) {
	requireChecker(t)
	w, dir, _ := multiWorkspace(t)
	p := multiWritePlan(t, w)
	e := applyPlan(t, w, p)
	rp, err := validation.Validate(context.Background(), filepath.Join(dir, candidate), validation.Options{})
	if err != nil || rp.Status != "pass" {
		t.Fatalf("real check: %+v %v", rp, err)
	}
	// Fabricate the exact durable intent Accept publishes, then interrupt
	// before the accepted pointer. Reopening must finish the original decision.
	id := randomID()
	stage := "staging/accept-" + id
	if err := w.root.Mkdir(stage, 0700); err != nil {
		t.Fatal(err)
	}
	tree, err := copyTree(w.root, candidate, stage+"/pub")
	if err != nil || tree.SHA256 != rp.InputTreeSHA256 {
		t.Fatalf("frozen tree: %v", err)
	}
	r := Revision{Version: 1, ID: id, WorkspaceID: w.id, Parent: "initial", TaskID: e.TaskID, ExecutionSHA256: digest(e), Rootfile: w.state.Rootfile, Tree: tree, Validation: &rp}
	put(t, filepath.Join(dir, stage, "revision.json"), editJSON(t, r))
	d := Decision{Version: 1, TaskID: e.TaskID, Status: "accepted", BaseRevision: "initial", RevisionID: id, TreeSHA256: tree.SHA256, Validation: &rp}
	j := settlement{Version: 1, WorkspaceID: w.id, Decision: d}
	if err := w.taskDigests("tasks/active", &j); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(dir, settlementJournal), editJSON(t, j))
	w.Close()
	w, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if w.current != id || exists(w.root, "tasks/active") || exists(w.root, settlementJournal) {
		t.Fatal("multi-operation settlement did not roll forward")
	}
	st, err := w.TaskStatus(e.TaskID)
	if err != nil || st.Status != "accepted" || st.Decision == nil || st.Decision.RevisionID != id {
		t.Fatalf("rolled-forward history: %+v %v", st, err)
	}
}

func TestMultiOperationAcceptHistoryAndNoOpRevision(t *testing.T) {
	requireChecker(t)
	w, dir, source := multiWorkspace(t)
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	initial := w.State()
	p := multiWritePlan(t, w)
	e := applyPlan(t, w, p)
	candidateTree := treeAt(t, filepath.Join(dir, candidate))
	candidateBytes := map[string][]byte{}
	for _, path := range p.WriteSet {
		b, err := os.ReadFile(filepath.Join(dir, candidate, path))
		if err != nil {
			t.Fatal(err)
		}
		candidateBytes[path] = b
	}
	d, err := w.Accept(context.Background(), e.TaskID, validation.Options{})
	if err != nil || d.Status != "accepted" || d.Validation == nil || d.Validation.Status != "pass" {
		t.Fatalf("accept: %+v %v", d, err)
	}
	if w.current != d.RevisionID || d.TreeSHA256 != candidateTree.SHA256 {
		t.Fatalf("accepted baseline: %+v", d)
	}
	// The accepted revision carries every planned write and preserves every
	// other resource byte.
	for path, want := range candidateBytes {
		assertBytes(t, filepath.Join(dir, revisionPath(d.RevisionID), path), want)
	}
	for _, entry := range initial.Tree.Entries {
		if entry.Type != "file" || slices.Contains(p.WriteSet, entry.Path) {
			continue
		}
		before, err := os.ReadFile(filepath.Join(dir, revision, entry.Path))
		if err != nil {
			t.Fatal(err)
		}
		assertBytes(t, filepath.Join(dir, revisionPath(d.RevisionID), entry.Path), before)
	}
	assertBytes(t, source, original)
	assertBytes(t, filepath.Join(dir, "original/book.epub"), original)
	// Historical status re-verifies the multi-operation settlement provenance.
	st, err := w.TaskStatus(e.TaskID)
	if err != nil || st.Status != "accepted" || st.Decision == nil || st.Decision.RevisionID != d.RevisionID {
		t.Fatalf("accepted history: %+v %v", st, err)
	}
	// A fully no-op multi-operation transaction still creates an independent
	// audited revision with identical bytes.
	noop, err := w.Plan(editJSON(t, Request{3, []Operation{
		metadataOp("title", "title", "Multi < & > Title", "Multi < & > Title"),
		contentOpN(t, w, "EPUB/chapter1.xhtml", 0, "First < & > 😀"),
		contentOpN(t, w, "EPUB/chapter2.xhtml", 0, "Second changed"),
	}}))
	if err != nil {
		t.Fatal(err)
	}
	ne := applyPlan(t, w, noop)
	if ne.Diff.Changed || len(noop.WriteSet) != 0 {
		t.Fatalf("no-op accepted transaction: %+v %+v", noop, ne)
	}
	nd, err := w.Accept(context.Background(), ne.TaskID, validation.Options{})
	if err != nil || nd.TreeSHA256 != d.TreeSHA256 || nd.RevisionID == d.RevisionID {
		t.Fatalf("no-op acceptance: %+v %v", nd, err)
	}
	// The previous multi-operation plan is now stale against the new baseline.
	if _, err := w.Apply(editJSON(t, p)); !errors.Is(err, ErrStalePlan) {
		t.Fatalf("stale multi-operation plan: %v", err)
	}
	w.Close()
	w, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if w.current != nd.RevisionID {
		t.Fatal("reopen lost the no-op revision")
	}
	st, err = w.TaskStatus(ne.TaskID)
	if err != nil || st.Status != "accepted" {
		t.Fatalf("reopened history: %+v %v", st, err)
	}
}
