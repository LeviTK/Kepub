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
