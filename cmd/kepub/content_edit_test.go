package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/workspace"
)

func TestContentEditBinaryLifecycle(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("real pinned EPUBCheck required")
	}
	binary := workspaceBinary(t)
	for _, version := range []string{"2.0", "3.0"} {
		t.Run(version, func(t *testing.T) {
			book, files := legalCLI(t, version)
			original, err := os.ReadFile(book)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			ws := filepath.Join(dir, "ws")
			ops := filepath.Join(dir, "ops.json")
			plan := filepath.Join(dir, "plan.json")
			run := func(args []string, want int) map[string]any {
				return processJSON(t, binary, args, want, 45*time.Second)
			}
			run([]string{"workspace", "open", book, "--output", ws}, 0)
			read := func() map[string]any {
				return run([]string{"content", "--workspace", ws, "--resource", "EPUB/chapter.xhtml", "--query", "precise"}, 0)["data"].(map[string]any)
			}
			initial := read()
			params := func(c map[string]any, new string) map[string]any {
				for _, node := range c["nodes"].([]any) {
					n := node.(map[string]any)
					if n["localName"] == "p" {
						return map[string]any{"bookPath": c["bookPath"], "revisionId": c["revisionId"], "resourceSha256": c["resourceSha256"], "locatorVersion": c["locatorVersion"], "locator": n["locator"], "expectedOldValue": n["text"], "newValue": new}
					}
				}
				t.Fatal("missing p", c)
				return nil
			}
			writeRequest := func(p map[string]any) {
				t.Helper()
				b, err := json.Marshal(map[string]any{"schemaVersion": 2, "operations": []any{map[string]any{"operationId": "content.text.set", "operationVersion": 1, "params": p}}})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(ops, b, 0600); err != nil {
					t.Fatal(err)
				}
			}
			for _, tc := range []struct {
				field string
				value any
				exit  int
				code  string
			}{
				{"expectedOldValue", "wrong", 2, "INVALID_OPERATIONS"}, {"locator", "/html[1]/head[1]/title[1]", 2, "INVALID_OPERATIONS"},
				{"locatorVersion", 2, 2, "INVALID_OPERATIONS"}, {"newValue", "\x00", 2, "INVALID_OPERATIONS"},
				{"newValue", strings.Repeat("a", (1<<20)+1), 2, "INVALID_OPERATIONS"},
				{"resourceSha256", strings.Repeat("0", 64), 4, "INPUT_DRIFT"}, {"revisionId", strings.Repeat("a", 32), 4, "INPUT_DRIFT"},
			} {
				p := params(initial, "new")
				p[tc.field] = tc.value
				writeRequest(p)
				r := run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", plan}, tc.exit)
				if r["error"].(map[string]any)["code"] != tc.code {
					t.Fatal(r)
				}
				if _, err := os.Stat(plan); !os.IsNotExist(err) {
					t.Fatal("failed plan published output", err)
				}
			}
			new := "New precise < & > 😀\r\n"
			writeRequest(params(initial, new))
			p := run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", plan}, 0)["data"].(map[string]any)
			if p["schemaVersion"] != float64(2) || !reflect.DeepEqual(p["writeSet"], []any{"EPUB/chapter.xhtml"}) {
				t.Fatal(p)
			}
			locked, err := workspace.Open(ws)
			if err != nil {
				t.Fatal(err)
			}
			run([]string{"apply", "--workspace", ws, "--plan", plan}, 4)
			locked.Close()
			e := run([]string{"apply", "--workspace", ws, "--plan", plan}, 0)["data"].(map[string]any)
			task := e["taskId"].(string)
			if e["version"] != float64(2) || e["reviewRequired"] != true || e["conformance"] != "not_run" {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(read(), initial) {
				t.Fatal("candidate leaked into content")
			}
			review := run([]string{"task", "diff", task, "--workspace", ws}, 0)["data"].(map[string]any)
			c := review["content"].(map[string]any)
			if review["matchesExecution"] != true || c["newValue"] != new || c["oldValue"] != "Original & precise." {
				t.Fatal(review)
			}
			checker := os.Getenv("KEPUB_EPUBCHECK_JAR")
			t.Setenv("KEPUB_EPUBCHECK_JAR", "missing-checker")
			r := run([]string{"task", "accept", task, "--workspace", ws}, 3)
			if r["error"].(map[string]any)["code"] != "DEPENDENCY_UNAVAILABLE" {
				t.Fatal(r)
			}
			if !reflect.DeepEqual(read(), initial) {
				t.Fatal("failed acceptance advanced baseline")
			}
			t.Setenv("KEPUB_EPUBCHECK_JAR", checker)
			accepted := run([]string{"task", "accept", task, "--workspace", ws}, 0)["data"].(map[string]any)
			if accepted["validation"].(map[string]any)["status"] != "pass" {
				t.Fatal(accepted)
			}
			after := read()
			if after["revisionId"] != accepted["revisionId"] || after["resourceSha256"] == initial["resourceSha256"] || after["nodes"].([]any)[0].(map[string]any)["text"] != new {
				t.Fatal(after)
			}
			writeRequest(params(initial, "stale"))
			run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", filepath.Join(dir, "stale.json")}, 4)
			files["EPUB/chapter.xhtml"] = bytes.Replace(files["EPUB/chapter.xhtml"], []byte(`>Original &amp; precise.</p>`), []byte(`>New precise &lt; &amp; &gt; 😀&#xD;&#xA;</p>`), 1)
			out := filepath.Join(dir, "final.epub")
			exported := run([]string{"workspace", "export", ws, "--output", out}, 0)["data"].(map[string]any)
			if exported["verified"] != true || exported["revisionId"] != accepted["revisionId"] {
				t.Fatal(exported)
			}
			checkExport(t, out, files)
			writeRequest(params(after, new))
			noop := filepath.Join(dir, "noop.json")
			np := run([]string{"plan", "--workspace", ws, "--operations", ops, "--output", noop}, 0)["data"].(map[string]any)
			if !reflect.DeepEqual(np["writeSet"], []any{}) {
				t.Fatal("no-op writeSet", np)
			}
			ne := run([]string{"apply", "--workspace", ws, "--plan", noop}, 0)["data"].(map[string]any)
			if ne["diff"].(map[string]any)["changed"] != false {
				t.Fatal("no-op changed bytes")
			}
			// Same resource path but wrong bytes: diff must show the actual text,
			// acceptance must refuse, and rejection must preserve accepted bytes.
			candidate := filepath.Join(ws, "tasks/active/work/pub/EPUB/chapter.xhtml")
			if err := os.WriteFile(candidate, bytes.Replace(files["EPUB/chapter.xhtml"], []byte(`>New precise &lt; &amp; &gt; 😀&#xD;&#xA;</p>`), []byte(`>Actual tamper</p>`), 1), 0600); err != nil {
				t.Fatal(err)
			}
			nid := ne["taskId"].(string)
			actual := run([]string{"task", "diff", nid, "--workspace", ws}, 0)["data"].(map[string]any)
			if actual["matchesExecution"] != false || actual["content"].(map[string]any)["newValue"] != "Actual tamper" {
				t.Fatal(actual)
			}
			run([]string{"task", "accept", nid, "--workspace", ws}, 4)
			run([]string{"task", "reject", nid, "--workspace", ws}, 0)
			if !reflect.DeepEqual(read(), after) {
				t.Fatal("rejection changed accepted")
			}
			run([]string{"apply", "--workspace", ws, "--plan", noop}, 4)
			got, err := os.ReadFile(book)
			if err != nil || !bytes.Equal(got, original) {
				t.Fatal("original bytes changed", err)
			}
		})
	}
}

func TestContentEditCapability(t *testing.T) {
	all := invoke(t, []string{"capabilities", "--json"}, 0)["data"].([]any)
	for _, v := range all {
		c := v.(map[string]any)
		if c["operationId"] != "content.text.set" {
			continue
		}
		if c["implementationStatus"] != "available" || c["operationVersion"] != float64(1) || c["risk"] != "bounded_edit" || c["mutatesPublication"] != true || c["requiresModel"] != false || c["requiresGUI"] != false {
			t.Fatal(c)
		}
		s := c["inputSchema"].(map[string]any)
		want := []any{"bookPath", "revisionId", "resourceSha256", "locatorVersion", "locator", "expectedOldValue", "newValue"}
		if !reflect.DeepEqual(s["required"], want) || s["additionalProperties"] != false {
			t.Fatal(s)
		}
		return
	}
	t.Fatal("missing content edit capability")
}
