package xmltext

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/fault"
)

// A non-validating internal subset processor. No resolver, reader, catalog or
// filesystem capability is supplied to it. Validity constraints (content model,
// duplicate declarations, #FIXED equality) are not well-formedness constraints.
type entityDecl struct {
	value, notation string
	external        bool
	inParameter     bool
}

type attributeDecl struct {
	name, value string
	tokenized   bool
	hasDefault  bool
	uncertain   bool
}

type Notation struct {
	Name     string `json:"name"`
	PublicID string `json:"publicId"`
	SystemID string `json:"systemId"`
}

type Unresolved struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	StartByte int    `json:"startByte"`
	EndByte   int    `json:"endByte"`
	Origin    string `json:"origin"`
}

type internalSubset struct {
	general, parameter     map[string]entityDecl
	attributes             map[string][]attributeDecl
	attributeIndex         map[string]map[string]attributeDecl
	notations              []Notation
	publicID, systemID     string
	externalSubset         bool
	externalEntity         bool
	standalone, hasPE      bool
	unreadPE               bool
	unresolved             []Unresolved
	declarations           int
	replacements, work     int
	producedDTD            int
	addedDTD               int
	processingInstructions []string
}

func newSubset(standalone bool) *internalSubset {
	return &internalSubset{general: map[string]entityDecl{}, parameter: map[string]entityDecl{}, attributes: map[string][]attributeDecl{}, attributeIndex: map[string]map[string]attributeDecl{}, standalone: standalone}
}

func malformed(format string, args ...any) error {
	return fault.New(1, "XML_NOT_WELL_FORMED", format, args...)
}

func xmlChar(r rune) bool {
	return r == 9 || r == 10 || r == 13 || r >= 0x20 && r <= 0xd7ff || r >= 0xe000 && r <= 0xfffd || r >= 0x10000 && r <= 0x10ffff
}

func nameStart(r rune) bool {
	return r == ':' || r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' ||
		r >= 0xc0 && r <= 0xd6 || r >= 0xd8 && r <= 0xf6 || r >= 0xf8 && r <= 0x2ff ||
		r >= 0x370 && r <= 0x37d || r >= 0x37f && r <= 0x1fff || r >= 0x200c && r <= 0x200d ||
		r >= 0x2070 && r <= 0x218f || r >= 0x2c00 && r <= 0x2fef || r >= 0x3001 && r <= 0xd7ff ||
		r >= 0xf900 && r <= 0xfdcf || r >= 0xfdf0 && r <= 0xfffd || r >= 0x10000 && r <= 0xeffff
}

func nameChar(r rune) bool {
	return nameStart(r) || r == '-' || r == '.' || r >= '0' && r <= '9' || r == 0xb7 || r >= 0x300 && r <= 0x36f || r >= 0x203f && r <= 0x2040
}

type dtdLex struct {
	text string
	pos  int
}

func (l *dtdLex) take(s string) bool {
	if strings.HasPrefix(l.text[l.pos:], s) {
		l.pos += len(s)
		return true
	}
	return false
}

func (l *dtdLex) space() bool {
	start := l.pos
	for l.pos < len(l.text) && strings.ContainsRune(" \t\r\n", rune(l.text[l.pos])) {
		l.pos++
	}
	return l.pos != start
}

func (l *dtdLex) requireSpace() error {
	if !l.space() {
		return malformed("XML declaration requires whitespace")
	}
	return nil
}

func (l *dtdLex) name(token bool) (string, error) {
	start := l.pos
	for l.pos < len(l.text) {
		r, n := utf8.DecodeRuneInString(l.text[l.pos:])
		if !nameChar(r) || l.pos == start && !token && !nameStart(r) {
			break
		}
		l.pos += n
	}
	if l.pos == start {
		return "", malformed("expected XML name")
	}
	name := l.text[start:l.pos]
	if !token {
		parts := strings.Split(name, ":")
		if len(parts) > 2 {
			return "", malformed("invalid XML qualified name")
		}
		for _, part := range parts {
			r, _ := utf8.DecodeRuneInString(part)
			if part == "" || !nameStart(r) || r == ':' {
				return "", malformed("invalid XML qualified name")
			}
		}
	}
	return name, nil
}

