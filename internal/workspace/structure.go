package workspace

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/metadata"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/references"
	"github.com/LeviTK/Kepub/internal/xmltext"
)

// recomputeStructure derives a schema 4 plan. metadata.set operations keep
// their frozen semantics on the package document; content.text.set and xhtml.*
// operations are resolved against the frozen XHTML bytes as disjoint edits.
func (w *Workspace) recomputeStructure(a publicationRoot, ops []Operation, revision string) (derivation, error) {
	pub, err := publication.Load(a, w.state.Rootfile)
	if err != nil {
		return derivation{}, err
	}
	profile := xmltext.Profile{Version: pub.Version, MediaType: "application/xhtml+xml"}
	outputs := map[string][]byte{}
	planned := map[string][]*publication.StructureEdit{}
	rootCurrent, rootTouched := []byte(nil), false
	seenMetadata := map[string]bool{}
	for _, op := range ops {
		if op.ID != "metadata.set" {
			continue
		}
		param := op.Params.(metadata.Set)
		b, err := a.Read(bookpath.BookPath(w.state.Rootfile), publication.XMLLimit)
		if err != nil {
			return derivation{}, err
		}
		if rootCurrent == nil {
			rootCurrent = b
		}
		location, err := metadata.Select(b, param)
		if err != nil {
			return derivation{}, err
		}
		key := "metadata\x00" + location
		if seenMetadata[key] {
			return derivation{}, fmt.Errorf("duplicate metadata target %s", location)
		}
		seenMetadata[key] = true
		out, _, err := metadata.Apply(rootCurrent, param)
		if err != nil {
			return derivation{}, err
		}
		rootCurrent = out
		rootTouched = true
	}
	if rootTouched {
		outputs[w.state.Rootfile] = rootCurrent
	}

	type group struct {
		path  string
		index []int
		ops   []Operation
	}
	groups := map[string]*group{}
	order := []string{}
	for i, op := range ops {
		if op.ID == "metadata.set" {
			continue
		}
		var path string
		switch param := op.Params.(type) {
		case publication.TextSet:
			path = string(param.BookPath)
		case publication.AttributeSet:
			path = string(param.BookPath)
		case publication.AttributeRemove:
			path = string(param.BookPath)
		case publication.ElementDelete:
			path = string(param.BookPath)
		case publication.ElementInsert:
			path = string(param.BookPath)
		case publication.ElementReplace:
			path = string(param.BookPath)
		case publication.ElementMove:
			path = string(param.BookPath)
		default:
			return derivation{}, fmt.Errorf("unsupported operation params")
		}
		if path == w.state.Rootfile {
			return derivation{}, fault.New(2, "INVALID_OPERATIONS", "the package document is not an XHTML edit target")
		}
		g, ok := groups[path]
		if !ok {
			g = &group{path: path}
			groups[path] = g
			order = append(order, path)
		}
		g.index = append(g.index, i)
		g.ops = append(g.ops, op)
	}

	// Phase 1 derives every resource's edits and binding checks. Dependency
	// facts are only collected here: identity and link validation must see the
	// whole transaction, never a partially processed group.
	gate := &structureGate{a: a, pub: pub, baseIDs: map[bookpath.BookPath]map[string]int{}, removed: map[bookpath.BookPath]map[string]int{}, added: map[bookpath.BookPath]map[string]int{}}
	type groupEdit struct {
		path  string
		bp    bookpath.BookPath
		base  []byte
		doc   *publication.StructureDocument
		edits []*publication.StructureEdit
	}
	derived := make([]*groupEdit, 0, len(order))
	for _, path := range order {
		g := groups[path]
		bp := bookpath.BookPath(path)
		if err := publication.CheckXHTMLTarget(pub, bp); err != nil {
			return derivation{}, err
		}
		base, err := a.Read(bp, publication.XMLLimit)
		if err != nil {
			return derivation{}, err
		}
		doc, err := publication.ParseStructureDocument(base, bp, profile)
		if err != nil {
			return derivation{}, err
		}
		edits := make([]*publication.StructureEdit, 0, len(g.ops))
		targets := map[string]string{}
		for j, op := range g.ops {
			index := g.index[j]
			key, kind := structureTargetKey(op)
			if key != "" {
				if _, ok := targets[key]; ok {
					return derivation{}, fault.New(2, "INVALID_OPERATIONS", "duplicate %s in %s", kind, path)
				}
				targets[key] = kind
			}
			var edit *publication.StructureEdit
			var err error
			switch param := op.Params.(type) {
			case publication.TextSet:
				if param.RevisionID != revision {
					return derivation{}, ErrStalePlan
				}
				if err := checkStructureHash(base, param.ResourceSHA256); err != nil {
					return derivation{}, err
				}
				edit, err = doc.TextSetEdit(param, param.NewValue)
			case publication.AttributeSet:
				if param.RevisionID != revision {
					return derivation{}, ErrStalePlan
				}
				if err := checkStructureHash(base, param.ResourceSHA256); err != nil {
					return derivation{}, err
				}
				edit, err = doc.AttributeSetEdit(param)
			case publication.AttributeRemove:
				if param.RevisionID != revision {
					return derivation{}, ErrStalePlan
				}
				if err := checkStructureHash(base, param.ResourceSHA256); err != nil {
					return derivation{}, err
				}
				edit, err = doc.AttributeRemoveEdit(param)
			case publication.ElementDelete:
				if param.RevisionID != revision {
					return derivation{}, ErrStalePlan
				}
				if err := checkStructureHash(base, param.ResourceSHA256); err != nil {
					return derivation{}, err
				}
				edit, err = doc.ElementDeleteEdit(param)
			case publication.ElementInsert:
				if param.RevisionID != revision {
					return derivation{}, ErrStalePlan
				}
				if err := checkStructureHash(base, param.ResourceSHA256); err != nil {
					return derivation{}, err
				}
				edit, err = doc.ElementInsertEdit(param)
			case publication.ElementReplace:
				if param.RevisionID != revision {
					return derivation{}, ErrStalePlan
				}
				if err := checkStructureHash(base, param.ResourceSHA256); err != nil {
					return derivation{}, err
				}
				edit, err = doc.ElementReplaceEdit(param)
			case publication.ElementMove:
				if param.RevisionID != revision {
					return derivation{}, ErrStalePlan
				}
				if err := checkStructureHash(base, param.ResourceSHA256); err != nil {
					return derivation{}, err
				}
				edit, err = doc.ElementMoveEdit(param)
			default:
				return derivation{}, fmt.Errorf("unsupported operation params")
			}
			if err != nil {
				return derivation{}, err
			}
			edit.OpIndex = index
			edits = append(edits, edit)
		}
		if err := publication.ValidateEdits(edits); err != nil {
			return derivation{}, fault.New(2, "INVALID_OPERATIONS", "%v", err)
		}
		derived = append(derived, &groupEdit{path: path, bp: bp, base: base, doc: doc, edits: edits})
	}
	// Phase 2 collects every identity fact of the whole transaction before any
	// dependency check, so operation order cannot change the outcome.
	for _, ge := range derived {
		gate.baseIDs[ge.bp] = ge.doc.IDs()
		// Attribute changes are merged per frozen node: identity belongs to the
		// element, so id and xml:id edits on one node form one final identity
		// set instead of independent per-attribute deltas.
		attributeChanges := map[string][]publication.StructureChange{}
		attributeOrder := []string{}
		for _, edit := range ge.edits {
			if edit.Change.Kind != "attribute-set" && edit.Change.Kind != "attribute-remove" {
				continue
			}
			locator := edit.Change.Locator
			if _, ok := attributeChanges[locator]; !ok {
				attributeOrder = append(attributeOrder, locator)
			}
			attributeChanges[locator] = append(attributeChanges[locator], edit.Change)
		}
		for _, locator := range attributeOrder {
			element, err := ge.doc.Locate(locator)
			if err != nil {
				return derivation{}, err
			}
			removed, added := publication.MergedIdentityDelta(element, attributeChanges[locator])
			for _, id := range removed {
				if gate.removed[ge.bp] == nil {
					gate.removed[ge.bp] = map[string]int{}
				}
				gate.removed[ge.bp][id]++
			}
			for _, id := range added {
				if gate.added[ge.bp] == nil {
					gate.added[ge.bp] = map[string]int{}
				}
				if gate.added[ge.bp][id] > 0 {
					return derivation{}, fault.New(2, "INVALID_OPERATIONS", "duplicate new id %q in %s", id, ge.path)
				}
				gate.added[ge.bp][id]++
			}
		}
		for _, edit := range ge.edits {
			if edit.Change.Kind == "attribute-set" || edit.Change.Kind == "attribute-remove" {
				continue // identity facts were merged per node above
			}
			for _, id := range edit.RemovedIDs {
				if gate.removed[ge.bp] == nil {
					gate.removed[ge.bp] = map[string]int{}
				}
				gate.removed[ge.bp][id]++
			}
			for _, id := range edit.AddedIDs {
				if gate.added[ge.bp] == nil {
					gate.added[ge.bp] = map[string]int{}
				}
				if gate.added[ge.bp][id] > 0 {
					return derivation{}, fault.New(2, "INVALID_OPERATIONS", "duplicate new id %q in %s", id, ge.path)
				}
				gate.added[ge.bp][id]++
			}
		}
	}
	for _, ge := range derived {
		for id := range gate.added[ge.bp] {
			// A new identity may only be written when every frozen instance is
			// removed in the same transaction; a residual instance would leave
			// the identity duplicated and any reference ambiguous.
			if residual := gate.baseIDs[ge.bp][id] - gate.removed[ge.bp][id]; residual > 0 {
				return derivation{}, fault.New(2, "INVALID_OPERATIONS", "new id %q already exists in %s", id, ge.path)
			}
		}
		for id := range gate.removed[ge.bp] {
			// Only an identity that disappears entirely can dangle an existing
			// reference; a remaining or re-added instance still resolves.
			if gate.finalCount(ge.bp, id) > 0 {
				continue
			}
			if err := gate.checkRemovedID(ge.bp, id); err != nil {
				return derivation{}, err
			}
		}
		for _, edit := range ge.edits {
			for _, link := range edit.Links {
				if err := gate.checkLink(ge.bp, link.Value); err != nil {
					return derivation{}, err
				}
			}
			for _, ref := range edit.IDREFs {
				if err := gate.checkIDREF(ge.bp, ref); err != nil {
					return derivation{}, err
				}
			}
		}
	}
	// Phase 3 applies and independently verifies each resource's bytes.
	for _, ge := range derived {
		output := publication.ApplyEdits(ge.base, ge.edits)
		if err := publication.VerifyStructure(ge.doc, ge.edits, output); err != nil {
			return derivation{}, fmt.Errorf("structural verification: %v", err)
		}
		outputs[ge.path] = output
		planned[ge.path] = ge.edits
	}
	writes := make([]string, 0, len(outputs))
	for path, out := range outputs {
		var base []byte
		switch path {
		case w.state.Rootfile:
			base, _ = a.Read(bookpath.BookPath(path), publication.XMLLimit)
		default:
			base, _ = a.Read(bookpath.BookPath(path), publication.XMLLimit)
		}
		if base == nil || !bytes.Equal(out, base) {
			writes = append(writes, path)
		}
	}
	slices.Sort(writes)
	final := map[string][]byte{}
	for _, path := range writes {
		final[path] = outputs[path]
	}
	return derivation{outputs: final, writes: writes, edits: planned}, nil
}

