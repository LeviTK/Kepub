//go:build linux || darwin

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const fakeThread = "T-11111111-2222-4333-8444-555555555555"

var fakeCLI string
var hostCLI string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "kepub-fake-cli-")
	if err != nil {
		panic(err)
	}
	fakeCLI = filepath.Join(dir, "fake-cli")
	cmd := exec.Command("go", "build", "-o", fakeCLI, "./testdata/fake-cli")
	if output, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic(fmt.Sprintf("build fake: %v: %s", err, output))
	}
	hostCLI = filepath.Join(dir, "amp-cli-spike")
	cmd = exec.Command("go", "build", "-o", hostCLI, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		panic(fmt.Sprintf("build host: %v: %s", err, output))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func fixture(t *testing.T, scenario string) (Start, config) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "scenario"), []byte(scenario), 0600); err != nil {
		t.Fatal(err)
	}
	return Start{SchemaVersion: 1, Type: "start", RequestID: "request-7", WorkspaceID: "workspace-3", TaskID: "task-9", BaseRevision: "revision-2", CWD: dir, Prompt: "Only fixture context; $(touch should-not-exist)\n中文"},
		config{cli: fakeCLI, timeout: 3 * time.Second, grace: 40 * time.Millisecond, cleanupTimeout: 400 * time.Millisecond}
}

func startJSON(s Start) []byte {
	data, _ := json.Marshal(s)
	return append(data, '\n')
}

func events(t *testing.T, output []byte, s Start) []Event {
	t.Helper()
	var all []Event
	for _, line := range bytes.Split(bytes.TrimSpace(output), []byte{'\n'}) {
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatalf("non-JSON stdout: %s: %v", line, err)
		}
		if event.SchemaVersion != 1 || event.Sequence != len(all)+1 || event.RequestID != s.RequestID || event.WorkspaceID != s.WorkspaceID || event.TaskID != s.TaskID || !reflect.DeepEqual(event.Generation, s.Generation) {
			t.Fatalf("wrong envelope: %+v", event)
		}
		if len(all) > 0 && (all[len(all)-1].Type == "completed" || all[len(all)-1].Type == "failed" || all[len(all)-1].Type == "cancelled") {
			t.Fatal("event after terminal")
		}
		all = append(all, event)
	}
	if len(all) == 0 {
		t.Fatal("no events")
	}
	last := all[len(all)-1]
	if last.Type != "completed" && last.Type != "failed" && last.Type != "cancelled" {
		t.Fatal("no terminal")
	}
	if last.Data["reviewRequired"] != (last.Type == "completed") {
		t.Fatalf("wrong review gate: %+v", last)
	}
	return all
}

func checkTerminal(t *testing.T, all []Event, kind string, code any, confirmed bool) {
	t.Helper()
	last := all[len(all)-1]
	if last.Type != kind || last.Data["code"] != code {
		t.Fatalf("terminal = %s %#v; want %s %#v", last.Type, last.Data, kind, code)
	}
	cleanup, ok := last.Data["cleanup"].(map[string]any)
	if !ok || cleanup["scope"] != "process-group" || cleanup["confirmed"] != confirmed {
		t.Fatalf("wrong cleanup: %+v", cleanup)
	}
}

