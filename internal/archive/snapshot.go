package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"syscall"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
)

// Entry and Tree use the workspace kepub-tree-v1 hash contract without depending
// on workspace. All directories, including empty ones, are explicit entries.
type Entry struct {
	Path   string `json:"path"`
	Type   string `json:"type"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
}
type Tree struct {
	SHA256  string  `json:"sha256"`
	Entries []Entry `json:"entries"`
}

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func scan(dir, copyTo string, limits Limits) (Tree, error) {
	t := Tree{Entries: []Entry{}}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return t, err
	}
	for p := abs; ; p = filepath.Dir(p) {
		i, e := os.Lstat(p)
		if e != nil {
			return t, e
		}
		if !i.IsDir() {
			return t, fault.New(1, "UNSAFE_ENTRY", "directory path contains link or non-directory")
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	r, err := os.OpenRoot(abs)
	if err != nil {
		return t, err
	}
	defer r.Close()
	names := map[string]string{}
	var total, pathBytes int64
	err = fs.WalkDir(r.FS(), ".", func(name string, d fs.DirEntry, e error) error {
		if e != nil || name == "." {
			return e
		}
		bp, e := bookpath.Parse(name)
		if e != nil {
			return e
		}
		key := bookpath.CollisionKey(bp)
		if old, ok := names[key]; ok && old != name {
			return fault.New(1, "ARCHIVE_COLLISION", "colliding directory paths")
		}
		names[key] = name
		i, e := r.Lstat(name)
		if e != nil {
			return e
		}
		if e := limits.checkEntry(len(t.Entries), pathBytes, bp); e != nil {
			return e
		}
		pathBytes += int64(len(bp))
		entry := Entry{Path: name}
		if i.IsDir() {
			entry.Type = "directory"
			if copyTo != "" {
				if e = os.Mkdir(filepath.Join(copyTo, filepath.FromSlash(name)), 0700); e != nil {
					return e
				}
			}
		} else {
			if !i.Mode().IsRegular() {
				return fault.New(1, "UNSAFE_ENTRY", "special or linked directory entry %q", name)
			}
			if st, ok := i.Sys().(*syscall.Stat_t); ok && st.Nlink != 1 {
				return fault.New(1, "UNSAFE_ENTRY", "hard linked entry %q", name)
			}
			// No-follow/nonblocking also prevents a raced FIFO or symlink from
			// turning a bounded snapshot into an unbounded wait or foreign read.
			f, e := r.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
			if e != nil {
				return e
			}
			defer f.Close()
			opened, e := f.Stat()
			if e != nil {
				return e
			}
			if !os.SameFile(i, opened) || !opened.Mode().IsRegular() {
				return fault.New(1, "INPUT_DRIFT", "entry changed during snapshot")
			}
			h := sha256.New()
			var w io.Writer = h
			var out *os.File
			if copyTo != "" {
				out, e = os.OpenFile(filepath.Join(copyTo, filepath.FromSlash(name)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if e != nil {
					return e
				}
				w = io.MultiWriter(h, out)
			}
			n, e := io.Copy(w, io.LimitReader(f, min(limits.FileBytes, limits.TotalBytes-total)+1))
			if out != nil {
				ce := out.Close()
				if e == nil {
					e = ce
				}
			}
			if e != nil {
				return e
			}
			if n > limits.FileBytes || n > limits.TotalBytes-total {
				return fault.New(1, "ARCHIVE_LIMIT", "actual directory bytes exceed limit")
			}
			after, e := f.Stat()
			if e != nil {
				return e
			}
			if n != opened.Size() || after.Size() != opened.Size() || !after.ModTime().Equal(opened.ModTime()) {
				return fault.New(1, "INPUT_DRIFT", "entry changed during snapshot")
			}
			entry.Type = "file"
			entry.Size = n
			entry.SHA256 = hex.EncodeToString(h.Sum(nil))
			total += n
		}
		t.Entries = append(t.Entries, entry)
		return nil
	})
	if err != nil {
		return t, err
	}
	sort.Slice(t.Entries, func(i, j int) bool { return t.Entries[i].Path < t.Entries[j].Path })
	b, _ := json.Marshal(t.Entries)
	t.SHA256 = digest(append([]byte("kepub-tree-v1\n"), b...))
	return t, nil
}

// SnapshotDirectory copies only the explicitly supplied publication root into
// private storage. Callers must stop writers; this is not an OS writer sandbox.
func SnapshotDirectory(dir string, limits Limits) (*Archive, Tree, error) {
	stage, err := privateDir("kepub-snapshot-")
	if err != nil {
		return nil, Tree{}, err
	}
	a := &Archive{dir: stage, limits: limits, Files: map[bookpath.BookPath]int64{}}
	t, err := scan(dir, stage, limits)
	if err == nil {
		var again Tree
		again, err = scan(dir, "", limits)
		if err == nil && again.SHA256 != t.SHA256 {
			err = fault.New(1, "INPUT_DRIFT", "directory changed during snapshot")
		}
	}
	if err != nil {
		a.Close()
		return nil, Tree{}, err
	}
	for _, entry := range t.Entries {
		if entry.Type == "file" {
			a.Files[bookpath.BookPath(entry.Path)] = entry.Size
		}
	}
	b, err := a.Read("mimetype", 64)
	if err == nil && !bytes.Equal(b, []byte("application/epub+zip")) {
		err = fault.New(1, "INVALID_EPUB", "invalid mimetype")
	}
	if err != nil {
		a.Close()
		return nil, Tree{}, err
	}
	return a, t, nil
}

func (a *Archive) Inventory() (Tree, error) { return scan(a.dir, "", a.limits) }

// WriteZIP consumes an approved, hash-bound inventory, never a workspace walk.
// It writes an unpublished artifact; applications must check it before PublishZIP.
func (a *Archive) WriteZIP(w io.Writer, approved Tree) error {
	actual, err := a.Inventory()
	if err != nil {
		return err
	}
	b, _ := json.Marshal(actual)
	want, _ := json.Marshal(approved)
	if !bytes.Equal(b, want) {
		return fault.New(1, "INPUT_DRIFT", "approved publication inventory differs from snapshot")
	}
	z := zip.NewWriter(w)
	defer z.Close()
	mime, err := a.Read("mimetype", 64)
	if err != nil {
		return err
	}
	if !bytes.Equal(mime, []byte("application/epub+zip")) {
		return fault.New(1, "INVALID_EPUB", "invalid mimetype")
	}
	// CreateRaw avoids data descriptors and any library-added timestamp extras.
	h := &zip.FileHeader{Name: "mimetype", Method: zip.Store, CRC32: crc32.ChecksumIEEE(mime), CompressedSize64: uint64(len(mime)), UncompressedSize64: uint64(len(mime))}
	m, err := z.CreateRaw(h)
	if err != nil {
		return err
	}
	if _, err = m.Write(mime); err != nil {
		return err
	}
	for _, e := range approved.Entries {
		if e.Path == "mimetype" {
			continue
		}
		name := e.Path
		method := uint16(zip.Deflate)
		if e.Type == "directory" {
			name += "/"
			method = zip.Store
		}
		h := &zip.FileHeader{Name: name, Method: method}
		h.SetMode(0600)
		if e.Type == "directory" {
			h.SetMode(os.ModeDir | 0700)
		}
		out, err := z.CreateHeader(h)
		if err != nil {
			return err
		}
		if e.Type == "file" {
			f, err := os.Open(filepath.Join(a.dir, filepath.FromSlash(e.Path)))
			if err != nil {
				return err
			}
			hash := sha256.New()
			n, err := io.Copy(io.MultiWriter(out, hash), io.LimitReader(f, e.Size+1))
			f.Close()
			if err != nil {
				return err
			}
			if n != e.Size || hex.EncodeToString(hash.Sum(nil)) != e.SHA256 {
				return fault.New(1, "INPUT_DRIFT", "snapshot resource changed")
			}
		}
	}
	return z.Close()
}

func FileSHA256(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	_, err = io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), err
}

// PublishZIP creates private staging in the output directory, checks final ZIP
// entries and bytes, calls check, rehashes, then atomically publishes no-replace.
// A failed check never publishes. check must not modify the artifact. The last
// ctx check is immediately before atomic publication, which is the commit point.
// Cancellation after that observation cannot turn a committed output into error.
func (a *Archive) PublishZIP(ctx context.Context, output string, approved Tree, check func(string, string) error) (string, error) {
	abs, err := filepath.Abs(output)
	if err != nil {
		return "", err
	}
	for p := filepath.Dir(abs); ; p = filepath.Dir(p) {
		i, e := os.Lstat(p)
		if e != nil {
			return "", e
		}
		if !i.IsDir() {
			return "", fault.New(2, "INVALID_OUTPUT", "output parent must not contain symlinks")
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	if _, err = os.Lstat(abs); err == nil {
		return "", fault.New(2, "OUTPUT_EXISTS", "output already exists")
	} else if !os.IsNotExist(err) {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(abs), ".kepub-pack-*.epub")
	if err != nil {
		return "", err
	}
	stage := f.Name()
	defer os.Remove(stage)
	err = a.WriteZIP(f, approved)
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		return "", err
	}
	final, err := Open(stage, a.limits)
	if err != nil {
		return "", err
	}
	t, err := final.Inventory()
	final.Close()
	if err != nil {
		return "", err
	}
	if t.SHA256 != approved.SHA256 {
		return "", fault.New(1, "INPUT_DRIFT", "final archive differs from approved tree")
	}
	hash, err := FileSHA256(stage)
	if err != nil {
		return "", err
	}
	if check == nil {
		return "", fmt.Errorf("final archive check is required")
	}
	if err = check(stage, hash); err != nil {
		return "", err
	}
	again, err := FileSHA256(stage)
	if err != nil {
		return "", err
	}
	if again != hash {
		return "", fault.New(1, "INPUT_DRIFT", "archive changed during final check")
	}
	if err = ctx.Err(); err != nil {
		return "", fault.New(5, "PUBLICATION_CANCELLED", "publication cancelled before commit: %v", err)
	}
	if err = publish(stage, abs); err != nil {
		if os.IsExist(err) {
			return "", fault.New(2, "OUTPUT_EXISTS", "output already exists")
		}
		return "", err
	}
	return hash, nil
}
