package xmltext

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestT1BPublicIdentifierMatchingNormalization(t *testing.T) {
	for _, profile := range []struct{ media, public, system string }{
		{"image/svg+xml", "-//W3C//DTD SVG 1.1//EN", "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd"},
		{"application/x-dtbncx+xml", "-//NISO//DTD ncx 2005-1//EN", "http://www.daisy.org/z3986/2005/ncx-2005-1.dtd"},
		{"application/mathml+xml", "-//W3C//DTD MathML 3.0//EN", "http://www.w3.org/Math/DTD/mathml3/mathml3.dtd"},
	} {
		for _, encoding := range []uint8{0, 1, 2} {
			for _, variant := range []string{"canonical", "XML-space", "wrong-public", "system-space", "system-case", "tab", "illegal-public", "NBSP"} {
				pub, sys := profile.public, profile.system
				if variant != "canonical" {
					pub = " \r\n" + strings.ReplaceAll(pub, " ", "  \n") + "\r\n "
				}
				switch variant {
				case "wrong-public":
					pub = strings.Replace(pub, "DTD", "OTHER", 1)
				case "system-space":
					sys = " " + sys + " "
				case "system-case":
					sys = strings.Replace(sys, "http:", "HTTP:", 1)
				case "tab":
					pub += "\t"
				case "illegal-public":
					pub += "["
				case "NBSP":
					pub += "\u00a0"
				}
				raw := `<!DOCTYPE r PUBLIC "` + pub + `" "` + sys + `" [<!NOTATION n PUBLIC "  notation\n identifier  ">]><r/>`
				// Actual CR/LF, not backslash notation, in the application-visible ID.
				raw = strings.Replace(raw, `notation\n`, "notation\n", 1)
				input := []byte(raw)
				if encoding != 0 {
					var order binary.ByteOrder = binary.LittleEndian
					if encoding == 2 {
						order = binary.BigEndian
					}
					input = testUTF16(`<?xml version="1.0" encoding="UTF-16"?>`+raw, order, true)
				}
				doc, err := Parse(input)
				if variant == "tab" || variant == "illegal-public" || variant == "NBSP" {
					if err == nil {
						t.Fatalf("invalid PubidLiteral accepted: %s/%d/%s", profile.media, encoding, variant)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				err = doc.CheckProfile(Profile{Version: "3.0", MediaType: profile.media})
				allowed := variant == "canonical" || variant == "XML-space"
				if (err == nil) != allowed {
					t.Errorf("PUBLIC normalization or exact SYSTEM match: %s/%d/%s: %v", profile.media, encoding, variant, err)
				}
				if doc.dtd.publicID != pub || doc.dtd.systemID != sys || doc.Notations[0].PublicID != "  notation\n identifier  " {
					t.Fatal("matching mutated literal identifiers/application output")
				}
			}
		}
	}
}
