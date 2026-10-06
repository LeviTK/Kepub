package xmltext

import (
	"bytes"
	"encoding/xml"
	"sort"
	"strconv"
	"strings"

	"github.com/LeviTK/Kepub/internal/fault"
)

// Segments describe the expanded stream without manufacturing writable spans
// for entity text. Direct offsets are decoded offsets in the original source;
// generated segments are anchored to the enclosing original reference.
type sourceSpan struct {
	start, end, from, to int
	linear, generated    bool
}

type expansion struct {
	input         source
	text          []byte
	spans         []sourceSpan
	uncertainties []sourceSpan
	dtd           *internalSubset
	stack         []string
	sawRoot       bool
	sawDoctype    bool
	active        map[string]bool
	streamBytes   int
	crPositions   []int
	inParameter   bool
}

func expand(s source) (source, *internalSubset, error) {
	decl := declaration.Find(s.text)
	x := &expansion{input: s, dtd: newSubset(bytes.Contains(decl, []byte("yes"))), active: map[string]bool{}}
	for _, r := range string(s.text) {
		if !xmlChar(r) {
			return source{}, nil, malformed("invalid XML character")
		}
	}
	if err := x.content(string(s.text), 0, 0, 0, 0); err != nil {
		return source{}, nil, err
	}
	if len(x.stack) != 0 {
		return source{}, nil, malformed("unclosed XML element")
	}
	original := s
	s.text, s.original, s.spans, s.uncertainties = x.text, &original, x.spans, x.uncertainties
	s.crPositions = x.crPositions
	return s, x.dtd, nil
}

func (x *expansion) append(text string, from, to int, linear, generated bool, size int) error {
	if x.streamBytes+size > DecodedLimit {
		return fault.New(1, "XML_LIMIT", "expanded XML stream exceeds 16 MiB")
	}
	x.streamBytes += size
	if len(text) == 0 {
		return nil
	}
	span := sourceSpan{len(x.text), len(x.text) + len(text), from, to, linear, generated}
	if len(x.spans) > 0 {
		last := &x.spans[len(x.spans)-1]
		if linear && last.linear && last.generated == generated && last.to == from {
			last.end, last.to = span.end, span.to
		} else if !linear && !last.linear && last.generated == generated && last.from == from && last.to == to {
			last.end = span.end
		} else {
			x.spans = append(x.spans, span)
		}
	} else {
		x.spans = append(x.spans, span)
	}
	x.text = append(x.text, text...)
	return nil
}

// A writable boundary must come from literal source syntax, not an entity
// anchor or a synthesized default. Ignore zero-length replacement markers:
// they may be adjacent to a real tag but cannot establish its provenance.
func (s source) literalRange(start, end int) (int, int, bool) {
	i := sort.Search(len(s.spans), func(i int) bool { return s.spans[i].end > start })
	if end <= start || i == len(s.spans) {
		return 0, 0, false
	}
	span := s.spans[i]
	if span.start > start || span.end < end || !span.linear || span.generated {
		return 0, 0, false
	}
	return s.original.offset(span.from + start - span.start), s.original.offset(span.from + end - span.start), true
}

// Character references are interpreted when constructing EntityValue. A CR
// produced there is not another physical line ending. Keep CDATA token shape
// through RawToken, restoring only indexed markers (literal U+E000 is untouched).
func (s source) restoreCDATA(value []byte, before int) []byte {
	base := before + len("<![CDATA[")
	i := sort.SearchInts(s.crPositions, base)
	if i == len(s.crPositions) || s.crPositions[i] >= base+len(value) {
		return value
	}
	out, cursor := []byte{}, 0
	for ; i < len(s.crPositions) && s.crPositions[i] < base+len(value); i++ {
		pos := s.crPositions[i] - base
		out = append(out, value[cursor:pos]...)
		out = append(out, '\r')
		cursor = pos + len("\ue000")
	}
	return append(out, value[cursor:]...)
}

func (s source) uncertain(start, end int) bool {
	i := sort.Search(len(s.uncertainties), func(i int) bool { return s.uncertainties[i].end > start })
	return i < len(s.uncertainties) && s.uncertainties[i].start < end
}

