package xmltext

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/fault"
)

// Written against the fixed XML internal-subset rules before implementation.
// This does not depend on the pending public coverage/profile output shape.
func TestT1BInternalDeclarationsAndNormalization(t *testing.T) {
	input := []byte(`<!DOCTYPE r [
<!ELEMENT r (p*)>
<!ELEMENT p (#PCDATA|b)*>
<!ELEMENT b (#PCDATA)>
<!ATTLIST r mode NMTOKENS "  first   second  " data CDATA #IMPLIED>
<!ENTITY word "fresh &amp; exact">
<!ENTITY markup "<b>child</b>">
<!ENTITY % attrs '<!ATTLIST r extra CDATA "default">'>
%attrs;
<!NOTATION png SYSTEM "urn:example:notation:png">
<!ATTLIST r format NOTATION (png) #IMPLIED>
]><r data="A&#x9;B" format="png"><p>plain</p><p>&word;</p><p>&markup;</p></r>`)
	d, err := Parse(input)
	if err != nil {
		t.Fatal("legal internal declarations must be processed, not unsupported", err)
	}
	attrs := map[string]string{}
	for _, a := range d.Root.Attributes {
		attrs[a.Name.Local] = a.Value
	}
	for name, want := range map[string]string{"mode": "first second", "data": "A\tB", "extra": "default", "format": "png"} {
		if attrs[name] != want {
			t.Fatalf("%s: got %q, want %q", name, attrs[name], want)
		}
	}
	if len(d.Root.Children) != 3 || d.Root.Children[1].Text != "fresh & exact" || d.Root.Children[2].Text != "child" || len(d.Root.Children[2].Children) != 1 {
		t.Fatal("entity text/markup or PE declaration was discarded")
	}
	generated := d.Root.Children[2].Children[0]
	if _, _, err := Replace(input, generated, "child", "changed"); err == nil {
		t.Fatal("entity-generated element cannot acquire a raw writable interval")
	}
}

func TestT1BEntityBypassAndReplacementConstruction(t *testing.T) {
	for _, tc := range []struct {
		value, want string
		child       bool
	}{
		{`&lt;b/>`, "<b/>", false},
		{`&#60;b/>`, "", true},
	} {
		input := []byte(`<!DOCTYPE r [<!ENTITY e "` + tc.value + `">]><r>&e;</r>`)
		d, err := Parse(input)
		if err != nil || d.Root.Text != tc.want || (len(d.Root.Children) == 1) != tc.child {
			t.Fatal("bypassed general reference differs from constructed character reference", tc, d, err)
		}
	}
}

func TestT1BDeclarationBudgetBoundary(t *testing.T) {
	for _, count := range []int{4096, 4097} {
		// Repeated ELEMENT declarations are a validity constraint, not a WFC.
		// This non-validating probe independently exercises declaration count.
		input := []byte(`<!DOCTYPE r [` + strings.Repeat(`<!ELEMENT r EMPTY>`, count) + `]><r/>`)
		d, err := Parse(input)
		if count == 4096 {
			if err != nil || d.Root.Name.Local != "r" {
				t.Fatal("exact declaration budget must parse", err)
			}
		} else if err == nil || !bytes.Contains([]byte(err.Error()), []byte("limit")) {
			t.Fatal("declaration overflow must be an explicit budget failure", err)
		}
	}
}

