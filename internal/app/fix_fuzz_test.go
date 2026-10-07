package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/fix"
)

// fixScenarioWant is the independent byte expectation of one legal menu entry.
func fixScenarioWant(kind int) []byte {
	want := fixDeltaChapter1
	if kind == 0 || kind == 2 || kind == 3 {
		want = strings.Replace(want, ` epub:type="secrecy"`, "", 1)
	}
	if kind == 1 || kind == 2 || kind == 3 {
		want = strings.Replace(want, "?q=1#start2", "#start2", 1)
	}
	return []byte(want)
}

// fixScenario executes one deterministic menu entry of the finite acceptance
// menu. Legal families must be accepted and must reach the independent byte
// comparison; an unexpected refusal of a legal family is a FAIL, and refusal
// families must carry their exact frozen code.
func fixScenario(t *testing.T, kind int) {
	t.Helper()
	w, _ := fixDeltaFixture(t)
	defer w.Close()
	all := fixDeltaPropose(t, w, fix.ModeAll)
	if len(all.Repairs) != 2 {
		t.Fatalf("menu fixture repairs: %+v", all.Repairs)
	}
	fr1, fr2 := all.Repairs[0].RepairID, all.Repairs[1].RepairID
	wantCode := ""
	var p fix.Proposal
	var ops []fix.Operation
	switch kind {
	case 0:
		p = fixDeltaPropose(t, w, fix.ModeExplicit, fr1)
	case 1:
		p = fixDeltaPropose(t, w, fix.ModeExplicit, fr2)
	case 2, 3, 5, 6, 7:
		p = fixDeltaPropose(t, w, fix.ModeExplicit, fr1, fr2)
	case 4:
		p = all
	default:
		t.Fatalf("unknown menu kind %d", kind)
	}
	ops = append([]fix.Operation{}, p.Derived.Operations...)
	switch kind {
	case 3:
		ops = []fix.Operation{p.Derived.Operations[1], p.Derived.Operations[0]}
	case 4:
		wantCode = "SELECTION_INVALID"
	case 5:
		ops = ops[:1]
		wantCode = "INVALID_OPERATIONS"
	case 6:
		ops = []fix.Operation{ops[0], ops[0]}
		wantCode = "INVALID_OPERATIONS"
	case 7:
		ops = append(ops, ops[0])
		wantCode = "INVALID_OPERATIONS"
	}
	plan, err := w.Plan(fixDeltaRequest(t, p, ops))
	if wantCode != "" {
		fixDeltaFault(t, err, wantCode)
		t.Logf("menu %d: refused with %s", kind, wantCode)
		return
	}
	if err != nil {
		t.Fatalf("menu %d: legal family refused: %v", kind, err)
	}
	b, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	e, err := w.Apply(b)
	if err != nil {
		t.Fatalf("menu %d: legal family apply refused: %v", kind, err)
	}
	dir := fixDeltaCandidate(t, w)
	got, err := os.ReadFile(filepath.Join(dir, "EPUB/chapter1.xhtml"))
	if err != nil {
		t.Fatal(err)
	}
	if want := fixScenarioWant(kind); !bytes.Equal(got, want) {
		t.Fatalf("menu %d candidate bytes:\n got %q\nwant %q", kind, got, want)
	}
	for name, want := range map[string]string{
		"EPUB/chapter2.xhtml":    fixDeltaChapter2,
		"EPUB/nav.xhtml":         fixDeltaNav,
		"EPUB/package.opf":       fixDeltaOPF,
		"META-INF/container.xml": `<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"mimetype":               "application/epub+zip",
	} {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != want {
			t.Fatalf("menu %d untouched resource %s changed: %q", kind, name, b)
		}
	}
	t.Logf("menu %d: accepted, candidate bytes match the independent oracle (task %s)", kind, e.TaskID)
}

// TestFixProposalFuzzCalibration runs the finite menu deterministically before
// any live fuzz: single FR-1, single FR-2, both orders, and the four refusal
// families each with their exact code.
func TestFixProposalFuzzCalibration(t *testing.T) {
	for kind := 0; kind < 8; kind++ {
		t.Run(fmt.Sprintf("menu-%d", kind), func(t *testing.T) { fixScenario(t, kind) })
	}
}

// FuzzFixProposalPlanApply replays the calibrated menu under live fuzz. The
// legal families must plan, apply and match the independent bytes; the refusal
// families must keep their exact code.
func FuzzFixProposalPlanApply(f *testing.F) {
	for _, seed := range []byte{0, 1, 2, 3, 4, 5, 6, 7} {
		f.Add([]byte{seed})
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		kind := 0
		if len(data) > 0 {
			kind = int(data[0] % 8)
		}
		fixScenario(t, kind)
	})
}
