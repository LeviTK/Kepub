package app

import (
	"os"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/references"
)

type Capability struct {
	ID                string   `json:"operationId"`
	Version           int      `json:"operationVersion"`
	Status            string   `json:"implementationStatus"`
	Reason            string   `json:"reason"`
	Mutates           bool     `json:"mutatesPublication"`
	RequiresGUI       bool     `json:"requiresGUI"`
	RequiresModel     bool     `json:"requiresModel"`
	RequiresNetwork   bool     `json:"requiresNetwork"`
	Commands          []string `json:"commands"`
	Risk              string   `json:"risk"`
	InputSchema       any      `json:"inputSchema"`
	OutputSchema      any      `json:"outputSchema"`
	SupportedFeatures []string `json:"supportedFeatures"`
	Preconditions     []string `json:"preconditions"`
	PostChecks        []string `json:"postChecks"`
	Idempotency       string   `json:"idempotency"`
}

func Capabilities() []Capability {
	stringSchema := map[string]any{"type": "string", "minLength": 1}
	out := []Capability{
		{ID: "capabilities", Commands: []string{"capabilities"}, InputSchema: map[string]any{"type": "object", "additionalProperties": false}, OutputSchema: map[string]any{"type": "array"}},
		{ID: "publication.inspect", Commands: []string{"info", "inspect", "toc"}, InputSchema: map[string]any{"type": "object", "additionalProperties": false, "required": []string{"book"}, "properties": map[string]any{"book": stringSchema, "rootfile": stringSchema, "section": map[string]any{"enum": []string{"metadata", "manifest", "spine", "navigation", "references", "capabilities"}}, "resource": stringSchema, "direction": map[string]any{"enum": []string{"incoming", "outgoing"}}}}, OutputSchema: map[string]any{"type": "object"}},
		{ID: "navigation.inspect", Commands: []string{"toc"}, InputSchema: map[string]any{"type": "object", "additionalProperties": false, "required": []string{"book"}, "properties": map[string]any{"book": stringSchema, "rootfile": stringSchema}}, OutputSchema: map[string]any{"type": "object", "required": []string{"rootfile", "section", "value", "limitations"}, "properties": map[string]any{"value": map[string]any{"type": "object", "required": []string{"entries", "diagnostics", "status"}}}}},
		{ID: "references.inspect", InputSchema: map[string]any{"type": "object", "additionalProperties": false, "required": []string{"book"}, "properties": map[string]any{"book": stringSchema, "rootfile": stringSchema, "resource": stringSchema, "direction": map[string]any{"enum": []string{"incoming", "outgoing"}}}, "dependentRequired": map[string]any{"direction": []string{"resource"}}}, OutputSchema: map[string]any{"type": "object", "required": []string{"rootfile", "section", "value", "limitations"}, "properties": map[string]any{"value": map[string]any{"type": "object", "required": []string{"edges", "coverage", "diagnostics", "status"}}}}},
		{ID: "publication.unpack", Commands: []string{"unpack"}, InputSchema: map[string]any{"type": "object", "additionalProperties": false, "required": []string{"book", "output"}, "properties": map[string]any{"book": stringSchema, "rootfile": stringSchema, "output": stringSchema}}, OutputSchema: map[string]any{"type": "object", "required": []string{"output", "rootfile", "files", "limitations"}}},
	}
	for i := range out {
		out[i].Version = 1
		out[i].Status = "available"
		out[i].Risk = "read_only"
		out[i].Reason = "M1 read-only queries; success is not EPUB conformance validation; inspect references reports per-syntax coverage"
		out[i].SupportedFeatures = []string{"EPUB2/3 ZIP", "UTF-8 XML", "metadata/manifest/spine", "EPUB3 nav / EPUB2 NCX", "read-only references with per-syntax coverage", "original resource bytes"}
		out[i].Preconditions = []string{"safe bounded archive", "explicit rootfile if ambiguous"}
		out[i].PostChecks = []string{}
		out[i].Idempotency = "read only; unpack retries reject an existing destination"
	}
	for _, id := range []string{"validate", "pack"} {
		properties := map[string]any{"book": stringSchema, "rootfile": stringSchema, "strict": map[string]any{"type": "boolean"}, "timeout": map[string]any{"type": "integer", "minimum": 1}}
		required := []string{"book"}
		if id == "pack" {
			properties["output"] = stringSchema
			properties["draft"] = map[string]any{"type": "boolean"}
			required = append(required, "output")
		}
		out = append(out, Capability{ID: "publication." + id, Version: 1, Status: "available", Reason: "Implemented; formal validation requires locally installed pinned EPUBCheck 5.3.0 and Java; no download or automatic draft fallback", Commands: []string{id}, Risk: "external", InputSchema: map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": properties}, OutputSchema: map[string]any{"type": "object"}, SupportedFeatures: []string{"safe ZIP / explicit publication directory snapshot", "kepub-tree-v1 approved inventory", "full EPUBCheck conformance", "partial reference coverage is not a conformance gate"}, Preconditions: []string{"frozen input", "explicit rootfile if ambiguous", "pack output outside publication root and absent"}, PostChecks: []string{"final ZIP safety and input hash", "EPUBCheck full report except explicit draft"}, Idempotency: "read only input; pack never replaces output"})
	}
	for _, c := range []Capability{
		{ID: "doctor", Commands: []string{"doctor"}},
		{ID: "metadata.set", Mutates: true}, {ID: "resource.rename", Mutates: true}, {ID: "workspace", Commands: []string{"workspace"}},
		{ID: "plan", Commands: []string{"plan"}}, {ID: "apply", Commands: []string{"apply"}, Mutates: true},
		{ID: "preview", Commands: []string{"preview", "serve"}, RequiresGUI: true}, {ID: "amp", Commands: []string{"amp", "task"}, RequiresModel: true},
	} {
		c.Version = 1
		c.Status = "planned"
		c.Reason = "Not implemented in M1-A; schemas/requirements not yet frozen"
		out = append(out, c)
	}
	return out
}

// Read is the single use case used by info and inspect; ownership remains here.
func Read(book, rootfile string) (*archive.Archive, *publication.Publication, error) {
	st, err := os.Stat(book)
	if err != nil {
		return nil, nil, fault.New(6, "IO_ERROR", "input: %v", err)
	}
	if st.IsDir() {
		return nil, nil, fault.New(3, "UNSUPPORTED_INPUT", "directory input is not implemented in M1-A")
	}
	if !st.Mode().IsRegular() {
		return nil, nil, fault.New(3, "UNSUPPORTED_INPUT", "input must be a regular EPUB ZIP file")
	}
	a, err := archive.Open(book, archive.DefaultLimits)
	if err != nil {
		return nil, nil, err
	}
	p, err := publication.Load(a, rootfile)
	if err != nil {
		a.Close()
		return nil, nil, err
	}
	return a, p, nil
}

// Inspect is shared by section queries and the toc convenience command. Missing
// content/unsupported syntax is returned as data; this is not validate.
func Inspect(a *archive.Archive, p *publication.Publication, section, resource, direction string) (any, error) {
	if err := ValidateInspect(section, resource, direction); err != nil {
		return nil, err
	}
	var value any
	switch section {
	case "metadata":
		value = p.Metadata
	case "manifest":
		value = p.Manifest
	case "spine":
		value = map[string]any{"items": p.Spine, "attributes": p.SpineAttributes}
	case "navigation":
		value = publication.LoadNavigation(a, p)
	case "references":
		graph := references.Build(a, p)
		value, _ = graph.Filter(resource, direction)
	case "capabilities":
		value = Capabilities()
	default:
		return nil, fault.New(2, "INVALID_ARGUMENT", "supported --section is required")
	}
	return map[string]any{"rootfile": p.Rootfile, "section": section, "value": value, "limitations": p.Limitations}, nil
}

// ValidateInspect runs before opening the book so input errors take precedence
// over I/O and publication errors, as required by the CLI envelope contract.
func ValidateInspect(section, resource, direction string) error {
	if section != "references" && (resource != "" || direction != "") {
		return fault.New(2, "INVALID_ARGUMENT", "resource/direction filters require the references section")
	}
	switch section {
	case "metadata", "manifest", "spine", "navigation", "capabilities":
		return nil
	case "references":
		_, err := (references.Graph{}).Filter(resource, direction)
		return err
	default:
		return fault.New(2, "INVALID_ARGUMENT", "supported --section is required")
	}
}
