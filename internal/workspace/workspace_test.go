package workspace

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/LeviTK/Kepub/internal/testfixture"
)

func fixture(t *testing.T) (string, []testfixture.Entry) {
	t.Helper()
	source := filepath.Join(t.TempDir(), "input.epub")
	entries := testfixture.EPUB("3.0", false)
	testfixture.ZIP(t, source, entries)
	return source, entries
}

func makeWorkspace(t *testing.T) (*Workspace, string) {
	t.Helper()
	source, _ := fixture(t)
	dir := filepath.Join(t.TempDir(), "ws")
	w, err := Create(dir, source, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close() })
	return w, dir
}

func assertBytes(t *testing.T, name string, want []byte) {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil || !bytes.Equal(b, want) {
		t.Fatalf("%s: got %q, want %q, err %v", name, b, want, err)
	}
}

func assertEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("directory not empty: %s: %v, %v", dir, entries, err)
	}
}

func TestIndependentCopiesRestoreAndReopen(t *testing.T) {
	source, entries := fixture(t)
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "ws")
	w, err := Create(dir, source, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	state := w.State()
	if state.InitialRevision != "initial" || state.Rootfile != "书/Deep/package.opf" || len(state.ReadOnlyReasons) != 0 {
		t.Fatalf("state: %+v", state)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name, "/") {
			assertBytes(t, filepath.Join(dir, revision, e.Name), e.Data)
		}
	}
	pub, err := w.NewCandidate()
	if err != nil {
		t.Fatal(err)
	}
	if treeAt(t, pub).SHA256 != state.Tree.SHA256 {
		t.Fatal("candidate differs from initial tree")
	}
	if _, err := w.NewCandidate(); !os.IsExist(err) && !errors.Is(err, os.ErrExist) {
		t.Fatalf("second candidate: %v", err)
	}
	first, err := w.Checkpoint()
	if err != nil {
		t.Fatal(err)
	}
	// Exercise ordinary truncation, in-place write, rename, add and delete: no
	// workspace write API is involved, and no hard-link break hook can help.
	put(t, filepath.Join(pub, "书/Text/-first.xhtml"), []byte("<broken"))
	f, err := os.OpenFile(filepath.Join(pub, "unlisted.bin"), os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte{99, 98}, 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := os.Rename(filepath.Join(pub, "书/nav.xhtml"), filepath.Join(pub, "书/renamed.xhtml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(pub, "mimetype")); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(pub, "new-user-file.txt"), []byte("preserve in checkpoint"))
	put(t, filepath.Join(dir, "tasks/active/work/notes.txt"), []byte("outside pub"))
	put(t, filepath.Join(dir, "user-file.txt"), []byte("outside metadata"))
	second, err := w.Checkpoint() // invalid publication content is not a rejected snapshot
	if err != nil {
		t.Fatal(err)
	}
	if first.Tree.SHA256 == second.Tree.SHA256 {
		t.Fatal("direct writes were ignored")
	}
	assertBytes(t, source, original)
	assertBytes(t, filepath.Join(dir, "original/book.epub"), original)
	if treeAt(t, filepath.Join(dir, revision)).SHA256 != state.Tree.SHA256 || treeAt(t, filepath.Join(dir, checkpointDir(first.ID), "pub")).SHA256 != state.Tree.SHA256 {
		t.Fatal("candidate writes polluted immutable copies")
	}
	if err := w.Restore(first.ID); err != nil {
		t.Fatal(err)
	}
	if treeAt(t, pub).SHA256 != state.Tree.SHA256 {
		t.Fatal("restore failed to remove added/renamed paths or recover deleted paths")
	}
	assertBytes(t, filepath.Join(dir, "tasks/active/work/notes.txt"), []byte("outside pub"))
	put(t, filepath.Join(pub, "unlisted.bin"), []byte("after restore"))
	if treeAt(t, filepath.Join(dir, checkpointDir(first.ID), "pub")).SHA256 != first.Tree.SHA256 {
		t.Fatal("restored candidate shares checkpoint storage")
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	// Reopening does not depend on the original input path still existing.
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	w, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	got, err := w.Candidate()
	if err != nil || got != pub {
		t.Fatalf("candidate after reopen: %q %v", got, err)
	}
	checkpoints, err := w.Checkpoints()
	if err != nil || len(checkpoints) != 2 {
		t.Fatalf("checkpoints after reopen: %v, %v", checkpoints, err)
	}
	if err := w.Restore(second.ID); err != nil {
		t.Fatal(err)
	}
	if treeAt(t, pub).SHA256 != second.Tree.SHA256 {
		t.Fatal("second checkpoint not restored exactly")
	}
	assertBytes(t, filepath.Join(pub, "new-user-file.txt"), []byte("preserve in checkpoint"))
	assertBytes(t, filepath.Join(dir, "user-file.txt"), []byte("outside metadata"))
	assertEmpty(t, filepath.Join(dir, "staging"))
	assertEmpty(t, filepath.Join(dir, "journal"))
}

func TestCreateFailuresLeaveNoPartialDestination(t *testing.T) {
	for _, kind := range []string{"invalid-zip", "invalid-opf", "missing-source", "source-symlink", "unsafe-entry", "multiple-rootfiles"} {
		t.Run(kind, func(t *testing.T) {
			base := t.TempDir()
			source := filepath.Join(base, "book.epub")
			entries := testfixture.EPUB("3.0", kind == "multiple-rootfiles")
			switch kind {
			case "invalid-zip":
				put(t, source, []byte("not ZIP"))
			case "missing-source":
			case "source-symlink":
				other, _ := fixture(t)
				if err := os.Symlink(other, source); err != nil {
					t.Fatal(err)
				}
			default:
				if kind == "invalid-opf" {
					entries[2].Data = []byte("<not-package/>")
				}
				if kind == "unsafe-entry" {
					entries = append(entries, testfixture.Entry{Name: "../outside", Data: []byte("no")})
				}
				testfixture.ZIP(t, source, entries)
			}
			before, err := os.ReadDir(base)
			if err != nil {
				t.Fatal(err)
			}
			if w, err := Create(filepath.Join(base, "ws"), source, Options{}); err == nil {
				w.Close()
				t.Fatal("bad input succeeded")
			}
			after, err := os.ReadDir(base)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("failed Create left filesystem changes: %v -> %v, %v", before, after, err)
			}
		})
	}
}

func TestExistingDestinationAndConcurrentCreation(t *testing.T) {
	source, _ := fixture(t)
	for _, kind := range []string{"file", "empty-dir", "nonempty-dir", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			base := t.TempDir()
			dest := filepath.Join(base, "existing")
			switch kind {
			case "file":
				put(t, dest, []byte("keep"))
			case "empty-dir", "nonempty-dir":
				if err := os.Mkdir(dest, 0700); err != nil {
					t.Fatal(err)
				}
				if kind == "nonempty-dir" {
					put(t, filepath.Join(dest, "keep"), []byte("keep"))
				}
			case "symlink":
				if err := os.Symlink("missing", dest); err != nil {
					t.Fatal(err)
				}
			}
			before, _ := os.Lstat(dest)
			if w, err := Create(dest, source, Options{}); err == nil {
				w.Close()
				t.Fatal("existing destination accepted")
			}
			after, _ := os.Lstat(dest)
			if !os.SameFile(before, after) {
				t.Fatal("existing object replaced")
			}
		})
	}
	base := t.TempDir()
	dest := filepath.Join(base, "race")
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan *Workspace, 8)
	for range 8 {
		wg.Go(func() {
			<-start
			w, err := Create(dest, source, Options{})
			if err == nil {
				results <- w
			} else if !errors.Is(err, os.ErrExist) {
				t.Errorf("unexpected create conflict: %v", err)
			}
		})
	}
	close(start)
	wg.Wait()
	close(results)
	count := 0
	for w := range results {
		count++
		w.Close()
	}
	if count != 1 {
		t.Fatalf("successful creators = %d", count)
	}
	entries, _ := os.ReadDir(base)
	if len(entries) != 1 || entries[0].Name() != "race" {
		t.Fatalf("race leaked staging: %v", entries)
	}
	w, err := Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
}

