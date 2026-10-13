package references

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

type cancelledGraphReader struct {
	cancel context.CancelFunc
	reads  int
}

func (r *cancelledGraphReader) HasFile(bookpath.BookPath) bool { return true }
func (r *cancelledGraphReader) Read(bookpath.BookPath, int64) ([]byte, error) {
	r.reads++
	r.cancel()
	return nil, context.Canceled
}

func TestR4CancelledGraphStopsReading(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	src := &cancelledGraphReader{cancel: cancel}
	files := map[bookpath.BookPath]int64{"a.xhtml": 10, "b.xhtml": 10, "c.xhtml": 10}
	g, err := BuildSource(ctx, src, files, &publication.Publication{Version: "3.0"}, DefaultGraphLimits)
	if !errors.Is(err, context.Canceled) || g.Status != "" {
		t.Fatalf("cancellation became a usable graph: %+v %v", g, err)
	}
	if src.reads != 1 {
		t.Fatalf("graph continued reading after cancellation: got %d reads, want 1", src.reads)
	}
}

type graphReader struct {
	data   map[bookpath.BookPath][]byte
	reads  []bookpath.BookPath
	limits []int64
}

func (r *graphReader) HasFile(bp bookpath.BookPath) bool { _, ok := r.data[bp]; return ok }
func (r *graphReader) Read(bp bookpath.BookPath, limit int64) ([]byte, error) {
	r.reads = append(r.reads, bp)
	r.limits = append(r.limits, limit)
	data := r.data[bp]
	if int64(len(data)) > limit {
		return nil, fault.New(1, "RESOURCE_LIMIT", "bounded reader")
	}
	return data, nil
}

const r4XML = `<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="same"/><p id="same"/><p id="unique"/><a href="https://example.invalid/x"/><a href="missing.xhtml#z"/><a href="#same"/><z xmlns="urn:mystery"/></body></html>`

func r4Source() (*graphReader, map[bookpath.BookPath]int64, *publication.Publication) {
	r := &graphReader{data: map[bookpath.BookPath][]byte{"a.xhtml": []byte(r4XML), "b.xhtml": []byte(r4XML)}}
	files := map[bookpath.BookPath]int64{"a.xhtml": int64(len(r4XML)), "b.xhtml": int64(len(r4XML))}
	return r, files, &publication.Publication{Version: "3.0"}
}

func r4Limit(t *testing.T, err error) {
	t.Helper()
	var f *fault.Error
	if !errors.As(err, &f) || f.Code != "REFERENCE_LIMIT" || f.Exit != 1 {
		t.Fatalf("want REFERENCE_LIMIT/1, got %v", err)
	}
}

func TestR4CumulativeGraphCounts(t *testing.T) {
	for _, kind := range []string{"scan", "edges", "ids", "diagnostics"} {
		for _, offset := range []int64{-1, 0, 1} {
			t.Run(fmt.Sprintf("%s/%+d", kind, offset), func(t *testing.T) {
				src, files, p := r4Source()
				limits := DefaultGraphLimits
				switch kind {
				case "scan":
					limits.ScanBytes = 2*int64(len(r4XML)) + offset
				case "edges":
					limits.Edges = 6 + offset
				case "ids":
					limits.IDs = 6 + offset
				case "diagnostics":
					limits.Diagnostics = 6 + offset
				}
				g, err := BuildSource(context.Background(), src, files, p, limits)
				if offset < 0 {
					r4Limit(t, err)
					if !reflect.DeepEqual(g, Graph{}) {
						t.Fatal("partial graph escaped", g)
					}
					if kind == "scan" && len(src.reads) != 1 {
						t.Fatal("over-budget second resource read", src.reads)
					}
					_, blockers := g.CertainIncoming("a.xhtml", "same")
					if len(blockers) != 1 || blockers[0].Syntax != "graph" {
						t.Fatal("failed graph became proof of absence", blockers)
					}
					return
				}
				if err != nil || len(g.Edges) != 6 || len(g.Diagnostics) != 6 || g.Status != "partial" {
					t.Fatalf("lost complete applicable observations: %+v %v", g, err)
				}
				if !slices.Equal(src.reads, []bookpath.BookPath{"a.xhtml", "b.xhtml"}) {
					t.Fatal("non-deterministic reads", src.reads)
				}
				for _, bp := range []bookpath.BookPath{"a.xhtml", "b.xhtml"} {
					edges, _ := g.CertainIncoming(bp, "same")
					if len(edges) != 1 || edges[0].FragmentStatus != "ambiguous" {
						t.Fatal("duplicate identities lost", edges)
					}
				}
			})
		}
	}
}