func (x *expansion) unknown(name, kind string, from, to, depth int, declared bool) error {
	if !declared && (x.dtd.standalone && !x.inParameter || !x.dtd.externalSubset && !x.dtd.hasPE) {
		return malformed("undeclared XML entity %q", name)
	}
	origin := "reference"
	if depth != 0 {
		origin = "expansion"
	}
	if err := x.dtd.charge("&"+name+";", depth+1); err != nil {
		return err
	}
	x.dtd.unresolved = append(x.dtd.unresolved, Unresolved{name, kind, x.input.offset(from), x.input.offset(to), origin})
	return nil
}

// Attribute normalization distinguishes literal whitespace from character
// references. Tokenized types subsequently trim/collapse only XML #x20.
func (x *expansion) attribute(value string, base, anchorStart, anchorEnd, depth int) (string, bool, error) {
	var out strings.Builder
	generated := depth != 0
	for i := 0; i < len(value); {
		switch value[i] {
		case '<':
			return "", false, malformed("literal '<' in attribute replacement text")
		case '\t', '\n', '\r':
			out.WriteByte(' ')
			if value[i] == '\r' && i+1 < len(value) && value[i+1] == '\n' {
				i++
			}
			i++
		case '&':
			name, end, err := reference(value, i, false)
			if err != nil {
				return "", false, err
			}
			lo, hi := anchorStart, anchorEnd
			if depth == 0 {
				lo, hi = base+i, base+end
			}
			if strings.HasPrefix(name, "#") || predefined(name) {
				var decoded string
				if strings.HasPrefix(name, "#") {
					base, digits := 10, name[1:]
					if strings.HasPrefix(digits, "x") {
						base, digits = 16, digits[1:]
					}
					n, _ := strconv.ParseUint(digits, base, 32)
					decoded = string(rune(n))
				} else {
					decoded = map[string]string{"lt": "<", "gt": ">", "amp": "&", "apos": "'", "quot": "\""}[name]
					if err := x.dtd.charge(decoded, depth+1); err != nil {
						return "", false, err
					}
				}
				out.WriteString(decoded)
			} else if e, ok := x.dtd.general[name]; !ok || e.external {
				if ok && e.notation != "" {
					return "", false, malformed("unparsed entity reference in attribute")
				}
				if ok && e.external {
					return "", false, malformed("external entity reference in attribute")
				}
				if err := x.unknown(name, "general", lo, hi, depth, false); err != nil {
					return "", false, err
				}
				out.WriteString(value[i:end])
				generated = true
			} else {
				if x.dtd.standalone && e.inParameter && !x.inParameter {
					return "", false, malformed("standalone general entity must be declared outside parameter entities")
				}
				if x.active[name] {
					return "", false, malformed("recursive general entity")
				}
				if err := x.dtd.charge("", depth+1); err != nil {
					return "", false, err
				}
				x.active[name] = true
				v, _, err := x.attribute(e.value, 0, lo, hi, depth+1)
				delete(x.active, name)
				if err != nil {
					return "", false, err
				}
				if err := x.dtd.chargeWork(len(v)); err != nil {
					return "", false, err
				}
				out.WriteString(v)
				generated = true
			}
			i = end
		default:
			out.WriteByte(value[i])
			i++
		}
		if out.Len() > DecodedLimit {
			return "", false, fault.New(1, "XML_LIMIT", "expanded attribute exceeds 16 MiB")
		}
	}
	return out.String(), generated, nil
}

func collapse(value string) string {
	return strings.Join(strings.FieldsFunc(value, func(r rune) bool { return r == ' ' }), " ")
}

func escaped(value string) string {
	var b bytes.Buffer
	// Values already passed XML character validation. Numeric escaping also
	// preserves character-reference whitespace through encoding/xml.
	_ = xml.EscapeText(&b, []byte(value))
	return b.String()
}