func checkStructureHash(base []byte, sha string) error {
	h := sha256.Sum256(base)
	if hex.EncodeToString(h[:]) != sha {
		return fault.New(4, "INPUT_DRIFT", "content resource hash changed")
	}
	return nil
}

// structureTargetKey identifies one operation's write target and the kind of
// target it is, so a transaction cannot carry duplicate or aliased targets. One
// insertion per anchor position keeps byte order unambiguous.
func structureTargetKey(op Operation) (string, string) {
	switch param := op.Params.(type) {
	case publication.TextSet:
		return "text\x00" + string(param.BookPath) + "\x00" + param.Locator, "text target"
	case publication.AttributeSet:
		return "attribute\x00" + string(param.BookPath) + "\x00" + param.Locator + "\x00" + param.Namespace + "\x00" + param.Name, "attribute target"
	case publication.AttributeRemove:
		return "attribute\x00" + string(param.BookPath) + "\x00" + param.Locator + "\x00" + param.Namespace + "\x00" + param.Name, "attribute target"
	case publication.ElementDelete:
		return "element\x00" + string(param.BookPath) + "\x00" + param.Locator, "element target"
	case publication.ElementReplace:
		return "element\x00" + string(param.BookPath) + "\x00" + param.Locator, "element target"
	case publication.ElementMove:
		return "element\x00" + string(param.BookPath) + "\x00" + param.Locator, "element target"
	case publication.ElementInsert:
		return "insert\x00" + string(param.BookPath) + "\x00" + param.Locator + "\x00" + param.Position, "insertion point"
	}
	return "", ""
}

