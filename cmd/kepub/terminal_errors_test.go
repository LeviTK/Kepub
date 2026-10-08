package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/app"
	"github.com/LeviTK/Kepub/internal/validation"
	"github.com/LeviTK/Kepub/internal/workspace"
)

func TestR5MissingPathTerminalEscape(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-\x1b[2J.epub")
	var out, errout bytes.Buffer
	exit := run([]string{"info", path}, &out, &errout)
	if exit == 0 || out.Len() != 0 {
		t.Fatalf("missing input succeeded: exit=%d stdout=%q stderr=%q", exit, out.Bytes(), errout.Bytes())
	}
	if strings.ContainsRune(errout.String(), '\x1b') || !strings.Contains(errout.String(), `\x1b[2J`) {
		t.Fatalf("unescaped terminal error: exit=%d stdout=%q stderr=%q", exit, out.Bytes(), errout.Bytes())
	}
}

func TestR5ErrorControlsAndJSONSemantics(t *testing.T) {
	for _, tc := range []struct{ name, text, escaped string }{
		{"ESC", "\x1b[31m", `\x1b[31m`},
		{"BEL", "\a", `\a`},
		{"CR", "\r", `\r`},
		{"LF", "\n", `\n`},
		{"TAB", "\t", `\t`},
		{"DEL", "\x7f", `\x7f`},
		{"C1", "\u0085", `\u0085`},
		{"bidi-override", "\u202e", `\u202e`},
		{"bidi-isolate", "\u2066\u2069", `\u2066\u2069`},
		{"line-separator", "\u2028", `\u2028`},
		{"ordinary", `中文%"`, `中文%\"`},
		{"long-path", strings.Repeat("目录%/", 25), strings.Repeat("目录%/", 25)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "missing-"+tc.text+".epub")
			_, inputErr := os.Stat(path)
			if inputErr == nil {
				t.Fatal("fixture must not exist")
			}
			// Expect the original OS diagnostic, independently of CLI rendering.
			message := "input: " + inputErr.Error()
			var out, errs bytes.Buffer
			exit := run([]string{"info", path}, &out, &errs)
			want := "IO_ERROR: " + strconv.QuoteToGraphic(message) + "\n"
			if exit != 6 || out.Len() != 0 || errs.String() != want || !strings.Contains(errs.String(), tc.escaped) || strings.Count(errs.String(), "\n") != 1 {
				t.Fatalf("bad human failure: exit=%d stdout=%q stderr=%q want=%q", exit, out.Bytes(), errs.Bytes(), want)
			}
			t.Logf("human exit=%d stdout=%q stderr=%q", exit, out.Bytes(), errs.Bytes())
			out.Reset()
			errs.Reset()
			exit = run([]string{"info", path, "--json"}, &out, &errs)
			var env envelope
			d := json.NewDecoder(bytes.NewReader(out.Bytes()))
			if err := d.Decode(&env); err != nil {
				t.Fatalf("invalid envelope: %q %v", out.Bytes(), err)
			}
			var extra any
			if exit != 6 || errs.Len() != 0 || env.OK || env.SchemaVersion != 1 || env.Command != "info" || env.RequestID == "" || env.Error == nil || env.Error.Code != "IO_ERROR" || env.Error.Message != message || d.Decode(&extra) != io.EOF {
				t.Fatalf("JSON meaning changed: exit=%d stdout=%q stderr=%q original=%q", exit, out.Bytes(), errs.Bytes(), message)
			}
			t.Logf("json exit=%d stdout=%q stderr=%q", exit, out.Bytes(), errs.Bytes())
		})
	}
}

type r5FailedWriter struct{ message string }

func (w r5FailedWriter) Write([]byte) (int, error) { return 0, errors.New(w.message) }

func TestR5OutputWriterErrorPaths(t *testing.T) {
	dir, _, _, _ := cliContentWorkspace(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>Unchanged.</p></body></html>`)
	message := "输出 中文%\"\x1b]8;;link\a\r\n\t\u0085\u202e\xff"
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"human-summary", []string{"version"}},
		{"machine-json", []string{"version", "--json"}},
		{"raw-json-document", []string{"fix", "propose", "--workspace", dir}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var positive, errors bytes.Buffer
			if exit := run(tc.args, &positive, &errors); exit != 0 || positive.Len() == 0 || errors.Len() != 0 {
				t.Fatalf("fixture did not reach output branch: exit=%d out=%q err=%q", exit, positive.Bytes(), errors.Bytes())
			}
			var errout bytes.Buffer
			exit := run(tc.args, r5FailedWriter{message}, &errout)
			want := "output: \"输出 中文%\\\"\\x1b]8;;link\\a\\r\\n\\t\\u0085\\u202e\\xff\"\n"
			if exit != 6 || errout.String() != want {
				t.Fatalf("unsafe writer diagnostic: exit=%d stderr=%q want=%q", exit, errout.Bytes(), want)
			}
			t.Logf("writer exit=%d stderr=%q", exit, errout.Bytes())
		})
	}
}

func TestR5SummaryDiagnosticsAndKeys(t *testing.T) {
	message := "诊断 中文%\r\n\t\x1b[2J\u0085\u202e"
	for _, data := range []any{
		validation.Report{Diagnostics: []validation.Diagnostic{{Source: "epubcheck", Message: message}}},
		[]app.Capability{{ID: "example", Reason: message}},
		map[string]any{"key\x1b[2J\u202e": message},
	} {
		out, err := humanSummary("validate", data)
		if err != nil || !strings.Contains(out, `诊断 中文%\r\n\t\x1b[2J\u0085\u202e`) || strings.ContainsAny(out, "\r\t\x1b\u0085\u202e") {
			t.Fatalf("unsafe summary: %q %v", out, err)
		}
	}
}

func TestR5TaskDiffIsNotDoubleEscaped(t *testing.T) {
	dir, _, _, _ := cliContentWorkspace(t, "<html xmlns=\"http://www.w3.org/1999/xhtml\"><body><p>old 中文</p></body></html>\r\n")
	w, err := workspace.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	path, err := w.NewCandidate()
	if err != nil {
		w.Close()
		t.Fatal(err)
	}
	id, err := w.TaskID()
	if err == nil {
		err = os.WriteFile(filepath.Join(path, cliContentPath), []byte("<html xmlns=\"http://www.w3.org/1999/xhtml\"><body><p>new 中文</p></body></html>\r\n"), 0600)
	}
	w.Close()
	if err != nil {
		t.Fatal(err)
	}
	var out, errs bytes.Buffer
	exit := run([]string{"task", "diff", id, "--workspace", dir}, &out, &errs)
	if exit != 0 || errs.Len() != 0 || !strings.Contains(out.String(), "old 中文") || !strings.Contains(out.String(), "new 中文") || strings.Count(out.String(), `\r\n`) != 2 || strings.Contains(out.String(), `\\r\\n`) || strings.ContainsRune(out.String(), '\r') {
		t.Fatalf("changed existing safe diff: exit=%d out=%q err=%q", exit, out.Bytes(), errs.Bytes())
	}
}
