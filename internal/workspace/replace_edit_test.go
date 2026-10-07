package workspace

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/validation"
)

func (b binding) replace(locator, mode, pattern, replacement string, hits int) Operation {
	return Operation{"content.text.replace", 1, publication.TextReplace{BookPath: bookpath.BookPath(b.bookPath), RevisionID: b.revision, ResourceSHA256: b.sha, LocatorVersion: 1, Locator: locator, Mode: mode, Pattern: pattern, Replacement: replacement, ExpectedHits: hits}}
}

func replacePlan(t *testing.T, w *Workspace, ops []Operation) Plan {
	t.Helper()
	p, err := w.Plan(editJSON(t, Request{5, ops}))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func replaceRefused(t *testing.T, w *Workspace, ops []Operation, code string) error {
	t.Helper()
	_, err := w.Plan(editJSON(t, Request{5, ops}))
	if err == nil {
		t.Fatalf("plan accepted: %+v", ops)
	}
	var f *fault.Error
	if errors.As(err, &f) {
		if f.Code != code {
			t.Fatalf("expected %s, got %v", code, err)
		}
		return err
	}
	if code != "INVALID_OPERATIONS" {
		t.Fatalf("expected %s, got %v", code, err)
	}
	return err
}

func utf16LEString(s string) string {
	return string(utf16LEBytes(s, true))
}

func utf16LEBytes(s string, bom bool) []byte {
	out := []byte{}
	if bom {
		out = append(out, 0xff, 0xfe)
	}
	for _, r := range s {
		if r > 0xffff {
			continue
		}
		out = append(out, byte(r), byte(r>>8))
	}
	return out
}

// TestReplacePlanApplyReviewAccept keeps the whole chain on one batch replace:
// the frozen rule and hit count decide the write set, review observes the actual
// candidate direct text, and accept re-derives the same write set.
func TestReplacePlanApplyReviewAccept(t *testing.T) {
	w, dir, original := structureWorkspace(t, nil)
	defer w.Close()
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	base := readResource(t, filepath.Join(dir, revision, "EPUB/chapter1.xhtml"))
	p := replacePlan(t, w, []Operation{ch1.replace(ch1.locatorID(t, "dir"), "literal", "RTL", "Right-to-left", 1)})
	if p.SchemaVersion != 5 || !reflect.DeepEqual(p.WriteSet, []string{"EPUB/chapter1.xhtml"}) {
		t.Fatalf("plan: %+v", p)
	}
	e := applyPlan(t, w, p)
	cand := readResource(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"))
	want := bytes.Replace(base, []byte(">RTL text<"), []byte(">Right-to-left text<"), 1)
	if !bytes.Equal(cand, want) {
		t.Fatalf("candidate:\n got %q\nwant %q", cand, want)
	}
	r, err := w.TaskDiff(e.TaskID)
	if err != nil || !r.MatchesExecution || len(r.Operations) != 1 || r.Operations[0].Replace == nil {
		t.Fatalf("review: %+v %v", r, err)
	}
	rep := r.Operations[0].Replace
	if rep.Mode != "literal" || rep.ExpectedHits != 1 || rep.Hits != 1 || len(rep.Nodes) != 1 {
		t.Fatalf("replace review: %+v", rep)
	}
	if rep.Nodes[0].Locator != ch1.locatorID(t, "dir") || rep.Nodes[0].NewValue == nil || *rep.Nodes[0].NewValue != "Right-to-left text" {
		t.Fatalf("replace node review: %+v", rep.Nodes[0])
	}
	assertBytes(t, filepath.Join(dir, revision, "EPUB/chapter1.xhtml"), base)
	assertBytes(t, original, readResource(t, original))
	d, err := w.Accept(context.Background(), e.TaskID, validation.Options{})
	if err != nil || d.Status != "accepted" || d.Validation == nil || d.Validation.Status != "pass" {
		t.Fatalf("accept: %+v %v", d, err)
	}
	assertBytes(t, filepath.Join(dir, revisionPath(d.RevisionID), "EPUB/chapter1.xhtml"), cand)
}

// TestReplaceWithStructureTransaction keeps schema 5 a real transaction: a
// replace and a structural write in one request produce one write set and one
// review entry per operation.
func TestReplaceWithStructureTransaction(t *testing.T) {
	w, dir, _ := structureWorkspace(t, nil)
	defer w.Close()
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	ops := []Operation{
		ch1.replace(ch1.locatorID(t, "dir"), "literal", "RTL", "R", 1),
		ch1.attrSet(ch1.locatorID(t, "mixed"), "dir", nil, "ltr"),
	}
	p := replacePlan(t, w, ops)
	if !reflect.DeepEqual(p.WriteSet, []string{"EPUB/chapter1.xhtml"}) {
		t.Fatalf("write set %v", p.WriteSet)
	}
	e := applyPlan(t, w, p)
	cand := readResource(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"))
	if !bytes.Contains(cand, []byte(">R text<")) || !bytes.Contains(cand, []byte(`<p id="mixed" dir="ltr">`)) {
		t.Fatalf("candidate %s", cand)
	}
	r, err := w.TaskDiff(e.TaskID)
	if err != nil || len(r.Operations) != 2 {
		t.Fatalf("review: %+v %v", r, err)
	}
	if r.Operations[0].Replace == nil || r.Operations[0].Replace.Hits != 1 {
		t.Fatalf("replace entry: %+v", r.Operations[0])
	}
	if r.Operations[1].Attribute == nil || r.Operations[1].Attribute.NewValue == nil || *r.Operations[1].Attribute.NewValue != "ltr" {
		t.Fatalf("attribute entry: %+v", r.Operations[1])
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}

// TestReplaceRefusals covers the frozen boundary: hit count, empty matches,
// target scope, duplicate targets, stale binding and non-writable matches.
func TestReplaceRefusals(t *testing.T) {
	w, _, _ := structureWorkspace(t, nil)
	defer w.Close()
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	cases := []struct {
		name string
		ops  []Operation
		code string
	}{
		{"hit-count", []Operation{ch1.replace(ch1.locatorID(t, "dir"), "literal", "text", "x", 2)}, "INVALID_OPERATIONS"},
		{"empty-pattern", []Operation{ch1.replace(ch1.locatorID(t, "dir"), "literal", "", "x", 0)}, "INVALID_OPERATIONS"},
		{"empty-regex", []Operation{ch1.replace(ch1.locatorID(t, "dir"), "regex", "t*", "x", 0)}, "INVALID_OPERATIONS"},
		{"head-target", []Operation{ch1.replace(ch1.locator(t, "head", 0), "literal", "One", "x", 0)}, "INVALID_OPERATIONS"},
		{"root-target", []Operation{ch1.replace("/html[1]", "literal", "One", "x", 0)}, "INVALID_OPERATIONS"},
		{"duplicate-target", []Operation{ch1.replace(ch1.locatorID(t, "dir"), "literal", "RTL", "A", 1), ch1.replace(ch1.locatorID(t, "dir"), "literal", "text", "B", 1)}, "INVALID_OPERATIONS"},
		{"cross-run", []Operation{ch1.replace(ch1.locatorID(t, "mixed"), "literal", "Alpha  & beta", "x", 1)}, "INVALID_OPERATIONS"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			replaceRefused(t, w, c.ops, c.code)
		})
	}
	t.Run("stale-revision", func(t *testing.T) {
		op := ch1.replace(ch1.locatorID(t, "dir"), "literal", "RTL", "x", 1)
		param := op.Params.(publication.TextReplace)
		param.RevisionID = strings.Repeat("a", 32)
		op.Params = param
		// A stale revision is the frozen stale-plan signal; the CLI maps it to
		// INPUT_DRIFT (exit 4), exactly as schema 4 does.
		if _, err := w.Plan(editJSON(t, Request{5, []Operation{op}})); !errors.Is(err, ErrStalePlan) {
			t.Fatalf("expected stale plan, got %v", err)
		}
	})
	t.Run("resource-drift", func(t *testing.T) {
		op := ch1.replace(ch1.locatorID(t, "dir"), "literal", "RTL", "x", 1)
		param := op.Params.(publication.TextReplace)
		param.ResourceSHA256 = strings.Repeat("1", 64)
		op.Params = param
		replaceRefused(t, w, []Operation{op}, "INPUT_DRIFT")
	})
}

// TestReplaceNoop keeps an explicit zero-hit operation a complete no-op: it
// plans, produces an empty write set and still has a full review record.
func TestReplaceNoop(t *testing.T) {
	w, _, _ := structureWorkspace(t, nil)
	defer w.Close()
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	p := replacePlan(t, w, []Operation{ch1.replace(ch1.locatorID(t, "dir"), "literal", "absent", "x", 0)})
	if len(p.WriteSet) != 0 {
		t.Fatalf("write set %v", p.WriteSet)
	}
	e := applyPlan(t, w, p)
	r, err := w.TaskDiff(e.TaskID)
	if err != nil || len(r.Operations) != 1 || r.Operations[0].Replace == nil || r.Operations[0].Replace.Hits != 0 || len(r.Operations[0].Replace.Nodes) != 0 {
		t.Fatalf("noop review: %+v %v", r, err)
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}

// TestReplaceUTF16EndToEnd keeps the physical encoding: a UTF-16LE chapter is
// replaced in place, its BOM and declaration stay, and untouched bytes survive.
func TestReplaceUTF16EndToEnd(t *testing.T) {
	chapter := utf16LEString(strings.Replace(structureChapter1, `encoding="utf-8"`, `encoding="utf-16"`, 1))
	w, dir, _ := structureWorkspace(t, map[string]string{"EPUB/chapter1.xhtml": chapter})
	defer w.Close()
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	base := readResource(t, filepath.Join(dir, revision, "EPUB/chapter1.xhtml"))
	p := replacePlan(t, w, []Operation{ch1.replace(ch1.locatorID(t, "dir"), "literal", "RTL", "R", 1)})
	e := applyPlan(t, w, p)
	cand := readResource(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"))
	if cand[0] != 0xff || cand[1] != 0xfe {
		t.Fatalf("BOM lost: %x", cand[:2])
	}
	want := bytes.Replace(base, utf16LEBytes(">RTL text<", false), utf16LEBytes(">R text<", false), 1)
	if !bytes.Equal(cand, want) {
		t.Fatalf("utf16 candidate differs")
	}
	r, err := w.TaskDiff(e.TaskID)
	if err != nil || r.Operations[0].Replace.Hits != 1 || r.Operations[0].Replace.Nodes[0].NewValue == nil || *r.Operations[0].Replace.Nodes[0].NewValue != "R text" {
		t.Fatalf("utf16 review: %+v %v", r, err)
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}
