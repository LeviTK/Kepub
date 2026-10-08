package workspace

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/metadata"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/testfixture"
	"github.com/LeviTK/Kepub/internal/validation"
)

// These are test input identities, not a replacement history store or approval.
type r6Fixture struct {
	Directory, Revision, FirstRevision, FirstTask, ActiveTask, OriginalSHA256 string
	Depth                                                                     int
	Files                                                                     map[string][]byte
}

// Only copies our bounded, closed synthetic fixture, including every historical
// record verbatim. No hard links, forged approval, pointer rewind or re-signing.
func r6CopyDirectory(t testing.TB, source, destination string) {
	t.Helper()
	if err := os.Mkdir(destination, 0700); err != nil {
		t.Fatal(err)
	}
	err := filepath.WalkDir(source, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == source {
			return nil
		}
		relative, err := filepath.Rel(source, name)
		if err != nil {
			return err
		}
		out := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.Mkdir(out, 0700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsafe synthetic fixture %s", relative)
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		return os.WriteFile(out, data, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func r6History(t testing.TB, parent string, depth, cssBytes int) r6Fixture {
	t.Helper()
	files := map[string][]byte{
		"mimetype":               []byte("application/epub+zip"),
		"META-INF/container.xml": []byte(`<container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`),
		"EPUB/package.opf":       []byte(`<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="uid">urn:uuid:12345678-1234-1234-1234-123456789012</dc:identifier><dc:title id="title">Title</dc:title><dc:creator id="creator">Writer</dc:creator><dc:language>en</dc:language><meta property="dcterms:modified">2026-10-08T00:00:00Z</meta></metadata><manifest><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/><item id="css" href="style.css" media-type="text/css"/><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/></manifest><spine><itemref idref="chapter"/></spine></package>`),
		"EPUB/chapter.xhtml":     []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Chapter</title></head><body><h1 id="start">Chapter</h1><p>Original &amp; precise.</p></body></html>`),
		"EPUB/nav.xhtml":         []byte(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="chapter.xhtml#start">Chapter</a></li></ol></nav></body></html>`),
		"EPUB/style.css":         []byte("p { color: #123456; }\n"),
	}
	random := rand.New(rand.NewPCG(17, 29))
	padding := make([]byte, cssBytes)
	for i := range padding {
		padding[i] = 'a' + byte(random.IntN(26))
	}
	files["EPUB/style.css"] = append(files["EPUB/style.css"], append(append([]byte("/*"), padding...), []byte("*/\n")...)...)
	names := []string{}
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := []testfixture.Entry{{Name: "mimetype", Data: files["mimetype"]}}
	for _, name := range names {
		if name != "mimetype" {
			entries = append(entries, testfixture.Entry{Name: name, Data: files[name]})
		}
	}
	entries = append(entries, testfixture.Entry{Name: "empty/", Mode: os.ModeDir | 0700})
	source := filepath.Join(parent, "source.epub")
	testfixture.ZIP(t, source, entries)
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	f := r6Fixture{Directory: filepath.Join(parent, "workspace"), Depth: depth, Files: files, OriginalSHA256: fmt.Sprintf("%x", sha256.Sum256(original))}
	w, err := Create(f.Directory, source, Options{Context: context.Background()})
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	old := "Title"
	for i := 1; i <= depth; i++ {
		next := fmt.Sprintf("Revision %03d", i)
		request, err := json.Marshal(Request{1, []Operation{{"metadata.set", 1, metadata.Set{Namespace: metadata.DC, LocalName: "title", ID: "title", ExpectedOldValue: old, NewValue: next}}}})
		if err != nil {
			t.Fatal(err)
		}
		p, err := w.Plan(request)
		if err != nil {
			t.Fatal(err)
		}
		plan, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		e, err := w.Apply(plan)
		if err != nil {
			t.Fatal(err)
		}
		d, err := w.Accept(context.Background(), e.TaskID, validation.Options{})
		if err != nil || d.Status != "accepted" || d.Validation == nil || d.Validation.Status != "pass" {
			t.Fatal("history requires real formal acceptance", i, d, err)
		}
		if i == 1 {
			f.FirstRevision, f.FirstTask = d.RevisionID, e.TaskID
		}
		f.Revision = d.RevisionID
		old = next
	}
	files["EPUB/package.opf"] = bytes.Replace(files["EPUB/package.opf"], []byte(">Title</dc:title>"), []byte(">"+old+"</dc:title>"), 1)
	// Leave one legitimate pending candidate for diff, without accepting it.
	request, _ := json.Marshal(Request{1, []Operation{{"metadata.set", 1, metadata.Set{Namespace: metadata.DC, LocalName: "title", ID: "title", ExpectedOldValue: old, NewValue: "Pending"}}}})
	p, err := w.Plan(request)
	if err != nil {
		t.Fatal(err)
	}
	plan, _ := json.Marshal(p)
	e, err := w.Apply(plan)
	if err != nil {
		t.Fatal(err)
	}
	f.ActiveTask = e.TaskID
	return f
}

// Default CI remains bounded at 1/10; the explicitly requested performance
// corpus adds 100, still at most 64KiB per CSS file. Nothing is a mock accept.
func TestR6HistoryMatrix(t *testing.T) {
	root := os.Getenv("KEPUB_R6_PREPARE")
	depths := []int{1, 10}
	if root == "" {
		root = t.TempDir()
	} else {
		if !filepath.IsAbs(root) {
			t.Fatal("explicit performance fixture output must be absolute")
		}
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal("explicit performance output must be absent", err)
		}
		depths = append(depths, 100)
	}
	fixtures := map[string]r6Fixture{}
	for _, size := range []int{1024, 64 << 10} {
		for _, depth := range depths {
			name := fmt.Sprintf("%d/%d", depth, size)
			t.Run(name, func(t *testing.T) {
				parent := filepath.Join(root, strings.ReplaceAll(name, "/", "-"))
				if err := os.Mkdir(parent, 0700); err != nil {
					t.Fatal(err)
				}
				f := r6History(t, parent, depth, size)
				fixtures[name] = f
				policy := defaultResourceIO(t.Context())
				visits := 0
				policy.onRevisionRead = func(string) { visits++ }
				w, err := openWithIO(f.Directory, policy)
				if err != nil {
					t.Fatal(err)
				}
				defer w.Close()
				// This fixture has an active execution. Open also verifies its
				// envelope twice; those separate guards are intentionally retained.
				if visits != 3*depth {
					t.Fatalf("active Open visited %d revisions; want %d", visits, 3*depth)
				}
				visits = 0
				a, _, r, err := w.AcceptedSnapshot()
				if err != nil {
					t.Fatal(err)
				}
				defer a.Close()
				if r.ID != f.Revision || visits != depth {
					t.Fatalf("one snapshot visited %d source revisions; want %d; revision %s", visits, depth, r.ID)
				}
				for name, want := range f.Files {
					got, err := a.Read(bookpath.BookPath(name), archive.DefaultLimits.FileBytes)
					if err != nil || !bytes.Equal(got, want) {
						t.Fatal("complete accepted bytes", name, err)
					}
				}
			})
		}
	}
	data, err := json.MarshalIndent(fixtures, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "fixtures.json"), data, 0600); err != nil {
		t.Fatal("fixture identity record", err)
	}
}

// Linux process counters include actual file reads/writes and JSON, not an
// estimate from Tree sizes. rchar/wchar are not physical I/O or peak memory.
func r6ProcessIO(t testing.TB) map[string]uint64 {
	t.Helper()
	data, err := os.ReadFile("/proc/self/io")
	if err != nil {
		t.Fatal("Linux I/O measurement unavailable; not a PASS", err)
	}
	values := map[string]uint64{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok {
			t.Fatal("process I/O counter format", line)
		}
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		values[key] = n
	}
	return values
}

func r6Query(t testing.TB, f r6Fixture, operation string, policy resourceIO) {
	t.Helper()
	w, err := openWithIO(f.Directory, policy)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	switch operation {
	case "content", "search":
		a, _, r, err := w.AcceptedSnapshot()
		if err != nil {
			t.Fatal(err)
		}
		defer a.Close()
		if r.ID != f.Revision {
			t.Fatal("query used candidate or wrong accepted revision", r.ID)
		}
		p, err := publication.Load(a, r.Rootfile)
		if err != nil {
			t.Fatal(err)
		}
		query := "Original"
		if operation == "content" {
			result, err := publication.ReadContent(a, p, "EPUB/chapter.xhtml", publication.ContentOptions{Query: &query})
			if err != nil || result.MatchedCount != 1 || result.ReturnedCount != 1 || result.Nodes[0].Text != "Original & precise." || result.Nodes[0].Locator != "/html[1]/body[1]/p[1]" || result.ResourceSHA256 != fmt.Sprintf("%x", sha256.Sum256(f.Files["EPUB/chapter.xhtml"])) {
				t.Fatal("content oracle", result, err)
			}
		} else {
			result, err := publication.SearchContent(a, p, publication.ContentOptions{Query: &query})
			if err != nil || result.MatchedCount != 1 || result.ReturnedCount != 1 || len(result.Results) != 1 || result.Results[0].Text != "Original & precise." {
				t.Fatal("search oracle", result, err)
			}
		}
	case "diff":
		r, err := w.TaskDiff(f.ActiveTask)
		if err != nil || !r.MatchesExecution || r.Metadata.NewValue == nil || *r.Metadata.NewValue != "Pending" {
			t.Fatal("diff oracle", r, err)
		}
	case "old-status":
		r, err := w.TaskStatus(f.FirstTask)
		if err != nil || r.Status != "accepted" || r.Decision == nil || r.Decision.RevisionID != f.FirstRevision {
			t.Fatal("history oracle", r, err)
		}
	default:
		t.Fatal("unknown test query", operation)
	}
}

func BenchmarkR6Queries(b *testing.B) {
	fixtures := map[string]r6Fixture{}
	if root := os.Getenv("KEPUB_R6_FIXTURES"); root != "" {
		data, err := os.ReadFile(filepath.Join(root, "fixtures.json"))
		if err != nil || json.Unmarshal(data, &fixtures) != nil {
			b.Fatal("prepared real-history fixtures required", err)
		}
	}
	for _, size := range []int{1024, 64 << 10} {
		for _, depth := range []int{1, 10, 100} {
			name := fmt.Sprintf("%d/%d", depth, size)
			f, ok := fixtures[name]
			if !ok {
				f = r6History(b, b.TempDir(), depth, size)
			}
			for _, operation := range []string{"content", "search", "diff", "old-status"} {
				b.Run(name+"/"+operation, func(b *testing.B) {
					policy := defaultResourceIO(context.Background())
					visits, originals := 0, 0
					policy.onRevisionRead = func(string) { visits++ }
					policy.onOriginalRead = func() { originals++ }
					before := r6ProcessIO(b)
					for b.Loop() {
						r6Query(b, f, operation, policy)
					}
					after := r6ProcessIO(b)
					b.ReportMetric(float64(visits)/float64(b.N), "revision-visits/op")
					b.ReportMetric(float64(originals)/float64(b.N), "original-reads/op")
					for _, metric := range []string{"rchar", "wchar", "read_bytes", "write_bytes"} {
						b.ReportMetric(float64(after[metric]-before[metric])/float64(b.N), metric+"/op")
					}
				})
			}
		}
	}
}
