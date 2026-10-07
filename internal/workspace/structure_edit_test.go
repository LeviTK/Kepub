package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/validation"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

// structureChapter1 is one line of XHTML with mixed content, ruby, a table, a
// footnote, a direction attribute and inline SVG, so byte-level fidelity of
// untouched syntax is observable in every candidate.
const structureChapter1 = `<?xml version="1.0" encoding="utf-8"?>` + "\n" +
	`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">` +
	`<head><title>One</title><link rel="stylesheet" type="text/css" href="style.css"/></head>` +
	`<body>` +
	`<h1 id="start">One</h1>` +
	`<p id="mixed">Alpha <em>em</em> &amp; beta <a href="chapter2.xhtml#start2">link</a>.</p>` +
	`<p id="dir" dir="rtl">RTL text</p>` +
	`<p id="unreferenced">Plain.</p>` +
	`<p id="last">Last.</p>` +
	`<div id="ruby"><ruby>漢<rp>(</rp><rt>kan</rt><rp>)</rp></ruby></div>` +
	`<table id="tbl"><tr><td>cell</td></tr></table>` +
	`<aside epub:type="footnote" id="fn1"><p>Note text.</p></aside>` +
	`<p id="foreign">Text <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg> tail.</p>` +
	`</body></html>` + "\n"

const structureChapter2 = `<?xml version="1.0" encoding="utf-8"?>` + "\n" +
	`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Two</title></head>` +
	`<body><h1 id="start2">Two</h1><p>Beta &amp; two.</p></body></html>` + "\n"

const structureOPF = `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:uuid:12345678-1234-1234-1234-123456789012</dc:identifier><dc:title id="title">Title</dc:title><dc:creator id="creator">Writer</dc:creator><dc:language>en</dc:language><meta property="dcterms:modified">2026-10-04T00:00:00Z</meta></metadata><manifest><item id="c1" href="chapter1.xhtml" media-type="application/xhtml+xml" properties="svg"/><item id="c2" href="chapter2.xhtml" media-type="application/xhtml+xml"/><item id="css" href="style.css" media-type="text/css"/><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>` + "\r\n"

