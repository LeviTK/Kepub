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
	"github.com/LeviTK/Kepub/internal/xmltext"
)

const ContentLocatorVersion = 1
const ContentTextLimit = 1 << 20

// Nil means omitted; explicit empty query and zero limit are invalid.
type ContentOptions struct {
	Query *string
	Limit *int

	// Per-call test observation; no global counter or persistent cache.
	onManifestLookup func()
	// Tests may lower the existing raw scan budget without changing the wire.
	scanLimit int64
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
	limit := 50
	if o.Limit != nil {
		limit = *o.Limit
	}
	return readContent(a, p, resource, o, limit, nil)
}

// A zero return limit still parses and counts the entire resource for search.
// A search supplies its call-local manifest index; a single read keeps the
// original exact membership check, including conflicting duplicate declarations.
func readContent(a *archive.Archive, p *Publication, resource bookpath.BookPath, o ContentOptions, limit int, targets map[bookpath.BookPath]bool) (Content, error) {
	if _, err := bookpath.Parse(string(resource)); err != nil {
		return Content{}, err
	}
	found, xhtml := false, true
	if targets != nil {
		if o.onManifestLookup != nil {
			o.onManifestLookup()
		}
		xhtml, found = targets[resource]
	} else {
		for _, item := range p.Manifest {
			if o.onManifestLookup != nil {
				o.onManifestLookup()
			}
			if item.Path == resource {
				found = true
				if item.MediaType != "application/xhtml+xml" {
					xhtml = false
					break
				}
			}
		}
	}
	if found && !xhtml {
		return Content{}, fault.New(3, "UNSUPPORTED_CONTENT_TYPE", "content requires manifest application/xhtml+xml")
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
	if err := root.XMLDocument.CheckProfile(xmltext.Profile{Version: p.Version, MediaType: "application/xhtml+xml"}); err != nil {
		return Content{}, err
	}
	if err := root.XMLDocument.RequireComplete(); err != nil {
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

const SearchScanLimit = 128 << 20

type SearchResult struct {
	BookPath       bookpath.BookPath `json:"bookPath"`
	ResourceSHA256 string            `json:"resourceSha256"`
	LocatorVersion int               `json:"locatorVersion"`
	ContentNode
}

type Search struct {
	MatchedCount  int            `json:"matchedCount"`
	ReturnedCount int            `json:"returnedCount"`
	Truncated     bool           `json:"truncated"`
	Results       []SearchResult `json:"results"`
}

// SearchContent observes every selected manifest XHTML, even after the return
// limit. A bad later resource cannot produce a successful incomplete count.
func SearchContent(a *archive.Archive, p *Publication, o ContentOptions) (Search, error) {
	if err := o.Validate(); err != nil {
		return Search{}, err
	}
	if o.Query == nil {
		return Search{}, fault.New(2, "INVALID_CONTENT_QUERY", "search requires query")
	}
	limit := 50
	if o.Limit != nil {
		limit = *o.Limit
	}
	// Aggregate all declarations, never last-wins: any non-XHTML declaration
	// keeps that exact path unsupported. Still iterate the original manifest
	// below so duplicates retain their original read/count/order semantics.
	targets := make(map[bookpath.BookPath]bool, len(p.Manifest))
	for _, item := range p.Manifest {
		if o.onManifestLookup != nil {
			o.onManifestLookup()
		}
		previous, exists := targets[item.Path]
		targets[item.Path] = item.MediaType == "application/xhtml+xml" && (!exists || previous)
	}
	scanLimit := int64(SearchScanLimit)
	if o.scanLimit > 0 {
		scanLimit = min(scanLimit, o.scanLimit)
	}
	out := Search{Results: []SearchResult{}}
	var scanned int64
	returnedBytes := 0
	for _, item := range p.Manifest {
		if item.MediaType != "application/xhtml+xml" {
			continue
		}
		scanned += a.Files[item.Path]
		if scanned > scanLimit {
			return Search{}, fault.New(1, "CONTENT_LIMIT", "XHTML raw scan exceeds 128 MiB")
		}
		c, err := readContent(a, p, item.Path, o, limit-len(out.Results), targets)
		if err != nil {
			return Search{}, err
		}
		out.MatchedCount += c.MatchedCount
		for _, n := range c.Nodes {
			returnedBytes += len(n.Text)
			if returnedBytes > ContentTextLimit {
				return Search{}, fault.New(1, "CONTENT_LIMIT", "search returned text exceeds 1 MiB")
			}
			out.Results = append(out.Results, SearchResult{item.Path, c.ResourceSHA256, c.LocatorVersion, n})
		}
	}
	out.ReturnedCount = len(out.Results)
	out.Truncated = out.ReturnedCount < out.MatchedCount
	return out, nil
}
