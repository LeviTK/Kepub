package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/testfixture"
	"golang.org/x/sys/unix"
)

// Linux /proc and SIGSTOP coordinate delivery after real staging starts. The
// signal is queued on the confirmed stopped child, not guessed from elapsed time.
func TestR3ReadCommandSignals(t *testing.T) {
	binary := workspaceBinary(t)
	book := filepath.Join(t.TempDir(), "book.epub")
	entries := append(testfixture.EPUB("3.0", false), testfixture.Entry{Name: "payload.bin", Data: bytes.Repeat([]byte("z"), 32<<20)})
	testfixture.ZIP(t, book, entries)
	original, err := os.ReadFile(book)
	if err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"info-read", "unpack-read", "unpack-copy", "workspace-read"} {
		for _, signal := range []syscall.Signal{0, syscall.SIGINT, syscall.SIGTERM} {
			t.Run(fmt.Sprintf("%s/%d", phase, signal), func(t *testing.T) {
				root := t.TempDir()
				tmp, output := filepath.Join(root, "tmp"), filepath.Join(root, "output")
				if err := os.Mkdir(tmp, 0700); err != nil {
					t.Fatal(err)
				}
				args := []string{"info", book, "--json"}
				if strings.HasPrefix(phase, "unpack") {
					args = []string{"unpack", book, "--output", output, "--json"}
				} else if phase == "workspace-read" {
					args = []string{"workspace", "open", book, "--output", output, "--json"}
				}
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, binary, args...)
				cmd.Env = append(os.Environ(), "TMPDIR="+tmp)
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if cmd.ProcessState == nil {
						cmd.Process.Kill()
						cmd.Wait()
					}
				}()
				if signal != 0 {
					pattern := filepath.Join(tmp, "kepub-read-*")
					if phase == "unpack-copy" {
						pattern = filepath.Join(root, ".kepub-unpack-*")
					}
					for {
						stages, err := filepath.Glob(pattern)
						if err != nil {
							t.Fatal(err)
						}
						if len(stages) != 0 {
							break
						}
						select {
						case <-ctx.Done():
							t.Fatal("did not reach staging phase", ctx.Err())
						case <-time.After(time.Millisecond):
						}
					}
					pid := cmd.Process.Pid
					if exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid)); err != nil || exe != binary {
						t.Fatal("wrong signal target", exe, err)
					}
					if err := cmd.Process.Signal(syscall.SIGSTOP); err != nil {
						t.Fatal(err)
					}
					var status unix.WaitStatus
					if got, err := unix.Wait4(pid, &status, unix.WUNTRACED, nil); err != nil || got != pid || !status.Stopped() {
						t.Fatal("child completed before phase coordination", got, status, err)
					}
					if stages, err := filepath.Glob(pattern); err != nil || len(stages) == 0 {
						t.Fatal("staging phase ended before signal coordination", stages, err)
					}
					if err := cmd.Process.Signal(signal); err != nil {
						t.Fatal(err)
					}
					statusBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
					if err != nil {
						t.Fatal(err)
					}
					var pending, caught uint64
					for _, line := range strings.Split(string(statusBytes), "\n") {
						fields := strings.Fields(line)
						if len(fields) != 2 {
							continue
						}
						bits, _ := strconv.ParseUint(fields[1], 16, 64)
						switch fields[0] {
						case "SigPnd:", "ShdPnd:":
							pending |= bits
						case "SigCgt:":
							caught = bits
						}
					}
					mask := uint64(1) << (signal - 1)
					if pending&mask == 0 || caught&mask == 0 {
						t.Fatalf("signal not queued/caught at runtime phase: pending=%x caught=%x", pending, caught)
					}
					t.Logf("pid=%d phase=%s signal=%d queued=%x caught=%x", pid, phase, signal, pending, caught)
					if err := cmd.Process.Signal(syscall.SIGCONT); err != nil {
						t.Fatal(err)
					}
				}
				err := cmd.Wait()
				want := 0
				if signal != 0 {
					want = 130
				}
				if ctx.Err() != nil || cmd.ProcessState.ExitCode() != want {
					t.Fatalf("exit=%d want=%d err=%v timeout=%v stdout=%s stderr=%s", cmd.ProcessState.ExitCode(), want, err, ctx.Err(), stdout.Bytes(), stderr.Bytes())
				}
				var envelope struct {
					OK    bool `json:"ok"`
					Error *struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || stderr.Len() != 0 || envelope.OK != (signal == 0) || signal != 0 && (envelope.Error == nil || envelope.Error.Code != "CANCELLED") {
					t.Fatal("bad cancellation envelope", stdout.String(), stderr.String(), err)
				}
				if signal != 0 {
					if _, err := os.Stat(output); !os.IsNotExist(err) {
						t.Fatal("cancelled request published output", err)
					}
					stages, err := os.ReadDir(root)
					if err != nil || len(stages) != 1 || stages[0].Name() != "tmp" {
						t.Fatal("unpublished staging remains", stages, err)
					}
				} else if phase != "info-read" {
					pub := output
					if phase == "workspace-read" {
						pub = filepath.Join(output, "revisions/initial/pub")
						if got, err := os.ReadFile(filepath.Join(output, "original/book.epub")); err != nil || !bytes.Equal(got, original) {
							t.Fatal("workspace original differs", err)
						}
					}
					want := map[string]bool{}
					for _, entry := range entries {
						name := strings.TrimSuffix(entry.Name, "/")
						want[name] = entry.Mode.IsDir()
						for parent := filepath.Dir(name); parent != "."; parent = filepath.Dir(parent) {
							want[parent] = true
						}
						if entry.Mode.IsDir() {
							continue
						}
						if got, err := os.ReadFile(filepath.Join(pub, entry.Name)); err != nil || !bytes.Equal(got, entry.Data) {
							t.Fatalf("resource %s differs: %v", entry.Name, err)
						}
					}
					if err := filepath.WalkDir(pub, func(path string, entry fs.DirEntry, err error) error {
						if err != nil || path == pub {
							return err
						}
						name, err := filepath.Rel(pub, path)
						if err != nil {
							return err
						}
						if directory, ok := want[name]; !ok || directory != entry.IsDir() {
							return fmt.Errorf("unexpected resource/type %s", name)
						}
						delete(want, name)
						return nil
					}); err != nil || len(want) != 0 {
						t.Fatal("publication inventory differs", want, err)
					}
				}
				if stages, err := os.ReadDir(tmp); err != nil || len(stages) != 0 {
					t.Fatal("read staging remains", stages, err)
				}
				if got, err := os.ReadFile(book); err != nil || !bytes.Equal(got, original) {
					t.Fatal("original changed", err)
				}
			})
		}
	}
}
