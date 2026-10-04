package validation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/app"
	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/validation"
	"github.com/LeviTK/Kepub/internal/workspace"
)

// Independent compliant fixtures: the shared read-query fixtures deliberately
// contain invalid EPUB navigation/URLs and are not conformance positive samples.
func legal(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	meta := `<meta property="dcterms:modified">2026-10-04T00:00:00Z</meta>`
	navItem := `<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>`
	spine := `<spine>`
	if version == "2.0" {
		meta = ""
		navItem = `<item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>`
		spine = `<spine toc="ncx">`
	}
	files := map[string]string{
		"mimetype":               "application/epub+zip",
		"META-INF/container.xml": `<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"EPUB/package.opf":       fmt.Sprintf(`<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" version="%s" unique-identifier="uid"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:uuid:12345678-1234-1234-1234-123456789012</dc:identifier><dc:title>Independent Fixture</dc:title><dc:language>en</dc:language>%s</metadata><manifest><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/><item id="css" href="style.css" media-type="text/css"/>%s</manifest>%s<itemref idref="chapter"/></spine></package>`, version, meta, navItem, spine),
		"EPUB/chapter.xhtml":     `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter</title><link rel="stylesheet" type="text/css" href="style.css"/></head><body><h1 id="start">Chapter</h1><p>Original &amp; precise.</p></body></html>` + "\r\n",
		"EPUB/style.css":         "p { color: #123456; }\r\n",
	}
	if version == "3.0" {
		files["EPUB/nav.xhtml"] = `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="chapter.xhtml#start">Chapter</a></li></ol></nav></body></html>`
	} else {
		files["EPUB/toc.ncx"] = `<?xml version="1.0"?><ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><head><meta name="dtb:uid" content="urn:uuid:12345678-1234-1234-1234-123456789012"/><meta name="dtb:depth" content="1"/><meta name="dtb:totalPageCount" content="0"/><meta name="dtb:maxPageNumber" content="0"/></head><docTitle><text>Independent Fixture</text></docTitle><navMap><navPoint id="n1" playOrder="1"><navLabel><text>Chapter</text></navLabel><content src="chapter.xhtml#start"/></navPoint></navMap></ncx>`
	}
	for p, b := range files {
		write(t, dir, p, b)
	}
	if err := os.Mkdir(filepath.Join(dir, "empty"), 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}
func write(t *testing.T, root, name, b string) {
	t.Helper()
	p := filepath.Join(root, name)
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(b), 0600); e != nil {
		t.Fatal(e)
	}
}
func zipFixture(t *testing.T, dir string) string {
	t.Helper()
	a, tree, e := archive.SnapshotDirectory(dir, archive.DefaultLimits)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	f, e := os.Create(filepath.Join(t.TempDir(), "fixture.epub"))
	if e != nil {
		t.Fatal(e)
	}
	e = a.WriteZIP(f, tree)
	ce := f.Close()
	if e != nil || ce != nil {
		t.Fatal(e, ce)
	}
	return f.Name()
}
func code(t *testing.T, e error, want string) {
	t.Helper()
	var f *fault.Error
	if !errors.As(e, &f) || f.Code != want {
		t.Fatalf("want %s: %v", want, e)
	}
}