func TestT1BReplacementBoundariesAndAttributeData(t *testing.T) {
	input := []byte(`<!DOCTYPE r [<!ENTITY text "&lt;em&gt;literal&lt;/em&gt;"><!ENTITY markup "&#60;em>Generated&#60;/em>"><!ENTITY quote '" injected="no'>]><r a="&quote;"><p>&text;</p><p>&markup;</p></r>`)
	d, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Root.Attributes) != 1 || d.Root.Attributes[0].Value != `" injected="no` || len(d.Root.Children[0].Children) != 0 || d.Root.Children[0].Text != "<em>literal</em>" || d.Root.Children[1].Children[0].Name.Local != "em" {
		t.Fatal("attribute/entity inclusion boundary was lost")
	}
	for _, bad := range []string{
		`<!DOCTYPE r [<!ENTITY open "<em>"><!ENTITY close "</em>">]><r>&open;&close;</r>`,
		`<!DOCTYPE r [<!ENTITY % n "r"><!ELEMENT %n; EMPTY>]><r/>`,
		`<!DOCTYPE r [<!ENTITY % n "r"><!ENTITY x "%n;">]><r/>`,
		`<!DOCTYPE r [<!ELEMENT r (a|b,c)>]><r/>`,
		`<!DOCTYPE r [<!ELEMENT r (#PCDATA|a)>]><r/>`,
		`<!DOCTYPE r [<!ATTLIST r a CDATA "&late;"><!ENTITY late "value">]><r/>`,
		`<!DOCTYPE r [<!ENTITY a "&b;"><!ENTITY b "&a;">]><r>&a;</r>`,
		`<!DOCTYPE r [<!ENTITY a "<em/>">]><r a="&a;"/>`,
	} {
		_, err := Parse([]byte(bad))
		var f *fault.Error
		if !errors.As(err, &f) || f.Code != "XML_NOT_WELL_FORMED" {
			t.Fatal("expected independent XML WFC/grammar failure", bad, err)
		}
	}
}

func TestT1BUnknownOriginsAndUnreadPE(t *testing.T) {
	for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		prefix := `<?xml version="1.0" encoding="UTF-16"?><!DOCTYPE r [<!ENTITY outer "&missing; &other;">%unread;<!ATTLIST r certainty CDATA "wrong"><!ENTITY late "wrong">]><r>`
		input := testUTF16(prefix+`&outer; &direct; &late;</r>`, order, true)
		d, err := Parse(input)
		if err != nil || len(d.Unresolved) != 5 || len(d.Root.Attributes) != 0 || d.Root.Text != "&missing; &other; &direct; &late;" {
			t.Fatal("unread PE must suppress later defaults/entities without erasing references", order, err)
		}
		want := []Unresolved{
			{"unread", "parameter", len(testUTF16(prefix[:strings.Index(prefix, "%unread;")], order, true)), len(testUTF16(prefix[:strings.Index(prefix, "%unread;")+len("%unread;")], order, true)), "reference"},
			{"missing", "general", len(testUTF16(prefix, order, true)), len(testUTF16(prefix+"&outer;", order, true)), "expansion"},
			{"other", "general", len(testUTF16(prefix, order, true)), len(testUTF16(prefix+"&outer;", order, true)), "expansion"},
			{"direct", "general", len(testUTF16(prefix+"&outer; ", order, true)), len(testUTF16(prefix+"&outer; &direct;", order, true)), "reference"},
			{"late", "general", len(testUTF16(prefix+"&outer; &direct; ", order, true)), len(testUTF16(prefix+"&outer; &direct; &late;", order, true)), "reference"},
		}
		for i, expected := range want {
			if d.Unresolved[i] != expected {
				t.Fatal("not an original-byte source, including BOM", i, d.Unresolved[i], expected)
			}
		}
		if _, _, err := Replace(input, d.Root, d.Root.Text, "replacement"); err == nil {
			t.Fatal("unknown text cannot become a writable literal target")
		}
	}
	for _, text := range []string{
		`<?xml version="1.0" standalone="yes"?><!DOCTYPE r [%unknown;]><r/>`,
		`<?xml version="1.0" standalone="yes"?><!DOCTYPE r SYSTEM "urn:never-read"><r>&unknown;</r>`,
	} {
		_, err := Parse([]byte(text))
		var f *fault.Error
		if !errors.As(err, &f) || f.Code != "XML_NOT_WELL_FORMED" {
			t.Fatal("standalone unknown reference is a WFC, not partial", err)
		}
	}
}

