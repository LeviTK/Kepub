package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/metadata"
	"github.com/LeviTK/Kepub/internal/validation"
)

// Independent EPUB2/3 conformance-positive fixtures, not the deliberately
// malformed query fixtures. Compare resource bytes, never ZIP compressed bytes.
func legalWorkspace(t *testing.T, version string) (*Workspace, string, string) {
	t.Helper()
	dir := t.TempDir()
	pub := filepath.Join(dir, "pub")
	meta := `<meta property="dcterms:modified">2026-10-04T00:00:00Z</meta>`
	nav := `<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`
	spine := `<spine>`
	if version == "2.0" {
		meta = ""
		nav = `<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>`
		spine = `<spine toc="ncx">`
	}
	files := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"EPUB/package.opf":       fmt.Sprintf(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="%s" unique-identifier="uid"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:uuid:12345678-1234-1234-1234-123456789012</dc:identifier><!-- Title is also in this untouched comment --><dc:title id="title">Title</dc:title><dc:creator id="creator">Writer</dc:creator><dc:language>en</dc:language>%s</metadata><manifest><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/><item id="css" href="style.css" media-type="text/css"/>%s</manifest>%s<itemref idref="chapter"/></spine></package>`, version, meta, nav, spine) + "\r\n",
		"EPUB/chapter.xhtml":     `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter</title><link rel="stylesheet" type="text/css" href="style.css"/></head><body><h1 id="start">Chapter</h1><p>Original &amp; precise.</p></body></html>` + "\r\n",
		"EPUB/style.css":         "p { color: #123456; }\r\n",
	}
	if version == "3.0" {
		files["EPUB/nav.xhtml"] = `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="chapter.xhtml#start">Chapter</a></li></ol></nav></body></html>`
	} else {
		files["EPUB/toc.ncx"] = `<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head><meta name="dtb:uid" content="urn:uuid:12345678-1234-1234-1234-123456789012"/><meta name="dtb:depth" content="1"/><meta name="dtb:totalPageCount" content="0"/><meta name="dtb:maxPageNumber" content="0"/></head><docTitle><text>Title</text></docTitle><navMap><navPoint id="n1" playOrder="1"><navLabel><text>Chapter</text></navLabel><content src="chapter.xhtml#start"/></navPoint></navMap></ncx>`
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
func requireChecker(t *testing.T) {
	t.Helper()
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("real pinned EPUBCheck required; run .agents/setup and a fresh login shell")
	}
}
func fieldPlan(t *testing.T, w *Workspace, field, id, old, new string) Plan {
	t.Helper()
	p, err := w.Plan(editJSON(t, Request{1, []Operation{{"metadata.set", 1, metadata.Set{Namespace: metadata.DC, LocalName: field, ID: id, ExpectedOldValue: old, NewValue: new}}}}))
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func applyPlan(t *testing.T, w *Workspace, p Plan) Execution {
	t.Helper()
	e, err := w.Apply(editJSON(t, p))
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestAcceptedLifecycle(t *testing.T) {
	requireChecker(t)
	for _, version := range []string{"2.0", "3.0"} {
		t.Run(version, func(t *testing.T) {
			w, dir, source := legalWorkspace(t, version)
			defer func() { w.Close() }()
			original, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			opf, err := os.ReadFile(filepath.Join(dir, revision, w.state.Rootfile))
			if err != nil {
				t.Fatal(err)
			}
			initial := w.State()
			p := fieldPlan(t, w, "title", "title", "Title", "New < & >")
			stale := fieldPlan(t, w, "title", "title", "Title", "Stale")
			e := applyPlan(t, w, p)
			a, _, r, err := w.AcceptedSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			a.Close()
			if r.ID != "initial" || r.Validation != nil {
				t.Fatal("unaccepted candidate leaked into accepted baseline", r)
			}
			review, err := w.TaskDiff(e.TaskID)
			if err != nil || review.Metadata.NewValue == nil || *review.Metadata.NewValue != "New < & >" || review.Metadata.OldValue != "Title" || !review.MatchesExecution {
				t.Fatalf("review %+v %v", review, err)
			}
			d, err := w.Accept(context.Background(), e.TaskID, validation.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if d.Status != "accepted" || d.Validation.Status != "pass" || w.current != d.RevisionID {
				t.Fatalf("accept %+v", d)
			}
			assertBytes(t, filepath.Join(dir, revision, w.state.Rootfile), opf)
			assertBytes(t, source, original)
			assertBytes(t, filepath.Join(dir, "original/book.epub"), original)
			if digest(w.State()) != digest(initial) {
				t.Fatal("initial State changed")
			}
			if _, err := w.Apply(editJSON(t, stale)); !errors.Is(err, ErrStalePlan) {
				t.Fatalf("old baseline plan: %v", err)
			}
			if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{}); !errors.Is(err, ErrTaskConflict) {
				t.Fatalf("closed task: %v", err)
			}
			w.Close()
			w, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			second := fieldPlan(t, w, "creator", "creator", "Writer", "Another & Δ")
			if second.BaseRevision != d.RevisionID || second.InputTreeSHA256 != d.TreeSHA256 {
				t.Fatal("second plan did not advance baseline")
			}
			failed, out := prepareExecution(t, w, second)
			failed, err = w.execute(failed, out, func() error {
				put(t, filepath.Join(dir, candidate, "EPUB/style.css"), []byte("unexpected accepted-baseline write"))
				return nil
			})
			if err == nil || failed.Status != "failed" || treeAt(t, filepath.Join(dir, candidate)).SHA256 != d.TreeSHA256 || w.current != d.RevisionID {
				t.Fatalf("accepted-baseline rollback %+v %v", failed, err)
			}
			if _, err := w.Reject(failed.TaskID); err != nil {
				t.Fatal(err)
			}
			second = fieldPlan(t, w, "creator", "creator", "Writer", "Another & Δ")
			e2 := applyPlan(t, w, second)
			snap, err := w.snapshot(e2.Checkpoint)
			if err != nil || snap.BaseRevision != d.RevisionID || snap.Tree.SHA256 != d.TreeSHA256 {
				t.Fatalf("second checkpoint %+v %v", snap, err)
			}
			d2, err := w.Accept(context.Background(), e2.TaskID, validation.Options{})
			if err != nil {
				t.Fatal(err)
			}
			a, tree, r, err := w.AcceptedSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			a.Close()
			if r.Parent != d.RevisionID || tree.SHA256 != d2.TreeSHA256 {
				t.Fatal("accepted chain did not advance")
			}
			want := bytes.Replace(opf, []byte(`>Title</dc:title>`), []byte(`>New &lt; &amp; &gt;</dc:title>`), 1)
			want = bytes.Replace(want, []byte(`>Writer</dc:creator>`), []byte(`>Another &amp; Δ</dc:creator>`), 1)
			assertBytes(t, filepath.Join(dir, revisionPath(r.ID), w.state.Rootfile), want)
			for _, entry := range initial.Tree.Entries {
				if entry.Type == "file" && entry.Path != w.state.Rootfile {
					before, err := os.ReadFile(filepath.Join(dir, revision, entry.Path))
					if err != nil {
						t.Fatal(err)
					}
					assertBytes(t, filepath.Join(dir, revisionPath(r.ID), entry.Path), before)
				}
			}
			noop := fieldPlan(t, w, "creator", "creator", "Another & Δ", "Another & Δ")
			ne := applyPlan(t, w, noop)
			if ne.Diff.Changed || len(ne.Plan.WriteSet) != 0 || !ne.ReviewRequired || ne.Conformance != "not_run" {
				t.Fatalf("no-op %+v", ne)
			}
			nd, err := w.Accept(context.Background(), ne.TaskID, validation.Options{})
			if err != nil || nd.TreeSHA256 != d2.TreeSHA256 {
				t.Fatalf("no-op acceptance %+v %v", nd, err)
			}
			assertBytes(t, filepath.Join(dir, revisionPath(nd.RevisionID), w.state.Rootfile), want)
			w.Close()
			w, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			if w.current != nd.RevisionID {
				t.Fatal("reopen lost current revision")
			}
		})
	}
}

func TestCandidateDriftCanReviewAndReject(t *testing.T) {
	w, dir := makeWorkspace(t)
	p := planTitle(t, w, "New")
	e := applyPlan(t, w, p)
	file := filepath.Join(dir, candidate, p.Rootfile)
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	put(t, file, bytes.Replace(b, []byte(`>New</dc:title>`), []byte(`>Actual</dc:title>`), 1))
	put(t, filepath.Join(dir, candidate, "unexpected.txt"), []byte("unapproved"))
	if err := os.Remove(filepath.Join(dir, candidate, "unlisted.bin")); err != nil {
		t.Fatal(err)
	}
	w.Close()
	w, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	r, err := w.TaskDiff(e.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if r.MatchesExecution || len(r.Diff.Changes) != 3 || r.Metadata.NewValue == nil || *r.Metadata.NewValue != "Actual" {
		t.Fatalf("not an actual review %+v", r)
	}
	if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{}); !errors.Is(err, ErrCandidateDrift) {
		t.Fatalf("tamper inherited approval: %v", err)
	}
	if _, err := w.TaskDiff(randomID()); !errors.Is(err, ErrTaskConflict) {
		t.Fatalf("wrong task ID: %v", err)
	}
	d, err := w.Reject(e.TaskID)
	if err != nil || d.Status != "rejected" {
		t.Fatalf("reject %+v %v", d, err)
	}
	if !exists(w.root, "tasks/"+e.TaskID+"/decision.json") {
		t.Fatal("audit lost")
	}
	if _, err := w.Apply(editJSON(t, p)); !errors.Is(err, ErrTaskConflict) {
		t.Fatalf("consumed rejected plan replay: %v", err)
	}
	applyPlan(t, w, planTitle(t, w, "Next"))
}

func TestAcceptRequiresChecksAndNoDraft(t *testing.T) {
	for _, scenario := range []string{"missing", "invalid", "draft", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			if scenario == "invalid" {
				requireChecker(t)
			}
			w, dir := makeWorkspace(t)
			defer w.Close()
			e := applyPlan(t, w, planTitle(t, w, "New"))
			o := validation.Options{}
			ctx := context.Background()
			switch scenario {
			case "missing":
				o.JAR = filepath.Join(t.TempDir(), "missing.jar")
			case "draft":
				o.Draft = true
			case "cancelled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			d, err := w.Accept(ctx, e.TaskID, o)
			if err == nil || w.current != "initial" || exists(w.root, "accepted.json") {
				t.Fatalf("failed gate updated pointer %+v %v", d, err)
			}
			if d.Validation != nil && d.Validation.Status == "pass" {
				t.Fatal("failed validation reported pass")
			}
			w.Close()
			w, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			got, err := w.Execution()
			if err != nil || got.Conformance != "not_run" {
				t.Fatalf("failed check changed execution %+v %v", got, err)
			}
			if _, err := w.Reject(e.TaskID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOutputBoundary(t *testing.T) {
	w, dir := makeWorkspace(t)
	defer w.Close()
	p := planTitle(t, w, "New")
	applyPlan(t, w, p)
	for _, root := range []string{"original", revision, candidate, "."} {
		out := filepath.Join(dir, root, "absent.json")
		if _, err := w.OutputPath(out); err == nil {
			t.Fatalf("accepted protected output %s", out)
		}
		if err := w.WritePlanReport(p, out); err == nil {
			t.Fatalf("wrote protected report %s", out)
		}
		if _, err := os.Lstat(out); !os.IsNotExist(err) {
			t.Fatal("protected tree polluted")
		}
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(filepath.Join(dir, candidate), alias); err != nil {
		t.Fatal(err)
	}
	if _, err := w.OutputPath(filepath.Join(alias, "absent.json")); err == nil {
		t.Fatal("symlink alias bypassed boundary")
	}
	out := filepath.Join(t.TempDir(), "plan.json")
	if err := w.WritePlanReport(p, out); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.WritePlanReport(p, out); err == nil {
		t.Fatal("replaced existing report")
	}
	assertBytes(t, out, before)
}

func TestAcceptanceJournalRecovery(t *testing.T) {
	requireChecker(t)
	w, _, source := legalWorkspace(t, "3.0")
	e := applyPlan(t, w, fieldPlan(t, w, "title", "title", "Title", "Reviewed"))
	rp, err := validation.Validate(context.Background(), filepath.Join(w.dir, candidate), validation.Options{})
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	for _, phase := range []string{"before-intent", "intent", "revision", "pointer", "decision", "archive", "clean", "tamper", "moved-intent", "moved-pointer", "moved-tamper"} {
		t.Run(phase, func(t *testing.T) {
			moved := strings.HasPrefix(phase, "moved-")
			phase = strings.TrimPrefix(phase, "moved-")
			dir := filepath.Join(t.TempDir(), "ws")
			w, err := Create(dir, source, Options{})
			if err != nil {
				t.Fatal(err)
			}
			e = applyPlan(t, w, fieldPlan(t, w, "title", "title", "Title", "Reviewed"))
			id := randomID()
			stage := "staging/accept-" + id
			if err := w.root.Mkdir(stage, 0700); err != nil {
				t.Fatal(err)
			}
			tree, err := copyTree(w.root, candidate, stage+"/pub")
			if err != nil {
				t.Fatal(err)
			}
			if tree.SHA256 != rp.InputTreeSHA256 {
				t.Fatal("cached real check not bound to fixture tree")
			}
			r := Revision{Version: 1, ID: id, WorkspaceID: w.id, Parent: "initial", TaskID: e.TaskID, ExecutionSHA256: digest(e), Rootfile: w.state.Rootfile, Tree: tree, Validation: &rp}
			if err := writeJSON(w.root, stage+"/revision.json", r); err != nil {
				t.Fatal(err)
			}
			d := Decision{Version: 1, TaskID: e.TaskID, Status: "accepted", BaseRevision: "initial", RevisionID: id, TreeSHA256: tree.SHA256, Validation: &rp}
			j := settlement{Version: 1, WorkspaceID: w.id, Decision: d}
			if err := w.taskDigests("tasks/active", &j); err != nil {
				t.Fatal(err)
			}
			if phase != "before-intent" {
				if err := writeJSON(w.root, settlementJournal, j); err != nil {
					t.Fatal(err)
				}
			}
			if phase == "revision" || phase == "pointer" || phase == "decision" || phase == "archive" || phase == "clean" {
				if err := publish(w.root, stage, "revisions/"+id); err != nil {
					t.Fatal(err)
				}
			}
			if phase == "pointer" || phase == "decision" || phase == "archive" || phase == "clean" {
				if err := writeJSON(w.root, "accepted.json", acceptedPointer{1, w.id, id}); err != nil {
					t.Fatal(err)
				}
			}
			if phase == "decision" || phase == "archive" || phase == "clean" {
				if err := writeJSON(w.root, "tasks/active/decision.json", d); err != nil {
					t.Fatal(err)
				}
			}
			if phase == "archive" || phase == "clean" {
				if err := publish(w.root, "tasks/active", "tasks/"+e.TaskID); err != nil {
					t.Fatal(err)
				}
			}
			if phase == "clean" {
				if err := w.root.Remove(settlementJournal); err != nil {
					t.Fatal(err)
				}
			}
			if phase == "tamper" {
				put(t, filepath.Join(dir, stage, "pub", w.state.Rootfile), []byte("changed frozen bytes"))
			}
			w.Close()
			if moved {
				if err := os.Rename(dir, dir+"-moved"); err != nil {
					t.Fatal(err)
				}
				dir += "-moved"
			}
			w, err = Open(dir)
			if phase == "tamper" {
				if err == nil {
					w.Close()
					t.Fatal("tampered frozen tree recovered approval")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			if phase == "before-intent" {
				if w.current != "initial" || !exists(w.root, "tasks/active") {
					t.Fatal("pre-intent stage accepted")
				}
				return
			}
			if w.current != id || exists(w.root, "tasks/active") || exists(w.root, settlementJournal) {
				t.Fatal("journal did not finish", phase)
			}
			if !exists(w.root, "tasks/"+e.TaskID+"/decision.json") {
				t.Fatal("audit missing after recovery")
			}
		})
	}
}

func TestRevisionTamperRefusesReopen(t *testing.T) {
	requireChecker(t)
	w, dir, _ := legalWorkspace(t, "3.0")
	e := applyPlan(t, w, fieldPlan(t, w, "title", "title", "Title", "Reviewed"))
	d, err := w.Accept(context.Background(), e.TaskID, validation.Options{})
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	file := filepath.Join(dir, revisionPath(d.RevisionID), "EPUB/package.opf")
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	put(t, file, []byte(strings.Replace(string(b), "Reviewed", "Tampered", 1)))
	if opened, err := Open(dir); err == nil {
		opened.Close()
		t.Fatal("tampered revision inherited pass")
	}
	put(t, file, b)
	record := filepath.Join(dir, "tasks", e.TaskID, "edit-result.json")
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	put(t, record, bytes.Replace(data, []byte(`"conformance":"not_run"`), []byte(`"conformance":"passed"`), 1))
	if opened, err := Open(dir); err == nil {
		opened.Close()
		t.Fatal("tampered task source trusted")
	}
}

// This context observes real filesystem state, not a validation override. The
// final Err must see the prepared journal (after all expensive digest scans).
type cancelBeforeIntent struct {
	context.Context
	prepared string
	observed atomic.Bool
}

func (c *cancelBeforeIntent) Err() error {
	if _, err := os.Stat(c.prepared); err == nil {
		c.observed.Store(true)
		return context.Canceled
	}
	return c.Context.Err()
}
func TestAcceptCancellationAtIntentBoundary(t *testing.T) {
	requireChecker(t)
	w, dir, _ := legalWorkspace(t, "3.0")
	defer w.Close()
	e := applyPlan(t, w, fieldPlan(t, w, "title", "title", "Title", "Reviewed"))
	ctx := &cancelBeforeIntent{Context: context.Background(), prepared: filepath.Join(dir, "staging/settlement.json")}
	d, err := w.Accept(ctx, e.TaskID, validation.Options{})
	if !errors.Is(err, context.Canceled) || !ctx.observed.Load() || d.Status != "checks_passed" || d.RevisionID != "" {
		t.Fatalf("cancel boundary %+v %v observed=%v", d, err, ctx.observed.Load())
	}
	if w.current != "initial" || exists(w.root, settlementJournal) || exists(w.root, "accepted.json") {
		t.Fatal("pre-intent cancellation committed acceptance")
	}
	entries, err := os.ReadDir(filepath.Join(dir, "tasks/active/checks"))
	if err != nil || len(entries) != 1 {
		t.Fatal("missing check audit", err)
	}
	var check Decision
	if err := readEditJSON(w.root, "tasks/active/checks/"+entries[0].Name(), &check); err != nil {
		t.Fatal(err)
	}
	if check.Status != "checks_passed" || check.Validation.Status != "pass" || check.RevisionID != "" {
		t.Fatalf("check success confused with failure/acceptance %+v", check)
	}
	if _, err := os.Stat(ctx.prepared); !os.IsNotExist(err) {
		t.Fatal("cancelled preparation leaked")
	}
	w.Close()
	w, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{}); err != nil {
		t.Fatal("cancelled task could not be retried", err)
	}
}

func TestLegacyManualCandidateCanBeRejected(t *testing.T) {
	w, dir := makeWorkspace(t)
	if _, err := w.NewCandidate(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Checkpoint(); err != nil {
		t.Fatal(err)
	}
	if err := w.root.Remove("tasks/active/task.json"); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(w.root, "tasks/active/task.json", taskRecord{Version: 1, BaseRevision: "initial"}); err != nil {
		t.Fatal(err)
	}
	if err := w.root.Remove("identity.json"); err != nil {
		t.Fatal(err)
	}
	w.Close()
	w, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	r, err := w.TaskDiff("active")
	if err != nil || r.Diff.Changed || r.MatchesExecution {
		t.Fatalf("legacy review %+v %v", r, err)
	}
	if _, err := w.Reject("active"); err != nil {
		t.Fatal(err)
	}
	if w.current != "initial" {
		t.Fatal("legacy rejection advanced baseline")
	}
	applyPlan(t, w, planTitle(t, w, "New"))
}

func TestAcceptedSnapshotDoesNotExposeStateEntries(t *testing.T) {
	w, _ := makeWorkspace(t)
	defer w.Close()
	before := w.State()
	a, _, r, err := w.AcceptedSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	a.Close()
	r.Tree.Entries[0].Path = "caller-local-change"
	if digest(w.State()) != digest(before) {
		t.Fatal("snapshot exposed mutable import State")
	}
	if err := w.verifyBaseline(); err != nil {
		t.Fatal("caller mutation affected baseline", err)
	}
}

func TestAcceptedHistoryAfterWorkspaceMove(t *testing.T) {
	requireChecker(t)
	for _, content := range []bool{false, true} {
		t.Run(fmt.Sprintf("content=%v", content), func(t *testing.T) {
			w, dir, _ := legalWorkspace(t, "3.0")
			p := fieldPlan(t, w, "title", "title", "Title", "New")
			if content {
				p = contentPlan(t, w, "Reviewed")
			}
			e := applyPlan(t, w, p)
			d, err := w.Accept(context.Background(), e.TaskID, validation.Options{})
			if err != nil {
				t.Fatal(err)
			}
			oldPlan := fieldPlan(t, w, "creator", "creator", "Writer", "Next")
			w.Close()
			moved := dir + "-moved"
			if err := os.Rename(dir, moved); err != nil {
				t.Fatal(err)
			}
			w, err = Open(moved)
			if err != nil {
				t.Fatal("moved accepted history unavailable", err)
			}
			defer w.Close()
			if w.current != d.RevisionID {
				t.Fatal("move changed accepted revision")
			}
			if _, err := w.Apply(editJSON(t, oldPlan)); !errors.Is(err, ErrStalePlan) {
				t.Fatal("old absolute-path plan not stale", err)
			}
			a, _, r, err := w.AcceptedSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			a.Close()
			if r.ID != d.RevisionID {
				t.Fatal("snapshot lost revision")
			}
			applyPlan(t, w, fieldPlan(t, w, "creator", "creator", "Writer", "Next"))
		})
	}
}

func TestActiveTaskAfterWorkspaceMove(t *testing.T) {
	requireChecker(t)
	for _, content := range []bool{false, true} {
		for _, action := range []string{"reject", "accept", "interrupted"} {
			t.Run(fmt.Sprintf("content=%v/%s", content, action), func(t *testing.T) {
				w, dir, _ := legalWorkspace(t, "3.0")
				p := fieldPlan(t, w, "title", "title", "Title", "New")
				if content {
					p = contentPlan(t, w, "Reviewed")
				}
				e := applyPlan(t, w, p)
				if action == "interrupted" {
					if err := w.root.Remove("tasks/active/edit-result.json"); err != nil {
						t.Fatal(err)
					}
				}
				w.Close()
				moved := dir + "-moved"
				if err := os.Rename(dir, moved); err != nil {
					t.Fatal(err)
				}
				w, err := Open(moved)
				if err != nil {
					t.Fatal("moved active task unavailable", err)
				}
				defer w.Close()
				if action == "interrupted" {
					x, err := w.Execution()
					actual, hashErr := hashAt(w.root, candidate)
					if err != nil || hashErr != nil || x.Status != "failed" || actual.SHA256 != w.base.SHA256 {
						t.Fatal("moved interrupted mutation was not rolled back", x, err, hashErr)
					}
					if _, err := w.Reject(e.TaskID); err != nil {
						t.Fatal(err)
					}
					return
				}
				r, err := w.TaskDiff(e.TaskID)
				if err != nil || !r.MatchesExecution || len(r.Diff.Changes) != 1 {
					t.Fatal("lost actual execution review", r, err)
				}
				if _, err := w.Apply(editJSON(t, p)); !errors.Is(err, ErrStalePlan) {
					t.Fatal("moved old plan reusable", err)
				}
				if action == "accept" {
					d, err := w.Accept(context.Background(), e.TaskID, validation.Options{})
					if err != nil || d.Validation == nil || d.Validation.Status != "pass" || w.current != d.RevisionID {
						t.Fatal("moved task acceptance lost real checker gate", d, err)
					}
					return
				}
				if _, err := w.Reject(e.TaskID); err != nil {
					t.Fatal("moved active task not rejectable", err)
				}
			})
		}
	}
}

func TestContentHistoryReplayWithoutTempDirectory(t *testing.T) {
	requireChecker(t)
	w, dir, _ := legalWorkspace(t, "3.0")
	e := applyPlan(t, w, contentPlan(t, w, "Reviewed"))
	if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{}); err != nil {
		t.Fatal(err)
	}
	next := contentRequest(t, w, "EPUB/chapter.xhtml", "Next body")
	w.Close()
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	w, err := Open(dir)
	if err != nil {
		t.Fatal("history replay requires temp copy", err)
	}
	defer w.Close()
	if _, err := w.Plan(editJSON(t, next)); err != nil {
		t.Fatal("content planning requires temp copy", err)
	}
	applyPlan(t, w, fieldPlan(t, w, "creator", "creator", "Writer", "Next"))
}

func TestTaskDiffInvalidCandidateMimetype(t *testing.T) {
	for _, content := range []bool{false, true} {
		for _, missing := range []bool{false, true} {
			t.Run(fmt.Sprintf("content=%v/missing=%v", content, missing), func(t *testing.T) {
				w, dir, _ := legalWorkspace(t, "3.0")
				defer w.Close()
				p := fieldPlan(t, w, "title", "title", "Title", "New")
				if content {
					p = contentPlan(t, w, "Reviewed")
				}
				e := applyPlan(t, w, p)
				mime := filepath.Join(dir, candidate, "mimetype")
				if missing {
					if err := os.Remove(mime); err != nil {
						t.Fatal(err)
					}
				} else {
					put(t, mime, []byte("application/x-drift"))
				}
				r, err := w.TaskDiff(e.TaskID)
				if err != nil || r.MatchesExecution || len(r.Diff.Changes) != 2 {
					t.Fatalf("drift review unavailable: %+v %v", r, err)
				}
				if content {
					if r.Content == nil || r.Content.NewValue != nil || r.Content.Unavailable == "" {
						t.Fatal("missing content unavailable reason", r)
					}
				} else if r.Metadata.NewValue != nil || r.Metadata.Unavailable == "" {
					t.Fatal("missing metadata unavailable reason", r)
				}
				if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{}); !errors.Is(err, ErrCandidateDrift) {
					t.Fatal("invalid candidate accepted", err)
				}
				if _, err := w.Reject(e.TaskID); err != nil {
					t.Fatal("invalid candidate not rejectable", err)
				}
			})
		}
	}
}
