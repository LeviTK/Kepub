package workspace

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/metadata"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

// Schema8 resolves every operation against the frozen revision. Only after all
// bindings and target facts are known do we budget and splice complete resources.
func (w *Workspace) recomputeCoreMetadata(a publicationRoot, ops []Operation, revision string) (derivation, error) {
	p, err := publication.Load(a, w.state.Rootfile)
	if err != nil {
		return derivation{}, err
	}
	type set struct {
		element *xmltext.Element
		value   string
		index   int
	}
	type group struct {
		base []byte
		sets []set
		doc  *publication.StructureDocument
	}
	groups := map[string]*group{}
	seen := map[string]bool{}
	for i, op := range ops {
		if err := w.resources.ctx.Err(); err != nil {
			return derivation{}, err
		}
		bp := bookpath.BookPath(w.state.Rootfile)
		if s, ok := op.Params.(publication.TextSet); ok {
			bp = s.BookPath
		}
		g := groups[string(bp)]
		if g == nil {
			b, err := a.Read(bp, publication.XMLLimit)
			if err != nil {
				return derivation{}, err
			}
			g = &group{base: b}
			groups[string(bp)] = g
		}
		var e *xmltext.Element
		var old, value string
		switch s := op.Params.(type) {
		case metadata.SetV2:
			if string(s.BookPath) != w.state.Rootfile {
				return derivation{}, fmt.Errorf("metadata bookPath is not selected rootfile")
			}
			if s.RevisionID != revision {
				return derivation{}, ErrStalePlan
			}
			if err := checkStructureHash(g.base, s.ResourceSHA256); err != nil {
				return derivation{}, err
			}
			e, err = metadata.SelectV2(g.base, s)
			old, value = s.ExpectedOldValue, s.NewValue
		case metadata.Set:
			var location string
			location, err = metadata.Select(g.base, s)
			if err == nil {
				var d *xmltext.Document
				d, err = xmltext.Parse(g.base)
				if err == nil {
					for _, target := range d.Elements {
						if target.Location == location {
							e = target
							break
						}
					}
				}
			}
			old, value = s.ExpectedOldValue, s.NewValue
		case publication.TextSet:
			if s.RevisionID != revision {
				return derivation{}, ErrStalePlan
			}
			if err := publication.CheckTextResourceHash(g.base, s); err != nil {
				return derivation{}, err
			}
			if err := publication.CheckXHTMLTarget(p, bp); err != nil {
				return derivation{}, err
			}
			if g.doc == nil {
				g.doc, err = publication.ParseStructureDocument(g.base, bp, xmltext.Profile{Version: p.Version, MediaType: "application/xhtml+xml"})
				if err != nil {
					return derivation{}, err
				}
			}
			// Reuse v1's complete simple-text authorization without allocating
			// replacement bytes before the final resource budget is known.
			_, err = g.doc.TextSetEdit(s, s.ExpectedOldValue)
			if err == nil {
				e, err = g.doc.Locate(s.Locator)
			}
			old, value = s.ExpectedOldValue, s.NewValue
		default:
			return derivation{}, fmt.Errorf("unsupported schema8 operation")
		}
		if err != nil {
			return derivation{}, err
		}
		if e == nil || e.Complex || e.Text != old {
			return derivation{}, fmt.Errorf("complex metadata/text target or old value mismatch")
		}
		key := string(bp) + "\x00" + e.Location
		if seen[key] {
			return derivation{}, fmt.Errorf("duplicate metadata/text target %s", e.Location)
		}
		seen[key] = true
		g.sets = append(g.sets, set{e, value, i})
	}
	inv, err := inventory(a.root)
	if err != nil {
		return derivation{}, err
	}
	var total int64
	for _, size := range inv {
		total += size
	}
	// Count all disjoint removals before additions, even when a growing
	// operation precedes a shrinking one. No intermediate OPF is authorized.
	for _, g := range groups {
		n := int64(len(g.base))
		for _, s := range g.sets {
			if s.value != s.element.Text {
				start, end, ok := s.element.PhysicalContent()
				if !ok {
					return derivation{}, fmt.Errorf("target has no physical text interval")
				}
				n -= int64(end - start)
			}
		}
		for _, s := range g.sets {
			if s.value != s.element.Text {
				size, err := s.element.ReplaceBytesSize(s.value)
				if err != nil {
					return derivation{}, err
				}
				n += size
			}
		}
		if n > publication.XMLLimit {
			return derivation{}, fault.New(1, "RESOURCE_LIMIT", "schema8 final XML resource exceeds 8 MiB")
		}
		total += n - int64(len(g.base))
	}
	if err := w.resources.ctx.Err(); err != nil {
		return derivation{}, err
	}
	if total > w.resources.limits.TotalBytes {
		return derivation{}, fault.New(1, "ARCHIVE_LIMIT", "schema8 final publication exceeds total byte budget")
	}
	d := derivation{outputs: map[string][]byte{}, writes: []string{}, edits: map[string][]*publication.StructureEdit{}}
	for bp, g := range groups {
		for _, s := range g.sets {
			ed := &publication.StructureEdit{OpIndex: s.index, Change: publication.StructureChange{Kind: "text-set", Locator: s.element.Location, Text: s.value}}
			if s.value != s.element.Text {
				value, err := s.element.ReplaceBytes(s.value)
				if err != nil {
					return derivation{}, err
				}
				start, end, _ := s.element.PhysicalContent()
				ed.Spans = []publication.EditSpan{{Start: start, End: end, Bytes: value}}
			}
			d.edits[bp] = append(d.edits[bp], ed)
		}
		if err := publication.ValidateEdits(d.edits[bp]); err != nil {
			return derivation{}, err
		}
		out := publication.ApplyEdits(g.base, d.edits[bp])
		check, err := xmltext.Parse(out)
		if err != nil {
			return derivation{}, err
		}
		for _, s := range g.sets {
			found := false
			for _, e := range check.Elements {
				if e.Location == s.element.Location && e.Name == s.element.Name && !e.Complex && e.Text == s.value {
					found = true
					break
				}
			}
			if !found {
				return derivation{}, fmt.Errorf("schema8 replacement text mismatch")
			}
		}
		if g.doc != nil {
			if err := publication.VerifyStructure(g.doc, d.edits[bp], out); err != nil {
				return derivation{}, err
			}
		}
		if !bytes.Equal(g.base, out) {
			d.outputs[bp] = out
			d.writes = append(d.writes, bp)
		}
	}
	slices.Sort(d.writes)
	return d, nil
}
