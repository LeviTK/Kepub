// Package bookpath keeps container paths distinct from raw URL references.
package bookpath

import (
	"net/url"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/fault"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

type BookPath string
type Href string

// Reference separates URL components from the exact container filename. Raw
// spelling belongs to the caller's Href; Query and Fragment are never filenames.
// ForceQuery records an empty query written as "?" so a rewritten reference can
// keep the same meaning.
type Reference struct {
	Path       BookPath `json:"bookPath"`
	Fragment   string   `json:"fragment"`
	Query      string   `json:"query"`
	ForceQuery bool     `json:"forceQuery,omitempty"`
	External   bool     `json:"external"`
}

// ResolveReference also accepts same-document and external URLs. External URLs
// are classified only, never fetched. Manifest loading retains its stricter API.
func ResolveReference(base BookPath, h Href) (Reference, error) {
	if _, err := Parse(string(base)); err != nil {
		return Reference{}, err
	}
	u, err := url.Parse(string(h))
	if err != nil || u == nil {
		return Reference{}, fault.New(1, "INVALID_HREF", "invalid URL reference %q", h)
	}
	r := Reference{Fragment: u.Fragment, Query: u.RawQuery, ForceQuery: u.ForceQuery}
	if u.IsAbs() || u.Host != "" || u.Opaque != "" || strings.HasPrefix(string(h), "//") {
		r.External = true
		return r, nil
	}
	if u.Path == "" {
		r.Path = base
		return r, nil
	}
	r.Path, err = Resolve(base, h)
	return r, err
}

func Parse(s string) (BookPath, error) {
	if len(s) > 4096 || strings.Count(s, "/") >= 128 {
		return "", fault.New(1, "PATH_LIMIT", "container path exceeds 4096 bytes or 128 components")
	}
	if s == "" || !utf8.ValidString(s) || strings.HasPrefix(s, "/") || strings.ContainsAny(s, "\\:") {
		return "", fault.New(1, "UNSAFE_PATH", "invalid container path %q", s)
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return "", fault.New(1, "UNSAFE_PATH", "control character in path")
		}
	}
	for _, p := range strings.Split(s, "/") {
		if p == "" || p == "." || p == ".." {
			return "", fault.New(1, "UNSAFE_PATH", "noncanonical container path %q", s)
		}
	}
	return BookPath(s), nil
}

func CollisionKey(p BookPath) string {
	return norm.NFC.String(cases.Fold().String(norm.NFC.String(string(p))))
}

// Resolve decodes once, preserving Href separately. Parent segments are allowed
// only when they stay inside the container. Fragment and query are not filenames.
func Resolve(base BookPath, h Href) (BookPath, error) {
	u, err := url.Parse(string(h))
	if err != nil || u == nil || u.IsAbs() || u.Host != "" || u.Opaque != "" {
		return "", fault.New(1, "UNSUPPORTED_HREF", "manifest href must be a local reference: %q", h)
	}
	decoded := u.Path
	if decoded == "" || strings.HasPrefix(decoded, "/") || strings.ContainsAny(decoded, "\\:") {
		return "", fault.New(1, "UNSAFE_PATH", "invalid href %q", h)
	}
	return Parse(path.Join(path.Dir(string(base)), decoded))
}

// RelativeHref renders the canonical relative URL from the directory of one
// resource to another BookPath. It only generates rewritten references, so the
// escaping is canonical: every segment is escaped as a URL path segment, and
// parent segments are emitted only while they stay inside the container.
func RelativeHref(from, to BookPath) (Href, error) {
	if _, err := Parse(string(from)); err != nil {
		return "", err
	}
	if _, err := Parse(string(to)); err != nil {
		return "", err
	}
	fromDir := path.Dir(string(from))
	fromSegments := []string{}
	if fromDir != "." && fromDir != "/" {
		fromSegments = strings.Split(fromDir, "/")
	}
	toSegments := strings.Split(string(to), "/")
	common := 0
	for common < len(fromSegments) && common < len(toSegments)-1 && fromSegments[common] == toSegments[common] {
		common++
	}
	segments := []string{}
	for i := common; i < len(fromSegments); i++ {
		segments = append(segments, "..")
	}
	for _, segment := range toSegments[common:] {
		segments = append(segments, url.PathEscape(segment))
	}
	return Href(strings.Join(segments, "/")), nil
}