func TestReadOnlyInputsPreserved(t *testing.T) {
	for _, name := range []string{"AGENTS.md", "sub/.amp/settings.json", ".mcp.json", "META-INF/signatures.xml", "META-INF/encryption.xml"} {
		t.Run(name, func(t *testing.T) {
			source := filepath.Join(t.TempDir(), "book.epub")
			entries := append(testfixture.EPUB("3.0", false), testfixture.Entry{Name: name, Data: []byte("<untrusted/>")})
			testfixture.ZIP(t, source, entries)
			dir := filepath.Join(t.TempDir(), "ws")
			w, err := Create(dir, source, Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			if len(w.State().ReadOnlyReasons) == 0 {
				t.Fatal("missing restriction")
			}
			if _, err := w.NewCandidate(); !errors.Is(err, ErrReadOnly) {
				t.Fatalf("untrusted candidate: %v", err)
			}
			assertBytes(t, filepath.Join(dir, revision, name), []byte("<untrusted/>"))
			assertEmpty(t, filepath.Join(dir, "tasks"))
		})
	}
}

func TestTamperingAndStateCopies(t *testing.T) {
	w, dir := makeWorkspace(t)
	s := w.State()
	s.Tree.Entries[0].Path = "changed"
	if w.State().Tree.Entries[0].Path == "changed" {
		t.Fatal("State aliases internal memory")
	}
	put(t, filepath.Join(dir, revision, "unlisted.bin"), []byte("corrupt"))
	if _, err := w.NewCandidate(); err == nil {
		t.Fatal("tampered baseline accepted")
	}
	w.Close()
	if _, err := w.NewCandidate(); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	for range 2 { // Failed Open must release its lock, not mask corruption as busy.
		if other, err := Open(dir); err == nil || errors.Is(err, ErrBusy) {
			if other != nil {
				other.Close()
			}
			t.Fatalf("tampered Open: %v", err)
		}
	}
}