// structureGate refuses dependency-removing writes that cannot be proven safe
// against the frozen publication.
type structureGate struct {
	a         publicationRoot
	pub       *publication.Publication
	inventory map[bookpath.BookPath]int64
	graph     *references.Graph
	baseIDs   map[bookpath.BookPath]map[string]int
	removed   map[bookpath.BookPath]map[string]int
	added     map[bookpath.BookPath]map[string]int
}

func (g *structureGate) inventoryOnce() (map[bookpath.BookPath]int64, error) {
	if g.inventory != nil {
		return g.inventory, nil
	}
	inv, err := inventory(g.a.root)
	if err != nil {
		return nil, err
	}
	g.inventory = inv
	return inv, nil
}

func (g *structureGate) graphOnce() (*references.Graph, error) {
	if g.graph != nil {
		return g.graph, nil
	}
	inv, err := g.inventoryOnce()
	if err != nil {
		return nil, err
	}
	graph := references.BuildSource(g.a, inv, g.pub)
	g.graph = &graph
	return g.graph, nil
}

// checkRemovedID refuses to remove or rename an id that any known reference
// targets, and refuses when reference extraction cannot prove the absence.
func (g *structureGate) checkRemovedID(resource bookpath.BookPath, id string) error {
	graph, err := g.graphOnce()
	if err != nil {
		return err
	}
	edges, blockers := graph.CertainIncoming(resource, id)
	if len(edges) > 0 {
		e := edges[0]
		return fault.New(1, "REFERENCE_CONFLICT", "id %q in %s is referenced from %s (%s)", id, resource, e.Source, e.Location)
	}
	if len(blockers) > 0 {
		b := blockers[0]
		return fault.New(1, "REFERENCE_COVERAGE_INCOMPLETE", "cannot prove id %q in %s is unreferenced: %s %s is %s", id, resource, b.Resource, b.Syntax, b.Status)
	}
	return nil
}

