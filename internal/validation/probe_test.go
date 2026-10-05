package validation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/fault"
)

func TestProbeVersionsIntegrityAndFailure(t *testing.T) {
	t.Setenv("KEPUB_EPUBCHECK_JAR", "")
	for _, tc := range []struct {
		name, script, status string
		exit                 int
	}{
		{"old", `echo 'openjdk version "11.0.1"' >&2`, "unavailable", 0},
		{"modern", `echo 'openjdk version "21.0.8"' >&2`, "ready", 0},
		{"unrecognized", `echo 'private-secret text'`, "unavailable", 0},
		{"nonzero", `echo 'openjdk version "21.0.8"' >&2; exit 7`, "failed", 5},
		{"overflow", `while :; do printf 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'; done`, "failed", 5},
		{"timeout", `/bin/sleep 30`, "failed", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			java := filepath.Join(t.TempDir(), "java")
			if e := os.WriteFile(java, []byte("#!/bin/sh\n"+tc.script+"\n"), 0700); e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()
			j, c, e := Probe(ctx, Options{Java: java})
			if j.Status != tc.status || c.Status != "unavailable" {
				t.Fatal(j, c, e)
			}
			if tc.exit == 0 && e != nil {
				t.Fatal(e)
			}
			if tc.exit != 0 {
				var f *fault.Error
				if !errors.As(e, &f) || f.Exit != tc.exit {
					t.Fatal(e)
				}
			}
		})
	}
	java := filepath.Join(t.TempDir(), "java")
	if e := os.WriteFile(java, []byte("#!/bin/sh\nif [ \"$1\" != '-version' ]; then exit 99; fi\necho 'openjdk version \"21.0.8\"' >&2\n"), 0700); e != nil {
		t.Fatal(e)
	}
	jar := filepath.Join(t.TempDir(), "epubcheck.jar")
	if e := os.WriteFile(jar, []byte("untrusted"), 0600); e != nil {
		t.Fatal(e)
	}
	j, c, e := Probe(context.Background(), Options{Java: java, JAR: jar})
	if e != nil || j.Status != "ready" || c.Status != "unavailable" {
		t.Fatal(j, c, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, e = Probe(ctx, Options{})
	var f *fault.Error
	if !errors.As(e, &f) || f.Exit != 130 {
		t.Fatal(e)
	}
}

func TestProbeRealPinnedChecker(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("requires installed pinned checker")
	}
	j, c, e := Probe(context.Background(), Options{})
	if e != nil || j.Status != "ready" || c.Status != "ready" || c.Version != "5.3.0" {
		t.Fatal(j, c, e)
	}
}

func TestProbeReclaimsClosedPipeDescendant(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "late-write")
	java := filepath.Join(dir, "java")
	script := "#!/bin/sh\n(/bin/sleep 1; printf leaked > '" + marker + "') </dev/null >/dev/null 2>&1 &\necho 'openjdk version \"21.0.8\"' >&2\n"
	if e := os.WriteFile(java, []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	_, _, e := Probe(context.Background(), Options{Java: java})
	var f *fault.Error
	if !errors.As(e, &f) || f.Exit != 5 {
		t.Fatal("residual writer must not report ready", e)
	}
	time.Sleep(1200 * time.Millisecond)
	if _, e := os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("late writer survived", e)
	}
}
