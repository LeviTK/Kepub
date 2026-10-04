package archive

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

func TestRejectMalicious(t *testing.T) {
	cases := []struct {
		name    string
		entries []testfixture.Entry
		code    string
	}{
		{"parent", []testfixture.Entry{{Name: "../escape"}}, "UNSAFE_PATH"},
		{"absolute", []testfixture.Entry{{Name: "/escape"}}, "UNSAFE_PATH"},
		{"windows", []testfixture.Entry{{Name: "C:\\escape"}}, "UNSAFE_PATH"},
		{"duplicate", []testfixture.Entry{{Name: "x"}, {Name: "x"}}, "ARCHIVE_COLLISION"},
		{"case parent", []testfixture.Entry{{Name: "Text/a"}, {Name: "text/b"}}, "ARCHIVE_COLLISION"},
		{"unicode parent", []testfixture.Entry{{Name: "é/a"}, {Name: "e\u0301/b"}}, "ARCHIVE_COLLISION"},
		{"dir/file", []testfixture.Entry{{Name: "x"}, {Name: "x/y"}}, "ARCHIVE_COLLISION"},
		{"symlink", []testfixture.Entry{{Name: "link", Data: []byte("../../escape"), Mode: os.ModeSymlink | 0777}}, "UNSAFE_ENTRY"},
		{"fifo", []testfixture.Entry{{Name: "fifo", Mode: os.ModeNamedPipe | 0600}}, "UNSAFE_ENTRY"},
		{"dir data", []testfixture.Entry{{Name: "dirx", Data: []byte("bad")}}, "UNSAFE_ENTRY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "evil.epub")
			testfixture.ZIP(t, filename, tc.entries)
			if tc.name == "dir data" {
				b, e := os.ReadFile(filename)
				if e != nil {
					t.Fatal(e)
				}
				b = bytes.ReplaceAll(b, []byte("dirx"), []byte("dir/"))
				if e = os.WriteFile(filename, b, 0600); e != nil {
					t.Fatal(e)
				}
			}
			a, e := Open(filename, DefaultLimits)
			if a != nil {
				a.Close()
			}
			var fe *fault.Error
			if !errors.As(e, &fe) || fe.Code != tc.code {
				t.Fatalf("want %s got %v", tc.code, e)
			}
		})
	}
}

func TestActualLimitsAndCRC(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "limits.epub")
	entries := []testfixture.Entry{{Name: "mimetype", Data: []byte("application/epub+zip")}, {Name: "bomb", Data: []byte(strings.Repeat("x", 1025))}}
	testfixture.ZIP(t, filename, entries)
	for _, limit := range []Limits{{2, 1024, 2048}, {2, 2048, 1044}, {1, 2048, 2048}} {
		if a, e := Open(filename, limit); e == nil {
			a.Close()
			t.Fatal("limit ignored", limit)
		}
	}
	a, e := Open(filename, Limits{2, 1025, 1045})
	if e != nil {
		t.Fatal("exact boundary", e)
	}
	a.Close()
	// Corrupt actual STORE bytes, rather than merely asserting the header is read.
	b, e := os.ReadFile(filename)
	if e != nil {
		t.Fatal(e)
	}
	idx := bytes.Index(b, []byte("application/epub+zip"))
	b[idx] = 'X'
	if e = os.WriteFile(filename, b, 0600); e != nil {
		t.Fatal(e)
	}
	if a, e := Open(filename, DefaultLimits); e == nil {
		a.Close()
		t.Fatal("CRC ignored")
	}
}

func TestForgedSizeAndZIPEncryption(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		t.Run(map[bool]string{false: "forged size", true: "encrypted"}[encrypted], func(t *testing.T) {
			f := filepath.Join(t.TempDir(), "forged.epub")
			testfixture.ZIP(t, f, []testfixture.Entry{{Name: "mimetype", Data: []byte("application/epub+zip")}})
			b, e := os.ReadFile(f)
			if e != nil {
				t.Fatal(e)
			}
			pos := bytes.Index(b, []byte{'P', 'K', 1, 2})
			if pos < 0 {
				t.Fatal("no central header")
			}
			want := "INVALID_ZIP"
			if encrypted {
				binary.LittleEndian.PutUint16(b[pos+8:], binary.LittleEndian.Uint16(b[pos+8:])|1)
				want = "UNSAFE_ENTRY"
			} else {
				binary.LittleEndian.PutUint32(b[pos+24:], 1)
			}
			if e = os.WriteFile(f, b, 0600); e != nil {
				t.Fatal(e)
			}
			a, e := Open(f, DefaultLimits)
			if a != nil {
				a.Close()
			}
			var fe *fault.Error
			if !errors.As(e, &fe) || fe.Code != want {
				t.Fatalf("want %s got %v", want, e)
			}
		})
	}
}

func TestUnpackPreservesBytesNoOverwriteAndConcurrent(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "book.epub")
	entries := testfixture.EPUB("3.0", false)
	testfixture.ZIP(t, filename, entries)
	original, _ := os.ReadFile(filename)
	a, e := Open(filename, DefaultLimits)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	dest := filepath.Join(dir, "新 目录")
	if e = a.Unpack(dest); e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name, "/") {
			if st, e := os.Stat(filepath.Join(dest, entry.Name)); e != nil || !st.IsDir() {
				t.Fatal("empty directory lost", e)
			}
			continue
		}
		b, e := os.ReadFile(filepath.Join(dest, entry.Name))
		if e != nil || !bytes.Equal(b, entry.Data) {
			t.Fatalf("bytes changed %s: %v", entry.Name, e)
		}
	}
	if e = a.Unpack(dest); e == nil {
		t.Fatal("overwrote output")
	}
	if e = a.Unpack(filename); e == nil {
		t.Fatal("overwrote input")
	}
	after, _ := os.ReadFile(filename)
	if !bytes.Equal(after, original) {
		t.Fatal("changed input archive")
	}
	// A staging read failure must not create output or leak publish staging.
	if e = os.Remove(filepath.Join(a.dir, "unlisted.bin")); e != nil {
		t.Fatal(e)
	}
	// Walk skips deleted entries, so force a real read failure with a dangling link.
	if e = os.Symlink("missing", filepath.Join(a.dir, "unlisted.bin")); e != nil {
		t.Fatal(e)
	}
	failed := filepath.Join(dir, "failed")
	if e = a.Unpack(failed); e == nil {
		t.Fatal("expected staging failure")
	}
	if _, e = os.Lstat(failed); !os.IsNotExist(e) {
		t.Fatal("partial output", e)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, ".kepub-unpack-*"))
	if len(matches) != 0 {
		t.Fatal("leaked staging", matches)
	}
	if e = os.Remove(filepath.Join(a.dir, "unlisted.bin")); e != nil {
		t.Fatal(e)
	}
	concurrent := filepath.Join(dir, "concurrent")
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- a.Unpack(concurrent) }()
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
		t.Fatalf("successes %d", success)
	}
	if _, e = a.Read(bookpath.BookPath("../escape"), 64); e == nil {
		t.Fatal("read outside manifest")
	}
}
