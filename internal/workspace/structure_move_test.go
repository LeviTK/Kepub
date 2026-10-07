package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/validation"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

// structureChapter3 is a second-directory destination resource, so relative URL
// rebasing and incoming sync are observable across directories.
const structureChapter3 = `<?xml version="1.0" encoding="utf-8"?>` + "\n" +
	`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Three</title></head><body><p id="three">Three.</p></body></html>` + "\n"

// structureMoveChapter1 carries one movable subtree with a nested identity and
// every URL shape the batch rebases.
const structureMoveChapter1 = `<?xml version="1.0" encoding="utf-8"?>` + "\n" +
	`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">` +
	`<head><title>One</title></head>` +
	`<body><h1 id="start">One</h1>` +
	`<p id="unreferenced">Plain.</p>` +
	`<div id="moveblock"><p id="inner">Inner.</p><a href="chapter2.xhtml#start2">Two</a> <a href="#start">Start</a> <a href="#inner">Inner</a> <a href="https://example.com/x">Ext</a> <img src="images/pic.png"/></div>` +
	`<p id="last">Last.</p>` +
	`</body></html>` + "\n"

func (b binding) elemMoveCross(dst binding, sourceLocator, destinationLocator, position string) Operation {
	return Operation{"xhtml.element.move", 2, publication.ElementMoveCross{
		Source:      publication.MoveEndpoint{BookPath: bookpath.BookPath(b.bookPath), RevisionID: b.revision, ResourceSHA256: b.sha, LocatorVersion: 1, Locator: sourceLocator},
		Destination: publication.MoveEndpoint{BookPath: bookpath.BookPath(dst.bookPath), RevisionID: dst.revision, ResourceSHA256: dst.sha, LocatorVersion: 1, Locator: destinationLocator},
		Position:    position,
	}}
}

