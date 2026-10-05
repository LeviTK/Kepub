package validation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/fault"
)

func reportFixture() (string, archive.Tree) {
	h := sha256.Sum256([]byte("payload"))
	hash := hex.EncodeToString(h[:])
	tree := archive.Tree{Entries: []archive.Entry{{Path: "resource", Type: "file", Size: 7, SHA256: hash}}}
	raw := fmt.Sprintf(`{"checker":{"checkerVersion":"5.3.0","nFatal":0,"nError":0,"nWarning":0,"nUsage":0},"publication":{"ePubVersion":"3.3","title":"Fixture","identifier":"urn:test:123","language":"en","nSpines":1},"items":[{"fileName":"resource","uncompressedSize":7,"checkSum":"%s"}],"messages":[]}`, upstreamChecksum(hash))
	return raw, tree
}

func TestReportRejectsFalseCompleteness(t *testing.T) {
	raw, tree := reportFixture()
	var u upstream
	if e := json.Unmarshal([]byte(raw), &u); e != nil {
		t.Fatal(e)
	}
	if e := validateReport(u, tree, true); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{
		`{}`, `{"checker":{"checkerVersion":"5.3.0","nError":0}}`,
		strings.Replace(raw, `"nFatal":0,`, "", 1),
		strings.Replace(raw, `"nError":0`, `"nError":-1`, 1),
		strings.Replace(raw, `"nError":0`, `"nError":1`, 1),
		strings.Replace(raw, `"5.3.0"`, `"5.4.0"`, 1),
		strings.Replace(raw, `"nSpines":1`, `"nSpines":0`, 1),
		strings.Replace(raw, `"items":[`, `"items":null,"ignored":[`, 1),
		strings.Replace(raw, `"resource"`, `"other"`, 1),
		strings.Replace(raw, `"uncompressedSize":7`, `"uncompressedSize":8`, 1),
		strings.Replace(raw, `"checkSum":"`, `"checkSum":"forged`, 1),
	} {
		var u upstream
		e := json.Unmarshal([]byte(bad), &u)
		if e == nil {
			e = validateReport(u, tree, true)
		}
		if e == nil {
			t.Fatal("false complete report accepted", bad)
		}
	}
	// Independent asymmetric unpadded checksum oracle includes zero/leading nibble.
	if got := upstreamChecksum(strings.Repeat("00010f102030405060708090a0b0c0d0", 2)); got != "01f102030405060708090a0b0c0d001f102030405060708090a0b0c0d0" {
		t.Fatal(got)
	}
	for _, bad := range []string{raw + `{}`, strings.Replace(raw, `"nError":0`, `"nError":1,"nError":0`, 1), strings.Repeat("[", 130) + "0" + strings.Repeat("]", 130)} {
		if _, e := decodeReport([]byte(bad)); e == nil {
			t.Fatal("ambiguous/unbounded evidence accepted")
		}
	}
}

