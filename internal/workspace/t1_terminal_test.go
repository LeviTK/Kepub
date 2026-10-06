package workspace

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestT1StatusExactHistoryAndConsumption(t *testing.T) {
	w, dir := makeWorkspace(t)
	defer w.Close()
	p, err := w.Plan(editJSON(t, contentRequest(t, w, "书/Text/第二 章.xhtml", "reviewed")))
	if err != nil {
		t.Fatal(err)
	}
	e := applyPlan(t, w, p)
	s, err := w.TaskStatus(e.TaskID)
	if err != nil || s.Status != "review_required" || !s.MatchesExecution || !s.ReviewRequired || s.BaseRevision != "initial" || s.Decision != nil {
		t.Fatal(s, err)
	}
	if _, err := w.TaskStatus(randomID()); !errors.Is(err, ErrTaskConflict) {
		t.Fatal("unknown substituted", err)
	}
	put(t, filepath.Join(dir, candidate, "书/Text/第二 章.xhtml"), []byte("outside writer"))
	s, err = w.TaskStatus(e.TaskID)
	if err != nil || s.Status != "candidate_drift" || s.MatchesExecution || s.ReviewRequired || s.ExecutionStatus != "review_required" {
		t.Fatal("drift inherited review", s, err)
	}
	if _, err := w.Reject(e.TaskID); err != nil {
		t.Fatal(err)
	}
	if _, err := w.NewCandidate(); err != nil {
		t.Fatal(err)
	}
	s, err = w.TaskStatus(e.TaskID)
	if err != nil || s.Status != "rejected" || s.TaskID != e.TaskID || s.Decision.Status != "rejected" {
		t.Fatal("historical task replaced by active", s, err)
	}
	if err := os.Remove(filepath.Join(dir, "plans", p.ID+".used.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.TaskStatus(e.TaskID); err == nil {
		t.Fatal("missing historical provenance accepted")
	}
}

func TestT1TextDiffOmissionsAndLineEndings(t *testing.T) {
	w, dir := makeWorkspace(t)
	defer w.Close()
	if _, err := w.NewCandidate(); err != nil {
		t.Fatal(err)
	}
	id, err := w.TaskID()
	if err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(dir, candidate, "new.txt"), []byte("actual\r\nlast"))
	put(t, filepath.Join(dir, candidate, "new.bin"), []byte{0, 255, 27})
	put(t, filepath.Join(dir, candidate, "new.large"), bytes.Repeat([]byte("x"), diffResourceLimit+1))
	if err := os.Remove(filepath.Join(dir, candidate, "书/Text/-first.xhtml")); err != nil {
		t.Fatal(err)
	}
	text, err := w.TaskDiffText(id)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{`new.txt`, `+actual\r\n`, `+last`, `new.bin`, `binary or undecodable`, `new.large`, `display budget exceeded`, `deleted`, `-<html`, `bytes`} {
		if !strings.Contains(text, s) {
			t.Fatal("missing actual diff fact", s, text)
		}
	}
	if strings.Contains(text, "\x1b") {
		t.Fatal("terminal escape emitted")
	}
	r, err := w.TaskDiff(id)
	if err != nil || len(r.Diff.Changes) != 4 {
		t.Fatal("display altered JSON inventory", r, err)
	}
}
