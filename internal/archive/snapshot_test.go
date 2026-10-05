package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

func fixtureSnapshot(t *testing.T) (*Archive, Tree, string) {
	t.Helper()
	dir := t.TempDir()
	filename := filepath.Join(dir, "source.epub")
	testfixture.ZIP(t, filename, testfixture.EPUB("3.0", false))
	a, e := Open(filename, DefaultLimits)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(a.Close)
	root := filepath.Join(dir, "pub")
	if e = a.Unpack(root); e != nil {
		t.Fatal(e)
	}
	s, tree, e := SnapshotDirectory(root, DefaultLimits)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(s.Close)
	return s, tree, root
}
func TestPackHeadersBytesAndNoClobber(t *testing.T) {
	a, tree, root := fixtureSnapshot(t)
	var b bytes.Buffer
	if e := a.WriteZIP(&b, tree); e != nil {
		t.Fatal(e)
	}
	data := b.Bytes()
	if binary.LittleEndian.Uint32(data) != 0x04034b50 || binary.LittleEndian.Uint16(data[6:]) != 0 || binary.LittleEndian.Uint16(data[8:]) != zip.Store || binary.LittleEndian.Uint16(data[28:]) != 0 || binary.LittleEndian.Uint16(data[26:]) != 8 || binary.LittleEndian.Uint32(data[18:]) != 20 || binary.LittleEndian.Uint32(data[22:]) != 20 || string(data[30:38]) != "mimetype" || string(data[38:58]) != "application/epub+zip" {
		t.Fatalf("bad raw first header: %x", data[:58])
	}
	z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		t.Fatal(e)
	}
	mime := z.File[0]
	if mime.Name != "mimetype" || mime.Method != zip.Store || len(mime.Extra) != 0 || mime.Flags != 0 {
		t.Fatal("bad central header", mime.FileHeader)
	}
	for _, f := range z.File {
		if f.FileInfo().IsDir() {
			if f.UncompressedSize64 != 0 {
				t.Fatal("directory bytes")
			}
			continue
		}
		r, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		actual, e := io.ReadAll(r)
		r.Close()
		expected, e2 := os.ReadFile(filepath.Join(root, f.Name))
		if e != nil || e2 != nil || !bytes.Equal(actual, expected) {
			t.Fatal("resource bytes changed", f.Name, e, e2)
		}
	}
	output := filepath.Join(t.TempDir(), "roundtrip.epub")
	check := func(f, h string) error {
		actual, e := FileSHA256(f)
		if e != nil {
			return e
		}
		if actual != h {
			return errors.New("hash differs")
		}
		return nil
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := a.PublishZIP(context.Background(), output, tree, check); results <- e }()
	}
	wg.Wait()
	close(results)
	success := 0
	for e := range results {
		if e == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatal("atomic no-replace", success)
	}
	if _, e = a.PublishZIP(context.Background(), output, tree, check); e == nil {
		t.Fatal("clobbered existing output")
	}
	source := filepath.Join(filepath.Dir(root), "source.epub")
	before, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = a.PublishZIP(context.Background(), source, tree, check); e == nil {
		t.Fatal("overwrote source archive")
	}
	after, e := os.ReadFile(source)
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("source changed")
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if e = os.Symlink(root, alias); e != nil {
		t.Fatal(e)
	}
	if _, e = a.PublishZIP(context.Background(), filepath.Join(alias, "export.epub"), tree, check); e == nil {
		t.Fatal("followed output parent link")
	}
	for _, mutate := range []bool{false, true} {
		output = filepath.Join(t.TempDir(), "failed.epub")
		_, e = a.PublishZIP(context.Background(), output, tree, func(f, h string) error {
			if mutate {
				return os.WriteFile(f, []byte("replacement"), 0600)
			}
			return errors.New("check failure")
		})
		if e == nil {
			t.Fatal("published checker failure/drift")
		}
		if _, e = os.Lstat(output); !os.IsNotExist(e) {
			t.Fatal("partial publication")
		}
		matches, _ := filepath.Glob(filepath.Join(filepath.Dir(output), ".kepub-pack-*"))
		if len(matches) != 0 {
			t.Fatal("leaked private staging", matches)
		}
	}
}

func TestPublicationCancellationBeforeCommit(t *testing.T) {
	a, tree, _ := fixtureSnapshot(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	output := filepath.Join(t.TempDir(), "cancelled.epub")
	hash, e := a.PublishZIP(ctx, output, tree, func(string, string) error { cancel(); return nil })
	var f *fault.Error
	if !errors.As(e, &f) || f.Code != "PUBLICATION_CANCELLED" || hash != "" {
		t.Fatal(hash, e)
	}
	if _, e := os.Stat(output); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("published after observed cancellation", e)
	}
	left, e := filepath.Glob(filepath.Join(filepath.Dir(output), ".kepub-pack-*"))
	if e != nil || len(left) != 0 {
		t.Fatal("staging leak", left, e)
	}
}

