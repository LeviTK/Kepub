package app

import (
	"regexp"
	"testing"
)

// TestStructureCapabilitySchemaAcceptsLocators keeps the advertised schemas for
// the structural operations consistent with the implementation: every locator
// field must accept an exact structural locator, and attribute names stay
// NCName-shaped.
func TestStructureCapabilitySchemaAcceptsLocators(t *testing.T) {
	locator := "/html[1]/body[1]/p[1]"
	locatorFields := map[string][]string{
		"xhtml.attribute.set":    {"locator"},
		"xhtml.attribute.remove": {"locator"},
		"xhtml.element.insert":   {"locator"},
		"xhtml.element.replace":  {"locator"},
		"xhtml.element.delete":   {"locator"},
		"xhtml.element.move":     {"locator", "anchor"},
	}
	seen := map[string]bool{}
	for _, c := range Capabilities() {
		fields, ok := locatorFields[c.ID]
		if !ok {
			continue
		}
		seen[c.ID] = true
		properties := c.InputSchema.(map[string]any)["properties"].(map[string]any)
		for _, field := range fields {
			schema, ok := properties[field].(map[string]any)
			if !ok {
				t.Fatalf("%s: missing %s schema", c.ID, field)
			}
			if pattern, ok := schema["pattern"].(string); ok {
				valid, err := regexp.MatchString(pattern, locator)
				if err != nil {
					t.Fatalf("%s %s pattern: %v", c.ID, field, err)
				}
				if !valid {
					t.Fatalf("%s advertised %s pattern %q rejects the required locator", c.ID, field, pattern)
				}
			}
			if schema["minLength"] != 1 {
				t.Fatalf("%s %s must require at least one byte: %+v", c.ID, field, schema)
			}
		}
	}
	for id := range locatorFields {
		if !seen[id] {
			t.Fatalf("missing capability %s", id)
		}
	}
	// An attribute name stays a plain XML name without a namespace prefix.
	for _, c := range Capabilities() {
		if c.ID != "xhtml.attribute.set" && c.ID != "xhtml.attribute.remove" {
			continue
		}
		properties := c.InputSchema.(map[string]any)["properties"].(map[string]any)
		name := properties["name"].(map[string]any)
		pattern, ok := name["pattern"].(string)
		if !ok {
			t.Fatalf("%s: attribute name needs a pattern", c.ID)
		}
		valid, err := regexp.MatchString(pattern, "type")
		if err != nil || !valid {
			t.Fatalf("%s attribute name pattern %q: %v", c.ID, pattern, err)
		}
	}
}
