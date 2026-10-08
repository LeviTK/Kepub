// Package archive stages every entry in a private directory before exposing it.
package archive

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
)

type Limits struct {
	// Entries counts unique files and directories, including implicit parents.
	Entries               int
	FileBytes, TotalBytes int64
	PathBytes             int64
}

var DefaultLimits = Limits{20000, 256 << 20, 2 << 30, 32 << 20}

// CheckEntry is the shared expanded-inventory budget for archive and workspace
// trees. Call it before adding an entry or creating its destination.
func (l Limits) CheckEntry(count int, pathBytes int64, p bookpath.BookPath) error {
	if count >= l.Entries {
		return fault.New(1, "ARCHIVE_LIMIT", "too many expanded entries")
	}
	if pathBytes > l.PathBytes || int64(len(p)) > l.PathBytes-pathBytes {
		return fault.New(1, "ARCHIVE_LIMIT", "cumulative expanded path bytes exceed limit")
	}
	return nil
}

type Archive struct {
	dir    string
	limits Limits
	ctx    context.Context
	Files  map[bookpath.BookPath]int64
}

func (a *Archive) Close() { _ = os.RemoveAll(a.dir) }

func (a *Archive) context() context.Context {
	if a.ctx == nil {
		return context.Background()
	}
	return a.ctx
}

// Canonicalize only our own newly created staging path. System temp roots can
// have aliases (notably on macOS); publication input links remain forbidden.
func privateDir(prefix string) (string, error) {
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return real, nil
}

func Open(filename string, limits Limits) (*Archive, error) {
	return OpenContext(context.Background(), filename, limits)
}