func TestReportInventoryAndMetadataAliases(t *testing.T) {
	raw, _ := reportFixture()
	var report map[string]any
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		t.Fatal(err)
	}
	var tree archive.Tree
	var inventory []any
	for i, name := range []string{"literal%20.xhtml", "literal .xhtml", "章 节.xhtml", "plus+.xhtml", "hash#.xhtml", "query?.xhtml", "cafe\u0301.xhtml", "cafe\u0301!'().xhtml", "cafe\u0301%20 .xhtml", "empty"} {
		payload := []byte(fmt.Sprintf("independent payload %d", i))
		if name == "empty" {
			payload = nil
		}
		h := sha256.Sum256(payload)
		entry := archive.Entry{Path: name, Type: "file", Size: int64(len(payload)), SHA256: hex.EncodeToString(h[:])}
		tree.Entries = append(tree.Entries, entry)
		inventory = append(inventory, map[string]any{"fileName": name, "uncompressedSize": entry.Size, "checkSum": upstreamChecksum(entry.SHA256)})
	}
	alias := func(name string) any {
		return map[string]any{"fileName": name, "uncompressedSize": 0, "compressedSize": 0, "compressionMethod": nil, "checkSum": nil}
	}
	// Independent expected spellings from official URLUtils/Galimatias:
	// literal percent is escaped, plus/!'() are preserved, and Unicode is NFC.
	aliases := []any{alias("literal%2520.xhtml"), alias("%E7%AB%A0%20%E8%8A%82.xhtml"), alias("hash%23.xhtml"), alias("query%3F.xhtml"), alias("café.xhtml"), alias("caf%C3%A9.xhtml"), alias("café!'().xhtml"), alias("caf%C3%A9!'().xhtml"), alias("café%20 .xhtml"), alias("caf%C3%A9%2520%20.xhtml")}
	// Only raw, complete size/checksum rows prove inventory. URI metadata
	// aliases are independent, and must not choose between literal %20 and space.
	for _, tc := range []struct {
		name string
		edit func([]any) []any
		pass bool
	}{
		{"raw inventory", func(rows []any) []any { return rows }, true},
		{"bound metadata aliases", func(rows []any) []any { return append(rows, aliases...) }, true},
		{"aliases before inventory", func(rows []any) []any { return append(append([]any{}, aliases...), rows...) }, true},
		{"alias cannot replace inventory", func(rows []any) []any { return append(rows[1:], aliases...) }, false},
		{"duplicate raw", func(rows []any) []any { return append(rows, rows[0]) }, false},
		{"duplicate alias", func(rows []any) []any { return append(rows, aliases[0], aliases[0]) }, false},
		{"raw alias collision", func(rows []any) []any { return append(rows, alias("literal%20.xhtml")) }, false},
		{"unknown raw", func(rows []any) []any { rows[0].(map[string]any)["fileName"] = "unknown"; return rows }, false},
		{"unknown alias", func(rows []any) []any { return append(rows, alias("unknown%20.xhtml")) }, false},
		{"bad escape", func(rows []any) []any { return append(rows, alias("literal%xy.xhtml")) }, false},
		{"double decode", func(rows []any) []any { return append(rows, alias("literal%252520.xhtml")) }, false},
		{"traversal alias", func(rows []any) []any { return append(rows, alias("../literal%2520.xhtml")) }, false},
		{"noncanonical plus escape", func(rows []any) []any { return append(rows, alias("plus%2B.xhtml")) }, false},
		{"noncanonical punctuation escape", func(rows []any) []any { return append(rows, alias("caf%C3%A9%21%27%28%29.xhtml")) }, false},
		{"non-NFC URI alias", func(rows []any) []any { return append(rows, alias("cafe%CC%81.xhtml")) }, false},
		{"lowercase escape alias", func(rows []any) []any { return append(rows, alias("caf%c3%a9.xhtml")) }, false},
		{"NFC name cannot replace raw inventory", func(rows []any) []any { rows[6].(map[string]any)["fileName"] = "café.xhtml"; return rows }, false},
		{"ambiguous alias cannot replace raw", func(rows []any) []any { return append(rows[1:], alias("literal%20.xhtml")) }, false},
		{"wrong size", func(rows []any) []any {
			rows[0].(map[string]any)["uncompressedSize"] = 0
			return append(rows, aliases...)
		}, false},
		{"swapped checksums", func(rows []any) []any {
			a, b := rows[0].(map[string]any), rows[1].(map[string]any)
			a["checkSum"], b["checkSum"] = b["checkSum"], a["checkSum"]
			return append(rows, aliases...)
		}, false},
		{"empty checksum is not metadata", func(rows []any) []any {
			a := alias("literal%2520.xhtml").(map[string]any)
			a["checkSum"] = ""
			return append(rows, a)
		}, false},
		{"nonzero metadata size", func(rows []any) []any {
			a := alias("literal%2520.xhtml").(map[string]any)
			a["uncompressedSize"] = 7
			return append(rows, a)
		}, false},
		{"missing metadata checksum", func(rows []any) []any {
			a := alias("literal%2520.xhtml").(map[string]any)
			delete(a, "checkSum")
			return append(rows, a)
		}, false},
		{"missing metadata compressed size", func(rows []any) []any {
			a := alias("literal%2520.xhtml").(map[string]any)
			delete(a, "compressedSize")
			return append(rows, a)
		}, false},
		{"null metadata compressed size", func(rows []any) []any {
			a := alias("literal%2520.xhtml").(map[string]any)
			a["compressedSize"] = nil
			return append(rows, a)
		}, false},
		{"nonzero metadata compressed size", func(rows []any) []any {
			a := alias("literal%2520.xhtml").(map[string]any)
			a["compressedSize"] = 1
			return append(rows, a)
		}, false},
		{"missing metadata method", func(rows []any) []any {
			a := alias("literal%2520.xhtml").(map[string]any)
			delete(a, "compressionMethod")
			return append(rows, a)
		}, false},
		{"non-null metadata method", func(rows []any) []any {
			a := alias("literal%2520.xhtml").(map[string]any)
			a["compressionMethod"] = "Stored"
			return append(rows, a)
		}, false},
		{"URL query is not a path", func(rows []any) []any { return append(rows, alias("query?.xhtml")) }, false},
		{"URL fragment is not a path", func(rows []any) []any { return append(rows, alias("hash#.xhtml")) }, false},
		{"non-string inventory checksum", func(rows []any) []any { rows[0].(map[string]any)["checkSum"] = true; return rows }, false},
		{"missing empty-file size", func(rows []any) []any { delete(rows[len(rows)-1].(map[string]any), "uncompressedSize"); return rows }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// JSON roundtrip keeps each mutation independent of the other cases.
			b, err := json.Marshal(inventory)
			if err != nil {
				t.Fatal(err)
			}
			var rows []any
			if err := json.Unmarshal(b, &rows); err != nil {
				t.Fatal(err)
			}
			report["items"] = tc.edit(rows)
			b, err = json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			u, err := decodeReport(b)
			if err == nil {
				err = validateReport(u, tree, true)
			}
			if (err == nil) != tc.pass {
				t.Fatalf("pass=%v: %v", tc.pass, err)
			}
			if tc.name == "ambiguous alias cannot replace raw" && !strings.Contains(err.Error(), "metadata alias mismatch") {
				t.Fatalf("ambiguous source was not rejected before counting inventory: %v", err)
			}
		})
	}
}