// Namespaces §7 requires entity, notation and PI names to be NCNames;
// element/attribute names remain QNames and NMTOKEN values may contain colons.
func (l *dtdLex) ncName() (string, error) {
	name, err := l.name(false)
	if err == nil && strings.Contains(name, ":") {
		err = malformed("colon in XML NCName")
	}
	return name, err
}

func (l *dtdLex) literal() (string, error) {
	if l.pos == len(l.text) || l.text[l.pos] != '\'' && l.text[l.pos] != '"' {
		return "", malformed("expected quoted XML literal")
	}
	q := l.text[l.pos]
	l.pos++
	start := l.pos
	for l.pos < len(l.text) && l.text[l.pos] != q {
		l.pos++
	}
	if l.pos == len(l.text) {
		return "", malformed("unclosed XML literal")
	}
	v := l.text[start:l.pos]
	l.pos++
	return v, nil
}

func publicLiteral(v string) error {
	for _, r := range v {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune(" \r\n-'()+,./:=?;!*#@$_%", r)) {
			return malformed("invalid PubidLiteral")
		}
	}
	return nil
}

func (l *dtdLex) externalID(notation bool) (string, string, error) {
	public := l.take("PUBLIC")
	if !public && !l.take("SYSTEM") {
		return "", "", malformed("expected SYSTEM or PUBLIC")
	}
	if err := l.requireSpace(); err != nil {
		return "", "", err
	}
	a, err := l.literal()
	if err != nil {
		return "", "", err
	}
	if !public {
		return "", a, nil
	}
	if err := publicLiteral(a); err != nil {
		return "", "", err
	}
	space := l.space()
	if notation && (l.pos == len(l.text) || l.text[l.pos] != '\'' && l.text[l.pos] != '"') {
		return a, "", nil
	}
	if !space {
		return "", "", malformed("PUBLIC external identifier requires system literal")
	}
	b, err := l.literal()
	return a, b, err
}

func (d *internalSubset) charge(value string, depth int) error {
	if depth > 16 {
		return fault.New(1, "XML_LIMIT", "XML entity nesting limit")
	}
	d.replacements++
	if d.replacements > 100000 {
		return fault.New(1, "XML_LIMIT", "XML entity replacement/work limit")
	}
	return d.chargeWork(len(value))
}

func (d *internalSubset) chargeWork(n int) error {
	d.work += n
	if d.work > DecodedLimit {
		return fault.New(1, "XML_LIMIT", "XML entity expansion work exceeds 16 MiB")
	}
	return nil
}

func reference(text string, pos int, parameter bool) (name string, end int, err error) {
	l := dtdLex{text: text, pos: pos + 1}
	if !parameter && l.take("#") {
		start := l.pos
		base := 10
		if l.take("x") {
			base, start = 16, l.pos
		}
		for l.pos < len(text) && (text[l.pos] >= '0' && text[l.pos] <= '9' || base == 16 && (text[l.pos] >= 'a' && text[l.pos] <= 'f' || text[l.pos] >= 'A' && text[l.pos] <= 'F')) {
			l.pos++
		}
		n, e := strconv.ParseUint(text[start:l.pos], base, 32)
		if e != nil || !xmlChar(rune(n)) || !l.take(";") {
			return "", 0, malformed("invalid XML character reference")
		}
		return text[pos+1 : l.pos-1], l.pos, nil
	}
	name, err = l.ncName()
	if err != nil || !l.take(";") {
		return "", 0, malformed("invalid XML entity reference")
	}
	return name, l.pos, nil
}

func constructEntity(value string) (string, error) {
	var out strings.Builder
	for i := 0; i < len(value); {
		switch value[i] {
		case '%':
			return "", malformed("parameter entity reference inside internal markup declaration")
		case '&':
			name, end, err := reference(value, i, false)
			if err != nil {
				return "", err
			}
			if strings.HasPrefix(name, "#") {
				base, digits := 10, name[1:]
				if strings.HasPrefix(digits, "x") {
					base, digits = 16, digits[1:]
				}
				n, _ := strconv.ParseUint(digits, base, 32)
				out.WriteRune(rune(n))
			} else {
				// General references are bypassed while constructing replacement
				// text. In particular &lt; must not become markup at this stage.
				out.WriteString(value[i:end])
			}
			i = end
		case '\r':
			out.WriteByte('\n')
			i++
			if i < len(value) && value[i] == '\n' {
				i++
			}
		default:
			out.WriteByte(value[i])
			i++
		}
	}
	return out.String(), nil
}

