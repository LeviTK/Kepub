//go:build linux || darwin

// A controlled process fixture, never an Amp/model substitute in integration claims.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const threadID = "T-11111111-2222-4333-8444-555555555555"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "hold-stdout" {
		os.WriteFile("child-ready", []byte(strconv.Itoa(os.Getpid())), 0600)
		time.Sleep(250 * time.Millisecond)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "child" {
		signal.Ignore(syscall.SIGTERM)
		os.WriteFile("child-ready", []byte(strconv.Itoa(os.Getpid())), 0600)
		delay, _ := time.ParseDuration(os.Args[2])
		time.Sleep(delay)
		os.WriteFile("late-write", []byte("unsafe write"), 0600)
		return
	}
	scenarioBytes, _ := os.ReadFile("scenario")
	scenario := string(scenarioBytes)
	input, _ := io.ReadAll(io.LimitReader(os.Stdin, 2<<20))
	os.WriteFile("captured-input", input, 0600)
	args, _ := json.Marshal(os.Args[1:])
	os.WriteFile("captured-args", args, 0600)
	cwd, _ := os.Getwd()
	enc := json.NewEncoder(os.Stdout)
	emit := func(v any) { enc.Encode(v) }
	emit(map[string]any{"type": "system", "subtype": "init", "session_id": threadID, "cwd": cwd, "future": true})
	result := map[string]any{"type": "result", "subtype": "success", "session_id": threadID, "is_error": false, "result": "fixture only"}
	ready := func() {
		emit(map[string]any{"type": "assistant", "session_id": threadID, "message": map[string]any{"content": []any{map[string]any{"type": "text", "text": "ready"}}}})
	}
	switch scenario {
	case "no-result":
		return
	case "truncated":
		data, _ := json.Marshal(result)
		os.Stdout.Write(data)
		return
	case "oversize":
		os.Stdout.Write(bytes.Repeat([]byte("x"), 1<<20+1))
		return
	case "unterminated_oversize":
		os.WriteFile("raw-started", nil, 0600)
		os.Stdout.Write(bytes.Repeat([]byte("x"), 1<<20))
		os.WriteFile("oversize-sent", nil, 0600)
		time.Sleep(10 * time.Second) // No newline or EOF; limit must fire first.
		return
	case "exit_before_eof":
		fmt.Fprintln(os.Stderr, "fixture diagnostic")
		ready()
		emit(result)
		os.WriteFile("result-sent", nil, 0600)
		exe, _ := os.Executable()
		child := exec.Command(exe, "hold-stdout")
		child.Stdout = os.Stdout
		if err := child.Start(); err != nil {
			panic(err)
		}
		for {
			if _, err := os.Stat("child-ready"); err == nil {
				break
			}
			time.Sleep(time.Millisecond)
		}
		os.Exit(23)
	case "malformed":
		fmt.Println("{broken}")
		return
	case "invalid-utf8":
		os.Stdout.Write([]byte{'"', 0xff, '"', '\n'})
		return
	case "stderr-limit":
		os.Stderr.Write(bytes.Repeat([]byte("!"), 1<<20+1))
	case "stderr_parallel":
		os.WriteFile("stderr-started", []byte("2883584"), 0600)
		// os.File.Write blocks until all bytes are written or an error occurs;
		// the success result is never sent before this explicit drain completes.
		chunk := bytes.Repeat([]byte("d"), 8192)
		for written := 0; written < 2883584; written += len(chunk) {
			n, err := os.Stderr.Write(chunk)
			if err != nil || n != len(chunk) {
				os.Exit(24)
			}
		}
		os.WriteFile("stderr-finished", nil, 0600)
	case "total-limit":
		for range 10 {
			emit(map[string]any{"type": "future", "payload": strings.Repeat("x", 900000)})
		}
	case "whitespace-limit":
		for range 10 {
			fmt.Println(strings.Repeat(" ", 900000) + `{"type":"future"}`)
		}
	case "amp-error":
		result["is_error"], result["subtype"] = true, "error_during_execution"
	case "missing-error-flag":
		delete(result, "is_error")
	case "missing-result-text":
		delete(result, "result")
	case "wrong-thread":
		result["session_id"] = "T-aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	case "wait", "result-wait":
		signal.Ignore(syscall.SIGTERM)
		ready()
		if scenario == "result-wait" {
			emit(result)
			os.WriteFile("result-sent", nil, 0600)
		}
		time.Sleep(10 * time.Second)
		return
	case "child-cancel", "child-success", "child-closed-pipes", "escaped-pipes":
		exe, _ := os.Executable()
		delay := "700ms"
		if scenario == "escaped-pipes" {
			delay = "5s"
		}
		child := exec.Command(exe, "child", delay)
		if scenario != "child-closed-pipes" {
			child.Stdout, child.Stderr = os.Stdout, os.Stderr
		}
		if scenario == "escaped-pipes" {
			child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		}
		if err := child.Start(); err != nil {
			panic(err)
		}
		for {
			if _, err := os.Stat("child-ready"); err == nil {
				break
			}
			time.Sleep(time.Millisecond)
		}
		ready()
		if scenario == "child-cancel" {
			signal.Ignore(syscall.SIGTERM)
			child.Wait()
			return
		}
	default:
		var wg sync.WaitGroup
		wg.Go(func() { os.Stderr.Write(bytes.Repeat([]byte("fixture diagnostic\n"), 10000)) })
		emit(map[string]any{"type": "future-event", "payload": map[string]any{"unknown": true}})
		msg := map[string]any{"type": "assistant", "session_id": threadID, "future": 17, "message": map[string]any{"content": []any{
			map[string]any{"type": "text", "text": "中文🙂分块"}, map[string]any{"type": "future-block"},
			map[string]any{"type": "tool_use", "id": "tool-1", "name": "fixture", "input": map[string]any{"x": 7}},
		}}}
		line, _ := json.Marshal(msg)
		for _, b := range append(line, '\n') {
			os.Stdout.Write([]byte{b})
		}
		emit(map[string]any{"type": "user", "session_id": threadID, "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "tool-1", "content": "fixture result"}}}})
		wg.Wait()
	}
	emit(result)
	if scenario == "duplicate-result" {
		emit(result)
	}
	if scenario == "nonzero" {
		os.Exit(7)
	}
}
