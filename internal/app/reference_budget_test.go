package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/references"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

func TestR3InspectRequestCancellation(t *testing.T) {
	book := filepath.Join(t.TempDir(), "book.epub")
	testfixture.ZIP(t, book, testfixture.EPUB("3.0", false))
	a, p, err := Read(book, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, section := range []string{"metadata", "manifest", "spine", "navigation", "references", "capabilities"} {
		t.Run(section, func(t *testing.T) {
			if value, err := Inspect(t.Context(), a, p, section, "", "", references.DefaultGraphLimits); err != nil || value == nil {
				t.Fatal("live query failed", value, err)
			}
			if value, err := Inspect(ctx, a, p, section, "", "", references.DefaultGraphLimits); !errors.Is(err, context.Canceled) || value != nil {
				t.Fatal("cancelled query returned data", value, err)
			}
		})
	}
}

func TestR4InspectFiltersDoNotBypassBudget(t *testing.T) {
	book := filepath.Join(t.TempDir(), "references.epub")
	testfixture.ZIP(t, book, testfixture.ReferenceEPUB())
	a, p, err := Read(book, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	limits := references.DefaultGraphLimits
	limits.Edges = 1
	for _, direction := range []string{"incoming", "outgoing"} {
		data, err := Inspect(context.Background(), a, p, "references", "书/Text/-first.xhtml", direction, limits)
		var f *fault.Error
		if !errors.As(err, &f) || f.Code != "REFERENCE_LIMIT" || f.Exit != 1 || data != nil {
			t.Fatal("filtered graph hid global exhaustion", data, err)
		}
	}
	if _, err := Inspect(context.Background(), a, p, "references", "书/Text/-first.xhtml", "incoming", references.DefaultGraphLimits); err != nil {
		t.Fatal("normal filter", err)
	}
}
