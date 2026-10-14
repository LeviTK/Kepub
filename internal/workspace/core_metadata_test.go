package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/metadata"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/validation"
)

const coreOPFPath = "EPUB/package.opf"

func coreOp(t *testing.T, w *Workspace, field, id, old, new string) Operation {
	t.Helper()
	a, _, r, err := w.AcceptedSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := a.Read(bookpath.BookPath(coreOPFPath), 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	p := metadata.SetV2{BookPath: bookpath.BookPath(coreOPFPath), RevisionID: r.ID, ResourceSHA256: fmt.Sprintf("%x", sha256.Sum256(b)), Namespace: metadata.DC, LocalName: field, ID: id, ExpectedOldValue: old, NewValue: new}
	if field == "meta" {
		p.Namespace = "http://www.idpf.org/2007/opf"
		p.Property = "dcterms:modified"
	}
	return Operation{"metadata.set", 2, p}
}

// Read every fixture resource, independently of the derived WriteSet. Empty
// directories are included so rollback cannot silently lose inventory entries.
func coreFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	m := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			m[filepath.ToSlash(rel)+"/"] = nil
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		m[filepath.ToSlash(rel)] = b
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func coreAssertFiles(t *testing.T, root string, want map[string][]byte) {
	t.Helper()
	got := coreFiles(t, root)
	if len(got) != len(want) {
		t.Fatalf("inventory size: got %d want %d", len(got), len(want))
	}
	for path, b := range want {
		actual, ok := got[path]
		if !ok || !bytes.Equal(actual, b) {
			t.Errorf("full resource oracle mismatch: %s (got %d bytes, want %d)", path, len(actual), len(b))
		}
	}
}

func corePlan(t *testing.T, w *Workspace, ops ...Operation) Plan {
	t.Helper()
	p, err := w.Plan(editJSON(t, Request{8, ops}))
	if err != nil {
		t.Fatal(err)
	}
	if p.SchemaVersion != 8 {
		t.Fatal("wrong plan schema", p.SchemaVersion)
	}
	return p
}

func coreRefusal(t *testing.T, w *Workspace, dir string, request []byte, code string, exit int) {
	t.Helper()
	before := coreFiles(t, dir)
	_, err := w.Plan(request)
	var f *fault.Error
	// Workspace exposes argument errors directly; app owns their wire mapping
	// to INVALID_OPERATIONS/2. Drift also has a public sentinel representation.
	valid := err != nil
	if errors.As(err, &f) {
		valid = f.Code == code && f.Exit == exit
	} else if code == "INPUT_DRIFT" {
		valid = errors.Is(err, ErrStalePlan)
	} else {
		valid = valid && code == "INVALID_OPERATIONS" && exit == 2
	}
	if !valid {
		t.Fatalf("want %s/%d, got %v", code, exit, err)
	}
	coreAssertFiles(t, dir, before)
}

func TestCoreMetadataMixedFrozenTransaction(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprint(reverse), func(t *testing.T) {
			w, dir, source := multiWorkspace(t)
			defer w.Close()
			original, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			baseline := coreFiles(t, filepath.Join(dir, revision))
			want := coreFiles(t, filepath.Join(dir, revision))
			ops := []Operation{coreOp(t, w, "language", "", "en", "en-GB-oed"), metadataOp("title", "title", "Title", "Title & new"), contentOpN(t, w, "EPUB/chapter1.xhtml", 0, "Body & new"), coreOp(t, w, "meta", "", "2026-10-04T00:00:00Z", "2024-02-29T12:34:56Z")}
			if reverse {
				for i, j := 0, len(ops)-1; i < j; i, j = i+1, j-1 {
					ops[i], ops[j] = ops[j], ops[i]
				}
			}
			want[coreOPFPath] = bytes.ReplaceAll(want[coreOPFPath], []byte("<dc:language>en</dc:language>"), []byte("<dc:language>en-GB-oed</dc:language>"))
			want[coreOPFPath] = bytes.ReplaceAll(want[coreOPFPath], []byte(">Title</dc:title>"), []byte(">Title &amp; new</dc:title>"))
			want[coreOPFPath] = bytes.ReplaceAll(want[coreOPFPath], []byte("2026-10-04T00:00:00Z"), []byte("2024-02-29T12:34:56Z"))
			want["EPUB/chapter1.xhtml"] = bytes.ReplaceAll(want["EPUB/chapter1.xhtml"], []byte("<p>Alpha &amp; one.</p>"), []byte("<p>Body &amp; new</p>"))
			p := corePlan(t, w, ops...)
			coreAssertFiles(t, filepath.Join(dir, revision), baseline)
			e := applyPlan(t, w, p)
			if e.Version != 8 || e.Status != "review_required" || !e.ReviewRequired || e.Conformance != "not_run" {
				t.Fatalf("execution: %+v", e)
			}
			coreAssertFiles(t, filepath.Join(dir, candidate), want)
			coreAssertFiles(t, filepath.Join(dir, revision), baseline)
			assertBytes(t, source, original)
			r, err := w.TaskDiff(e.TaskID)
			if err != nil || !r.MatchesExecution || len(r.Operations) != len(ops) {
				t.Fatalf("review: %+v %v", r, err)
			}
			if _, err := w.Reject(e.TaskID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCoreMetadataEveryBindingAndAlias(t *testing.T) {
	w, dir, _ := multiWorkspace(t)
	defer w.Close()
	for _, kind := range []string{"revision", "hash"} {
		for _, reverse := range []bool{false, true} {
			t.Run(kind+fmt.Sprint(reverse), func(t *testing.T) {
				good := coreOp(t, w, "title", "title", "Title", "Title")
				bad := coreOp(t, w, "creator", "creator", "Writer", "Writer")
				p := bad.Params.(metadata.SetV2)
				if kind == "revision" {
					p.RevisionID = randomID()
				} else {
					p.ResourceSHA256 = strings.Repeat("0", 64)
				}
				bad.Params = p
				ops := []Operation{good, bad}
				if reverse {
					ops[0], ops[1] = ops[1], ops[0]
				}
				coreRefusal(t, w, dir, editJSON(t, Request{8, ops}), "INPUT_DRIFT", 4)
			})
		}
	}
	for _, v1 := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("alias-v1-%v-reverse-%v", v1, reverse), func(t *testing.T) {
				a := coreOp(t, w, "title", "title", "Title", "Title")
				b := coreOp(t, w, "title", "", "Title", "Title")
				if v1 {
					b = metadataOp("title", "", "Title", "Title")
				}
				ops := []Operation{a, b}
				if reverse {
					ops[0], ops[1] = ops[1], ops[0]
				}
				coreRefusal(t, w, dir, editJSON(t, Request{8, ops}), "INVALID_OPERATIONS", 2)
			})
		}
	}
	// Legal repeated-resource no-ops must still produce a reviewable task.
	p := corePlan(t, w, coreOp(t, w, "title", "title", "Title", "Title"), coreOp(t, w, "creator", "creator", "Writer", "Writer"))
	baseline := coreFiles(t, filepath.Join(dir, revision))
	e := applyPlan(t, w, p)
	if e.Diff.Changed || !e.ReviewRequired || len(p.WriteSet) != 0 {
		t.Fatal("no-op changed publication or skipped review", e)
	}
	coreAssertFiles(t, filepath.Join(dir, candidate), baseline)
	r, err := w.TaskDiff(e.TaskID)
	if err != nil || !r.MatchesExecution || len(r.Operations) != 2 {
		t.Fatal(r, err)
	}
	for _, op := range r.Operations {
		if op.NewValue == nil || *op.NewValue != op.OldValue {
			t.Fatal("no-op actual value", op)
		}
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}

func TestCoreMetadataStrictSchema(t *testing.T) {
	w, dir, _ := multiWorkspace(t)
	defer w.Close()
	b := string(editJSON(t, Request{8, []Operation{coreOp(t, w, "title", "title", "Title", "New")}}))
	for name, request := range map[string]string{
		"unknown":       strings.Replace(b, `"localName":"title"`, `"localName":"title","extra":1`, 1),
		"missing":       strings.Replace(b, `"expectedOldValue":"Title",`, "", 1),
		"null":          strings.Replace(b, `"expectedOldValue":"Title"`, `"expectedOldValue":null`, 1),
		"duplicate":     strings.Replace(b, `"localName":"title"`, `"localName":"title","localName":"title"`, 1),
		"old-schema":    strings.Replace(b, `"schemaVersion":8`, `"schemaVersion":3`, 1),
		"invalid-utf8":  strings.Replace(b, `"newValue":"New"`, "\"newValue\":\"\xff\"", 1),
		"surrogate":     strings.Replace(b, `"newValue":"New"`, `"newValue":"\ud800"`, 1),
		"low-surrogate": strings.Replace(b, `"newValue":"New"`, `"newValue":"\udc00"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if request == b {
				t.Fatal("fixture mutation missed")
			}
			coreRefusal(t, w, dir, []byte(request), "INVALID_OPERATIONS", 2)
		})
	}
	// Paired escapes and literal backslash spellings must not be confused with
	// malformed Unicode; both retain their independently expected XML bytes.
	for _, tc := range []struct{ json, value string }{{`\ud83d\ude00`, "😀"}, {`\\ud800`, `\ud800`}, {`\ufffd`, "�"}} {
		request := strings.Replace(b, `"newValue":"New"`, `"newValue":"`+tc.json+`"`, 1)
		p, err := w.Plan([]byte(request))
		if err != nil {
			t.Fatal("valid Unicode rejected", tc.json, err)
		}
		before := coreFiles(t, filepath.Join(dir, revision))
		e := applyPlan(t, w, p)
		before[coreOPFPath] = bytes.Replace(before[coreOPFPath], []byte(">Title</dc:title>"), []byte(">"+tc.value+"</dc:title>"), 1)
		coreAssertFiles(t, filepath.Join(dir, candidate), before)
		if _, err := w.Reject(e.TaskID); err != nil {
			t.Fatal(err)
		}
	}
	// The whole schema 8 request, including legacy operations, is lossless.
	mixed := string(editJSON(t, Request{8, []Operation{coreOp(t, w, "language", "", "en", "zz"), metadataOp("title", "title", "Title", "New")}}))
	coreRefusal(t, w, dir, []byte(strings.Replace(mixed, `"newValue":"New"`, `"newValue":"\ud800"`, 1)), "INVALID_OPERATIONS", 2)
	text := contentOpN(t, w, "EPUB/chapter1.xhtml", 0, "New")
	p := text.Params.(publication.TextSet)
	structural := Operation{"xhtml.element.delete", 1, publication.ElementDelete{BookPath: p.BookPath, RevisionID: p.RevisionID, ResourceSHA256: p.ResourceSHA256, LocatorVersion: p.LocatorVersion, Locator: p.Locator}}
	coreRefusal(t, w, dir, editJSON(t, Request{8, []Operation{coreOp(t, w, "title", "title", "Title", "New"), structural}}), "INVALID_OPERATIONS", 2)
	coreRefusal(t, w, dir, editJSON(t, Request{8, []Operation{text}}), "INVALID_OPERATIONS", 2)
}

func TestCoreMetadataDriftReviewRejectHistory(t *testing.T) {
	w, dir, _ := legalWorkspace(t, "3.0")
	baseline := coreFiles(t, filepath.Join(dir, revision))
	p := corePlan(t, w, coreOp(t, w, "title", "title", "Title", "Planned"))
	e := applyPlan(t, w, p)
	want := bytes.Replace(baseline[coreOPFPath], []byte(">Title</dc:title>"), []byte(">Actual drift</dc:title>"), 1)
	put(t, filepath.Join(dir, candidate, coreOPFPath), want)
	w.Close()
	w, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	r, err := w.TaskDiff(e.TaskID)
	if err != nil || r.MatchesExecution || len(r.Operations) != 1 || r.Operations[0].NewValue == nil || *r.Operations[0].NewValue != "Actual drift" || r.Operations[0].OldValue != "Title" {
		t.Fatalf("actual drift review: %+v %v", r, err)
	}
	if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{}); !errors.Is(err, ErrCandidateDrift) {
		t.Fatal("drift acceptance", err)
	}
	coreAssertFiles(t, filepath.Join(dir, revision), baseline)
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
	w.Close()
	w, err = Open(dir)
	if err != nil {
		t.Fatal("rejected history reopen", err)
	}
	defer w.Close()
	if _, err := w.TaskStatus(e.TaskID); err != nil {
		t.Fatal("rejected history", err)
	}
	coreAssertFiles(t, filepath.Join(dir, revision), baseline)
}

func TestCoreMetadataPartialExecutionRecovery(t *testing.T) {
	w, dir, _ := multiWorkspace(t)
	baseline := coreFiles(t, filepath.Join(dir, revision))
	p := corePlan(t, w, coreOp(t, w, "title", "title", "Title", "Changed"), contentOpN(t, w, "EPUB/chapter1.xhtml", 0, "Changed body"))
	e, _ := prepareExecution(t, w, p)
	// Inject only the first physical resource write, with an independent oracle.
	put(t, filepath.Join(dir, candidate, coreOPFPath), bytes.Replace(baseline[coreOPFPath], []byte(">Title</dc:title>"), []byte(">Changed</dc:title>"), 1))
	w.Close()
	w, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := w.Execution()
	if err != nil || got.Status != "failed" || got.Failure != "interrupted apply" {
		t.Fatalf("recovery: %+v %v", got, err)
	}
	coreAssertFiles(t, filepath.Join(dir, candidate), baseline)
	coreAssertFiles(t, filepath.Join(dir, revision), baseline)
	if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{}); !errors.Is(err, ErrTaskConflict) {
		t.Fatal("failed task accepted", err)
	}
	w.Close()
	w, err = Open(dir)
	if err != nil {
		t.Fatal("second reopen", err)
	}
	coreAssertFiles(t, filepath.Join(dir, candidate), baseline)
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
	w.Close()
	w, err = Open(dir)
	if err != nil {
		t.Fatal("failed rejected history reopen", err)
	}
	defer w.Close()
	if _, err := w.TaskStatus(e.TaskID); err != nil {
		t.Fatal(err)
	}
}

func TestCoreMetadataEncodingBytes(t *testing.T) {
	for _, enc := range []string{"utf8", "utf16le", "utf16be"} {
		t.Run(enc, func(t *testing.T) {
			src := strings.Replace(structureOPF, `<?xml version="1.0"?>`, `<?xml version="1.0" encoding="utf-8"?>`, 1)
			src = strings.Replace(src, "</metadata>", `<meta refines="#title" property="alternate-script" xml:lang="ja">書名</meta><!-- untouched 😀 -->`+"</metadata>", 1)
			w, dir, _ := structureWorkspace(t, map[string]string{coreOPFPath: encodeChapter(src, enc)})
			defer w.Close()
			want := coreFiles(t, filepath.Join(dir, revision))
			want[coreOPFPath] = []byte(encodeChapter(strings.Replace(src, ">Title</dc:title>", ">新 &amp; 😀</dc:title>", 1), enc))
			e := applyPlan(t, w, corePlan(t, w, coreOp(t, w, "title", "title", "Title", "新 & 😀")))
			coreAssertFiles(t, filepath.Join(dir, candidate), want)
			w.Close()
			var err error
			w, err = Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			r, err := w.TaskDiff(e.TaskID)
			if err != nil || !r.MatchesExecution || r.Operations[0].NewValue == nil || *r.Operations[0].NewValue != "新 & 😀" {
				t.Fatal(r, err)
			}
			if _, err := w.Reject(e.TaskID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCoreMetadataSubjectsAndDependencies(t *testing.T) {
	const old = "2026-10-04T00:00:00Z"
	const next = "2024-02-29T12:34:56Z"
	const modified = `<meta property="dcterms:modified">` + old + `</meta>`
	for _, value := range []string{old, "2020-01-01T00:00:00Z"} {
		t.Run("collection-"+value, func(t *testing.T) {
			src := strings.Replace(structureOPF, "</metadata>", `<meta name="cover" content="legacy"/><meta property="dcterms:modified" refines="#title">`+value+`</meta></metadata>`, 1)
			src = strings.Replace(src, "</package>", `<collection role="http://example.org/roles/test"><metadata><meta property="dcterms:modified">`+value+`</meta></metadata></collection></package>`, 1)
			w, dir, _ := structureWorkspace(t, map[string]string{coreOPFPath: src})
			defer w.Close()
			want := coreFiles(t, filepath.Join(dir, revision))
			want[coreOPFPath] = []byte(strings.Replace(src, modified, `<meta property="dcterms:modified">`+next+`</meta>`, 1))
			e := applyPlan(t, w, corePlan(t, w, coreOp(t, w, "meta", "", old, next)))
			coreAssertFiles(t, filepath.Join(dir, candidate), want)
			if _, err := w.Reject(e.TaskID); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("ncx", func(t *testing.T) {
		const uid = "urn:uuid:12345678-1234-1234-1234-123456789012"
		src := strings.Replace(structureOPF, "</metadata>", `<dc:identifier id="secondary">secondary</dc:identifier></metadata>`, 1)
		src = strings.Replace(src, "</manifest>", `<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/></manifest>`, 1)
		const ncx = `<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head><meta name="dtb:uid" content="` + uid + `"/></head></ncx>`
		w, dir, _ := structureWorkspace(t, map[string]string{coreOPFPath: src, "EPUB/toc.ncx": ncx})
		defer w.Close()
		for _, new := range []string{uid, "changed"} {
			coreRefusal(t, w, dir, editJSON(t, Request{8, []Operation{coreOp(t, w, "identifier", "uid", uid, new)}}), "UNSUPPORTED_INPUT", 3)
		}
		want := coreFiles(t, filepath.Join(dir, revision))
		want[coreOPFPath] = []byte(strings.Replace(src, ">secondary</dc:identifier>", ">changed</dc:identifier>", 1))
		e := applyPlan(t, w, corePlan(t, w, coreOp(t, w, "identifier", "secondary", "secondary", "changed")))
		coreAssertFiles(t, filepath.Join(dir, candidate), want)
		if _, err := w.Reject(e.TaskID); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("epub2", func(t *testing.T) {
		w, dir, _ := legalWorkspace(t, "2.0")
		defer w.Close()
		coreRefusal(t, w, dir, editJSON(t, Request{8, []Operation{coreOp(t, w, "title", "title", "Title", "Changed")}}), "UNSUPPORTED_INPUT", 3)
	})
}

// Every generated transaction is legal. Wrongly rejecting one is a failure,
// not a skipped fuzz iteration; the oracle never uses derived output/spans.
func FuzzCoreMetadataTransaction(f *testing.F) {
	for i := uint8(0); i < 4; i++ {
		f.Add(i, false)
		f.Add(i, true)
	}
	f.Fuzz(func(t *testing.T, choice uint8, reverse bool) {
		values := [][2]string{{"<&😀", "&lt;&amp;😀"}, {"Writer", "Writer"}, {"漢字", "漢字"}, {"a\rb", "a&#xD;b"}}
		value := values[int(choice)%len(values)]
		w, dir, _ := multiWorkspace(t)
		defer w.Close()
		want := coreFiles(t, filepath.Join(dir, revision))
		want[coreOPFPath] = bytes.Replace(want[coreOPFPath], []byte(">Writer</dc:creator>"), []byte(">"+value[1]+"</dc:creator>"), 1)
		want[coreOPFPath] = bytes.Replace(want[coreOPFPath], []byte(">en</dc:language>"), []byte(">en-a-aaa-a-bbb</dc:language>"), 1)
		ops := []Operation{coreOp(t, w, "creator", "creator", "Writer", value[0]), coreOp(t, w, "language", "", "en", "en-a-aaa-a-bbb")}
		if reverse {
			ops[0], ops[1] = ops[1], ops[0]
		}
		e := applyPlan(t, w, corePlan(t, w, ops...))
		coreAssertFiles(t, filepath.Join(dir, candidate), want)
		if _, err := w.Reject(e.TaskID); err != nil {
			t.Fatal(err)
		}
	})
}

func TestCoreMetadataFinalOPFSizeBudget(t *testing.T) {
	// The raw XML cap is the public 8 MiB contract, not an injected private
	// limit. Growing title alone crosses it; shrinking creator in the same
	// frozen transaction makes the final OPF fit, regardless of operation order.
	creator := strings.Repeat("w", 2048)
	src := strings.Replace(structureOPF, ">Writer</dc:creator>", ">"+creator+"</dc:creator>", 1)
	padding := (8 << 20) - 128 - len(src) - len("<!---->")
	src = strings.Replace(src, "</package>", "<!--"+strings.Repeat("p", padding)+"--></package>", 1)
	if len(src) != (8<<20)-128 {
		t.Fatal("boundary fixture size", len(src))
	}
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprint(reverse), func(t *testing.T) {
			w, dir, _ := structureWorkspace(t, map[string]string{coreOPFPath: src})
			defer w.Close()
			grow := coreOp(t, w, "title", "title", "Title", strings.Repeat("T", 1024))
			shrink := coreOp(t, w, "creator", "creator", creator, "W")
			coreRefusal(t, w, dir, editJSON(t, Request{8, []Operation{grow}}), "RESOURCE_LIMIT", 1)
			ops := []Operation{grow, shrink}
			if reverse {
				ops[0], ops[1] = ops[1], ops[0]
			}
			want := coreFiles(t, filepath.Join(dir, revision))
			want[coreOPFPath] = []byte(strings.Replace(strings.Replace(src, ">Title</dc:title>", ">"+strings.Repeat("T", 1024)+"</dc:title>", 1), ">"+creator+"</dc:creator>", ">W</dc:creator>", 1))
			if reflect.DeepEqual(want[coreOPFPath], []byte(src)) {
				t.Fatal("oracle did not change")
			}
			e := applyPlan(t, w, corePlan(t, w, ops...))
			coreAssertFiles(t, filepath.Join(dir, candidate), want)
			if _, err := w.Reject(e.TaskID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCoreMetadataFinalPublicationBudget(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprint(reverse), func(t *testing.T) {
			w, dir, _ := multiWorkspace(t)
			defer w.Close()
			want := coreFiles(t, filepath.Join(dir, revision))
			var total int64
			for _, b := range want {
				total += int64(len(b))
			}
			// Exercise the same publication limit without allocating 2 GiB.
			// Body grows by 12 physical bytes; OPF shrinks by 5: net +7.
			w.resources.limits.TotalBytes = total + 7
			grow := contentOpN(t, w, "EPUB/chapter1.xhtml", 0, "Alpha & one.123456789012")
			noop := coreOp(t, w, "creator", "creator", "Writer", "Writer")
			coreRefusal(t, w, dir, editJSON(t, Request{8, []Operation{grow, noop}}), "ARCHIVE_LIMIT", 1)
			shrink := coreOp(t, w, "creator", "creator", "Writer", "W")
			ops := []Operation{grow, shrink}
			if reverse {
				ops[0], ops[1] = ops[1], ops[0]
			}
			want["EPUB/chapter1.xhtml"] = bytes.Replace(want["EPUB/chapter1.xhtml"], []byte("Alpha &amp; one."), []byte("Alpha &amp; one.123456789012"), 1)
			want[coreOPFPath] = bytes.Replace(want[coreOPFPath], []byte(">Writer</dc:creator>"), []byte(">W</dc:creator>"), 1)
			e := applyPlan(t, w, corePlan(t, w, ops...))
			coreAssertFiles(t, filepath.Join(dir, candidate), want)
			if _, err := w.Reject(e.TaskID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCoreMetadataPlanAllocationScaling(t *testing.T) {
	// The same large attribute is unrelated to all targets. Increasing the
	// operation count must not allocate another parsed copy of it per target.
	var extra strings.Builder
	for i := 0; i < 32; i++ {
		fmt.Fprintf(&extra, `<dc:title id="t%d">T%d</dc:title>`, i, i)
	}
	extra.WriteString(`<meta name="pad" content="` + strings.Repeat("p", 512<<10) + `"/>`)
	src := strings.Replace(structureOPF, "</metadata>", extra.String()+"</metadata>", 1)
	w, dir, _ := structureWorkspace(t, map[string]string{coreOPFPath: src})
	defer w.Close()
	binding := coreOp(t, w, "title", "t0", "T0", "U0").Params.(metadata.SetV2)
	ops := make([]Operation, 32)
	want := coreFiles(t, filepath.Join(dir, revision))
	for i := range ops {
		s := binding
		s.ID, s.ExpectedOldValue, s.NewValue = fmt.Sprintf("t%d", i), fmt.Sprintf("T%d", i), fmt.Sprintf("U%d", i)
		ops[i] = Operation{"metadata.set", 2, s}
		want[coreOPFPath] = bytes.Replace(want[coreOPFPath], []byte(">"+s.ExpectedOldValue+"</dc:title>"), []byte(">"+s.NewValue+"</dc:title>"), 1)
	}
	measure := func(operations []Operation) (Plan, uint64) {
		t.Helper()
		request := editJSON(t, Request{8, operations})
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		p, err := w.Plan(request)
		runtime.ReadMemStats(&after)
		if err != nil {
			t.Fatal(err)
		}
		return p, after.TotalAlloc - before.TotalAlloc
	}
	_, one := measure(ops[:1])
	p, many := measure(ops)
	// A generous allocation-growth guard, not a new product memory budget.
	// It permits different indexes/implementations while rejecting N OPF copies.
	if many > one+(64<<20) {
		t.Fatalf("operation count multiplies OPF allocations: one=%d bytes, 32=%d bytes", one, many)
	}
	t.Logf("Plan allocation: one=%d bytes, 32=%d bytes", one, many)
	e := applyPlan(t, w, p)
	coreAssertFiles(t, filepath.Join(dir, candidate), want)
	runtime.GC()
	var beforeDiff, afterDiff runtime.MemStats
	runtime.ReadMemStats(&beforeDiff)
	r, err := w.TaskDiff(e.TaskID)
	runtime.ReadMemStats(&afterDiff)
	diffBytes := afterDiff.TotalAlloc - beforeDiff.TotalAlloc
	if diffBytes > 256<<20 {
		t.Fatalf("diff multiplies OPF allocations: %d bytes", diffBytes)
	}
	t.Logf("32-operation diff allocation: %d bytes", diffBytes)
	if err != nil || !r.MatchesExecution || len(r.Operations) != len(ops) {
		t.Fatalf("large mixed resource diff: %+v %v", r, err)
	}
	for i, actual := range r.Operations {
		if actual.NewValue == nil || *actual.NewValue != fmt.Sprintf("U%d", i) {
			t.Fatalf("operation %d actual value: %+v", i, actual)
		}
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal("large rejected history reopen", err)
	}
	defer reopened.Close()
	if _, err := reopened.TaskStatus(e.TaskID); err != nil {
		t.Fatal("large rejected history", err)
	}
}
