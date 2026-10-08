package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
)

// Private per-call/workspace policy; publication limits are archive's exact
// model, while originalBytes bounds the distinct raw ZIP stream.
type resourceIO struct {
	ctx           context.Context
	limits        archive.Limits
	originalBytes int64
	// Private call-local observation; never a cache or a source-verification gate.
	onRevisionRead func(string)
	onOriginalRead func()
}

func defaultResourceIO(ctx context.Context) resourceIO {
	if ctx == nil {
		ctx = context.Background()
	}
	return resourceIO{ctx: ctx, limits: archive.DefaultLimits, originalBytes: archive.MaxInputBytes}
}

// Entry records exact, case-sensitive POSIX paths. Directory sizes are zero;
// file sizes are bytes read, not metadata estimates. Modes and times are absent.
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

// HashTree hashes all regular files and directories, including empty directories
// and unlisted publication resources. Symlinks, hard-linked files, special files,
// noncanonical paths and case/Unicode collisions are refused. Callers must stop
// external writers first: a filesystem walk is not an atomic snapshot.
func HashTree(dir string) (Tree, error) {
	r, err := openDir(dir)
	if err != nil {
		return Tree{}, err
	}
	defer r.Close()
	return scanTree(r, nil, defaultResourceIO(nil))
}

// scanTree can also copy into a newly created, empty root. No link-based cloning.
func scanTree(src, dst *os.Root, policy resourceIO) (Tree, error) {
	tree := Tree{Entries: []Entry{}}
	names := map[string]string{}
	var total, pathBytes int64
	err := archive.WalkDirectory(policy.ctx, src, func(name string, info os.FileInfo) error {
		bp, err := bookpath.Parse(name)
		if err != nil {
			return err
		}
		if err := policy.limits.CheckEntry(len(tree.Entries), pathBytes, bp); err != nil {
			return err
		}
		pathBytes += int64(len(bp))
		key := bookpath.CollisionKey(bp)
		if old, ok := names[key]; ok && old != name {
			return fmt.Errorf("colliding paths %q and %q", old, name)
		}
		names[key] = name
		entry := Entry{Path: name}
		switch {
		case info.IsDir():
			entry.Type = "directory"
			if dst != nil {
				if err := dst.Mkdir(name, 0700); err != nil {
					return err
				}
			}
		case info.Mode().IsRegular():
			entry.Type = "file"
			f, err := openRegular(src, name)
			if err != nil {
				return err
			}
			opened, err := f.Stat()
			remaining := min(policy.limits.FileBytes, policy.limits.TotalBytes-total)
			if err == nil && opened.Size() > remaining {
				err = fault.New(1, "ARCHIVE_LIMIT", "resource %q exceeds remaining byte budget", name)
			}
			if err != nil {
				return errors.Join(err, f.Close())
			}
			h := sha256.New()
			var out *os.File
			var writer io.Writer = h
			if dst != nil {
				out, err = dst.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
				if err != nil {
					f.Close()
					return err
				}
				writer = io.MultiWriter(h, out)
			}
			entry.Size, err = archive.CopyBounded(policy.ctx, writer, f, remaining)
			if err == nil {
				after, statErr := f.Stat()
				err = statErr
				if err == nil && (entry.Size != opened.Size() || after.Size() != opened.Size() || !after.ModTime().Equal(opened.ModTime())) {
					err = fault.New(4, "INPUT_DRIFT", "resource %q changed while scanning", name)
				}
			}
			err = errors.Join(err, f.Close())
			if out != nil {
				err = errors.Join(err, out.Sync(), out.Close())
			}
			if err != nil {
				return err
			}
			total += entry.Size
			entry.SHA256 = hex.EncodeToString(h.Sum(nil))
		default:
			return fmt.Errorf("unsafe tree entry %q: %s", name, info.Mode())
		}
		tree.Entries = append(tree.Entries, entry)
		return nil
	})
	if err != nil {
		return Tree{}, err
	}
	// Explicit sorting makes the hash contract independent of traversal order.
	sort.Slice(tree.Entries, func(i, j int) bool { return tree.Entries[i].Path < tree.Entries[j].Path })
	// JSON arrays/objects frame every field unambiguously (including odd names).
	tree.SHA256 = hashEntries(tree.Entries)
	if dst != nil {
		// Child directories before their parents; files were synced above.
		for i := len(tree.Entries) - 1; i >= 0; i-- {
			if err := policy.ctx.Err(); err != nil {
				return Tree{}, err
			}
			if tree.Entries[i].Type == "directory" {
				if err := syncDir(dst, tree.Entries[i].Path); err != nil {
					return Tree{}, err
				}
			}
		}
		if err := syncDir(dst, "."); err != nil {
			return Tree{}, err
		}
	}
	return tree, nil
}