func TestMissingOrUnpinnedBackend(t *testing.T) {
	for _, o := range []Options{{Java: "kepub-nonexistent-java"}, {Java: "sh", JAR: filepath.Join(t.TempDir(), "epubcheck.jar")}} {
		c, _, e := epubcheck(context.Background(), "unused", "", archive.Tree{}, o)
		var f *fault.Error
		if !errors.As(e, &f) || f.Exit != 3 || c.Status != "unavailable" {
			t.Fatal(c, e)
		}
	}
}

func TestBackendFailuresAndHashDrift(t *testing.T) {
	jar := os.Getenv("KEPUB_EPUBCHECK_JAR")
	if jar == "" {
		t.Skip("fault injection uses verified official JAR set; set KEPUB_EPUBCHECK_JAR")
	}
	raw, tree := reportFixture()
	for _, tc := range []struct {
		name, body, code string
		timeout          time.Duration
	}{
		{"missing fields", "printf '{}'", "CHECKER_REPORT_INVALID", time.Second},
		{"truncated", "printf '{\"checker\":'", "CHECKER_REPORT_INVALID", time.Second},
		{"trailing", `cat "$REPORT"; printf '{}'`, "CHECKER_REPORT_INVALID", time.Second},
		{"nonzero zero errors", `cat "$REPORT"; exit 1`, "CHECKER_EXECUTION_FAILED", time.Second},
		{"bad inventory checksum", `sed 's/checkSum/checkSumWrong/' "$REPORT"`, "CHECKER_REPORT_INVALID", time.Second},
		{"timeout", "sleep 30", "CHECKER_TIMEOUT", 150 * time.Millisecond},
		{"drift", `printf replacement > "$BOOK"; cat "$REPORT"`, "INPUT_DRIFT", time.Second},
		{"overflow", "head -c 16777217 /dev/zero", "CHECKER_REPORT_INVALID", 5 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			book := filepath.Join(dir, "book.epub")
			report := filepath.Join(dir, "report.json")
			java := filepath.Join(dir, "fake-java")
			if e := os.WriteFile(book, []byte("payload"), 0600); e != nil {
				t.Fatal(e)
			}
			hash, _ := archive.FileSHA256(book)
			if e := os.WriteFile(report, []byte(raw), 0600); e != nil {
				t.Fatal(e)
			}
			t.Setenv("REPORT", report)
			t.Setenv("BOOK", book)
			body := "#!/bin/sh\ncase \"$*\" in *--version*) echo 'EPUBCheck v5.3.0'; exit 0;; esac\n" + tc.body + "\n"
			if e := os.WriteFile(java, []byte(body), 0700); e != nil {
				t.Fatal(e)
			}
			c, _, e := epubcheck(context.Background(), book, hash, tree, Options{Java: java, JAR: jar, Timeout: tc.timeout})
			var f *fault.Error
			if !errors.As(e, &f) || f.Code != tc.code || c.Status == "passed" {
				t.Fatalf("%+v %v want %s", c, e, tc.code)
			}
			if c.InputSHA256 != hash {
				t.Fatal("wrong report binding")
			}
			if tc.name == "truncated" && c.InvalidReport == "" {
				t.Fatal("lost invalid raw report")
			}
		})
	}
}

