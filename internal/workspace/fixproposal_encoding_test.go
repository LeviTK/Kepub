package workspace

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/fix"
	"github.com/LeviTK/Kepub/internal/publication"
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

// Empty new href is a physical edit, not a missing fact or a no-op. The
// no-fragment cross-resource case must bind the existence dependency too.
func TestFixRelativeURLQueryBytes(t *testing.T) {
	for _, enc := range []string{"utf8", "utf16le", "utf16be"} {
		for _, tc := range []struct{ old, value string }{
			{"?x=1", ""}, {"?", ""}, {"chapter2.xhtml?q=1", "chapter2.xhtml"},
		} {
			t.Run(enc+"/"+tc.old, func(t *testing.T) {
				src := `<?xml version="1.0" encoding="utf-8"?>` + "\r\n" +
					strings.Replace(strings.Replace(fixSource, "chapter2.xhtml?q=1#start2", tc.old, 1), "One</title>", "字😀 &amp; One</title>", 1) + "\r\n"
				w, dir, _ := structureWorkspace(t, map[string]string{"EPUB/chapter1.xhtml": encodeChapter(src, enc)})
				defer w.Close()
				// The independently chosen seven-resource set never comes from a
				// product WriteSet. Only the explicit href literal may change.
				want := map[string][]byte{}
				for _, name := range []string{"mimetype", "META-INF/container.xml", "EPUB/package.opf", "EPUB/chapter1.xhtml", "EPUB/chapter2.xhtml", "EPUB/style.css", "EPUB/nav.xhtml"} {
					want[name] = readResource(t, filepath.Join(dir, revision, filepath.FromSlash(name)))
				}
				want["EPUB/chapter1.xhtml"] = []byte(encodeChapter(strings.Replace(src, `href="`+tc.old+`"`, `href="`+tc.value+`"`, 1), enc))
				check := func(t *testing.T) {
					t.Helper()
					for name, expected := range want {
						got := readResource(t, filepath.Join(dir, candidate, filepath.FromSlash(name)))
						if !bytes.Equal(got, expected) {
							t.Fatalf("complete %s %s bytes mismatch", enc, name)
						}
					}
				}
				b := structureBinding(t, w, "EPUB/chapter1.xhtml")
				op := b.attrSet(b.locator(t, "a", 0), "href", &tc.old, tc.value)
				legacy, err := w.Plan(editJSON(t, map[string]any{"schemaVersion": 4, "operations": []Operation{op}}))
				if err != nil {
					t.Fatalf("legacy gate control: %v", err)
				}
				e := applyPlan(t, w, legacy)
				check(t)
				if _, err := w.Reject(e.TaskID); err != nil {
					t.Fatal(err)
				}
				s, err := w.FixSnapshot()
				if err != nil {
					t.Fatal(err)
				}
				all := fixSourceProposal(t, w)
				if len(all.Repairs) != 2 || len(all.Limitations) != 0 || len(fix.NativeDiagnostics(s)) != 2 {
					t.Fatalf("query/native facts: %+v", all)
				}
				r := all.Repairs[1]
				if r.Operation == nil {
					t.Fatalf("legal query has no operation: %+v", r)
				}
				set, ok := r.Operation.Params.(publication.AttributeSet)
				if !ok || r.Status != fix.StatusFixable || set.Value != tc.value || r.Target.ExpectedOldValue != tc.old {
					t.Fatalf("query repair: %+v", r)
				}
				read := []string{"EPUB/chapter1.xhtml", "EPUB/package.opf", "META-INF/container.xml"}
				if tc.value != "" {
					read = []string{"EPUB/chapter1.xhtml", "EPUB/chapter2.xhtml", "EPUB/package.opf", "META-INF/container.xml"}
				}
				p, err := fix.Propose(s, fix.Selection{Mode: fix.ModeExplicit, RepairIDs: []string{r.RepairID}})
				if err != nil || !slices.Equal(r.ReadSet, read) || !slices.Equal(p.Derived.ReadSet, read) {
					t.Fatalf("semantic dependencies: repair=%v derived=%v want=%v err=%v", r.ReadSet, p.Derived.ReadSet, read, err)
				}
				plan, err := w.Plan(fixSchema7JSON(t, p))
				if err != nil {
					t.Fatal(err)
				}
				e = applyPlan(t, w, plan)
				check(t)
				if _, err := w.Reject(e.TaskID); err != nil {
					t.Fatal(err)
				}
				// Mixed FR1+FR2 must execute both legal permutations, each against
				// an independent byte oracle rather than just against the other.
				want["EPUB/chapter1.xhtml"] = []byte(encodeChapter(strings.Replace(strings.Replace(src, `href="`+tc.old+`"`, `href="`+tc.value+`"`, 1), ` epub:type="secrecy"`, "", 1), enc))
				ops := slices.Clone(all.Derived.Operations)
				for order := 0; order < 2; order++ {
					plan, err := w.Plan(editJSON(t, map[string]any{"schemaVersion": 7, "proposal": all, "operations": ops}))
					if err != nil {
						t.Fatalf("mixed order %d: %v", order, err)
					}
					e = applyPlan(t, w, plan)
					check(t)
					if _, err := w.Reject(e.TaskID); err != nil {
						t.Fatal(err)
					}
					slices.Reverse(ops)
				}
			})
		}
	}
}