// checkLink validates a new URL-bearing attribute value: no script or data
// schemes, an existing internal target, and a resolvable unambiguous fragment.
func (g *structureGate) checkLink(resource bookpath.BookPath, value string) error {
	lower := strings.ToLower(strings.TrimSpace(value))
	if strings.HasPrefix(lower, "javascript:") || strings.HasPrefix(lower, "data:") {
		return fault.New(2, "INVALID_OPERATIONS", "unsupported URL scheme in %q", value)
	}
	ref, err := bookpath.ResolveReference(resource, bookpath.Href(value))
	if err != nil {
		return fault.New(2, "INVALID_OPERATIONS", "invalid reference %q: %v", value, err)
	}
	if ref.External {
		return nil
	}
	inv, err := g.inventoryOnce()
	if err != nil {
		return err
	}
	if _, ok := inv[ref.Path]; !ok {
		return fault.New(2, "INVALID_OPERATIONS", "reference target %q does not exist", ref.Path)
	}
	if ref.Fragment == "" {
		return nil
	}
	ids, err := g.idsFor(ref.Path)
	if err != nil {
		return err
	}
	switch ids[ref.Fragment] {
	case 1:
		return nil
	case 0:
		return fault.New(2, "INVALID_OPERATIONS", "fragment %q is not present in %s", ref.Fragment, ref.Path)
	default:
		return fault.New(2, "INVALID_OPERATIONS", "fragment %q is ambiguous in %s", ref.Fragment, ref.Path)
	}
}

