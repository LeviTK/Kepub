package validation_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/validation"
)

// These fixtures check the three changes identified in the 2026 EPUB 3.3
// Recommendation changelog (#2637, #2555, #2556), not full REC coverage.
// Character references preserve the separators through XML attribute parsing.
func TestRealREC2026Viewport(t *testing.T) {
	for _, tc := range []struct{ name, separator, diagnostic string }{
		{"space", " ", ""},
		{"tab", "&#x9;", ""},
		{"lf", "&#xA;", ""},
		{"cr", "&#xD;", ""},
		{"form-feed-not-xml", "&#xC;", "RSC-016"},
		// NBSP remains part of each property name, so width/height are absent.
		{"nbsp-not-xml-space", "&#xA0;", "HTM_056"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := legal(t, "3.0")
			opf, err := os.ReadFile(filepath.Join(dir, "EPUB/package.opf"))
			if err != nil {
				t.Fatal(err)
			}
			write(t, dir, "EPUB/package.opf", strings.Replace(string(opf), "</metadata>", `<meta property="rendition:layout">pre-paginated</meta></metadata>`, 1))
			// Only the first viewport of fixed-layout spine content is used.
			write(t, dir, "EPUB/chapter.xhtml", fmt.Sprintf(`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Viewport</title><meta name="viewport" content="width%[1]s=%[1]s600,%[1]sheight%[1]s=%[1]s800"/></head><body><p id="start">Fixed layout</p></body></html>`, tc.separator))
			checkREC2026(t, dir, "EPUB/chapter.xhtml", tc.diagnostic)
		})
	}
}

func TestRealREC2026SVGType(t *testing.T) {
	for _, inline := range []bool{false, true} {
		for _, tc := range []struct{ name, svg, diagnostic string }{
			{"a-allowed", `<title>Diagram</title><a href="chapter.xhtml#start" epub:type="bodymatter"><rect width="10" height="10"/></a>`, ""},
			{"defs-forbidden", `<title>Diagram</title><defs epub:type="bodymatter"><rect id="unused" width="10" height="10"/></defs><rect width="10" height="10"/>`, "RSC-005"},
			{"title-forbidden", `<title epub:type="bodymatter">Diagram</title><rect width="10" height="10"/>`, "RSC-005"},
		} {
			t.Run(fmt.Sprintf("inline=%v/%s", inline, tc.name), func(t *testing.T) {
				dir := legal(t, "3.0")
				svg := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:epub="http://www.idpf.org/2007/ops" width="100" height="100" viewBox="0 0 100 100">` + tc.svg + `</svg>`
				opfBytes, err := os.ReadFile(filepath.Join(dir, "EPUB/package.opf"))
				if err != nil {
					t.Fatal(err)
				}
				opf := string(opfBytes)
				path := "EPUB/diagram.svg"
				if inline {
					path = "EPUB/chapter.xhtml"
					write(t, dir, path, `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Inline SVG</title></head><body><p id="start">Target</p>`+svg+`</body></html>`)
					opf = strings.Replace(opf, `id="chapter"`, `id="chapter" properties="svg"`, 1)
				} else {
					write(t, dir, path, svg)
					opf = strings.Replace(opf, "</manifest>", `<item id="diagram" href="diagram.svg" media-type="image/svg+xml"/></manifest>`, 1)
					opf = strings.Replace(opf, "</spine>", `<itemref idref="diagram"/></spine>`, 1)
					write(t, dir, "EPUB/nav.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="chapter.xhtml#start">Chapter</a></li><li><a href="diagram.svg">Diagram</a></li></ol></nav></body></html>`)
				}
				write(t, dir, "EPUB/package.opf", opf)
				checkREC2026(t, dir, path, tc.diagnostic)
			})
		}
	}
}

func checkREC2026(t *testing.T, dir, diagnosticPath, wantDiagnostic string) {
	t.Helper()
	jar := os.Getenv("KEPUB_EPUBCHECK_JAR")
	if jar == "" {
		t.Skip("requires pinned real EPUBCheck; a skipped run is not REC coverage")
	}
	book := zipFixture(t, dir)
	before, err := archive.FileSHA256(book)
	if err != nil {
		t.Fatal(err)
	}
	r, err := validation.Validate(context.Background(), book, validation.Options{JAR: jar, Timeout: 30 * time.Second})
	if after, hashErr := archive.FileSHA256(book); hashErr != nil || after != before {
		t.Fatal("validation changed original ZIP", hashErr)
	}
	if r.ArchiveSHA256 != before || r.InputTreeSHA256 == "" {
		t.Fatal("missing input binding", r)
	}
	wantExit, wantStatus := 0, "passed"
	if wantDiagnostic != "" {
		wantExit, wantStatus = 1, "failed"
	}
	checked := false
	for _, c := range r.Checks {
		if c.ID != "epubcheck" {
			continue
		}
		checked = true
		if c.BackendExitCode == nil || *c.BackendExitCode != wantExit || c.Status != wantStatus || c.Version != "5.3.0" || c.ToolSHA256 != validation.ToolSHA256 || c.InputSHA256 != before || len(c.RawReport) == 0 || c.InvalidReport != "" {
			t.Fatalf("unexpected checker result: %+v; error=%v; diagnostics=%+v", c, err, r.Diagnostics)
		}
		t.Logf("backend=%d status=%s version=%s rules=%s archive=%s tree=%s", *c.BackendExitCode, c.Status, c.Version, c.Rules, before, r.InputTreeSHA256)
	}
	if !checked {
		t.Fatal("real checker not run")
	}
	if wantDiagnostic == "" {
		if err != nil || r.Status != "pass" {
			t.Fatalf("legal fixture rejected: %v; %+v", err, r.Diagnostics)
		}
		return
	}
	if err == nil || r.Status != "fail" {
		t.Fatalf("illegal fixture not rejected: %v; %+v", err, r)
	}
	for _, d := range r.Diagnostics {
		if d.Source == "epubcheck" && d.UpstreamCode == wantDiagnostic && d.BookPath == diagnosticPath && (d.Severity == "error" || d.Severity == "fatal") {
			t.Logf("upstream=%s path=%s severity=%s", d.UpstreamCode, d.BookPath, d.Severity)
			return
		}
	}
	t.Fatalf("missing %s at %s: %+v", wantDiagnostic, diagnosticPath, r.Diagnostics)
}