func TestT1BNotationAndNonvalidatingDefaults(t *testing.T) {
	d, err := Parse([]byte(`<!DOCTYPE r [<!NOTATION image SYSTEM "file:///never-read"><!NOTATION image PUBLIC "ignored" "urn:ignored"><!NOTATION helper PUBLIC "helper-id"><!ATTLIST r code NMTOKENS #FIXED " a  b "><!ATTLIST r code CDATA "ignored">]><r code="  different  token "/>`))
	if err != nil || len(d.Notations) != 2 || d.Notations[0] != (Notation{"image", "", "file:///never-read"}) || d.Notations[1] != (Notation{"helper", "helper-id", ""}) || d.Root.Attributes[0].Value != "different token" {
		t.Fatal("first declaration and non-validating #FIXED normalization", err)
	}
	if err := d.CheckProfile(Profile{"3.0", "application/xhtml+xml"}); err != nil {
		t.Fatal("NOTATION identifier is not a DOCTYPE/external ENTITY policy violation", err)
	}
}

func TestT1BExactEntityWorkDepthAndReplacementBudgets(t *testing.T) {
	for _, size := range []int{1 << 20, (1 << 20) + 1} {
		var b strings.Builder
		b.WriteString(`<!DOCTYPE r [<!ENTITY e0 "` + strings.Repeat("x", size) + `">`)
		for i := 1; i < 16; i++ {
			fmt.Fprintf(&b, `<!ENTITY e%d "&e%d;">`, i, i-1)
		}
		b.WriteString(`]><r>&e15;</r>`)
		d, err := Parse([]byte(b.String()))
		if size == 1<<20 {
			if err != nil || len(d.Root.Text) != size {
				t.Fatal("exact 16 MiB cumulative work and depth 16 must pass", err)
			}
		} else {
			var f *fault.Error
			if !errors.As(err, &f) || f.Code != "XML_LIMIT" {
				t.Fatal("nested produced bytes, not merely raw reference length, must count", err)
			}
		}
	}
	for _, count := range []int{100000, 100001} {
		d, err := Parse([]byte(`<!DOCTYPE r [<!ENTITY empty "">]><r>` + strings.Repeat("&empty;", count) + `</r>`))
		if count == 100000 {
			if err != nil || d.Root.Text != "" || d.Root.Complex {
				t.Fatal("empty substitutions count but do not make physical tags virtual", err)
			}
		} else {
			var f *fault.Error
			if !errors.As(err, &f) || f.Code != "XML_LIMIT" {
				t.Fatal("empty replacement count overflow", err)
			}
		}
	}
}

func TestT1BExpandedStreamBoundaryIndependentOfWorkAndIndex(t *testing.T) {
	literal := strings.Repeat("x", 1<<20)
	prefix := `<!DOCTYPE r [<!--`
	suffix := `--><!ENTITY e "` + literal + `">]><r>`
	body := strings.Repeat("&e;", 9) + `</r>`
	// Expanded stream retains the declaration; the body independently expands
	// to 9 MiB, with 9 MiB work and an 18 MiB text index, all below their limits.
	padding := DecodedLimit - len(prefix) - len(suffix) - (9 << 20) - len(`</r>`)
	for _, extra := range []int{0, 1} {
		input := []byte(prefix + strings.Repeat("c", padding+extra) + suffix + body)
		if len(input) >= Limit {
			t.Fatal("probe must not accidentally exercise raw XML limit")
		}
		d, err := Parse(input)
		if extra == 0 {
			if err != nil || len(d.Root.Text) != 9<<20 {
				t.Fatal("exact expanded stream limit", err)
			}
		} else {
			var f *fault.Error
			if !errors.As(err, &f) || f.Code != "XML_LIMIT" {
				t.Fatal("expanded stream overflow, with work/index still legal", err)
			}
		}
	}
}

