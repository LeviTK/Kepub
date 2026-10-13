package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

func TestR4ReferenceFailureEnvelope(t *testing.T) {
	book := filepath.Join(t.TempDir(), "graph.epub")
	entries := testfixture.ReferenceEPUB()
	for i := range entries {
		if entries[i].Name == "书/Text/-first.xhtml" {
			// A 140 KiB synthetic resource exceeds only the documented diagnostic
			// count cap. Other boundary tests inject tiny budgets; no exhaustion.
			entries[i].Data = []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body>` + strings.Repeat(`<p id="same"/>`, 10002) + `</body></html>`)
		}
	}
	testfixture.ZIP(t, book, entries)
	before, err := os.ReadFile(book)
	if err != nil {
		t.Fatal(err)
	}
	binary := workspaceBinary(t)
	for _, direction := range []string{"incoming", "outgoing"} {
		result := processJSON(t, binary, []string{"inspect", book, "--section", "references", "--resource", "书/nav.xhtml", "--direction", direction}, 1, 30*time.Second)
		if result["data"] != nil || result["error"].(map[string]any)["code"] != "REFERENCE_LIMIT" {
			t.Fatal("wrong failure envelope", result)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = execute(ctx, options{command: "inspect", book: book, section: "references"})
	var f *fault.Error
	if !errors.As(err, &f) || f.Code != "CANCELLED" || f.Exit != 130 {
		t.Fatal("cancellation became IO_ERROR", err)
	}
	after, err := os.ReadFile(book)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("modified original", err)
	}
}
