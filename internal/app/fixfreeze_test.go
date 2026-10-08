package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/fix"
	"github.com/LeviTK/Kepub/internal/validation"
)

// TestFixOutputExists pins the frozen OUTPUT_EXISTS refusal for an existing
// external output without touching its bytes, including the budget refusal.
func TestFixOutputExists(t *testing.T) {
	w, ws := fixDeltaFixture(t)
	all := fixDeltaPropose(t, w, fix.ModeAll)
	p := fixDeltaPropose(t, w, fix.ModeExplicit, all.Repairs[0].RepairID)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	existing := filepath.Join(dir, "request.json")
	sentinel := []byte("NUL-SENTINEL\x00END")
	if err := os.WriteFile(existing, sentinel, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := FixPropose(t.Context(), ws, p.Repairs[0].RepairID, true, existing, true); err == nil {
		t.Fatal("existing output accepted by propose")
	} else {
		fixDeltaFault(t, err, "OUTPUT_EXISTS")
	}
	if _, err := FixDelta(context.Background(), ws, "initial", "", "", existing, validation.Options{}, true); err == nil {
		t.Fatal("existing output accepted by delta")
	} else {
		fixDeltaFault(t, err, "OUTPUT_EXISTS")
	}
	if got, err := os.ReadFile(existing); err != nil || string(got) != string(sentinel) {
		t.Fatalf("existing output changed: %q %v", got, err)
	}
	if _, err := FixPropose(t.Context(), ws, p.Repairs[0].RepairID, true, filepath.Join(ws, "out.json"), true); err == nil {
		t.Fatal("workspace output accepted")
	} else {
		fixDeltaFault(t, err, "INVALID_OUTPUT")
	}
}

// TestFixEmitRequestBudget pins the inherited 256-operation budget: the
// boundary emits and 257 refuses with INVALID_OPERATIONS before publishing.
func TestFixEmitRequestBudget(t *testing.T) {
	for _, count := range []int{256, 257} {
		t.Run(map[int]string{256: "boundary-256", 257: "over-257"}[count], func(t *testing.T) {
			extras := strings.Repeat(`<title epub:type="secrecy">Extra</title>`, count-2)
			src := strings.Replace(fixDeltaChapter1, "</head>", extras+"</head>", 1)
			w, ws := fixDeltaFixtureWith(t, map[string]string{"EPUB/chapter1.xhtml": src})
			all := fixDeltaPropose(t, w, fix.ModeAll)
			if len(all.Repairs) != count {
				t.Fatalf("fixture repairs: %d want %d", len(all.Repairs), count)
			}
			ids := make([]string, 0, len(all.Repairs))
			for _, r := range all.Repairs {
				ids = append(ids, r.RepairID)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(t.TempDir(), "request.json")
			_, err := FixPropose(t.Context(), ws, strings.Join(ids, ","), true, out, true)
			if count == 256 {
				if err != nil {
					t.Fatalf("boundary emit: %v", err)
				}
				b, err := os.ReadFile(out)
				if err != nil {
					t.Fatal(err)
				}
				var req struct {
					Operations []fix.Operation `json:"operations"`
				}
				if err := json.Unmarshal(b, &req); err != nil || len(req.Operations) != count {
					t.Fatalf("boundary request: %d %v", len(req.Operations), err)
				}
				return
			}
			if err == nil {
				t.Fatal("over-budget emit published a request")
			}
			fixDeltaFault(t, err, "INVALID_OPERATIONS")
			if _, err := os.Lstat(out); !os.IsNotExist(err) {
				t.Fatal("over-budget emit wrote an output file")
			}
		})
	}
}

// TestFixDeltaFrozenSide pins that each side binds its own verified inventory
// and that the checker reads the frozen private snapshot, never the live path.
func TestFixDeltaFrozenSide(t *testing.T) {
	if os.Getenv("KEPUB_EPUBCHECK_JAR") == "" {
		t.Skip("real pinned EPUBCheck required; set KEPUB_EPUBCHECK_JAR")
	}
	t.Run("historical-identity", func(t *testing.T) {
		w, _ := fixDeltaFixture(t)
		defer w.Close()
		initial, err := w.FixDeltaSnapshot("initial", "")
		if err != nil {
			t.Fatal(err)
		}
		defer initial.Close()
		all := fixDeltaPropose(t, w, fix.ModeAll)
		p := fixDeltaPropose(t, w, fix.ModeExplicit, all.Repairs[0].RepairID, all.Repairs[1].RepairID)
		e := fixDeltaApply(t, w, p)
		decision, err := w.Accept(context.Background(), e.TaskID, validation.Options{Timeout: 2 * time.Minute})
		if err != nil {
			t.Fatal(err)
		}
		historical, err := w.FixDeltaSnapshot("initial", "")
		if err != nil {
			t.Fatal(err)
		}
		defer historical.Close()
		current, err := w.FixDeltaSnapshot(decision.RevisionID, "")
		if err != nil {
			t.Fatal(err)
		}
		defer current.Close()
		if current.Snapshot.Workspace.InputTreeSHA256 != current.TreeSHA256 {
			t.Fatal("current inventory control")
		}
		if historical.TreeSHA256 != initial.TreeSHA256 || historical.Snapshot.Workspace.InputTreeSHA256 != initial.TreeSHA256 {
			t.Fatalf("historical identity: %s vs %s", historical.Snapshot.Workspace.InputTreeSHA256, initial.TreeSHA256)
		}
	})
	t.Run("private-snapshot-drift", func(t *testing.T) {
		w, _ := fixDeltaFixture(t)
		defer w.Close()
		all := fixDeltaPropose(t, w, fix.ModeAll)
		task := fixDeltaApply(t, w, fixDeltaPropose(t, w, fix.ModeExplicit, all.Repairs[0].RepairID, all.Repairs[1].RepairID))
		side, err := w.FixDeltaSnapshot("", task.TaskID)
		if err != nil {
			t.Fatal(err)
		}
		defer side.Close()
		if err := os.WriteFile(filepath.Join(side.Dir, "EPUB/chapter1.xhtml"), []byte(fixDeltaChapter1), 0o600); err != nil {
			t.Fatal(err)
		}
		built, err := buildDeltaSide(context.Background(), side, validation.Options{Timeout: 2 * time.Minute})
		if err != nil {
			t.Fatalf("frozen side: %v", err)
		}
		if report := built.Report.(validation.Report); report.InputTreeSHA256 != side.TreeSHA256 {
			t.Fatalf("checker read the live path: %s vs %s", report.InputTreeSHA256, side.TreeSHA256)
		}
		for _, m := range built.Checks {
			if m.CheckerID == "epubcheck" {
				if m.ToolSHA256 != validation.ToolSHA256 || m.Profile == "" || m.Flags == "" || m.RunStatus != "completed" {
					t.Fatalf("checker metadata: %+v", m)
				}
			}
		}
	})
}
