package xmltext

import (
	"encoding/binary"
	"testing"
)

func TestLocalContentCertainty(t *testing.T) {
	for _, tc := range []struct {
		body                    string
		children, content, text bool
	}{
		{`&unknown;`, true, true, false},
		{`&#10;&unknown;&#32;`, true, true, false},
		{`known&unknown;`, true, true, true},
		{`&unknown;known`, true, true, true},
		{`&unknown;&#65;&other;`, true, true, true},
		{`<c>&unknown;</c>`, false, true, false},
		{`<c title="&unknown;">known</c>`, false, false, false},
		{`&amp;unknown;`, false, false, true},
		{`<![CDATA[&unknown;]]>`, false, false, true},
		{`&nested;`, true, true, false},
	} {
		for _, order := range []binary.ByteOrder{nil, binary.LittleEndian, binary.BigEndian} {
			raw := `<!DOCTYPE r [%unread; <!ENTITY nested "&unknown;">]><r>` + tc.body + `</r>`
			input := []byte(raw)
			if order != nil {
				input = testUTF16(`<?xml version="1.0" encoding="UTF-16"?>`+raw, order, true)
			}
			doc, err := Parse(input)
			if err != nil {
				t.Fatal(tc.body, order, err)
			}
			e := doc.Root
			if e.ChildrenUnknown != tc.children || e.ContentUnknown != tc.content || e.KnownDirectText != tc.text {
				t.Fatalf("%s %v: children=%v content=%v text=%v", tc.body, order, e.ChildrenUnknown, e.ContentUnknown, e.KnownDirectText)
			}
		}
	}
}

func FuzzT1BLocalContentCertainty(f *testing.F) {
	f.Add("Known text", true)
	f.Add(" \r\n", false)
	f.Fuzz(func(t *testing.T, text string, after bool) {
		// Literal known data and an unresolved entity are independently located.
		// Compare the known-text flag against parsing that known data alone.
		if len(text) > 4096 {
			return
		}
		known := escaped(text)
		control, err := Parse([]byte(`<r>` + known + `</r>`))
		if err != nil {
			return
		}
		body := known + `&unknown;`
		if after {
			body = `&unknown;` + known
		}
		doc, err := Parse([]byte(`<!DOCTYPE r [%unread;]><r>` + body + `</r>`))
		if err != nil {
			t.Fatal(err)
		}
		if !doc.Root.ChildrenUnknown || !doc.Root.ContentUnknown || doc.Root.KnownDirectText != control.Root.KnownDirectText {
			t.Fatal("unknown content erased known text or became definite text")
		}
	})
}
