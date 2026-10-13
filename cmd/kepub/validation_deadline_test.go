package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/testfixture"
)

// The first observation admits validation; the next interrupts freeze/copy,
// before the checker can classify the error. No wall-clock race is required.
type validationDeadlineContext struct {
	context.Context
	calls int
	after int
}

func (c *validationDeadlineContext) Err() error {
	c.calls++
	if c.calls >= c.after {
		return context.DeadlineExceeded
	}
	return nil
}

func TestD2EarlyValidationDeadline(t *testing.T) {
	dir := t.TempDir()
	book := filepath.Join(t.TempDir(), "input.epub")
	entries := []testfixture.Entry{
		{Name: "mimetype", Data: []byte("application/epub+zip")},
		{Name: "META-INF/container.xml", Data: []byte(`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`)},
		{Name: "package.opf", Data: []byte(`<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:test:deadline</dc:identifier><dc:title>Deadline</dc:title><dc:language>en</dc:language></metadata><manifest><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="chapter"/></spine></package>`)},
		{Name: "chapter.xhtml", Data: []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Deadline</title></head><body><p>Unchanged.</p></body></html>`)},
	}
	testfixture.ZIP(t, book, entries)
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, entry.Data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	binary := workspaceBinary(t)
	for _, command := range []string{"validate", "pack"} {
		for _, control := range []string{"deadline", "cancel", "io", "success"} {
			t.Run(command+"/"+control, func(t *testing.T) {
				input := dir
				if command == "validate" {
					input = book
				}
				out := filepath.Join(t.TempDir(), "output.epub")
				deadline := &validationDeadlineContext{Context: context.Background(), after: 2}
				var ctx context.Context = deadline
				wantCode, wantExit := "CHECKER_TIMEOUT", 5
				switch control {
				case "cancel":
					cancelled, cancel := context.WithCancel(context.Background())
					cancel()
					ctx = cancelled
					wantCode, wantExit = "CANCELLED", 130
				case "io":
					// Lstat fails before the deadline is observed by execute.
					// That unrelated I/O error must retain its original identity.
					input = filepath.Join(t.TempDir(), "missing")
					if command == "pack" {
						deadline.after = 3 // Snapshot entry and scan both check context.
					}
				case "success":
					ctx = context.Background()
				}
				_, err := execute(ctx, options{command: command, book: input, output: out, draft: true})
				switch control {
				case "success":
					if err != nil {
						t.Fatal(err)
					}
				case "io":
					if !errors.Is(err, os.ErrNotExist) || !errors.Is(ctx.Err(), context.DeadlineExceeded) {
						t.Fatalf("I/O error reclassified: %v", err)
					}
				default:
					var f *fault.Error
					if !errors.As(err, &f) || f.Code != wantCode || f.Exit != wantExit {
						t.Fatalf("got %v; want %s/%d", err, wantCode, wantExit)
					}
				}
				if command == "pack" {
					_, statErr := os.Stat(out)
					if control == "success" && statErr != nil || control != "success" && !errors.Is(statErr, os.ErrNotExist) {
						t.Fatalf("publication state: %v", statErr)
					}
				}
				if control == "io" || control == "success" && command == "pack" {
					args := []string{command, input}
					if command == "pack" {
						args = append(args, "--draft", "--output", filepath.Join(t.TempDir(), "cli.epub"))
					}
					exit := 0
					if control == "io" {
						exit = 6
					}
					result := processJSON(t, binary, args, exit, 30*time.Second)
					if control == "io" && result["error"].(map[string]any)["code"] != "IO_ERROR" {
						t.Fatal("wrong I/O envelope", result)
					}
				}
			})
		}
	}
}
