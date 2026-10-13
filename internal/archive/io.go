package archive

import (
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path"
	"syscall"

	"github.com/LeviTK/Kepub/internal/fault"
)

// MaxInputBytes bounds raw ZIP input, independently of expanded Limits. Its
// value preserves the existing formal-check input cap; compressed and expanded
// bytes are different budgets even when their default totals happen to match.
const MaxInputBytes int64 = 2 << 30

type contextReader struct {
	ctx context.Context
	io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.Reader.Read(p)
	if err == nil {
		err = r.ctx.Err()
	}
	return n, err
}

// CopyBounded consumes at most limit+1 actual bytes, including the over-limit
// probe. Reader/short-write errors remain I/O errors, not budget failures.
func CopyBounded(ctx context.Context, dst io.Writer, src io.Reader, limit int64) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if limit < 0 {
		return 0, fault.New(1, "ARCHIVE_LIMIT", "negative remaining byte budget")
	}
	probe := limit
	if probe < math.MaxInt64 {
		probe++
	}
	n, err := io.Copy(dst, io.LimitReader(contextReader{ctx, src}, probe))
	if err == nil {
		err = ctx.Err()
	}
	if err == nil && n > limit {
		err = fault.New(1, "ARCHIVE_LIMIT", "actual bytes exceed %d-byte budget", limit)
	}
	return n, err
}

// WalkDirectory visits one entry at a time, never allocating an unbounded
// ReadDir result before the caller can check entry/path budgets. The callback
// runs before descending or opening file data. Hash consumers sort afterwards.
func WalkDirectory(ctx context.Context, root *os.Root, visit func(string, os.FileInfo) error) error {
	var walk func(string) error
	walk = func(dir string) (err error) {
		if err := ctx.Err(); err != nil {
			return err
		}
		f, err := root.OpenFile(dir, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_DIRECTORY, 0)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, f.Close()) }()
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			names, err := f.Readdirnames(1)
			if err != nil && err != io.EOF {
				return err
			}
			for _, leaf := range names {
				name := path.Join(dir, leaf)
				info, err := root.Lstat(name)
				if err != nil {
					return err
				}
				if err := visit(name, info); err != nil {
					return err
				}
				if info.IsDir() {
					if err := walk(name); err != nil {
						return err
					}
				}
			}
			if err == io.EOF {
				return nil
			}
		}
	}
	return walk(".")
}
