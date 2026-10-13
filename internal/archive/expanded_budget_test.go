package archive

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

func TestR2ExpandedEntriesBeforeStaging(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "deep.epub")
	entries := []testfixture.Entry{
		{Name: "mimetype", Data: []byte("application/epub+zip")},
		{Name: "a/b/c/d/file", Data: []byte("x")},
	}
	testfixture.ZIP(t, filename, entries)
	original, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	limits := DefaultLimits
	limits.Entries = 6 // two files and four unique implicit directories
	a, err := Open(filename, limits)
	if err != nil {
		t.Fatal("exact expanded boundary", err)
	}
	tree, err := a.Inventory()
	a.Close()
	if err != nil || len(tree.Entries) != 6 {
		t.Fatalf("boundary inventory: %+v, %v", tree, err)
	}
	limits.Entries = 5
	for _, blockedTemp := range []bool{false, true} {
		t.Run(map[bool]string{false: "no published archive", true: "before temp creation"}[blockedTemp], func(t *testing.T) {
			tmp := t.TempDir()
			if blockedTemp {
				tmp = filepath.Join(tmp, "not-a-directory")
				if err := os.WriteFile(tmp, []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("TMPDIR", tmp)
			a, err := Open(filename, limits)
			if a != nil {
				a.Close()
			}
			var f *fault.Error
			if a != nil || !errors.As(err, &f) || f.Code != "ARCHIVE_LIMIT" || f.Exit != 1 {
				t.Fatalf("expanded limit must reject before staging: archive=%v, error=%v", a != nil, err)
			}
			if !blockedTemp {
				remaining, err := os.ReadDir(tmp)
				if err != nil || len(remaining) != 0 {
					t.Fatal("private staging leaked", remaining, err)
				}
			}
		})
	}
	after, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatal("original archive changed", err)
	}
}

func TestR2InventoryRetainsEntryBudget(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "small.epub")
	testfixture.ZIP(t, filename, []testfixture.Entry{{Name: "mimetype", Data: []byte("application/epub+zip")}})
	limits := DefaultLimits
	limits.Entries = 1
	a, err := Open(filename, limits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if _, err := a.Inventory(); err != nil {
		t.Fatal("exact entry boundary", err)
	}
	// Owner-side alteration of the private snapshot must not reset its budget.
	if err := os.Mkdir(filepath.Join(a.dir, "empty"), 0700); err != nil {
		t.Fatal(err)
	}
	var f *fault.Error
	if _, err := a.Inventory(); !errors.As(err, &f) || f.Code != "ARCHIVE_LIMIT" {
		t.Fatalf("Inventory discarded injected budget: %v", err)
	}
	output := filepath.Join(dir, "unpacked")
	if err := a.Unpack(output); !errors.As(err, &f) || f.Code != "ARCHIVE_LIMIT" {
		t.Fatalf("Unpack discarded injected budget: %v", err)
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatal("over-budget unpack published", err)
	}
	if remaining, err := filepath.Glob(filepath.Join(dir, ".kepub-unpack-*")); err != nil || len(remaining) != 0 {
		t.Fatal("unpack staging leaked", remaining, err)
	}
}

