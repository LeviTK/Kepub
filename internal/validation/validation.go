package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/references"
)

type Report struct {
	Status          string                   `json:"status"`
	InputTreeSHA256 string                   `json:"inputTreeSha256"`
	ArchiveSHA256   string                   `json:"archiveSha256"`
	ConfigSHA256    string                   `json:"configSha256"`
	Checks          []Check                  `json:"checks"`
	Diagnostics     []Diagnostic             `json:"diagnostics"`
	Limitations     []publication.Limitation `json:"limitations"`
	Draft           bool                     `json:"draft"`
}

func newReport(hash string, o Options) Report {
	b, _ := json.Marshal(o)
	h, _ := json.Marshal(map[string]any{"policy": "kepub-validation-v1", "options": json.RawMessage(b), "epubcheckVersion": Version, "toolSha256": ToolSHA256, "backendPolicy": "full ZIP; -Xmx512m; epubcheck.offline=true; javax.xml.accessExternalDTD/Schema/Stylesheet empty; --json - --quiet --locale en; strict adds --failonwarnings"})
	digest := sha256.Sum256(h)
	r := Report{Status: "incomplete", ArchiveSHA256: hash, ConfigSHA256: hex.EncodeToString(digest[:]), Checks: []Check{}, Diagnostics: []Diagnostic{}, Limitations: []publication.Limitation{}, Draft: o.Draft}
	for _, id := range []string{"archive", "parse.structure", "references", "epubcheck", "rendering", "accessibility"} {
		required := id == "archive" || id == "parse.structure" || id == "epubcheck" && !o.Draft
		r.Checks = append(r.Checks, Check{ID: id, Status: "not_run", Required: required, Source: "kepub", Version: "1", Rules: "kepub-validation-v1", InputSHA256: hash, ConfigSHA256: r.ConfigSHA256, BlockedBy: []string{}})
	}
	r.Checks[4].Coverage = "not implemented: actual reader rendering"
	r.Checks[5].Coverage = "not implemented: human accessibility review"
	return r
}

func appendDiagnostic(r *Report, d publication.Diagnostic) {
	r.Diagnostics = append(r.Diagnostics, Diagnostic{Source: d.Source, Code: d.Code, Severity: d.Severity, BookPath: string(d.BookPath), Line: d.Line, Column: d.Column, Message: d.Message})
}

func failedInput(hash string, o Options, err error) (Report, error) {
	r := newReport(hash, o)
	if priority(err) == 1 {
		r.Status = "fail"
	}
	r.Checks[0].Status = "failed"
	appendDiagnostic(&r, publication.DiagnosticFor(err, "", ""))
	for i := 1; i < 4; i++ {
		r.Checks[i].Status = "blocked"
		r.Checks[i].BlockedBy = []string{"archive"}
	}
	return r, err
}

