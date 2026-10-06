package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/app"
	"github.com/LeviTK/Kepub/internal/workspace"
)

func TestT1HumanSummariesAndTerminalSafety(t *testing.T) {
	for _, tc := range []struct {
		command string
		data    any
		want    string
	}{
		{"metadata", map[string]any{"title": "T\x1b[2J\n中文"}, `T\x1b[2J\n中文`},
		{"search", map[string]any{"matchedCount": 12, "results": make([]string, 12)}, "2 more omitted; use --json"},
		{"capabilities", []app.Capability{{ID: "content.text.set", Version: 1, Status: "supported"}}, "content.text.set v1"},
	} {
		out, err := humanSummary(tc.command, tc.data)
		if err != nil || !strings.Contains(out, tc.want) || strings.Contains(out, "\x1b") || strings.HasPrefix(strings.TrimSpace(out), "{") {
			t.Fatal(tc.command, out, err)
		}
	}
	var out, errs bytes.Buffer
	if exit := run([]string{"--help"}, &out, &errs); exit != 0 || !strings.Contains(out.String(), "search") || !strings.Contains(out.String(), "task status") {
		t.Fatal("human command discovery", exit, out.String(), errs.String())
	}
}

func TestSearchManifestOrderAndLateFailure(t *testing.T) {
	dir, _, id, _ := cliContentWorkspace(t, `<!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml"><body><p>First First</p><p>First</p></body></html>`)
	w, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := w.NewCandidate()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(candidate, cliContentPath), []byte("invalid candidate"), 0600); err != nil {
		t.Fatal(err)
	}
	w.Close()
	args := []string{"search", "--workspace", dir, "--query", "First", "--limit", "1", "--json"}
	d := invoke(t, args, 0)["data"].(map[string]any)
	if d["workspaceId"] != id || d["matchedCount"] != float64(2) || d["returnedCount"] != float64(1) || d["truncated"] != true {
		t.Fatal(d)
	}
	if d["results"].([]any)[0].(map[string]any)["bookPath"] != cliContentPath {
		t.Fatal("manifest order", d)
	}
	// Independently of parsing/counting, immutable source drift must also fail
	// through the existing Open provenance check, not produce search results.
	if err := os.WriteFile(filepath.Join(dir, "revisions/initial/pub/书/Text/-first.xhtml"), []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	invoke(t, args, 6)
	for _, a := range [][]string{{"search", "--workspace", dir, "--json"}, {"search", "--workspace", dir, "--query", "x", "--resource", cliContentPath, "--json"}, {"search", "--workspace", dir, "--query", "x", "--rootfile", "alternate.opf", "--json"}} {
		invoke(t, a, 2)
	}
}

func TestTaskStatusExactAndHumanDiffActual(t *testing.T) {
	dir, _, _, _ := cliContentWorkspace(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>old</p></body></html>`)
	w, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.NewCandidate()
	if err != nil {
		t.Fatal(err)
	}
	id, err := w.TaskID()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "tasks/active/work/pub", cliContentPath)
	if err := os.WriteFile(file, []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p>actual not planned</p></body></html>`), 0600); err != nil {
		t.Fatal(err)
	}
	w.Close()
	status := []string{"task", "status", id, "--workspace", dir, "--json"}
	d := invoke(t, status, 0)["data"].(map[string]any)
	if d["taskId"] != id || d["baseRevision"] != "initial" || d["status"] != "pending" {
		t.Fatal(d)
	}
	invoke(t, []string{"task", "status", strings.Repeat("0", 32), "--workspace", dir, "--json"}, 4)
	var out, errs bytes.Buffer
	if n := run([]string{"task", "diff", id, "--workspace", dir}, &out, &errs); n != 0 {
		t.Fatal(n, errs.String())
	}
	if !strings.Contains(out.String(), "-<html") || !strings.Contains(out.String(), "+<html") || !strings.Contains(out.String(), "actual not planned") || strings.HasPrefix(strings.TrimSpace(out.String()), "{") {
		t.Fatal("not actual per-file text diff", out.String())
	}
	invoke(t, []string{"task", "reject", id, "--workspace", dir, "--json"}, 0)
	d = invoke(t, status, 0)["data"].(map[string]any)
	if d["status"] != "rejected" || d["taskId"] != id {
		t.Fatal(d)
	}
}
