package workspace

import (
	"encoding/xml"
	"strings"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/references"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

// moveSyncRewrite is one incoming reference synchronized by a cross-resource
// move: the referring resource, the element locator, the attribute name and both
// values.
type moveSyncRewrite struct {
	path    string
	locator string
	name    string
	old     string
	new     string
}

// movePlan is one cross-resource move's complete derived facts: the frozen
// source/destination edit, the synchronized incoming references and, per source
// identity, the exact incoming edge locations the move accounted for.
type movePlan struct {
	index   int
	edit    *publication.CrossMoveEdit
	sync    []moveSyncRewrite
	covered map[string]map[string]map[string]bool
}

// deriveCrossMoves derives every xhtml.element.move v2 operation from the frozen
// bytes before any resource group is assembled, and synchronizes the known
// incoming references to every moved identity. A reference that is inside the
// moved block moves with it; an XHTML href from any other location is rewritten
// to the destination; anything else is refused with its syntax and location, so
// the move can never leave an unsynchronized dangling reference behind.
func (w *Workspace) deriveCrossMoves(a publicationRoot, pub *publication.Publication, profile xmltext.Profile, ops []Operation, revision string) (map[string][]*publication.StructureEdit, []*movePlan, *references.Graph, map[bookpath.BookPath]int64, error) {
	contrib := map[string][]*publication.StructureEdit{}
	moves := []*movePlan{}
	any := false
	for _, op := range ops {
		if op.ID == "xhtml.element.move" && op.Version == 2 {
			any = true
			break
		}
	}
	if !any {
		return contrib, moves, nil, nil, nil
	}
	docs := map[string]*publication.StructureDocument{}
	bases := map[string][]byte{}
	load := func(path string, sha string) (*publication.StructureDocument, error) {
		bp := bookpath.BookPath(path)
		base, ok := bases[path]
		if !ok {
			if err := publication.CheckXHTMLTarget(pub, bp); err != nil {
				return nil, err
			}
			var err error
			base, err = a.Read(bp, publication.XMLLimit)
			if err != nil {
				return nil, err
			}
			bases[path] = base
		}
		// Every endpoint validates its own frozen hash against the frozen bytes
		// on every use: a cached parse or an earlier implicit load for reference
		// synchronization must never relax a later explicit binding.
		if sha != "" {
			if err := checkStructureHash(base, sha); err != nil {
				return nil, err
			}
		}
		if doc, ok := docs[path]; ok {
			return doc, nil
		}
		doc, err := publication.ParseStructureDocument(base, bp, profile)
		if err != nil {
			return nil, err
		}
		docs[path] = doc
		return doc, nil
	}
	inv, err := inventory(a.root)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	graph := references.BuildSource(a, inv, pub)
	for index, op := range ops {
		param, ok := op.Params.(publication.ElementMoveCross)
		if !ok {
			continue
		}
		if param.Source.RevisionID != revision || param.Destination.RevisionID != revision {
			return nil, nil, nil, nil, ErrStalePlan
		}
		src, err := load(string(param.Source.BookPath), param.Source.ResourceSHA256)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		dst, err := load(string(param.Destination.BookPath), param.Destination.ResourceSHA256)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		edit, err := src.ElementMoveCrossEdit(param, dst)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		ids := src.IDs()
		for _, id := range edit.MovedIDs {
			if ids[id] != 1 {
				return nil, nil, nil, nil, fault.New(2, "INVALID_OPERATIONS", "moved identity %q is not unique in %s", id, param.Source.BookPath)
			}
		}
		plan := &movePlan{index: index, edit: edit, covered: map[string]map[string]map[string]bool{}}
		unique := map[string]bool{}
		for _, id := range edit.MovedIDs {
			if unique[id] {
				continue
			}
			unique[id] = true
			edges, blockers := graph.CertainIncoming(edit.SourcePath, id)
			if len(blockers) > 0 {
				b := blockers[0]
				return nil, nil, nil, nil, fault.New(1, "REFERENCE_COVERAGE_INCOMPLETE", "cannot prove references to %q in %s are synchronized: %s %s is %s", id, edit.SourcePath, b.Resource, b.Syntax, b.Status)
			}
			covered := map[string]bool{}
			for _, edge := range edges {
				if edge.Source == edit.SourcePath && locationInside(edge.Location, edit.SourceLocator) {
					covered[edge.Location] = true
					continue
				}
				switch edge.Syntax {
				case "xhtml.href", "nav.href":
					locator, name, ok := attributeLocation(edge.Location)
					if !ok || name != "href" {
						return nil, nil, nil, nil, fault.New(1, "REFERENCE_CONFLICT", "reference to %q in %s has an unsupported location %s", id, edit.SourcePath, edge.Location)
					}
					ref, err := load(string(edge.Source), "")
					if err != nil {
						return nil, nil, nil, nil, err
					}
					element, err := ref.Locate(locator)
					if err != nil {
						return nil, nil, nil, nil, err
					}
					current, present := elementAttributeValue(element, xml.Name{Local: "href"})
					if !present {
						return nil, nil, nil, nil, fault.New(1, "REFERENCE_CONFLICT", "reference to %q in %s has no literal href at %s", id, edit.SourcePath, edge.Location)
					}
					target, err := bookpath.RelativeHref(edge.Source, edit.DestinationPath)
					if err != nil {
						return nil, nil, nil, nil, err
					}
					value := string(target)
					if edge.Target.Query != "" || edge.Target.ForceQuery {
						value += "?" + edge.Target.Query
					}
					value += "#" + edge.Target.Fragment
					markup, ok := element.AttributeBytes(xml.Name{Local: "href"})
					if !ok {
						return nil, nil, nil, nil, fault.New(1, "REFERENCE_CONFLICT", "reference to %q in %s has no writable href interval at %s", id, edit.SourcePath, edge.Location)
					}
					encoded, err := ref.EncodeAttributeValue(value)
					if err != nil {
						return nil, nil, nil, nil, err
					}
					contrib[string(edge.Source)] = append(contrib[string(edge.Source)], &publication.StructureEdit{
						OpIndex: index,
						Spans:   []publication.EditSpan{{Start: markup.ValueStart, End: markup.ValueEnd, Bytes: encoded}},
						Links:   []publication.StructureLink{{Locator: locator, Name: "href", Value: value}},
						Change:  publication.StructureChange{Kind: "attribute-set", Locator: locator, Attr: &xml.Attr{Name: xml.Name{Local: "href"}, Value: value}},
					})
					plan.sync = append(plan.sync, moveSyncRewrite{path: string(edge.Source), locator: locator, name: "href", old: current, new: value})
					covered[edge.Location] = true
				default:
					return nil, nil, nil, nil, fault.New(1, "REFERENCE_CONFLICT", "reference to %q in %s uses %s at %s, which cannot be synchronized across resources", id, edit.SourcePath, edge.Syntax, edge.Location)
				}
			}
			if plan.covered[string(edit.SourcePath)] == nil {
				plan.covered[string(edit.SourcePath)] = map[string]map[string]bool{}
			}
			plan.covered[string(edit.SourcePath)][id] = covered
		}
		contrib[string(edit.SourcePath)] = append(contrib[string(edit.SourcePath)], &publication.StructureEdit{
			OpIndex:    index,
			Spans:      []publication.EditSpan{edit.SourceSpan},
			RemovedIDs: edit.MovedIDs,
			Change:     publication.StructureChange{Kind: "element-move-out", Locator: edit.SourceLocator},
		})
		contrib[string(edit.DestinationPath)] = append(contrib[string(edit.DestinationPath)], &publication.StructureEdit{
			OpIndex:  index,
			Points:   []publication.EditPoint{{At: edit.InsertAt, Bytes: edit.Block}},
			AddedIDs: edit.MovedIDs,
			Links:    edit.Links,
			IDREFs:   edit.IDREFs,
			Change: publication.StructureChange{
				Kind: "element-move-in", Locator: edit.Anchor, Position: edit.Position,
				Fragment: edit.Fragment, Block: edit.Block, At: edit.InsertAt,
			},
		})
		moves = append(moves, plan)
	}
	return contrib, moves, &graph, inv, nil
}

// elementAttributeValue returns one frozen attribute value by resolved name.
func elementAttributeValue(e *xmltext.Element, name xml.Name) (string, bool) {
	for _, a := range e.Attributes {
		if a.Name == name {
			return a.Value, true
		}
	}
	return "", false
}

// locationInside reports whether a reference index location belongs to one
// element subtree, including the element's own attributes.
func locationInside(location, root string) bool {
	return location == root || strings.HasPrefix(location, root+"/")
}

// attributeLocation splits one index location into its element locator and the
// attribute name after the final /@.
func attributeLocation(location string) (string, string, bool) {
	i := strings.LastIndex(location, "/@")
	if i < 0 || i == 0 {
		return "", "", false
	}
	return location[:i], location[i+2:], true
}
