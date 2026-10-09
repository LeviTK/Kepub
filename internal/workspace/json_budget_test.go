package workspace

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

func TestR1JSONByteBudget(t *testing.T) {
	value := struct {
		Paths []string `json:"paths"`
	}{[]string{"<&\n中文"}}
	// Independent literal: HTML escaping, escaped newline, nested array,
	// UTF-8 Chinese bytes, and the final physical LF all count.
	want := []byte(`{"paths":["\u003c\u0026\n中文"]}` + "\n")
	for _, delta := range []int{-1, 0, 1} {
		t.Run(fmt.Sprintf("budget%+d", delta), func(t *testing.T) {
			r, err := os.OpenRoot(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			limit := int64(len(want) + delta)
			err = writeJSONLimit(r, "record.json", value, limit)
			if delta < 0 {
				var f *fault.Error
				if !errors.As(err, &f) || f.Code != "WORKSPACE_JSON_LIMIT" || f.Exit != 1 {
					t.Errorf("writer accepted JSON beyond byte budget (including LF): %v", err)
				}
				names, readErr := fs.ReadDir(r.FS(), ".")
				if readErr != nil || len(names) != 0 {
					t.Fatalf("over-budget write created files: %v, %v", names, readErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			b, err := r.ReadFile("record.json")
			if err != nil || !bytes.Equal(b, want) {
				t.Fatalf("JSON encoding changed: %q, %v", b, err)
			}
			var got struct {
				Paths []string `json:"paths"`
			}
			if err := readJSONLimit(r, "record.json", &got, limit); err != nil || !reflect.DeepEqual(got, value) {
				t.Fatalf("within-budget record unreadable: %+v, %v", got, err)
			}
			var f *fault.Error
			err = readJSONLimit(r, "record.json", &got, int64(len(want)-1))
			if !errors.As(err, &f) || f.Code != "WORKSPACE_JSON_LIMIT" || f.Exit != 1 {
				t.Fatalf("reader limit not classified consistently: %v", err)
			}
		})
	}
}

func TestR1LegacyRecordByteBudget(t *testing.T) {
	r, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	// Historical v1 task shape without the newer optional ID, authored bytes
	// rather than a reserialization of the record under test.
	legacy := []byte("{\"version\":1,\"baseRevision\":\"initial\"}\n")
	if err := r.WriteFile("task.json", legacy, 0600); err != nil {
		t.Fatal(err)
	}
	for _, delta := range []int{-1, 0, 1} {
		t.Run(fmt.Sprintf("budget%+d", delta), func(t *testing.T) {
			var got taskRecord
			err := readEditJSONLimit(r, "task.json", &got, int64(len(legacy)+delta))
			if delta < 0 {
				var f *fault.Error
				if !errors.As(err, &f) || f.Code != "WORKSPACE_JSON_LIMIT" || f.Exit != 1 {
					t.Fatalf("strict persisted reader budget: %v", err)
				}
				return
			}
			if err != nil || got != (taskRecord{Version: 1, BaseRevision: "initial"}) {
				t.Fatalf("legacy record changed: %+v, %v", got, err)
			}
		})
	}
	if err := writeJSON(r, "copy.json", taskRecord{Version: 1, BaseRevision: "initial"}); err != nil {
		t.Fatal(err)
	}
	b, err := r.ReadFile("copy.json")
	if err != nil || !bytes.Equal(b, legacy) {
		t.Fatalf("legacy field order/encoding changed: %q, %v", b, err)
	}
}

func TestR1ApplyRecordPreflight(t *testing.T) {
	for _, boundary := range []string{"plan", "running", "unstarted", "result", "recovery-1", "recovery", "recovery+1"} {
		t.Run(boundary, func(t *testing.T) {
			w, dir, source := multiWorkspace(t)
			defer func() { w.Close() }()
			newText := strings.Repeat("<&中文", 250)
			p, err := w.Plan(editJSON(t, Request{3, []Operation{
				contentOpN(t, w, "EPUB/chapter1.xhtml", 0, newText),
				contentOpN(t, w, "EPUB/chapter2.xhtml", 0, "Second changed"),
			}}))
			if err != nil {
				t.Fatal(err)
			}
			before := r1Files(t, dir)
			original, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			// Authored complete expected resources, independent of recompute,
			// expectedDiff and candidate output. Hash spelling is always 64 bytes.
			want := map[string][]byte{
				"EPUB/chapter1.xhtml": bytes.Replace(before[revision+"/EPUB/chapter1.xhtml"], []byte("Alpha &amp; one."), []byte(strings.Repeat("&lt;&amp;中文", 250)), 1),
				"EPUB/chapter2.xhtml": bytes.Replace(before[revision+"/EPUB/chapter2.xhtml"], []byte("Beta &amp; two."), []byte("Second changed"), 1),
			}
			changes := []map[string]any{}
			for _, name := range []string{"EPUB/chapter1.xhtml", "EPUB/chapter2.xhtml"} {
				old := before[revision+"/"+name]
				changes = append(changes, map[string]any{"path": name, "kind": "modified",
					"before": map[string]any{"path": name, "type": "file", "size": len(old), "sha256": fmt.Sprintf("%x", sha256.Sum256(old))},
					"after":  map[string]any{"path": name, "type": "file", "size": len(want[name]), "sha256": fmt.Sprintf("%x", sha256.Sum256(want[name]))}})
			}
			diff := map[string]any{"beforeSha256": strings.Repeat("0", 64), "afterSha256": strings.Repeat("0", 64), "changed": false, "changes": []any{}}
			record := map[string]any{"version": 3, "taskId": strings.Repeat("0", 32), "plan": p, "checkpoint": strings.Repeat("0", 32), "status": "running", "reviewRequired": false, "conformance": "not_run", "diff": diff}
			size := func(v any) int64 {
				b, err := json.Marshal(v)
				if err != nil {
					t.Fatal(err)
				}
				return int64(len(b) + 1)
			}
			limits := map[string]int64{"plan": size(p), "running": size(record)}
			record["status"] = "unstarted"
			limits["unstarted"] = size(record)
			diff["changed"], diff["changes"] = true, changes
			record["status"], record["reviewRequired"] = "review_required", true
			limits["result"] = size(record)
			record["status"], record["reviewRequired"], record["failure"] = "failed", false, "interrupted apply"
			limits["recovery"] = size(record)
			limits["recovery-1"], limits["recovery+1"] = size(record)-1, size(record)+1
			limit := limits[boundary]
			e, err := w.apply(editJSON(t, p), limit)
			accept := boundary == "recovery" || boundary == "recovery+1"
			if !accept {
				var f *fault.Error
				if !errors.As(err, &f) || f.Code != "WORKSPACE_JSON_LIMIT" || f.Exit != 1 || e.TaskID != "" {
					t.Fatalf("must refuse before publication at %d bytes: task=%q err=%v", limit, e.TaskID, err)
				}
				if !reflect.DeepEqual(before, r1Files(t, dir)) {
					t.Fatal("refused Apply changed workspace files or consumed plan")
				}
			} else {
				if err != nil || e.Status != "review_required" {
					t.Fatalf("exact/adjacent budget must apply: %s %v", e.Status, err)
				}
				for name, old := range before {
					if strings.HasPrefix(name, revision+"/") {
						bp := strings.TrimPrefix(name, revision+"/")
						expected := old
						if b, ok := want[bp]; ok {
							expected = b
						}
						assertBytes(t, filepath.Join(dir, candidate, bp), expected)
						assertBytes(t, filepath.Join(dir, name), old)
					}
				}
			}
			for range 2 {
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
				w, err = Open(dir)
				if err != nil {
					t.Fatal("must reopen without repeated checkpoint creation", err)
				}
				if !accept && (exists(w.root, "tasks/active") || exists(w.root, "plans/"+p.ID+".used.json")) {
					t.Fatal("refused plan left a task or consumption record")
				}
			}
			if !accept {
				// Refusal did not consume the plan: the same bytes can be used
				// under the unchanged production limit after reopening.
				e, err = w.Apply(editJSON(t, p))
				if err != nil || e.Status != "review_required" {
					t.Fatalf("refused plan is no longer usable: %v", err)
				}
			}
			if _, err := w.TaskStatus(e.TaskID); err != nil {
				t.Fatal(err)
			}
			if _, err := w.TaskDiff(e.TaskID); err != nil {
				t.Fatal(err)
			}
			if _, err := w.Reject(e.TaskID); err != nil {
				t.Fatal(err)
			}
			assertBytes(t, source, original)
			assertBytes(t, filepath.Join(dir, "original/book.epub"), original)
			t.Logf("limit=%d plan=%d recovery=%d accepted=%v", limit, limits["plan"], limits["recovery"], accept)
		})
	}
}

// Exercise the public API at the real production limit as well as the small
// injected matrix. The initial book is tiny; large strings are created serially.
func TestR1ApplyProductionRecordLimit(t *testing.T) {
	dir := t.TempDir()
	source, ws := filepath.Join(dir, "input.epub"), filepath.Join(dir, "workspace")
	entries := []testfixture.Entry{
		{Name: "mimetype", Data: []byte("application/epub+zip")},
		{Name: "META-INF/container.xml", Data: []byte(`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)},
		{Name: "EPUB/package.opf"},
	}
	const chapter = `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>One</title></head><body><p>old</p></body></html>`
	var manifest, spine strings.Builder
	ops := []Operation{}
	for i := range 40 {
		name := fmt.Sprintf("chapter%02d.xhtml", i)
		fmt.Fprintf(&manifest, `<item id="c%d" href="%s" media-type="application/xhtml+xml"/>`, i, name)
		fmt.Fprintf(&spine, `<itemref idref="c%d"/>`, i)
		entries = append(entries, testfixture.Entry{Name: "EPUB/" + name, Data: []byte(chapter)})
		ops = append(ops, Operation{"content.text.set", 1, publication.TextSet{
			BookPath: bookpath.BookPath("EPUB/" + name), RevisionID: "initial", ResourceSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(chapter))),
			LocatorVersion: 1, Locator: "/html[1]/body[1]/p[1]", ExpectedOldValue: "old", NewValue: "x",
		}})
	}
	for i := range entries {
		if entries[i].Name == "EPUB/package.opf" {
			entries[i].Data = []byte(`<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">test</dc:identifier><dc:title>Title</dc:title><dc:language>en</dc:language></metadata><manifest>` + manifest.String() + `</manifest><spine>` + spine.String() + `</spine></package>`)
		}
	}
	testfixture.ZIP(t, source, entries)
	w, err := Create(ws, source, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if w != nil {
			w.Close()
		}
	}()
	p, err := w.Plan(editJSON(t, Request{3, ops}))
	if err != nil {
		t.Fatal(err)
	}
	// Plan fields and write set keep the same encoded lengths. Distribute
	// ASCII padding below the per-text limit, to make plan+LF exactly 32 MiB.
	padding := maxJSONBytes - (len(editJSON(t, p)) + 1)
	for i := range ops {
		s := ops[i].Params.(publication.TextSet)
		n := padding / (len(ops) - i)
		s.NewValue += strings.Repeat("x", n)
		padding -= n
		ops[i].Params = s
	}
	p, err = w.Plan(editJSON(t, Request{3, ops}))
	if err != nil {
		t.Fatal(err)
	}
	b := editJSON(t, p)
	if len(b)+1 != maxJSONBytes {
		t.Fatalf("fixture did not hit actual plan limit: %d", len(b)+1)
	}
	stored, err := w.root.Stat("plans/" + p.ID + ".json")
	if err != nil || stored.Size() != maxJSONBytes {
		t.Fatalf("plan not stored at limit: %v", err)
	}
	t.Logf("stored plan=%d bytes, production limit=%d", stored.Size(), maxJSONBytes)
	e, err := w.Apply(b)
	var f *fault.Error
	if !errors.As(err, &f) || f.Code != "WORKSPACE_JSON_LIMIT" || f.Exit != 1 {
		t.Errorf("oversize execution must be classified: %v", err)
	}
	if e.TaskID != "" || exists(w.root, "tasks/active") || exists(w.root, "plans/"+p.ID+".used.json") {
		t.Error("oversize execution published a task or consumed the plan")
	}
	for range 2 {
		w.Close()
		w, err = Open(ws)
		if err != nil {
			t.Fatal("over-limit Apply poisoned subsequent Open", err)
		}
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name, "/") {
			assertBytes(t, filepath.Join(ws, revision, entry.Name), entry.Data)
		}
	}
	if stages, err := fs.ReadDir(w.root.FS(), "staging"); err != nil || len(stages) != 0 {
		t.Fatalf("preflight left staging: %v %v", stages, err)
	}
}

func r1Files(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	files := map[string][]byte{}
	err := filepath.WalkDir(dir, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		b, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, name)
		files[filepath.ToSlash(rel)] = b
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestR1CreateLongPathInventory(t *testing.T) {
	parent := t.TempDir()
	source := filepath.Join(parent, "input.epub")
	entries := testfixture.EPUB("3.0", false)
	for i := range 40 {
		entries = append(entries, testfixture.Entry{
			Name: fmt.Sprintf("资料/%03d-%s.bin", i, strings.Repeat("中文<&", 20)),
			Data: []byte{byte(i)},
		})
	}
	testfixture.ZIP(t, source, entries)
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	before := sha256.Sum256(original)
	baseline := filepath.Join(parent, "baseline")
	w, err := Create(baseline, source, Options{})
	if err != nil {
		t.Fatal("fixture paths and archive must be legal", err)
	}
	state := w.State()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(baseline, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, delta := range []int{-1, 0, 1} {
		t.Run(fmt.Sprintf("budget%+d", delta), func(t *testing.T) {
			dir := filepath.Join(parent, fmt.Sprintf("workspace%+d", delta))
			w, err := create(dir, source, Options{}, int64(len(want)+delta))
			if delta < 0 {
				if w != nil {
					w.Close()
				}
				var f *fault.Error
				if !errors.As(err, &f) || f.Code != "WORKSPACE_JSON_LIMIT" || f.Exit != 1 {
					t.Errorf("Create published an over-budget inventory: %v", err)
				}
				if _, err := os.Lstat(dir); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("rejected Create left a destination: %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
				assertBytes(t, filepath.Join(dir, "state.json"), want)
				w, err = Open(dir)
				if err != nil {
					t.Fatal("successful Create must reopen", err)
				}
				defer w.Close()
				if !reflect.DeepEqual(w.State(), state) {
					t.Fatal("inventory/provenance changed on reopen")
				}
				for _, e := range entries {
					if !strings.HasSuffix(e.Name, "/") {
						assertBytes(t, filepath.Join(dir, revision, e.Name), e.Data)
					}
				}
			}
			after, err := os.ReadFile(source)
			if err != nil || sha256.Sum256(after) != before {
				t.Fatal("source archive changed", err)
			}
			assertBytes(t, filepath.Join(baseline, "state.json"), want)
			stages, err := filepath.Glob(filepath.Join(parent, ".kepub-create-*"))
			if err != nil || len(stages) != 0 {
				t.Fatalf("Create leaked staging: %v, %v", stages, err)
			}
		})
	}
}
