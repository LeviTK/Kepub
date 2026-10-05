package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/validation"
)

type PackResult struct {
	Output        string            `json:"output"`
	ArchiveSHA256 string            `json:"archiveSha256"`
	Draft         bool              `json:"draft"`
	Verified      bool              `json:"verified"`
	Validation    validation.Report `json:"validation"`
}

// PackSnapshot is the reusable export boundary. The caller supplies its approved
// inventory, matching kepub-tree-v1, and a frozen Archive. It cannot override the
// final ZIP check or atomically replace an existing output.
func PackSnapshot(ctx context.Context, a *archive.Archive, approved archive.Tree, output string, o validation.Options) (result PackResult, err error) {
	result.Output = output
	result.Draft = o.Draft
	result.ArchiveSHA256, err = a.PublishZIP(ctx, output, approved, func(filename, hash string) error {
		r, e := validation.CheckZIP(ctx, filename, hash, o)
		result.Validation = r
		return e
	})
	result.Verified = err == nil && !o.Draft && result.Validation.Status == "pass"
	return result, err
}

// Pack treats DIR as an explicit publication root and approves its frozen full
// resource inventory. It never searches parent directories for workspace files.
func Pack(ctx context.Context, dir, output string, o validation.Options) (PackResult, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return PackResult{}, err
	}
	out, err := filepath.Abs(output)
	if err != nil {
		return PackResult{}, err
	}
	// Staging/output inside the publication would be an unapproved new resource.
	rel, err := filepath.Rel(abs, out)
	if err != nil {
		return PackResult{}, err
	}
	if rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return PackResult{}, fault.New(2, "INVALID_OUTPUT", "output must be outside publication root")
	}
	a, t, err := archive.SnapshotDirectory(dir, archive.DefaultLimits)
	if err != nil {
		return PackResult{}, err
	}
	defer a.Close()
	return PackSnapshot(ctx, a, t, output, o)
}
