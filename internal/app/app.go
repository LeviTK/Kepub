package app

import (
	"os"

	"github.com/LeviTK/Kepub/internal/archive"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/references"
)

type Capability struct {
	ID                string    `json:"operationId"`
	Version           int       `json:"operationVersion"`
	Status            string    `json:"implementationStatus"`
	Reason            string    `json:"reason"`
	Mutates           bool      `json:"mutatesPublication"`
	RequiresGUI       bool      `json:"requiresGUI"`
	RequiresModel     bool      `json:"requiresModel"`
	RequiresNetwork   bool      `json:"requiresNetwork"`
	Commands          []string  `json:"commands"`
	Risk              string    `json:"risk"`
	InputSchema       any       `json:"inputSchema"`
	OutputSchema      any       `json:"outputSchema"`
	SupportedFeatures []string  `json:"supportedFeatures"`
	Preconditions     []string  `json:"preconditions"`
	PostChecks        []string  `json:"postChecks"`
	Idempotency       string    `json:"idempotency"`
	CommandSchemas    []Command `json:"commandSchemas,omitempty"`
}

func Capabilities() []Capability {
	stringSchema := map[string]any{"type": "string", "minLength": 1}
	out := []Capability{
		{ID: "version", Commands: []string{"version"}},
		{ID: "doctor", Commands: []string{"doctor"}},
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
		out[i].SupportedFeatures = []string{"EPUB2/3 ZIP", "UTF-8/UTF-16 XML; native HTML DOCTYPE; T1b DTD/entities pending", "metadata/manifest/spine", "EPUB3 nav / EPUB2 NCX", "read-only references with per-syntax coverage", "original resource bytes"}
		out[i].Preconditions = []string{"safe bounded archive", "explicit rootfile if ambiguous"}
		out[i].PostChecks = []string{}
		out[i].Idempotency = "read only; unpack retries reject an existing destination"
		if out[i].ID == "version" || out[i].ID == "doctor" {
			out[i].InputSchema = map[string]any{"type": "object", "additionalProperties": false}
			out[i].OutputSchema = map[string]any{"type": "object"}
			out[i].Reason = "C1 build information and local readiness only; no install, authentication, network or model execution"
			out[i].SupportedFeatures = []string{"headless core", "build metadata without runtime Git", "bounded Java/checker readiness; optional Amp discovery only"}
			out[i].Preconditions = []string{}
			out[i].Idempotency = "read only"
		}
	}
	for _, id := range []string{"validate", "pack"} {
		properties := map[string]any{"book": stringSchema, "rootfile": stringSchema, "strict": map[string]any{"type": "boolean"}, "timeout": map[string]any{"type": "integer", "minimum": 1, "maximum": 4294967295}}
		required := []string{"book"}
		if id == "pack" {
			properties["output"] = stringSchema
			properties["draft"] = map[string]any{"type": "boolean"}
			required = append(required, "output")
		}
		out = append(out, Capability{ID: "publication." + id, Version: 1, Status: "available", Reason: "Implemented; formal validation requires locally installed pinned EPUBCheck 5.3.0 and Java; no download or automatic draft fallback", Commands: []string{id}, Risk: "external", InputSchema: map[string]any{"type": "object", "additionalProperties": false, "required": required, "properties": properties}, OutputSchema: map[string]any{"type": "object"}, SupportedFeatures: []string{"safe ZIP / explicit publication directory snapshot", "kepub-tree-v1 approved inventory", "full EPUBCheck conformance", "partial reference coverage is not a conformance gate"}, Preconditions: []string{"frozen input", "explicit rootfile if ambiguous", "pack output outside publication root and absent"}, PostChecks: []string{"final ZIP safety and input hash", "EPUBCheck full report except explicit draft"}, Idempotency: "read only input; pack never replaces output"})
	}
	out = append(out, Capability{
		ID: "publication.content", Version: 1, Status: "available", Commands: []string{"content"}, Risk: "read_only",
		Reason: "C2 bounded accepted-revision XHTML text extraction; not full-text search, EPUB conformance or permission to edit",
		InputSchema: map[string]any{"type": "object", "additionalProperties": false, "required": []string{"workspace", "resource"}, "properties": map[string]any{
			"workspace": stringSchema, "resource": stringSchema,
			"query": map[string]any{"type": "string", "minLength": 1, "maxLength": 4096, "x-maxUtf8Bytes": 4096, "description": "1–4096 UTF-8 bytes; case-sensitive literal substring of each decoded element text; omitted selects all nodes"},
			"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 200, "default": 50},
		}},
		OutputSchema:      map[string]any{"type": "object", "required": []string{"workspaceId", "revisionId", "rootfile", "bookPath", "resourceSha256", "locatorVersion", "matchedCount", "returnedCount", "truncated", "nodes"}},
		SupportedFeatures: []string{"selected manifest application/xhtml+xml only", "exact BookPath", "decoded mixed text and structural locator v1", "excluded script/style/head/foreign subtrees and their ancestors", "8 MiB XML input; 1 MiB returned text; no clipping"},
		Preconditions:     []string{"explicit workspace directory and resource", "cooperative exclusive workspace lock", "same frozen accepted input for publication, hash and text"},
		PostChecks:        []string{"resource hash from original bytes", "matched/returned counts and explicit truncation"}, Idempotency: "read only; no publication changes",
	})
	out = append(out, Capability{
		ID: "publication.search", Version: 1, Status: "available", Commands: []string{"search"}, Risk: "read_only",
		Reason: "T1a accepted-only manifest-order literal XHTML search; not browser visibility, conformance or editing permission",
		InputSchema: map[string]any{"type": "object", "additionalProperties": false, "required": []string{"workspace", "query"}, "properties": map[string]any{
			"workspace": stringSchema,
			"query":     map[string]any{"type": "string", "minLength": 1, "maxLength": 4096, "x-maxUtf8Bytes": 4096},
			"limit":     map[string]any{"type": "integer", "minimum": 1, "maximum": 200, "default": 50},
		}},
		OutputSchema:      map[string]any{"type": "object", "required": []string{"workspaceId", "revisionId", "rootfile", "matchedCount", "returnedCount", "truncated", "results"}},
		SupportedFeatures: []string{"content node/exclusion semantics; no cross-resource matching", "all selected manifest XHTML counted after return limit", "8 MiB raw/resource; 16 MiB decoded/resource; 32 MiB index/resource; 128 MiB raw scan; 1 MiB returned text"},
		Preconditions:     []string{"explicit workspace; exclusive cooperative lock; verified accepted source"}, PostChecks: []string{"original resource hashes and exact identities"}, Idempotency: "read only",
	})
	metadataSchema := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"namespace", "localName", "expectedOldValue", "newValue"}, "properties": map[string]any{"namespace": map[string]any{"const": "http://purl.org/dc/elements/1.1/"}, "localName": map[string]any{"enum": []string{"title", "creator"}}, "id": stringSchema, "expectedOldValue": map[string]any{"type": "string"}, "newValue": map[string]any{"type": "string"}}}
	contentSchema := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"bookPath", "revisionId", "resourceSha256", "locatorVersion", "locator", "expectedOldValue", "newValue"}, "properties": map[string]any{
		"bookPath": stringSchema, "revisionId": stringSchema,
		"resourceSha256": map[string]any{"type": "string", "pattern": "^[0-9a-f]{64}$"},
		"locatorVersion": map[string]any{"const": 1}, "locator": map[string]any{"type": "string", "minLength": 1, "x-maxUtf8Bytes": 4096},
		"expectedOldValue": map[string]any{"type": "string", "x-maxUtf8Bytes": publication.ContentTextLimit}, "newValue": map[string]any{"type": "string", "x-maxUtf8Bytes": publication.ContentTextLimit},
	}}
	for _, c := range []Capability{
		{ID: "metadata.set", Mutates: true, Risk: "bounded_edit", InputSchema: metadataSchema, SupportedFeatures: []string{"unique existing dc:title/dc:creator simple text", "exact namespace/local name/optional ID", "expected old value", "local escaped byte replacement", "no-op preserves bytes; no automatic timestamp"}},
		{ID: "content.text.set", Mutates: true, Risk: "bounded_edit", InputSchema: contentSchema, SupportedFeatures: []string{"request/plan schema 2; execution 2; operation 1", "accepted revision and original resource SHA-256 binding", "exact manifest XHTML and structural locator v1", "simple independently closed body text only; no mixed/foreign/script/style/head subtree", "escaped local byte replacement; no-op preserves bytes; no automatic timestamp"}},
		{ID: "workspace.open", Commands: []string{"workspace open"}, InputSchema: map[string]any{"type": "object", "additionalProperties": false, "required": []string{"book", "output"}, "properties": map[string]any{"book": stringSchema, "output": stringSchema, "rootfile": stringSchema}}},
		{ID: "plan", Commands: []string{"plan"}, InputSchema: map[string]any{"type": "object", "additionalProperties": false, "required": []string{"workspace", "operations", "output"}, "properties": map[string]any{"workspace": stringSchema, "operations": stringSchema, "output": stringSchema}}},
		{ID: "apply", Commands: []string{"apply"}, Mutates: true, Risk: "bounded_edit", InputSchema: map[string]any{"type": "object", "additionalProperties": false, "required": []string{"workspace", "plan"}, "properties": map[string]any{"workspace": stringSchema, "plan": stringSchema}}},
		{ID: "task.status", Commands: []string{"task status"}}, {ID: "task.diff", Commands: []string{"task diff"}}, {ID: "task.accept", Commands: []string{"task accept"}, Mutates: true, Risk: "external"}, {ID: "task.reject", Commands: []string{"task reject"}},
		{ID: "workspace.export", Commands: []string{"workspace export"}, Risk: "external"},
	} {
		c.Version, c.Status = 1, "available"
		c.Reason = "Explicit workspace directory; one metadata.set v1 (schema 1) or content.text.set v1 (schema 2); apply remains review_required/conformance not_run; accept and formal export run pinned EPUBCheck (must be installed)"
		if c.Risk == "" {
			c.Risk = "read_only"
		}
		if c.InputSchema == nil {
			c.InputSchema = map[string]any{"type": "object", "additionalProperties": false, "required": []string{"workspace", "task"}, "properties": map[string]any{"workspace": stringSchema, "task": stringSchema}}
			if c.ID == "task.accept" {
				c.InputSchema = map[string]any{"type": "object", "additionalProperties": false, "required": []string{"workspace", "task"}, "properties": map[string]any{"workspace": stringSchema, "task": stringSchema, "strict": map[string]any{"type": "boolean"}, "timeout": map[string]any{"type": "integer", "minimum": 1, "maximum": 4294967295}}}
			}
			if c.ID == "workspace.export" {
				c.InputSchema = map[string]any{"type": "object", "additionalProperties": false, "required": []string{"workspace", "output"}, "properties": map[string]any{"workspace": stringSchema, "output": stringSchema, "draft": map[string]any{"type": "boolean"}, "strict": map[string]any{"type": "boolean"}, "timeout": map[string]any{"type": "integer", "minimum": 1, "maximum": 4294967295}}}
			}
		}
		c.OutputSchema = map[string]any{"type": "object"}
		if c.SupportedFeatures == nil {
			c.SupportedFeatures = []string{"explicit path/identity/rootfile", "immutable initial and accepted revisions", "real full-tree diff and actual old/new metadata or content", "audited accept/reject", "accepted-only export"}
		}
		c.Preconditions = []string{"exclusive workspace owner; external writers stopped", "exact baseline and persisted plan/task provenance", "plan/export output absent and outside workspace root"}
		c.PostChecks = []string{"actual full tree hash/write set", "formal checks for acceptance and final ZIP export; no passed flag", "partial CSS coverage remains diagnostic, not permission to rename"}
		c.Idempotency = "no overwrite; plans/tasks bound to revision; consumed plans cannot be reapplied; no-op changed:false"
		out = append(out, c)
	}
	for _, c := range []Capability{
		{ID: "resource.rename", Mutates: true}, {ID: "workspace.list", Commands: []string{"workspace list"}},
		{ID: "preview", Commands: []string{"preview", "serve"}, RequiresGUI: true}, {ID: "amp", Commands: []string{"amp", "task run"}, RequiresModel: true},
	} {
		c.Version = 1
		c.Status = "planned"
		c.Reason = "Not implemented in M1-A; schemas/requirements not yet frozen"
		out = append(out, c)
	}
	for _, command := range describeCommands(out) {
		for i := range out {
			for _, name := range out[i].Commands {
				if name == command.Name {
					out[i].CommandSchemas = append(out[i].CommandSchemas, command)
				}
			}
		}
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
