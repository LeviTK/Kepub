package workspace

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/validation"
)

// The fixture becomes a revision only through real formal acceptance. The
// observation seam does not bypass any read, hash, provenance or checker gate.
func TestR6SingleCallRevisionVerification(t *testing.T) {
	w, _, _ := legalWorkspace(t, "3.0")
	defer w.Close()
	e := applyPlan(t, w, fieldPlan(t, w, "title", "title", "Title", "Reviewed"))
	d, err := w.Accept(t.Context(), e.TaskID, validation.Options{})
	if err != nil || d.Status != "accepted" {
		t.Fatal("real acceptance required", d, err)
	}
	reads := 0
	w.resources.onRevisionRead = func(id string) {
		if id != d.RevisionID {
			t.Fatal("unexpected revision", id)
		}
		reads++
	}
	a, tree, r, err := w.AcceptedSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if r.ID != d.RevisionID || tree.SHA256 != d.TreeSHA256 {
		t.Fatal("snapshot lost its approved revision/tree binding", r, tree)
	}
	if reads != 1 {
		t.Fatalf("one locked AcceptedSnapshot re-verifies the complete revision source %d times; want 1", reads)
	}
}

func TestR6VerificationIsCallLocal(t *testing.T) {
	f := r6History(t, t.TempDir(), 2, 1024)
	for _, boundary := range []string{"legal", "original", "initial", "ancestor", "current", "checkpoint", "execution", "pointer", "closed", "cancelled"} {
		t.Run(boundary, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "workspace")
			r6CopyDirectory(t, f.Directory, dir)
			w, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			a, _, r, err := w.AcceptedSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			// Reusing loadCurrent's result must not expose the private base slice.
			basePath := w.base.Entries[0].Path
			r.Tree.Entries[0].Path = "caller changed an observation"
			if w.base.Entries[0].Path != basePath {
				t.Fatal("public revision observation aliases the private base")
			}
			name := ""
			switch boundary {
			case "original":
				name = "original/book.epub"
			case "initial":
				name = revision + "/EPUB/style.css"
			case "ancestor":
				name = revisionPath(f.FirstRevision) + "/EPUB/style.css"
			case "current":
				name = revisionPath(f.Revision) + "/EPUB/style.css"
			case "checkpoint":
				var e Execution
				if err := readEditJSON(w.root, "tasks/"+f.FirstTask+"/edit-result.json", &e); err != nil {
					t.Fatal(err)
				}
				name = "tasks/" + f.FirstTask + "/checkpoints/" + e.Checkpoint + "/pub/EPUB/style.css"
			case "execution":
				name = "tasks/" + f.FirstTask + "/edit-result.json"
			case "pointer":
				put(t, filepath.Join(dir, "accepted.json"), editJSON(t, acceptedPointer{1, w.id, f.FirstRevision}))
			case "closed":
				if err := w.Close(); err != nil {
					t.Fatal(err)
				}
			case "cancelled":
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				w.resources.ctx = ctx
			}
			if name != "" {
				before := readResource(t, filepath.Join(dir, filepath.FromSlash(name)))
				changed := bytes.Clone(before)
				changed[len(changed)-1] ^= 1 // Same size; no mtime/size cache can authorize it.
				put(t, filepath.Join(dir, filepath.FromSlash(name)), changed)
			}
			// A previously returned snapshot stays frozen, even if the unqueried
			// live stylesheet has since changed. A new query must reverify it.
			got, err := a.Read("EPUB/style.css", archive.DefaultLimits.FileBytes)
			if err != nil || !bytes.Equal(got, f.Files["EPUB/style.css"]) {
				t.Fatal("frozen bytes borrowed from a live reader", err)
			}
			next, _, _, err := w.AcceptedSnapshot()
			if next != nil {
				next.Close()
			}
			if boundary == "legal" {
				if err != nil {
					t.Fatal("next legal query rejected", err)
				}
			} else if err == nil {
				t.Fatal("previous verification reused across mutation/lifetime", boundary)
			}
			if boundary == "closed" && !errors.Is(err, ErrClosed) || boundary == "cancelled" && !errors.Is(err, context.Canceled) {
				t.Fatal("lifetime error lost", err)
			}
		})
	}
}

func TestR6Schema7SnapshotRevalidatesSource(t *testing.T) {
	w, dir, source := structureWorkspace(t, map[string]string{"EPUB/chapter1.xhtml": fixSourceSVG()})
	defer w.Close()
	original := readResource(t, source)
	p, err := w.Plan(fixSchema7JSON(t, fixSourceProposal(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	e := applyPlan(t, w, p)
	d, err := w.Accept(t.Context(), e.TaskID, validation.Options{})
	if err != nil || d.Status != "accepted" {
		t.Fatal("schema7 real acceptance", err)
	}
	a, _, r, err := w.AcceptedSnapshot()
	if err != nil {
		t.Fatal("legal schema7 snapshot", err)
	}
	a.Close()
	// Operations and publication bytes stay legal; re-sign every saved copy,
	// including the enclosing revision digest, to isolate complete-source truth.
	p.Proposal.Repairs[0].Risk = "forged safe approved repair"
	if err := p.Proposal.Sign(); err != nil {
		t.Fatal(err)
	}
	task := "tasks/" + e.TaskID
	var start Execution
	if err := readEditJSON(w.root, task+"/edit-start.json", &start); err != nil {
		t.Fatal(err)
	}
	start.Plan, e.Plan = p, p
	for name, record := range map[string]any{
		task + "/edit-intent.json": p, task + "/edit-start.json": start, task + "/edit-result.json": e,
		"plans/" + p.ID + ".json": p, "plans/" + p.ID + ".used.json": planUse{1, e.TaskID, digest(p)},
	} {
		put(t, filepath.Join(dir, filepath.FromSlash(name)), editJSON(t, record))
	}
	r.ExecutionSHA256 = digest(e)
	put(t, filepath.Join(dir, "revisions", r.ID, "revision.json"), editJSON(t, r))
	a, _, _, err = w.AcceptedSnapshot()
	if a != nil {
		a.Close()
	}
	if !errors.Is(err, ErrStalePlan) {
		t.Fatal("query reused approval instead of re-deriving schema7", err)
	}
	assertBytes(t, source, original)
	assertBytes(t, filepath.Join(dir, revisionPath(d.RevisionID), "EPUB/chapter1.xhtml"), fixSourceWant(fixSourceSVG()))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	opened, err := Open(dir)
	if opened != nil {
		opened.Close()
	}
	if !errors.Is(err, ErrStalePlan) {
		t.Fatal("reopen reused a previous query context", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "journal/settlement.json")); !os.IsNotExist(err) {
		t.Fatal("query manufactured a recovery decision", err)
	}
}
