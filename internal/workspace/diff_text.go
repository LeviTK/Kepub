package workspace

import (
	"crypto/sha256"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/LeviTK/Kepub/internal/xmltext"
)

const diffResourceLimit = 256 << 10
const diffDisplayLimit = 1 << 20

// TaskDiffText uses the same verified actual-tree review as JSON, under the same
// lock. Display omissions never erase a change or substitute planned bytes.
func (w *Workspace) TaskDiffText(id string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r, err := w.taskDiff(id)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	fmt.Fprintf(&out, "Task %s · base %s · matches execution: %t\n", r.TaskID, r.BaseRevision, r.MatchesExecution)
	fmt.Fprintf(&out, "Tree %s → %s\n", r.Diff.BeforeSHA256, r.Diff.AfterSHA256)
	if !r.Diff.Changed {
		out.WriteString("No byte changes.\n")
	}
	remaining := diffDisplayLimit
	for _, c := range r.Diff.Changes {
		fmt.Fprintf(&out, "\n%s %q\n", c.Kind, c.Path)
		for _, side := range []struct {
			label string
			e     *Entry
		}{{"before", c.Before}, {"after", c.After}} {
			if side.e == nil {
				fmt.Fprintf(&out, "  %s: absent\n", side.label)
			} else {
				fmt.Fprintf(&out, "  %s: %s · %d bytes · %s\n", side.label, side.e.Type, side.e.Size, side.e.SHA256)
			}
		}
		before, why, err := w.diffText(revisionPath(w.current), c.Before, remaining)
		if err != nil {
			return "", err
		}
		if why != "" {
			fmt.Fprintf(&out, "  Text omitted: %s\n", why)
			continue
		}
		after, why, err := w.diffText(candidate, c.After, remaining-len(before))
		if err != nil {
			return "", err
		}
		if why != "" {
			fmt.Fprintf(&out, "  Text omitted: %s\n", why)
			continue
		}
		remaining -= len(before) + len(after)
		if before == after {
			out.WriteString("  Decoded text unchanged; byte representation/presence/type changed.\n")
			continue
		}
		lines := func(s string) []string {
			parts := strings.SplitAfter(s, "\n")
			if parts[len(parts)-1] == "" {
				parts = parts[:len(parts)-1]
			}
			return parts
		}
		old, new := lines(before), lines(after)
		start := 0
		for start < min(len(old), len(new)) && old[start] == new[start] {
			start++
		}
		end := 0
		for end < min(len(old)-start, len(new)-start) && old[len(old)-1-end] == new[len(new)-1-end] {
			end++
		}
		fmt.Fprintf(&out, "--- before/%s\n+++ after/%s\n@@ -%d,%d +%d,%d @@ (line endings/control characters escaped; display, not a patch)\n", strconv.Quote(c.Path), strconv.Quote(c.Path), start+1, len(old)-start-end, start+1, len(new)-start-end)
		for _, side := range []struct {
			prefix string
			lines  []string
		}{{"-", old[start : len(old)-end]}, {"+", new[start : len(new)-end]}} {
			for _, line := range side.lines {
				q := strconv.QuoteToGraphic(line)
				fmt.Fprintf(&out, "%s%s\n", side.prefix, q[1:len(q)-1])
			}
		}
	}
	actual, err := w.hashAt(candidate)
	if err != nil {
		return "", err
	}
	if actual.SHA256 != r.Diff.AfterSHA256 {
		return "", ErrCandidateDrift
	}
	if err := w.verifyBaseline(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func (w *Workspace) diffText(root string, e *Entry, budget int) (string, string, error) {
	if e == nil {
		return "", "", nil
	}
	if e.Type != "file" {
		return "", "directory/type change", nil
	}
	if e.Size > diffResourceLimit || e.Size > int64(budget) {
		return "", "display budget exceeded", nil
	}
	f, err := openRegular(w.root, root+"/"+e.Path)
	if err != nil {
		return "", "", err
	}
	b, err := io.ReadAll(io.LimitReader(f, diffResourceLimit+1))
	closeErr := f.Close()
	if err != nil {
		return "", "", err
	}
	if closeErr != nil {
		return "", "", closeErr
	}
	if int64(len(b)) != e.Size || fmt.Sprintf("%x", sha256.Sum256(b)) != e.SHA256 {
		return "", "", ErrCandidateDrift
	}
	s, err := xmltext.DecodeText(b)
	if err != nil {
		return "", "binary or undecodable resource", nil
	}
	if len(s) > budget || len(s) > diffResourceLimit {
		return "", "decoded display budget exceeded", nil
	}
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return "", "binary/control-containing resource", nil
		}
	}
	return s, "", nil
}