func TestSharedScenarios(t *testing.T) {
	for _, tc := range []struct{ name, code string }{
		{"chunked-utf8-streaming-input-unknown-fields-stderr", ""}, {"oversize", "STREAM_ERROR"},
		{"total-limit", "OUTPUT_LIMIT"}, {"whitespace-limit", "OUTPUT_LIMIT"}, {"stderr-limit", "STDERR_ERROR"},
		{"no-result", "MISSING_RESULT"}, {"truncated", "STREAM_ERROR"}, {"malformed", "PROTOCOL_ERROR"},
		{"invalid-utf8", "STREAM_ERROR"}, {"nonzero", "EXIT_ERROR"}, {"amp-error", "AMP_ERROR"},
		{"missing-error-flag", "PROTOCOL_ERROR"}, {"wrong-thread", "PROTOCOL_ERROR"}, {"duplicate-result", "PROTOCOL_ERROR"},
		{"missing-result-text", "PROTOCOL_ERROR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, cfg := fixture(t, tc.name)
			generation := int64(13)
			s.Generation, s.SelectedText, s.BookPath, s.Fragment = &generation, "只发送选区", "EPUB/ch01.xhtml", "note-8"
			var output, diagnostics bytes.Buffer
			// The host pipe remains open through terminal; EOF is not needed to
			// dispatch start. Every byte of the JSONL, including UTF-8, is split.
			r, w := io.Pipe()
			go func() {
				for _, b := range startJSON(s) {
					if _, err := w.Write([]byte{b}); err != nil {
						return
					}
				}
			}()
			defer w.Close()
			exit := run(context.Background(), cfg, r, &output, &diagnostics)
			all := events(t, output.Bytes(), s)
			if tc.code != "" {
				checkTerminal(t, all, "failed", tc.code, true)
				if exit == 0 {
					t.Fatal("failure exit 0")
				}
				return
			}
			checkTerminal(t, all, "completed", nil, true)
			if exit != 0 || len(all) != 5 || all[1].Type != "assistant" || all[1].Data["text"] != "中文🙂分块" || all[2].Type != "tool" || all[3].Type != "tool" || all[4].Data["threadId"] != fakeThread {
				t.Fatalf("wrong stream: exit %d, %+v", exit, all)
			}
			if !strings.Contains(diagnostics.String(), "fixture diagnostic") || bytes.Contains(output.Bytes(), []byte("fixture diagnostic")) {
				t.Fatal("stderr not separated")
			}
			data, _ := os.ReadFile(filepath.Join(s.CWD, "captured-input"))
			var input struct {
				Type, RequestID string
				Message         struct {
					Role    string
					Content []struct{ Type, Text string }
				}
			}
			if json.Unmarshal(data, &input) != nil || input.Type != "user" || input.RequestID != s.RequestID || input.Message.Role != "user" || len(input.Message.Content) != 1 {
				t.Fatalf("wrong Amp input: %s", data)
			}
			var captured Start
			text := strings.TrimPrefix(input.Message.Content[0].Text, "Kepub task context (JSON):\n")
			if json.Unmarshal([]byte(text), &captured) != nil || !reflect.DeepEqual(captured, s) {
				t.Fatalf("context not preserved: %s", text)
			}
			if _, err := os.Stat(filepath.Join(s.CWD, "should-not-exist")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("prompt interpreted as shell")
			}
		})
	}
}

type readyWriter struct {
	bytes.Buffer
	ready chan struct{}
	seen  bool
}

func (w *readyWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if !w.seen && bytes.Contains(p, []byte(`"text":"ready"`)) {
		w.seen = true
		close(w.ready)
	}
	return n, err
}

func waitFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func TestCancellationAndDelayedWriters(t *testing.T) {
	for _, scenario := range []string{"wait", "result-wait", "child-cancel", "child-success", "child-closed-pipes"} {
		t.Run(scenario, func(t *testing.T) {
			s, cfg := fixture(t, scenario)
			r, w := io.Pipe()
			defer w.Close()
			out := &readyWriter{ready: make(chan struct{})}
			finished := make(chan int, 1)
			go func() { finished <- run(context.Background(), cfg, r, out, io.Discard) }()
			w.Write(startJSON(s))
			select {
			case <-out.ready:
			case <-time.After(2 * time.Second):
				t.Fatal("fake not ready")
			}
			cancelled := scenario == "wait" || scenario == "result-wait" || scenario == "child-cancel"
			if scenario == "result-wait" {
				waitFile(t, filepath.Join(s.CWD, "result-sent"))
			}
			if cancelled {
				cancel := []byte(`{"schemaVersion":1,"type":"cancel","requestId":"request-7"}` + "\n")
				w.Write(append(cancel, cancel...)) // Repeated cancellation is idempotent.
			}
			select {
			case exit := <-finished:
				all := events(t, out.Bytes(), s)
				if cancelled {
					checkTerminal(t, all, "cancelled", "CANCELLED", true)
					if exit != 130 {
						t.Fatalf("exit=%d", exit)
					}
				} else {
					checkTerminal(t, all, "completed", nil, true)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("unbounded cancellation")
			}
			if strings.HasPrefix(scenario, "child-") {
				// Beyond the child's intended write time, not merely immediately
				// after result or root exit. Both inherited and closed pipes tested.
				time.Sleep(800 * time.Millisecond)
				if _, err := os.Stat(filepath.Join(s.CWD, "late-write")); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("descendant wrote after terminal")
				}
				pidBytes, _ := os.ReadFile(filepath.Join(s.CWD, "child-ready"))
				pid, _ := strconv.Atoi(string(pidBytes))
				t.Logf("delayed writer pid=%d: no late-write after 800ms", pid)
			}
		})
	}
}