// Parse one content model, without enforcing validity against instance content.
func (l *dtdLex) model(depth int) error {
	if depth > 128 {
		return fault.New(1, "XML_LIMIT", "DTD content model nesting limit")
	}
	if !l.take("(") {
		return malformed("expected ELEMENT content model")
	}
	l.space()
	if l.take("#PCDATA") {
		more := false
		for {
			l.space()
			if !l.take("|") {
				break
			}
			more = true
			l.space()
			if _, err := l.name(false); err != nil {
				return err
			}
		}
		if !l.take(")") {
			return malformed("invalid mixed content model")
		}
		star := l.take("*")
		if more && !star {
			return malformed("mixed content names require repetition")
		}
		return nil
	}
	separator := ""
	for {
		if strings.HasPrefix(l.text[l.pos:], "(") {
			if err := l.model(depth + 1); err != nil {
				return err
			}
		} else if _, err := l.name(false); err != nil {
			return err
		} else {
			if !l.take("?") && !l.take("*") {
				l.take("+")
			}
		}
		l.space()
		if l.take(")") {
			break
		}
		if l.pos == len(l.text) || l.text[l.pos] != '|' && l.text[l.pos] != ',' {
			return malformed("invalid ELEMENT content model separator")
		}
		s := l.text[l.pos : l.pos+1]
		if separator != "" && separator != s {
			return malformed("mixed ELEMENT content model separators")
		}
		separator = s
		l.pos++
		l.space()
	}
	if !l.take("?") && !l.take("*") {
		l.take("+")
	}
	return nil
}

func (l *dtdLex) enumeration(names bool) error {
	if !l.take("(") {
		return malformed("expected attribute enumeration")
	}
	for {
		l.space()
		var err error
		if names {
			_, err = l.ncName()
		} else {
			_, err = l.name(true)
		}
		if err != nil {
			return err
		}
		l.space()
		if l.take(")") {
			return nil
		}
		if !l.take("|") {
			return malformed("invalid attribute enumeration")
		}
	}
}

