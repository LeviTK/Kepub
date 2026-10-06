package testfixture

import (
	"encoding/binary"
	"unicode/utf16"
)

// UTF16 builds independent original-byte fixtures/expectations for integration
// tests. It never calls the production decoding or replacement implementation.
func UTF16(text string, bigEndian, bom bool) []byte {
	var order binary.ByteOrder = binary.LittleEndian
	if bigEndian {
		order = binary.BigEndian
	}
	units := utf16.Encode([]rune(text))
	if bom {
		units = append([]uint16{0xfeff}, units...)
	}
	out := make([]byte, 2*len(units))
	for i, u := range units {
		order.PutUint16(out[2*i:], u)
	}
	return out
}