func TestR2CumulativePathBytes(t *testing.T) {
	entries := []testfixture.Entry{
		{Name: "mimetype", Data: []byte("application/epub+zip")},
		{Name: "é/a", Data: []byte("a")},
		{Name: "é/b", Data: []byte("b")},
		{Name: "empty/", Mode: os.ModeDir | 0700},
	}
	// Unique BookPaths are mimetype, é, é/a, é/b, empty: 8+2+4+4+5 UTF-8 bytes.
	for _, explicitFirst := range []bool{false, true} {
		mixed := append([]testfixture.Entry{}, entries...)
		parent := testfixture.Entry{Name: "é/", Mode: os.ModeDir | 0700}
		if explicitFirst {
			mixed = append([]testfixture.Entry{parent}, mixed...)
		} else {
			mixed = append(mixed, parent)
		}
		for _, budget := range []int64{22, 23, 24} {
			t.Run(fmt.Sprintf("explicit-first=%t/budget=%d", explicitFirst, budget), func(t *testing.T) {
				dir, tmp := t.TempDir(), t.TempDir()
				t.Setenv("TMPDIR", tmp)
				filename := filepath.Join(dir, "paths.epub")
				testfixture.ZIP(t, filename, mixed)
				root := filepath.Join(dir, "publication")
				for _, entry := range mixed {
					p := filepath.Join(root, entry.Name)
					if strings.HasSuffix(entry.Name, "/") {
						if err := os.MkdirAll(p, 0700); err != nil {
							t.Fatal(err)
						}
					} else {
						if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(p, entry.Data, 0600); err != nil {
							t.Fatal(err)
						}
					}
				}
				limits := DefaultLimits
				limits.Entries, limits.PathBytes = 5, budget
				for _, directory := range []bool{false, true} {
					var a *Archive
					var err error
					if directory {
						a, _, err = SnapshotDirectory(root, limits)
					} else {
						a, err = Open(filename, limits)
					}
					if budget == 22 {
						var f *fault.Error
						if a != nil || !errors.As(err, &f) || f.Code != "ARCHIVE_LIMIT" || !strings.Contains(f.Message, "path bytes") {
							t.Fatalf("directory=%t: path budget not enforced: %v", directory, err)
						}
					} else {
						if err != nil {
							t.Fatalf("directory=%t: exact/under budget rejected: %v", directory, err)
						}
						inventory, err := a.Inventory()
						a.Close()
						if err != nil || len(inventory.Entries) != 5 {
							t.Fatal("shared/explicit directory counted twice", inventory, err)
						}
					}
				}
				if remaining, err := os.ReadDir(tmp); err != nil || len(remaining) != 0 {
					t.Fatal("private staging leaked", remaining, err)
				}
			})
		}
	}
}

func TestR2IndependentBudgets(t *testing.T) {
	dir, tmp := t.TempDir(), t.TempDir()
	t.Setenv("TMPDIR", tmp)
	filename := filepath.Join(dir, "budgets.epub")
	testfixture.ZIP(t, filename, []testfixture.Entry{
		{Name: "mimetype", Data: []byte("application/epub+zip")},
		{Name: "a/b/c/d/file", Data: bytes.Repeat([]byte("x"), 25)},
	})
	for _, tc := range []struct {
		name, message string
		limits        Limits
	}{
		{"entries", "expanded entries", Limits{5, 25, 45, 36}},
		{"paths", "path bytes", Limits{6, 25, 45, 35}},
		{"file", "expanded bytes", Limits{6, 24, 45, 36}},
		{"total", "expanded bytes", Limits{6, 25, 44, 36}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, err := Open(filename, tc.limits)
			if a != nil {
				a.Close()
			}
			var f *fault.Error
			if a != nil || !errors.As(err, &f) || f.Code != "ARCHIVE_LIMIT" || !strings.Contains(f.Message, tc.message) {
				t.Fatalf("wrong budget diagnostic: %v", err)
			}
			if remaining, err := os.ReadDir(tmp); err != nil || len(remaining) != 0 {
				t.Fatal("staging leaked", remaining, err)
			}
		})
	}
	t.Run("empty directories", func(t *testing.T) {
		filename := filepath.Join(dir, "directories.epub")
		testfixture.ZIP(t, filename, []testfixture.Entry{
			{Name: "mimetype", Data: []byte("application/epub+zip")},
			{Name: "a/b/c/d/", Mode: os.ModeDir | 0700},
		})
		limits := Limits{5, 20, 20, 24} // one file, four directories, 8+1+3+5+7 path bytes
		a, err := Open(filename, limits)
		if err != nil {
			t.Fatal("empty directory boundary", err)
		}
		tree, err := a.Inventory()
		a.Close()
		if err != nil || len(tree.Entries) != 5 {
			t.Fatal("empty implicit directories lost", tree, err)
		}
		limits.Entries = 4
		a, err = Open(filename, limits)
		if a != nil {
			a.Close()
		}
		var f *fault.Error
		if a != nil || !errors.As(err, &f) || f.Code != "ARCHIVE_LIMIT" {
			t.Fatalf("empty directories escaped expanded budget: %v", err)
		}
	})
	// The subtraction guard must reject, rather than wrap an overflowing sum.
	limits := Limits{1, 25, 45, math.MaxInt64}
	if err := limits.CheckEntry(0, math.MaxInt64-2, "xx"); err != nil {
		t.Fatal("exact int64 boundary", err)
	}
	if err := limits.CheckEntry(0, math.MaxInt64-1, "xx"); err == nil {
		t.Fatal("cumulative path sum overflow accepted")
	}
}

