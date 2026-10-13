package app

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/testfixture"
	"github.com/LeviTK/Kepub/internal/workspace"
)

func TestContentWorkspaceValidatesBeforeOpen(t *testing.T) {
	empty, invalid := "", string([]byte{0xff})
	zero := 0
	for _, tc := range []struct {
		resource string
		options  publication.ContentOptions
		code     string
	}{
		{"good.xhtml", publication.ContentOptions{Query: &empty}, "INVALID_CONTENT_QUERY"},
		{"good.xhtml", publication.ContentOptions{Query: &invalid}, "INVALID_CONTENT_QUERY"},
		{"good.xhtml", publication.ContentOptions{Limit: &zero}, "INVALID_CONTENT_LIMIT"},
		{"../escape.xhtml", publication.ContentOptions{}, "INVALID_ARGUMENT"},
		{"", publication.ContentOptions{}, "INVALID_ARGUMENT"},
		{invalid, publication.ContentOptions{}, "INVALID_ARGUMENT"},
	} {
		_, err := ContentWorkspace(t.Context(), filepath.Join(t.TempDir(), "does-not-exist"), tc.resource, tc.options)
		var f *fault.Error
		if !errors.As(err, &f) || f.Exit != 2 || f.Code != tc.code {
			t.Fatalf("expected argument error %s, got %v", tc.code, err)
		}
	}
}

func TestContentWorkspaceBusyAndReleaseOnError(t *testing.T) {
	book := filepath.Join(t.TempDir(), "book.epub")
	testfixture.ZIP(t, book, testfixture.EPUB("3.0", false))
	dir := filepath.Join(t.TempDir(), "workspace")
	w, err := workspace.Create(dir, book, workspace.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	_, err = ContentWorkspace(t.Context(), dir, "书/Text/第二 章.xhtml", publication.ContentOptions{})
	var f *fault.Error
	if !errors.As(err, &f) || f.Exit != 4 || f.Code != "WORKSPACE_BUSY" {
		t.Fatal(err)
	}
	// Invalid input wins over the held cooperative lock too.
	_, err = ContentWorkspace(t.Context(), dir, "/invalid", publication.ContentOptions{})
	if !errors.As(err, &f) || f.Exit != 2 {
		t.Fatal(err)
	}
	w.Close()
	for _, resource := range []string{"not-declared.xhtml", "书/Text/第二 章.xhtml"} {
		_, err := ContentWorkspace(t.Context(), dir, resource, publication.ContentOptions{})
		if (err == nil) != (resource == "书/Text/第二 章.xhtml") {
			t.Fatal(resource, err)
		}
		// The previous call must release the workspace on both success/error.
		reopened, err := workspace.Open(dir)
		if err != nil {
			t.Fatal("content leaked lock", err)
		}
		reopened.Close()
	}
}
