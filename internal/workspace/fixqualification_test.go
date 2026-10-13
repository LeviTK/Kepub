package workspace

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/fix"
	"github.com/LeviTK/Kepub/internal/testfixture"
	"github.com/LeviTK/Kepub/internal/validation"
)

func TestN3DirectoryTargetQualification(t *testing.T) {
	entries := testfixture.EPUB("3.0", false)
	for i := range entries {
		if entries[i].Name == "书/Text/-first.xhtml" {
			entries[i].Data = []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>目录</title></head><body><a href="../../empty?cleanup=1">目录目标</a></body></html>`)
		}
	}
	book := filepath.Join(t.TempDir(), "directory.epub")
	testfixture.ZIP(t, book, entries)
	w, err := Create(filepath.Join(t.TempDir(), "ws"), book, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	s, err := w.FixSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	p, err := fix.Propose(s, fix.Selection{Mode: fix.ModeAll})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Repairs) != 1 {
		t.Fatalf("directory fact disappeared: %+v", p)
	}
	r := p.Repairs[0]
	if r.Status != fix.StatusUnfixable || r.Operation != nil || len(r.WriteSet) != 0 || r.UnfixableReason == "" {
		t.Fatalf("directory is not a resource file, but proposal claims executability: %+v", r)
	}
	if info, err := os.Stat(filepath.Join(w.dir, revision, "empty")); err != nil || !info.IsDir() {
		t.Fatalf("empty directory was not retained: %v", err)
	}
}

func TestN3FileTargetMatrix(t *testing.T) {
	for _, enc := range []string{"utf8", "utf16le", "utf16be"} {
		for _, tc := range []struct {
			name, old, value   string
			fixable, duplicate bool
		}{
			{"file", "第二%20章.xhtml?cleanup=1", "第二%20章.xhtml", true, false},
			{"empty-query", "第二%20章.xhtml?", "第二%20章.xhtml", true, false},
			{"unique-fragment", "第二%20章.xhtml?#note", "第二%20章.xhtml#note", true, false},
			{"missing-fragment", "第二%20章.xhtml?x=1#absent", "第二%20章.xhtml#absent", false, false},
			{"duplicate-fragment", "第二%20章.xhtml?x=1#note", "第二%20章.xhtml#note", false, true},
			{"empty-directory", "../../empty?cleanup=1", "../../empty", false, false},
			{"nonempty-directory", "../../full?", "../../full", false, false},
			{"directory-fragment", "../../empty?x=1#note", "../../empty#note", false, false},
			{"missing-file", "absent.xhtml?x=1", "absent.xhtml", false, false},
			{"literal-percent-file", "../../字%25.bin?", "../../字%25.bin", true, false},
			{"unindexable-fragment", "../../unlisted.bin?x=1#note", "../../unlisted.bin#note", false, false},
		} {
			t.Run(enc+"/"+tc.name, func(t *testing.T) {
				entries := testfixture.EPUB("3.0", false)
				entries = append(entries, testfixture.Entry{Name: "full/child.bin", Data: []byte{0, 255, 13}}, testfixture.Entry{Name: "字%.bin", Data: []byte{42, 0, 255}})
				src := `<?xml version="1.0" encoding="utf-8"?>` + "\r\n" + `<html xmlns="http://www.w3.org/1999/xhtml"><head><title>字😀</title></head><body><a href="` + tc.old + `">目标 &amp; 尾</a></body></html>` + "\r\n"
				want := map[string][]byte{}
				for i := range entries {
					if entries[i].Name == "书/Text/-first.xhtml" {
						entries[i].Data = []byte(encodeChapter(src, enc))
					}
					if tc.duplicate && entries[i].Name == "书/Text/第二 章.xhtml" {
						entries[i].Data = []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>T</title></head><body><p id="note">One</p><p id="note">Two</p></body></html>`)
					}
					if !strings.HasSuffix(entries[i].Name, "/") {
						want[entries[i].Name] = bytes.Clone(entries[i].Data)
					}
				}
				book := filepath.Join(t.TempDir(), "book.epub")
				testfixture.ZIP(t, book, entries)
				original := readResource(t, book)
				w, err := Create(filepath.Join(t.TempDir(), "ws"), book, Options{})
				if err != nil {
					t.Fatal(err)
				}
				defer w.Close()
				baseline, err := HashTree(filepath.Join(w.dir, revision))
				if err != nil {
					t.Fatal(err)
				}
				s, err := w.FixSnapshot()
				if err != nil {
					t.Fatal(err)
				}
				a, tree, err := archive.SnapshotDirectory(filepath.Join(w.dir, revision), archive.DefaultLimits)
				if err != nil {
					t.Fatal(err)
				}
				defer a.Close()
				frozen, err := w.fixSnapshotOfArchive(a, tree, "initial")
				if err != nil || !reflect.DeepEqual(s, frozen) {
					t.Fatalf("snapshot entrypoints disagree: %v", err)
				}
				if s.Inventory["empty"] != "directory" || s.Inventory["full"] != "directory" || s.Inventory["字%.bin"] != "file" || s.Workspace.InputTreeSHA256 != baseline.SHA256 {
					t.Fatal("typed inventory/hash lost", s.Inventory)
				}
				all, err := fix.Propose(s, fix.Selection{Mode: fix.ModeAll})
				if err != nil || len(all.Repairs) != 1 {
					t.Fatalf("applicable fact lost: %+v %v", all, err)
				}
				r := all.Repairs[0]
				p, err := fix.Propose(s, fix.Selection{Mode: fix.ModeExplicit, RepairIDs: []string{r.RepairID}})
				if err != nil || fix.Validate(s, p) != nil || p.DerivedFrom.ProposalSHA256 != all.ProposalSHA256 {
					t.Fatalf("complete selected source: %+v %v", p, err)
				}
				b := structureBinding(t, w, "书/Text/-first.xhtml")
				gate, gateErr := w.Plan(editJSON(t, Request{4, []Operation{b.attrSet(b.locator(t, "a", 0), "href", &tc.old, tc.value)}}))
				if !tc.fixable {
					if r.Status != fix.StatusUnfixable || r.Operation != nil || len(r.WriteSet) != 0 || r.UnfixableReason == "" || len(p.Derived.Operations) != 0 {
						t.Fatalf("unsupported target authorized: %+v", r)
					}
					// Binary fragment targets are outside FR2's XHTML index range;
					// the legacy gate is independently checked for every other case.
					if tc.name != "unindexable-fragment" && gateErr == nil {
						t.Fatal("execution gate allowed non-file or invalid fragment")
					}
				} else {
					if gateErr != nil || r.Status != fix.StatusFixable || r.Operation == nil {
						t.Fatalf("valid file refused: %v %+v", gateErr, r)
					}
					want["书/Text/-first.xhtml"] = []byte(encodeChapter(strings.Replace(src, `href="`+tc.old+`"`, `href="`+tc.value+`"`, 1), enc))
					plan, err := w.Plan(fixSchema7JSON(t, p))
					if err != nil {
						t.Fatal("qualified proposal failed Plan", err)
					}
					for _, plan := range []Plan{gate, plan} {
						e := applyPlan(t, w, plan)
						for name, expected := range want {
							assertBytes(t, filepath.Join(w.dir, candidate, filepath.FromSlash(name)), expected)
						}
						if _, err := w.Reject(e.TaskID); err != nil {
							t.Fatal(err)
						}
					}
				}
				current, err := HashTree(filepath.Join(w.dir, revision))
				if err != nil || !reflect.DeepEqual(current, baseline) {
					t.Fatal("accepted tree changed", err)
				}
				assertBytes(t, book, original)
			})
		}
	}
}

