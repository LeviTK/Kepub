package publication

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

// Field order is the content.text.set v1 canonical JSON encoding.
type TextSet struct {
	BookPath         bookpath.BookPath `json:"bookPath"`
	RevisionID       string            `json:"revisionId"`
	ResourceSHA256   string            `json:"resourceSha256"`
	LocatorVersion   int               `json:"locatorVersion"`
	Locator          string            `json:"locator"`
	ExpectedOldValue string            `json:"expectedOldValue"`
	NewValue         string            `json:"newValue"`
}

func (s TextSet) Validate() error {
	if _, err := bookpath.Parse(string(s.BookPath)); err != nil {
		return err
	}
	if s.RevisionID == "" || !utf8.ValidString(s.RevisionID) {
		return fmt.Errorf("revisionId is required")
	}
	h, err := hex.DecodeString(s.ResourceSHA256)
	if err != nil || len(h) != 32 || hex.EncodeToString(h) != s.ResourceSHA256 {
		return fmt.Errorf("resourceSha256 must be lowercase SHA-256")
	}
	if s.LocatorVersion != ContentLocatorVersion || s.Locator == "" || len(s.Locator) > 4096 || !utf8.ValidString(s.Locator) {
		return fmt.Errorf("unsupported or invalid locator")
	}
	if len(s.ExpectedOldValue) > ContentTextLimit || len(s.NewValue) > ContentTextLimit || !utf8.ValidString(s.ExpectedOldValue) || !utf8.ValidString(s.NewValue) {
		return fmt.Errorf("text exceeds 1 MiB or is not UTF-8")
	}
	for _, r := range s.NewValue {
		if !(r == 9 || r == 10 || r == 13 || r >= 0x20 && r <= 0xD7FF || r >= 0xE000 && r <= 0xFFFD || r >= 0x10000 && r <= 0x10FFFF) {
			return fmt.Errorf("new value is not XML text")
		}
	}
	return nil
}

// ApplyText requires the caller to verify revisionId against its locked accepted
// revision. Resource hash, manifest selection and byte intervals are rederived.
func ApplyText(a ResourceReader, p *Publication, s TextSet) ([]byte, bool, error) {
	if err := s.Validate(); err != nil {
		return nil, false, err
	}
	if err := checkTextTarget(p, s.BookPath); err != nil {
		return nil, false, err
	}
	input, err := a.Read(s.BookPath, XMLLimit)
	if err != nil {
		return nil, false, fmt.Errorf("read target: %v", err)
	}
	if err := CheckTextResourceHash(input, s); err != nil {
		return nil, false, err
	}
	return applyTextTarget(input, p, s)
}

// CheckTextResourceHash verifies that input is the frozen resource the v1
// operation was bound to. It decides nothing about target support or content.
func CheckTextResourceHash(input []byte, s TextSet) error {
	h := sha256.Sum256(input)
	if hex.EncodeToString(h[:]) != s.ResourceSHA256 {
		return fault.New(4, "INPUT_DRIFT", "content resource hash changed")
	}
	return nil
}

// ApplyTextAt applies s to caller-supplied bytes already bound to the frozen
// baseline resource hash. Manifest selection, target support and the expected
// old value are rederived from input, never trusted from the request.
func ApplyTextAt(input []byte, p *Publication, s TextSet) ([]byte, bool, error) {
	if err := s.Validate(); err != nil {
		return nil, false, err
	}
	return applyTextTarget(input, p, s)
}

func applyTextTarget(input []byte, p *Publication, s TextSet) ([]byte, bool, error) {
	if err := checkTextTarget(p, s.BookPath); err != nil {
		return nil, false, err
	}
	e, err := simpleTextElement(input, s.Locator, xmltext.Profile{Version: p.Version, MediaType: "application/xhtml+xml"})
	if err != nil {
		return nil, false, err
	}
	return xmltext.Replace(input, e, s.ExpectedOldValue, s.NewValue)
}

// CheckXHTMLTarget exposes the single-operation manifest permission boundary so
// structural operations cannot widen it.
func CheckXHTMLTarget(p *Publication, bp bookpath.BookPath) error {
	return checkTextTarget(p, bp)
}

func checkTextTarget(p *Publication, bp bookpath.BookPath) error {
	found := false
	for _, item := range p.Manifest {
		if item.Path == bp {
			found = true
			if item.MediaType != "application/xhtml+xml" {
				return fmt.Errorf("target must be manifest XHTML")
			}
		}
	}
	if !found {
		return fmt.Errorf("target is not in selected manifest")
	}
	return nil
}

// ContentText observes the actual candidate simple-text target for review. It
// deliberately does not require the old hash or planned value to still match.
func ContentText(input []byte, locator string, profile xmltext.Profile) (string, error) {
	e, err := simpleTextElement(input, locator, profile)
	if err != nil {
		return "", err
	}
	return e.Text, nil
}

func simpleTextElement(input []byte, locator string, profile xmltext.Profile) (*xmltext.Element, error) {
	doc, err := xmltext.Parse(input)
	if err != nil {
		return nil, err
	}
	if err := doc.CheckProfile(profile); err != nil {
		return nil, err
	}
	return simpleTextElementIn(doc, locator)
}

// simpleTextElementIn applies the v1 simple-text rules to an already parsed and
// profile-checked document.
func simpleTextElementIn(doc *xmltext.Document, locator string) (*xmltext.Element, error) {
	if err := doc.RequireComplete(); err != nil {
		return nil, err
	}
	if doc.Root.Name != (xml.Name{Space: XHTMLNamespace, Local: "html"}) {
		return nil, fmt.Errorf("expected XHTML html root")
	}
	var body, target *xmltext.Element
	bodies := 0
	for _, e := range doc.Elements {
		if e.Name == (xml.Name{Space: XHTMLNamespace, Local: "body"}) {
			body = e
			bodies++
		}
		if e.Location == locator {
			target = e
		}
	}
	if bodies != 1 || body.Parent != doc.Root || target == nil || target == body || target.Complex {
		return nil, fmt.Errorf("missing or unsupported simple-text target/body")
	}
	for e := target; e != nil; e = e.Parent {
		if e.Name.Space != XHTMLNamespace || e.Name.Local == "head" || e.Name.Local == "script" || e.Name.Local == "style" {
			return nil, fmt.Errorf("excluded target ancestor")
		}
		if e == body {
			return target, nil
		}
	}
	return nil, fmt.Errorf("target is outside body")
}
