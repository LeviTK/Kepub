package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

func r8PathFixture(t *testing.T, components int) (string, string, []testfixture.Entry) {
	t.Helper()
	resource := "书/Deep/" + strings.Repeat("d/", components-3) + "chapter.xhtml"
	entries := testfixture.EPUB("3.0", false)
	entries[2].Data = []byte(strings.Replace(string(entries[2].Data), "../Text/-first.xhtml", strings.TrimPrefix(resource, "书/Deep/"), 1))
	entries[4].Name = resource
	entries[4].Data = []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="boundary">R8 original</p></body></html>`)
	source := filepath.Join(t.TempDir(), "deep.epub")
	testfixture.ZIP(t, source, entries)
	return source, resource, entries
}

// This assertion also runs unchanged against the Issue's sealed review base.
// It is deliberately independent of the later private I/O-budget test seam.
func TestR8CreateAtDepthBoundary(t *testing.T) {
	for _, components := range []int{125, 126, 127, 128} {
		for _, root := range []string{"w", strings.Repeat("host", 25)} {
			t.Run(fmt.Sprintf("%d/root-%d", components, len(root)), func(t *testing.T) {
				source, resource, entries := r8PathFixture(t, components)
				dir := filepath.Join(t.TempDir(), root)
				w, err := Create(dir, source, Options{})
				if err != nil {
					t.Fatalf("legal %d-component BookPath rejected by internal prefix: %v", components, err)
				}
				for _, entry := range entries {
					if !entry.Mode.IsDir() {
						assertBytes(t, filepath.Join(dir, revision, entry.Name), entry.Data)
					}
				}
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
				w, err = Open(dir)
				if err != nil {
					t.Fatal("created workspace cannot reopen", err)
				}
				defer w.Close()
				assertBytes(t, filepath.Join(dir, revision, resource), entries[4].Data)
			})
		}
	}
}

// Fixture bytes and explicit parent directories determine the complete expected
// kepub-tree-v1 inventory, independently of a workspace/hash/copy result.
func r8ExpectedTree(t *testing.T, files []testfixture.Entry) Tree {
	t.Helper()
	entries := map[string]Entry{}
	for _, file := range files {
		name := strings.TrimSuffix(file.Name, "/")
		entry := Entry{Path: name, Type: "directory"}
		if !file.Mode.IsDir() {
			entry.Type, entry.Size = "file", int64(len(file.Data))
			entry.SHA256 = fmt.Sprintf("%x", sha256.Sum256(file.Data))
		}
		entries[name] = entry
		for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
			entries[parent] = Entry{Path: parent, Type: "directory"}
		}
	}
	tree := Tree{Entries: []Entry{}}
	for _, entry := range entries {
		tree.Entries = append(tree.Entries, entry)
	}
	sort.Slice(tree.Entries, func(i, j int) bool { return tree.Entries[i].Path < tree.Entries[j].Path })
	wire, err := json.Marshal(tree.Entries)
	if err != nil {
		t.Fatal(err)
	}
	tree.SHA256 = fmt.Sprintf("%x", sha256.Sum256(append([]byte("kepub-tree-v1\n"), wire...)))
	return tree
}

func TestR8PublicationPrefixesAndBudgets(t *testing.T) {
	source, resource, files := r8PathFixture(t, 128)
	original := readResource(t, source)
	want := r8ExpectedTree(t, files)
	policy := defaultResourceIO(t.Context())
	policy.limits.Entries = len(want.Entries)
	policy.limits.PathBytes = 0
	for _, entry := range want.Entries {
		policy.limits.PathBytes += int64(len(entry.Path))
	}
	dir := filepath.Join(t.TempDir(), "workspace")
	w, err := createWithIO(dir, source, Options{}, maxJSONBytes, policy)
	if err != nil {
		t.Fatal("management paths counted as publication inventory", err)
	}
	defer func() { w.Close() }()
	if !reflect.DeepEqual(w.State().Tree, want) {
		t.Fatal("initial tree differs from fixture bytes/explicit parent inventory")
	}
	a, inventory, r, err := w.AcceptedSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	p, err := publication.Load(a, r.Rootfile)
	if err != nil {
		a.Close()
		t.Fatal(err)
	}
	c, err := publication.ReadContent(a, p, bookpath.BookPath(resource), publication.ContentOptions{})
	a.Close()
	if err != nil || c.ReturnedCount != 1 || c.Nodes[0].Text != "R8 original" || c.Nodes[0].Locator != "/html[1]/body[1]/p[1]" || c.ResourceSHA256 != fmt.Sprintf("%x", sha256.Sum256(files[4].Data)) || inventory.SHA256 != want.SHA256 {
		t.Fatal("accepted read changed exact bytes/hash/locator", c, err)
	}
	pub, err := w.NewCandidate()
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := w.Checkpoint()
	if err != nil || !reflect.DeepEqual(checkpoint.Tree, want) {
		t.Fatal("checkpoint prefix affected publication tree", checkpoint, err)
	}
	put(t, filepath.Join(pub, resource), []byte(strings.Replace(string(files[4].Data), "original", "modified", 1)))
	if err := w.Restore(checkpoint.ID); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	w, err = openWithIO(dir, policy)
	if err != nil {
		t.Fatal("deep candidate/checkpoint cannot reopen", err)
	}
	checkpoints, err := w.Checkpoints()
	if err != nil || len(checkpoints) != 1 || !reflect.DeepEqual(checkpoints[0].Tree, want) {
		t.Fatal(checkpoints, err)
	}
	for _, prefix := range []string{revision, candidate, checkpointDir(checkpoint.ID) + "/pub"} {
		for _, file := range files {
			if !file.Mode.IsDir() {
				assertBytes(t, filepath.Join(dir, prefix, file.Name), file.Data)
			}
		}
	}
	assertBytes(t, source, original)
	assertBytes(t, filepath.Join(dir, "original/book.epub"), original)
	assertEmpty(t, filepath.Join(dir, "staging"))
	assertEmpty(t, filepath.Join(dir, "journal"))
}

func TestR8PathRefusalsPreserveInputs(t *testing.T) {
	for _, tc := range []struct{ name, path, code string }{
		{"depth", strings.Repeat("d/", 128) + "file", "PATH_LIMIT"},
		{"bytes", strings.Repeat("x", 4097), "PATH_LIMIT"},
		{"traversal", "../outside", "UNSAFE_PATH"},
		{"absolute", "/absolute", "UNSAFE_PATH"},
		{"symlink", "linked", "UNSAFE_ENTRY"},
		{"case", "MIMETYPE", "ARCHIVE_COLLISION"},
		{"unicode", "e\u0301.txt", "ARCHIVE_COLLISION"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := testfixture.EPUB("3.0", false)
			if tc.name == "unicode" {
				files = append(files, testfixture.Entry{Name: "é.txt", Data: []byte("first")})
			}
			entry := testfixture.Entry{Name: tc.path, Data: []byte("refused")}
			if tc.name == "symlink" {
				entry.Mode = os.ModeSymlink | 0700
			}
			files = append(files, entry)
			root := t.TempDir()
			source := filepath.Join(root, "input.epub")
			testfixture.ZIP(t, source, files)
			original := readResource(t, source)
			dir := filepath.Join(root, "refused-workspace")
			w, err := Create(dir, source, Options{})
			if w != nil {
				w.Close()
				t.Fatal("invalid path published")
			}
			r3Limit(t, err, tc.code)
			if _, err := os.Lstat(dir); !os.IsNotExist(err) {
				t.Fatal("partial workspace retained", err)
			}
			stages, err := filepath.Glob(filepath.Join(root, ".kepub-create-*"))
			if err != nil || len(stages) != 0 {
				t.Fatal("failed create leaked stage", stages, err)
			}
			assertBytes(t, source, original)
		})
	}
}

func TestR8PrepublishSyncCancellation(t *testing.T) {
	source, _, _ := r8PathFixture(t, 128)
	original := readResource(t, source)
	root := t.TempDir()
	dir := filepath.Join(root, "ws")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	policy := defaultResourceIO(r3HookContext{Context: ctx, check: func() {
		identities, _ := filepath.Glob(filepath.Join(root, ".kepub-create-*/identity.json"))
		if len(identities) != 0 {
			cancel() // Metadata is complete; cancel at the publication sync phase.
		}
	}})
	w, err := createWithIO(dir, source, Options{}, maxJSONBytes, policy)
	if w != nil {
		w.Close()
		t.Fatal("cancelled sync published workspace")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatal("sync cancellation lost", err)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatal("failed sync retained destination", err)
	}
	stages, _ := filepath.Glob(filepath.Join(root, ".kepub-create-*"))
	if len(stages) != 0 {
		t.Fatal("cancelled sync leaked stage", stages)
	}
	assertBytes(t, source, original)
	w, err = Create(dir, source, Options{})
	if err != nil {
		t.Fatal("clean retry after sync cancellation failed", err)
	}
	w.Close()
	w, err = Open(dir)
	if err != nil {
		t.Fatal("retry cannot reopen", err)
	}
	w.Close()
}

func TestR8HostPathFailureIsNotBookPathBudget(t *testing.T) {
	name := strings.Repeat("x", 256) // Logical path fits; ext4's leaf limit does not.
	if _, err := bookpath.Parse(name); err != nil {
		t.Fatal("physical leaf length turned into a BookPath budget", err)
	}
	source := filepath.Join(t.TempDir(), "host-limit.epub")
	files := append(testfixture.EPUB("3.0", false), testfixture.Entry{Name: name, Data: []byte("one byte")})
	testfixture.ZIP(t, source, files)
	original := readResource(t, source)
	a, err := archive.Open(source, archive.DefaultLimits)
	if a != nil {
		a.Close()
		t.Fatal("fixture did not hit the actual host leaf limit")
	}
	var pathError *os.PathError
	if !errors.As(err, &pathError) || !errors.Is(err, syscall.ENAMETOOLONG) {
		t.Fatal("physical refusal not preserved as OS path error", err)
	}
	assertBytes(t, source, original)
}