func TestN3SnapshotHashError(t *testing.T) {
	w, dir, original := structureWorkspace(t, nil)
	defer w.Close()
	before := readResource(t, original)
	link := filepath.Join(dir, revision, "unsafe-link")
	if err := os.Symlink(original, link); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(link)
	s, err := w.fixSnapshotAt(revision)
	if err == nil || s.Workspace.InputTreeSHA256 != "" || len(s.Resources) != 0 {
		t.Fatalf("failed HashTree became a complete proposal snapshot: %+v %v", s, err)
	}
	if _, err := w.FixSnapshot(); err == nil {
		t.Fatal("public snapshot hid unsafe inventory")
	}
	assertBytes(t, original, before)
}

func TestN3LegacyTargetSource(t *testing.T) {
	w, dir, original := structureWorkspace(t, map[string]string{"EPUB/chapter1.xhtml": strings.Replace(fixSource, "chapter2.xhtml?q=1#start2", "full?q=1", 1), "EPUB/full/child.bin": "child"})
	defer func() { w.Close() }()
	s, err := w.FixSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	s.TargetVersion = 1
	all, err := fix.Propose(s, fix.Selection{Mode: fix.ModeAll})
	if err != nil || all.Repairs[1].Status != fix.StatusFixable {
		t.Fatal("old directory qualification not reproduced", err)
	}
	p, err := fix.Propose(s, fix.Selection{Mode: fix.ModeExplicit, RepairIDs: []string{all.Repairs[0].RepairID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Plan(fixSchema7JSON(t, p)); err == nil {
		t.Fatal("old derivedFrom was silently reinterpreted")
	} else {
		var fe *fault.Error
		if !errors.As(err, &fe) || fe.Code != "PROPOSAL_DRIFT" {
			t.Fatal("wrong stale source refusal", err)
		}
	}
	op := p.Derived.Operations[0]
	ops := []Operation{{ID: op.ID, Version: op.Version, Params: op.Params}}
	plan, derived := n2LegacyPlan(t, w, ops)
	plan.SchemaVersion, plan.Proposal, plan.PolicySHA256 = 7, &p, digest(fixPolicy+referencePolicy)
	put(t, filepath.Join(dir, "plans", plan.ID+".json"), editJSON(t, plan))
	// Real consumption decodes the stored JSON (including generic proposal
	// params) before hashing intent/used; model that boundary, not typed params.
	if err := readEditJSON(w.root, "plans/"+plan.ID+".json", &plan); err != nil {
		t.Fatal(err)
	}
	if err := w.verifyFixPlan(plan); err != nil {
		t.Fatal("legal old complete source refused", err)
	}
	forged := plan
	clone := *plan.Proposal
	clone.Repairs = append([]fix.Repair{}, clone.Repairs...)
	clone.Repairs[0].Risk = "forged approved safe repair"
	if err := clone.Sign(); err != nil {
		t.Fatal(err)
	}
	forged.Proposal = &clone
	if err := w.verifyFixPlan(forged); !errors.Is(err, ErrStalePlan) {
		t.Fatal("legacy replay skipped complete-source validation", err)
	}
	stored, book := readResource(t, filepath.Join(dir, "plans", plan.ID+".json")), readResource(t, original)
	if _, err := w.Apply(editJSON(t, plan)); !errors.Is(err, ErrStalePlan) {
		t.Fatal("old unconsumed plan authorized", err)
	}
	if _, err := w.createCandidate(&plan); err != nil {
		t.Fatal(err)
	}
	e, err := w.startExecution(plan)
	if err != nil {
		t.Fatal(err)
	}
	e, err = w.execute(e, derived.outputs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.TaskDiff(e.TaskID); err != nil {
		t.Fatal("old source diff failed", err)
	}
	if _, err := w.Accept(t.Context(), e.TaskID, validation.Options{}); !errors.Is(err, ErrTaskConflict) {
		t.Fatal("old source got a new accept", err)
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
	w.Close()
	w, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	status, err := w.TaskStatus(e.TaskID)
	if err != nil || status.Status != "rejected" {
		t.Fatalf("legacy complete-source history lost: %+v %v", status, err)
	}
	assertBytes(t, filepath.Join(dir, "plans", plan.ID+".json"), stored)
	assertBytes(t, original, book)
}

// TestFixReadOnlyQualification pins that a read-only workspace never claims an
// executable repair the legacy gates refuse, while the editable control keeps
// both repairs executable and generated markup stays explicitly unfixable.
func TestFixReadOnlyQualification(t *testing.T) {
	for _, signed := range []bool{false, true} {
		t.Run(map[bool]string{false: "editable-control", true: "signed-readonly"}[signed], func(t *testing.T) {
			extra := map[string]string{"EPUB/chapter1.xhtml": fixSource}
			if signed {
				extra["META-INF/signatures.xml"] = `<signatures xmlns="urn:oasis:names:tc:opendocument:xmlns:container"/>`
			}
			w, _, _ := structureWorkspace(t, extra)
			defer w.Close()
			s, err := w.FixSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			p, err := fix.Propose(s, fix.Selection{Mode: fix.ModeAll, RepairIDs: []string{}})
			if err != nil {
				t.Fatal(err)
			}
			if !signed {
				if len(p.Derived.Operations) != 2 {
					t.Fatalf("editable control: %+v", p.Derived)
				}
				return
			}
			if len(w.State().ReadOnlyReasons) == 0 {
				t.Fatal("fixture is not read-only")
			}
			if len(p.Derived.Operations) != 0 || len(p.Repairs) != 2 {
				t.Fatalf("read-only proposal: operations=%d repairs=%+v", len(p.Derived.Operations), p.Repairs)
			}
			for _, r := range p.Repairs {
				if r.Status != fix.StatusUnfixable || r.Operation != nil || len(r.WriteSet) != 0 || r.UnfixableReason == "" {
					t.Fatalf("read-only repair: %+v", r)
				}
			}
			if _, err := w.Plan(editJSON(t, map[string]any{"schemaVersion": 7, "proposal": p, "operations": []fix.Operation{}})); !errors.Is(err, ErrReadOnly) {
				t.Fatalf("legacy read-only gate: %v", err)
			}
		})
	}
	for _, node := range []string{`<title epub:type="secrecy">Generated</title>`, `<a href="chapter2.xhtml?q=1#start2">Generated</a>`} {
		t.Run("generated-source", func(t *testing.T) {
			src := `<!DOCTYPE html [<!ENTITY node '` + node + `'>]><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>One</title></head><body><p id="start">One.</p>&node;</body></html>`
			w, _, _ := structureWorkspace(t, map[string]string{"EPUB/chapter1.xhtml": src})
			defer w.Close()
			s, err := w.FixSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			p, err := fix.Propose(s, fix.Selection{Mode: fix.ModeAll, RepairIDs: []string{}})
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Derived.Operations) != 0 {
				t.Fatal("generated markup acquired write authority")
			}
			if len(p.Repairs) != 1 || p.Repairs[0].Status != fix.StatusUnfixable || p.Repairs[0].Operation != nil {
				t.Fatalf("generated fact: %+v", p.Repairs)
			}
		})
	}
}