func TestT1BProfilesAreNotInferredFromDOCTYPE(t *testing.T) {
	input := []byte(`<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd"><svg xmlns="http://www.w3.org/2000/svg"/>`)
	for _, tc := range []struct{ version, media, code string }{
		{"3.0", "image/svg+xml", ""},
		{"3.0", "application/xhtml+xml", "XML_POLICY"},
		{"3.0", "", "XML_POLICY"},
		{"2.0", "image/svg+xml", "UNSUPPORTED_XML_DTD"},
	} {
		d, err := Parse(input)
		if err != nil {
			t.Fatal(err)
		}
		err = d.CheckProfile(Profile{tc.version, tc.media})
		var f *fault.Error
		if tc.code == "" && err != nil || tc.code != "" && (!errors.As(err, &f) || f.Code != tc.code) {
			t.Fatal("effective version and actual manifest MIME must govern policy", tc, err)
		}
	}
}

func TestT1BConstructedCRAndDefaultDeclarationOrder(t *testing.T) {
	input := []byte("<!DOCTYPE r [<!ENTITY e 'A&#13;B<![CDATA[C&#13;D\ue000]]>'>]><r>&e;<p>literal\r\nline\ue000</p></r>")
	d, err := Parse(input)
	if err != nil || d.Root.DirectText != "A\rBC\rD\ue000" || d.Root.Children[0].Text != "literal\nline\ue000" {
		t.Fatal("constructed CR is not a second physical EOL; literal sentinel must survive", err)
	}
	for _, bad := range []string{
		`<!DOCTYPE r [<!ENTITY first "&later;"><!ATTLIST r a CDATA "&first;"><!ENTITY later "wrong">]><r/>`,
		`<?xml version="1.0" standalone="yes"?><!DOCTYPE r [<!ENTITY % p '<!ENTITY e "text">'>%p;]><r>&e;</r>`,
		`<?xml version="1.0" standalone="yes"?><!DOCTYPE r [<!ENTITY % p '<!ENTITY e "text">'>%p;]><r a="&e;"/>`,
	} {
		_, err := Parse([]byte(bad))
		var f *fault.Error
		if !errors.As(err, &f) || f.Code != "XML_NOT_WELL_FORMED" {
			t.Fatal("declaration order/standalone WFC cannot be repaired by later declarations", bad, err)
		}
	}
}

func TestT1BFifthEditionNamesAndDefaultAttributeTextSource(t *testing.T) {
	input := []byte(`<!DOCTYPE r [<!ENTITY 🧩 "<🧪>virtual</🧪>"><!ATTLIST p 🧪 CDATA "default">]><r>&🧩;<p>literal</p><🧪:𐀀 xmlns:🧪="urn:test">source</🧪:𐀀></r>`)
	d, err := Parse(input)
	if err != nil {
		t.Fatal("XML Fifth Edition Names are not the older Go Appendix-B character table", err)
	}
	if d.Root.Children[0].Name.Local != "🧪" || d.Root.Children[2].Name.Space != "urn:test" || d.Root.Children[2].Name.Local != "𐀀" || d.Root.Children[1].Attributes[0].Name.Local != "🧪" {
		t.Fatal("lexical names/namespaces/default attributes were changed")
	}
	for _, i := range []int{1, 2} {
		e := d.Root.Children[i]
		out, changed, err := Replace(input, e, e.Text, "edited")
		want := bytes.Replace(input, []byte(">"+e.Text+"<"), []byte(">edited<"), 1)
		if err != nil || !changed || !bytes.Equal(out, want) {
			t.Fatal("real text interval stays writable despite a virtual default attribute", i, err)
		}
	}
	if _, _, err := Replace(input, d.Root.Children[0], "virtual", "edited"); err == nil {
		t.Fatal("generated name/markup does not create a writable raw source")
	}
}