func TestR2ExpandedRoundTrip(t *testing.T) {
	entries := testfixture.EPUB("3.0", false)
	expected := []Entry{{Path: "META-INF", Type: "directory"}, {Path: "书", Type: "directory"}, {Path: "书/Deep", Type: "directory"}, {Path: "书/Text", Type: "directory"}}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name, "/") {
			expected = append(expected, Entry{Path: strings.TrimSuffix(entry.Name, "/"), Type: "directory"})
		} else {
			h := sha256.Sum256(entry.Data)
			expected = append(expected, Entry{Path: entry.Name, Type: "file", Size: int64(len(entry.Data)), SHA256: hex.EncodeToString(h[:])})
		}
	}
	sort.Slice(expected, func(i, j int) bool { return expected[i].Path < expected[j].Path })
	encoded, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(append([]byte("kepub-tree-v1\n"), encoded...))
	want := Tree{Entries: expected, SHA256: hex.EncodeToString(h[:])}
	limits := DefaultLimits
	limits.Entries = 12
	limits.PathBytes = 0
	for _, entry := range expected {
		limits.PathBytes += int64(len(entry.Path))
	}
	filename := filepath.Join(t.TempDir(), "source.epub")
	testfixture.ZIP(t, filename, entries)
	original, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	a, err := Open(filename, limits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	root := filepath.Join(t.TempDir(), "unpacked")
	if err := a.Unpack(root); err != nil {
		t.Fatal(err)
	}
	snapshot, tree, err := SnapshotDirectory(root, limits)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	inventory, err := snapshot.Inventory()
	if err != nil || !reflect.DeepEqual(tree, want) || !reflect.DeepEqual(inventory, want) {
		t.Fatalf("expanded inventory/hash changed: tree=%+v, inventory=%+v, want=%+v, err=%v", tree, inventory, want, err)
	}
	var packed bytes.Buffer
	if err := snapshot.WriteZIP(&packed, want); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(packed.Bytes()), int64(packed.Len()))
	if err != nil || len(z.File) != 12 {
		t.Fatal("packed expanded count", z, err)
	}
	packedEntries := []Entry{}
	for _, f := range z.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		actual, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatal(err)
		}
		entry := Entry{Path: strings.TrimSuffix(f.Name, "/")}
		if f.FileInfo().IsDir() {
			entry.Type = "directory"
			if len(actual) != 0 {
				t.Fatal("directory has data", f.Name)
			}
		} else {
			h := sha256.Sum256(actual)
			entry.Type, entry.Size, entry.SHA256 = "file", int64(len(actual)), hex.EncodeToString(h[:])
		}
		packedEntries = append(packedEntries, entry)
	}
	sort.Slice(packedEntries, func(i, j int) bool { return packedEntries[i].Path < packedEntries[j].Path })
	if !reflect.DeepEqual(packedEntries, expected) {
		t.Fatalf("packed paths/types/bytes differ: got %+v, want %+v", packedEntries, expected)
	}
	if err := a.Unpack(root); err == nil {
		t.Fatal("existing output overwritten")
	}
	after, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatal("original archive changed", err)
	}
}
