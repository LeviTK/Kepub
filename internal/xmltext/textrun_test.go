package xmltext

import (
	"strings"
	"testing"
)

// TestTextRunsKeepsWritableLiteralIntervals pins the direct-run model the batch
// text replace relies on: decoded text, physical intervals per decoded byte,
// CRLF normalization and the non-writable entity/CDATA cases.
func TestTextRunsKeepsWritableLiteralIntervals(t *testing.T) {
	input := []byte(`<html xmlns="http://www.w3.org/1999/xhtml"><body><p>hello world</p><p>a&amp;b</p><p>line` + "\r\n" + `two</p><p>foo<em>x</em>bar</p><p><![CDATA[cdata]]></p></body></html>`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	byText := map[string]*Element{}
	for _, e := range doc.Elements {
		if e.Name.Local == "p" {
			byText[e.DirectText] = e
		}
	}
	plain := byText["hello world"]
	if plain == nil {
		t.Fatalf("plain run element missing: %v", keys(byText))
	}
	runs := plain.TextRuns()
	if len(runs) != 1 || !runs[0].Writable || runs[0].Text != "hello world" {
		t.Fatalf("plain runs: %+v", runs)
	}
	start, end, ok := runs[0].Range(0, 5)
	if !ok || string(input[start:end]) != "hello" {
		t.Fatalf("plain range: %d %d %v %q", start, end, ok, input[start:end])
	}
	if _, _, ok := runs[0].Range(0, len(runs[0].Text)+1); ok {
		t.Fatal("out-of-range interval accepted")
	}
	entity := byText["a&b"]
	if entity == nil {
		t.Fatalf("entity element missing: %v", keys(byText))
	}
	if runs := entity.TextRuns(); len(runs) != 1 || runs[0].Writable {
		t.Fatalf("entity runs: %+v", runs)
	}
	crlf := byText["line\ntwo"]
	if crlf == nil {
		t.Fatalf("crlf element missing: %v", keys(byText))
	}
	runs = crlf.TextRuns()
	if len(runs) != 1 || !runs[0].Writable || runs[0].Text != "line\ntwo" {
		t.Fatalf("crlf runs: %+v", runs)
	}
	start, end, ok = runs[0].Range(0, 5)
	if !ok || string(input[start:end]) != "line\r\n" {
		t.Fatalf("crlf range: %d %d %v %q", start, end, ok, input[start:end])
	}
	start, end, ok = runs[0].Range(5, len(runs[0].Text))
	if !ok || string(input[start:end]) != "two" {
		t.Fatalf("crlf tail range: %d %d %v %q", start, end, ok, input[start:end])
	}
	mixed := byText["foobar"]
	if mixed == nil {
		t.Fatalf("mixed element missing: %v", keys(byText))
	}
	runs = mixed.TextRuns()
	if len(runs) != 2 || runs[0].Text != "foo" || runs[1].Text != "bar" || !runs[0].Writable || !runs[1].Writable {
		t.Fatalf("mixed runs: %+v", runs)
	}
	start, end, ok = runs[1].Range(0, 3)
	if !ok || string(input[start:end]) != "bar" {
		t.Fatalf("mixed tail range: %d %d %v %q", start, end, ok, input[start:end])
	}
	cdata := byText["cdata"]
	if cdata == nil {
		t.Fatalf("cdata element missing: %v", keys(byText))
	}
	if runs := cdata.TextRuns(); len(runs) != 1 || runs[0].Writable {
		t.Fatalf("cdata runs: %+v", runs)
	}
}

func keys(m map[string]*Element) string {
	out := []string{}
	for k := range m {
		out = append(out, k)
	}
	return strings.Join(out, "|")
}
