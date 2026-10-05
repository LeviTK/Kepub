// Package validation separates EPUB conformance from partial Kepub indexes.
package validation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/fault"
)

const Version = "5.3.0"
const ToolSHA256 = "158b7c2778c3b64d5dbd87151b7d1f870b03beda3bee973d52b3fe58bea16266"
const ReportLimit = 16 << 20

type Options struct {
	Rootfile string        `json:"rootfile"`
	Strict   bool          `json:"strict"`
	Draft    bool          `json:"draft"`
	Timeout  time.Duration `json:"timeoutNanoseconds"`
	Java     string        `json:"-"`
	JAR      string        `json:"-"`
}

type Diagnostic struct {
	Source       string `json:"source"`
	Code         string `json:"code"`
	UpstreamCode string `json:"upstreamCode,omitempty"`
	Severity     string `json:"severity"`
	BookPath     string `json:"bookPath"`
	Line         *int   `json:"line"`
	Column       *int   `json:"column"`
	Message      string `json:"message"`
}
type Check struct {
	ID                  string          `json:"id"`
	Status              string          `json:"status"`
	Reason              string          `json:"reason"`
	Required            bool            `json:"required"`
	Source              string          `json:"source"`
	Version             string          `json:"version"`
	Rules               string          `json:"rules"`
	InputSHA256         string          `json:"inputSha256"`
	ConfigSHA256        string          `json:"configSha256"`
	Coverage            any             `json:"coverage"`
	BlockedBy           []string        `json:"blockedBy"`
	ElapsedMilliseconds int64           `json:"elapsedMilliseconds"`
	BackendExitCode     *int            `json:"backendExitCode,omitempty"`
	ToolSHA256          string          `json:"toolSha256,omitempty"`
	RawReport           json.RawMessage `json:"rawReport,omitempty"`
	InvalidReport       string          `json:"invalidReport,omitempty"`
	Output              string          `json:"output,omitempty"`
}

type upstream struct {
	Checker *struct {
		Version string `json:"checkerVersion"`
		Fatal   *int   `json:"nFatal"`
		Error   *int   `json:"nError"`
		Warning *int   `json:"nWarning"`
		Usage   *int   `json:"nUsage"`
	} `json:"checker"`
	Publication *struct {
		Version    string `json:"ePubVersion"`
		Title      string `json:"title"`
		Identifier string `json:"identifier"`
		Language   string `json:"language"`
		Spines     int    `json:"nSpines"`
	} `json:"publication"`
	Items []struct {
		Name     string `json:"fileName"`
		Size     int64  `json:"uncompressedSize"`
		Checksum string `json:"checkSum"`
	} `json:"items"`
	Messages []struct {
		ID         string `json:"ID"`
		Severity   string `json:"severity"`
		Message    string `json:"message"`
		Additional int    `json:"additionalLocations"`
		Locations  []struct {
			Path   string `json:"path"`
			Line   *int   `json:"line"`
			Column *int   `json:"column"`
		} `json:"locations"`
	} `json:"messages"`
}

// A capped writer stops the external process on output overflow. Report JSON is
// requested on stdout, so a malicious/failed backend cannot fill a report file.
type bounded struct {
	bytes.Buffer
	cancel   context.CancelFunc
	overflow bool
	limit    int
}

func (b *bounded) Write(p []byte) (int, error) {
	limit := b.limit
	if limit == 0 {
		limit = ReportLimit
	}
	if b.Len()+len(p) > limit {
		b.overflow = true
		b.cancel()
		return 0, fmt.Errorf("backend output limit")
	}
	return b.Buffer.Write(p)
}

func runProcess(ctx context.Context, java string, args []string) ([]byte, []byte, int, error) {
	return runProcessLimit(ctx, java, args, ReportLimit)
}