func TestDraftMissingDependenciesAndTreeBinding(t *testing.T) {
	dir := legal(t, "3.0")
	write(t, dir, "unlisted.bin", string([]byte{0, 255, 13, 10, 42}))
	// Parent tool files never enter the publication snapshot.
	write(t, filepath.Dir(dir), "AGENTS.md", "not publication")
	a, tree, e := archive.SnapshotDirectory(dir, archive.DefaultLimits)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	w, e := workspace.HashTree(dir)
	if e != nil || w.SHA256 != tree.SHA256 {
		t.Fatal("tree contract mismatch", e, w.SHA256, tree.SHA256)
	}
	output := filepath.Join(t.TempDir(), "draft.epub")
	p, e := app.PackSnapshot(context.Background(), a, tree, output, validation.Options{Draft: true, Java: "missing-java"})
	if e != nil {
		t.Fatal(e)
	}
	if p.Verified || !p.Draft || p.Validation.Status != "incomplete" || p.Validation.Checks[3].Status != "not_run" {
		t.Fatal(p)
	}
	final, e := archive.Open(output, archive.DefaultLimits)
	if e != nil {
		t.Fatal(e)
	}
	defer final.Close()
	ft, e := final.Inventory()
	if e != nil || ft.SHA256 != tree.SHA256 {
		t.Fatal("resource/empty directory bytes changed", e)
	}
	_, e = app.PackSnapshot(context.Background(), a, tree, output, validation.Options{Draft: true})
	code(t, e, "OUTPUT_EXISTS")
	missing := filepath.Join(t.TempDir(), "formal.epub")
	p, e = app.PackSnapshot(context.Background(), a, tree, missing, validation.Options{Java: "missing-java"})
	code(t, e, "DEPENDENCY_UNAVAILABLE")
	if p.Validation.Checks[3].Status != "unavailable" || p.Validation.Status != "incomplete" {
		t.Fatal(p)
	}
	if _, e = os.Lstat(missing); !os.IsNotExist(e) {
		t.Fatal("published failed formal", e)
	}
	_, e = app.Pack(context.Background(), dir, filepath.Join(dir, "inside.epub"), validation.Options{Draft: true})
	code(t, e, "INVALID_OUTPUT")
	book := zipFixture(t, dir)
	hash, _ := archive.FileSHA256(book)
	r, e := validation.CheckZIP(context.Background(), book, strings.Repeat("0", 64), validation.Options{Draft: true})
	code(t, e, "INPUT_DRIFT")
	if r.Status == "pass" || r.ArchiveSHA256 != hash {
		t.Fatal(r)
	}
	// Parse failure and missing dependency are both retained; dependency wins.
	write(t, dir, "EPUB/package.opf", "<broken>")
	r, e = validation.Validate(context.Background(), dir, validation.Options{Java: "missing-java"})
	code(t, e, "DEPENDENCY_UNAVAILABLE")
	if r.Checks[1].Status != "failed" || r.Checks[2].Status != "blocked" || len(r.Diagnostics) == 0 {
		t.Fatal(r)
	}
}

func TestRealEPUBCheck(t *testing.T) {
	jar := os.Getenv("KEPUB_EPUBCHECK_JAR")
	if jar == "" {
		t.Skip("install official pinned EPUBCheck; set KEPUB_EPUBCHECK_JAR")
	}
	for _, version := range []string{"2.0", "3.0"} {
		t.Run(version, func(t *testing.T) {
			dir := legal(t, version)
			o := validation.Options{JAR: jar, Timeout: 30 * time.Second}
			book := zipFixture(t, dir)
			r, e := validation.Validate(context.Background(), book, o)
			if e != nil {
				t.Fatalf("legal EPUB%s: %v report=%+v", version, e, r)
			}
			if r.Status != "pass" || r.Checks[3].Status != "passed" || r.Checks[3].ToolSHA256 != validation.ToolSHA256 || r.Checks[2].Required {
				t.Fatal(r)
			}
			if r.Checks[2].Status != "blocked" {
				t.Fatal("CSS coverage falsely complete", r.Checks[2])
			}
			hash, _ := archive.FileSHA256(book)
			if hash != r.ArchiveSHA256 {
				t.Fatal("report wrong byte binding")
			}
			t.Logf("EPUB%s pass, rules=%s, archive=%s, tree=%s, reference checker=%s required=%v", version, r.Checks[3].Rules, r.ArchiveSHA256, r.InputTreeSHA256, r.Checks[2].Status, r.Checks[2].Required)
			out := filepath.Join(t.TempDir(), "formal.epub")
			packed, e := app.Pack(context.Background(), dir, out, o)
			if e != nil || !packed.Verified || packed.Validation.Status != "pass" {
				t.Fatalf("formal: %+v %v", packed, e)
			}
			// Real illegal content with both parser and upstream diagnostics retained.
			chapter, _ := os.ReadFile(filepath.Join(dir, "EPUB/chapter.xhtml"))
			write(t, dir, "EPUB/chapter.xhtml", strings.Replace(string(chapter), "</p>", "</wrong>", 1))
			r, e = validation.Validate(context.Background(), dir, o)
			if e == nil || r.Status == "pass" || r.Checks[3].Status != "failed" {
				t.Fatal("illegal passed", r, e)
			}
			upstream := false
			for _, d := range r.Diagnostics {
				if d.Source == "epubcheck" && d.UpstreamCode != "" {
					upstream = true
				}
			}
			if !upstream {
				t.Fatal("lost upstream codes", r, e)
			}
			out = filepath.Join(t.TempDir(), "bad.epub")
			_, e = app.Pack(context.Background(), dir, out, o)
			if e == nil {
				t.Fatal("illegal exported")
			}
			if _, e = os.Lstat(out); !os.IsNotExist(e) {
				t.Fatal("failure published", e)
			}
		})
	}
	// Empty directory produces real PKG-014: default accepts, strict fails.
	dir := legal(t, "3.0")
	write(t, dir, "bonus.txt", "explicit unmanifested original bytes")
	o := validation.Options{JAR: jar, Timeout: 30 * time.Second}
	r, e := validation.Validate(context.Background(), dir, o)
	if e != nil || r.Status != "pass" {
		t.Fatal(r, e)
	}
	warning := false
	for _, d := range r.Diagnostics {
		if d.Source == "epubcheck" && d.Severity == "warning" {
			warning = true
			t.Log("real warning", d.UpstreamCode)
		}
	}
	if !warning {
		t.Fatal("warning fixture did not produce warning")
	}
	o.Strict = true
	r, e = validation.Validate(context.Background(), dir, o)
	if e == nil || r.Status == "pass" {
		t.Fatal("strict warning passed", r, e)
	}
}

