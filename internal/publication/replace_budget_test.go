package publication

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

// The 64-byte capture and 128-byte template allocate only 4 KiB on the old
// implementation. A 1 KiB injected budget must reject BEFORE ExpandString.
func TestN1ExpansionBeforeAllocation(t *testing.T) {
	op := literalReplace("/html[1]/body[1]/p[1]", `(a+)`, strings.Repeat("$1", 64), 1)
	op.Mode = "regex"
	budget := NewReplaceBudget()
	budget.ReplacementBytes = 1024
	matches, err := findReplaceMatches(strings.Repeat("a", 64), op, regexp.MustCompile(op.Pattern), budget)
	var resourceFault *fault.Error
	if !errors.As(err, &resourceFault) || resourceFault.Code != "RESOURCE_LIMIT" {
		length := 0
		if len(matches) != 0 {
			length = len(matches[0].replacement)
		}
		t.Fatalf("expanded %d bytes before 1024-byte budget: err=%v", length, err)
	}
	if budget.expansions != 0 {
		t.Fatalf("over-budget expansion materialized: %d", budget.expansions)
	}
}

func TestN1HitAndMetadataBudgets(t *testing.T) {
	for _, mode := range []string{"literal", "regex"} {
		for _, hits := range []int{2, 3, 4} {
			t.Run(fmt.Sprintf("%s/hits=%d", mode, hits), func(t *testing.T) {
				op := literalReplace("/html[1]/body[1]/p[1]", "a", "b", hits)
				op.Mode = mode
				budget := NewReplaceBudget()
				budget.Hits = 3
				matches, err := findReplaceMatches(strings.Repeat("a", hits), op, regexp.MustCompile("a"), budget)
				if hits == 4 {
					n1Fault(t, err, "RESOURCE_LIMIT")
					if budget.captures != 0 || budget.expansions != 0 {
						t.Fatalf("over-hit captures/expansions: %+v", budget)
					}
				} else if err != nil || len(matches) != hits || budget.Hits != 3-hits {
					t.Fatalf("matches=%+v budget=%+v err=%v", matches, budget, err)
				}
			})
		}
		// Metadata consists of replaceMatch's four machine words. Regex also
		// retains two index slices: 5 words overall, 7 words with one capture.
		cost := int64(4 * strconv.IntSize / 8)
		if mode == "regex" {
			cost += int64(12 * strconv.IntSize / 8)
		}
		for _, delta := range []int64{-1, 0, 1} {
			t.Run(fmt.Sprintf("%s/metadata=%d", mode, cost+delta), func(t *testing.T) {
				op := literalReplace("/html[1]/body[1]/p[1]", "a", "b", 1)
				op.Mode = mode
				budget := NewReplaceBudget()
				budget.MatchBytes = cost + delta
				matches, err := findReplaceMatches("a", op, regexp.MustCompile("(a)"), budget)
				if delta < 0 {
					n1Fault(t, err, "RESOURCE_LIMIT")
					if budget.captures != 0 || budget.expansions != 0 {
						t.Fatalf("metadata allocated before gate: %+v", budget)
					}
				} else if err != nil || len(matches) != 1 || budget.MatchBytes != delta {
					t.Fatalf("matches=%+v budget=%+v err=%v", matches, budget, err)
				}
			})
		}
	}
}

func TestN1BindingBeforeExpansion(t *testing.T) {
	for _, mode := range []string{"literal", "regex"} {
		t.Run(mode, func(t *testing.T) {
			op := literalReplace("/html[1]/body[1]/p[1]", "a", "b", 1)
			op.Mode = mode
			budget := NewReplaceBudget()
			_, err := findReplaceMatches("aa", op, regexp.MustCompile("a"), budget)
			n1Fault(t, err, "INVALID_OPERATIONS")
			if budget.expansions != 0 || budget.encodings != 0 {
				t.Fatalf("unexpected materialization: %+v", budget)
			}
		})
	}
}

func TestN1TemplateGrammar(t *testing.T) {
	re := regexp.MustCompile(`(?P<first>a+)(?P<other>b)?|(?P<first>c)(?P<_z>d)?`)
	source := "aaccdabbc"
	for _, template := range []string{
		"$0/$1/${first}/${other}/$$", "$first-other", "${_z}", "$9/$1000000000/$01",
		"$1x", "${1}x", "${missing}/$不存在_1", "$$1", "$", "${}", "${first", "$-/$${first}",
	} {
		t.Run(template, func(t *testing.T) {
			for _, match := range re.FindAllStringSubmatchIndex(source, -1) {
				want := re.ExpandString(nil, template, source, match)
				got, err := replacementSize(template, match, re.SubexpNames(), int64(len(want)))
				if err != nil || got != int64(len(want)) {
					t.Fatalf("size=%d want=%q err=%v", got, want, err)
				}
				if len(want) > 0 {
					_, err = replacementSize(template, match, re.SubexpNames(), int64(len(want)-1))
					n1Fault(t, err, "RESOURCE_LIMIT")
				}
			}
		})
	}
	// A virtual capture tests overflow without allocating any corresponding text.
	_, err := replacementSize("$0$0", []int{0, math.MaxInt}, []string{""}, int64(math.MaxInt))
	n1Fault(t, err, "RESOURCE_LIMIT")
}

