package xmltext

import (
	"bytes"
	"encoding/binary"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/fault"
)

const DecodedLimit = 16 << 20

// The parser consumes UTF-8, but every interval refers to the original resource.
// No document serialization or external charset/entity resolver is involved.
type source struct {
	text          []byte
	order         binary.ByteOrder
	bom           int
	offsets       []uint32
	original      *source
	spans         []sourceSpan
	uncertainties []sourceSpan
	crPositions   []int
}

// XML S is exactly space, tab, CR and LF, not Go's regexp \s or Unicode space.
var declaration = regexp.MustCompile(`^<\?xml[\x20\x09\x0d\x0a]+version[\x20\x09\x0d\x0a]*=[\x20\x09\x0d\x0a]*(?:"1\.0"|'1\.0')(?:[\x20\x09\x0d\x0a]+encoding[\x20\x09\x0d\x0a]*=[\x20\x09\x0d\x0a]*(?:"([A-Za-z][A-Za-z0-9._-]*)"|'([A-Za-z][A-Za-z0-9._-]*)'))?(?:[\x20\x09\x0d\x0a]+standalone[\x20\x09\x0d\x0a]*=[\x20\x09\x0d\x0a]*(?:"(?:yes|no)"|'(?:yes|no)'))?[\x20\x09\x0d\x0a]*\?>`)

func decode(input []byte) (source, error) {
	s := source{}
	invalid := func() (source, error) {
		return source{}, fault.New(1, "XML_NOT_WELL_FORMED", "invalid or mismatched XML encoding/declaration")
	}
	if len(input) > Limit {
		return s, fault.New(1, "XML_LIMIT", "raw XML exceeds 8 MiB including BOM")
	}
	switch {
	case bytes.HasPrefix(input, []byte{0xff, 0xfe, 0, 0}), bytes.HasPrefix(input, []byte{0, 0, 0xfe, 0xff}), bytes.HasPrefix(input, []byte{0, 0, 0, '<'}), bytes.HasPrefix(input, []byte{'<', 0, 0, 0}):
		return s, fault.New(3, "UNSUPPORTED_XML_ENCODING", "UTF-32 is outside the supported encoding subset")
	case bytes.HasPrefix(input, []byte{0xff, 0xfe}):
		s.order, s.bom = binary.LittleEndian, 2
	case bytes.HasPrefix(input, []byte{0xfe, 0xff}):
		s.order, s.bom = binary.BigEndian, 2
	case bytes.HasPrefix(input, []byte{0, '<', 0, '?'}):
		s.order = binary.BigEndian
	case bytes.HasPrefix(input, []byte{'<', 0, '?', 0}):
		s.order = binary.LittleEndian
	case bytes.HasPrefix(input, []byte{0xef, 0xbb, 0xbf}):
		s.bom = 3
	}
	if s.order == nil {
		s.text = input[s.bom:]
		if !utf8.Valid(s.text) {
			return invalid()
		}
	} else {
		if (len(input)-s.bom)%2 != 0 {
			return invalid()
		}
		s.offsets = []uint32{uint32(s.bom)}
		for i := s.bom; i < len(input); {
			start := i
			u := s.order.Uint16(input[i:])
			i += 2
			r := rune(u)
			if utf16.IsSurrogate(r) {
				if u < 0xd800 || u > 0xdbff || i == len(input) {
					return invalid()
				}
				v := s.order.Uint16(input[i:])
				i += 2
				if v < 0xdc00 || v > 0xdfff {
					return invalid()
				}
				r = utf16.DecodeRune(r, rune(v))
			}
			if len(s.text)+utf8.RuneLen(r) > DecodedLimit {
				return source{}, fault.New(1, "XML_LIMIT", "decoded XML exceeds 16 MiB")
			}
			s.text = utf8.AppendRune(s.text, r)
			for n := 1; n < utf8.RuneLen(r); n++ {
				s.offsets = append(s.offsets, uint32(start))
			}
			s.offsets = append(s.offsets, uint32(i))
		}
	}
	enc := ""
	if bytes.HasPrefix(s.text, []byte("<?xml")) && len(s.text) > 5 && strings.ContainsRune(" \t\r\n?", rune(s.text[5])) {
		m := declaration.FindSubmatch(s.text)
		if m == nil {
			return invalid()
		}
		enc = strings.ToUpper(string(m[1]) + string(m[2]))
	}
	if enc != "" && enc != "UTF-8" && enc != "UTF-16" && enc != "UTF-16LE" && enc != "UTF-16BE" {
		return source{}, fault.New(3, "UNSUPPORTED_XML_ENCODING", "declared encoding is outside UTF-8/UTF-16")
	}
	if s.order == nil {
		if enc != "" && enc != "UTF-8" {
			return invalid()
		}
	} else {
		if enc == "UTF-16" && s.bom != 2 {
			return invalid()
		}
		if enc != "" && enc != "UTF-16" && !(enc == "UTF-16LE" && s.order == binary.LittleEndian) && !(enc == "UTF-16BE" && s.order == binary.BigEndian) {
			return invalid()
		}
		if s.bom == 0 && enc == "" {
			return invalid()
		}
	}
	return s, nil
}