func TestR4ReservationBoundaries(t *testing.T) {
	for _, offset := range []int64{-1, 0, 1} {
		t.Run(fmt.Sprintf("index/%+d", offset), func(t *testing.T) {
			b := builder{ctx: context.Background(), remaining: DefaultGraphLimits}
			b.remaining.IndexBytes = 2*(64+int64(len("repeated/source/location"))) + offset
			b.index("repeated/source/location")
			ok := b.index("repeated/source/location")
			if offset < 0 {
				r4Limit(t, b.err)
			} else if !ok {
				t.Fatal(b.err)
			}
		})
		t.Run(fmt.Sprintf("coverage/%+d", offset), func(t *testing.T) {
			b := builder{ctx: context.Background(), remaining: DefaultGraphLimits, covered: map[string]int{}, g: Graph{ParserVersion: 2}}
			// Coverage has five fields, three strings, an empty slice and an int.
			one := int64(128 + 5*64 + 3*2 + 2 + 32 + 6*(len("a.xhtml")+len("unknown-xml")+len("partial")))
			b.remaining.CoverageBytes = 2*one + 2*(3+6*int64(len("unknown vocabulary"))) + offset
			b.cover("a.xhtml", "unknown-xml", "partial", "unknown vocabulary")
			b.cover("b.xhtml", "unknown-xml", "partial", "unknown vocabulary")
			if offset < 0 {
				r4Limit(t, b.err)
			} else if b.err != nil || len(b.g.Coverage) != 2 {
				t.Fatal(b.err, b.g)
			}
		})
		t.Run(fmt.Sprintf("result/%+d", offset), func(t *testing.T) {
			b := builder{ctx: context.Background(), remaining: DefaultGraphLimits}
			e := Edge{Source: "a.xhtml", Location: "/p[1]/@href", Syntax: "xhtml.href", Href: "bad", Status: "invalid", FragmentStatus: "blocked", ParserVersion: 2}
			// Edge has eight fields: six strings, nil Target, and one integer.
			one := int64(128 + 8*64 + 6*2 + 4 + 32 + 6*(len("a.xhtml")+len("/p[1]/@href")+len("xhtml.href")+len("bad")+len("invalid")+len("blocked")))
			b.remaining.ResultBytes = 2*one + offset
			b.edge(e)
			b.edge(e)
			if offset < 0 {
				r4Limit(t, b.err)
				if len(b.g.Edges) != 1 {
					t.Fatal("appended before paying", b.g)
				}
			} else if b.err != nil || len(b.g.Edges) != 2 {
				t.Fatal(b.err, b.g)
			}
			actual, err := json.Marshal(b.g.Edges)
			if err != nil || int64(len(actual)) > 2*one {
				t.Fatal("reservation does not bound serialized output", len(actual), err)
			}
		})
	}
}

