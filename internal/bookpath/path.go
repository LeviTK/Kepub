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