func (d *internalSubset) declaration(l *dtdLex, base, anchorStart, anchorEnd, depth int, s source) error {
	d.declarations++
	if d.declarations > 4096 {
		return fault.New(1, "XML_LIMIT", "XML DTD declaration limit")
	}
	register := !d.unreadPE || d.standalone
	switch {
	case l.take("<!ELEMENT"):
		if err := l.requireSpace(); err != nil {
			return err
		}
		if _, err := l.name(false); err != nil {
			return err
		}
		if err := l.requireSpace(); err != nil {
			return err
		}
		if !l.take("EMPTY") && !l.take("ANY") {
			if err := l.model(0); err != nil {
				return err
			}
		}
	case l.take("<!ATTLIST"):
		if err := l.requireSpace(); err != nil {
			return err
		}
		element, err := l.name(false)
		if err != nil {
			return err
		}
		for {
			spaced := l.space()
			if strings.HasPrefix(l.text[l.pos:], ">") {
				break
			}
			if !spaced {
				return malformed("ATTLIST requires whitespace")
			}
			name, err := l.name(false)
			if err != nil {
				return err
			}
			if err := l.requireSpace(); err != nil {
				return err
			}
			a := attributeDecl{name: name, tokenized: true}
			switch {
			case l.take("CDATA"):
				a.tokenized = false
			case l.take("NOTATION"):
				if err := l.requireSpace(); err != nil {
					return err
				}
				if err := l.enumeration(true); err != nil {
					return err
				}
			case strings.HasPrefix(l.text[l.pos:], "("):
				if err := l.enumeration(false); err != nil {
					return err
				}
			default:
				typeName, err := l.name(false)
				if err != nil || typeName != "ID" && typeName != "IDREF" && typeName != "IDREFS" && typeName != "ENTITY" && typeName != "ENTITIES" && typeName != "NMTOKEN" && typeName != "NMTOKENS" {
					return malformed("invalid attribute type")
				}
			}
			if err := l.requireSpace(); err != nil {
				return err
			}
			if !l.take("#REQUIRED") && !l.take("#IMPLIED") {
				if l.take("#FIXED") {
					if err := l.requireSpace(); err != nil {
						return err
					}
				}
				valueStart := l.pos + 1
				a.value, err = l.literal()
				if err != nil || strings.Contains(a.value, "<") {
					return malformed("invalid attribute default literal")
				}
				a.hasDefault = true
				for i := 0; i < len(a.value); i++ {
					if a.value[i] != '&' {
						continue
					}
					ref, end, err := reference(a.value, i, false)
					if err != nil {
						return err
					}
					if !strings.HasPrefix(ref, "#") && !predefined(ref) && register {
						if _, ok := d.general[ref]; !ok {
							return malformed("entity must precede attribute default reference")
						}
					}
					i = end - 1
				}
				if register {
					x := expansion{input: s, dtd: d, active: map[string]bool{}, inParameter: depth != 0}
					before := len(d.unresolved)
					a.value, _, err = x.attribute(a.value, base+valueStart, anchorStart, anchorEnd, depth)
					if err != nil {
						return err
					}
					a.uncertain = len(d.unresolved) != before
					if a.tokenized {
						a.value = collapse(a.value)
					}
				}
			}
			if register {
				if d.attributeIndex[element] == nil {
					d.attributeIndex[element] = map[string]attributeDecl{}
				}
				if _, exists := d.attributeIndex[element][name]; !exists {
					d.attributes[element] = append(d.attributes[element], a)
					d.attributeIndex[element][name] = a
				}
			}
		}
	case l.take("<!ENTITY"):
		if err := l.requireSpace(); err != nil {
			return err
		}
		parameter := l.take("%")
		if parameter {
			if err := l.requireSpace(); err != nil {
				return err
			}
		}
		name, err := l.ncName()
		if err != nil {
			return err
		}
		if err := l.requireSpace(); err != nil {
			return err
		}
		e := entityDecl{inParameter: depth != 0}
		if l.pos < len(l.text) && (l.text[l.pos] == '\'' || l.text[l.pos] == '"') {
			v, err := l.literal()
			if err != nil {
				return err
			}
			e.value, err = constructEntity(v)
			if err != nil {
				return err
			}
		} else {
			_, _, err := l.externalID(false)
			if err != nil {
				return err
			}
			e.external, d.externalEntity = true, true
			space := l.space()
			if !parameter && l.take("NDATA") {
				if !space {
					return malformed("NDATA requires whitespace")
				}
				if err := l.requireSpace(); err != nil {
					return err
				}
				e.notation, err = l.ncName()
				if err != nil {
					return err
				}
			}
		}
		if register {
			dest := d.general
			if parameter {
				dest = d.parameter
			}
			if _, exists := dest[name]; !exists {
				dest[name] = e
			}
		}
	case l.take("<!NOTATION"):
		if err := l.requireSpace(); err != nil {
			return err
		}
		name, err := l.ncName()
		if err != nil {
			return err
		}
		if err := l.requireSpace(); err != nil {
			return err
		}
		pub, sys, err := l.externalID(true)
		if err != nil {
			return err
		}
		found := false
		for _, n := range d.notations {
			found = found || n.Name == name
		}
		if !found {
			d.notations = append(d.notations, Notation{name, pub, sys})
		}
	default:
		return malformed("invalid internal DTD declaration")
	}
	l.space()
	if !l.take(">") {
		return malformed("unclosed or invalid DTD declaration")
	}
	return nil
}

func predefined(name string) bool {
	return name == "lt" || name == "gt" || name == "amp" || name == "apos" || name == "quot"
}

