// Experimental Linux supervisor; not part of cmd/kepub or a publication sandbox.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
	"time"
	"unicode/utf8"
)

const maxLine = 64 * 1024

type start struct {
	SchemaVersion int             `json:"schemaVersion"`
	Type          string          `json:"type"`
	RequestID     string          `json:"requestId"`
	WorkspaceID   string          `json:"workspaceId"`
	TaskID        string          `json:"taskId"`
	BaseRevision  string          `json:"baseRevision"`
	Cwd           string          `json:"cwd"`
	Prompt        string          `json:"prompt"`
	ThreadID      string          `json:"threadId,omitempty"`
	Generation    json.RawMessage `json:"generation"`
	BookPath      json.RawMessage `json:"bookPath"`
	Fragment      json.RawMessage `json:"fragment"`
	Progression   json.RawMessage `json:"progression"`
	SelectedText  json.RawMessage `json:"selectedText"`
}

func (s start) context() map[string]json.RawMessage {
	fields := map[string]json.RawMessage{
		"bookPath": s.BookPath, "fragment": s.Fragment, "progression": s.Progression,
		"selectedText": s.SelectedText, "generation": s.Generation,
	}
	for key, raw := range fields {
		if len(raw) == 0 {
			delete(fields, key)
		}
	}
	return fields
}

type binding struct {
	ThreadID     string `json:"threadId"`
	WorkspaceID  string `json:"workspaceId"`
	TaskID       string `json:"taskId"`
	BaseRevision string `json:"baseRevision"`
	Cwd          string `json:"cwd"`
}

type packet struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

type line struct {
	raw []byte
	err error
}

// Require newline, bound before JSON parsing, and reject broken UTF-8.
func split(data []byte, eof bool) (int, []byte, error) {
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		if i > maxLine || !utf8.Valid(data[:i]) {
			return 0, nil, errors.New("invalid or oversized line")
		}
		return i + 1, data[:i], nil
	}
	if len(data) > maxLine {
		return 0, nil, errors.New("oversized line")
	}
	if eof && len(data) != 0 {
		return 0, nil, io.ErrUnexpectedEOF
	}
	return 0, nil, nil
}

func lines(r io.Reader) <-chan line {
	ch := make(chan line, 8)
	go func() {
		defer close(ch)
		s := bufio.NewScanner(r)
		s.Buffer(make([]byte, 4096), maxLine+2)
		s.Split(split)
		for s.Scan() {
			ch <- line{raw: bytes.Clone(s.Bytes())}
		}
		if err := s.Err(); err != nil {
			ch <- line{err: err}
		}
	}()
	return ch
}