// checkIDREF validates a new same-document IDREF against the transaction's
// final identity state: the referenced identity must exist exactly once. The
// shared IDREF vocabulary means an existing attribute and a new write cannot
// disagree about what is a reference.
func (g *structureGate) checkIDREF(resource bookpath.BookPath, ref publication.StructureIDREF) error {
	ids, err := g.idsFor(resource)
	if err != nil {
		return err
	}
	switch ids[ref.Value] {
	case 1:
		return nil
	case 0:
		return fault.New(2, "INVALID_OPERATIONS", "IDREF %s=%q is not present in %s", ref.Name, ref.Value, resource)
	default:
		return fault.New(2, "INVALID_OPERATIONS", "IDREF %s=%q is ambiguous in %s", ref.Name, ref.Value, resource)
	}
}

// finalCount is the number of identities with this value in the transaction's
// final state of one resource: frozen instances minus removed instances plus
// added instances. Counts, not flags, so a residual duplicate stays visible and
// an ambiguous reference cannot pass.
func (g *structureGate) finalCount(resource bookpath.BookPath, id string) int {
	count := g.baseIDs[resource][id] - g.removed[resource][id] + g.added[resource][id]
	if count < 0 {
		return 0
	}
	return count
}

func (g *structureGate) idsFor(path bookpath.BookPath) (map[string]int, error) {
	if ids, ok := g.baseIDs[path]; ok {
		out := map[string]int{}
		for id := range ids {
			if count := g.finalCount(path, id); count > 0 {
				out[id] = count
			}
		}
		for id := range g.added[path] {
			if count := g.finalCount(path, id); count > 0 {
				out[id] = count
			}
		}
		return out, nil
	}
	data, err := g.a.Read(path, publication.XMLLimit)
	if err != nil {
		return nil, err
	}
	doc, err := xmltext.Parse(data)
	if err != nil {
		return nil, fault.New(2, "INVALID_OPERATIONS", "reference target %s cannot be indexed: %v", path, err)
	}
	if err := doc.RequireComplete(); err != nil {
		return nil, fault.New(2, "INVALID_OPERATIONS", "reference target %s is incomplete: %v", path, err)
	}
	return publication.CountIDs(doc), nil
}

// inventory walks a frozen revision without hashing. Unsafe entries are refused
// so the dependency gate cannot run over links or special files.
func inventory(r *os.Root) (map[bookpath.BookPath]int64, error) {
	out := map[bookpath.BookPath]int64{}
	names := map[string]string{}
	err := fs.WalkDir(r.FS(), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || name == "." {
			return err
		}
		bp, err := bookpath.Parse(name)
		if err != nil {
			return err
		}
		key := bookpath.CollisionKey(bp)
		if old, ok := names[key]; ok && old != name {
			return fmt.Errorf("colliding paths %q and %q", old, name)
		}
		names[key] = name
		info, err := r.Lstat(name)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsafe tree entry %q: %s", name, info.Mode())
		}
		out[bp] = info.Size()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
