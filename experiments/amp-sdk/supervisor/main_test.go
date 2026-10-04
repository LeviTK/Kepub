package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoundedJSONL(t *testing.T) {
	for _, tt := range []struct {
		name, input string
		wantErr     bool
	}{
		{"utf8", "中文🙂\n", false},
		{"boundary", strings.Repeat("a", maxLine) + "\n", false},
		{"over", strings.Repeat("a", maxLine+1) + "\n", true},
		{"unterminated", "{}", true},
		{"invalid_utf8", string([]byte{0xff, '\n'}), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var gotErr bool
			var n int
			for l := range lines(strings.NewReader(tt.input)) {
				if l.err != nil {
					gotErr = true
				} else {
					n++
					if string(l.raw) != strings.TrimSuffix(tt.input, "\n") {
						t.Fatalf("modified input: %q", l.raw)
					}
				}
			}
			if gotErr != tt.wantErr || (!gotErr && n != 1) {
				t.Fatalf("error=%v lines=%d", gotErr, n)
			}
		})
	}
}

func TestBindingIdentity(t *testing.T) {
	cwd, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := start{SchemaVersion: 1, Type: "start", RequestID: "req", WorkspaceID: "ws", TaskID: "task", BaseRevision: "rev", Cwd: cwd, Prompt: "fixture"}
	if err := validate(s, ""); err != nil {
		t.Fatal(err)
	}
	s.ThreadID = "T-11111111-2222-4333-8444-555555555555"
	b := binding{s.ThreadID, s.WorkspaceID, s.TaskID, s.BaseRevision, s.Cwd}
	path := filepath.Join(t.TempDir(), "bindings.json")
	for _, count := range []int{0, 1, 2} {
		list := make([]binding, count)
		for i := range list {
			list[i] = b
		}
		data, err := json.Marshal(map[string]any{"schemaVersion": 1, "threads": list})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if got := validate(s, path); (got == nil) != (count == 1) {
			t.Fatalf("count=%d got=%v", count, got)
		}
	}
}

func TestContextSchema(t *testing.T) {
	cwd, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		field, raw string
		valid      bool
	}{
		{"generation", `null`, true}, {"generation", `0`, true},
		{"generation", `9007199254740993`, true}, {"generation", `9223372036854775807`, true},
		{"generation", `9223372036854775808`, false}, {"generation", `-1`, false},
		{"generation", `0.5`, false}, {"generation", `"1"`, false}, {"generation", `{}`, false},
		{"progression", `null`, true}, {"progression", `0`, true}, {"progression", `1`, true},
		{"progression", `0.375`, true}, {"progression", `-0.001`, false}, {"progression", `1.001`, false},
		{"progression", `"0.5"`, false}, {"progression", `[]`, false},
		{"bookPath", `""`, true}, {"bookPath", `"Text/章.xhtml"`, true}, {"bookPath", `null`, false},
		{"fragment", `"note-2"`, true}, {"fragment", `null`, false}, {"fragment", `1`, false},
		{"selectedText", `"显式选区\n乙"`, true}, {"selectedText", `""`, true}, {"selectedText", `null`, false},
	} {
		t.Run(tt.field+"="+tt.raw, func(t *testing.T) {
			s := start{SchemaVersion: 1, Type: "start", RequestID: "req", WorkspaceID: "ws", TaskID: "task", BaseRevision: "rev", Cwd: cwd, Prompt: "fixture"}
			fields := map[string]*json.RawMessage{
				"generation": &s.Generation, "progression": &s.Progression,
				"bookPath": &s.BookPath, "fragment": &s.Fragment, "selectedText": &s.SelectedText,
			}
			*fields[tt.field] = json.RawMessage(tt.raw)
			if err := validate(s, ""); (err == nil) != tt.valid {
				t.Fatalf("valid=%t, got %v", tt.valid, err)
			}
			context := s.context()
			if len(context) != 1 || string(context[tt.field]) != tt.raw {
				t.Fatalf("explicit context changed or omitted fields were inferred: %v", context)
			}
		})
	}
}
