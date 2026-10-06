package publication

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

type XMLCoverage struct {
	Resources []XMLResourceCoverage `json:"resources"`
}

type XMLResourceCoverage struct {
	BookPath       bookpath.BookPath    `json:"bookPath"`
	ResourceSHA256 string               `json:"resourceSha256"`
	Status         string               `json:"status"`
	Unresolved     []xmltext.Unresolved `json:"unresolved"`
	Notations      []xmltext.Notation   `json:"notations"`
}

func xmlCoverage(bp bookpath.BookPath, input []byte, doc *xmltext.Document) *XMLCoverage {
	if len(doc.Unresolved) == 0 && len(doc.Notations) == 0 {
		return nil
	}
	h := sha256.Sum256(input)
	r := XMLResourceCoverage{BookPath: bp, ResourceSHA256: hex.EncodeToString(h[:]), Status: "complete", Unresolved: []xmltext.Unresolved{}, Notations: []xmltext.Notation{}}
	r.Unresolved = append(r.Unresolved, doc.Unresolved...)
	r.Notations = append(r.Notations, doc.Notations...)
	if len(doc.Unresolved) != 0 {
		r.Status = "partial"
	}
	return &XMLCoverage{Resources: []XMLResourceCoverage{r}}
}

// Merge reports only resources already parsed by this request, without sharing
// mutable accumulation with an info result or silently widening its read scope.
func (c *XMLCoverage) Merge(other *XMLCoverage) *XMLCoverage {
	if other == nil {
		return c
	}
	out := &XMLCoverage{}
	if c != nil {
		out.Resources = slices.Clone(c.Resources)
	}
	for _, r := range other.Resources {
		i := slices.IndexFunc(out.Resources, func(old XMLResourceCoverage) bool { return old.BookPath == r.BookPath })
		if i < 0 {
			out.Resources = append(out.Resources, r)
		} else {
			out.Resources[i] = r
		}
	}
	slices.SortFunc(out.Resources, func(a, b XMLResourceCoverage) int {
		if a.BookPath < b.BookPath {
			return -1
		}
		if a.BookPath > b.BookPath {
			return 1
		}
		return 0
	})
	return out
}