// structureWorkspace is a conformance-positive EPUB3 whose first chapter carries
// the fidelity fixture. extra files are imported too, so a test can build a
// reference-coverage-blocked variant without touching the shared fixture.
func structureWorkspace(t *testing.T, extra map[string]string) (*Workspace, string, string) {
	t.Helper()
	dir := t.TempDir()
	pub := filepath.Join(dir, "pub")
	files := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"EPUB/package.opf":       structureOPF,
		"EPUB/chapter1.xhtml":    structureChapter1,
		"EPUB/chapter2.xhtml":    structureChapter2,
		"EPUB/style.css":         "p { color: #123456; }\r\n",
		"EPUB/nav.xhtml":         `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="chapter1.xhtml#start">One</a></li><li><a href="chapter2.xhtml#start2">Two</a></li></ol></nav></body></html>`,
	}
	for name, b := range extra {
		files[name] = b
	}
	for name, b := range files {
		put(t, filepath.Join(pub, filepath.FromSlash(name)), []byte(b))
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
	w, err := Create(filepath.Join(dir, "workspace"), source, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return w, filepath.Join(dir, "workspace"), source
}

// binding is the frozen accepted revision of one XHTML resource.
type binding struct {
	bookPath string
	revision string
	sha      string
	doc      *publication.StructureDocument
}

func structureBinding(t *testing.T, w *Workspace, bp string) binding {
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
	media := ""
	for _, item := range p.Manifest {
		if item.Path == bookpath.BookPath(bp) {
			media = item.MediaType
		}
	}
	if media != "application/xhtml+xml" {
		t.Fatalf("%s is not manifest XHTML", bp)
	}
	b, err := a.Read(bookpath.BookPath(bp), publication.XMLLimit)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := publication.ParseStructureDocument(b, bookpath.BookPath(bp), xmltext.Profile{Version: p.Version, MediaType: media})
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(b)
	return binding{bookPath: bp, revision: r.ID, sha: hex.EncodeToString(h[:]), doc: doc}
}

func (b binding) locator(t *testing.T, local string, n int) string {
	t.Helper()
	seen := 0
	for _, e := range b.doc.Doc.Elements {
		if e.Name.Local != local {
			continue
		}
		if seen == n {
			return e.Location
		}
		seen++
	}
	t.Fatalf("no %s[%d] in %s", local, n, b.bookPath)
	return ""
}

func (b binding) locatorID(t *testing.T, id string) string {
	t.Helper()
	for _, e := range b.doc.Doc.Elements {
		for _, a := range e.Attributes {
			if a.Name.Space == "" && a.Name.Local == "id" && a.Value == id {
				return e.Location
			}
		}
	}
	t.Fatalf("no id %q in %s", id, b.bookPath)
	return ""
}

func strPtr(s string) *string { return &s }

func (b binding) attrSet(locator, name string, old *string, value string) Operation {
	return Operation{"xhtml.attribute.set", 1, publication.AttributeSet{BookPath: bookpath.BookPath(b.bookPath), RevisionID: b.revision, ResourceSHA256: b.sha, LocatorVersion: 1, Locator: locator, Name: name, ExpectedOldValue: old, Value: value}}
}

func (b binding) attrSetNS(locator, namespace, name string, old *string, value string) Operation {
	op := b.attrSet(locator, name, old, value)
	param := op.Params.(publication.AttributeSet)
	param.Namespace = namespace
	op.Params = param
	return op
}

func (b binding) attrRemove(locator, name, old string) Operation {
	return Operation{"xhtml.attribute.remove", 1, publication.AttributeRemove{BookPath: bookpath.BookPath(b.bookPath), RevisionID: b.revision, ResourceSHA256: b.sha, LocatorVersion: 1, Locator: locator, Name: name, ExpectedOldValue: old}}
}

func (b binding) elemDelete(locator string) Operation {
	return Operation{"xhtml.element.delete", 1, publication.ElementDelete{BookPath: bookpath.BookPath(b.bookPath), RevisionID: b.revision, ResourceSHA256: b.sha, LocatorVersion: 1, Locator: locator}}
}

func (b binding) elemInsert(locator, position, fragment string) Operation {
	return Operation{"xhtml.element.insert", 1, publication.ElementInsert{BookPath: bookpath.BookPath(b.bookPath), RevisionID: b.revision, ResourceSHA256: b.sha, LocatorVersion: 1, Locator: locator, Position: position, Fragment: fragment}}
}

func (b binding) elemReplace(locator, fragment string) Operation {
	return Operation{"xhtml.element.replace", 1, publication.ElementReplace{BookPath: bookpath.BookPath(b.bookPath), RevisionID: b.revision, ResourceSHA256: b.sha, LocatorVersion: 1, Locator: locator, Fragment: fragment}}
}

func (b binding) elemMove(locator, anchor, position string) Operation {
	return Operation{"xhtml.element.move", 1, publication.ElementMove{BookPath: bookpath.BookPath(b.bookPath), RevisionID: b.revision, ResourceSHA256: b.sha, LocatorVersion: 1, Locator: locator, Anchor: anchor, Position: position}}
}

func structurePlan(t *testing.T, w *Workspace, ops []Operation) Plan {
	t.Helper()
	p, err := w.Plan(editJSON(t, Request{4, ops}))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// planRefused requires a plan to be refused. A plain error carries the same
// INVALID_OPERATIONS code the CLI reports for rejected edit arguments.
func planRefused(t *testing.T, w *Workspace, ops []Operation, code string) {
	t.Helper()
	_, err := w.Plan(editJSON(t, Request{4, ops}))
	if err == nil {
		t.Fatalf("plan accepted: %+v", ops)
	}
	var f *fault.Error
	if errors.As(err, &f) {
		if f.Code != code {
			t.Fatalf("expected %s, got %v", code, err)
		}
		return
	}
	if code != "INVALID_OPERATIONS" {
		t.Fatalf("expected %s, got %v", code, err)
	}
}

func readResource(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestStructureAttributeSetRemoveAndFidelity edits only start tags and requires
// every other byte, including mixed content, ruby, a table, a footnote, the
// direction attribute and inline foreign markup, to survive exactly.
func TestStructureAttributeSetRemoveAndFidelity(t *testing.T) {
	requireChecker(t)
	w, dir, source := structureWorkspace(t, nil)
	defer w.Close()
	original := readResource(t, source)
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	base := readResource(t, filepath.Join(dir, revision, "EPUB/chapter1.xhtml"))
	p := structurePlan(t, w, []Operation{
		ch1.attrSet(ch1.locatorID(t, "mixed"), "dir", nil, "ltr"),
		ch1.attrSet(ch1.locatorID(t, "unreferenced"), "class", nil, "note"),
		ch1.attrRemove(ch1.locatorID(t, "dir"), "dir", "rtl"),
	})
	if !reflect.DeepEqual(p.WriteSet, []string{"EPUB/chapter1.xhtml"}) {
		t.Fatalf("write set %v", p.WriteSet)
	}
	e := applyPlan(t, w, p)
	cand := readResource(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"))
	want := bytes.Replace(base, []byte(`<p id="mixed">`), []byte(`<p id="mixed" dir="ltr">`), 1)
	want = bytes.Replace(want, []byte(`<p id="unreferenced">`), []byte(`<p id="unreferenced" class="note">`), 1)
	want = bytes.Replace(want, []byte(` dir="rtl"`), nil, 1)
	if !bytes.Equal(cand, want) {
		t.Fatalf("candidate bytes:\n got %q\nwant %q", cand, want)
	}
	for _, keep := range []string{
		`Alpha <em>em</em> &amp; beta <a href="chapter2.xhtml#start2">link</a>.`,
		`<ruby>漢<rp>(</rp><rt>kan</rt><rp>)</rp></ruby>`,
		`<table id="tbl"><tr><td>cell</td></tr></table>`,
		`<aside epub:type="footnote" id="fn1"><p>Note text.</p></aside>`,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`,
		`<?xml version="1.0" encoding="utf-8"?>`,
	} {
		if !bytes.Contains(cand, []byte(keep)) {
			t.Fatalf("candidate lost %q", keep)
		}
	}
	assertBytes(t, filepath.Join(dir, revision, "EPUB/chapter1.xhtml"), base)
	assertBytes(t, filepath.Join(dir, "original/book.epub"), original)
	d, err := w.Accept(context.Background(), e.TaskID, validation.Options{})
	if err != nil || d.Status != "accepted" || d.Validation == nil || d.Validation.Status != "pass" {
		t.Fatalf("accept: %+v %v", d, err)
	}
	assertBytes(t, filepath.Join(dir, revisionPath(d.RevisionID), "EPUB/chapter1.xhtml"), cand)
}

// TestStructureAttributeRefusals covers wrong old values, existing attributes,
// denied attributes and unknown locators before any candidate exists.
func TestStructureAttributeRefusals(t *testing.T) {
	w, _, _ := structureWorkspace(t, nil)
	defer w.Close()
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	cases := []struct {
		name string
		ops  []Operation
		code string
	}{
		{"old-value-mismatch", []Operation{ch1.attrSet(ch1.locatorID(t, "dir"), "dir", strPtr("ltr"), "ltr")}, "INVALID_OPERATIONS"},
		{"existing-without-old", []Operation{ch1.attrSet(ch1.locatorID(t, "mixed"), "id", nil, "other")}, "INVALID_OPERATIONS"},
		{"style-denied", []Operation{ch1.attrSet(ch1.locatorID(t, "mixed"), "style", nil, "color: red")}, "INVALID_OPERATIONS"},
		{"handler-denied", []Operation{ch1.attrSet(ch1.locatorID(t, "mixed"), "onclick", nil, "x()")}, "INVALID_OPERATIONS"},
		{"xml-base-denied", []Operation{ch1.attrSetNS(ch1.locatorID(t, "mixed"), publication.XMLNamespace, "base", nil, "x")}, "INVALID_OPERATIONS"},
		{"unknown-locator", []Operation{ch1.attrSet("/html[1]/body[1]/p[9]", "dir", nil, "ltr")}, "INVALID_OPERATIONS"},
		{"foreign-element", []Operation{ch1.attrSet(ch1.locator(t, "rect", 0), "width", strPtr("10"), "11")}, "INVALID_OPERATIONS"},
		{"remove-old-mismatch", []Operation{ch1.attrRemove(ch1.locatorID(t, "dir"), "dir", "ltr")}, "INVALID_OPERATIONS"},
		{"remove-absent", []Operation{ch1.attrRemove(ch1.locatorID(t, "mixed"), "dir", "ltr")}, "INVALID_OPERATIONS"},
		{"xml-id-not-name", []Operation{ch1.attrSet(ch1.locatorID(t, "mixed"), "id", nil, "1bad")}, "INVALID_OPERATIONS"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			planRefused(t, w, c.ops, c.code)
		})
	}
	if exists(w.root, candidate) || exists(w.root, "tasks/active") {
		t.Fatal("refused structure plan created task state")
	}
}

// TestStructureElementInsertMoveAndFidelity combines insertions at different
// points with a move and requires the untouched syntax to survive byte for byte.
func TestStructureElementInsertMoveAndFidelity(t *testing.T) {
	requireChecker(t)
	w, dir, _ := structureWorkspace(t, nil)
	defer w.Close()
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	base := readResource(t, filepath.Join(dir, revision, "EPUB/chapter1.xhtml"))
	ins := `<p id="ins">Inserted <em>em</em> &amp; more</p>`
	lead := `<p class="lead">Lead</p>`
	last := `<p id="last">Last.</p>`
	p := structurePlan(t, w, []Operation{
		ch1.elemInsert(ch1.locatorID(t, "mixed"), "before", ins),
		ch1.elemMove(ch1.locatorID(t, "last"), ch1.locatorID(t, "dir"), "after"),
		ch1.elemInsert(ch1.locatorID(t, "ruby"), "first-child", lead),
	})
	e := applyPlan(t, w, p)
	cand := readResource(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"))
	want := bytes.Replace(base, []byte(`<p id="mixed">`), []byte(ins+`<p id="mixed">`), 1)
	want = bytes.Replace(want, []byte(last), nil, 1)
	want = bytes.Replace(want, []byte(`<p id="dir" dir="rtl">RTL text</p>`), []byte(`<p id="dir" dir="rtl">RTL text</p>`+last), 1)
	want = bytes.Replace(want, []byte(`<div id="ruby">`), []byte(`<div id="ruby">`+lead), 1)
	if !bytes.Equal(cand, want) {
		t.Fatalf("candidate bytes:\n got %q\nwant %q", cand, want)
	}
	for _, keep := range []string{
		`<ruby>漢<rp>(</rp><rt>kan</rt><rp>)</rp></ruby>`,
		`<table id="tbl"><tr><td>cell</td></tr></table>`,
		`<aside epub:type="footnote" id="fn1"><p>Note text.</p></aside>`,
		`<p id="foreign">Text <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg> tail.</p>`,
	} {
		if !bytes.Contains(cand, []byte(keep)) {
			t.Fatalf("candidate lost %q", keep)
		}
	}
	d, err := w.Accept(context.Background(), e.TaskID, validation.Options{})
	if err != nil || d.Status != "accepted" || d.Validation == nil || d.Validation.Status != "pass" {
		t.Fatalf("accept: %+v %v", d, err)
	}
}

// TestStructureElementReplaceDeleteAndFidelity replaces and deletes elements
// while proving that surrounding mixed, ruby, footnote and foreign bytes are
// untouched.
func TestStructureElementReplaceDeleteAndFidelity(t *testing.T) {
	requireChecker(t)
	w, dir, _ := structureWorkspace(t, nil)
	defer w.Close()
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	base := readResource(t, filepath.Join(dir, revision, "EPUB/chapter1.xhtml"))
	block := `<blockquote id="repl"><p>Replaced &amp; kept</p></blockquote>`
	p := structurePlan(t, w, []Operation{
		ch1.elemReplace(ch1.locatorID(t, "unreferenced"), block),
		ch1.elemDelete(ch1.locatorID(t, "fn1")),
	})
	e := applyPlan(t, w, p)
	cand := readResource(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"))
	want := bytes.Replace(base, []byte(`<p id="unreferenced">Plain.</p>`), []byte(block), 1)
	want = bytes.Replace(want, []byte(`<aside epub:type="footnote" id="fn1"><p>Note text.</p></aside>`), nil, 1)
	if !bytes.Equal(cand, want) {
		t.Fatalf("candidate bytes:\n got %q\nwant %q", cand, want)
	}
	for _, keep := range []string{
		`Alpha <em>em</em> &amp; beta <a href="chapter2.xhtml#start2">link</a>.`,
		`<ruby>漢<rp>(</rp><rt>kan</rt><rp>)</rp></ruby>`,
		`<p id="dir" dir="rtl">RTL text</p>`,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">`,
	} {
		if !bytes.Contains(cand, []byte(keep)) {
			t.Fatalf("candidate lost %q", keep)
		}
	}
	if bytes.Contains(cand, []byte(`id="fn1"`)) || bytes.Contains(cand, []byte(`id="unreferenced"`)) {
		t.Fatal("removed identity survived the candidate")
	}
	d, err := w.Accept(context.Background(), e.TaskID, validation.Options{})
	if err != nil || d.Status != "accepted" || d.Validation == nil || d.Validation.Status != "pass" {
		t.Fatalf("accept: %+v %v", d, err)
	}
}

// TestStructureDuplicateOverlapStaleAndTargetRefusals covers duplicate and
// aliased targets, points inside removed ranges, an insertion exactly at a
// removed target's start, stale bindings and denied skeleton targets.
func TestStructureDuplicateOverlapStaleAndTargetRefusals(t *testing.T) {
	w, dir, _ := structureWorkspace(t, nil)
	defer w.Close()
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	set := ch1.attrSet(ch1.locatorID(t, "mixed"), "dir", nil, "ltr")
	planRefused(t, w, []Operation{set, set}, "INVALID_OPERATIONS")
	del := ch1.elemDelete(ch1.locatorID(t, "unreferenced"))
	planRefused(t, w, []Operation{del, del}, "INVALID_OPERATIONS")
	planRefused(t, w, []Operation{ch1.elemDelete(ch1.locatorID(t, "ruby")), ch1.attrSet(ch1.locator(t, "rt", 0), "lang", nil, "ja")}, "INVALID_OPERATIONS")
	planRefused(t, w, []Operation{ch1.elemDelete(ch1.locatorID(t, "ruby")), ch1.elemInsert(ch1.locatorID(t, "ruby"), "first-child", "<p>x</p>")}, "INVALID_OPERATIONS")
	planRefused(t, w, []Operation{ch1.elemDelete(ch1.locatorID(t, "ruby")), ch1.elemMove(ch1.locatorID(t, "ruby"), ch1.locatorID(t, "last"), "after")}, "INVALID_OPERATIONS")
	planRefused(t, w, []Operation{ch1.elemMove(ch1.locatorID(t, "ruby"), ch1.locator(t, "rt", 0), "before")}, "INVALID_OPERATIONS")
	planRefused(t, w, []Operation{ch1.elemMove(ch1.locatorID(t, "last"), ch1.locatorID(t, "last"), "before")}, "INVALID_OPERATIONS")
	planRefused(t, w, []Operation{ch1.elemDelete(ch1.locator(t, "body", 0))}, "INVALID_OPERATIONS")
	planRefused(t, w, []Operation{ch1.elemDelete(ch1.locator(t, "html", 0))}, "INVALID_OPERATIONS")
	planRefused(t, w, []Operation{ch1.elemInsert(ch1.locator(t, "head", 0), "last-child", "<title>x</title>")}, "INVALID_OPERATIONS")
	planRefused(t, w, []Operation{ch1.elemInsert(ch1.locator(t, "html", 0), "first-child", "<p>x</p>")}, "INVALID_OPERATIONS")
	planRefused(t, w, []Operation{ch1.elemInsert(ch1.locatorID(t, "mixed"), "after", "<script>x()</script>")}, "INVALID_OPERATIONS")
	planRefused(t, w, []Operation{ch1.elemInsert(ch1.locatorID(t, "mixed"), "after", `<svg xmlns="http://www.w3.org/2000/svg"/>`)}, "INVALID_OPERATIONS")
	planRefused(t, w, []Operation{ch1.elemInsert(ch1.locatorID(t, "mixed"), "after", `<p style="color: red">x</p>`)}, "INVALID_OPERATIONS")
	planRefused(t, w, []Operation{ch1.elemInsert(ch1.locatorID(t, "mixed"), "after", "<p>unclosed")}, "INVALID_OPERATIONS")
	planRefused(t, w, []Operation{ch1.elemInsert(ch1.locatorID(t, "mixed"), "after", "<!--c--><p>x</p>")}, "INVALID_OPERATIONS")
	planRefused(t, w, []Operation{ch1.attrSet(ch1.locatorID(t, "mixed"), "id", nil, "start")}, "INVALID_OPERATIONS")
	planRefused(t, w, []Operation{
		ch1.attrSet(ch1.locatorID(t, "mixed"), "id", nil, "dup"),
		ch1.attrSet(ch1.locatorID(t, "unreferenced"), "id", nil, "dup"),
	}, "INVALID_OPERATIONS")
	stale := ch1.attrSet(ch1.locatorID(t, "mixed"), "dir", nil, "ltr").Params.(publication.AttributeSet)
	stale.RevisionID = strings.Repeat("ab", 16)
	if _, err := w.Plan(editJSON(t, Request{4, []Operation{{"xhtml.attribute.set", 1, stale}}})); !errors.Is(err, ErrStalePlan) {
		t.Fatalf("stale revision: %v", err)
	}
	drift := ch1.attrSet(ch1.locatorID(t, "mixed"), "dir", nil, "ltr").Params.(publication.AttributeSet)
	drift.ResourceSHA256 = strings.Repeat("0", 64)
	var f *fault.Error
	if _, err := w.Plan(editJSON(t, Request{4, []Operation{{"xhtml.attribute.set", 1, drift}}})); !errors.As(err, &f) || f.Code != "INPUT_DRIFT" {
		t.Fatalf("drifted resource hash: %v", err)
	}
	// An insertion at the exact boundary of a deleted target shares one offset,
	// so it is refused instead of being ordered silently.
	planRefused(t, w, []Operation{
		ch1.elemInsert(ch1.locatorID(t, "unreferenced"), "before", `<p id="kept">Kept.</p>`),
		ch1.elemDelete(ch1.locatorID(t, "unreferenced")),
	}, "INVALID_OPERATIONS")
	// Insertions outside every replaced range stay legal and exact.
	p := structurePlan(t, w, []Operation{
		ch1.elemInsert(ch1.locatorID(t, "mixed"), "before", `<p id="kept">Kept.</p>`),
		ch1.elemDelete(ch1.locatorID(t, "last")),
	})
	e := applyPlan(t, w, p)
	cand := readResource(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"))
	if !bytes.Contains(cand, []byte(`<p id="kept">Kept.</p>`)) || bytes.Contains(cand, []byte(`id="last"`)) {
		t.Fatalf("adjacent insert/delete candidate: %q", cand)
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}

// TestStructureReferenceGate proves that removing a referenced identity, or an
// identity whose absence cannot be proven, is refused before any candidate
// exists, and that new link values must resolve.
func TestStructureReferenceGate(t *testing.T) {
	blocker := map[string]string{
		"EPUB/scripted.xhtml": `<?xml version="1.0" encoding="utf-8"?>` + "\n" +
			`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Scripted</title></head><body><p>Scripted.</p><script>var x = 1;</script></body></html>` + "\n",
		"EPUB/package.opf": strings.Replace(structureOPF,
			`<item id="c2" href="chapter2.xhtml" media-type="application/xhtml+xml"/>`,
			`<item id="c2" href="chapter2.xhtml" media-type="application/xhtml+xml"/><item id="sc" href="scripted.xhtml" media-type="application/xhtml+xml"/>`, 1),
	}
	for _, tc := range []struct {
		name  string
		extra map[string]string
		code  string
		build func(t *testing.T, w *Workspace) []Operation
	}{
		{"referenced-id-delete", nil, "REFERENCE_CONFLICT", func(t *testing.T, w *Workspace) []Operation {
			ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
			return []Operation{ch1.elemDelete(ch1.locatorID(t, "start"))}
		}},
		{"referenced-id-rename", nil, "REFERENCE_CONFLICT", func(t *testing.T, w *Workspace) []Operation {
			ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
			return []Operation{ch1.attrSet(ch1.locatorID(t, "start"), "id", strPtr("start"), "begin")}
		}},
		{"referenced-id-replace", nil, "REFERENCE_CONFLICT", func(t *testing.T, w *Workspace) []Operation {
			ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
			return []Operation{ch1.elemReplace(ch1.locatorID(t, "start"), `<h1 id="begin">One</h1>`)}
		}},
		{"coverage-blocker", blocker, "REFERENCE_COVERAGE_INCOMPLETE", func(t *testing.T, w *Workspace) []Operation {
			ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
			return []Operation{ch1.elemDelete(ch1.locatorID(t, "unreferenced"))}
		}},
		{"missing-link-target", nil, "INVALID_OPERATIONS", func(t *testing.T, w *Workspace) []Operation {
			ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
			return []Operation{ch1.attrSet(ch1.locatorID(t, "mixed"), "class", nil, "x"), ch1.elemInsert(ch1.locatorID(t, "mixed"), "after", `<p><a href="missing.xhtml">x</a></p>`)}
		}},
		{"script-link", nil, "INVALID_OPERATIONS", func(t *testing.T, w *Workspace) []Operation {
			ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
			return []Operation{ch1.elemInsert(ch1.locatorID(t, "mixed"), "after", `<p><a href="javascript:alert(1)">x</a></p>`)}
		}},
		{"missing-fragment", nil, "INVALID_OPERATIONS", func(t *testing.T, w *Workspace) []Operation {
			ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
			return []Operation{ch1.elemInsert(ch1.locatorID(t, "mixed"), "after", `<p><a href="chapter2.xhtml#absent">x</a></p>`)}
		}},
		{"fragment-in-edited-resource", nil, "INVALID_OPERATIONS", func(t *testing.T, w *Workspace) []Operation {
			ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
			return []Operation{
				ch1.elemDelete(ch1.locatorID(t, "unreferenced")),
				ch1.elemInsert(ch1.locatorID(t, "mixed"), "after", `<p><a href="chapter1.xhtml#unreferenced">x</a></p>`),
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, _, _ := structureWorkspace(t, tc.extra)
			defer w.Close()
			planRefused(t, w, tc.build(t, w), tc.code)
			if exists(w.root, candidate) {
				t.Fatal("refused reference write created a candidate")
			}
		})
	}
	// External links and links to a present, uniquely identified fragment are
	// accepted, and an identity added earlier in the same transaction can be
	// referenced by a later insertion at a different point.
	w, dir, _ := structureWorkspace(t, nil)
	defer w.Close()
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	note := `<p id="note"><a href="https://example.invalid/x">ext</a> <a href="chapter2.xhtml#start2">two</a></p>`
	self := `<p><a href="chapter1.xhtml#note">self</a></p>`
	// Two insertions at one point are a duplicate target, not an ordering rule.
	planRefused(t, w, []Operation{
		ch1.elemInsert(ch1.locatorID(t, "mixed"), "after", note),
		ch1.elemInsert(ch1.locatorID(t, "mixed"), "after", self),
	}, "INVALID_OPERATIONS")
	p := structurePlan(t, w, []Operation{
		ch1.elemInsert(ch1.locatorID(t, "mixed"), "after", note),
		ch1.elemInsert(ch1.locatorID(t, "unreferenced"), "after", self),
	})
	e := applyPlan(t, w, p)
	cand := readResource(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"))
	if !bytes.Contains(cand, []byte(note)) || !bytes.Contains(cand, []byte(self)) {
		t.Fatalf("two-point insertions: %q", cand)
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}

// TestStructureMultiResourceFailureAndInterruptionRecovery proves structural
// edits use the same multi-resource transaction: a failure at a later resource
// rolls the whole candidate back, and an interruption is recovered to the
// frozen baseline.
func TestStructureMultiResourceFailureAndInterruptionRecovery(t *testing.T) {
	w, dir, _ := structureWorkspace(t, nil)
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	ch2 := structureBinding(t, w, "EPUB/chapter2.xhtml")
	p := structurePlan(t, w, []Operation{
		metadataOp("title", "title", "Title", "Structure Title"),
		ch1.attrSet(ch1.locatorID(t, "mixed"), "class", nil, "one"),
		ch2.attrSet(ch2.locatorID(t, "start2"), "class", nil, "two"),
	})
	if !reflect.DeepEqual(p.WriteSet, []string{"EPUB/chapter1.xhtml", "EPUB/chapter2.xhtml", "EPUB/package.opf"}) {
		t.Fatalf("write set %v", p.WriteSet)
	}
	e, outputs := prepareExecution(t, w, p)
	// The second planned resource becomes a hard-linked file, so the write
	// fails after the first resource was already replaced.
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
	if err == nil || failed.Status != "failed" || failed.ReviewRequired {
		t.Fatalf("second-resource failure: %+v %v", failed, err)
	}
	if treeAt(t, filepath.Join(dir, candidate)).SHA256 != p.InputTreeSHA256 {
		t.Fatal("failed structural write left a partial candidate")
	}
	assertBytes(t, outside, []byte("outside"))
	assertEmpty(t, filepath.Join(dir, "staging"))
	assertEmpty(t, filepath.Join(dir, "journal"))
	if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{}); !errors.Is(err, ErrTaskConflict) {
		t.Fatalf("failed structural task accepted: %v", err)
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
	// Interruption after the first of several planned writes is recovered to
	// the baseline and can never be accepted.
	p = structurePlan(t, w, []Operation{
		metadataOp("title", "title", "Title", "Structure Title"),
		ch1.attrSet(ch1.locatorID(t, "mixed"), "class", nil, "one"),
		ch2.attrSet(ch2.locatorID(t, "start2"), "class", nil, "two"),
	})
	e, outputs = prepareExecution(t, w, p)
	first := p.WriteSet[0]
	put(t, filepath.Join(dir, candidate, first), outputs[first])
	w.Close()
	w, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	got, err := w.Execution()
	if err != nil || got.Status != "failed" || got.Failure != "interrupted apply" || got.ReviewRequired {
		t.Fatalf("interrupted structural apply: %+v %v", got, err)
	}
	if treeAt(t, filepath.Join(dir, candidate)).SHA256 != p.InputTreeSHA256 {
		t.Fatal("partial structural write survived recovery")
	}
	if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{}); !errors.Is(err, ErrTaskConflict) {
		t.Fatalf("interrupted structural task accepted: %v", err)
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}

// TestStructureReviewObservationsAndDrift requires task diff to report planned
// and actual attribute/element state from real candidate bytes, and to report
// drift instead of the planned value.
func TestStructureReviewObservationsAndDrift(t *testing.T) {
	w, dir, _ := structureWorkspace(t, nil)
	defer w.Close()
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	p := structurePlan(t, w, []Operation{
		ch1.attrSet(ch1.locatorID(t, "mixed"), "dir", nil, "ltr"),
		ch1.attrRemove(ch1.locatorID(t, "dir"), "dir", "rtl"),
		ch1.elemInsert(ch1.locatorID(t, "last"), "after", `<p id="ins">Inserted.</p>`),
		ch1.elemDelete(ch1.locatorID(t, "unreferenced")),
	})
	e := applyPlan(t, w, p)
	r, err := w.TaskDiff(e.TaskID)
	if err != nil || !r.MatchesExecution || len(r.Operations) != 4 {
		t.Fatalf("review: %+v %v", r, err)
	}
	if r.Operations[0].Attribute == nil || r.Operations[0].Attribute.NewValue == nil || *r.Operations[0].Attribute.NewValue != "ltr" {
		t.Fatalf("attribute observation: %+v", r.Operations[0])
	}
	if r.Operations[1].Attribute == nil || r.Operations[1].Attribute.NewValue != nil || r.Operations[1].Attribute.Unavailable != "" {
		t.Fatalf("removal observation: %+v", r.Operations[1])
	}
	if r.Operations[2].Element == nil || !strings.Contains(r.Operations[2].Element.Candidate, "present") {
		t.Fatalf("insert observation: %+v", r.Operations[2])
	}
	if r.Operations[3].Element == nil || !strings.Contains(r.Operations[3].Element.Candidate, "removed ids absent: unreferenced") {
		t.Fatalf("delete observation: %+v", r.Operations[3])
	}
	// Tampering with the candidate is reported as the actual value, never as
	// the planned value.
	candPath := filepath.Join(dir, candidate, "EPUB/chapter1.xhtml")
	cand := readResource(t, candPath)
	put(t, candPath, bytes.Replace(cand, []byte(`dir="ltr"`), []byte(`dir="ttb"`), 1))
	drifted, err := w.TaskDiff(e.TaskID)
	if err != nil || drifted.MatchesExecution || drifted.Operations[0].Attribute.NewValue == nil || *drifted.Operations[0].Attribute.NewValue != "ttb" {
		t.Fatalf("drifted review: %+v %v", drifted, err)
	}
	if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{}); !errors.Is(err, ErrCandidateDrift) {
		t.Fatalf("drifted structural candidate accepted: %v", err)
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
	// The file diff is authoritative for placement: an edited file is reported
	// as one changed resource with real before/after hashes.
	p = structurePlan(t, w, []Operation{ch1.attrSet(ch1.locatorID(t, "mixed"), "dir", nil, "ltr")})
	e = applyPlan(t, w, p)
	r, err = w.TaskDiff(e.TaskID)
	if err != nil || len(r.Diff.Changes) != 1 || r.Diff.Changes[0].Path != "EPUB/chapter1.xhtml" || r.Diff.Changes[0].Before.SHA256 == r.Diff.Changes[0].After.SHA256 {
		t.Fatalf("file diff: %+v %v", r.Diff, err)
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}

// TestStructureNoOpMixedAndStalePlan accepts a mixed no-op/changed transaction
// and then refuses the same plan against the new baseline.
func TestStructureNoOpMixedAndStalePlan(t *testing.T) {
	requireChecker(t)
	w, dir, _ := structureWorkspace(t, nil)
	defer w.Close()
	ch1 := structureBinding(t, w, "EPUB/chapter1.xhtml")
	changed := ch1.attrSet(ch1.locatorID(t, "mixed"), "dir", nil, "ltr")
	noop := Operation{"content.text.set", 1, publication.TextSet{BookPath: bookpath.BookPath(ch1.bookPath), RevisionID: ch1.revision, ResourceSHA256: ch1.sha, LocatorVersion: 1, Locator: ch1.locatorID(t, "unreferenced"), ExpectedOldValue: "Plain.", NewValue: "Plain."}}
	p := structurePlan(t, w, []Operation{changed, noop})
	e := applyPlan(t, w, p)
	cand := readResource(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"))
	want := bytes.Replace(readResource(t, filepath.Join(dir, revision, "EPUB/chapter1.xhtml")), []byte(`<p id="mixed">`), []byte(`<p id="mixed" dir="ltr">`), 1)
	if !bytes.Equal(cand, want) {
		t.Fatalf("mixed no-op transaction: %q", cand)
	}
	d, err := w.Accept(context.Background(), e.TaskID, validation.Options{})
	if err != nil || d.Status != "accepted" {
		t.Fatalf("accept: %+v %v", d, err)
	}
	if _, err := w.Apply(editJSON(t, p)); !errors.Is(err, ErrStalePlan) {
		t.Fatalf("stale structure plan: %v", err)
	}
	st, err := w.TaskStatus(e.TaskID)
	if err != nil || st.Status != "accepted" || st.Decision == nil || st.Decision.RevisionID != d.RevisionID {
		t.Fatalf("history: %+v %v", st, err)
	}
}