func OpenContext(ctx context.Context, filename string, limits Limits) (*Archive, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if info, err := os.Stat(filename); err == nil && info.Size() > MaxInputBytes {
		return nil, fault.New(1, "ARCHIVE_LIMIT", "raw ZIP input exceeds %d bytes", MaxInputBytes)
	}
	z, err := zip.OpenReader(filename)
	if err != nil {
		var pe *os.PathError
		if errors.As(err, &pe) {
			return nil, fault.New(6, "IO_ERROR", "open input: %v", err)
		}
		if _, e := os.Stat(filename); e != nil {
			return nil, fault.New(6, "IO_ERROR", "open input: %v", e)
		}
		return nil, fault.New(1, "INVALID_ZIP", "read ZIP: %v", err)
	}
	defer z.Close()
	if len(z.File) > limits.Entries {
		return nil, fault.New(1, "ARCHIVE_LIMIT", "too many ZIP entries")
	}
	// Check explicit entries AND every implicit parent, including directory case aliases.
	seen := map[bookpath.BookPath]bool{}
	names := map[string]bookpath.BookPath{}
	kinds := map[bookpath.BookPath]bool{}
	var pathBytes int64
	for _, f := range z.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		isDir := strings.HasSuffix(f.Name, "/")
		name := strings.TrimSuffix(f.Name, "/")
		p, e := bookpath.Parse(name)
		if e != nil {
			return nil, e
		}
		if seen[p] {
			return nil, fault.New(1, "ARCHIVE_COLLISION", "duplicate entry %q", p)
		}
		seen[p] = true
		mode := f.Mode()
		if (!mode.IsRegular() && !mode.IsDir()) || mode.IsDir() != isDir || f.Flags&1 != 0 {
			return nil, fault.New(1, "UNSAFE_ENTRY", "special or encrypted ZIP entry %q", p)
		}
		for q, dir := p, isDir; q != "."; q, dir = bookpath.BookPath(path.Dir(string(q))), true {
			key := bookpath.CollisionKey(q)
			if old, ok := names[key]; ok && old != q {
				return nil, fault.New(1, "ARCHIVE_COLLISION", "colliding paths %q and %q", old, q)
			}
			names[key] = q
			if old, ok := kinds[q]; ok {
				if old != dir {
					return nil, fault.New(1, "ARCHIVE_COLLISION", "file/directory conflict %q", q)
				}
				continue
			}
			if e := limits.CheckEntry(len(kinds), pathBytes, q); e != nil {
				return nil, e
			}
			pathBytes += int64(len(q))
			kinds[q] = dir
		}
	}
	dir, err := privateDir("kepub-read-")
	if err != nil {
		return nil, err
	}
	a := &Archive{dir: dir, limits: limits, ctx: ctx, Files: map[bookpath.BookPath]int64{}}
	success := false
	defer func() {
		if !success {
			a.Close()
		}
	}()
	var total int64
	for _, f := range z.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		target := filepath.Join(dir, filepath.FromSlash(strings.TrimSuffix(f.Name, "/")))
		if strings.HasSuffix(f.Name, "/") {
			r, e := f.Open()
			if e != nil {
				return nil, fault.New(1, "INVALID_ZIP", "directory: %v", e)
			}
			b, e := io.ReadAll(io.LimitReader(r, 1))
			r.Close()
			if e != nil || len(b) != 0 {
				return nil, fault.New(1, "UNSAFE_ENTRY", "directory contains data")
			}
			if e = os.MkdirAll(target, 0700); e != nil {
				return nil, e
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return nil, err
		}
		r, e := f.Open()
		if e != nil {
			return nil, fault.New(1, "INVALID_ZIP", "entry: %v", e)
		}
		w, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			r.Close()
			return nil, e
		}
		max := min(limits.FileBytes, limits.TotalBytes-total)
		n, e := CopyBounded(ctx, w, r, max)
		ce := w.Close()
		r.Close()
		if e != nil {
			var fe *fault.Error
			if errors.As(e, &fe) && fe.Code == "ARCHIVE_LIMIT" {
				return nil, fault.New(1, "ARCHIVE_LIMIT", "actual expanded bytes exceed limit")
			}
			if errors.As(e, &fe) || ctx.Err() != nil {
				return nil, e
			}
			var pe *os.PathError
			if errors.As(e, &pe) {
				return nil, e
			}
			return nil, fault.New(1, "INVALID_ZIP", "entry checksum/decompression: %v", e)
		}
		if ce != nil {
			return nil, ce
		}
		total += n
		a.Files[bookpath.BookPath(f.Name)] = n
	}
	b, e := a.Read("mimetype", 64)
	if e != nil {
		return nil, e
	}
	if !bytes.Equal(b, []byte("application/epub+zip")) {
		return nil, fault.New(1, "INVALID_EPUB", "invalid mimetype")
	}
	success = true
	return a, nil
}

func (a *Archive) HasFile(p bookpath.BookPath) bool {
	_, ok := a.Files[p]
	return ok
}

func (a *Archive) Read(p bookpath.BookPath, max int64) ([]byte, error) {
	n, ok := a.Files[p]
	if !ok {
		return nil, fault.New(1, "MISSING_RESOURCE", "missing resource %q", p)
	}
	if n > max {
		return nil, fault.New(1, "RESOURCE_LIMIT", "resource %q exceeds parsing limit", p)
	}
	return os.ReadFile(filepath.Join(a.dir, filepath.FromSlash(string(p))))
}

// Unpack stages all data on the destination filesystem, then publishes with an
// atomic no-replace rename. Existing destinations are never replaced.
func (a *Archive) Unpack(output string) error {
	abs, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	if _, err = os.Lstat(abs); err == nil {
		return fault.New(2, "OUTPUT_EXISTS", "output already exists")
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err = a.Inventory(); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(abs), ".kepub-unpack-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	_, err = scan(a.context(), a.dir, stage, a.limits)
	if err != nil {
		return err
	}
	if err := a.context().Err(); err != nil {
		return err
	}
	if err = publish(stage, abs); err != nil {
		if os.IsExist(err) {
			return fault.New(2, "OUTPUT_EXISTS", "output already exists")
		}
		return err
	}
	return nil
}