func TestT1BNamespaceUnknownIsCapabilityNotInventedURI(t *testing.T) {
	for _, raw := range []string{
		`<!DOCTYPE r [%unread;]><r xmlns="&unknown;"/>`,
		`<!DOCTYPE r [%unread;]><p:r xmlns:p="&unknown;"/>`,
	} {
		_, err := Parse([]byte(raw))
		var f *fault.Error
		if !errors.As(err, &f) || f.Exit != 3 || f.Code != "XML_ENTITY_UNRESOLVED" {
			t.Fatal("necessary namespace cannot be fabricated or called malformed", err)
		}
	}
	for _, raw := range []string{
		`<p:1bad xmlns:p="urn:test"/>`, `<r xmlns:1bad="urn:test"/>`,
		`<p: xmlns:p="urn:test"/>`, `<:r/>`, `<r a:b:c="x"/>`,
	} {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Fatal("Namespace NCName component grammar is independent of XML Name", raw)
		}
	}
	for _, decl := range []string{`<!-- bounded -->`, `<?bounded data?>`} {
		for _, count := range []int{4096, 4097} {
			_, err := Parse([]byte(`<!DOCTYPE r [` + strings.Repeat(decl, count) + `]><r/>`))
			var f *fault.Error
			if count == 4096 && err != nil || count == 4097 && (!errors.As(err, &f) || f.Code != "XML_LIMIT") {
				t.Fatal("markupdecl budget includes comments/PI retained by the application", count, err)
			}
		}
	}
}

func TestT1BDocumentMiscIsLexicalBeforeExpansion(t *testing.T) {
	decl := `<!DOCTYPE r [<!ENTITY empty ""><!ENTITY space " ">]>`
	for _, extra := range []string{"\u00a0", `<![CDATA[ ]]>`, `&#10;`, `&empty;`, `&space;`} {
		for position, text := range []string{decl + extra + `<r/>`, decl + `<r/>` + extra} {
			for _, encoding := range []int{0, 1, 2} {
				t.Run(fmt.Sprintf("%x/%d/%d", extra, position, encoding), func(t *testing.T) {
					input := []byte(text)
					if encoding != 0 {
						var order binary.ByteOrder = binary.LittleEndian
						if encoding == 2 {
							order = binary.BigEndian
						}
						input = testUTF16(`<?xml version="1.0" encoding="UTF-16"?>`+text, order, true)
					}
					_, err := Parse(input)
					var f *fault.Error
					if !errors.As(err, &f) || f.Code != "XML_NOT_WELL_FORMED" {
						t.Fatal("document Misc is not decoded whitespace or vanished replacement", extra, encoding, err)
					}
				})
			}
		}
	}
	for _, text := range []string{
		" \t\r\n<!-- before --><?before data?>" + decl + `<r/>` + "\r\n<!-- after --><?after data?>\t ",
		decl + `<r>&empty;&space;&#10;<![CDATA[ ]]></r>`,
	} {
		if _, err := Parse([]byte(text)); err != nil {
			t.Fatal("legal Misc or the same lexical constructs inside the root must remain valid", err)
		}
	}
}

