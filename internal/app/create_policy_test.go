package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

func TestWorkspaceCreatePolicyErrors(t *testing.T) {
	for _, kind := range []string{"destination", "symlink", "hardlink", "directory", "missing-source", "missing-parent"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			book, ws := filepath.Join(dir, "book.epub"), filepath.Join(dir, "ws")
			testfixture.ZIP(t, book, testfixture.EPUB("3.0", false))
			wantExit, wantCode := 2, "INVALID_ARGUMENT"
			switch kind {
			case "destination":
				if err := os.Mkdir(ws, 0700); err != nil {
					t.Fatal(err)
				}
				wantCode = "OUTPUT_EXISTS"
			case "symlink", "hardlink":
				other := filepath.Join(dir, "other.epub")
				var err error
				if kind == "symlink" {
					err = os.Symlink(book, other)
				} else {
					err = os.Link(book, other)
				}
				if err != nil {
					t.Fatal(err)
				}
				book = other
			case "directory":
				book = dir
			case "missing-source":
				book += ".absent"
				wantExit, wantCode = 6, "IO_ERROR"
			case "missing-parent":
				ws = filepath.Join(dir, "absent", "ws")
				wantExit, wantCode = 6, "IO_ERROR"
			}
			_, err := OpenWorkspace(t.Context(), book, ws, "")
			var f *fault.Error
			if wantExit == 6 && err != nil && !errors.As(err, &f) {
				// The CLI's unchanged fallback maps ordinary filesystem errors to IO6.
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("not an OS failure: %v", err)
				}
				return
			}
			if !errors.As(err, &f) || f.Exit != wantExit || f.Code != wantCode {
				t.Fatalf("%v, want %d %s", err, wantExit, wantCode)
			}
			if kind != "destination" {
				if _, err := os.Lstat(ws); !os.IsNotExist(err) {
					t.Fatalf("published rejected source: %v", err)
				}
			}
		})
	}
}
