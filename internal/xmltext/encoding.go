package xmltext

import (
	"bytes"
	"encoding/binary"
	"regexp"
	"sort"
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
