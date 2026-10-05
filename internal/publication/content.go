package publication

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"strings"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
)

const ContentLocatorVersion = 1
const ContentTextLimit = 1 << 20

// Nil means omitted; explicit empty query and zero limit are invalid.
type ContentOptions struct {
	Query *string
	Limit *int
}

// Validate can be called before opening a workspace or acquiring its lock.
func (o ContentOptions) Validate() error {
	if o.Query != nil && (*o.Query == "" || len(*o.Query) > 4096 || !utf8.ValidString(*o.Query)) {
		return fault.New(2, "INVALID_CONTENT_QUERY", "query must contain 1–4096 UTF-8 bytes")
	}
	if o.Limit != nil && (*o.Limit < 1 || *o.Limit > 200) {
		return fault.New(2, "INVALID_CONTENT_LIMIT", "limit must be between 1 and 200")
	}
	return nil
}

type ContentNode struct {
	Namespace        string  `json:"namespace"`
	LocalName        string  `json:"localName"`
	ID               *string `json:"id,omitempty"`
	Locator          string  `json:"locator"`
	Text             string  `json:"text"`
	HasChildElements bool    `json:"hasChildElements"`
}

type Content struct {
	BookPath       bookpath.BookPath `json:"bookPath"`
	ResourceSHA256 string            `json:"resourceSha256"`
	LocatorVersion int               `json:"locatorVersion"`
	MatchedCount   int               `json:"matchedCount"`
	ReturnedCount  int               `json:"returnedCount"`
	Truncated      bool              `json:"truncated"`
	Nodes          []ContentNode     `json:"nodes"`
}

// ReadContent extracts bounded text from one exact manifest XHTML resource.
// This is neither full-text search nor EPUB conformance checking. Mixed content
// and locators are read-only observations, not editing permissions or offsets.
func ReadContent(a *archive.Archive, p *Publication, resource bookpath.BookPath, o ContentOptions) (Content, error) {
	if err := o.Validate(); err != nil {
		return Content{}, err
	}
	if _, err := bookpath.Parse(string(resource)); err != nil {
		return Content{}, err
	}
	found := false
	for _, item := range p.Manifest {
		if item.Path == resource {
			found = true
			if item.MediaType != "application/xhtml+xml" {
				return Content{}, fault.New(3, "UNSUPPORTED_CONTENT_TYPE", "content requires manifest application/xhtml+xml")
			}
		}
	}
	if !found {
		return Content{}, fault.New(2, "CONTENT_RESOURCE_NOT_DECLARED", "exact resource path is not in the selected manifest")
	}
	data, err := a.Read(resource, XMLLimit)
	if err != nil {
		return Content{}, err
	}
	root, err := parseXML(data)
	if err != nil {
		return Content{}, err
	}
	if root.Name != (xml.Name{Space: XHTMLNamespace, Local: "html"}) {
		return Content{}, fault.New(1, "CONTENT_STRUCTURE", "expected XHTML html root")
	}
	bodies := root.children(XHTMLNamespace, "body")
	bodyCount := 0
	var countBodies func(*Element)
	countBodies = func(e *Element) {
		if e.Name == (xml.Name{Space: XHTMLNamespace, Local: "body"}) {
			bodyCount++
		}
		for _, c := range e.Children {
			countBodies(c)
		}
	}
	countBodies(root)
	if len(bodies) != 1 || bodyCount != 1 {
		return Content{}, fault.New(1, "CONTENT_STRUCTURE", "expected one direct XHTML body and no nested or duplicate body")
	}
	limit := 50
	if o.Limit != nil {
		limit = *o.Limit
	}
	hash := sha256.Sum256(data)
	result := Content{BookPath: resource, ResourceSHA256: hex.EncodeToString(hash[:]), LocatorVersion: ContentLocatorVersion, Nodes: []ContentNode{}}
	// Mark ancestors too: their Content contains excluded descendants. Continue
	// into supported siblings/children, but never traverse an excluded subtree.
	blocked := map[*Element]bool{}
	var mark func(*Element) bool
	mark = func(e *Element) bool {
		if e.Name.Space != XHTMLNamespace || e.Name.Local == "script" || e.Name.Local == "style" || e.Name.Local == "head" {
			blocked[e] = true
			return true
		}
		for _, c := range e.Children {
			if mark(c) {
				blocked[e] = true
			}
		}
		return blocked[e]
	}
	mark(bodies[0])
	textBytes := 0
	var walk func(*Element) error
	walk = func(e *Element) error {
		if e.Name.Space != XHTMLNamespace || e.Name.Local == "script" || e.Name.Local == "style" || e.Name.Local == "head" {
			return nil
		}
		if e.Name.Local != "body" && !blocked[e] && (len(e.Children) == 0 || strings.TrimSpace(e.Text) != "") && (o.Query == nil || strings.Contains(e.Content, *o.Query)) {
			result.MatchedCount++
			if len(result.Nodes) < limit {
				textBytes += len(e.Content)
				if textBytes > ContentTextLimit {
					return fault.New(1, "CONTENT_LIMIT", "returned text exceeds 1 MiB")
				}
				node := ContentNode{Namespace: e.Name.Space, LocalName: e.Name.Local, Locator: e.Location, Text: e.Content, HasChildElements: len(e.Children) != 0}
				if id, ok := e.Attribute("", "id"); ok {
					node.ID = &id
				}
				result.Nodes = append(result.Nodes, node)
			}
		}
		for _, c := range e.Children {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(bodies[0]); err != nil {
		return Content{}, err
	}
	result.ReturnedCount = len(result.Nodes)
	result.Truncated = result.ReturnedCount < result.MatchedCount
	return result, nil
}