func TestRealExternalEntitiesNotRead(t *testing.T) {
	jar := os.Getenv("KEPUB_EPUBCHECK_JAR")
	if jar == "" {
		t.Skip("real checker needs KEPUB_EPUBCHECK_JAR")
	}
	var hits atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); fmt.Fprint(w, "REMOTE_ENTITY_SENTINEL") }))
	defer s.Close()
	secret := filepath.Join(t.TempDir(), "outside.txt")
	if e := os.WriteFile(secret, []byte("OUTSIDE_ENTITY_SENTINEL"), 0600); e != nil {
		t.Fatal(e)
	}
	for _, version := range []string{"2.0", "3.0"} {
		dir := legal(t, version)
		chapter := fmt.Sprintf(`<?xml version="1.0"?><!DOCTYPE html [<!ENTITY remote SYSTEM "%s/entity"><!ENTITY local SYSTEM "file://%s">]><html xmlns="http://www.w3.org/1999/xhtml"><head><title>Entity test</title></head><body><p>&remote;&local;</p></body></html>`, s.URL, secret)
		write(t, dir, "EPUB/chapter.xhtml", chapter)
		r, e := validation.Validate(context.Background(), dir, validation.Options{JAR: jar, Timeout: 30 * time.Second})
		if e == nil || r.Status == "pass" || r.Checks[3].Status == "not_run" {
			t.Fatal("unsafe entity book accepted/not exercised", e)
		}
		encoded, _ := json.Marshal(r)
		if bytes.Contains(encoded, []byte("OUTSIDE_ENTITY_SENTINEL")) || bytes.Contains(encoded, []byte("REMOTE_ENTITY_SENTINEL")) {
			t.Fatal("external entity expanded")
		}
	}
	if hits.Load() != 0 {
		t.Fatal("unexpected EPUBCheck network access", hits.Load())
	}
}