// base is a decoded document offset. For generated declarations the enclosing
// PE reference is the real source, not a fabricated offset into replacement text.
func (d *internalSubset) subset(text string, base, anchorStart, anchorEnd, depth int, active map[string]bool, s source) error {
	l := dtdLex{text: text}
	for l.pos < len(text) {
		spaceStart := l.pos
		l.space()
		d.producedDTD += l.pos - spaceStart
		if l.pos == len(text) {
			break
		}
		pieceStart := l.pos
		// Comments and PIs are also markupdecl productions in the subset;
		// bound their retained/application-visible data by the same limit.
		if strings.HasPrefix(l.text[l.pos:], "<!--") || strings.HasPrefix(l.text[l.pos:], "<?") {
			d.declarations++
			if d.declarations > 4096 {
				return fault.New(1, "XML_LIMIT", "XML DTD declaration limit")
			}
		}
		switch {
		case l.take("<!--"):
			end := strings.Index(l.text[l.pos:], "-->")
			if end < 0 || strings.Contains(l.text[l.pos:l.pos+end], "--") || strings.HasSuffix(l.text[l.pos:l.pos+end], "-") {
				return malformed("invalid DTD comment")
			}
			l.pos += end + 3
		case l.take("<?"):
			target, err := l.ncName()
			if err != nil || strings.EqualFold(target, "xml") {
				return malformed("invalid DTD processing instruction")
			}
			if !l.take("?>") {
				if err := l.requireSpace(); err != nil {
					return err
				}
				end := strings.Index(l.text[l.pos:], "?>")
				if end < 0 {
					return malformed("unclosed DTD processing instruction")
				}
				l.pos += end + 2
			}
			d.processingInstructions = append(d.processingInstructions, target)
		case l.text[l.pos] == '%':
			start := l.pos
			name, end, err := reference(text, start, true)
			if err != nil {
				return err
			}
			l.pos = end
			d.hasPE = true
			e, exists := d.parameter[name]
			if !exists || e.external {
				if d.standalone && !exists {
					return malformed("undeclared parameter entity in standalone document")
				}
				lo, hi, origin := anchorStart, anchorEnd, "expansion"
				if depth == 0 {
					lo, hi, origin = base+start, base+end, "reference"
				}
				if err := d.charge(text[start:end], depth+1); err != nil {
					return err
				}
				d.unresolved = append(d.unresolved, Unresolved{name, "parameter", s.offset(lo), s.offset(hi), origin})
				d.unreadPE = true
				d.producedDTD += end - start
				continue
			}
			if active[name] {
				return malformed("recursive parameter entity")
			}
			if err := d.charge("", depth+1); err != nil {
				return err
			}
			active[name] = true
			lo, hi := anchorStart, anchorEnd
			if depth == 0 {
				lo, hi = base+start, base+end
			}
			producedBefore := d.producedDTD
			err = d.subset(e.value, 0, lo, hi, depth+1, active, s)
			delete(active, name)
			if err != nil {
				return err
			}
			produced := d.producedDTD - producedBefore
			if err := d.chargeWork(produced); err != nil {
				return err
			}
			if depth == 0 {
				d.addedDTD += produced - (end - start)
			}
			continue
		default:
			if err := d.declaration(&l, base, anchorStart, anchorEnd, depth, s); err != nil {
				return err
			}
		}
		d.producedDTD += l.pos - pieceStart
	}
	return nil
}

func (d *internalSubset) doctype(text string, base int, s source) (int, error) {
	l := dtdLex{text: text}
	if !l.take("<!DOCTYPE") {
		return 0, malformed("invalid DOCTYPE")
	}
	if err := l.requireSpace(); err != nil {
		return 0, err
	}
	if _, err := l.name(false); err != nil {
		return 0, err
	}
	space := l.space()
	if strings.HasPrefix(text[l.pos:], "SYSTEM") || strings.HasPrefix(text[l.pos:], "PUBLIC") {
		if !space {
			return 0, malformed("DOCTYPE external identifier requires whitespace")
		}
		pub, sys, err := l.externalID(false)
		if err != nil {
			return 0, err
		}
		d.publicID, d.systemID, d.externalSubset = pub, sys, true
		l.space()
	}
	if l.take("[") {
		start := l.pos
		// Locate the internal-subset boundary outside literals/comments/PIs.
		for l.pos < len(text) && text[l.pos] != ']' {
			switch {
			case l.take("<!--"):
				i := strings.Index(text[l.pos:], "-->")
				if i < 0 {
					return 0, malformed("unclosed DTD comment")
				}
				l.pos += i + 3
			case l.take("<?"):
				i := strings.Index(text[l.pos:], "?>")
				if i < 0 {
					return 0, malformed("unclosed DTD processing instruction")
				}
				l.pos += i + 2
			case text[l.pos] == '\'' || text[l.pos] == '"':
				if _, err := l.literal(); err != nil {
					return 0, err
				}
			default:
				l.pos++
			}
		}
		if l.pos == len(text) {
			return 0, malformed("unclosed internal subset")
		}
		if err := d.subset(text[start:l.pos], base+start, 0, 0, 0, map[string]bool{}, s); err != nil {
			return 0, err
		}
		l.pos++
		l.space()
	}
	if !l.take(">") {
		return 0, malformed("invalid DOCTYPE end")
	}
	return l.pos, nil
}