func TestN1CumulativeExpansion(t *testing.T) {
	for _, mode := range []string{"literal", "regex"} {
		for _, scope := range []string{"matches", "nodes"} {
			t.Run(mode+"/"+scope, func(t *testing.T) {
				text := `<p>aa</p>`
				if scope == "nodes" {
					text = `<p>a</p><p>a</p>`
				}
				doc := replaceDoc(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body>`+text+`</body></html>`)
				op := literalReplace("/html[1]/body[1]", "a", "xxxx", 2)
				op.Mode = mode
				budget := NewReplaceBudget()
				budget.ReplacementBytes = 7
				edits, _, err := doc.ReplaceTextEdits(op, budget)
				n1Fault(t, err, "RESOURCE_LIMIT")
				if edits != nil || budget.ReplacementBytes != 3 {
					t.Fatalf("partial output/budget: %+v %+v", edits, budget)
				}
				if mode == "regex" && budget.expansions != 1 {
					t.Fatalf("second over-budget expansion executed: %+v", budget)
				}
				budget = NewReplaceBudget()
				budget.ReplacementBytes = 8
				edits, facts, err := doc.ReplaceTextEdits(op, budget)
				if err != nil || facts.Hits != 2 || budget.ReplacementBytes != 0 {
					t.Fatalf("exact budget: %+v %+v %v", facts, budget, err)
				}
				want := strings.ReplaceAll(string(doc.Input), "a</p>", "xxxx</p>")
				if scope == "matches" {
					want = strings.Replace(string(doc.Input), "aa</p>", "xxxxxxxx</p>", 1)
				}
				if got := ApplyEdits(doc.Input, edits); string(got) != want {
					t.Fatalf("output=%q want=%q", got, want)
				}
			})
		}
	}
}

func TestN1EncodedAndResultBudgets(t *testing.T) {
	input := `<html xmlns="http://www.w3.org/1999/xhtml"><body><p data-note='keep'>x</p><p>untouched &amp; text</p></body></html>` + "\r\n"
	for _, enc := range []string{"utf8", "utf16le", "utf16be"} {
		t.Run(enc, func(t *testing.T) {
			doc, err := ParseStructureDocument(replaceEncode(input, enc), "EPUB/a.xhtml", xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
			if err != nil {
				t.Fatal(err)
			}
			op := literalReplace("/html[1]/body[1]/p[1]", "x", "&<>新🙂", 1)
			encodedSize := int64(20) // 13 ASCII escape bytes + 3-byte BMP + 4-byte emoji
			if enc != "utf8" {
				encodedSize = 32 // 13 ASCII units + one BMP + one surrogate pair
			}
			budget := NewReplaceBudget()
			budget.EncodedBytes = encodedSize - 1
			_, _, err = doc.ReplaceTextEdits(op, budget)
			n1Fault(t, err, "RESOURCE_LIMIT")
			if budget.encodings != 0 {
				t.Fatalf("over-budget escape/encoding executed: %+v", budget)
			}
			budget = NewReplaceBudget()
			budget.EncodedBytes = encodedSize
			edits, _, err := doc.ReplaceTextEdits(op, budget)
			if err != nil || budget.EncodedBytes != 0 || budget.encodings != 1 {
				t.Fatalf("exact encoding budget: %+v %v", budget, err)
			}
			want := replaceEncode(strings.Replace(input, ">x<", ">&amp;&lt;&gt;新🙂<", 1), enc)
			budget.ResourceBytes = int64(len(want) - 1)
			n1Fault(t, budget.TakeOutput(doc.Input, edits), "RESOURCE_LIMIT")
			budget.ResourceBytes = int64(len(want))
			budget.OutputBytes = int64(len(want))
			if err := budget.TakeOutput(doc.Input, edits); err != nil || budget.OutputBytes != 0 {
				t.Fatalf("exact output budget: %+v %v", budget, err)
			}
			got := ApplyEdits(doc.Input, edits)
			if !bytes.Equal(got, want) {
				t.Fatalf("physical bytes: got=%q want=%q", got, want)
			}
			if err := VerifyStructure(doc, edits, got); err != nil {
				t.Fatal(err)
			}
			budget = NewReplaceBudget()
			budget.ResultBytes = int64(len(op.Replacement) - 1)
			_, _, err = doc.ReplaceTextEdits(op, budget)
			n1Fault(t, err, "RESOURCE_LIMIT")
			if budget.results != 0 || budget.encodings != 0 {
				t.Fatalf("over-budget result constructed: %+v", budget)
			}
		})
	}
}

func TestN1NoopAndZeroWidth(t *testing.T) {
	for _, enc := range []string{"utf8", "utf16le", "utf16be"} {
		t.Run(enc, func(t *testing.T) {
			input := `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>a` + "\r\n" + `b</p></body></html>`
			doc, err := ParseStructureDocument(replaceEncode(input, enc), "EPUB/a.xhtml", xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
			if err != nil {
				t.Fatal(err)
			}
			budget := NewReplaceBudget()
			budget.EncodedBytes = 0
			edits, facts, err := doc.ReplaceTextEdits(literalReplace("/html[1]/body[1]/p[1]", "\n", "\n", 1), budget)
			if err != nil || facts.Hits != 1 || budget.encodings != 0 || !bytes.Equal(ApplyEdits(doc.Input, edits), doc.Input) {
				t.Fatalf("no-op changed physical CRLF: %+v %+v %v", facts, budget, err)
			}
			budget = NewReplaceBudget()
			budget.Hits, budget.MatchBytes = 0, 0
			for _, mode := range []string{"literal", "regex"} {
				op := literalReplace("/html[1]/body[1]/p[1]", "absent", "x", 0)
				op.Mode = mode
				edits, facts, err = doc.ReplaceTextEdits(op, budget)
				if err != nil || len(edits) != 0 || facts.Hits != 0 {
					t.Fatalf("no-match incorrectly refused: %+v %v", facts, err)
				}
			}
		})
	}
	entity := replaceDoc(t, `<html xmlns="http://www.w3.org/1999/xhtml"><body><p>a&amp;b</p></body></html>`)
	_, _, err := entity.ReplaceTextEdits(literalReplace("/html[1]/body[1]/p[1]", "&", "&", 1), NewReplaceBudget())
	n1Fault(t, err, "INVALID_OPERATIONS")
	if !strings.Contains(err.Error(), "writable literal") {
		t.Fatalf("no-op bypassed generated provenance: %v", err)
	}
	op := literalReplace("/html[1]/body[1]/p[1]", `alpha|\b`, "X", 0)
	op.Mode = "regex"
	budget := NewReplaceBudget()
	_, err = findReplaceMatches("alpha beta", op, regexp.MustCompile(op.Pattern), budget)
	n1Fault(t, err, "INVALID_OPERATIONS")
	if !strings.Contains(err.Error(), "zero-width") || budget.expansions != 0 {
		t.Fatalf("zero-width not detected before expansion: %+v %v", budget, err)
	}
}

func TestN1OutputNetShrink(t *testing.T) {
	input := []byte("ab01234567")
	for _, reverse := range []bool{false, true} {
		edits := []*StructureEdit{
			{Spans: []EditSpan{{Start: 0, End: 1, Bytes: []byte("XXXX")}}},
			{Spans: []EditSpan{{Start: 2, End: 7}}},
		}
		if reverse {
			edits[0], edits[1] = edits[1], edits[0]
		}
		if err := ValidateEdits(edits); err != nil {
			t.Fatal(err)
		}
		budget := NewReplaceBudget()
		budget.ResourceBytes, budget.OutputBytes = 8, 8
		if err := budget.TakeOutput(input, edits); err != nil {
			t.Fatalf("legal final net shrink rejected (reverse=%v): %v", reverse, err)
		}
		if got := ApplyEdits(input, edits); string(got) != "XXXXb567" {
			t.Fatalf("output=%q", got)
		}
	}
}

func n1Fault(t *testing.T, err error, code string) {
	t.Helper()
	var f *fault.Error
	if !errors.As(err, &f) || f.Code != code || code == "RESOURCE_LIMIT" && f.Exit != 1 {
		t.Fatalf("expected %s, got %v", code, err)
	}
}

func BenchmarkN1RejectedCapture(b *testing.B) {
	op := literalReplace("/html[1]/body[1]/p[1]", `(a+)`, strings.Repeat("$1", 64), 1)
	op.Mode = "regex"
	re := regexp.MustCompile(op.Pattern)
	text := strings.Repeat("a", 64)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		budget := NewReplaceBudget()
		budget.ReplacementBytes = 1024
		_, err := findReplaceMatches(text, op, re, budget)
		if err == nil || budget.expansions != 0 {
			b.Fatalf("over-budget expansion: %+v %v", budget, err)
		}
	}
}

func FuzzN1ReplacementSize(f *testing.F) {
	for _, template := range []string{"$0/${first}/$$", "$1x", "${1}x", "$01/${missing}", "${first", "$${first}"} {
		f.Add(template)
	}
	re := regexp.MustCompile(`(?P<first>a+)(b)?|(?P<first>c)(d)?`)
	source := "aacdbbcaa"
	f.Fuzz(func(t *testing.T, template string) {
		if len(template) > 256 || !utf8.ValidString(template) {
			return
		}
		for _, match := range re.FindAllStringSubmatchIndex(source, -1) {
			want := re.ExpandString(nil, template, source, match)
			got, err := replacementSize(template, match, re.SubexpNames(), int64(len(want)))
			if err != nil || got != int64(len(want)) {
				t.Fatalf("template=%q size=%d want=%q err=%v", template, got, want, err)
			}
		}
	})
}