var threadPattern = regexp.MustCompile(`^T-[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func validate(s start, bindingsPath string) error {
	if s.SchemaVersion != 1 || s.Type != "start" || len(s.RequestID) == 0 || len(s.RequestID) > 256 || s.WorkspaceID == "" || s.TaskID == "" || s.BaseRevision == "" || s.Prompt == "" || !filepath.IsAbs(s.Cwd) {
		return errors.New("INVALID_START")
	}
	for key, raw := range s.context() {
		switch key {
		case "generation":
			var generation *int64
			if json.Unmarshal(raw, &generation) != nil || (generation != nil && *generation < 0) {
				return errors.New("INVALID_CONTEXT")
			}
		case "progression":
			var progression *float64
			if json.Unmarshal(raw, &progression) != nil || (progression != nil && (*progression < 0 || *progression > 1)) {
				return errors.New("INVALID_CONTEXT")
			}
		default:
			var text *string
			if json.Unmarshal(raw, &text) != nil || text == nil {
				return errors.New("INVALID_CONTEXT")
			}
		}
	}
	canonical, err := filepath.EvalSymlinks(s.Cwd)
	info, statErr := os.Stat(s.Cwd)
	if err != nil || statErr != nil || !info.IsDir() || canonical != s.Cwd {
		return errors.New("INVALID_CWD")
	}
	if s.ThreadID == "" {
		return nil
	}
	if !threadPattern.MatchString(s.ThreadID) {
		return errors.New("INVALID_THREAD")
	}
	data, err := os.ReadFile(bindingsPath)
	var registry struct {
		SchemaVersion int       `json:"schemaVersion"`
		Threads       []binding `json:"threads"`
	}
	if err != nil || json.Unmarshal(data, &registry) != nil || registry.SchemaVersion != 1 {
		return errors.New("BINDING_MISMATCH")
	}
	want := binding{s.ThreadID, s.WorkspaceID, s.TaskID, s.BaseRevision, s.Cwd}
	count := 0
	for _, b := range registry.Threads {
		if b.ThreadID == s.ThreadID {
			if b != want {
				return errors.New("BINDING_MISMATCH")
			}
			count++
		}
	}
	if count != 1 {
		return errors.New("BINDING_MISMATCH")
	}
	return nil
}

// Drain all stderr, publish at most 16 KiB; never let a verbose helper block.
type limitedLog struct{ remaining int }

func (w *limitedLog) Write(p []byte) (int, error) {
	if w.remaining > 0 {
		n := min(len(p), w.remaining)
		_, _ = os.Stderr.Write(p[:n])
		w.remaining -= n
	}
	return len(p), nil
}

func main() {
	os.Exit(run())
}

func run() int {
	// Go otherwise exits on a broken fd 1 before we can reclaim the child group.
	signal.Ignore(syscall.SIGPIPE)
	node := flag.String("node", "node", "Node 26.10.0 executable")
	helper := flag.String("helper", "dist/helper.js", "compiled helper path")
	bindings := flag.String("bindings", "", "trusted schemaVersion 1 thread binding registry")
	timeout := flag.Duration("timeout", 10*time.Second, "whole task deadline")
	grace := flag.Duration("grace", 100*time.Millisecond, "AbortSignal then TERM grace; KILL follows")
	flag.Parse()
	if *timeout <= 0 || *grace <= 0 {
		fmt.Fprintln(os.Stderr, "timeout and grace must be positive")
		return 2
	}
	input := lines(os.Stdin)
	first, ok := <-input
	var s start
	if !ok || first.err != nil || json.Unmarshal(first.raw, &s) != nil {
		fmt.Fprintln(os.Stderr, "expected bounded start JSONL")
		return 2 // No usable identity: cannot fabricate a public task envelope.
	}
	// An invalid incoming generation must not escape into the failure envelope.
	var generation *int64
	if json.Unmarshal(s.Generation, &generation) != nil || (generation != nil && *generation < 0) {
		generation = nil
	}
	seq := 0
	encode := json.NewEncoder(os.Stdout)
	var outputErr error
	emit := func(typ string, data map[string]any) error {
		if outputErr != nil {
			return outputErr
		}
		seq++
		outputErr = encode.Encode(map[string]any{
			"schemaVersion": 1, "requestId": s.RequestID, "workspaceId": s.WorkspaceID,
			"taskId": s.TaskID, "sequence": seq, "generation": generation, "type": typ, "data": data,
		})
		return outputErr
	}
	var thread any
	if s.ThreadID != "" {
		thread = s.ThreadID
	}
	terminal := func(typ, code string, cleaned bool, evidence map[string]any) int {
		if !cleaned {
			typ, code = "failed", "CLEANUP_UNCONFIRMED"
		}
		var codeValue any
		if code != "" {
			codeValue = code
		}
		data := map[string]any{"reviewRequired": typ == "completed", "code": codeValue, "threadId": thread,
			"cleanup": map[string]any{"scope": "process-group", "confirmed": cleaned}}
		for k, v := range evidence {
			data[k] = v
		}
		if err := emit(typ, data); err != nil {
			fmt.Fprintf(os.Stderr, "OUTPUT_FAILED: terminal undeliverable; EPIPE=%t; cleanup.process-group.confirmed=%t\n", errors.Is(err, syscall.EPIPE), cleaned)
			return 1
		}
		if typ == "completed" {
			return 0
		}
		return 1
	}
	if err := validate(s, *bindings); err != nil {
		return terminal("failed", err.Error(), true, nil)
	}
	prompt := s.Prompt
	if context := s.context(); len(context) > 0 {
		data, _ := json.Marshal(context) // validated JSON, preserve raw numeric values
		prompt += "\n\nKepub task context (JSON):\n" + string(data)
	}
	// Private helper input carries only what it needs, avoiding a duplicate copy
	// of explicit context. Never read files or infer omitted context fields.
	helperStart, err := json.Marshal(map[string]any{
		"type": "start", "requestId": s.RequestID, "cwd": s.Cwd, "prompt": prompt, "threadId": s.ThreadID,
	})
	if err != nil || len(helperStart) > maxLine {
		return terminal("failed", "MESSAGE_LIMIT", true, nil)
	}
	if err := enableReaping(); err != nil {
		return terminal("failed", "PLATFORM_UNVERIFIED", false, nil)
	}
	helperPath, err := filepath.Abs(*helper)
	if err != nil {
		return terminal("failed", "HELPER_PATH", true, nil)
	}
	cmd := exec.Command(*node, "--max-old-space-size=128", helperPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = time.Second
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return terminal("failed", "HELPER_PIPE", true, nil)
	}
	defer stdin.Close()
	stdout, writer, err := os.Pipe()
	if err != nil {
		return terminal("failed", "HELPER_PIPE", true, nil)
	}
	defer stdout.Close()
	cmd.Stdout = writer
	cmd.Stderr = &limitedLog{remaining: 16 * 1024}
	if err := cmd.Start(); err != nil {
		writer.Close()
		return terminal("failed", "HELPER_START", true, nil)
	}
	writer.Close()
	pid := cmd.Process.Pid
	control := make(chan []byte, 1)
	writeErrors := make(chan error, 1)
	defer close(control)
	// Never block deadline/cancellation on a Node process not reading its stdin.
	go func() {
		if _, err := stdin.Write(append(helperStart, '\n')); err != nil {
			writeErrors <- err
			return
		}
		for raw := range control {
			if _, err := stdin.Write(raw); err != nil {
				writeErrors <- err
				return
			}
		}
	}()
	emit("started", map[string]any{"executor": "local", "visibility": "private", "mode": "ultra"})
	out := lines(stdout)
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.Now().Add(*timeout)
	var stopped time.Time
	reason, cancelled := "", false
	stop := func(code string, cancel bool) {
		if reason == "" {
			reason, cancelled, stopped = code, cancel, time.Now()
			raw, _ := json.Marshal(map[string]any{"schemaVersion": 1, "type": "cancel", "requestId": s.RequestID})
			control <- append(raw, '\n') // only the first stop sends a control
		}
	}
	if outputErr != nil {
		stop("OUTPUT_FAILED", false)
	}
	var result map[string]any
	var waitErr error
	nodeDone, outDone := false, false
	termSent := false
	count, total := 0, 0
	for !nodeDone || !outDone {
		select {
		case <-writeErrors:
			stop("HELPER_INPUT", false)
		case l, ok := <-input:
			if !ok {
				input = nil // stdin EOF ends controls, not the task.
				continue
			}
			var c start
			if l.err != nil || json.Unmarshal(l.raw, &c) != nil || c.SchemaVersion != 1 || c.Type != "cancel" || c.RequestID != s.RequestID {
				stop("INVALID_CONTROL", false)
			} else {
				stop("CANCELLED", true) // idempotent, including after result but before exit.
			}
		case <-signals:
			stop("CANCELLED", true)
		case l, ok := <-out:
			if !ok {
				outDone, out = true, nil
				continue
			}
			count++
			total += len(l.raw)
			var p packet
			if l.err != nil || count > 4096 || total > 8*1024*1024 || json.Unmarshal(l.raw, &p) != nil || p.Data == nil {
				stop("HELPER_PROTOCOL", false)
				continue
			}
			if result != nil {
				stop("HELPER_AFTER_END", false)
				continue
			}
			if id, ok := p.Data["threadId"].(string); ok {
				thread = id
			}
			switch p.Type {
			case "assistant", "tool":
				if reason == "" {
					if err := emit(p.Type, p.Data); err != nil {
						stop("OUTPUT_FAILED", false)
					}
				}
			case "sdk_end":
				if p.Data["cliExitEvidence"] != "sdk-iterator-validated-zero" {
					stop("HELPER_PROTOCOL", false)
				} else {
					result = p.Data
				}
			case "sdk_error":
				code, _ := p.Data["code"].(string)
				if code == "" {
					code = "SDK_EXECUTION"
				}
				stop(code, false)
			default:
				stop("HELPER_PROTOCOL", false)
			}
		case waitErr = <-wait:
			nodeDone, wait = true, nil
		case <-ticker.C:
			if time.Now().After(deadline) {
				stop("TIMEOUT", false)
			}
			if !stopped.IsZero() {
				if time.Since(stopped) >= *grace && !termSent {
					_ = syscall.Kill(-pid, syscall.SIGTERM)
					termSent = true
				}
				if time.Since(stopped) >= 2**grace {
					_ = syscall.Kill(-pid, syscall.SIGKILL)
				}
				if time.Since(stopped) >= 2**grace+2*time.Second {
					return terminal("failed", "CLEANUP_UNCONFIRMED", false, nil)
				}
			}
		}
	}
	stdin.Close()
	// Node Wait is not process-tree completion. Check and reap the entire group.
	cleaned, leftover := cleanGroup(pid, *grace)
	if reason != "" {
		typ := "failed"
		if cancelled {
			typ = "cancelled"
		}
		return terminal(typ, reason, cleaned, nil)
	}
	if waitErr != nil {
		return terminal("failed", "HELPER_EXIT", cleaned, nil)
	}
	if leftover {
		return terminal("failed", "WRITERS_AFTER_SDK", cleaned, nil)
	}
	if result == nil {
		return terminal("failed", "MISSING_SDK_END", cleaned, nil)
	}
	return terminal("completed", "", cleaned, map[string]any{
		"result": result["result"], "cliExitEvidence": result["cliExitEvidence"], "helperExitCode": 0,
	})
}

func cleanGroup(pgid int, grace time.Duration) (bool, bool) {
	reapGroup(pgid)
	if errors.Is(syscall.Kill(-pgid, 0), syscall.ESRCH) {
		return true, false
	}
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	started := time.Now()
	for time.Since(started) < grace+time.Second {
		if time.Since(started) >= grace {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		}
		reapGroup(pgid)
		if errors.Is(syscall.Kill(-pgid, 0), syscall.ESRCH) {
			return true, true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false, true
}
