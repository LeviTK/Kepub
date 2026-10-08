package workspace

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

func r6Profile(t *testing.T, f r6Fixture, operation, mode string) {
	t.Helper()
	temp := t.TempDir()
	t.Setenv("TMPDIR", temp)
	peak := int64(0)
	stages := map[string]bool{}
	policy := defaultResourceIO(r3HookContext{Context: context.Background(), check: func() {
		var payload int64
		err := filepath.WalkDir(temp, func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if name == temp {
				return nil
			}
			if entry.IsDir() {
				if filepath.Dir(name) == temp {
					stages[name] = true
				}
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			payload += info.Size()
			return nil
		})
		if err != nil {
			t.Fatal("temporary payload measurement", err)
		}
		peak = max(peak, payload)
	}})
	visits, originals := 0, 0
	policy.onRevisionRead = func(string) { visits++ }
	policy.onOriginalRead = func() { originals++ }
	before := map[string]uint64{}
	if runtime.GOOS == "linux" {
		before = r6ProcessIO(t)
	}
	r6Query(t, f, operation, policy)
	io := map[string]uint64{}
	if runtime.GOOS == "linux" {
		for name, value := range r6ProcessIO(t) {
			io[name] = value - before[name]
		}
	}
	entries, err := os.ReadDir(temp)
	if err != nil || len(entries) != 0 {
		t.Fatal("read operation leaked a private snapshot", entries, err)
	}
	if operation == "content" || operation == "search" {
		want := int64(0)
		for _, data := range f.Files {
			want += int64(len(data))
		}
		if len(stages) != 1 || peak != want {
			t.Fatal("complete private snapshot/copy contract changed", len(stages), peak, want)
		}
	}
	data, err := json.Marshal(map[string]any{"operation": operation, "mode": mode, "depth": f.Depth,
		"revision_visits": visits, "original_reads": originals, "self_io": io, "snapshot_directories": len(stages),
		"sampled_peak_temp_payload_bytes": peak, "pid": os.Getpid()})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("R6_PROFILE=%s", data)
}

func TestR6QueryProfiles(t *testing.T) {
	fixtures := map[string]r6Fixture{}
	if root := os.Getenv("KEPUB_R6_FIXTURES"); root != "" {
		data, err := os.ReadFile(filepath.Join(root, "fixtures.json"))
		if err != nil || json.Unmarshal(data, &fixtures) != nil {
			t.Fatal("prepared fixture identity", err)
		}
	} else {
		fixtures["1/1024"] = r6History(t, t.TempDir(), 1, 1024)
	}
	names := []string{}
	for name := range fixtures {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, operation := range []string{"content", "search", "diff", "old-status"} {
			for _, mode := range []string{"first-open", "same-process-reopen"} {
				t.Run(name+"/"+operation+"/"+mode, func(t *testing.T) {
					r6Profile(t, fixtures[name], operation, mode)
				})
			}
		}
	}
}

// This is a fresh process, not a cold-disk claim: OS page caches are not flushed.
func TestR6ColdProcess(t *testing.T) {
	if input := os.Getenv("KEPUB_R6_CHILD_INPUT"); input != "" {
		data, err := os.ReadFile(input)
		var f r6Fixture
		if err != nil || json.Unmarshal(data, &f) != nil {
			t.Fatal("cold process fixture", err)
		}
		r6Profile(t, f, "content", "fresh-process")
		return
	}
	f := r6History(t, t.TempDir(), 1, 1024)
	input := filepath.Join(t.TempDir(), "fixture.json")
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, data, 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		cmd := exec.Command(os.Args[0], "-test.run=^TestR6ColdProcess$", "-test.v")
		cmd.Env = append(os.Environ(), "KEPUB_R6_CHILD_INPUT="+input)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("actual fresh process %d: %v\n%s", i, err, output)
		}
		t.Logf("fresh process %d: %s", i, output)
	}
}
