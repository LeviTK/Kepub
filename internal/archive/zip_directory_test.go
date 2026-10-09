package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

func TestR2ZIPCollisionAtEntryBoundary(t *testing.T) {
	// Exercise the archive gate, not the standard library's opt-in path refusal.
	t.Setenv("GODEBUG", "zipinsecurepath=1")
	for _, tc := range []struct {
		name     string
		names    []string
		limit    int
		code     string
		badCount bool
	}{
		{"single", []string{"mimetype"}, 1, "", false},
		{"duplicate-with-room", []string{"mimetype", "mimetype"}, 2, "ARCHIVE_COLLISION", false},
		{"duplicate-at-limit", []string{"mimetype", "mimetype"}, 1, "ARCHIVE_COLLISION", false},
		{"unique-over-limit", []string{"mimetype", "other"}, 1, "ARCHIVE_LIMIT", false},
		{"unsafe-before-count-mismatch", []string{"../x"}, 2, "UNSAFE_PATH", true},
		{"unsafe-at-limit", []string{"mimetype", "../x"}, 1, "UNSAFE_PATH", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "book.epub")
			entries := []testfixture.Entry{}
			for _, name := range tc.names {
				entries = append(entries, testfixture.Entry{Name: name, Data: []byte("application/epub+zip")})
			}
			testfixture.ZIP(t, filename, entries)
			before, err := os.ReadFile(filename)
			if err != nil {
				t.Fatal(err)
			}
			if tc.badCount {
				// testfixture.ZIP emits a final 22-byte EOCD without a comment.
				end := before[len(before)-22:]
				binary.LittleEndian.PutUint16(end[10:], uint16(len(tc.names)+1))
				if err := os.WriteFile(filename, before, 0600); err != nil {
					t.Fatal(err)
				}
			}
			limits := DefaultLimits
			limits.Entries = tc.limit
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			a, err := Open(filename, limits)
			if tc.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				b, err := a.Read("mimetype", 20)
				a.Close()
				if err != nil || string(b) != "application/epub+zip" {
					t.Fatal("legal bytes", err)
				}
			} else {
				if a != nil {
					a.Close()
					t.Error("refusal returned archive")
				}
				var f *fault.Error
				if !errors.As(err, &f) || f.Code != tc.code || f.Exit != 1 {
					t.Fatalf("want %s, got %v", tc.code, err)
				}
			}
			after, err := os.ReadFile(filename)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("source changed", err)
			}
			left, err := os.ReadDir(tmp)
			if err != nil || len(left) != 0 {
				t.Fatal("staging leaked", left, err)
			}
		})
	}
}

func TestR2ZIPSmallBudgetAllocation(t *testing.T) {
	// The generator is outside measurement. This is cumulative allocation, NOT
	// RSS. A rejected archive must not allocate metadata for its unread suffix.
	measure := func(count int) uint64 {
		filename := filepath.Join(t.TempDir(), "many.zip")
		out, err := os.Create(filename)
		if err != nil {
			t.Fatal(err)
		}
		z := zip.NewWriter(out)
		for i := range count {
			if _, err := z.CreateHeader(&zip.FileHeader{Name: fmt.Sprintf("f%05d", i), Method: zip.Store}); err != nil {
				t.Fatal(err)
			}
		}
		if err := z.Close(); err != nil {
			t.Fatal(err)
		}
		if err := out.Close(); err != nil {
			t.Fatal(err)
		}
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		limits := DefaultLimits
		limits.Entries = 1
		a, err := Open(filename, limits)
		runtime.ReadMemStats(&after)
		if a != nil {
			a.Close()
			t.Fatal("over-budget archive accepted")
		}
		var f *fault.Error
		if !errors.As(err, &f) || f.Code != "ARCHIVE_LIMIT" {
			t.Fatal(err)
		}
		return after.TotalAlloc - before.TotalAlloc
	}
	small, large := measure(2), measure(12000)
	t.Logf("TotalAlloc bytes (not RSS): 2 entries=%d, 12000 entries=%d", small, large)
	if large > small+(256<<10) {
		t.Fatal("allocation scales with central-directory suffix before budget refusal")
	}
}