func (x *expansion) tag(text string, base, anchorStart, anchorEnd, depth int) (int, error) {
	l := dtdLex{text: text, pos: 1}
	name, err := l.name(false)
	if err != nil {
		return 0, err
	}
	appendPiece := func(v string, lo, hi int, linear, generated bool, size int) error {
		if depth != 0 {
			lo, hi, linear, generated = anchorStart, anchorEnd, false, true
		}
		return x.append(v, lo, hi, linear, generated, size)
	}
	if err := appendPiece(text[:l.pos], base, base+l.pos, true, false, l.pos); err != nil {
		return 0, err
	}
	seen := map[string]bool{}
	for {
		start := l.pos
		space := l.space()
		if strings.HasPrefix(text[l.pos:], ">") || strings.HasPrefix(text[l.pos:], "/>") {
			if err := appendPiece(text[start:l.pos], base+start, base+l.pos, true, false, l.pos-start); err != nil {
				return 0, err
			}
			break
		}
		if !space {
			return 0, malformed("XML attribute requires whitespace")
		}
		attr, err := l.name(false)
		if err != nil || seen[attr] {
			return 0, malformed("invalid or duplicate XML attribute")
		}
		seen[attr] = true
		l.space()
		if !l.take("=") {
			return 0, malformed("XML attribute requires '='")
		}
		l.space()
		valueStart := l.pos + 1
		value, err := l.literal()
		if err != nil {
			return 0, err
		}
		valueEnd := l.pos - 1
		lo, hi := base+valueStart, base+valueEnd
		if depth != 0 {
			lo, hi = anchorStart, anchorEnd
		}
		unresolvedBefore := len(x.dtd.unresolved)
		v, generated, err := x.attribute(value, base+valueStart, lo, hi, depth)
		if err != nil {
			return 0, err
		}
		if a, ok := x.dtd.attributeIndex[name][attr]; ok && a.tokenized {
			v = collapse(v)
		}
		if err := appendPiece(text[start:valueStart], base+start, base+valueStart, true, false, valueStart-start); err != nil {
			return 0, err
		}
		outputStart := len(x.text)
		if err := appendPiece(escaped(v), base+valueStart, base+valueEnd, false, generated, len(v)); err != nil {
			return 0, err
		}
		if len(x.dtd.unresolved) != unresolvedBefore {
			x.uncertainties = append(x.uncertainties, sourceSpan{start: outputStart, end: len(x.text)})
		}
		if err := appendPiece(text[valueEnd:l.pos], base+valueEnd, base+l.pos, true, false, l.pos-valueEnd); err != nil {
			return 0, err
		}
	}
	for _, a := range x.dtd.attributes[name] {
		if !a.hasDefault || seen[a.name] {
			continue
		}
		// A default has no independent writable interval in the start tag.
		v := a.value
		outputStart := len(x.text)
		if err := appendPiece(" "+a.name+"=\""+escaped(v)+"\"", base+l.pos, base+l.pos, false, true, len(a.name)+4+len(v)); err != nil {
			return 0, err
		}
		if a.uncertain {
			x.uncertainties = append(x.uncertainties, sourceSpan{start: outputStart, end: len(x.text)})
		}
	}
	start := l.pos
	selfClosing := l.take("/>")
	if !selfClosing && !l.take(">") {
		return 0, malformed("invalid XML start tag")
	}
	if !selfClosing {
		if len(x.stack) >= 128 {
			return 0, fault.New(1, "XML_LIMIT", "XML depth limit")
		}
		x.stack = append(x.stack, name)
	}
	x.sawRoot = true
	return l.pos, appendPiece(text[start:l.pos], base+start, base+l.pos, true, false, l.pos-start)
}

