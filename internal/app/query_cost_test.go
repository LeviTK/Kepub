package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/validation"
)

// Formal-export timing uses the same genuine histories as the workspace query
// matrix. Prepare them with TestR6HistoryMatrix/KEPUB_R6_PREPARE before this
// explicit benchmark; a missing input or checker is an error, never a PASS.
func BenchmarkR6FormalExport(b *testing.B) {
	data, err := os.ReadFile(filepath.Join(os.Getenv("KEPUB_R6_FIXTURES"), "fixtures.json"))
	if err != nil {
		b.Fatal("prepare the bounded real-acceptance R6 history matrix first", err)
	}
	var fixtures map[string]struct {
		Directory, Revision string
		Files               map[string][]byte
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		b.Fatal(err)
	}
	names := []string{}
	for name := range fixtures {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		f := fixtures[name]
		b.Run(name, func(b *testing.B) {
			out := filepath.Join(b.TempDir(), "formal.epub")
			readIO := func() map[string]uint64 {
				data, err := os.ReadFile("/proc/self/io")
				if err != nil {
					b.Fatal("Linux self-I/O counters unavailable", err)
				}
				values := map[string]uint64{}
				for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
					key, value, ok := strings.Cut(line, ": ")
					if !ok {
						b.Fatal("counter format", line)
					}
					n, err := strconv.ParseUint(value, 10, 64)
					if err != nil {
						b.Fatal(err)
					}
					values[key] = n
				}
				return values
			}
			before := readIO()
			for b.Loop() {
				r, err := ExportWorkspace(context.Background(), f.Directory, out, validation.Options{})
				if err != nil || !r.Verified || r.RevisionID != f.Revision || r.Validation.Status != "pass" {
					b.Fatal("actual formal export", r, err)
				}
				z, err := zip.OpenReader(out)
				if err != nil {
					b.Fatal(err)
				}
				seen := map[string]bool{}
				for _, entry := range z.File {
					if seen[entry.Name] {
						b.Fatal("duplicate ZIP entry", entry.Name)
					}
					seen[entry.Name] = true
					if entry.FileInfo().IsDir() {
						if entry.Name != "empty/" && entry.Name != "EPUB/" && entry.Name != "META-INF/" {
							b.Fatal("unexpected directory", entry.Name)
						}
						continue
					}
					reader, err := entry.Open()
					if err != nil {
						b.Fatal(err)
					}
					var output bytes.Buffer
					_, err = archive.CopyBounded(context.Background(), &output, reader, 128<<10)
					reader.Close()
					if want, ok := f.Files[entry.Name]; err != nil || !ok || !bytes.Equal(output.Bytes(), want) {
						b.Fatal("independent complete ZIP bytes", entry.Name, err)
					}
				}
				z.Close()
				if len(seen) != len(f.Files)+3 {
					b.Fatal("incomplete exported inventory", fmt.Sprint(seen))
				}
				if err := os.Remove(out); err != nil {
					b.Fatal(err)
				}
			}
			after := readIO()
			for _, metric := range []string{"rchar", "wchar", "read_bytes", "write_bytes"} {
				b.ReportMetric(float64(after[metric]-before[metric])/float64(b.N), "self-"+metric+"/op")
			}
		})
	}
}