func TestApprovalRejectsDriftAndExtra(t *testing.T) {
	for _, kind := range []string{"content", "extra", "removed", "approval", "link"} {
		t.Run(kind, func(t *testing.T) {
			a, tree, _ := fixtureSnapshot(t)
			switch kind {
			case "content":
				if e := os.WriteFile(filepath.Join(a.dir, "unlisted.bin"), []byte("different"), 0600); e != nil {
					t.Fatal(e)
				}
			case "extra":
				if e := os.WriteFile(filepath.Join(a.dir, "new-tool.txt"), []byte("not approved"), 0600); e != nil {
					t.Fatal(e)
				}
			case "removed":
				if e := os.Remove(filepath.Join(a.dir, "unlisted.bin")); e != nil {
					t.Fatal(e)
				}
			case "approval":
				tree.SHA256 = "forged"
			case "link":
				if e := os.Symlink("/etc/passwd", filepath.Join(a.dir, "link")); e != nil {
					t.Fatal(e)
				}
			}
			if e := a.WriteZIP(io.Discard, tree); e == nil {
				t.Fatal("drift accepted")
			}
		})
	}
}

func TestDirectorySafetyLimitsAndFrozenCopy(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "fifo", "case", "unicode", "backslash", "parentlink", "limit"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			if e := os.WriteFile(filepath.Join(dir, "mimetype"), []byte("application/epub+zip"), 0600); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "symlink":
				if e := os.Symlink("/etc/passwd", filepath.Join(dir, "x")); e != nil {
					t.Fatal(e)
				}
			case "hardlink":
				if e := os.Link(filepath.Join(dir, "mimetype"), filepath.Join(dir, "x")); e != nil {
					t.Fatal(e)
				}
			case "fifo":
				if e := syscall.Mkfifo(filepath.Join(dir, "x"), 0600); e != nil {
					t.Fatal(e)
				}
			case "case":
				os.Mkdir(filepath.Join(dir, "Text"), 0700)
				os.Mkdir(filepath.Join(dir, "text"), 0700)
			case "unicode":
				os.Mkdir(filepath.Join(dir, "é"), 0700)
				os.Mkdir(filepath.Join(dir, "e\u0301"), 0700)
			case "backslash":
				os.WriteFile(filepath.Join(dir, "..\\escape"), nil, 0600)
			case "parentlink":
				link := filepath.Join(t.TempDir(), "alias")
				if e := os.Symlink(dir, link); e != nil {
					t.Fatal(e)
				}
				dir = link
			case "limit":
				os.WriteFile(filepath.Join(dir, "big"), make([]byte, 33), 0600)
			}
			limits := DefaultLimits
			if kind == "limit" {
				limits.FileBytes = 32
			}
			a, _, e := SnapshotDirectory(dir, limits)
			if e == nil {
				a.Close()
				t.Fatal("unsafe directory accepted")
			}
			var f *fault.Error
			if !errors.As(e, &f) {
				t.Fatal("missing safety diagnostic", e)
			}
		})
	}
	a, tree, root := fixtureSnapshot(t)
	if e := os.WriteFile(filepath.Join(root, "unlisted.bin"), []byte("external edit"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := a.WriteZIP(io.Discard, tree); e != nil {
		t.Fatal("frozen copy follows source edit", e)
	}
	for _, limits := range []Limits{{1, 100, 100}, {200, 100, 20}, {200, 19, 100}} {
		if a, _, e := SnapshotDirectory(root, limits); e == nil {
			a.Close()
			t.Fatal("snapshot limit ignored", limits)
		}
	}
}

func TestPrivateSnapshotWithAliasedSystemTemp(t *testing.T) {
	_, _, root := fixtureSnapshot(t)
	alias := filepath.Join(t.TempDir(), "system-temp")
	if e := os.Symlink(t.TempDir(), alias); e != nil {
		t.Fatal(e)
	}
	t.Setenv("TMPDIR", alias)
	a, tree, e := SnapshotDirectory(root, DefaultLimits)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if e = a.WriteZIP(io.Discard, tree); e != nil {
		t.Fatal("private system temp alias mistaken for publication link", e)
	}
	z, e := Open(filepath.Join(filepath.Dir(root), "source.epub"), DefaultLimits)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	if _, e = z.Inventory(); e != nil {
		t.Fatal("ZIP private temp alias", e)
	}
}