func (x *expansion) content(text string, base, anchorStart, anchorEnd, depth int) error {
	entryDepth := len(x.stack)
	appendPiece := func(v string, lo, hi int, generated bool, size int) error {
		linear := len(v) == hi-lo
		if depth != 0 {
			lo, hi, linear, generated = anchorStart, anchorEnd, false, true
		}
		return x.append(v, lo, hi, linear, generated, size)
	}
	for i := 0; i < len(text); {
		start := i
		switch {
		case strings.HasPrefix(text[i:], "<!DOCTYPE"):
			if depth != 0 || x.sawRoot || x.sawDoctype {
				return malformed("duplicate or misplaced DOCTYPE")
			}
			n, err := x.dtd.doctype(text[i:], base+i, x.input)
			if err != nil {
				return err
			}
			x.sawDoctype = true
			i += n
			// Count expanded PE declarations while retaining the original source.
			if err := appendPiece(strings.Repeat(" ", n+x.dtd.addedDTD), base+start, base+i, false, n+x.dtd.addedDTD); err != nil {
				return err
			}
		case strings.HasPrefix(text[i:], "<!--"), strings.HasPrefix(text[i:], "<![CDATA["), strings.HasPrefix(text[i:], "<?"):
			open, close := "<!--", "-->"
			if strings.HasPrefix(text[i:], "<![CDATA[") {
				open, close = "<![CDATA[", "]]>"
				if len(x.stack) == 0 {
					return malformed("CDATA outside root")
				}
			} else if strings.HasPrefix(text[i:], "<?") {
				open, close = "<?", "?>"
			}
			end := strings.Index(text[i+len(open):], close)
			if end < 0 {
				return malformed("unclosed XML comment/CDATA/processing instruction")
			}
			i += len(open) + end + len(close)
			piece := text[start:i]
			if depth != 0 && open == "<![CDATA[" && strings.Contains(piece, "\r") {
				var b strings.Builder
				for _, r := range piece {
					if r == '\r' {
						x.crPositions = append(x.crPositions, len(x.text)+b.Len())
						b.WriteRune('\ue000')
					} else {
						b.WriteRune(r)
					}
				}
				piece = b.String()
			}
			if err := appendPiece(piece, base+start, base+i, false, i-start); err != nil {
				return err
			}
		case strings.HasPrefix(text[i:], "</"):
			l := dtdLex{text: text[i:], pos: 2}
			name, err := l.name(false)
			if err != nil {
				return err
			}
			l.space()
			if !l.take(">") || len(x.stack) == 0 || x.stack[len(x.stack)-1] != name || depth != 0 && len(x.stack) <= entryDepth {
				return malformed("mismatched XML end tag or entity boundary")
			}
			x.stack = x.stack[:len(x.stack)-1]
			i += l.pos
			if err := appendPiece(text[start:i], base+start, base+i, false, i-start); err != nil {
				return err
			}
		case text[i] == '<':
			n, err := x.tag(text[i:], base+i, anchorStart, anchorEnd, depth)
			if err != nil {
				return err
			}
			i += n
		case text[i] == '&':
			// document/Misc permits lexical S, comments and PIs only. Check
			// before decoding or an empty/whitespace entity could disappear.
			if len(x.stack) == 0 {
				return malformed("entity or character reference outside root")
			}
			name, end, err := reference(text, i, false)
			if err != nil {
				return err
			}
			lo, hi := base+i, base+end
			if depth != 0 {
				lo, hi = anchorStart, anchorEnd
			}
			if strings.HasPrefix(name, "#") || predefined(name) {
				size := 1
				if strings.HasPrefix(name, "#") {
					base, digits := 10, name[1:]
					if strings.HasPrefix(digits, "x") {
						base, digits = 16, digits[1:]
					}
					r, _ := strconv.ParseUint(digits, base, 32)
					size = len(string(rune(r)))
				}
				if predefined(name) {
					if err := x.dtd.charge("x", depth+1); err != nil {
						return err
					}
				}
				if err := appendPiece(text[i:end], base+i, base+end, false, size); err != nil {
					return err
				}
			} else if e, ok := x.dtd.general[name]; !ok || e.external {
				if ok && e.notation != "" {
					return malformed("unparsed entity used as parsed content")
				}
				if err := x.unknown(name, "general", lo, hi, depth, ok); err != nil {
					return err
				}
				outputStart := len(x.text)
				if err := appendPiece("&#38;"+name+";", base+i, base+end, true, end-i); err != nil {
					return err
				}
				x.uncertainties = append(x.uncertainties, sourceSpan{start: outputStart, end: len(x.text)})
			} else {
				if x.dtd.standalone && e.inParameter {
					return malformed("standalone general entity must be declared outside parameter entities")
				}
				if x.active[name] {
					return malformed("recursive general entity")
				}
				if err := x.dtd.charge("", depth+1); err != nil {
					return err
				}
				x.active[name] = true
				if e.value == "" {
					x.spans = append(x.spans, sourceSpan{start: len(x.text), end: len(x.text), from: lo, to: hi, generated: true})
				}
				producedStart := x.streamBytes
				err := x.content(e.value, 0, lo, hi, depth+1)
				delete(x.active, name)
				if err != nil {
					return err
				}
				if err := x.dtd.chargeWork(x.streamBytes - producedStart); err != nil {
					return err
				}
			}
			i = end
		default:
			for i < len(text) && text[i] != '<' && text[i] != '&' {
				i++
			}
			piece := text[start:i]
			if len(x.stack) == 0 && strings.Trim(piece, " \t\r\n") != "" {
				return malformed("non-S text outside root")
			}
			if depth != 0 {
				piece = strings.ReplaceAll(piece, "\r", "&#xD;")
			}
			if err := appendPiece(piece, base+start, base+i, false, i-start); err != nil {
				return err
			}
		}
	}
	if depth != 0 && len(x.stack) != entryDepth {
		return malformed("entity replacement is not balanced XML content")
	}
	return nil
}
