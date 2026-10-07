package publication

import (
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

// Field order is the content.text.replace v1 canonical JSON encoding.
type TextReplace struct {
	BookPath       bookpath.BookPath `json:"bookPath"`
	RevisionID     string            `json:"revisionId"`
	ResourceSHA256 string            `json:"resourceSha256"`
	LocatorVersion int               `json:"locatorVersion"`
	Locator        string            `json:"locator"`
	Mode           string            `json:"mode"`
	Pattern        string            `json:"pattern"`
	Replacement    string            `json:"replacement"`
	ExpectedHits   int               `json:"expectedHits"`
}

const (
	ReplacePatternLimit = 64 << 10
	ReplaceHitsLimit    = 10000
)

func (r TextReplace) Validate() error {
	if _, err := bookpath.Parse(string(r.BookPath)); err != nil {
		return err
	}
	if r.RevisionID == "" || !utf8.ValidString(r.RevisionID) {
		return fmt.Errorf("revisionId is required")
	}
	h, err := hex.DecodeString(r.ResourceSHA256)
	if err != nil || len(h) != 32 || hex.EncodeToString(h) != r.ResourceSHA256 {
		return fmt.Errorf("resourceSha256 must be lowercase SHA-256")
	}
	if r.LocatorVersion != ContentLocatorVersion || r.Locator == "" || len(r.Locator) > 4096 || !utf8.ValidString(r.Locator) {
		return fmt.Errorf("unsupported or invalid locator")
	}
	switch r.Mode {
	case "literal", "regex":
	default:
		return fmt.Errorf("mode must be literal or regex")
	}
	if r.Pattern == "" || len(r.Pattern) > ReplacePatternLimit || !utf8.ValidString(r.Pattern) {
		return fmt.Errorf("pattern is empty, exceeds 64 KiB or is not UTF-8")
	}
	if r.Mode == "regex" {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return fmt.Errorf("pattern is not a valid regex: %v", err)
		}
		if re.MatchString("") {
			return fmt.Errorf("pattern can match the empty string")
		}
	}
	if len(r.Replacement) > ContentTextLimit || !utf8.ValidString(r.Replacement) {
		return fmt.Errorf("replacement exceeds 1 MiB or is not UTF-8")
	}
	for _, c := range r.Replacement {
		if !(c == 9 || c == 10 || c == 13 || c >= 0x20 && c <= 0xD7FF || c >= 0xE000 && c <= 0xFFFD || c >= 0x10000 && c <= 0x10FFFF) {
			return fmt.Errorf("replacement is not XML text")
		}
	}
	if r.ExpectedHits < 0 || r.ExpectedHits > ReplaceHitsLimit {
		return fmt.Errorf("expectedHits must be 0..%d", ReplaceHitsLimit)
	}
	return nil
}

// ReplaceNode is one element the operation changes, with its frozen and planned
// direct character data for review.
type ReplaceNode struct {
	Locator string
	Before  string
	After   string
}

// ReplaceFacts is the complete deterministic evidence of one replace operation.
type ReplaceFacts struct {
	Mode         string
	ExpectedHits int
	Hits         int
	Skipped      int // subtree roots outside the writable XHTML text scope
	Nodes        []ReplaceNode
}

// ReplaceTextEdits derives one replace operation against the frozen document. It
// returns one text-set edit per affected element whose spans replace exactly the
// matched literal intervals, so untouched bytes and the element tree stay.
func (d *StructureDocument) ReplaceTextEdits(op TextReplace) ([]*StructureEdit, ReplaceFacts, error) {
	if err := op.Validate(); err != nil {
		return nil, ReplaceFacts{}, err
	}
	target, err := d.Locate(op.Locator)
	if err != nil {
		return nil, ReplaceFacts{}, err
	}
	if err := replaceTargetRefused(target); err != nil {
		return nil, ReplaceFacts{}, err
	}
	var re *regexp.Regexp
	if op.Mode == "regex" {
		re, err = regexp.Compile(op.Pattern)
		if err != nil {
			return nil, ReplaceFacts{}, fault.New(2, "INVALID_OPERATIONS", "pattern is not a valid regex: %v", err)
		}
	}
	facts := ReplaceFacts{Mode: op.Mode, ExpectedHits: op.ExpectedHits}
	edits := []*StructureEdit{}
	err = walkReplaceScope(target, &facts.Skipped, func(e *xmltext.Element) error {
		edit, node, hits, err := replaceElement(e, op, re)
		if err != nil {
			return err
		}
		if edit != nil {
			edits = append(edits, edit)
			facts.Nodes = append(facts.Nodes, node)
			facts.Hits += hits
		}
		return nil
	})
	if err != nil {
		return nil, ReplaceFacts{}, err
	}
	if facts.Hits != op.ExpectedHits {
		return nil, ReplaceFacts{}, fault.New(2, "INVALID_OPERATIONS", "expected %d hits, found %d", op.ExpectedHits, facts.Hits)
	}
	return edits, facts, nil
}

