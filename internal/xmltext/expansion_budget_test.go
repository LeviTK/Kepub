package xmltext

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/fault"
)

func TestT1BActualRetainedExpansionBytes(t *testing.T) {
	for _, tc := range []struct{ name, value, body string }{
		{"audit-CR-content", strings.Repeat("&#13;", 1500000), `<r>&e1;</r>`},
		{"escaped-attribute", strings.Repeat("&#34;", 1200000), `<r value="&e1;"/>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := []byte(`<!DOCTYPE r [<!ENTITY e0 "` + tc.value + `"><!ENTITY e1 "&e0;&e0;&e0;&e0;">]>` + tc.body)
			if len(input) >= Limit {
				t.Fatal("counterexample must be below raw limit", len(input))
			}
			s, err := decode(input)
			if err != nil {
				t.Fatal(err)
			}
			es, dtd, err := expand(s)
			var limit *fault.Error
			if !errors.As(err, &limit) || limit.Code != "XML_LIMIT" {
				if err == nil {
					t.Fatalf("retained stream overflow accepted: raw=%d actualStream=%d work=%d", len(input), len(es.text), dtd.work)
				}
				t.Fatal("wrong budget failure", err)
			}
			if _, err := Parse(input); !errors.As(err, &limit) || limit.Code != "XML_LIMIT" {
				t.Fatal("Parse did not propagate the real expansion budget failure", err)
			}
			// Same declaration and bytes, one use instead of four: this must
			// remain accepted, including constructed CR/quote semantics.
			positive := []byte(strings.Replace(string(input), tc.body, strings.Replace(tc.body, "e1", "e0", 1), 1))
			doc, err := Parse(positive)
			if err != nil {
				t.Fatal("under-limit control was rejected", err)
			}
			if tc.name == "audit-CR-content" && doc.Root.Text != strings.Repeat("\r", 1500000) || tc.name == "escaped-attribute" && doc.Root.Attributes[0].Value != strings.Repeat(`"`, 1200000) {
				t.Fatal("under-limit control lost CR/attribute data")
			}
		})
	}
}

func TestT1BStreamAppendExactByteBoundary(t *testing.T) {
	// The retained XML spelling is longer than its eventual decoded value.
	// This exercises the append boundary directly, independently of index/work.
	for _, extra := range []int{0, 1} {
		x := expansion{text: make([]byte, DecodedLimit-len("&#xD;"))}
		err := x.append("&#xD;"+strings.Repeat(" ", extra), 0, 1, false, true)
		if extra == 0 {
			if err != nil || len(x.text) != DecodedLimit {
				t.Fatal("exact retained-stream limit must pass", len(x.text), err)
			}
		} else {
			var limit *fault.Error
			if !errors.As(err, &limit) || limit.Code != "XML_LIMIT" || len(x.text) != DecodedLimit-len("&#xD;") {
				t.Fatal("overflow must be rejected before adding any bytes", len(x.text), err)
			}
		}
	}
}

func TestT1BSerializedIntermediateWorkBoundary(t *testing.T) {
	// Sixteen bounded nested inclusions produce a small final stream, but
	// each level constructs the same five-byte CR spelling. Count all levels.
	const levels = 16
	for _, extra := range []int{0, 1} {
		count := DecodedLimit/(levels*len("&#xD;")) + extra
		var input strings.Builder
		input.WriteString(`<!DOCTYPE r [<!ENTITY e0 "` + strings.Repeat("&#13;", count) + `">`)
		for i := 1; i < levels; i++ {
			fmt.Fprintf(&input, `<!ENTITY e%d "&e%d;">`, i, i-1)
		}
		input.WriteString(`]><r>&e15;</r>`)
		doc, err := Parse([]byte(input.String()))
		if extra == 0 {
			if err != nil || doc.Root.Text != strings.Repeat("\r", count) {
				t.Fatal("last legal serialized intermediate-work value", err)
			}
		} else {
			var limit *fault.Error
			if !errors.As(err, &limit) || limit.Code != "XML_LIMIT" || !strings.Contains(limit.Message, "work") {
				t.Fatal("work overflow with raw/stream/index still legal", err)
			}
		}
	}
}

func TestT1BSerializedAttributeWorkBoundary(t *testing.T) {
	// Each of sixteen inclusions builds one quote byte; the final normalized
	// attribute serializes each quote as five bytes. A default also adds the
	// nine-byte ` value="..."` syntax. Raw/final-stream/index all stay legal.
	for _, defaulted := range []bool{false, true} {
		syntax := 0
		if defaulted {
			syntax = len(` value=""`)
		}
		for _, extra := range []int{0, 1} {
			count := (DecodedLimit-syntax)/(16+len("&#34;")) + extra
			var input strings.Builder
			input.WriteString(`<!DOCTYPE r [<!ENTITY e0 "` + strings.Repeat("&#34;", count) + `">`)
			for i := 1; i < 16; i++ {
				fmt.Fprintf(&input, `<!ENTITY e%d "&e%d;">`, i, i-1)
			}
			if defaulted {
				input.WriteString(`<!ATTLIST r value CDATA "&e15;">]><r/>`)
			} else {
				input.WriteString(`]><r value="&e15;"/>`)
			}
			doc, err := Parse([]byte(input.String()))
			if extra == 0 {
				if err != nil || len(doc.Root.Attributes) != 1 || doc.Root.Attributes[0].Value != strings.Repeat(`"`, count) {
					t.Fatal("last legal attribute intermediate-work value", defaulted, err)
				}
			} else {
				var limit *fault.Error
				if !errors.As(err, &limit) || limit.Code != "XML_LIMIT" || !strings.Contains(limit.Message, "work") {
					t.Fatal("serialized attribute/default work overflow was not counted", defaulted, err)
				}
			}
		}
	}
}
