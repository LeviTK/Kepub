package references

import (
	"context"
	"errors"
	"reflect"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
)

// GraphLimits are per complete build, including unmanifested resources. Byte
// reservations count repeated strings too; they are not an RSS measurement.
type GraphLimits struct {
	ScanBytes     int64
	Edges         int64
	IDs           int64
	Diagnostics   int64
	CoverageBytes int64
	IndexBytes    int64
	ResultBytes   int64
}

var DefaultGraphLimits = GraphLimits{64 << 20, 100000, 100000, 10000, 8 << 20, 32 << 20, 32 << 20}

func (b *builder) ready() bool {
	if b.err == nil {
		b.err = b.ctx.Err()
	}
	return b.err == nil
}

func (b *builder) take(name string, left *int64, n int64) bool {
	if !b.ready() {
		return false
	}
	if n < 0 || n > *left {
		b.err = fault.New(1, "REFERENCE_LIMIT", "reference graph exceeds %s budget", name)
		return false
	}
	*left -= n
	return true
}

func (b *builder) index(strings ...string) bool {
	n := int64(64)
	for _, s := range strings {
		n += int64(len(s))
	}
	return b.take("index bytes", &b.remaining.IndexBytes, n)
}

// jsonReservation is an allocation-free conservative bound, not a marshal of
// the completed result. Each record/field reserves fixed JSON punctuation and
// numbers, strings reserve the worst-case six-byte JSON escape per input byte.
// The spare fixed overhead also covers later edge/coverage status transitions.
func jsonReservation(v reflect.Value) int64 {
	if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return 4
		}
		return jsonReservation(v.Elem())
	}
	switch v.Kind() {
	case reflect.String:
		return 2 + 6*int64(v.Len())
	case reflect.Slice, reflect.Array:
		n := int64(2)
		for i := 0; i < v.Len(); i++ {
			n += 1 + jsonReservation(v.Index(i))
		}
		return n
	case reflect.Struct:
		n := int64(128)
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).Tag.Get("json") != "-" {
				n += 64 + jsonReservation(v.Field(i))
			}
		}
		return n
	default:
		return 32
	}
}

func (b *builder) result(value any) bool {
	return b.take("serialized result bytes", &b.remaining.ResultBytes, jsonReservation(reflect.ValueOf(value)))
}

func (b *builder) edge(e Edge) {
	if b.take("edges", &b.remaining.Edges, 1) && b.result(e) {
		b.g.Edges = append(b.g.Edges, e)
	}
}

func (b *builder) report(d publication.Diagnostic) {
	if b.take("diagnostics", &b.remaining.Diagnostics, 1) && b.result(d) {
		b.g.Diagnostics = append(b.g.Diagnostics, d)
	}
}

func (b *builder) xmlCoverage(c *publication.XMLCoverage) {
	if c == nil || !b.ready() {
		return
	}
	n := jsonReservation(reflect.ValueOf(c))
	if b.take("coverage bytes", &b.remaining.CoverageBytes, n) && b.take("serialized result bytes", &b.remaining.ResultBytes, n) {
		b.g.XMLCoverage = b.g.XMLCoverage.Merge(c)
	}
}

// Read charges the frozen size before asking the reader for bytes. Actual data
// still cannot exceed the remaining cap if the declared inventory drifts.
func (b *builder) Read(bp bookpath.BookPath, max int64) ([]byte, error) {
	if !b.ready() {
		return nil, b.err
	}
	size := b.files[bp]
	left := b.remaining.ScanBytes
	if !b.take("scanned bytes", &b.remaining.ScanBytes, size) {
		return nil, b.err
	}
	data, err := b.src.Read(bp, min(max, left))
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		b.err = err
	} else if err != nil && left < max {
		var f *fault.Error
		if errors.As(err, &f) && f.Code == "RESOURCE_LIMIT" {
			b.err = fault.New(1, "REFERENCE_LIMIT", "reference graph exceeds scanned bytes budget")
		}
	}
	if extra := int64(len(data)) - size; extra > 0 {
		b.take("scanned bytes", &b.remaining.ScanBytes, extra)
	}
	if !b.ready() {
		return nil, b.err
	}
	return data, err
}

func (b *builder) HasFile(bp bookpath.BookPath) bool { return b.src.HasFile(bp) }