func hashEntries(entries []Entry) string {
	data, _ := json.Marshal(entries)
	h := sha256.Sum256(append([]byte("kepub-tree-v1\n"), data...))
	return hex.EncodeToString(h[:])
}

// openDir rejects symlinks in user-supplied directory paths, not just the leaf.
// os.Root then confines all descendant access even if a tree changes mid-walk.
func openDir(dir string) (*os.Root, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	for p := abs; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("not a real directory: %q", p)
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	return os.OpenRoot(abs)
}

func subdir(r *os.Root, name string) (*os.Root, error) {
	for p := name; p != "."; p = path.Dir(p) {
		i, err := r.Lstat(p)
		if err != nil {
			return nil, err
		}
		if !i.IsDir() {
			return nil, fmt.Errorf("not a real directory: %q", p)
		}
	}
	return r.OpenRoot(name)
}

// publicationRoot reads a locked, tree-verified revision or checkpoint. It does
// not own/delete the root and never substitutes for full-tree provenance checks.
type publicationRoot struct{ root *os.Root }

func (r publicationRoot) HasFile(bp bookpath.BookPath) bool {
	if _, err := bookpath.Parse(string(bp)); err != nil {
		return false
	}
	p, err := subdir(r.root, path.Dir(string(bp)))
	if err != nil {
		return false
	}
	defer p.Close()
	i, err := p.Lstat(path.Base(string(bp)))
	return err == nil && i.Mode().IsRegular()
}

func (r publicationRoot) Read(bp bookpath.BookPath, max int64) ([]byte, error) {
	if _, err := bookpath.Parse(string(bp)); err != nil {
		return nil, err
	}
	p, err := subdir(r.root, path.Dir(string(bp)))
	if err != nil {
		return nil, err
	}
	defer p.Close()
	f, err := openRegular(p, path.Base(string(bp)))
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	err = errors.Join(err, f.Close())
	if int64(len(b)) > max {
		return nil, fmt.Errorf("resource %q exceeds parsing limit", bp)
	}
	return b, err
}

func syncDir(r *os.Root, name string) error {
	f, err := r.Open(name)
	if err != nil {
		return err
	}
	return errors.Join(f.Sync(), f.Close())
}

func hashAt(r *os.Root, name string) (Tree, error) {
	return hashAtWithIO(r, name, defaultResourceIO(nil))
}

func (w *Workspace) hashAt(name string) (Tree, error) {
	return hashAtWithIO(w.root, name, w.resources)
}

func hashAtWithIO(r *os.Root, name string, policy resourceIO) (Tree, error) {
	src, err := subdir(r, name)
	if err != nil {
		return Tree{}, err
	}
	defer src.Close()
	return scanTree(src, nil, policy)
}

func copyTree(r *os.Root, from, to string) (Tree, error) {
	return copyTreeWithIO(r, from, to, defaultResourceIO(nil))
}

func (w *Workspace) copyTree(from, to string) (Tree, error) {
	return copyTreeWithIO(w.root, from, to, w.resources)
}

func copyTreeWithIO(r *os.Root, from, to string, policy resourceIO) (tree Tree, err error) {
	if err := policy.ctx.Err(); err != nil {
		return Tree{}, err
	}
	src, err := subdir(r, from)
	if err != nil {
		return Tree{}, err
	}
	defer src.Close()
	if err := r.Mkdir(to, 0700); err != nil {
		return Tree{}, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, r.RemoveAll(to))
		}
	}()
	dst, err := subdir(r, to)
	if err != nil {
		return Tree{}, err
	}
	defer dst.Close()
	return scanTree(src, dst, policy)
}
