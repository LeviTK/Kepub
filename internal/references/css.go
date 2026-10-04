package references

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/bookpath"
)

// This lexer extracts literal, unescaped url() and @import values. It is not
// a CSS grammar validator: image-set strings, escapes, dynamic substitutions,
// future properties/functions and invalid declarations cannot imply no links.
func (b *builder) css(bp bookpath.BookPath, location, css string) {
	b.cover(bp, "css.grammar", "partial", "only literal unescaped url() and @import references are extracted; other CSS syntax is not proven")
	if location != "stylesheet" {
		b.cover(bp, "inline-style", "partial", "inline CSS uses the same partial grammar coverage as stylesheets")
	}
	status := "complete"
	if !utf8.ValidString(css) || strings.ContainsRune(css, '\x00') {
		status = "blocked"
	}
	for i := 0; i < len(css) && status != "blocked"; {
		start := i
		if strings.HasPrefix(css[i:], "/*") {
			end := strings.Index(css[i+2:], "*/")
			if end < 0 {
				status = "partial"
				break
			}
			i += end + 4
			continue
		}
		if css[i] == '\\' {
			status = "partial"
			// Escaped identifiers can hide function names. Keep walking only to
			// report other literal candidates; coverage cannot become complete.
			i += min(2, len(css)-i)
			continue
		}
		if css[i] == '\'' || css[i] == '"' {
			_, next, ok := cssString(css, i)
			if !ok {
				status = "partial"
			}
			i = next
			continue
		}
		isImport := css[i] == '@'
		if isImport {
			i++
		}
		if i < len(css) && cssIdent(css[i]) {
			j := i
			for j < len(css) && cssIdent(css[j]) {
				j++
			}
			word := strings.ToLower(css[i:j])
			i = j
			syntax := "css.url"
			if isImport && word == "import" {
				syntax = "css.import"
				var ok bool
				i, ok = cssSpace(css, i)
				if !ok || i == len(css) {
					status = "partial"
					break
				}
				if css[i] == '\'' || css[i] == '"' {
					value, next, ok := cssString(css, i)
					if ok {
						b.add(bp, fmt.Sprintf("%s/byte[%d]", location, start), syntax, value)
					} else {
						status = "partial"
					}
					i = next
					continue
				}
				if len(css)-i < 4 || !strings.EqualFold(css[i:i+4], "url(") {
					status = "partial"
					continue
				}
				i += 3
			} else if isImport || word != "url" || i == len(css) || css[i] != '(' {
				continue
			}
			value, next, ok := cssURL(css, i)
			if ok {
				b.add(bp, fmt.Sprintf("%s/byte[%d]", location, start), syntax, value)
			} else {
				status = "partial"
			}
			i = next
			continue
		}
		if i == start {
			i++
		}
	}
	reason := "literal unescaped forms only; full CSS grammar remains partial"
	if status != "complete" {
		reason = "unsupported encoding, escape, or malformed CSS token may conceal references"
	}
	b.cover(bp, "css.url", status, reason)
	b.cover(bp, "css.import", status, reason)
}

func cssIdent(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c >= 0x80
}

func cssWhitespace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f'
}

func cssSpace(s string, i int) (int, bool) {
	for i < len(s) {
		if cssWhitespace(s[i]) {
			i++
		} else if strings.HasPrefix(s[i:], "/*") {
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				return len(s), false
			}
			i += end + 4
		} else {
			break
		}
	}
	return i, true
}

func cssString(s string, i int) (string, int, bool) {
	quote, start := s[i], i+1
	ok := true
	for i++; i < len(s); i++ {
		if s[i] == quote {
			return s[start:i], i + 1, ok
		}
		if s[i] == '\\' {
			ok = false
			i++
		} else if s[i] == '\r' || s[i] == '\n' || s[i] == '\f' {
			return "", i + 1, false
		}
	}
	return "", len(s), false
}

func cssURL(s string, open int) (string, int, bool) {
	i, ok := cssSpace(s, open+1)
	if !ok || i == len(s) {
		return "", i, false
	}
	if s[i] == '\'' || s[i] == '"' {
		value, next, valid := cssString(s, i)
		i, ok = cssSpace(s, next)
		if i < len(s) && s[i] == ')' {
			return value, i + 1, valid && ok
		}
		return "", i, false
	}
	start := i
	for i < len(s) && s[i] != ')' {
		if s[i] == '\\' || s[i] == '(' || s[i] == '\'' || s[i] == '"' || strings.HasPrefix(s[i:], "/*") {
			ok = false
		}
		i++
	}
	if i == len(s) {
		return "", i, false
	}
	value := strings.TrimSpace(s[start:i])
	for j := 0; j < len(value); j++ {
		if cssWhitespace(value[j]) || value[j] < 0x20 {
			ok = false
		}
	}
	return value, i + 1, ok
}