func runProcessLimit(ctx context.Context, java string, args []string, limit int) ([]byte, []byte, int, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	out := &bounded{cancel: cancel, limit: limit}
	stderr := &bounded{cancel: cancel, limit: limit}
	cmd := exec.CommandContext(ctx, java, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	cmd.Stdout = out
	cmd.Stderr = stderr
	// Do not inherit Java option injection or classpath from the environment.
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		if k != "JAVA_TOOL_OPTIONS" && k != "JDK_JAVA_OPTIONS" && k != "_JAVA_OPTIONS" && k != "CLASSPATH" {
			cmd.Env = append(cmd.Env, e)
		}
	}
	err := cmd.Run()
	// Wait observes only the root and its pipes. Closed-pipe descendants and
	// WaitDelay leftovers must be killed even after a normal/nonzero root exit.
	if cmd.Process != nil {
		pid := cmd.Process.Pid
		present := syscall.Kill(-pid, 0)
		if !errors.Is(present, syscall.ESRCH) {
			if cleanupErr := stopBackendGroup(pid); cleanupErr != nil {
				err = cleanupErr
			} else if err == nil {
				err = fmt.Errorf("backend left residual process group after root exit")
			}
		}
	}
	exit := -1
	if cmd.ProcessState != nil {
		exit = cmd.ProcessState.ExitCode()
	}
	if out.overflow || stderr.overflow {
		err = fmt.Errorf("backend output limit")
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return out.Bytes(), stderr.Bytes(), exit, err
}

func stopBackendGroup(pid int) error {
	if e := syscall.Kill(-pid, syscall.SIGKILL); e != nil && !errors.Is(e, syscall.ESRCH) {
		return fmt.Errorf("backend group kill: %w", e)
	}
	// Inspect Linux Z/X only after SIGKILL stabilizes group membership. Darwin
	// conservatively requires ESRCH. This is not reclamation of escaped groups.
	deadline := time.Now().Add(2 * time.Second)
	for {
		alive, e := backendGroupAlive(pid)
		if e != nil {
			return fmt.Errorf("backend group inspection: %w", e)
		}
		if !alive {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("backend group cleanup timed out")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// fingerprint pins the official release's complete executable JAR set, not
// merely --version text. Download/install is deliberately outside this API.
func fingerprint(jar string) (string, error) {
	abs, err := filepath.Abs(jar)
	if err != nil {
		return "", err
	}
	if filepath.Base(abs) != "epubcheck.jar" {
		return "", fmt.Errorf("expected official epubcheck.jar")
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(abs), "lib", "*.jar"))
	if err != nil {
		return "", err
	}
	files = append(files, abs)
	lines := []string{}
	for _, f := range files {
		i, err := os.Lstat(f)
		if err != nil {
			return "", err
		}
		if !i.Mode().IsRegular() || i.Size() > 64<<20 {
			return "", fmt.Errorf("unsafe checker JAR")
		}
		h, err := archive.FileSHA256(f)
		if err != nil {
			return "", err
		}
		rel, _ := filepath.Rel(filepath.Dir(abs), f)
		lines = append(lines, filepath.ToSlash(rel)+":"+h+"\n")
	}
	sort.Strings(lines)
	h := sha256.Sum256([]byte(strings.Join(lines, "")))
	return hex.EncodeToString(h[:]), nil
}

func epubcheck(ctx context.Context, filename, hash string, tree archive.Tree, o Options) (c Check, diagnostics []Diagnostic, err error) {
	start := time.Now()
	c = Check{ID: "epubcheck", Status: "unavailable", Required: true, Source: "epubcheck", Version: Version, Rules: "unknown", InputSHA256: hash, BlockedBy: []string{}, Coverage: "full packaged EPUB conformance; not rendering or human accessibility review"}
	defer func() {
		c.ElapsedMilliseconds = time.Since(start).Milliseconds()
		if err != nil {
			c.Reason = err.Error()
		}
	}()
	if o.Draft {
		c.Required = false
		c.Status = "not_run"
		c.Reason = "explicit draft: not formally verified"
		return c, nil, nil
	}
	java := o.Java
	if java == "" {
		java = "java"
	}
	java, err = exec.LookPath(java)
	if err != nil {
		return c, nil, fault.New(3, "DEPENDENCY_UNAVAILABLE", "Java is unavailable")
	}
	jar := o.JAR
	if jar == "" {
		jar = os.Getenv("KEPUB_EPUBCHECK_JAR")
	}
	if jar == "" {
		return c, nil, fault.New(3, "DEPENDENCY_UNAVAILABLE", "set KEPUB_EPUBCHECK_JAR to official EPUBCheck 5.3.0 epubcheck.jar")
	}
	jar, err = filepath.Abs(jar)
	if err != nil {
		return c, nil, err
	}
	c.ToolSHA256, err = fingerprint(jar)
	if err != nil || c.ToolSHA256 != ToolSHA256 {
		return c, nil, fault.New(3, "DEPENDENCY_UNAVAILABLE", "EPUBCheck executable JAR set does not match official 5.3.0 release")
	}
	if o.Timeout == 0 {
		o.Timeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	v, _, exit, e := runProcess(ctx, java, []string{"-jar", jar, "--version"})
	if e != nil || exit != 0 || strings.TrimSpace(string(v)) != "EPUBCheck v"+Version {
		c.Status = "failed"
		if ctx.Err() != nil {
			return c, nil, fault.New(5, "CHECKER_TIMEOUT", "EPUBCheck version probe timed out or was cancelled")
		}
		return c, nil, fault.New(5, "CHECKER_EXECUTION_FAILED", "EPUBCheck version probe failed")
	}
	c.Status = "failed"
	before, e := archive.FileSHA256(filename)
	if e != nil {
		return c, nil, e
	}
	if before != hash {
		c.Status = "failed"
		return c, nil, fault.New(4, "INPUT_DRIFT", "archive hash changed before check")
	}
	args := []string{"-Xmx512m", "-Depubcheck.offline=true", "-Djavax.xml.accessExternalDTD=", "-Djavax.xml.accessExternalSchema=", "-Djavax.xml.accessExternalStylesheet=", "-jar", jar, "--json", "-", "--quiet", "--locale", "en", filename}
	if o.Strict {
		args = append(args, "--failonwarnings")
	}
	raw, logs, exit, processErr := runProcess(ctx, java, args)
	c.BackendExitCode = &exit
	c.Output = string(logs)
	after, e := archive.FileSHA256(filename)
	if e != nil {
		return c, nil, e
	}
	toolAfter, e := fingerprint(jar)
	if e != nil || toolAfter != c.ToolSHA256 || after != hash {
		c.Status = "failed"
		return c, nil, fault.New(4, "INPUT_DRIFT", "archive or checker bytes changed during check")
	}
	u, parseErr := decodeReport(raw)
	if json.Valid(raw) {
		c.RawReport = append(json.RawMessage{}, raw...)
	} else {
		c.InvalidReport = string(raw)
	}
	if parseErr == nil {
		parseErr = validateReport(u, tree, exit == 0)
	}
	if parseErr == nil {
		c.Rules = u.Publication.Version
		for _, m := range u.Messages {
			if len(m.Locations) == 0 {
				diagnostics = append(diagnostics, Diagnostic{Source: "epubcheck", Code: "EPUBCHECK_MESSAGE", UpstreamCode: m.ID, Severity: strings.ToLower(m.Severity), Message: m.Message})
			}
			for _, l := range m.Locations {
				bp := l.Path
				if bp == filename || filepath.IsAbs(bp) {
					bp = ""
				}
				if l.Line != nil && *l.Line < 0 {
					l.Line = nil
				}
				if l.Column != nil && *l.Column < 0 {
					l.Column = nil
				}
				diagnostics = append(diagnostics, Diagnostic{Source: "epubcheck", Code: "EPUBCHECK_MESSAGE", UpstreamCode: m.ID, Severity: strings.ToLower(m.Severity), BookPath: bp, Line: l.Line, Column: l.Column, Message: m.Message})
			}
		}
	}
	c.Status = "failed"
	if ctx.Err() != nil {
		return c, diagnostics, fault.New(5, "CHECKER_TIMEOUT", "EPUBCheck timed out or was cancelled")
	}
	if parseErr != nil {
		return c, diagnostics, fault.New(5, "CHECKER_REPORT_INVALID", "EPUBCheck report is incomplete or invalid: %v", parseErr)
	}
	if processErr != nil || exit != 0 {
		if exit == 1 && (*u.Checker.Fatal > 0 || *u.Checker.Error > 0 || o.Strict && *u.Checker.Warning > 0) {
			return c, diagnostics, fault.New(1, "VALIDATION_FAILED", "EPUBCheck reported validation errors; nonzero exit is never pass")
		}
		return c, diagnostics, fault.New(5, "CHECKER_EXECUTION_FAILED", "EPUBCheck did not exit successfully")
	}
	if *u.Checker.Fatal > 0 || *u.Checker.Error > 0 || o.Strict && *u.Checker.Warning > 0 {
		return c, diagnostics, fault.New(1, "VALIDATION_FAILED", "EPUBCheck report failed policy")
	}
	c.Status = "passed"
	return c, diagnostics, nil
}

// Duplicate fields and excessive nesting are invalid evidence, even though
// encoding/json would otherwise silently use the last duplicate value.
func decodeReport(raw []byte) (upstream, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	var value func(int) error
	value = func(depth int) error {
		if depth > 128 {
			return fmt.Errorf("report nesting limit")
		}
		t, e := d.Token()
		if e != nil {
			return e
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, e := d.Token()
				if e != nil {
					return e
				}
				s, ok := key.(string)
				if !ok || seen[s] {
					return fmt.Errorf("duplicate/invalid report key")
				}
				seen[s] = true
				if e = value(depth + 1); e != nil {
					return e
				}
			}
		case '[':
			for d.More() {
				if e = value(depth + 1); e != nil {
					return e
				}
			}
		default:
			return fmt.Errorf("unexpected report delimiter")
		}
		_, e = d.Token()
		return e
	}
	var u upstream
	if e := value(0); e != nil {
		return u, e
	}
	if _, e := d.Token(); e != io.EOF {
		return u, fmt.Errorf("trailing report JSON")
	}
	err := json.Unmarshal(raw, &u)
	return u, err
}

func validateReport(u upstream, tree archive.Tree, successful bool) error {
	if u.Checker == nil || u.Publication == nil || u.Checker.Version != Version || u.Messages == nil || u.Items == nil {
		return fmt.Errorf("missing report fields/version")
	}
	counts := []*int{u.Checker.Fatal, u.Checker.Error, u.Checker.Warning, u.Checker.Usage}
	for _, n := range counts {
		if n == nil || *n < 0 {
			return fmt.Errorf("missing/negative count")
		}
	}
	actual := map[string]int{"FATAL": 0, "ERROR": 0, "WARNING": 0, "USAGE": 0}
	for _, m := range u.Messages {
		if _, ok := actual[m.Severity]; !ok || m.ID == "" || m.Message == "" || m.Additional < 0 || len(m.Locations) > 25 {
			return fmt.Errorf("invalid message")
		}
		actual[m.Severity]++
	}
	for i, s := range []string{"FATAL", "ERROR", "WARNING", "USAGE"} {
		if actual[s] != *counts[i] {
			return fmt.Errorf("message/count mismatch")
		}
	}
	if successful {
		p := u.Publication
		if (p.Version != "3.3" && p.Version != "2.0.1") || p.Title == "" || p.Identifier == "" || p.Language == "" || p.Spines < 1 {
			return fmt.Errorf("missing full-publication evidence")
		}
		want := map[string]archive.Entry{}
		for _, e := range tree.Entries {
			if e.Type == "file" {
				want[e.Path] = e
			}
		}
		for _, e := range u.Items {
			entry, ok := want[e.Name]
			if !ok || e.Size != entry.Size || e.Checksum != upstreamChecksum(entry.SHA256) {
				return fmt.Errorf("report inventory mismatch")
			}
			delete(want, e.Name)
		}
		if len(want) != 0 {
			return fmt.Errorf("report inventory incomplete")
		}
	}
	return nil
}

// EPUBCheck 5.3.0 formats SHA-256 bytes as unpadded hex (upstream issue),
// so comparing normal 64-character hex would reject real valid reports.
func upstreamChecksum(hash string) string {
	b, err := hex.DecodeString(hash)
	if err != nil || len(b) != 32 {
		return "invalid hash"
	}
	var out strings.Builder
	for _, n := range b {
		fmt.Fprintf(&out, "%x", n)
	}
	return out.String()
}

func priority(err error) int {
	var f *fault.Error
	if errors.As(err, &f) {
		switch f.Exit {
		case 2:
			return 6
		case 4:
			return 5
		case 3:
			return 4
		case 5, 6:
			return 3
		default:
			return 1
		}
	}
	if err != nil {
		return 3
	}
	return 0
}