func TestBinaryValidatePack(t *testing.T) {
	jar := os.Getenv("KEPUB_EPUBCHECK_JAR")
	if jar == "" {
		t.Skip("real checker CLI smoke needs KEPUB_EPUBCHECK_JAR")
	}
	bin := filepath.Join(t.TempDir(), "kepub")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/kepub")
	if b, e := build.CombinedOutput(); e != nil {
		t.Fatal(string(b), e)
	}
	dir := legal(t, "3.0")
	book := zipFixture(t, dir)
	out := filepath.Join(t.TempDir(), "含 空格.epub")
	dir2 := legal(t, "2.0")
	book2 := zipFixture(t, dir2)
	badBooks := []string{}
	for _, version := range []string{"2.0", "3.0"} {
		bad := legal(t, version)
		write(t, bad, "EPUB/chapter.xhtml", "<broken>")
		badBooks = append(badBooks, zipFixture(t, bad))
	}
	clean := legal(t, "3.0")
	if e := os.Remove(filepath.Join(clean, "empty")); e != nil {
		t.Fatal(e)
	}
	fakeBin := t.TempDir()
	if e := os.WriteFile(filepath.Join(fakeBin, "java"), []byte("#!/bin/sh\ncase \"$*\" in *--version*) echo 'EPUBCheck v5.3.0'; exit 0;; esac\nif [ -n \"$KEPUB_FAKE_MARKER\" ]; then printf ready > \"$KEPUB_FAKE_MARKER\"; fi\n/bin/sleep 30\n"), 0700); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		args    []string
		exit    int
		missing bool
	}{
		{[]string{"validate", book, "--json"}, 0, false},
		{[]string{"validate", book2, "--json"}, 0, false},
		{[]string{"validate", badBooks[0], "--json"}, 1, false},
		{[]string{"validate", badBooks[1], "--json"}, 1, false},
		{[]string{"validate", clean, "--strict", "--json"}, 0, false},
		{[]string{"validate", dir, "--strict", "--json"}, 1, false},
		{[]string{"pack", dir, "--output", out, "--json"}, 0, false},
		{[]string{"pack", dir, "--output", out, "--json"}, 2, false},
		{[]string{"validate", book, "--json"}, 3, true},
		{[]string{"info", book, "--json"}, 0, true},
		{[]string{"pack", dir, "-o", filepath.Join(t.TempDir(), "draft.epub"), "--draft", "--json"}, 0, true},
		{[]string{"pack", dir, "-o", filepath.Join(t.TempDir(), "unavailable.epub"), "--json"}, 3, true},
		{[]string{"validate", book, "--timeout", "1", "--timeout", "2", "--json"}, 2, false},
		{[]string{"pack", dir, "--output", filepath.Join(t.TempDir(), "timeout.epub"), "--timeout", "1", "--json"}, 5, false},
	} {
		c := exec.Command(bin, tc.args...)
		c.Env = os.Environ()
		if tc.missing {
			c.Env = append(c.Env, "KEPUB_EPUBCHECK_JAR=", "PATH=/nonexistent")
		}
		if tc.exit == 5 {
			c.Env = append(c.Env, "PATH="+fakeBin)
		}
		var stdout, stderr bytes.Buffer
		c.Stdout = &stdout
		c.Stderr = &stderr
		e := c.Run()
		exit := 0
		if e != nil {
			var ee *exec.ExitError
			if !errors.As(e, &ee) {
				t.Fatal(e)
			}
			exit = ee.ExitCode()
		}
		if exit != tc.exit || stderr.Len() != 0 {
			t.Fatalf("%v exit %d: %s stderr %s", tc.args, exit, &stdout, &stderr)
		}
		var env struct {
			OK    bool            `json:"ok"`
			Data  json.RawMessage `json:"data"`
			Error any             `json:"error"`
		}
		d := json.NewDecoder(&stdout)
		if e := d.Decode(&env); e != nil {
			t.Fatal(e)
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			t.Fatal("multiple envelopes")
		}
		if env.OK != (exit == 0) {
			t.Fatal(env)
		}
		if tc.exit == 5 {
			if _, e := os.Lstat(tc.args[3]); !os.IsNotExist(e) {
				t.Fatal("timed-out binary published", e)
			}
		}
		if tc.args[0] == "validate" && exit == 0 {
			var report validation.Report
			if e := json.Unmarshal(env.Data, &report); e != nil || report.Status != "pass" || report.Checks[3].Status != "passed" {
				t.Fatal("binary did not validate full report", e)
			}
		}
		if tc.args[0] == "pack" && exit == 0 {
			var result app.PackResult
			if e := json.Unmarshal(env.Data, &result); e != nil {
				t.Fatal(e)
			}
			draft := strings.Contains(strings.Join(tc.args, " "), "--draft")
			if result.Draft != draft || result.Verified == draft || result.ArchiveSHA256 == "" {
				t.Fatal("wrong binary pack result", result)
			}
		}
		t.Logf("binary %s: exit=%d single JSON envelope; no stderr", strings.Join(tc.args, " "), exit)
	}
	marker := filepath.Join(t.TempDir(), "started")
	cancelled := filepath.Join(t.TempDir(), "cancelled.epub")
	c := exec.Command(bin, "pack", dir, "--output", cancelled, "--json")
	c.Env = append(os.Environ(), "PATH="+fakeBin, "KEPUB_FAKE_MARKER="+marker)
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	if e := c.Start(); e != nil {
		t.Fatal(e)
	}
	started := false
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		if _, e := os.Stat(marker); e == nil {
			started = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if e := c.Process.Signal(os.Interrupt); e != nil {
		t.Fatal(e)
	}
	e := c.Wait()
	var exit *exec.ExitError
	if !started || !errors.As(e, &exit) || exit.ExitCode() != 130 || stderr.Len() != 0 {
		t.Fatalf("cancel failed: %v stdout=%s stderr=%s", e, &stdout, &stderr)
	}
	var env map[string]any
	d := json.NewDecoder(&stdout)
	if e := d.Decode(&env); e != nil {
		t.Fatal(e)
	}
	var extra any
	if d.Decode(&extra) != io.EOF || env["ok"] != false || env["error"].(map[string]any)["code"] != "CANCELLED" {
		t.Fatal(env)
	}
	if _, e := os.Stat(cancelled); !os.IsNotExist(e) {
		t.Fatal("cancel published output", e)
	}
	t.Log("binary SIGINT during check: exit=130 single JSON envelope; no publication")
}
