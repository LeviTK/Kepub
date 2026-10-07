package workspace

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/fix"
	"github.com/LeviTK/Kepub/internal/validation"
)

// TestFixProposalEncodingLayering keeps the T1 layering for the native fix
// rules: all three encodings plan and apply without transcoding, while the
// pinned EPUBCheck still formally rejects UTF-16 XHTML.
func TestFixProposalEncodingLayering(t *testing.T) {
	src := `<?xml version="1.0" encoding="utf-8"?>` + "\n" + strings.Replace(fixSource,
		`<p id="start">One.</p>`,
		`<p id="start">One.</p><p>Text <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg> tail.</p>`, 1)
	for _, enc := range []string{"utf8", "utf16le", "utf16be"} {
		t.Run(enc, func(t *testing.T) {
			wantDecoded := strings.Replace(src, ` epub:type="secrecy"`, "", 1)
			wantDecoded = strings.Replace(wantDecoded, "?q=1#start2", "#start2", 1)
			if enc != "utf8" {
				wantDecoded = strings.Replace(wantDecoded, `encoding="utf-8"`, `encoding="utf-16"`, 1)
			}
			w, dir, _ := structureWorkspace(t, map[string]string{"EPUB/chapter1.xhtml": encodeChapter(src, enc)})
			defer w.Close()
			s, err := w.FixSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			all, err := fix.Propose(s, fix.Selection{Mode: fix.ModeAll, RepairIDs: []string{}})
			if err != nil {
				t.Fatal(err)
			}
			if len(all.Repairs) != 2 {
				t.Fatalf("repairs: %+v", all.Repairs)
			}
			p, err := fix.Propose(s, fix.Selection{Mode: fix.ModeExplicit, RepairIDs: []string{all.Repairs[0].RepairID, all.Repairs[1].RepairID}})
			if err != nil {
				t.Fatal(err)
			}
			plan, err := w.Plan(fixSchema7JSON(t, p))
			if err != nil {
				t.Fatal(err)
			}
			e := applyPlan(t, w, plan)
			cand := readResource(t, filepath.Join(dir, candidate, "EPUB/chapter1.xhtml"))
			decoded, err := decodeChapter(cand, enc)
			if err != nil {
				t.Fatal(err)
			}
			if decoded != wantDecoded {
				t.Fatalf("%s candidate:\n got %q\nwant %q", enc, decoded, wantDecoded)
			}
			if enc != "utf8" {
				bom := []byte{0xFF, 0xFE}
				if enc == "utf16be" {
					bom = []byte{0xFE, 0xFF}
				}
				if len(cand) < 2 || !bytes.Equal(cand[:2], bom) {
					t.Fatalf("%s: candidate lost its BOM", enc)
				}
			}
			if os.Getenv("KEPUB_EPUBCHECK_JAR") != "" {
				report, verr := validation.Validate(context.Background(), filepath.Join(dir, candidate), validation.Options{Timeout: 2 * time.Minute})
				htm := false
				for _, d := range report.Diagnostics {
					if strings.Contains(d.UpstreamCode, "HTM_058") || strings.Contains(d.Message, "HTM_058") {
						htm = true
					}
				}
				if enc == "utf8" {
					if verr != nil {
						t.Fatalf("utf8 candidate rejected: %v %+v", verr, report.Diagnostics)
					}
				} else {
					if verr == nil {
						t.Fatalf("%s: checker formally accepted UTF-16 XHTML", enc)
					}
					if !htm {
						t.Fatalf("%s: HTM_058 not reported: %+v", enc, report.Diagnostics)
					}
				}
			}
			if _, err := w.Reject(e.TaskID); err != nil {
				t.Fatal(err)
			}
		})
	}
}
