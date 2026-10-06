package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/workspace"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

// Opt-in only: no private attachment, source text or receipt is in the repo.
func TestT1AuthorizedOriginalLifecycle(t *testing.T) {
	book := os.Getenv("KEPUB_T1_AUTHORIZED_ORIGINAL")
	if book == "" {
		t.Skip("explicit authorized original-book path required")
	}
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Fatal("real pinned checker required")
	}
	original, err := os.ReadFile(book)
	if err != nil {
		t.Fatal(err)
	}
	if len(original) != 3328634 || fmt.Sprintf("%x", sha256.Sum256(original)) != "91b9d80c84258c89f47f6faac43eff6477b7c6649140ac848b3dc78924761e4b" {
		t.Fatal("not the authorized unmodified attachment")
	}
	a, err := archive.Open(book, archive.DefaultLimits)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	p, err := publication.Load(a, "")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for bp := range a.Files {
		b, err := a.Read(bp, archive.DefaultLimits.FileBytes)
		if err != nil {
			t.Fatal(err)
		}
		files[string(bp)] = b
	}
	var selected publication.Content
	var target publication.ContentNode
	found := false
	for _, item := range p.Manifest {
		if item.MediaType != "application/xhtml+xml" {
			continue
		}
		c, err := publication.ReadContent(a, p, item.Path, publication.ContentOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range c.Nodes {
			if n.LocalName != "p" || n.HasChildElements || n.Text == "" {
				continue
			}
			if text, err := publication.ContentText(files[string(item.Path)], n.Locator, xmltext.Profile{Version: p.Version, MediaType: item.MediaType}); err == nil && text == n.Text {
				selected, target, found = c, n, true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("authorized original has no supported simple paragraph; no preprocessing allowed")
	}
	binary := workspaceBinary(t)
	call := func(args []string, exit int) map[string]any {
		return processJSON(t, binary, args, exit, 120*time.Second)
	}
	dir := t.TempDir()
	ws := filepath.Join(dir, "ws")
	call([]string{"workspace", "open", book, "--output", ws}, 0)
	queryRunes := []rune(target.Text)
	query := string(queryRunes[:min(12, len(queryRunes))])
	result := call([]string{"search", "--workspace", ws, "--query", query, "--limit", "1"}, 0)["data"].(map[string]any)
	if result["matchedCount"].(float64) < 1 || result["revisionId"] != "initial" {
		t.Fatal("original search did not use initial")
	}
	newValue := target.Text + "（T1a测试）"
	for pass := 0; pass < 2; pass++ {
		request := workspace.Request{SchemaVersion: 2, Operations: []workspace.Operation{{ID: "content.text.set", Version: 1, Params: publication.TextSet{BookPath: selected.BookPath, RevisionID: "initial", ResourceSHA256: selected.ResourceSHA256, LocatorVersion: 1, Locator: target.Locator, ExpectedOldValue: target.Text, NewValue: newValue}}}}
		b, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		ops, plan := filepath.Join(dir, fmt.Sprintf("ops%d.json", pass)), filepath.Join(dir, fmt.Sprintf("plan%d.json", pass))
		if err := os.WriteFile(ops, b, 0600); err != nil {
			t.Fatal(err)
		}
		call([]string{"plan", "--workspace", ws, "--operations", ops, "--output", plan}, 0)
		e := call([]string{"apply", "--workspace", ws, "--plan", plan}, 0)["data"].(map[string]any)
		task := e["taskId"].(string)
		review := call([]string{"task", "diff", task, "--workspace", ws}, 0)["data"].(map[string]any)
		if review["matchesExecution"] != true || len(review["diff"].(map[string]any)["changes"].([]any)) != 1 {
			t.Fatal("original candidate has unrelated byte changes")
		}
		if pass == 0 {
			call([]string{"task", "reject", task, "--workspace", ws}, 0)
			status := call([]string{"task", "status", task, "--workspace", ws}, 0)["data"].(map[string]any)
			if status["status"] != "rejected" || status["currentRevision"] != "initial" {
				t.Fatal("original rejection changed accepted")
			}
			continue
		}
		decision := call([]string{"task", "accept", task, "--workspace", ws}, 0)["data"].(map[string]any)
		status := call([]string{"task", "status", task, "--workspace", ws}, 0)["data"].(map[string]any)
		if status["status"] != "accepted" || status["currentRevision"] != decision["revisionId"] {
			t.Fatal("original accepted history lost on reopen")
		}
		output := filepath.Join(dir, "formal.epub")
		packed := call([]string{"workspace", "export", ws, "--output", output}, 0)["data"].(map[string]any)
		if packed["verified"] != true || packed["draft"] != false {
			t.Fatal("original formal export not verified")
		}
		input := files[string(selected.BookPath)]
		doc, err := xmltext.Parse(input)
		if err != nil {
			t.Fatal(err)
		}
		var element *xmltext.Element
		for _, e := range doc.Elements {
			if e.Location == target.Locator {
				element = e
				break
			}
		}
		if element == nil {
			t.Fatal("source interval missing")
		}
		var escaped bytes.Buffer
		if err := xml.EscapeText(&escaped, []byte(newValue)); err != nil {
			t.Fatal(err)
		}
		want := append(bytes.Clone(input[:element.Start]), escaped.Bytes()...)
		want = append(want, input[element.End:]...)
		files[string(selected.BookPath)] = want
		assertExportFiles(t, output, files)
	}
	got, err := os.ReadFile(book)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatal("authorized original archive changed")
	}
	t.Log("unmodified attachment SHA verified; literal search, edit, reject, accept, historical reopen, formal export and per-resource byte comparison completed")
}
