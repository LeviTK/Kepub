package validation_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/LeviTK/Kepub/internal/app"
	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/references"
	"github.com/LeviTK/Kepub/internal/validation"
)

func TestR4ValidationAndFormalExportBudget(t *testing.T) {
	dir := legal(t, "3.0")
	book := zipFixture(t, dir)
	hash, err := archive.FileSHA256(book)
	if err != nil {
		t.Fatal(err)
	}
	limits := references.DefaultGraphLimits
	limits.Edges = 1
	o := validation.Options{GraphLimits: &limits}
	r, err := validation.CheckZIP(context.Background(), book, hash, o)
	code(t, err, "REFERENCE_LIMIT")
	if r.Status != "incomplete" || r.Checks[2].Status != "blocked" || r.Checks[3].Status != "blocked" {
		t.Fatal("partial graph became pass", r)
	}
	output := filepath.Join(t.TempDir(), "formal.epub")
	result, err := app.Pack(context.Background(), dir, output, o)
	code(t, err, "REFERENCE_LIMIT")
	if result.Verified {
		t.Fatal("budget bypassed formal export")
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		t.Fatal("failed export published", err)
	}
	result, err = app.Pack(context.Background(), dir, output, validation.Options{})
	if err != nil || !result.Verified || result.Validation.Checks[3].Status != "passed" || result.Validation.Checks[3].ToolSHA256 != validation.ToolSHA256 {
		t.Fatal("real checker positive control", result, err)
	}
	a, err := archive.Open(output, archive.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if len(a.Files) != 6 {
		t.Fatal("export lost resource", len(a.Files))
	}
	for _, name := range []string{"mimetype", "META-INF/container.xml", "EPUB/package.opf", "EPUB/chapter.xhtml", "EPUB/style.css", "EPUB/nav.xhtml"} {
		want, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		got, err := a.Read(bookpath.BookPath(name), 8<<20)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatal("export modified bytes", name, err)
		}
	}
	if after, err := archive.FileSHA256(book); err != nil || after != hash {
		t.Fatal("modified original", err)
	}
}
