package publication

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

func moveDoc(t *testing.T, path, input string) *StructureDocument {
	t.Helper()
	doc, err := ParseStructureDocument([]byte(input), bookpath.BookPath(path), xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func moveOp(source, destination MoveEndpoint, position string) ElementMoveCross {
	return ElementMoveCross{Source: source, Destination: destination, Position: position}
}

func moveEndpoint(path, locator string) MoveEndpoint {
	return MoveEndpoint{BookPath: bookpath.BookPath(path), RevisionID: "initial", ResourceSHA256: strings.Repeat("0", 64), LocatorVersion: 1, Locator: locator}
}

// TestElementMoveCrossRebasesBlockURLs keeps the moved block byte-exact except
// for the URL values this batch rebases: other-resource links are re-based to
// the destination directory, a fragment that stays in the source resource is
// rewritten to the source, a fragment that moves with the block stays local, and
// external URLs are untouched.
func TestElementMoveCrossRebasesBlockURLs(t *testing.T) {
	source := moveDoc(t, "EPUB/a.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><body><div id="b"><p id="inner">x</p><a href="sub/other.xhtml?q=1#f">o</a><a href="#stay">s</a><a href="#inner">i</a><a href="https://example.com/x">e</a></div><p id="stay">y</p></body></html>`)
	dest := moveDoc(t, "EPUB/text/b.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="d">z</p></body></html>`)
	edit, err := source.ElementMoveCrossEdit(moveOp(moveEndpoint("EPUB/a.xhtml", "/html[1]/body[1]/div[1]"), moveEndpoint("EPUB/text/b.xhtml", "/html[1]/body[1]/p[1]"), "after"), dest)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(edit.Block), `<div id="b"><p id="inner">x</p><a href="../sub/other.xhtml?q=1#f">o</a><a href="../a.xhtml#stay">s</a><a href="#inner">i</a><a href="https://example.com/x">e</a></div>`; got != want {
		t.Fatalf("block:\n got %s\nwant %s", got, want)
	}
	if len(edit.MovedIDs) != 2 || edit.MovedIDs[0] != "b" || edit.MovedIDs[1] != "inner" {
		t.Fatalf("moved ids: %v", edit.MovedIDs)
	}
	if len(edit.Rewrites) != 2 {
		t.Fatalf("rewrites: %+v", edit.Rewrites)
	}
	if edit.Rewrites[0].Old != "sub/other.xhtml?q=1#f" || edit.Rewrites[0].New != "../sub/other.xhtml?q=1#f" {
		t.Fatalf("rewrite[0]: %+v", edit.Rewrites[0])
	}
	if edit.Rewrites[1].Old != "#stay" || edit.Rewrites[1].New != "../a.xhtml#stay" {
		t.Fatalf("rewrite[1]: %+v", edit.Rewrites[1])
	}
	if len(edit.Links) != 4 {
		t.Fatalf("links: %+v", edit.Links)
	}
	// Every URL that moves with the block is a gate fact in its final form,
	// including the local fragment and the external URL this batch does not
	// rewrite.
	if edit.Links[0].Value != "../sub/other.xhtml?q=1#f" || edit.Links[1].Value != "../a.xhtml#stay" || edit.Links[2].Value != "#inner" || edit.Links[3].Value != "https://example.com/x" {
		t.Fatalf("link facts: %+v", edit.Links)
	}
	if edit.SourceSpan.Start != bytes.Index(source.Input, []byte(`<div id="b">`)) || string(source.Input[edit.SourceSpan.Start:edit.SourceSpan.End]) != `<div id="b"><p id="inner">x</p><a href="sub/other.xhtml?q=1#f">o</a><a href="#stay">s</a><a href="#inner">i</a><a href="https://example.com/x">e</a></div>` {
		t.Fatalf("source span: %d %d", edit.SourceSpan.Start, edit.SourceSpan.End)
	}
}

// TestElementMoveCrossRefusals keeps the conservative boundaries: a same-document
// IDREF cannot leave the block, a namespace context mismatch is refused, and
// block content the fragment rules do not insert is refused before any byte is
// written.
func TestElementMoveCrossRefusals(t *testing.T) {
	dest := moveDoc(t, "EPUB/text/b.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="d">z</p></body></html>`)
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{"idref", `<html xmlns="http://www.w3.org/1999/xhtml"><body><div id="b"><p for="stay">x</p></div><p id="stay">y</p></body></html>`, "IDREF"},
		{"namespace", `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body><div id="b" epub:type="note">x</div></body></html>`, "namespace"},
		{"denied-attribute", `<html xmlns="http://www.w3.org/1999/xhtml"><body><div id="b" style="color:red">x</div></body></html>`, "style"},
		{"srcset", `<html xmlns="http://www.w3.org/1999/xhtml"><body><div id="b"><img srcset="a.png 1x"/></div></body></html>`, "srcset"},
		{"dangling-fragment", `<html xmlns="http://www.w3.org/1999/xhtml"><body><div id="b"><a href="#missing">x</a></div></body></html>`, "does not resolve"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := moveDoc(t, "EPUB/a.xhtml", c.source)
			_, err := source.ElementMoveCrossEdit(moveOp(moveEndpoint("EPUB/a.xhtml", "/html[1]/body[1]/div[1]"), moveEndpoint("EPUB/text/b.xhtml", "/html[1]/body[1]/p[1]"), "after"), dest)
			if err == nil {
				t.Fatal("move accepted")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Fatalf("want %q in error, got %v", c.want, err)
			}
		})
	}
}

// TestElementMoveCrossSameResourceAndEncoding keeps the same-resource refusal and
// the physical re-encoding: the block's decoded text is the same in both
// encodings while only the destination's bytes are produced.
func TestElementMoveCrossSameResourceAndEncoding(t *testing.T) {
	source := moveDoc(t, "EPUB/a.xhtml", `<html xmlns="http://www.w3.org/1999/xhtml"><body><div id="b"><p>x</p></div></body></html>`)
	if _, err := source.ElementMoveCrossEdit(moveOp(moveEndpoint("EPUB/a.xhtml", "/html[1]/body[1]/div[1]"), moveEndpoint("EPUB/a.xhtml", "/html[1]/body[1]/div[1]"), "after"), source); err == nil {
		t.Fatal("same-resource move accepted")
	}
	utf16 := []byte{0xff, 0xfe}
	for _, r := range `<html xmlns="http://www.w3.org/1999/xhtml"><body><p id="d">z</p></body></html>` {
		utf16 = append(utf16, byte(r), byte(r>>8))
	}
	dest, err := ParseStructureDocument(utf16, "EPUB/text/b.xhtml", xmltext.Profile{Version: "3.0", MediaType: "application/xhtml+xml"})
	if err != nil {
		t.Fatal(err)
	}
	edit, err := source.ElementMoveCrossEdit(moveOp(moveEndpoint("EPUB/a.xhtml", "/html[1]/body[1]/div[1]"), moveEndpoint("EPUB/text/b.xhtml", "/html[1]/body[1]/p[1]"), "after"), dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(edit.Block, []byte{'<', 0, 'd', 0}) {
		t.Fatalf("block is not UTF-16LE: %q", edit.Block)
	}
}