func TestEscapedPipeIsNotCompleted(t *testing.T) {
	s, cfg := fixture(t, "escaped-pipes")
	var output bytes.Buffer
	exit := run(context.Background(), cfg, io.NopCloser(bytes.NewReader(startJSON(s))), &output, io.Discard)
	pidBytes, _ := os.ReadFile(filepath.Join(s.CWD, "child-ready"))
	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil || pid <= 1 {
		t.Fatal("missing fixture child pid")
	}
	// Explicit cleanup of the intentionally escaped test fixture only.
	t.Cleanup(func() { syscall.Kill(pid, syscall.SIGKILL) })
	checkTerminal(t, events(t, output.Bytes(), s), "failed", "CLEANUP_UNCONFIRMED", false)
	if exit == 0 {
		t.Fatal("escaped writer reported success")
	}
}

func TestExplicitThreadBinding(t *testing.T) {
	s, cfg := fixture(t, "normal")
	s.ThreadID = fakeThread
	bound := Binding{s.ThreadID, s.WorkspaceID, s.TaskID, s.BaseRevision, s.CWD}
	cfg.bindings = []Binding{bound}
	var output bytes.Buffer
	if run(context.Background(), cfg, io.NopCloser(bytes.NewReader(startJSON(s))), &output, io.Discard) != 0 {
		t.Fatal(output.String())
	}
	checkTerminal(t, events(t, output.Bytes(), s), "completed", nil, true)
	argsBytes, _ := os.ReadFile(filepath.Join(s.CWD, "captured-args"))
	var args []string
	json.Unmarshal(argsBytes, &args)
	want := []string{"--execute", "--stream-json", "--stream-json-input", "--executor", "local", "--visibility", "private", "--mode", "ultra", "--no-ide", "--no-color", "threads", "continue", fakeThread}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args=%q", args)
	}
	for _, field := range []string{"missing", "workspace", "task", "revision", "cwd", "thread"} {
		t.Run(field, func(t *testing.T) {
			s, cfg := fixture(t, "normal")
			s.ThreadID = fakeThread
			b := Binding{s.ThreadID, s.WorkspaceID, s.TaskID, s.BaseRevision, s.CWD}
			switch field {
			case "workspace":
				b.WorkspaceID += "other"
			case "task":
				b.TaskID += "other"
			case "revision":
				b.BaseRevision += "other"
			case "cwd":
				b.CWD += "other"
			case "thread":
				b.ThreadID = "T-aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
			}
			if field != "missing" {
				cfg.bindings = []Binding{b}
			}
			var output bytes.Buffer
			run(context.Background(), cfg, io.NopCloser(bytes.NewReader(startJSON(s))), &output, io.Discard)
			checkTerminal(t, events(t, output.Bytes(), s), "failed", "INVALID_INPUT", true)
			if _, err := os.Stat(filepath.Join(s.CWD, "captured-args")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("invalid continuation spawned CLI")
			}
		})
	}
}

func TestFrameBoundaries(t *testing.T) {
	for _, tc := range []struct {
		input string
		limit int
		valid bool
	}{
		{"123\n", 4, true}, {"1234\n", 4, false}, {"123", 4, false}, {"\xff\n", 4, false},
	} {
		_, err := readFrame(bufio.NewReader(strings.NewReader(tc.input)), tc.limit)
		if (err == nil) != tc.valid {
			t.Fatalf("input %q: %v", tc.input, err)
		}
	}
}

