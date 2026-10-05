//go:build linux || darwin

package workspace

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestWorkspaceProcessHelper(t *testing.T) {
	mode := os.Getenv("KEPUB_WORKSPACE_HELPER")
	if mode == "" {
		return
	}
	dir := os.Getenv("KEPUB_WORKSPACE_DIR")
	if mode == "limited-recovery" {
		// Confine the real short-write fault to this subprocess, not the test
		// runner or concurrent tests. Metadata is larger than 2 KiB, while
		// checkpoint/tree metadata and original publication files are smaller.
		signal.Ignore(unix.SIGXFSZ)
		var limit unix.Rlimit
		if err := unix.Getrlimit(unix.RLIMIT_FSIZE, &limit); err != nil {
			t.Fatal(err)
		}
		limit.Cur = 2048
		if err := unix.Setrlimit(unix.RLIMIT_FSIZE, &limit); err != nil {
			t.Fatal(err)
		}
		w, err := Open(dir)
		if w != nil {
			w.Close()
		}
		if !errors.Is(err, unix.EFBIG) {
			t.Fatalf("expected actual short-write failure, got %v", err)
		}
		return
	}
	if mode == "create" {
		fmt.Println("ready")
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		w, err := Create(dir, os.Getenv("KEPUB_WORKSPACE_SOURCE"), Options{})
		if err == nil {
			w.Close()
			fmt.Println("created")
		} else if errors.Is(err, os.ErrExist) {
			fmt.Println("exists")
		} else {
			t.Fatal(err)
		}
		return
	}
	w, err := Open(dir)
	if mode == "probe" {
		if !errors.Is(err, ErrBusy) {
			if w != nil {
				w.Close()
			}
			t.Fatalf("expected cross-process conflict; got %v", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if mode == "limited-apply" {
		defer w.Close()
		var p Plan
		if err := readEditJSON(w.root, "plans/"+os.Getenv("KEPUB_WORKSPACE_SOURCE")+".json", &p); err != nil {
			t.Fatal(err)
		}
		b := editJSON(t, p)
		signal.Ignore(unix.SIGXFSZ)
		var limit unix.Rlimit
		if err := unix.Getrlimit(unix.RLIMIT_FSIZE, &limit); err != nil {
			t.Fatal(err)
		}
		// Intent fits exactly; the containing Execution start is larger.
		// Publication/checkpoint therefore precede the real startup fault.
		limit.Cur = uint64(len(b) + 1)
		if err := unix.Setrlimit(unix.RLIMIT_FSIZE, &limit); err != nil {
			t.Fatal(err)
		}
		e, err := w.Apply(b)
		id, idErr := w.taskID()
		if !errors.Is(err, unix.EFBIG) || idErr != nil || !validID(id) || e.TaskID != id || e.Status != "failed" || e.ReviewRequired || e.Conformance != "not_run" || digest(e.Plan) != digest(p) {
			t.Fatalf("Apply lost published failure identity: %+v, %v; durable %q, %v", e, err, id, idErr)
		}
		fmt.Println("task-id", e.TaskID)
		return
	}
	// Keep the handle live but deliberately omit Close, including on normal
	// process exit. PID-file conventions cannot satisfy this test.
	fmt.Println("owned")
	_, _ = io.Copy(io.Discard, os.Stdin)
	runtime.KeepAlive(w)
	os.Exit(0)
}

func helper(t *testing.T, mode, dir, source string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	c := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWorkspaceProcessHelper$")
	c.Env = append(os.Environ(), "KEPUB_WORKSPACE_HELPER="+mode, "KEPUB_WORKSPACE_DIR="+dir, "KEPUB_WORKSPACE_SOURCE="+source)
	return c
}

func TestPublicApplyStartupFailureRetainsID(t *testing.T) {
	w, dir := makeWorkspace(t)
	p := planTitle(t, w, strings.Repeat("x", 4096))
	w.Close()
	out, err := helper(t, "limited-apply", dir, p.ID).CombinedOutput()
	if err != nil {
		t.Fatalf("public Apply fault: %v\n%s", err, out)
	}
	fields := strings.Fields(string(out))
	if len(fields) < 2 || fields[0] != "task-id" || !validID(fields[1]) {
		t.Fatalf("missing public Apply ID: %s", out)
	}
	id := fields[1]
	if _, err := os.Stat(filepath.Join(dir, "tasks/active/edit-start.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial start published: %v", err)
	}
	w, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	d, err := w.TaskDiff(id)
	if err != nil || d.Diff.Changed {
		t.Fatalf("published failure diff: %+v, %v", d, err)
	}
	if _, err := w.Reject(id); err != nil {
		t.Fatal(err)
	}
}

func TestRecoveryShortWriteCanBeRetried(t *testing.T) {
	for _, stage := range []string{"start", "result"} {
		t.Run(stage, func(t *testing.T) {
			w, dir := makeWorkspace(t)
			p := planTitle(t, w, strings.Repeat("x", 4096))
			if _, err := w.createCandidate(&p); err != nil {
				t.Fatal(err)
			}
			id, err := w.TaskID()
			if err != nil {
				t.Fatal(err)
			}
			if stage == "result" {
				if _, err := w.startExecution(p); err != nil {
					t.Fatal(err)
				}
			}
			w.Close()
			if out, err := helper(t, "limited-recovery", dir, "").CombinedOutput(); err != nil {
				t.Fatalf("fault subprocess: %v\n%s", err, out)
			}
			if _, err := os.Stat(filepath.Join(dir, "tasks/active/edit-"+stage+".json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("partial record was published: %v", err)
			}
			w, err = Open(dir)
			if err != nil {
				t.Fatal("retry with recovered I/O environment", err)
			}
			defer w.Close()
			d, err := w.TaskDiff(id)
			if err != nil || d.Diff.Changed {
				t.Fatalf("retry diff: %+v, %v", d, err)
			}
			if _, err := w.Reject(id); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestJSONPublicationDoesNotReplace(t *testing.T) {
	r, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := writeJSON(r, "record.json", "original"); err != nil {
		t.Fatal(err)
	}
	if err := r.Symlink("record.json", "link.json"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"record.json", "link.json"} {
		if err := writeJSON(r, name, "replacement"); !errors.Is(err, os.ErrExist) {
			t.Fatalf("replaced existing %s: %v", name, err)
		}
	}
	b, err := r.ReadFile("record.json")
	if err != nil || string(b) != "\"original\"\n" {
		t.Fatalf("original record changed: %q, %v", b, err)
	}
	if target, err := r.Readlink("link.json"); err != nil || target != "record.json" {
		t.Fatalf("link replaced: %q, %v", target, err)
	}
}

func TestCrossProcessLockAndExitRecovery(t *testing.T) {
	w, dir := makeWorkspace(t)
	if other, err := Open(dir); !errors.Is(err, ErrBusy) {
		if other != nil {
			other.Close()
		}
		t.Fatalf("same process competing owner: %v", err)
	}
	if out, err := helper(t, "probe", dir, "").CombinedOutput(); err != nil {
		t.Fatalf("child contention: %s: %v", out, err)
	}
	info, err := os.Stat(filepath.Join(dir, "owner.lock"))
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
	for _, killed := range []bool{false, true} {
		t.Run(fmt.Sprintf("killed=%t", killed), func(t *testing.T) {
			cmd := helper(t, "hold", dir, "")
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer stdin.Close()
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			s := bufio.NewScanner(stdout)
			if !s.Scan() || s.Text() != "owned" {
				t.Fatalf("child not ready: %q %v", s.Text(), s.Err())
			}
			if other, err := Open(dir); !errors.Is(err, ErrBusy) {
				if other != nil {
					other.Close()
				}
				t.Fatalf("parent contention: %v", err)
			}
			if killed {
				if err := cmd.Process.Kill(); err != nil {
					t.Fatal(err)
				}
			} else {
				stdin.Close()
			}
			if err := cmd.Wait(); !killed && err != nil {
				t.Fatal(err)
			}
			other, err := Open(dir)
			if err != nil {
				t.Fatalf("lock not reclaimed after process exit: %v", err)
			}
			other.Close()
			after, err := os.Stat(filepath.Join(dir, "owner.lock"))
			if err != nil || !os.SameFile(info, after) {
				t.Fatalf("lock inode was unlinked/replaced: %v", err)
			}
		})
	}
}

func TestCrossProcessCreateRace(t *testing.T) {
	source, _ := fixture(t)
	base := t.TempDir()
	dir := filepath.Join(base, "workspace")
	var commands []*exec.Cmd
	var inputs []io.WriteCloser
	var outputs []*bufio.Scanner
	for range 4 {
		cmd := helper(t, "create", dir, source)
		in, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		out, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(out)
		if !scanner.Scan() || scanner.Text() != "ready" {
			t.Fatalf("create child not ready: %s", scanner.Text())
		}
		commands, inputs, outputs = append(commands, cmd), append(inputs, in), append(outputs, scanner)
	}
	for _, in := range inputs {
		if _, err := io.WriteString(in, "go\n"); err != nil {
			t.Fatal(err)
		}
		in.Close()
	}
	winners := 0
	for i, cmd := range commands {
		s := outputs[i]
		if !s.Scan() {
			t.Fatal("missing create result")
		}
		if s.Text() == "created" {
			winners++
		} else if s.Text() != "exists" {
			t.Fatalf("unexpected create result: %s", s.Text())
		}
		for s.Scan() {
		}
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	if winners != 1 {
		t.Fatalf("winners = %d", winners)
	}
	entries, err := os.ReadDir(base)
	if err != nil || len(entries) != 1 || entries[0].Name() != "workspace" {
		t.Fatalf("failed creators leaked files: %v %v", entries, err)
	}
	w, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	w.Close()
}

func TestSpecialFilesAndDiskFailure(t *testing.T) {
	w, dir := makeWorkspace(t)
	pub, err := w.NewCandidate()
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(filepath.Join(pub, "pipe"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Checkpoint(); err == nil {
		t.Fatal("FIFO accepted")
	}
	if err := os.Remove(filepath.Join(pub, "pipe")); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() == 0 {
		t.Skip("permission-based I/O failure requires non-root")
	}
	if err := os.Chmod(filepath.Join(dir, "staging"), 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(filepath.Join(dir, "staging"), 0700)
	if _, err := w.Checkpoint(); err == nil || !strings.Contains(err.Error(), "permission") {
		t.Fatalf("expected filesystem failure: %v", err)
	}
	assertEmpty(t, filepath.Join(dir, "tasks/active/checkpoints"))
	assertEmpty(t, filepath.Join(dir, "staging"))
	if treeAt(t, pub).SHA256 != w.State().Tree.SHA256 {
		t.Fatal("failed checkpoint changed candidate")
	}
	if err := os.Chmod(filepath.Join(dir, "staging"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Checkpoint(); err != nil {
		t.Fatalf("failed checkpoint poisoned handle: %v", err)
	}
}

func TestLateIOFailuresAndRecovery(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission-based I/O failure requires non-root")
	}
	w, dir := makeWorkspace(t)
	pub, err := w.NewCandidate()
	if err != nil {
		t.Fatal(err)
	}
	cpDir := filepath.Join(dir, "tasks/active/checkpoints")
	if err := os.Chmod(cpDir, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(cpDir, 0700)
	if _, err := w.Checkpoint(); err == nil {
		t.Fatal("checkpoint publication into read-only directory succeeded")
	}
	assertEmpty(t, cpDir)
	assertEmpty(t, filepath.Join(dir, "staging"))
	if err := os.Chmod(cpDir, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := w.Checkpoint()
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(pub, "new-file"), []byte("before failed restore"))
	work := filepath.Dir(pub)
	if err := os.Chmod(work, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(work, 0700)
	if err := w.Restore(s.ID); err == nil {
		t.Fatal("restore rename from read-only directory succeeded")
	}
	assertBytes(t, filepath.Join(pub, "new-file"), []byte("before failed restore"))
	if _, err := w.Candidate(); !errors.Is(err, ErrRecovery) {
		t.Fatalf("failed journaled restore did not block handle: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, restoreJournal)); err != nil {
		t.Fatalf("missing recovery record: %v", err)
	}
	if err := os.Chmod(work, 0700); err != nil {
		t.Fatal(err)
	}
	w.Close()
	other, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if treeAt(t, pub).SHA256 != s.Tree.SHA256 {
		t.Fatal("reopen did not finish interrupted restore")
	}
	assertEmpty(t, filepath.Join(dir, "staging"))
	assertEmpty(t, filepath.Join(dir, "journal"))
}

func TestPublishDoesNotReplaceLateEmptyDestination(t *testing.T) {
	r, err := openDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.Mkdir("stage", 0700); err != nil {
		t.Fatal(err)
	}
	if err := r.WriteFile("stage/data", []byte("staged"), 0600); err != nil {
		t.Fatal(err)
	}
	// Models a destination created after Create's initial existence check.
	if err := r.Mkdir("late", 0700); err != nil {
		t.Fatal(err)
	}
	before, _ := r.Stat("late")
	if err := publish(r, "stage", "late"); !errors.Is(err, os.ErrExist) {
		t.Fatalf("no-replace publication: %v", err)
	}
	after, _ := r.Stat("late")
	if !os.SameFile(before, after) {
		t.Fatal("late destination replaced")
	}
	b, err := r.ReadFile("stage/data")
	if err != nil || string(b) != "staged" {
		t.Fatalf("stage lost: %s %v", b, err)
	}
}