func FuzzT1BEntitySourceAndLocalReplacement(f *testing.F) {
	for _, seed := range []string{"asymmetric 😀", "< & quotes\"", "\r\n", "", "中"} {
		for _, encoding := range []uint8{0, 1, 2} {
			f.Add(seed, encoding)
		}
	}
	f.Fuzz(func(t *testing.T, text string, encoding uint8) {
		if len(text) > 8192 {
			text = text[:8192]
		}
		// Exercise valid documents rather than spending mutations on rejection.
		text = strings.ToValidUTF8(text, "�")
		text = strings.Map(func(r rune) rune {
			if !xmlChar(r) {
				return 'q'
			}
			return r
		}, text)
		var quoted bytes.Buffer
		// Entity construction is independently specified: literal & must be
		// bypassed as &amp;, while '<' must stay text through &lt;.
		for _, r := range text {
			switch r {
			case '&':
				quoted.WriteString("&amp;")
			case '<':
				quoted.WriteString("&lt;")
			case '>':
				quoted.WriteString("&gt;")
			case '\r':
				quoted.WriteString("&#13;")
			case '"':
				quoted.WriteString("&quot;")
			case '%':
				quoted.WriteString("&#37;")
			default:
				quoted.WriteRune(r)
			}
		}
		subset := `<!DOCTYPE r [<!ENTITY e "` + quoted.String() + `"><!ENTITY node "<p>virtual</p>">]>`
		prefix := subset + `<r><p>&e;</p><p>`
		input := []byte(prefix + "literal" + `</p>&node;</r>`)
		want := []byte(prefix + "changed" + `</p>&node;</r>`)
		replacement := "changed"
		if text == replacement {
			replacement = "different"
		}
		firstWant := []byte(subset + `<r><p>` + replacement + `</p><p>literal</p>&node;</r>`)
		if encoding%3 != 0 {
			var order binary.ByteOrder = binary.LittleEndian
			if encoding%3 == 2 {
				order = binary.BigEndian
			}
			decl := `<?xml version="1.0" encoding="UTF-16"?>`
			input = testUTF16(decl+string(input), order, true)
			want = testUTF16(decl+string(want), order, true)
			firstWant = testUTF16(decl+string(firstWant), order, true)
		}
		d, err := Parse(input)
		if err != nil || d.Root.Children[0].Text != text {
			t.Fatal("entity inclusion lost text", err)
		}
		out, changed, err := Replace(input, d.Root.Children[0], text, replacement)
		if err != nil || !changed || !bytes.Equal(out, firstWant) {
			t.Fatal("physical entity-text range changed unrelated source or was denied", err)
		}
		if _, _, err := Replace(input, d.Root.Children[2], "virtual", "changed"); err == nil {
			t.Fatal("entity-generated node became writable")
		}
		out, changed, err = Replace(input, d.Root.Children[1], "literal", "changed")
		if err != nil || !changed || !bytes.Equal(out, want) {
			t.Fatal("unrelated original sibling changed bytes or lost write permission", err)
		}
	})
}

func TestT1BNamespaceNamesInDeclarationsAndProcessingInstructions(t *testing.T) {
	for _, raw := range []string{
		`<!DOCTYPE r [<!ENTITY p:e "text">]><r/>`,
		`<!DOCTYPE r [<!ENTITY % p:e "">%p:e;]><r/>`,
		`<!DOCTYPE r [%p:e;]><r/>`,
		`<!DOCTYPE r [<!NOTATION p:n SYSTEM "urn:never-read">]><r/>`,
		`<!DOCTYPE r [<!ENTITY e SYSTEM "urn:never-read" NDATA p:n>]><r/>`,
		`<!DOCTYPE r [<!ATTLIST r kind NOTATION (p:n) #IMPLIED>]><r/>`,
		`<?p:target data?><r/>`,
		`<!DOCTYPE r [<?p:target data?>]><r/>`,
		`<!DOCTYPE r [<!ENTITY e "<?p:target data?>">]><r>&e;</r>`,
		`<!DOCTYPE r [%unread;]><r>&p:e;</r>`,
		`<!DOCTYPE r [<!ELEMENT p:a:b EMPTY>]><r/>`,
		`<!DOCTYPE r [<!ATTLIST r p:a:b CDATA #IMPLIED>]><r/>`,
	} {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Errorf("accepted invalid namespace name: %s", raw)
		}
	}
	// QName element/attribute names and colons in identifiers or NMTOKEN
	// values are not NCName violations. DTD validity is not being enforced.
	for _, raw := range []string{
		`<!DOCTYPE r [<!ELEMENT p:e EMPTY><!ATTLIST p:e p:a NMTOKEN "a:b"><!NOTATION n SYSTEM "urn:opaque:id">]><r xmlns:p="urn:p"><p:e/></r>`,
		`<!DOCTYPE r [<!ENTITY 🧪 "<?target data?>text"><!NOTATION 🧪 PUBLIC "notation:id">]><r>&🧪;</r>`,
	} {
		if _, err := Parse([]byte(raw)); err != nil {
			t.Errorf("rejected valid namespace-name control: %v: %s", err, raw)
		}
	}
}