func TestProcessTimeoutKillsGroup(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux /proc PGID state evidence; not a Darwin execution claim")
	}
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "child.pid")
	groupfile := filepath.Join(dir, "group.pid")
	late := filepath.Join(dir, "late-write")
	script := filepath.Join(dir, "backend")
	// A real delayed writer, not just sleep: the positive control below proves
	// this exact fixture writes if allowed to finish. Its descendants retain PGID.
	body := "#!/bin/sh\necho $$ > " + strconv.Quote(groupfile) + "\nsh -c 'sleep 0.6; printf unsafe > \"$1\"' writer " + strconv.Quote(late) + " &\necho $! > " + strconv.Quote(pidfile) + "\nwait\n"
	if e := os.WriteFile(script, []byte(body), 0700); e != nil {
		t.Fatal(e)
	}
	_, _, exit, e := runProcess(context.Background(), script, nil)
	written, readErr := os.ReadFile(late)
	if e != nil || exit != 0 || readErr != nil || string(written) != "unsafe" {
		t.Fatalf("delayed writer positive control: exit=%d err=%v data=%q read=%v", exit, e, written, readErr)
	}
	if e := os.Remove(late); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, exit, e = runProcess(ctx, script, nil)
	if e == nil || exit == 0 || time.Since(start) > 3*time.Second {
		t.Fatal("timeout did not reclaim", exit, e)
	}
	b, e := os.ReadFile(pidfile)
	if e != nil {
		t.Fatal(e)
	}
	pid, e := strconv.Atoi(strings.TrimSpace(string(b)))
	if e != nil {
		t.Fatal(e)
	}
	state, statErr := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	t.Logf("immediate child stat=%s error=%v", state, statErr)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) && !errors.Is(statErr, syscall.ESRCH) {
		t.Fatal(statErr)
	}
	b, e = os.ReadFile(groupfile)
	if e != nil {
		t.Fatal(e)
	}
	pgid, e := strconv.Atoi(strings.TrimSpace(string(b)))
	if e != nil || pgid <= 0 {
		t.Fatalf("invalid group: %q %v", b, e)
	}
	if actual, e := syscall.Getpgid(pid); e == nil && actual != pgid {
		t.Fatalf("child %d escaped group: %d want %d", pid, actual, pgid)
	} else if e != nil && !errors.Is(e, syscall.ESRCH) {
		t.Fatal(e)
	}
	// Linux Z/X members are not writers. Inspect the entire managed PGID, not
	// merely kill(child, 0), which can also succeed for a dead orphan zombie.
	states := processGroupStates(t, pgid)
	t.Logf("terminal child=%d pgid=%d states=%v", pid, pgid, states)
	alive := false
	for _, state := range states {
		alive = alive || state != "Z" && state != "X"
	}
	time.Sleep(800 * time.Millisecond) // Beyond the actual intended delayed write.
	if _, e := os.Stat(late); !errors.Is(e, os.ErrNotExist) {
		t.Fatalf("descendant wrote after terminal: %v", e)
	}
	after := processGroupStates(t, pgid)
	t.Logf("after 800ms: states=%v, no late-write", after)
	for _, state := range after {
		if state != "Z" && state != "X" {
			t.Fatalf("live group member after 800ms: %v", after)
		}
	}
	if alive {
		t.Fatalf("live process state at terminal: %v; after=%v", states, after)
	}
}

