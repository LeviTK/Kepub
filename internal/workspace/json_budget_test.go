package workspace

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/fault"
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