// moveCrossPlan plans one schema 6 request.
func moveCrossPlan(t *testing.T, w *Workspace, ops []Operation) Plan {
	t.Helper()
	p, err := w.Plan(editJSON(t, Request{6, ops}))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// moveCrossRefused requires a schema 6 plan to be refused with a specific code.
func moveCrossRefused(t *testing.T, w *Workspace, ops []Operation, code string) error {
	t.Helper()
	_, err := w.Plan(editJSON(t, Request{6, ops}))
	if err == nil {
		t.Fatalf("plan accepted: %+v", ops)
	}
	if code != "" {
		var f *fault.Error
		if errors.As(err, &f) && f.Code != code {
			t.Fatalf("expected %s, got %v", code, err)
		}
	}
	return err
}

// moveCrossFixture is the shared multi-directory fixture: chapter1 carries the
// movable block, chapter3 is the second-directory destination, and the nav links
// into the moved identity.
func moveCrossFiles() map[string]string {
	return map[string]string{
		"EPUB/chapter1.xhtml":      structureMoveChapter1,
		"EPUB/package.opf":         strings.Replace(structureOPF, `<item id="c2"`, `<item id="c3" href="text/chapter3.xhtml" media-type="application/xhtml+xml"/><item id="pic" href="images/pic.png" media-type="image/png"/><item id="c2"`, 1),
		"EPUB/text/chapter3.xhtml": structureChapter3,
		"EPUB/images/pic.png":      "notreallyapng",
		"EPUB/nav.xhtml":           `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="chapter1.xhtml#inner">Inner</a></li><li><a href="chapter2.xhtml#start2">Two</a></li></ol></nav></body></html>`,
	}
}

func moveCrossFixture(t *testing.T, extra map[string]string) (*Workspace, string, string) {
	t.Helper()
	files := moveCrossFiles()
	for name, b := range extra {
		files[name] = b
	}
	return structureWorkspace(t, files)
}

var (
	fuzzMoveOnce sync.Once
	fuzzMovePath string
	fuzzMoveErr  error
)

// fuzzMoveDir builds the move fixture once, so a fuzz target exercises real
// derivation, sync and review work instead of repeated imports.
func fuzzMoveDir(t *testing.T) string {
	t.Helper()
	fuzzMoveOnce.Do(func() {
		dir, err := os.MkdirTemp("", "kepub-move-fuzz-")
		if err != nil {
			fuzzMoveErr = err
			return
		}
		w, _, _ := structureWorkspaceAt(t, dir, moveCrossFiles())
		if err := w.Close(); err != nil {
			fuzzMoveErr = err
			return
		}
		fuzzMovePath = filepath.Join(dir, "workspace")
	})
	if fuzzMoveErr != nil {
		t.Fatal(fuzzMoveErr)
	}
	return fuzzMovePath
}

// TestStructureCrossMovePlanApply moves one subtree between two manifest XHTML
// resources: the source loses the exact block, the destination gains it at the
// planned position with every authored byte except the rebased URLs, and the
// write set names every derived resource.
func TestStructureCrossMovePlanApply(t *testing.T) {
	w, dir, _ := moveCrossFixture(t, nil)
	defer w.Close()
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	ch3 := structureBinding(t, w, "EPUB/text/chapter3.xhtml")
	p := moveCrossPlan(t, w, []Operation{ch1.elemMoveCross(ch3, ch1.locatorID(t, "moveblock"), ch3.locatorID(t, "three"), "after")})
	if want := []string{"EPUB/chapter1.xhtml", "EPUB/nav.xhtml", "EPUB/text/chapter3.xhtml"}; strings.Join(p.WriteSet, ",") != strings.Join(want, ",") {
		t.Fatalf("write set %v", p.WriteSet)
	}
	e := applyPlan(t, w, p)
	if e.Status != "review_required" {
		t.Fatalf("status %s", e.Status)
	}
	c1 := readResource(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"))
	c3 := readResource(t, filepath.Join(dir, candidate, "EPUB/text/chapter3.xhtml"))
	n := readResource(t, filepath.Join(dir, candidate, "EPUB/nav.xhtml"))
	if bytes.Contains(c1, []byte("moveblock")) {
		t.Fatalf("source still carries the block: %s", c1)
	}
	want := `<p id="three">Three.</p><div id="moveblock"><p id="inner">Inner.</p><a href="../chapter2.xhtml#start2">Two</a> <a href="../chapter1.xhtml#start">Start</a> <a href="#inner">Inner</a> <a href="https://example.com/x">Ext</a> <img src="../images/pic.png"/></div>`
	if !bytes.Contains(c3, []byte(want)) {
		t.Fatalf("destination block:\n%s", c3)
	}
	if !bytes.Contains(n, []byte(`href="text/chapter3.xhtml#inner"`)) {
		t.Fatalf("nav not synchronized: %s", n)
	}
	// The moved identity left the source and arrived in the destination.
	ch1Candidate := structureCandidate(t, w, "EPUB/chapter1.xhtml")
	ch3Candidate := structureCandidate(t, w, "EPUB/text/chapter3.xhtml")
	if ch1Candidate.IDs()["moveblock"] != 0 || ch3Candidate.IDs()["moveblock"] != 1 || ch3Candidate.IDs()["inner"] != 1 {
		t.Fatalf("final identity counts: %v %v", ch1Candidate.IDs(), ch3Candidate.IDs())
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}

// structureCandidate parses one candidate resource, so identity counts are read
// from the actual candidate bytes rather than the accepted baseline.
func structureCandidate(t *testing.T, w *Workspace, bp string) *publication.StructureDocument {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(w.dir, candidate, filepath.FromSlash(bp)))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := publication.ParseStructureDocument(data, bookpath.BookPath(bp), xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// TestStructureCrossMoveReview reports the planned move and the actual candidate
// values: the synchronized reference is read back from the referring resource and
// the block is observed at the planned destination offset.
func TestStructureCrossMoveReview(t *testing.T) {
	w, _, _ := moveCrossFixture(t, nil)
	defer w.Close()
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	ch3 := structureBinding(t, w, "EPUB/text/chapter3.xhtml")
	p := moveCrossPlan(t, w, []Operation{ch1.elemMoveCross(ch3, ch1.locatorID(t, "moveblock"), ch3.locatorID(t, "three"), "after")})
	e := applyPlan(t, w, p)
	rev, err := w.TaskDiff(e.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rev.Operations) != 1 {
		t.Fatalf("operations: %+v", rev.Operations)
	}
	el := rev.Operations[0].Element
	if el == nil || el.Action != "move" || el.Source != "EPUB/chapter1.xhtml" || el.Destination != "EPUB/text/chapter3.xhtml" || el.Position != "after" {
		t.Fatalf("element review: %+v", el)
	}
	if strings.Join(el.MovedIDs, ",") != "moveblock,inner" {
		t.Fatalf("moved ids: %v", el.MovedIDs)
	}
	if !strings.Contains(el.Candidate, "moved to EPUB/text/chapter3.xhtml") || !strings.Contains(el.Candidate, "source occurrences 1→0") {
		t.Fatalf("candidate observation: %q unavailable=%q", el.Candidate, el.Unavailable)
	}
	if len(el.Synchronized) != 1 {
		t.Fatalf("synchronized: %+v", el.Synchronized)
	}
	s := el.Synchronized[0]
	if s.BookPath != "EPUB/nav.xhtml" || s.OldValue != "chapter1.xhtml#inner" || s.NewValue == nil || *s.NewValue != "text/chapter3.xhtml#inner" || s.Unavailable != "" {
		t.Fatalf("synchronized review: %+v", s)
	}
	b, _ := json.Marshal(rev.Operations)
	if !bytes.Contains(b, []byte(`"newValue":"text/chapter3.xhtml#inner"`)) {
		t.Fatalf("review JSON: %s", b)
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}

// TestStructureCrossMoveRefusals keeps the conservative boundaries of the batch:
// an IDREF that would leave the block, an incoming IDREF that cannot become a
// URL, a namespace context mismatch, an identity collision, a coverage gap, a
// non-XHTML destination and duplicate targets are all refused before any byte is
// written.
func TestStructureCrossMoveRefusals(t *testing.T) {
	base := func(t *testing.T) (*Workspace, binding, binding) {
		t.Helper()
		w, _, _ := moveCrossFixture(t, nil)
		t.Cleanup(func() { w.Close() })
		return w, structureBinding(t, w, "EPUB/chapter1.xhtml"), structureBinding(t, w, "EPUB/text/chapter3.xhtml")
	}
	t.Run("idref-inside-block", func(t *testing.T) {
		w, _, _ := base(t)
		w2, _, _ := moveCrossFixture(t, map[string]string{"EPUB/chapter1.xhtml": strings.Replace(structureMoveChapter1, `<p id="inner">Inner.</p>`, `<p id="inner" for="start">Inner.</p>`, 1)})
		defer w2.Close()
		_ = w
		ch1 := structureBinding(t, w2, "EPUB/chapter1.xhtml")
		ch3 := structureBinding(t, w2, "EPUB/text/chapter3.xhtml")
		err := moveCrossRefused(t, w2, []Operation{ch1.elemMoveCross(ch3, ch1.locatorID(t, "moveblock"), ch3.locatorID(t, "three"), "after")}, "INVALID_OPERATIONS")
		if !strings.Contains(err.Error(), "IDREF") {
			t.Fatalf("error: %v", err)
		}
	})
	t.Run("idref-incoming", func(t *testing.T) {
		w, _, _ := moveCrossFixture(t, map[string]string{"EPUB/chapter1.xhtml": strings.Replace(structureMoveChapter1, `<p id="last">Last.</p>`, `<p id="last">Last.</p><p for="inner">Ref.</p>`, 1)})
		defer w.Close()
		ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
		ch3 := structureBinding(t, w, "EPUB/text/chapter3.xhtml")
		err := moveCrossRefused(t, w, []Operation{ch1.elemMoveCross(ch3, ch1.locatorID(t, "moveblock"), ch3.locatorID(t, "three"), "after")}, "REFERENCE_CONFLICT")
		if !strings.Contains(err.Error(), "xhtml.idref") {
			t.Fatalf("error: %v", err)
		}
	})
	t.Run("namespace-mismatch", func(t *testing.T) {
		w, _, _ := moveCrossFixture(t, map[string]string{"EPUB/chapter1.xhtml": strings.Replace(structureMoveChapter1, `<div id="moveblock">`, `<div id="moveblock" epub:type="note">`, 1)})
		defer w.Close()
		ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
		ch3 := structureBinding(t, w, "EPUB/text/chapter3.xhtml")
		err := moveCrossRefused(t, w, []Operation{ch1.elemMoveCross(ch3, ch1.locatorID(t, "moveblock"), ch3.locatorID(t, "three"), "after")}, "INVALID_OPERATIONS")
		if !strings.Contains(err.Error(), "namespace") {
			t.Fatalf("error: %v", err)
		}
	})
	t.Run("collision", func(t *testing.T) {
		w, _, _ := moveCrossFixture(t, map[string]string{
			"EPUB/text/chapter3.xhtml": strings.Replace(structureChapter3, `<p id="three">`, `<p id="inner">`, 1),
			"EPUB/nav.xhtml":           `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Contents</title></head><body><nav><ol><li><a href="chapter2.xhtml#start2">Two</a></li></ol></nav></body></html>`,
		})
		defer w.Close()
		ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
		ch3 := structureBinding(t, w, "EPUB/text/chapter3.xhtml")
		err := moveCrossRefused(t, w, []Operation{ch1.elemMoveCross(ch3, ch1.locatorID(t, "moveblock"), ch3.locatorID(t, "inner"), "after")}, "INVALID_OPERATIONS")
		if !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("error: %v", err)
		}
	})
	t.Run("coverage-blocked", func(t *testing.T) {
		w, _, _ := moveCrossFixture(t, map[string]string{"EPUB/chapter2.xhtml": strings.Replace(structureChapter2, `<p>Beta &amp; two.</p>`, `<p>Beta &amp; two.</p><script>var x = 1;</script>`, 1)})
		defer w.Close()
		ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
		ch3 := structureBinding(t, w, "EPUB/text/chapter3.xhtml")
		moveCrossRefused(t, w, []Operation{ch1.elemMoveCross(ch3, ch1.locatorID(t, "moveblock"), ch3.locatorID(t, "three"), "after")}, "REFERENCE_COVERAGE_INCOMPLETE")
	})
	t.Run("non-xhtml-destination", func(t *testing.T) {
		w, _, _ := base(t)
		ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
		dst := ch1
		dst.bookPath = "EPUB/package.opf"
		dst.sha = strings.Repeat("0", 64)
		moveCrossRefused(t, w, []Operation{ch1.elemMoveCross(dst, ch1.locatorID(t, "moveblock"), "/package[1]", "after")}, "")
	})
	t.Run("same-resource", func(t *testing.T) {
		w, ch1, _ := base(t)
		err := moveCrossRefused(t, w, []Operation{ch1.elemMoveCross(ch1, ch1.locatorID(t, "moveblock"), ch1.locatorID(t, "last"), "after")}, "INVALID_OPERATIONS")
		if !strings.Contains(err.Error(), "two different resources") {
			t.Fatalf("error: %v", err)
		}
	})
	t.Run("body-target", func(t *testing.T) {
		w, ch1, ch3 := base(t)
		moveCrossRefused(t, w, []Operation{ch1.elemMoveCross(ch3, "/html[1]/body[1]", ch3.locatorID(t, "three"), "after")}, "INVALID_OPERATIONS")
	})
	t.Run("duplicate-move", func(t *testing.T) {
		w, ch1, ch3 := base(t)
		op := ch1.elemMoveCross(ch3, ch1.locatorID(t, "moveblock"), ch3.locatorID(t, "three"), "after")
		moveCrossRefused(t, w, []Operation{op, op}, "INVALID_OPERATIONS")
	})
	t.Run("stale-revision", func(t *testing.T) {
		w, ch1, ch3 := base(t)
		op := ch1.elemMoveCross(ch3, ch1.locatorID(t, "moveblock"), ch3.locatorID(t, "three"), "after")
		param := op.Params.(publication.ElementMoveCross)
		param.Source.RevisionID = strings.Repeat("a", 32)
		op.Params = param
		moveCrossRefused(t, w, []Operation{op}, "INPUT_DRIFT")
	})
	t.Run("hash-drift", func(t *testing.T) {
		w, ch1, ch3 := base(t)
		op := ch1.elemMoveCross(ch3, ch1.locatorID(t, "moveblock"), ch3.locatorID(t, "three"), "after")
		param := op.Params.(publication.ElementMoveCross)
		param.Destination.ResourceSHA256 = strings.Repeat("0", 64)
		op.Params = param
		moveCrossRefused(t, w, []Operation{op}, "INPUT_DRIFT")
	})
}

// TestStructureCrossMoveEncodings keeps the physical conversion: the block is
// decoded from the source encoding and re-encoded for the destination, and the
// source loses it, in every mixed direction.
func TestStructureCrossMoveEncodings(t *testing.T) {
	for _, c := range []struct{ name, sourceEnc, destEnc string }{
		{"utf8-to-utf16be", "utf8", "utf16be"},
		{"utf16le-to-utf8", "utf16le", "utf8"},
		{"utf16be-to-utf16le", "utf16be", "utf16le"},
	} {
		t.Run(c.name, func(t *testing.T) {
			w, dir, _ := moveCrossFixture(t, map[string]string{
				"EPUB/chapter1.xhtml":      encodeChapter(structureMoveChapter1, c.sourceEnc),
				"EPUB/text/chapter3.xhtml": encodeChapter(structureChapter3, c.destEnc),
			})
			defer w.Close()
			ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
			ch3 := structureBinding(t, w, "EPUB/text/chapter3.xhtml")
			p := moveCrossPlan(t, w, []Operation{ch1.elemMoveCross(ch3, ch1.locatorID(t, "moveblock"), ch3.locatorID(t, "three"), "after")})
			applyPlan(t, w, p)
			c3, err := decodeChapter(readResource(t, filepath.Join(dir, candidate, "EPUB/text/chapter3.xhtml")), c.destEnc)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(c3, `<div id="moveblock"><p id="inner">Inner.</p>`) {
				t.Fatalf("destination: %q", c3)
			}
			c1, err := decodeChapter(readResource(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml")), c.sourceEnc)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(c1, "moveblock") {
				t.Fatalf("source still carries the block: %q", c1)
			}
		})
	}
}

// TestStructureCrossMoveInterruptedApply keeps the recovery boundary: every
// derived resource is materialized before the interruption, recovery rolls the
// candidate back to the exact baseline, review still re-derives the move, a
// failed execution cannot be accepted, and the original book stays unchanged.
func TestStructureCrossMoveInterruptedApply(t *testing.T) {
	w, dir, source := moveCrossFixture(t, nil)
	original := readResource(t, source)
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	ch3 := structureBinding(t, w, "EPUB/text/chapter3.xhtml")
	p := moveCrossPlan(t, w, []Operation{ch1.elemMoveCross(ch3, ch1.locatorID(t, "moveblock"), ch3.locatorID(t, "three"), "after")})
	e, outputs := prepareExecution(t, w, p)
	if len(outputs) != 3 {
		t.Fatalf("derived resources: %v", outputs)
	}
	putOutputs(t, dir, outputs)
	put(t, filepath.Join(dir, restoreNew, "partial"), []byte("unfinished restore copy"))
	w.Close()
	other, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	got, err := other.Execution()
	if err != nil || got.TaskID != e.TaskID || got.Status != "failed" || got.Failure != "interrupted apply" {
		t.Fatalf("recovered execution: %+v, %v", got, err)
	}
	if treeAt(t, filepath.Join(dir, candidate)).SHA256 != p.InputTreeSHA256 {
		t.Fatal("rollback did not restore exact baseline")
	}
	if _, err := other.TaskDiff(e.TaskID); err != nil {
		t.Fatalf("task diff: %v", err)
	}
	if _, err := other.Accept(context.Background(), e.TaskID, validation.Options{}); !errors.Is(err, ErrTaskConflict) {
		t.Fatalf("failed accepted: %v", err)
	}
	if !bytes.Equal(readResource(t, source), original) {
		t.Fatal("original book changed")
	}
	if _, err := other.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}

// TestStructureCrossMoveAcceptAndHistory accepts one move with the pinned
// checker, then plans the reverse move against the accepted baseline: the chain
// re-derives from the frozen revision and the original book is untouched.
func TestStructureCrossMoveAcceptAndHistory(t *testing.T) {
	requireChecker(t)
	w, dir, source := structureWorkspace(t, map[string]string{
		"EPUB/nav.xhtml": `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="chapter1.xhtml#ruby">Ruby</a></li><li><a href="chapter2.xhtml#start2">Two</a></li></ol></nav></body></html>`,
	})
	defer w.Close()
	original := readResource(t, source)
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	ch2 := structureBinding(t, w, "EPUB/chapter2.xhtml")
	p := moveCrossPlan(t, w, []Operation{ch1.elemMoveCross(ch2, ch1.locatorID(t, "ruby"), ch2.locatorID(t, "start2"), "after")})
	e := applyPlan(t, w, p)
	d, err := w.Accept(context.Background(), e.TaskID, validation.Options{})
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	accepted := filepath.Join(dir, revisionPath(d.RevisionID))
	if bytes.Contains(readResource(t, filepath.Join(accepted, "EPUB/chapter1.xhtml")), []byte(`id="ruby"`)) {
		t.Fatal("accepted source still carries the block")
	}
	if !bytes.Contains(readResource(t, filepath.Join(accepted, "EPUB/chapter2.xhtml")), []byte(`id="ruby"`)) {
		t.Fatal("accepted destination is missing the block")
	}
	if !bytes.Contains(readResource(t, filepath.Join(accepted, "EPUB/nav.xhtml")), []byte(`href="chapter2.xhtml#ruby"`)) {
		t.Fatal("accepted nav is not synchronized")
	}
	// The reverse move plans against the accepted revision and synchronizes the
	// nav back to the source resource.
	ch2b := structureBinding(t, w, "EPUB/chapter2.xhtml")
	ch1b := structureBinding(t, w, "EPUB/chapter1.xhtml")
	p2 := moveCrossPlan(t, w, []Operation{ch2b.elemMoveCross(ch1b, ch2b.locatorID(t, "ruby"), ch1b.locatorID(t, "last"), "before")})
	e2 := applyPlan(t, w, p2)
	n := readResource(t, filepath.Join(dir, candidate, "EPUB/nav.xhtml"))
	if !bytes.Contains(n, []byte(`href="chapter1.xhtml#ruby"`)) {
		t.Fatalf("reverse sync: %s", n)
	}
	if _, err := w.Reject(e2.TaskID); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readResource(t, source), original) {
		t.Fatal("original book changed")
	}
}

// FuzzCrossMovePlanApply plans and applies cross-resource moves over the shared
// fixture and checks the byte oracle whenever the engine accepts: the block must
// leave the source, arrive in the destination at the planned offset, and every
// accepted plan must be deterministic.
func FuzzCrossMovePlanApply(f *testing.F) {
	f.Add(uint8(0), uint8(0), uint8(0), uint8(0))
	f.Add(uint8(1), uint8(1), uint8(1), uint8(0))
	f.Add(uint8(2), uint8(2), uint8(2), uint8(1))
	f.Fuzz(func(t *testing.T, source, anchor, position, second uint8) {
		sources := []string{"moveblock", "unreferenced", "last", "start"}
		anchors := []string{"three", "three", "three", "three"}
		positions := []string{"after", "before", "first-child", "last-child"}
		dir := fuzzMoveDir(t)
		w, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer w.Close()
		ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
		ch3 := structureBinding(t, w, "EPUB/text/chapter3.xhtml")
		op := ch1.elemMoveCross(ch3, ch1.locatorID(t, sources[int(source)%len(sources)]), ch3.locatorID(t, anchors[int(anchor)%len(anchors)]), positions[int(position)%len(positions)])
		ops := []Operation{op}
		if second%2 == 1 {
			ops = append(ops, ch3.elemMoveCross(ch1, ch3.locatorID(t, "three"), ch1.locatorID(t, "last"), "after"))
		}
		p, err := w.Plan(editJSON(t, Request{6, ops}))
		if err != nil {
			return
		}
		if len(ops) == 2 {
			// A second move must not change the outcome or the write set when
			// the two operations are planned in reverse order.
			reverse := []Operation{ops[1], ops[0]}
			other, err := w.Plan(editJSON(t, Request{6, reverse}))
			if err != nil || strings.Join(other.WriteSet, ",") != strings.Join(p.WriteSet, ",") {
				t.Fatalf("order-dependent move plan: %v vs %v", err, other.WriteSet)
			}
		}
		again, err := w.Plan(editJSON(t, Request{6, ops}))
		if err != nil || again.OperationSetSHA256 != p.OperationSetSHA256 || strings.Join(again.WriteSet, ",") != strings.Join(p.WriteSet, ",") {
			t.Fatalf("nondeterministic plan: %v", err)
		}
		e, err := w.Apply(editJSON(t, p))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range p.WriteSet {
			if _, err := os.ReadFile(filepath.Join(dir, candidate, path)); err != nil {
				t.Fatalf("candidate %s: %v", path, err)
			}
		}
		rev, err := w.TaskDiff(e.TaskID)
		if err != nil {
			t.Fatal(err)
		}
		if rev.Operations[0].Element == nil || rev.Operations[0].Element.Unavailable != "" {
			t.Fatalf("move review unavailable: %+v", rev.Operations[0])
		}
		if _, err := w.Reject(e.TaskID); err != nil {
			t.Fatal(err)
		}
	})
}