func replaceTargetRefused(e *xmltext.Element) error {
	if e.Name.Space != XHTMLNamespace {
		return fault.New(2, "INVALID_OPERATIONS", "replace target must be an XHTML element")
	}
	switch e.Name.Local {
	case "html", "head", "script", "style":
		return fault.New(2, "INVALID_OPERATIONS", "replace target %s is not editable text", e.Name.Local)
	}
	for p := e; p != nil; p = p.Parent {
		if p.Name == (xml.Name{Space: XHTMLNamespace, Local: "body"}) {
			return nil
		}
	}
	return fault.New(2, "INVALID_OPERATIONS", "replace target is outside body")
}

// walkReplaceScope visits the target and its XHTML descendants in document
// order. Foreign-namespace subtrees and head/script/style subtrees are outside
// the writable text scope and are counted, never searched.
func walkReplaceScope(e *xmltext.Element, skipped *int, visit func(*xmltext.Element) error) error {
	if e.Name.Space != XHTMLNamespace {
		*skipped++
		return nil
	}
	switch e.Name.Local {
	case "head", "script", "style":
		*skipped++
		return nil
	}
	if err := visit(e); err != nil {
		return err
	}
	for _, c := range e.Children {
		if err := walkReplaceScope(c, skipped, visit); err != nil {
			return err
		}
	}
	return nil
}

type replaceMatch struct {
	start, end  int
	replacement string
}

// findReplaceMatches returns the leftmost non-overlapping matches of one
// element's direct text. Regex replacement supports $name/${name} expansion.
func findReplaceMatches(text string, op TextReplace, re *regexp.Regexp) []replaceMatch {
	out := []replaceMatch{}
	if op.Mode == "literal" {
		for i := 0; i+len(op.Pattern) <= len(text); {
			j := strings.Index(text[i:], op.Pattern)
			if j < 0 {
				break
			}
			start := i + j
			out = append(out, replaceMatch{start, start + len(op.Pattern), op.Replacement})
			i = start + len(op.Pattern)
		}
		return out
	}
	for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
		if m[0] == m[1] {
			continue
		}
		expanded := re.ExpandString(nil, op.Replacement, text, m)
		out = append(out, replaceMatch{m[0], m[1], string(expanded)})
	}
	return out
}

func replaceElement(e *xmltext.Element, op TextReplace, re *regexp.Regexp) (*StructureEdit, ReplaceNode, int, error) {
	runs := e.TextRuns()
	if len(runs) == 0 {
		return nil, ReplaceNode{}, 0, nil
	}
	var builder strings.Builder
	for _, r := range runs {
		builder.WriteString(r.Text)
	}
	text := builder.String()
	if text == "" {
		return nil, ReplaceNode{}, 0, nil
	}
	matches := findReplaceMatches(text, op, re)
	if len(matches) == 0 {
		return nil, ReplaceNode{}, 0, nil
	}
	type runSpan struct {
		start, end int
		run        xmltext.TextRun
	}
	spans := make([]runSpan, 0, len(runs))
	cursor := 0
	for _, r := range runs {
		if len(r.Text) == 0 {
			continue
		}
		spans = append(spans, runSpan{cursor, cursor + len(r.Text), r})
		cursor += len(r.Text)
	}
	edit := &StructureEdit{Change: StructureChange{Kind: "text-set", Locator: e.Location}}
	var replaced strings.Builder
	last := 0
	for _, m := range matches {
		index := -1
		for i := range spans {
			if m.start >= spans[i].start && m.start < spans[i].end {
				index = i
				break
			}
		}
		if index < 0 || m.end > spans[index].end {
			return nil, ReplaceNode{}, 0, fault.New(2, "INVALID_OPERATIONS", "match in %s spans a non-writable text boundary", e.Location)
		}
		start, end, ok := spans[index].run.Range(m.start-spans[index].start, m.end-spans[index].start)
		if !ok {
			return nil, ReplaceNode{}, 0, fault.New(2, "INVALID_OPERATIONS", "match in %s is not in writable literal text", e.Location)
		}
		replacement, err := e.ReplaceBytes(m.replacement)
		if err != nil {
			return nil, ReplaceNode{}, 0, err
		}
		edit.Spans = append(edit.Spans, EditSpan{Start: start, End: end, Bytes: replacement})
		replaced.WriteString(text[last:m.start])
		replaced.WriteString(m.replacement)
		last = m.end
	}
	replaced.WriteString(text[last:])
	edit.Change.Text = replaced.String()
	if len(edit.Change.Text) > ContentTextLimit {
		return nil, ReplaceNode{}, 0, fault.New(2, "INVALID_OPERATIONS", "replaced text exceeds 1 MiB in %s", e.Location)
	}
	return edit, ReplaceNode{Locator: e.Location, Before: text, After: edit.Change.Text}, len(matches), nil
}
