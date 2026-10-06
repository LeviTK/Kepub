package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/testfixture"
	"github.com/LeviTK/Kepub/internal/workspace"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

func encodingBook(t *testing.T, bigEndian, xhtml, large bool) (string, map[string][]byte) {
	t.Helper()
	_, files := legalCLI(t, "3.0")
	for name, b := range files {
		if strings.HasSuffix(name, ".xhtml") {
			files[name] = append([]byte(`<!DOCTYPE html>`), b...)
		}
	}
	for _, name := range []string{"META-INF/container.xml", "EPUB/package.opf"} {
		s := strings.TrimPrefix(string(files[name]), `<?xml version="1.0"?>`)
		files[name] = testfixture.UTF16(`<?xml version="1.0" encoding="UTF-16"?>`+s, bigEndian, true)
	}
	if xhtml {
		s := string(files["EPUB/chapter.xhtml"])
		if large {
			s = strings.Replace(s, "<body>", "<body><!--"+strings.Repeat("中", 3<<20)+"-->", 1)
		}
		files["EPUB/chapter.xhtml"] = testfixture.UTF16(`<?xml version="1.0" encoding="UTF-16"?>`+s, bigEndian, true)
	}
	book := filepath.Join(t.TempDir(), "encoded.epub")
	names := []string{}
	for name := range files {
		if name != "mimetype" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	entries := []testfixture.Entry{{Name: "mimetype", Data: files["mimetype"]}}
	for _, name := range names {
		entries = append(entries, testfixture.Entry{Name: name, Data: files[name]})
	}
	testfixture.ZIP(t, book, entries)
	return book, files
}

func assertExportFiles(t *testing.T, file string, want map[string][]byte) {
	t.Helper()
	a, err := archive.Open(file, archive.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if len(a.Files) != len(want) {
		t.Fatal("export inventory mismatch")
	}
	for name, b := range want {
		got, err := a.Read(bookpath.BookPath(name), archive.DefaultLimits.FileBytes)
		if err != nil || !bytes.Equal(got, b) {
			t.Fatal("export original-byte mismatch", name, err)
		}
	}
}

func TestT1EncodingMismatchVersusHTMLConstraint(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("real pinned checker required")
	}
	for _, be := range []bool{false, true} {
		t.Run(map[bool]string{false: "LE", true: "BE"}[be], func(t *testing.T) {
			book, files := encodingBook(t, be, true, false)
			if _, err := xmltext.Parse(files["EPUB/chapter.xhtml"]); err != nil {
				t.Fatal("matching BOM/declaration must be well-formed XML", err)
			}
			r := invoke(t, []string{"validate", book, "--json"}, 1)
			b, _ := json.Marshal(r)
			if !bytes.Contains(b, []byte("HTM_058")) || bytes.Contains(b, []byte("RSC-016")) {
				t.Fatal("matching encoding must fail HTML constraint, not XML parsing", string(b))
			}
			t.Log("matched BOM/declaration:", string(b))
			decl := "UTF-16BE"
			if be {
				decl = "UTF-16LE"
			}
			files["EPUB/chapter.xhtml"] = bytes.Replace(files["EPUB/chapter.xhtml"], testfixture.UTF16("UTF-16", be, false), testfixture.UTF16(decl, be, false), 1)
			if _, err := xmltext.Parse(files["EPUB/chapter.xhtml"]); err == nil {
				t.Fatal("mismatched declaration accepted by core")
			}
			entries := []testfixture.Entry{{Name: "mimetype", Data: files["mimetype"]}}
			for name, data := range files {
				if name != "mimetype" {
					entries = append(entries, testfixture.Entry{Name: name, Data: data})
				}
			}
			bad := filepath.Join(t.TempDir(), "mismatch.epub")
			testfixture.ZIP(t, bad, entries)
			r = invoke(t, []string{"validate", bad, "--json"}, 1)
			b, _ = json.Marshal(r)
			if !bytes.Contains(b, []byte("RSC-016")) || !bytes.Contains(b, []byte("XML_NOT_WELL_FORMED")) {
				t.Fatal("checker must also report XML-level failure", string(b))
			}
			t.Log("mismatched BOM/declaration:", string(b))
		})
	}
}

func TestT1EncodedFormalLifecycle(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("real pinned checker required")
	}
	for _, be := range []bool{false, true} {
		for _, mode := range []string{"metadata-positive", "xhtml-negative", "large-comment-negative"} {
			t.Run(map[bool]string{false: "LE", true: "BE"}[be]+"/"+mode, func(t *testing.T) {
				xhtml := mode != "metadata-positive"
				book, files := encodingBook(t, be, xhtml, mode == "large-comment-negative")
				original, err := os.ReadFile(book)
				if err != nil {
					t.Fatal(err)
				}
				dir := t.TempDir()
				ws := filepath.Join(dir, "ws")
				call := func(args []string, exit int) map[string]any { return invoke(t, append(args, "--json"), exit) }
				call([]string{"workspace", "open", book, "--output", ws}, 0)
				// Search/read still operate on original UTF-16 bytes and accepted.
				read := call([]string{"content", "--workspace", ws, "--resource", "EPUB/chapter.xhtml", "--query", "precise"}, 0)["data"].(map[string]any)
				if read["matchedCount"] != float64(1) {
					t.Fatal("content match count")
				}
				search := call([]string{"search", "--workspace", ws, "--query", "precise"}, 0)["data"].(map[string]any)
				if search["matchedCount"] != float64(1) || search["results"].([]any)[0].(map[string]any)["resourceSha256"] != read["resourceSha256"] {
					t.Fatal("search original binding")
				}
				ops, plan := filepath.Join(dir, "ops.json"), filepath.Join(dir, "plan.json")
				changedPath := "EPUB/package.opf"
				newValue := "Reviewed 😀 & 中文"
				if !xhtml {
					operationFile(t, ops, "title", "Title", newValue)
				} else {
					changedPath = "EPUB/chapter.xhtml"
					n := read["nodes"].([]any)[0].(map[string]any)
					request := workspace.Request{SchemaVersion: 2, Operations: []workspace.Operation{{ID: "content.text.set", Version: 1, Params: publication.TextSet{BookPath: bookpath.BookPath(changedPath), RevisionID: read["revisionId"].(string), ResourceSHA256: read["resourceSha256"].(string), LocatorVersion: 1, Locator: n["locator"].(string), ExpectedOldValue: n["text"].(string), NewValue: newValue}}}}
					b, err := json.Marshal(request)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(ops, b, 0600); err != nil {
						t.Fatal(err)
					}
				}
				call([]string{"plan", "--workspace", ws, "--operations", ops, "--output", plan}, 0)
				e := call([]string{"apply", "--workspace", ws, "--plan", plan}, 0)["data"].(map[string]any)
				task := e["taskId"].(string)
				oldToken, newToken := "Title", `Reviewed 😀 &amp; 中文`
				if xhtml {
					oldToken = "Original &amp; precise."
				}
				want := bytes.Replace(files[changedPath], testfixture.UTF16(">"+oldToken+"</", be, false), testfixture.UTF16(">"+newToken+"</", be, false), 1)
				got, err := os.ReadFile(filepath.Join(ws, "tasks/active/work/pub", changedPath))
				if err != nil || bytes.Equal(want, files[changedPath]) || !bytes.Equal(got, want) {
					t.Fatal("not exact encoded local edit", err)
				}
				// Each CLI call reopens and rederives the persisted execution.
				review := call([]string{"task", "diff", task, "--workspace", ws}, 0)["data"].(map[string]any)
				if review["matchesExecution"] != true {
					t.Fatal("reopen/recomputation mismatch")
				}
				if xhtml {
					r := call([]string{"task", "accept", task, "--workspace", ws}, 1)
					b, _ := json.Marshal(r)
					if !bytes.Contains(b, []byte("HTM_058")) {
						t.Fatal("must retain actual checker rejection")
					}
					status := call([]string{"task", "status", task, "--workspace", ws}, 0)["data"].(map[string]any)
					if status["currentRevision"] != "initial" || status["status"] != "review_required" || len(status["checks"].([]any)) != 1 {
						t.Fatal("failed check advanced accepted or disappeared", status)
					}
					output := filepath.Join(dir, "formal.epub")
					call([]string{"workspace", "export", ws, "--output", output}, 1)
					if _, err := os.Stat(output); !os.IsNotExist(err) {
						t.Fatal("formal failed output exists")
					}
					draft := filepath.Join(dir, "draft.epub")
					d := call([]string{"workspace", "export", ws, "--output", draft, "--draft"}, 0)["data"].(map[string]any)
					if d["verified"] != false || d["draft"] != true || d["revisionId"] != "initial" {
						t.Fatal("draft not explicit accepted-only")
					}
					assertExportFiles(t, draft, files)
					// Interrupted execution is rolled back exactly, not rerun. The
					// failed task can then be rejected with its original identity.
					if err := os.Remove(filepath.Join(ws, "tasks/active/edit-result.json")); err != nil {
						t.Fatal(err)
					}
					status = call([]string{"task", "status", task, "--workspace", ws}, 0)["data"].(map[string]any)
					if status["status"] != "failed" {
						t.Fatal("missing-result recovery did not fail")
					}
					got, err := os.ReadFile(filepath.Join(ws, "tasks/active/work/pub", changedPath))
					if err != nil || !bytes.Equal(got, files[changedPath]) {
						t.Fatal("rollback original bytes", err)
					}
					call([]string{"task", "reject", task, "--workspace", ws}, 0)
				} else {
					call([]string{"task", "accept", task, "--workspace", ws, "--strict"}, 1)
					decision := call([]string{"task", "accept", task, "--workspace", ws}, 0)["data"].(map[string]any)
					status := call([]string{"task", "status", task, "--workspace", ws}, 0)["data"].(map[string]any)
					if status["status"] != "accepted" || status["currentRevision"] != decision["revisionId"] {
						t.Fatal("accepted history missing")
					}
					strict := filepath.Join(dir, "strict.epub")
					call([]string{"workspace", "export", ws, "--output", strict, "--strict"}, 1)
					if _, err := os.Stat(strict); !os.IsNotExist(err) {
						t.Fatal("strict output exists")
					}
					output := filepath.Join(dir, "formal.epub")
					call([]string{"workspace", "export", ws, "--output", output}, 0)
					files[changedPath] = want
					assertExportFiles(t, output, files)
				}
				got, err = os.ReadFile(book)
				if err != nil || !bytes.Equal(got, original) {
					t.Fatal("original archive mutated", err)
				}
			})
		}
	}
}
