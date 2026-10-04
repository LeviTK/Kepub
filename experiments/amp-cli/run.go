package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

type config struct {
	cli            string
	bindings       []Binding
	timeout        time.Duration
	grace          time.Duration
	cleanupTimeout time.Duration
}

func terminal(e *emitter, state streamState, code string, cleaned bool) int {
	kind, exit := "failed", 1
	if !cleaned {
		code = "CLEANUP_UNCONFIRMED"
	} else if code == "CANCELLED" {
		kind, exit = "cancelled", 130
	} else if code == "" {
		kind, exit = "completed", 0
	}
	var threadID, wireCode any
	if state.threadID != "" {
		threadID = state.threadID
	}
	if code != "" {
		wireCode = code
	}
	data := map[string]any{"code": wireCode, "threadId": threadID, "reviewRequired": kind == "completed",
		"cleanup": map[string]any{"scope": "process-group", "confirmed": cleaned}}
	if kind == "completed" {
		data["result"] = state.text
	}
	e.send(kind, data)
	if e.err != nil {
		return 1 // A broken stdout cannot deliver a terminal event; never claim delivery.
	}
	return exit
}

// run owns and closes input. Output and diagnostics must remain drainable;
// downstream output backpressure is a documented host transport requirement.
func run(ctx context.Context, cfg config, input io.ReadCloser, output, diagnostics io.Writer) int {
	diagnostics = &lockedWriter{writer: diagnostics}
	defer input.Close()
	done := make(chan struct{})
	defer close(done)
	controls := pump(input, maxLine, done)
	e := &emitter{out: json.NewEncoder(output)}
	state := streamState{}
	var first frame
	select {
	case first = <-controls:
	case <-ctx.Done():
		return terminal(e, state, "CANCELLED", true)
	}
	if first.err != nil || json.Unmarshal(first.line, &e.s) != nil {
		return terminal(e, state, "INVALID_INPUT", true)
	}
	var binding *Binding
	for i := range cfg.bindings {
		if cfg.bindings[i].ThreadID == e.s.ThreadID {
			if binding != nil {
				return terminal(e, state, "INVALID_BINDING", true)
			}
			binding = &cfg.bindings[i]
		}
	}
	if err := e.s.validate(binding); err != nil {
		fmt.Fprintln(diagnostics, err)
		return terminal(e, state, "INVALID_INPUT", true)
	}
	state.threadID = e.s.ThreadID
	if ctx.Err() != nil {
		return terminal(e, state, "CANCELLED", true)
	}
	message := ampInput(e.s)
	if len(message) > maxLine {
		return terminal(e, state, "INPUT_LIMIT", true)
	}
	cmd := exec.Command(cfg.cli, ampArgs(e.s)...)
	cmd.Dir = e.s.CWD
	if err := configureProcess(cmd); err != nil {
		fmt.Fprintln(diagnostics, err)
		return terminal(e, state, "UNSUPPORTED_PLATFORM", true)
	}
	// Explicit pipe ownership prevents cmd.Wait from closing unread output.
	var pipes []*os.File
	defer func() {
		for _, pipe := range pipes {
			pipe.Close()
		}
	}()
	for range 3 {
		r, w, err := os.Pipe()
		if err != nil {
			return terminal(e, state, "PIPE_ERROR", true)
		}
		pipes = append(pipes, r, w)
	}
	stdinR, stdinW, stdoutR, stdoutW, stderrR, stderrW := pipes[0], pipes[1], pipes[2], pipes[3], pipes[4], pipes[5]
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdinR, stdoutW, stderrW
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(diagnostics, "cannot start CLI:", err)
		return terminal(e, state, "SPAWN_ERROR", true)
	}
	stdinR.Close()
	stdoutW.Close()
	stderrW.Close()
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	writeCh := make(chan error, 1)
	go func() {
		_, err := stdinW.Write(message)
		stdinW.Close() // One task, one user input record; cancel is a host control.
		writeCh <- err
	}()
	stderrCh := make(chan error, 1)
	go func() {
		// Drain stderr concurrently, but cap the volume retained by the host.
		n, err := io.Copy(diagnostics, io.LimitReader(stderrR, 1<<20+1))
		if n > 1<<20 {
			err = errors.New("stderr exceeds byte limit")
		}
		stderrCh <- err
	}()
	stdoutCh := pump(stdoutR, maxLine, done)
	e.send("started", map[string]any{"threadId": nullable(state.threadID), "executor": "local", "visibility": "private", "mode": "ultra"})
	deadline := time.NewTimer(cfg.timeout)
	defer deadline.Stop()
	var cleanupCh <-chan bool
	var finishCh <-chan time.Time
	var finishTimer *time.Timer
	defer func() {
		if finishTimer != nil {
			finishTimer.Stop()
		}
	}()
	code := ""
	stopping, cleaned, cleanupDone, waited, stdoutEOF := false, false, false, false, false
	var waitErr error
	stop := func(reason string) {
		if code == "" && reason != "" {
			code = reason
		}
		if stopping {
			return
		}
		stopping = true
		ch := make(chan bool, 1)
		cleanupCh = ch
		go func() { ch <- stopGroup(cmd.Process.Pid, cfg.grace, cfg.cleanupTimeout) }()
		finishTimer = time.NewTimer(cfg.grace + cfg.cleanupTimeout + 200*time.Millisecond)
		finishCh = finishTimer.C
	}
	if e.err != nil {
		stop("OUTPUT_ERROR")
	}
	ctxDone := ctx.Done()
	total := 0
	for !(cleanupDone && waited && stdoutCh == nil && stderrCh == nil && writeCh == nil) {
		select {
		case control, ok := <-controls:
			if !ok || errors.Is(control.err, io.EOF) {
				controls = nil // Host EOF means no more controls, not cancellation.
				continue
			}
			var cancel struct {
				SchemaVersion int    `json:"schemaVersion"`
				Type          string `json:"type"`
				RequestID     string `json:"requestId"`
			}
			if control.err != nil || json.Unmarshal(control.line, &cancel) != nil || cancel.SchemaVersion != 1 || cancel.Type != "cancel" || cancel.RequestID != e.s.RequestID {
				stop("INVALID_CONTROL")
			} else {
				stop("CANCELLED")
			}
		case err := <-writeCh:
			writeCh = nil
			if err != nil {
				stop("STDIN_ERROR")
			}
		case err := <-stderrCh:
			stderrCh = nil
			if err != nil {
				stop("STDERR_ERROR")
			}
		case f, ok := <-stdoutCh:
			if !ok || f.err != nil {
				stdoutCh = nil
				stdoutEOF = errors.Is(f.err, io.EOF)
				if !stdoutEOF {
					stop("STREAM_ERROR")
				} else if !state.result {
					stop("MISSING_RESULT")
				}
				continue
			}
			total += len(f.line)
			if total > 8<<20 {
				stop("OUTPUT_LIMIT")
			} else if err := state.consume(f.line, e); err != nil {
				fmt.Fprintln(diagnostics, err)
				if errors.Is(err, errAmpResult) {
					stop("AMP_ERROR")
				} else {
					stop("PROTOCOL_ERROR")
				}
			}
		case waitErr = <-waitCh:
			waitCh, waited = nil, true
			if !stopping {
				if waitErr != nil {
					stop("EXIT_ERROR")
				} else if present, err := groupPresent(cmd.Process.Pid); err != nil {
					stop("GROUP_PROBE_ERROR")
				} else if present {
					// Cleanup success cannot turn an incomplete writer lifecycle
					// into task success, even if a success result was emitted.
					stop("WRITERS_AFTER_CLI")
				}
			}
			stop("") // Cleanup descendants even after result + root exit 0.
		case cleaned = <-cleanupCh:
			cleanupCh, cleanupDone = nil, true
		case <-ctxDone:
			ctxDone = nil
			stop("CANCELLED")
		case <-deadline.C:
			stop("TIMEOUT")
		case <-finishCh:
			// A stuck writer/escaped descendant or uninterruptible process may
			// prevent EOF/reaping. Do not wait forever or report a completed task.
			return terminal(e, state, "CLEANUP_UNCONFIRMED", false)
		}
	}
	if code == "" {
		switch {
		case waitErr != nil:
			code = "EXIT_ERROR"
		case !stdoutEOF || !state.result || !state.success:
			code = "MISSING_RESULT"
		}
	}
	return terminal(e, state, code, cleaned)
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

type lockedWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(p)
}