func TestCancelBeforeSpawnAndTimeout(t *testing.T) {
	s, cfg := fixture(t, "wait")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	if run(ctx, cfg, io.NopCloser(bytes.NewReader(startJSON(s))), &output, io.Discard) != 130 {
		t.Fatal(output.String())
	}
	if _, err := os.Stat(filepath.Join(s.CWD, "captured-args")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("spawned after pre-cancel")
	}
	output.Reset()
	cfg.timeout = 100 * time.Millisecond
	run(context.Background(), cfg, io.NopCloser(bytes.NewReader(startJSON(s))), &output, io.Discard)
	checkTerminal(t, events(t, output.Bytes(), s), "failed", "TIMEOUT", true)
}

func TestInvalidControlAndUnavailableCLI(t *testing.T) {
	for _, tc := range []struct{ control, code string }{
		{`{"schemaVersion":1,"type":"cancel","requestId":"different"}`, "INVALID_CONTROL"},
		{`{"schemaVersion":1,"type":"start","requestId":"request-7"}`, "INVALID_CONTROL"},
		{`{"schemaVersion":2,"type":"cancel","requestId":"request-7"}`, "INVALID_CONTROL"},
	} {
		s, cfg := fixture(t, "wait")
		var output bytes.Buffer
		input := append(startJSON(s), []byte(tc.control+"\n")...)
		run(context.Background(), cfg, io.NopCloser(bytes.NewReader(input)), &output, io.Discard)
		checkTerminal(t, events(t, output.Bytes(), s), "failed", tc.code, true)
	}
	s, cfg := fixture(t, "normal")
	cfg.cli = filepath.Join(s.CWD, "nonexistent")
	var output bytes.Buffer
	run(context.Background(), cfg, io.NopCloser(bytes.NewReader(startJSON(s))), &output, io.Discard)
	checkTerminal(t, events(t, output.Bytes(), s), "failed", "SPAWN_ERROR", true)
}

func TestHostExecutableBindingsAndSIGTERM(t *testing.T) {
	s, _ := fixture(t, "wait")
	s.ThreadID = fakeThread
	bindings := map[string]any{"schemaVersion": 1, "threads": []Binding{{s.ThreadID, s.WorkspaceID, s.TaskID, s.BaseRevision, s.CWD}}}
	data, _ := json.Marshal(bindings)
	path := filepath.Join(t.TempDir(), "bindings.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, hostCLI, "--cli", fakeCLI, "--bindings", path)
	input := bytes.TrimSuffix(startJSON(s), []byte("}\n"))
	input = append(input, []byte(",\"futureContext\":true}\n")...)
	cmd.Stdin = bytes.NewReader(input)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	scanner := bufio.NewScanner(stdout)
	signalled := false
	for scanner.Scan() {
		line := scanner.Bytes()
		output.Write(line)
		output.WriteByte('\n')
		if bytes.Contains(line, []byte(`"text":"ready"`)) {
			if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			signalled = true
		}
	}
	if scanner.Err() != nil {
		t.Fatal(scanner.Err())
	}
	err = cmd.Wait()
	var exit *exec.ExitError
	if !signalled || !errors.As(err, &exit) || exit.ExitCode() != 130 {
		t.Fatalf("signalled=%v, exit=%v, output=%s", signalled, err, output.String())
	}
	checkTerminal(t, events(t, output.Bytes(), s), "cancelled", "CANCELLED", true)
}

func TestDelayedWriterPositiveControl(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(fakeCLI, "child", "20ms")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "late-write"))
	if err != nil || string(data) != "unsafe write" {
		t.Fatalf("fixture did not actually write: %q, %v", data, err)
	}
}

func TestClosedHostOutputDoesNotSIGPIPE(t *testing.T) {
	s, _ := fixture(t, "wait")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	r.Close() // No reader: the first started event encounters EPIPE.
	defer w.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, hostCLI, "--cli", fakeCLI)
	cmd.Stdin, cmd.Stdout = bytes.NewReader(startJSON(s)), w
	err = cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("want handled EPIPE exit 1, not SIGPIPE: %v", err)
	}
}