func TestR2PublishZIPRetainsEntryLimit(t *testing.T) {
	// The final ZIP is within an explicitly larger supported inventory budget.
	// Resetting only the final reopen to DefaultLimits must break this positive.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "mimetype"), []byte("application/epub+zip"), 0600); err != nil {
		t.Fatal(err)
	}
	for i := range DefaultLimits.Entries {
		if err := os.Mkdir(filepath.Join(root, fmt.Sprintf("d%05d", i)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	limits := DefaultLimits
	limits.Entries++
	a, tree, err := SnapshotDirectory(root, limits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	output := filepath.Join(t.TempDir(), "published.epub")
	checked := false
	_, err = a.PublishZIP(context.Background(), output, tree, func(name, _ string) error {
		z, err := zip.OpenReader(name)
		if err != nil {
			return err
		}
		defer z.Close()
		if len(z.File) != 20001 {
			return fmt.Errorf("wrong explicit inventory: %d", len(z.File))
		}
		seen := map[string]bool{}
		for _, f := range z.File {
			if seen[f.Name] {
				return fmt.Errorf("duplicate %q", f.Name)
			}
			seen[f.Name] = true
			r, err := f.Open()
			if err != nil {
				return err
			}
			data, err := io.ReadAll(r)
			r.Close()
			if err != nil {
				return err
			}
			if f.Name == "mimetype" {
				if !f.Mode().IsRegular() || string(data) != "application/epub+zip" {
					return fmt.Errorf("wrong mimetype bytes/type")
				}
			} else if !f.Mode().IsDir() || len(data) != 0 {
				return fmt.Errorf("wrong directory bytes/type: %q", f.Name)
			}
		}
		if !seen["mimetype"] {
			return fmt.Errorf("missing mimetype")
		}
		for i := range 20000 {
			if !seen[fmt.Sprintf("d%05d/", i)] {
				return fmt.Errorf("missing directory %d", i)
			}
		}
		checked = true
		return nil
	})
	if err != nil || !checked {
		t.Fatal("final reopen discarded source limits", err)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatal(err)
	}
}

func TestR2ZIPFramingCompatibility(t *testing.T) {
	// Hand-written little-endian records keep expected payload and offsets
	// independent of the production directory parser and zip.Writer.
	for _, tc := range []struct {
		name                        string
		base                        int64
		zip64, gap, corrupt         bool
		count                       uint64
		comment                     int
		badComment, truncatedRecord bool
		code                        string
	}{
		{name: "ordinary", count: 1},
		{name: "prefix", base: 100, count: 1},
		{name: "negative-base", base: -100, count: 1},
		{name: "zip64", zip64: true, count: 1},
		{name: "zip64-negative-base", base: -(1 << 32), zip64: true, count: 1},
		{name: "zip64-modulo-count", zip64: true, count: 65537},
		{name: "directory-gap-fallback", gap: true, count: 1},
		{name: "crc-failure", corrupt: true, count: 1, code: "INVALID_ZIP"},
		{name: "count-mismatch", count: 2, code: "INVALID_ZIP"},
		{name: "declared-record-truncated", count: 2, truncatedRecord: true, code: "INVALID_ZIP"},
		{name: "long-eocd-comment", count: 1, comment: 2048},
		{name: "eocd-comment-length-overrun", count: 1, comment: 2048, badComment: true, code: "INVALID_ZIP"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p16, p32, p64 := binary.LittleEndian.PutUint16, binary.LittleEndian.PutUint32, binary.LittleEndian.PutUint64
			name, data := []byte("mimetype"), []byte("application/epub+zip")
			crc := crc32.ChecksumIEEE(data)
			local := make([]byte, 30)
			p32(local, 0x04034b50)
			p16(local[4:], 20)
			p32(local[14:], crc)
			p32(local[18:], uint32(len(data)))
			p32(local[22:], uint32(len(data)))
			p16(local[26:], uint16(len(name)))
			body := append(append(local, name...), data...)
			central := make([]byte, 46)
			p32(central, 0x02014b50)
			p16(central[4:], 20)
			p16(central[6:], 20)
			p32(central[16:], crc)
			p32(central[20:], uint32(len(data)))
			p32(central[24:], uint32(len(data)))
			p16(central[28:], uint16(len(name)))
			localOffset := uint64(max(-tc.base, 0))
			p32(central[42:], uint32(localOffset))
			if tc.zip64 {
				p32(central[42:], 0xffffffff)
				p16(central[30:], 12)
			}
			central = append(central, name...)
			if tc.zip64 {
				extra := make([]byte, 12)
				p16(extra, 1)
				p16(extra[2:], 8)
				p64(extra[4:], localOffset)
				central = append(central, extra...)
			}
			prefix := make([]byte, int(max(tc.base, 0)))
			buf := append(prefix, body...)
			offset := uint64(int64(len(buf)) - tc.base)
			buf = append(buf, central...)
			centralSize := len(central)
			if tc.truncatedRecord {
				// The declared second record has only 20 header bytes. Even with
				// the following EOCD, fewer than the required 46 bytes remain.
				buf = append(buf, central[:20]...)
				centralSize += 20
			}
			if tc.gap {
				buf = append(buf, make([]byte, 100)...)
			}
			end := make([]byte, 22)
			p32(end, 0x06054b50)
			p16(end[8:], uint16(tc.count))
			p16(end[10:], uint16(tc.count))
			p32(end[12:], uint32(centralSize))
			p32(end[16:], uint32(offset))
			p16(end[20:], uint16(tc.comment))
			if tc.badComment {
				p16(end[20:], uint16(tc.comment+1))
			}
			if tc.zip64 {
				z := make([]byte, 76)
				p32(z, 0x06064b50)
				p64(z[4:], 44)
				p64(z[24:], tc.count)
				p64(z[32:], tc.count)
				p64(z[40:], uint64(len(central)))
				p64(z[48:], offset)
				p32(z[56:], 0x07064b50)
				p64(z[64:], uint64(len(buf)))
				p32(z[72:], 1)
				buf = append(buf, z...)
				p16(end[10:], 0xffff)
				p32(end[16:], 0xffffffff)
			}
			buf = append(buf, end...)
			buf = append(buf, bytes.Repeat([]byte("c"), tc.comment)...)
			if tc.corrupt {
				buf[len(prefix)+30+len(name)] ^= 1
			}
			filename := filepath.Join(t.TempDir(), "book.zip")
			if err := os.WriteFile(filename, buf, 0600); err != nil {
				t.Fatal(err)
			}
			limits := DefaultLimits
			limits.Entries = 1
			limits.PathBytes = 8
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			a, err := Open(filename, limits)
			if tc.code != "" {
				if a != nil {
					a.Close()
					t.Fatal("invalid archive accepted")
				}
				var f *fault.Error
				if !errors.As(err, &f) || f.Code != tc.code || f.Exit != 1 {
					t.Fatalf("want %s/1, got %v", tc.code, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				defer a.Close()
				actual, err := a.Read("mimetype", 20)
				if err != nil || !bytes.Equal(actual, data) || len(a.Files) != 1 {
					t.Fatalf("payload/inventory mismatch: %q %v", actual, err)
				}
				a.Close()
			}
			after, err := os.ReadFile(filename)
			if err != nil || !bytes.Equal(after, buf) {
				t.Fatal("source changed", err)
			}
			left, err := os.ReadDir(tmp)
			if err != nil || len(left) != 0 {
				t.Fatal("staging leaked", left, err)
			}
		})
	}
}
