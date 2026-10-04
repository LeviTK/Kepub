package workspace

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func put(t *testing.T, name string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func treeAt(t *testing.T, dir string) Tree {
	t.Helper()
	tree, err := HashTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestTreeHashContract(t *testing.T) {
	dir := t.TempDir()
	put(t, filepath.Join(dir, "z"), []byte("abc"))
	if err := os.Mkdir(filepath.Join(dir, "a"), 0700); err != nil {
		t.Fatal(err)
	}
	got := treeAt(t, dir)
	// Independent literal framing and known SHA256("abc"), not the production
	// serializer or its generated manifest, establish this versioned contract.
	wire := `[{"path":"a","type":"directory","size":0},{"path":"z","type":"file","size":3,"sha256":"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"}]`
	want := fmt.Sprintf("%x", sha256.Sum256([]byte("kepub-tree-v1\n"+wire)))
	if got.SHA256 != want || len(got.Entries) != 2 {
		t.Fatalf("unexpected tree: %+v, want %s", got, want)
	}
	if err := os.Chtimes(filepath.Join(dir, "z"), time.Unix(123, 0), time.Unix(456, 0)); err != nil {
		t.Fatal(err)
	}
	if treeAt(t, dir).SHA256 != want {
		t.Fatal("mtime changed content hash")
	}
	put(t, filepath.Join(dir, "z"), []byte("abd")) // same size, different content
	if treeAt(t, dir).SHA256 == want {
		t.Fatal("content change ignored")
	}
}

func TestTreeOrderPathTypeAndSize(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	files := []string{"书/第二 章", "z", "a", "d/nested", "d/a"}
	for _, p := range files {
		put(t, filepath.Join(a, p), []byte(p))
	}
	for i := len(files) - 1; i >= 0; i-- {
		put(t, filepath.Join(b, files[i]), []byte(files[i]))
	}
	if !reflect.DeepEqual(treeAt(t, a), treeAt(t, b)) {
		t.Fatal("directory insertion order affected manifest/hash")
	}
	before := treeAt(t, a).SHA256
	if err := os.Rename(filepath.Join(a, "z"), filepath.Join(a, "Z")); err != nil {
		t.Fatal(err)
	}
	if treeAt(t, a).SHA256 == before {
		t.Fatal("same content, different exact path hashed equally")
	}
	c := t.TempDir()
	put(t, filepath.Join(c, "x"), nil)
	emptyFile := treeAt(t, c).SHA256
	if err := os.Remove(filepath.Join(c, "x")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(c, "x"), 0700); err != nil {
		t.Fatal(err)
	}
	if treeAt(t, c).SHA256 == emptyFile {
		t.Fatal("empty file and empty directory hashed equally")
	}
	put(t, filepath.Join(c, "x", "f"), []byte{0, 1, 2, 3})
	for _, e := range treeAt(t, c).Entries {
		if e.Path == "x/f" && e.Size != 4 {
			t.Fatalf("size = %d", e.Size)
		}
	}
}

func TestTreeRejectsUnsafeEntries(t *testing.T) {
	for _, kind := range []string{"file-link", "directory-link", "internal-link", "hardlink", "case-collision", "unicode-collision"} {
		t.Run(kind, func(t *testing.T) {
			dir, outside := t.TempDir(), t.TempDir()
			put(t, filepath.Join(outside, "secret"), []byte("do not read or change"))
			var err error
			switch kind {
			case "file-link":
				err = os.Symlink(filepath.Join(outside, "secret"), filepath.Join(dir, "x"))
			case "directory-link":
				err = os.Symlink(outside, filepath.Join(dir, "x"))
			case "internal-link":
				put(t, filepath.Join(dir, "a"), nil)
				err = os.Symlink("a", filepath.Join(dir, "x"))
			case "hardlink":
				err = os.Link(filepath.Join(outside, "secret"), filepath.Join(dir, "x"))
			case "case-collision":
				put(t, filepath.Join(dir, "a", "one"), nil)
				put(t, filepath.Join(dir, "A", "two"), nil)
			case "unicode-collision":
				put(t, filepath.Join(dir, "é"), nil)
				put(t, filepath.Join(dir, "e\u0301"), nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := HashTree(dir); err == nil {
				t.Fatal("unsafe tree accepted")
			}
			assertBytes(t, filepath.Join(outside, "secret"), []byte("do not read or change"))
		})
	}
}
