package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/validation"
)

func nativeDefinition() NewBook {
	return NewBook{1, "中文 & 😀", "作者", "zh-Hans", "urn:uuid:93b80bc0-a0d1-4a7b-a121-32b4478db8ec", "2026-10-13T00:00:00Z", []NewChapter{{"第三章", []string{"原文 < & > 😀"}}, {"第一章", []string{"不同正文"}}, {"第一章", []string{"重复标题"}}}}
}

func TestNativeDefinitionInput(t *testing.T) {
	b, err := json.Marshal(nativeDefinition())
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "definition.json")
	for _, tc := range []struct {
		name, raw string
		valid     bool
	}{
		{"literal", string(b), true},
		{"surrogate-pair", strings.ReplaceAll(string(b), "😀", `\ud83d\ude00`), true},
		{"literal-backslash", strings.ReplaceAll(string(b), "😀", `\\ud800`), true},
		{"replacement-character", strings.ReplaceAll(string(b), "😀", "�"), true},
		{"unpaired-high", strings.ReplaceAll(string(b), "😀", `\ud800`), false},
		{"unpaired-low", strings.ReplaceAll(string(b), "😀", `\udc00`), false},
		{"bad-utf8", strings.ReplaceAll(string(b), "😀", "\xff"), false},
		{"xml-control", strings.ReplaceAll(string(b), "😀", `\u0000`), false},
		{"unknown", strings.Replace(string(b), `"schemaVersion":1`, `"extra":1,"schemaVersion":1`, 1), false},
		{"duplicate", strings.Replace(string(b), `"schemaVersion":1`, `"schemaVersion":1,"schemaVersion":1`, 1), false},
		{"missing", strings.Replace(string(b), `"schemaVersion":1,`, "", 1), false},
		{"null", strings.Replace(string(b), `"creator":"作者"`, `"creator":null`, 1), false},
		{"language-underscore", strings.Replace(string(b), "zh-Hans", "zh_Hans", 1), false},
		{"invalid-timestamp", strings.Replace(string(b), "2026-10-13T00:00:00Z", "2026-02-30T00:00:00Z", 1), false},
		{"trailing", string(b) + "{}", false},
		{"limit", string(b) + strings.Repeat(" ", definitionLimit), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			put(t, file, []byte(tc.raw))
			_, err := ReadNewBook(file)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
		})
	}
}

func TestNativeInitializationRefusals(t *testing.T) {
	parent := t.TempDir()
	ws := filepath.Join(parent, "ws")
	b := nativeDefinition()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := CreateNew(ctx, ws, b, validation.Options{})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	_, err = CreateNew(context.Background(), ws, b, validation.Options{JAR: filepath.Join(parent, "missing.jar")})
	if err == nil {
		t.Fatal("missing checker permitted native initialization")
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("partial initialization: %v %v", entries, err)
	}
	if err := os.Mkdir(ws, 0700); err != nil {
		t.Fatal(err)
	}
	_, err = CreateNew(context.Background(), ws, b, validation.Options{})
	if !errors.Is(err, os.ErrExist) {
		t.Fatal(err)
	}
}

func TestNativeSourceForbiddenInImportedState(t *testing.T) {
	w, dir := makeWorkspace(t)
	w.Close()
	file := filepath.Join(dir, "state.json")
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"source":null`, `"Source":null`, `"source":{}`} {
		bad := append([]byte("{"+field+","), raw[1:]...)
		put(t, file, bad)
		opened, err := Open(dir)
		if err == nil {
			opened.Close()
			t.Fatalf("imported state accepted %s", field)
		}
	}
	put(t, file, raw)
	opened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	opened.Close()
}

func TestNativeSourceAndReopen(t *testing.T) {
	requireChecker(t)
	dir := filepath.Join(t.TempDir(), "native")
	w, err := CreateNew(context.Background(), dir, nativeDefinition(), validation.Options{})
	if err != nil {
		t.Fatal(err)
	}
	state := w.State()
	if state.Version != 2 || state.OriginalSHA256 != "" || state.Source.Validation.Status != "pass" {
		t.Fatal(state)
	}
	if _, err := os.Lstat(filepath.Join(dir, "original")); !os.IsNotExist(err) {
		t.Fatal("fake original", err)
	}
	state.Source.Definition.Chapters[0].Paragraphs[0] = "mutated public alias"
	state.Source.Validation.Checks[0].Status = "failed"
	if a, _, r, err := w.AcceptedSnapshot(); err != nil {
		t.Fatal(err)
	} else {
		a.Close()
		if r.ID != "initial" {
			t.Fatal(r)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	w, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	state = w.State()
	for _, mutate := range []func(*State){
		func(s *State) {
			s.Source.Definition.Chapters[0].Paragraphs[0] = "changed"
			s.Source.DefinitionSHA256 = digest(s.Source.Definition)
		},
		func(s *State) { s.Source.TemplateVersion = 2 },
		func(s *State) { s.OriginalSHA256 = strings.Repeat("a", 64) },
		func(s *State) { s.Rootfile = "EPUB/nav.xhtml" },
		func(s *State) { s.Source.Validation.InputTreeSHA256 = strings.Repeat("a", 64) },
		func(s *State) { s.Version = 1 },
	} {
		var bad State
		raw, _ := json.Marshal(state)
		if err := json.Unmarshal(raw, &bad); err != nil {
			t.Fatal(err)
		}
		mutate(&bad)
		raw, _ = json.Marshal(bad)
		put(t, filepath.Join(dir, "state.json"), raw)
		opened, err := Open(dir)
		if err == nil {
			opened.Close()
			t.Fatal("tampered native source opened")
		}
	}
	raw, _ := json.Marshal(state)
	put(t, filepath.Join(dir, "state.json"), raw)
	w, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
}

func TestNativeInterruptedEditRecovery(t *testing.T) {
	requireChecker(t)
	dir := filepath.Join(t.TempDir(), "native")
	w, err := CreateNew(context.Background(), dir, nativeDefinition(), validation.Options{})
	if err != nil {
		t.Fatal(err)
	}
	baseline := readResource(t, filepath.Join(dir, revision, "EPUB/package.opf"))
	p := fieldPlan(t, w, "title", "title", "中文 & 😀", "候选")
	e, outputs := prepareExecution(t, w, p)
	putOutputs(t, dir, outputs) // Registered running execution, no result.
	w.Close()
	w, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if got := readResource(t, filepath.Join(dir, candidate, "EPUB/package.opf")); string(got) != string(baseline) {
		t.Fatal("native checkpoint was not restored")
	}
	result, err := w.Execution()
	if err != nil || result.Status != "failed" || result.ReviewRequired {
		t.Fatalf("%+v %v", result, err)
	}
	if _, err := w.Accept(context.Background(), e.TaskID, validation.Options{}); err == nil {
		t.Fatal("interrupted native edit accepted")
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
}
