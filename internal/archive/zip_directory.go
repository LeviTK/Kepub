package archive

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"errors"
	"io"
)

// ZIP framing follows Go 1.27.1 archive/zip/reader.go, including its EOCD
// search, ZIP64 locator, SFX fallback and modulo-65536 count compatibility:
// https://github.com/golang/go/blob/go1.27.1/src/archive/zip/reader.go
// Only one central record is parsed at a time. The inventory gate runs before
// retaining it or reading the next one; advertised counts never size a slice.
func readZIPDirectory(ctx context.Context, r io.ReaderAt, size int64, visit func(*zip.File) error) ([]*zip.File, error) {
	start, base, count, err := zipDirectoryEnd(r, size)
	if err != nil {
		return nil, err
	}
	stream := io.NewSectionReader(r, start, size-start)
	files := []*zip.File{}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		f, err := zipDirectoryFile(stream, r, base)
		if err == zip.ErrFormat || err == io.ErrUnexpectedEOF {
			if uint16(len(files)) != uint16(count) {
				return nil, err
			}
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		if err := visit(f); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
}

func zipDirectoryEnd(r io.ReaderAt, size int64) (start, base int64, count uint64, err error) {
	u16, u32, u64 := binary.LittleEndian.Uint16, binary.LittleEndian.Uint32, binary.LittleEndian.Uint64
	var end []byte
	var endOffset int64
	for _, window := range []int64{1024, 65 * 1024} {
		n := min(window, size)
		b := make([]byte, int(n))
		if _, err := r.ReadAt(b, size-n); err != nil && err != io.EOF {
			return 0, 0, 0, err
		}
		for i := len(b) - 22; i >= 0; i-- {
			if u32(b[i:]) != 0x06054b50 {
				continue
			}
			if i+22+int(u16(b[i+20:])) > len(b) {
				break
			}
			end, endOffset = b[i:], size-n+int64(i)
			break
		}
		if end != nil {
			break
		}
		if n == size {
			return 0, 0, 0, zip.ErrFormat
		}
	}
	if end == nil {
		return 0, 0, 0, zip.ErrFormat
	}
	count, length, offset := uint64(u16(end[10:])), uint64(u32(end[12:])), uint64(u32(end[16:]))
	if count == 0xffff || length == 0xffffffff || offset == 0xffffffff {
		if endOffset >= 20 {
			var locator [20]byte
			if _, err := r.ReadAt(locator[:], endOffset-20); err != nil {
				return 0, 0, 0, err
			}
			p := int64(u64(locator[8:]))
			if u32(locator[:]) == 0x07064b50 && u32(locator[4:]) == 0 && u32(locator[16:]) == 1 && p >= 0 {
				var z64 [56]byte
				if _, err := r.ReadAt(z64[:], p); err != nil {
					return 0, 0, 0, err
				}
				if u32(z64[:]) != 0x06064b50 {
					return 0, 0, 0, zip.ErrFormat
				}
				endOffset = p
				count, length, offset = u64(z64[32:]), u64(z64[40:]), u64(z64[48:])
			}
		}
	}
	if length > 1<<63-1 || offset > 1<<63-1 {
		return 0, 0, 0, zip.ErrFormat
	}
	base = endOffset - int64(length) - int64(offset)
	start = base + int64(offset)
	if start < 0 || start >= size {
		return 0, 0, 0, zip.ErrFormat
	}
	if base > 0 {
		// Go probes a complete header at the unadjusted offset, not just its
		// signature. Insecure paths do not invalidate that structural probe.
		_, err := zipDirectoryFile(io.NewSectionReader(r, int64(offset), size-int64(offset)), r, 0)
		if err == nil || errors.Is(err, zip.ErrInsecurePath) {
			base, start = 0, int64(offset)
		}
	}
	return start, base, count, nil
}

// oneZIPView exists only for one zip.NewReader constructor. Afterwards source
// is switched once, before the file can be opened, to the unchanged input.
// The virtual zero prefix preserves the original base INSIDE zip.File, before
// File.Open constructs SectionReaders (shifting ReadAt offsets is not equivalent
// when ZIP64 sizes/offsets overflow). No prefix-sized allocation occurs.
type oneZIPView struct {
	metadata []byte
	start    int64
	source   io.ReaderAt
}

func (v *oneZIPView) ReadAt(p []byte, off int64) (int, error) {
	if v.source != nil {
		return v.source.ReadAt(p, off)
	}
	end := v.start + int64(len(v.metadata))
	if off < 0 || off >= end {
		return 0, io.EOF
	}
	n := int(min(int64(len(p)), end-off))
	clear(p[:n])
	if off+int64(n) > v.start {
		begin := max(off, v.start)
		copy(p[begin-off:n], v.metadata[begin-v.start:])
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

func zipDirectoryFile(stream io.Reader, source io.ReaderAt, base int64) (*zip.File, error) {
	var header [46]byte
	if _, err := io.ReadFull(stream, header[:]); err != nil {
		return nil, err
	}
	if binary.LittleEndian.Uint32(header[:]) != 0x02014b50 {
		return nil, zip.ErrFormat
	}
	// Three uint16 fields bound transient metadata, independent of count. Do
	// not retain extras/comments for the complete archive: Open only needs the
	// fields decoded by the standard library, and the original local headers.
	u16 := binary.LittleEndian.Uint16
	n := int(u16(header[28:])) + int(u16(header[30:])) + int(u16(header[32:]))
	record := make([]byte, 46+n)
	copy(record, header[:])
	if _, err := io.ReadFull(stream, record[46:]); err != nil {
		return nil, err
	}
	start := max(base, 0)
	offset := uint64(start - base)
	var end [22]byte
	p16, p32, p64 := binary.LittleEndian.PutUint16, binary.LittleEndian.PutUint32, binary.LittleEndian.PutUint64
	p32(end[:], 0x06054b50)
	p16(end[8:], 1)
	p16(end[10:], 1)
	p32(end[12:], uint32(len(record)))
	if offset >= 0xffffffff {
		var z64 [76]byte // end record and locator, no extensible sector
		p32(z64[:], 0x06064b50)
		p64(z64[4:], 44)
		p64(z64[24:], 1)
		p64(z64[32:], 1)
		p64(z64[40:], uint64(len(record)))
		p64(z64[48:], offset)
		p32(z64[56:], 0x07064b50)
		p64(z64[64:], uint64(start)+uint64(len(record)))
		p32(z64[72:], 1)
		record = append(record, z64[:]...)
		p32(end[16:], 0xffffffff)
	} else {
		p32(end[16:], uint32(offset))
	}
	record = append(record, end[:]...)
	v := &oneZIPView{metadata: record, start: start}
	z, err := zip.NewReader(v, start+int64(len(record)))
	if err != nil {
		return nil, err
	}
	if len(z.File) != 1 {
		return nil, zip.ErrFormat
	}
	f := z.File[0]
	f.Extra, f.Comment = nil, ""
	v.metadata, v.source = nil, source
	return f, nil
}
