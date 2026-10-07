package xmltext

import (
	"bytes"
	"encoding/xml"
	"strings"
)

// encoding/xml still validates Names using the pre-Fifth-Edition Appendix B
// table. Validate lexical names with our XML 1.0 grammar, then mask non-ASCII
// name bytes only for RawToken. Lengths/offsets, values and source bytes stay
// unchanged; names are restored before namespace/duplicate/structure checks.
func tokenStream(text []byte) ([]byte, error) {
	out := text
	s := string(text)
	for i := 0; i < len(s); {
		n := strings.IndexByte(s[i:], '<')
		if n < 0 {
			break
		}
		i += n
		if strings.HasPrefix(s[i:], "<!--") || strings.HasPrefix(s[i:], "<![CDATA[") {
			open, close := 4, "-->"
			if strings.HasPrefix(s[i:], "<![CDATA[") {
				open, close = 9, "]]>"
			}
			n := strings.Index(s[i+open:], close)
			if n < 0 {
				return nil, malformed("unclosed XML comment/CDATA")
			}
			i += open + n + len(close)
			continue
		}
		end, err := lexicalNames(s, i, func(from, to int) {
			for j := from; j < to; j++ {
				if text[j] >= 0x80 {
					if len(out) != 0 && &out[0] == &text[0] {
						out = bytes.Clone(text)
					}
					out[j] = 'x'
				}
			}
		})
		if err != nil {
			return nil, err
		}
		i = end
	}
	return out, nil
}

// Visit element/attribute or PI names, not names appearing in attribute values.
func lexicalNames(text string, start int, visit func(int, int)) (int, error) {
	l := dtdLex{text: text, pos: start + 1}
	pi, endTag := l.take("?"), false
	if !pi {
		endTag = l.take("/")
	}
	name := func() error {
		from := l.pos
		var err error
		if pi {
			_, err = l.ncName()
		} else {
			_, err = l.name(false)
		}
		if err != nil {
			return err
		}
		visit(from, l.pos)
		return nil
	}
	if err := name(); err != nil {
		return 0, err
	}
	if pi {
		n := strings.Index(text[l.pos:], "?>")
		if n < 0 {
			return 0, malformed("unclosed XML processing instruction")
		}
		return l.pos + n + 2, nil
	}
	for {
		space := l.space()
		if l.take(">") || !endTag && l.take("/>") {
			return l.pos, nil
		}
		if endTag || !space {
			return 0, malformed("invalid XML tag")
		}
		if err := name(); err != nil {
			return 0, err
		}
		l.space()
		if !l.take("=") {
			return 0, malformed("XML attribute requires '='")
		}
		l.space()
		if _, err := l.literal(); err != nil {
			return 0, err
		}
	}
}

type lexicalName struct {
	Name  xml.Name
	Start int
}

// ValidNCName reports whether s is a legal XML Name that contains no colon, as
// required for an unprefixed element or attribute name.
func ValidNCName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == ':' || !nameChar(r) || i == 0 && !nameStart(r) {
			return false
		}
	}
	return true
}

// attrSpan records expanded-stream intervals of one lexical attribute in a start
// tag: the whole attribute including preceding whitespace, and the value between
// the quotes.
type attrSpan struct {
	start, end           int
	nameStart, nameEnd   int
	valueStart, valueEnd int
}

// lexAttributes re-lexes one serialized start tag. Callers map the positions to
// original bytes and reject entity-generated pieces.
func lexAttributes(text string) ([]attrSpan, error) {
	l := dtdLex{text: text, pos: 1}
	if _, err := l.name(false); err != nil {
		return nil, err
	}
	var out []attrSpan
	for {
		start := l.pos
		space := l.space()
		if l.take(">") || l.take("/>") {
			return out, nil
		}
		if !space {
			return nil, malformed("invalid XML tag")
		}
		nameStart := l.pos
		if _, err := l.name(false); err != nil {
			return nil, err
		}
		nameEnd := l.pos
		l.space()
		if !l.take("=") {
			return nil, malformed("XML attribute requires '='")
		}
		l.space()
		if l.pos == len(l.text) || l.text[l.pos] != '\'' && l.text[l.pos] != '"' {
			return nil, malformed("expected quoted XML literal")
		}
		valueStart := l.pos + 1
		if _, err := l.literal(); err != nil {
			return nil, err
		}
		valueEnd := l.pos - 1
		out = append(out, attrSpan{start: start, end: l.pos, nameStart: nameStart, nameEnd: nameEnd, valueStart: valueStart, valueEnd: valueEnd})
	}
}

func lexicalTokenNames(text []byte) ([]lexicalName, error) {
	s := string(text)
	var names []lexicalName
	var nameErr error
	_, err := lexicalNames(s, 0, func(from, to int) {
		n := xml.Name{Local: s[from:to]}
		if prefix, local, ok := strings.Cut(n.Local, ":"); ok {
			n.Space, n.Local = prefix, local
			if prefix == "" || local == "" {
				nameErr = malformed("empty qualified-name component")
			}
		}
		for _, part := range []string{n.Local, n.Space} {
			if part == "" {
				continue
			}
			for i, r := range part {
				if r == ':' || !nameChar(r) || i == 0 && !nameStart(r) {
					nameErr = malformed("invalid XML qualified name")
				}
			}
		}
		names = append(names, lexicalName{n, from})
	})
	if nameErr != nil {
		return nil, nameErr
	}
	return names, err
}
