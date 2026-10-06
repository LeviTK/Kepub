package xmltext

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

// Independent test encoder: expectations are constructed from source fragments,
// not from the production decoder, offset map or replacement implementation.
func testUTF16(s string, order binary.ByteOrder, bom bool) []byte {
	out := []byte{}
	units := utf16.Encode([]rune(s))
	if bom {
		units = append([]uint16{0xfeff}, units...)
	}
	for _, u := range units {
		var b [2]byte
		order.PutUint16(b[:], u)
		out = append(out, b[:]...)
	}
	return out
}

func TestEncodedLocalReplacement(t *testing.T) {
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for _, bom := range []bool{true, false} {
			enc := "UTF-16LE"
			if order == binary.BigEndian {
				enc = "UTF-16BE"
			}
			prefix := `<?xml version='1.0' encoding='` + enc + `'?>` + "\r\n" + `<!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml"><body><!--原 😀--><p>same</p><p id='t'>`
			suffix := `</p><p>same</p></body></html>` + "\r\n"
			input := testUTF16(prefix+`same &amp; 😀`+suffix, order, bom)
			d, err := Parse(input)
			if err != nil {
				t.Fatal(enc, bom, err)
			}
			e := d.Elements[3]
			if e.Location != "/html[1]/body[1]/p[2]" || e.Text != "same & 😀" {
				t.Fatal(e)
			}
			if e.Start != len(testUTF16(prefix, order, bom)) || e.End != len(testUTF16(prefix+`same &amp; 😀`, order, bom)) {
				t.Fatal("not original byte intervals", e.Start, e.End)
			}
			got, changed, err := Replace(input, e, "same & 😀", "新 < 😀\r\n")
			want := testUTF16(prefix+`新 &lt; 😀&#xD;&#xA;`+suffix, order, bom)
			if err != nil || !changed || !bytes.Equal(got, want) {
				t.Fatal("not exact local replacement", enc, bom, err)
			}
		}
	}
}

func TestMalformedEncodingAndDeclarations(t *testing.T) {
	for _, b := range [][]byte{
		[]byte("<r>\xff</r>"),
		{0xff, 0xfe, 0x00, 0xd8}, {0xfe, 0xff, 0xdc, 0x00}, {0xff, 0xfe, 0x3c},
		testUTF16(`<?xml version="1.0" encoding="UTF-16BE"?><r/>`, binary.LittleEndian, true),
		testUTF16(`<?xml version="1.0" encoding="UTF-16"?><r/>`, binary.LittleEndian, false),
		[]byte(`<?xml encoding="UTF-8"?><r/>`), []byte(`<?xml version="1.0" garbage="x"?><r/>`),
		[]byte("<?xml version\v=\"1.0\"?><r/>"), []byte(`<?xml version="1.1"?><r/>`),
		[]byte(`<?xml version="1.0" encoding="UTF-16"?><r/>`),
		[]byte(` <r xmlns:p="u"><p:x/></r><?xml version="1.0"?>`),
	} {
		if _, err := Parse(b); err == nil {
			t.Fatalf("accepted malformed encoding/declaration: %x", b[:min(40, len(b))])
		}
	}
}

func TestRawBoundaryAndDecodedCommentBudget(t *testing.T) {
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		prefix := `<?xml version="1.0" encoding="UTF-16"?><html xmlns="http://www.w3.org/1999/xhtml"><body><!--`
		suffix := `--><p>old</p></body></html>`
		// CJK comment exceeds 8 MiB after UTF-8 decoding but adds no text index.
		s := prefix + strings.Repeat("中", (Limit-2)/2-len(prefix)-len(suffix)) + suffix
		b := testUTF16(s, order, true)
		if len(b) != Limit {
			t.Fatal("fixture raw size", len(b))
		}
		d, err := Parse(b)
		if err != nil {
			t.Fatal("raw boundary/decoded >8MiB must parse", err)
		}
		got, _, err := Replace(b, d.Elements[2], "old", "new")
		if err != nil || !bytes.Equal(got, testUTF16(strings.TrimSuffix(s, suffix)+`--><p>new</p></body></html>`, order, true)) {
			t.Fatal("large comment edit", err)
		}
		if _, err := Parse(append(b, 0, 0)); err == nil {
			t.Fatal("raw overflow accepted")
		}
		if _, _, err := Replace(b, d.Elements[2], "old", "longer"); err == nil {
			t.Fatal("edited raw overflow accepted")
		}
		body := prefix + `--><p>` + strings.Repeat("中", 3<<20) + `</p></body></html>`
		if _, err := Parse(testUTF16(body, order, true)); err == nil {
			t.Fatal("body-heavy index overflow accepted")
		}
		body = prefix + `-->` + strings.Repeat("<div>", 18) + `<p>` + strings.Repeat("中", 600000) + `</p>` + strings.Repeat("</div>", 18) + `</body></html>`
		if _, err := Parse(testUTF16(body, order, true)); err == nil {
			t.Fatal("nested ancestor aggregation overflow accepted")
		}
	}
}

func FuzzEncodedReplacement(f *testing.F) {
	for _, s := range []string{"", "新😀 & < >\r\n", "old & 😀", "é", "\x00\xff"} {
		for _, be := range []bool{false, true} {
			f.Add(s, be, true)
			f.Add(s, be, false)
		}
	}
	f.Fuzz(func(t *testing.T, value string, be, bom bool) {
		if len(value) > 4096 {
			t.Skip()
		}
		if !utf8.ValidString(value) {
			value = strings.ToValidUTF8(value, "😀")
		}
		value = strings.Map(func(r rune) rune {
			if r < 32 && r != 9 && r != 10 && r != 13 || r == 0xfffe || r == 0xffff {
				return 'X'
			}
			return r
		}, value)
		var order binary.ByteOrder = binary.LittleEndian
		enc := "UTF-16LE"
		if be {
			order = binary.BigEndian
			enc = "UTF-16BE"
		}
		prefix := `<?xml version="1.0" encoding="` + enc + `"?><html xmlns="http://www.w3.org/1999/xhtml"><body><p>old &amp; 😀</p><p id="t">`
		suffix := `</p><p>old &amp; 😀</p></body></html>`
		input := testUTF16(prefix+`old &amp; 😀`+suffix, order, bom)
		d, err := Parse(input)
		if err != nil {
			t.Fatal(err)
		}
		got, changed, err := Replace(input, d.Elements[3], "old & 😀", value)
		if err != nil || changed != (value != "old & 😀") {
			t.Fatal("encoded replacement", err)
		}
		var escaped bytes.Buffer
		if err := xml.EscapeText(&escaped, []byte(value)); err != nil {
			t.Fatal(err)
		}
		want := testUTF16(prefix+escaped.String()+suffix, order, bom)
		if value == "old & 😀" {
			want = input
		}
		if !bytes.Equal(got, want) {
			t.Fatal("original prefix/suffix, scalar encoding or escaped bytes changed")
		}
	})
}