// CheckZIP validates the actual final artifact against expected archive bytes.
// It independently imports the ZIP rather than trusting an unrelated snapshot.
func CheckZIP(ctx context.Context, filename, expectedSHA256 string, o Options) (Report, error) {
	if o.Timeout < 0 {
		return Report{}, fault.New(2, "INVALID_ARGUMENT", "timeout must be positive")
	}
	if ctx.Err() != nil {
		return failedInput(expectedSHA256, o, fault.New(5, "CHECKER_TIMEOUT", "request timed out or was cancelled"))
	}
	i, e := os.Lstat(filename)
	if e != nil {
		return failedInput("", o, e)
	}
	if !i.Mode().IsRegular() {
		return failedInput("", o, fault.New(1, "UNSAFE_ENTRY", "final ZIP must be a regular file"))
	}
	if i.Size() > archive.DefaultLimits.TotalBytes {
		return failedInput("", o, fault.New(1, "ARCHIVE_LIMIT", "compressed input exceeds limit"))
	}
	hash, err := archive.FileSHA256(filename)
	if err != nil {
		return failedInput("", o, err)
	}
	if hash != expectedSHA256 {
		return failedInput(hash, o, fault.New(4, "INPUT_DRIFT", "expected archive hash differs"))
	}
	a, err := archive.Open(filename, archive.DefaultLimits)
	if err != nil {
		return failedInput(hash, o, err)
	}
	defer a.Close()
	tree, err := a.Inventory()
	if err != nil {
		return failedInput(hash, o, err)
	}
	r := newReport(hash, o)
	r.InputTreeSHA256 = tree.SHA256
	r.Checks[0].Status = "passed"
	r.Checks[0].Coverage = "safe bounded ZIP; exact inventory including directories"
	start := time.Now()
	p, parseErr := publication.Load(a, o.Rootfile)
	r.Checks[1].ElapsedMilliseconds = time.Since(start).Milliseconds()
	r.Checks[1].Coverage = "selected container/OPF structure and supported UTF-8 XML; not EPUB conformance"
	if parseErr != nil {
		r.Checks[1].Status = "failed"
		appendDiagnostic(&r, publication.DiagnosticFor(parseErr, "", ""))
		r.Checks[2].Status = "blocked"
		r.Checks[2].BlockedBy = []string{"parse.structure"}
	} else {
		r.Checks[1].Status = "passed"
		r.Limitations = p.Limitations
		start = time.Now()
		g := references.Build(a, p)
		r.Checks[2].ElapsedMilliseconds = time.Since(start).Milliseconds()
		r.Checks[2].Coverage = g.Coverage
		r.Checks[2].Status = "passed"
		if g.Status != "complete" {
			r.Checks[2].Status = "blocked"
		}
		for _, d := range g.Diagnostics {
			appendDiagnostic(&r, d)
		}
		for _, d := range g.Diagnostics {
			if d.Severity == "error" || d.Severity == "fatal" {
				r.Checks[2].Status = "failed"
			}
		}
	}
	c, ds, checkErr := epubcheck(ctx, filename, hash, tree, o)
	c.ConfigSHA256 = r.ConfigSHA256
	r.Checks[3] = c
	r.Diagnostics = append(r.Diagnostics, ds...)
	err = parseErr
	if priority(checkErr) > priority(err) {
		err = checkErr
	}
	for _, d := range r.Diagnostics {
		if d.Severity == "error" || d.Severity == "fatal" || o.Strict && d.Severity == "warning" {
			if err == nil {
				err = fault.New(1, "VALIDATION_FAILED", "reported diagnostics failed policy")
			}
		}
	}
	again, e := archive.FileSHA256(filename)
	if e != nil {
		err = e
	} else if again != hash {
		err = fault.New(4, "INPUT_DRIFT", "archive changed during checks")
	}
	if err == nil && !o.Draft {
		r.Status = "pass"
	} else if err != nil && priority(err) == 1 {
		r.Status = "fail"
	}
	if ctx.Err() != nil {
		r.Status = "incomplete"
		err = fault.New(5, "CHECKER_TIMEOUT", "request timed out or was cancelled")
	}
	return r, err
}

// Validate freezes a ZIP byte-for-byte or snapshots the explicit directory root.
// A directory is checked as a generated final ZIP; its tree hash stays identical.
func Validate(ctx context.Context, input string, o Options) (Report, error) {
	if ctx.Err() != nil {
		return failedInput("", o, fault.New(5, "CHECKER_TIMEOUT", "request timed out or was cancelled"))
	}
	i, err := os.Lstat(input)
	if err != nil {
		return failedInput("", o, err)
	}
	tmp, err := os.MkdirTemp("", "kepub-validate-")
	if err != nil {
		return Report{}, err
	}
	defer os.RemoveAll(tmp)
	filename := filepath.Join(tmp, "input.epub")
	w, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return Report{}, err
	}
	if i.IsDir() {
		a, t, e := archive.SnapshotDirectory(input, archive.DefaultLimits)
		if e != nil {
			w.Close()
			return failedInput("", o, e)
		}
		err = a.WriteZIP(w, t)
		a.Close()
	} else if i.Mode().IsRegular() {
		f, e := os.OpenFile(input, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if e != nil {
			w.Close()
			return failedInput("", o, e)
		}
		opened, e := f.Stat()
		if e != nil || !os.SameFile(i, opened) {
			f.Close()
			w.Close()
			return failedInput("", o, fault.New(4, "INPUT_DRIFT", "input replaced before freeze"))
		}
		n, e := io.Copy(w, io.LimitReader(f, archive.DefaultLimits.TotalBytes+1))
		after, se := f.Stat()
		f.Close()
		err = e
		if n > archive.DefaultLimits.TotalBytes {
			err = fault.New(1, "ARCHIVE_LIMIT", "compressed input exceeds limit")
		}
		if se != nil || n != opened.Size() || after.Size() != opened.Size() || !after.ModTime().Equal(opened.ModTime()) {
			err = fault.New(4, "INPUT_DRIFT", "input changed during freeze")
		}
	} else {
		err = fault.New(1, "UNSAFE_ENTRY", "input must be a real directory or regular ZIP")
	}
	ce := w.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		return failedInput("", o, err)
	}
	hash, err := archive.FileSHA256(filename)
	if err != nil {
		return Report{}, err
	}
	r, err := CheckZIP(ctx, filename, hash, o)
	// Private snapshot paths are operational details, not portable diagnostics.
	for n := range r.Checks {
		r.Checks[n].Output = strings.ReplaceAll(r.Checks[n].Output, tmp, "<snapshot>")
	}
	return r, err
}