func TestR4ByteBudgetsAndDeterminism(t *testing.T) {
	for _, kind := range []string{"index", "coverage", "result"} {
		src, files, p := r4Source()
		limits := DefaultGraphLimits
		switch kind {
		case "index":
			limits.IndexBytes = 100
		case "coverage":
			limits.CoverageBytes = 1200
		case "result":
			limits.ResultBytes = 3000
		}
		g, err := BuildSource(context.Background(), src, files, p, limits)
		r4Limit(t, err)
		if !reflect.DeepEqual(g, Graph{}) || len(src.reads) > 1 {
			t.Fatal("late rejection", kind, len(src.reads), g)
		}
	}
	src, files, p := r4Source()
	p.Manifest = []publication.Item{{ID: "a", Path: "a.xhtml", MediaType: "application/xhtml+xml", Exists: true}, {ID: "b", Path: "b.xhtml", MediaType: "application/xhtml+xml", Exists: true}}
	first, err := BuildSource(context.Background(), src, files, p, DefaultGraphLimits)
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(p.Manifest)
	second, err := BuildSource(context.Background(), src, files, p, DefaultGraphLimits)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatal("manifest order changed graph", err)
	}
	for _, bp := range []bookpath.BookPath{"a.xhtml", "b.xhtml"} {
		if !bytes.Equal(src.data[bp], []byte(r4XML)) {
			t.Fatal("mutated source")
		}
	}
}

func TestR4ReadGrowthAndDeadline(t *testing.T) {
	src, files, p := r4Source()
	files["a.xhtml"] = 1 // Reader still obeys its actual byte limit.
	limits := DefaultGraphLimits
	limits.ScanBytes = 10
	_, err := BuildSource(context.Background(), src, files, p, limits)
	r4Limit(t, err)
	if len(src.reads) != 1 || src.limits[0] != 10 {
		t.Fatal("unbounded actual read", src.limits)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	src.reads = nil
	_, err = BuildSource(ctx, src, files, p, DefaultGraphLimits)
	if !errors.Is(err, context.DeadlineExceeded) || len(src.reads) != 0 {
		t.Fatal("ignored deadline", err, src.reads)
	}
}

func TestR4XMLCoverageReservation(t *testing.T) {
	b := builder{ctx: context.Background(), remaining: DefaultGraphLimits}
	c := &publication.XMLCoverage{Resources: []publication.XMLResourceCoverage{{BookPath: "a.xhtml", Status: "partial", Notations: []xmltext.Notation{{Name: "n", SystemID: strings.Repeat("x", 500)}}}}}
	b.remaining.CoverageBytes = 2000
	b.xmlCoverage(c)
	r4Limit(t, b.err)
	if b.g.XMLCoverage != nil {
		t.Fatal("XML coverage appended before reservation")
	}
}

func TestR4FilteredResultHeaderBudget(t *testing.T) {
	// Nine Graph fields: four strings, three empty slices, int and nil pointer.
	header := int64(128 + 9*64 + 4*2 + 3*2 + 32 + 4 + 6*(len("complete")+len("archive resources; selected rootfile only; extraction is not conformance validation")+len("both")))
	reserved := header + 6*(4096+4)
	for _, offset := range []int64{-1, 0, 1} {
		limits := DefaultGraphLimits
		limits.ResultBytes = reserved + offset
		g, err := BuildSource(context.Background(), &graphReader{}, nil, &publication.Publication{}, limits)
		if offset < 0 {
			r4Limit(t, err)
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		filtered, err := g.Filter(strings.Repeat("x", 4096), "outgoing")
		if err != nil {
			t.Fatal(err)
		}
		actual, err := json.Marshal(filtered)
		if err != nil || int64(len(actual)) > reserved {
			t.Fatal("filter escaped pre-build reservation", len(actual), err)
		}
	}
}

func BenchmarkR4BoundedInventory(b *testing.B) {
	for _, count := range []int{64, 4096} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			files := map[bookpath.BookPath]int64{}
			for i := 0; i < count; i++ {
				files[bookpath.BookPath(fmt.Sprintf("%04d.xhtml", i))] = 10
			}
			limits := DefaultGraphLimits
			limits.IndexBytes = 1024
			src := &graphReader{}
			p := &publication.Publication{Version: "3.0"}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := BuildSource(context.Background(), src, files, p, limits)
				var f *fault.Error
				if !errors.As(err, &f) || f.Code != "REFERENCE_LIMIT" || len(src.reads) != 0 {
					b.Fatal(err)
				}
			}
		})
	}
}