// DecodeText is also used for bounded human diffs. It validates the physical
// encoding/declaration without parsing or normalizing the document's text.
func DecodeText(input []byte) (string, error) {
	s, err := decode(input)
	if err != nil {
		return "", err
	}
	return string(s.text), nil
}

func (s source) offset(n int) int {
	if s.original != nil {
		i := sort.Search(len(s.spans), func(i int) bool { return s.spans[i].end >= n })
		if i == len(s.spans) {
			return s.original.offset(len(s.original.text))
		}
		span := s.spans[i]
		if span.linear {
			return s.original.offset(span.from + n - span.start)
		}
		if n == span.end {
			return s.original.offset(span.to)
		}
		return s.original.offset(span.from)
	}
	if s.order == nil {
		return n + s.bom
	}
	return int(s.offsets[n])
}

// normalizeRun decodes one literal piece of a linear span with XML line-ending
// normalization and returns the text plus the original byte offset of every
// decoded byte and the piece end. The offsets come from the piece's own span, so
// a stream junction (for example a zero-length entity reference) cannot move
// them onto the neighbouring piece.
func (s source) normalizeRun(span sourceSpan, lo, hi int) (string, []int, bool) {
	stream := s.text[lo:hi]
	offset := func(n int) int { return s.original.offset(span.from + n - span.start) }
	out := make([]byte, 0, len(stream))
	offsets := make([]int, 0, len(stream)+1)
	si := 0
	for si < len(stream) {
		if stream[si] == '\r' {
			out = append(out, '\n')
			offsets = append(offsets, offset(lo+si))
			si++
			if si < len(stream) && stream[si] == '\n' {
				si++
			}
			continue
		}
		r, size := utf8.DecodeRune(stream[si:])
		if r == utf8.RuneError && size <= 1 {
			return "", nil, false
		}
		out = append(out, stream[si:si+size]...)
		for k := 0; k < size; k++ {
			offsets = append(offsets, offset(lo+si+k))
		}
		si += size
	}
	offsets = append(offsets, offset(lo+si))
	return string(out), offsets, true
}

// decodeEntityReference decodes one entity-reference spelling that the XML
// decoder itself expands: the predefined names and numeric character references.
// Internal entities are already expanded into generated spans and are not
// writable literal source.
func decodeEntityReference(ref string) (string, bool) {
	switch ref {
	case "&amp;":
		return "&", true
	case "&lt;":
		return "<", true
	case "&gt;":
		return ">", true
	case "&apos;":
		return "'", true
	case "&quot;":
		return "\"", true
	}
	if !strings.HasPrefix(ref, "&#") || !strings.HasSuffix(ref, ";") {
		return "", false
	}
	body := ref[2 : len(ref)-1]
	base := 10
	if strings.HasPrefix(body, "x") || strings.HasPrefix(body, "X") {
		base, body = 16, body[1:]
	}
	if body == "" {
		return "", false
	}
	value, err := strconv.ParseInt(body, base, 32)
	if err != nil || value <= 0 || value > 0x10FFFF || (value >= 0xD800 && value <= 0xDFFF) {
		return "", false
	}
	return string(rune(value)), true
}

