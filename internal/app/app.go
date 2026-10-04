package app

import (
	"os"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
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
		{ID: "publication.inspect", Commands: []string{"info", "inspect"}, InputSchema: map[string]any{"type": "object", "additionalProperties": false, "required": []string{"book"}, "properties": map[string]any{"book": stringSchema, "rootfile": stringSchema, "section": map[string]any{"enum": []string{"metadata", "manifest", "spine", "capabilities"}}}}, OutputSchema: map[string]any{"type": "object"}},
		{ID: "publication.unpack", Commands: []string{"unpack"}, InputSchema: map[string]any{"type": "object", "additionalProperties": false, "required": []string{"book", "output"}, "properties": map[string]any{"book": stringSchema, "rootfile": stringSchema, "output": stringSchema}}, OutputSchema: map[string]any{"type": "object", "required": []string{"output", "rootfile", "files", "limitations"}}},
	}
	for i := range out {
		out[i].Version = 1
		out[i].Status = "available"
		out[i].Risk = "read_only"
		out[i].Reason = "M1-A only; success is not EPUB conformance validation"
		out[i].SupportedFeatures = []string{"EPUB2/3 ZIP", "UTF-8 XML", "metadata/manifest/spine", "original resource bytes"}
		out[i].Preconditions = []string{"safe bounded archive", "explicit rootfile if ambiguous"}
		out[i].PostChecks = []string{}
		out[i].Idempotency = "read only; unpack retries reject an existing destination"
	}
	for _, c := range []Capability{
		{ID: "doctor", Commands: []string{"doctor"}}, {ID: "navigation.inspect", Commands: []string{"toc"}}, {ID: "references.inspect"},
		{ID: "publication.validate", Commands: []string{"validate"}}, {ID: "publication.pack", Commands: []string{"pack"}},
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