func TestT1BPhysicalEntityTextLocalReplacement(t *testing.T) {
	subset := `<!DOCTYPE r [<!ENTITY empty ""><!ENTITY word "PE text"><!ENTITY nested "&word; + 中😀"><!ATTLIST p mode CDATA "default">]>`
	parameter := `<!DOCTYPE r [<!ENTITY % defs '<!ENTITY word "PE text"><!ENTITY nested "&word; + raw">'>%defs;]>`
	for _, tc := range []struct{ name, subset, body, old string }{
		{"entity-only", subset, "&word;", "PE text"},
		{"empty-only", subset, "&empty;", ""},
		{"empty-ends", subset, "&empty;前&nested;后&empty;", "前PE text + 中😀后"},
		{"parameter", parameter, "&nested; + raw", "PE text + raw + raw"},
	} {
		for _, encoding := range []uint8{0, 1, 2} {
			t.Run(fmt.Sprintf("%s/%d", tc.name, encoding), func(t *testing.T) {
				prefix := tc.subset + `<r><p id="physical">`
				suffix := `</p><p>&word;</p></r>`
				input, want := []byte(prefix+tc.body+suffix), []byte(prefix+"changed &amp; exact"+suffix)
				if encoding != 0 {
					var order binary.ByteOrder = binary.LittleEndian
					if encoding == 2 {
						order = binary.BigEndian
					}
					decl := `<?xml version="1.0" encoding="UTF-16"?>`
					input = testUTF16(decl+string(input), order, true)
					want = testUTF16(decl+string(want), order, true)
				}
				d, err := Parse(input)
				if err != nil {
					t.Fatal(err)
				}
				e := d.Root.Children[0]
				unchanged, changed, err := Replace(input, e, tc.old, tc.old)
				if err != nil || changed || !bytes.Equal(unchanged, input) {
					t.Fatal("no-op did not retain the exact references/defaults/BOM", err)
				}
				out, changed, err := Replace(input, e, tc.old, "changed & exact")
				if err != nil || !changed || !bytes.Equal(out, want) {
					t.Fatal("physical text range was denied or changed declarations/another reference", err)
				}
			})
		}
	}
}

func TestT1BVirtualOrNonSimpleEntityTargetsRemainUnwritable(t *testing.T) {
	for _, raw := range []string{
		`<!DOCTYPE r [<!ENTITY e "<p>virtual</p>">]><r>&e;</r>`,
		`<!DOCTYPE r [<!ENTITY e "<b>child</b>">]><r><p>&e;</p></r>`,
		`<!DOCTYPE r [<!ENTITY e "<!--comment-->">]><r><p>&e;</p></r>`,
		`<!DOCTYPE r [<!ENTITY e "<![CDATA[ ]]>">]><r><p>&e;</p></r>`,
		`<!DOCTYPE r [<!ENTITY e "<?target data?>">]><r><p>&e;</p></r>`,
		`<!DOCTYPE r [%unread;]><r><p>&unknown;</p></r>`,
	} {
		d, err := Parse([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		e := d.Root.Children[0]
		if _, _, err := Replace([]byte(raw), e, e.Text, "changed"); err == nil {
			t.Fatalf("virtual/unknown/non-simple target became writable: %s", raw)
		}
	}
}

func TestT1BMixedContentIsNotAChildContentParticle(t *testing.T) {
	for _, model := range []string{`(a,(#PCDATA))`, `((#PCDATA)*)`, `(a|(#PCDATA|em)*)`} {
		if _, err := Parse([]byte(`<!DOCTYPE r [<!ELEMENT unused ` + model + `>]><r/>`)); err == nil {
			t.Errorf("nested Mixed accepted as a cp despite unused declaration: %s", model)
		}
	}
	for _, model := range []string{`(#PCDATA|em)*`, `(#PCDATA)`, `(a,(b|c)+)`, `((a,b)*|c)+`} {
		if _, err := Parse([]byte(`<!DOCTYPE r [<!ELEMENT unused ` + model + `>]><r/>`)); err != nil {
			t.Errorf("valid contentspec/child model was rejected: %s: %v", model, err)
		}
	}
}