// literalFragments splits one literal span piece into writable literal pieces
// and non-writable entity-reference pieces. Every literal piece keeps the
// original byte interval of each of its decoded bytes through its own span.
func (s source) literalFragments(span sourceSpan, lo, hi int) ([]TextRun, bool) {
	piece := s.text[lo:hi]
	fragments := []TextRun{}
	start := 0
	for {
		amp := bytes.IndexByte(piece[start:], '&')
		if amp < 0 {
			break
		}
		amp += start
		if amp > start {
			text, offsets, ok := s.normalizeRun(span, lo+start, lo+amp)
			if !ok {
				return nil, false
			}
			fragments = append(fragments, TextRun{Text: text, Writable: true, offsets: offsets})
		}
		semi := bytes.IndexByte(piece[amp:], ';')
		if semi < 0 || semi > 64 {
			return nil, false
		}
		semi += amp
		decoded, ok := decodeEntityReference(string(piece[amp : semi+1]))
		if !ok {
			return nil, false
		}
		fragments = append(fragments, TextRun{Text: decoded})
		start = semi + 1
	}
	if start < len(piece) {
		text, offsets, ok := s.normalizeRun(span, lo+start, lo+len(piece))
		if !ok {
			return nil, false
		}
		fragments = append(fragments, TextRun{Text: text, Writable: true, offsets: offsets})
	}
	return fragments, true
}

// textFragments splits one direct character-data token into provenance pieces:
// literal source pieces are writable with their original byte intervals, while
// entity-generated, non-linear, CDATA and entity-reference pieces are not
// writable. The pieces must concatenate to the decoded token text, otherwise the
// whole token falls back to one non-writable fragment, so an expanded-stream
// offset or a decoded entity spelling can never be mistaken for author-written
// literal source.
func (s source) textFragments(before, end int, text string, cdata bool) []TextRun {
	if cdata {
		return []TextRun{{Text: text}}
	}
	fragments := []TextRun{}
	cursor := 0
	appendPieces := func(pieces []TextRun) bool {
		for _, piece := range pieces {
			if cursor+len(piece.Text) > len(text) || text[cursor:cursor+len(piece.Text)] != piece.Text {
				return false
			}
			fragments = append(fragments, piece)
			cursor += len(piece.Text)
		}
		return true
	}
	for index := sort.Search(len(s.spans), func(i int) bool { return s.spans[i].end > before }); index < len(s.spans) && s.spans[index].start < end; index++ {
		span := s.spans[index]
		lo, hi := before, end
		if span.start > lo {
			lo = span.start
		}
		if span.end < hi {
			hi = span.end
		}
		if lo >= hi {
			continue
		}
		if !span.linear || span.generated || s.uncertain(lo, hi) {
			if !appendPieces([]TextRun{{Text: string(s.text[lo:hi])}}) {
				return []TextRun{{Text: text}}
			}
			continue
		}
		pieces, ok := s.literalFragments(span, lo, hi)
		if !ok || !appendPieces(pieces) {
			return []TextRun{{Text: text}}
		}
	}
	if cursor != len(text) {
		return []TextRun{{Text: text}}
	}
	return fragments
}

func encode(text []byte, order binary.ByteOrder) []byte {
	if order == nil {
		return text
	}
	out := make([]byte, 0, 2*len(text))
	appendUnit := func(u uint16) { var b [2]byte; order.PutUint16(b[:], u); out = append(out, b[:]...) }
	for _, r := range string(text) {
		if r <= 0xffff {
			appendUnit(uint16(r))
		} else {
			a, b := utf16.EncodeRune(r)
			appendUnit(uint16(a))
			appendUnit(uint16(b))
		}
	}
	return out
}