func TestProcessTimeoutOriginalSleepState(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux /proc state regression")
	}
	dir := t.TempDir()
	pidfile := filepath.Join(dir, "child.pid")
	script := filepath.Join(dir, "backend")
	if e := os.WriteFile(script, []byte("#!/bin/sh\nsleep 30 &\necho $! > "+strconv.Quote(pidfile)+"\nwait\n"), 0700); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, _, exit, e := runProcess(ctx, script, nil)
	if !errors.Is(e, context.DeadlineExceeded) || exit == 0 {
		t.Fatal(exit, e)
	}
	b, e := os.ReadFile(pidfile)
	if e != nil {
		t.Fatal(e)
	}
	pid, e := strconv.Atoi(strings.TrimSpace(string(b)))
	if e != nil {
		t.Fatal(e)
	}
	state, e := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	t.Logf("original sleep child=%d stat=%s error=%v", pid, state, e)
	// Linux may successfully open stat, then return ESRCH on read when the
	// process disappears. Observed under race repetition; not a live writer.
	if errors.Is(e, os.ErrNotExist) || errors.Is(e, syscall.ESRCH) {
		return
	}
	if e != nil {
		t.Fatal(e)
	}
	fields := strings.Fields(string(state[strings.LastIndexByte(string(state), ')')+1:]))
	if len(fields) < 3 || fields[0] != "Z" && fields[0] != "X" {
		t.Fatalf("original sleep remains live at terminal: %s", state)
	}
}

func TestProcessRootExitReclaimsWriters(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux PGID evidence")
	}
	for _, tc := range []struct{ name, pipes, exit string }{
		{"normal closed pipes", ">/dev/null 2>&1", "0"},
		{"nonzero closed pipes", ">/dev/null 2>&1", "7"},
		{"WaitDelay inherited pipes", "", "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			late := filepath.Join(dir, "late")
			group := filepath.Join(dir, "group")
			script := filepath.Join(dir, "backend")
			body := "#!/bin/sh\necho $$ > " + strconv.Quote(group) + "\nsh -c 'sleep 1.6; printf unsafe > \"$1\"' writer " + strconv.Quote(late) + " " + tc.pipes + " &\nif [ \"$1\" = control ]; then wait; else exit " + tc.exit + "; fi\n"
			if e := os.WriteFile(script, []byte(body), 0700); e != nil {
				t.Fatal(e)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, _, exit, e := runProcess(ctx, script, []string{"control"})
			b, readErr := os.ReadFile(late)
			if e != nil || exit != 0 || readErr != nil || string(b) != "unsafe" {
				t.Fatalf("positive control: %d %v %q %v", exit, e, b, readErr)
			}
			if e := os.Remove(late); e != nil {
				t.Fatal(e)
			}
			_, _, exit, e = runProcess(ctx, script, nil)
			b, readErr = os.ReadFile(group)
			if readErr != nil {
				t.Fatal(readErr)
			}
			pgid, parseErr := strconv.Atoi(strings.TrimSpace(string(b)))
			if parseErr != nil || pgid <= 0 {
				t.Fatal("invalid pgid", string(b), parseErr)
			}
			t.Cleanup(func() { _ = syscall.Kill(-pgid, syscall.SIGKILL) })
			states := processGroupStates(t, pgid)
			time.Sleep(1800 * time.Millisecond)
			_, lateErr := os.Stat(late)
			t.Logf("exit=%d error=%v terminal states=%v late-write=%v", exit, e, states, lateErr)
			if e == nil {
				t.Error("residual writers accepted as success")
			}
			for _, state := range states {
				if state != "Z" && state != "X" {
					t.Errorf("live member at terminal: %v", states)
				}
			}
			if !errors.Is(lateErr, os.ErrNotExist) {
				t.Error("descendant wrote after terminal", lateErr)
			}
		})
	}
}

func processGroupStates(t *testing.T, pgid int) map[int]string {
	t.Helper()
	entries, e := os.ReadDir("/proc")
	if e != nil {
		t.Fatal(e)
	}
	states := map[int]string{}
	for _, entry := range entries {
		pid, e := strconv.Atoi(entry.Name())
		if e != nil {
			continue
		}
		b, e := os.ReadFile("/proc/" + entry.Name() + "/stat")
		if errors.Is(e, os.ErrNotExist) || errors.Is(e, syscall.ESRCH) {
			continue
		}
		if e != nil {
			t.Fatal(e)
		}
		end := strings.LastIndexByte(string(b), ')')
		if end < 0 {
			t.Fatalf("invalid process stat: %q", b)
		}
		fields := strings.Fields(string(b[end+1:]))
		if len(fields) < 3 {
			t.Fatalf("short process stat: %q", b)
		}
		group, e := strconv.Atoi(fields[2])
		if e != nil {
			t.Fatal(e)
		}
		if group == pgid {
			states[pid] = fields[0]
		}
	}
	return states
}
