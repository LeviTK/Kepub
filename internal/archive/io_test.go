package archive

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/fault"
)

type r3Reader struct {
	io.Reader
	read  int
	after func()
}

func (r *r3Reader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read += n
	if r.after != nil {
		r.after()
	}
	return n, err
}

type r3ShortWriter struct{}

func (r3ShortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

type r3ErrorReader struct{ err error }

func (r r3ErrorReader) Read(p []byte) (int, error) { return 0, r.err }

type r3DataErrorReader struct{ err error }

func (r r3DataErrorReader) Read(p []byte) (int, error) {
	return copy(p, "abcdefghijklmnop"), r.err
}

func TestR3ActualReadBudget(t *testing.T) {
	for _, size := range []int{14, 15, 16, 31} {
		src := &r3Reader{Reader: strings.NewReader(strings.Repeat("x", size))}
		var dst bytes.Buffer
		n, err := CopyBounded(t.Context(), &dst, src, 15)
		want := min(size, 16)
		if n != int64(want) || src.read != want || dst.String() != strings.Repeat("x", want) {
			t.Fatalf("size=%d: consumed/written %d/%d/%q; want exactly %d", size, n, src.read, dst.String(), want)
		}
		var f *fault.Error
		if size <= 15 {
			if err != nil {
				t.Fatal("legal boundary rejected", size, err)
			}
		} else if !errors.As(err, &f) || f.Code != "ARCHIVE_LIMIT" || f.Exit != 1 {
			t.Fatal("actual over-limit bytes not classified", size, err)
		}
	}
	// MaxInt64+1 must not wrap to a negative reader limit.
	var dst bytes.Buffer
	if n, err := CopyBounded(t.Context(), &dst, strings.NewReader("abc"), math.MaxInt64); err != nil || n != 3 || dst.String() != "abc" {
		t.Fatal("int64 boundary", n, err)
	}
}

func TestR3BoundedCopyFaults(t *testing.T) {
	readFault := errors.New("injected read fault")
	if _, err := CopyBounded(t.Context(), io.Discard, r3ErrorReader{readFault}, 15); !errors.Is(err, readFault) {
		t.Fatal("read fault became a budget/format error", err)
	}
	// A reader may return both the probe byte and a real I/O error. The size
	// observation must not erase that error or a simultaneous cancellation.
	for _, cause := range []error{readFault, context.Canceled} {
		if n, err := CopyBounded(t.Context(), io.Discard, r3DataErrorReader{cause}, 15); n != 16 || !errors.Is(err, cause) {
			t.Fatal("probe byte hid the underlying read failure", n, err)
		}
	}
	src := &r3Reader{Reader: strings.NewReader(strings.Repeat("x", 31))}
	if _, err := CopyBounded(t.Context(), r3ShortWriter{}, src, 15); !errors.Is(err, io.ErrShortWrite) || src.read > 16 {
		t.Fatal("short write lost or read escaped probe allowance", err, src.read)
	}
	for _, early := range []bool{true, false} {
		ctx, cancel := context.WithCancel(t.Context())
		src := &r3Reader{Reader: strings.NewReader("abc")}
		if early {
			cancel()
		} else {
			src.after = cancel // Cancel while the source returns its first bytes.
		}
		_, err := CopyBounded(ctx, io.Discard, src, 15)
		cancel()
		if !errors.Is(err, context.Canceled) || early && src.read != 0 || src.read > 3 {
			t.Fatal("cancellation ignored or excessive read", early, src.read, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	src = &r3Reader{Reader: strings.NewReader(""), after: cancel}
	if _, err := CopyBounded(ctx, io.Discard, src, 15); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation delivered at EOF was ignored", err)
	}
}
