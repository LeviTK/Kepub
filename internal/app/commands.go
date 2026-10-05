package app

import "strings"

// Command describes CLI syntax independently of operation parameters (for
// example metadata.set is an operation, not a directly executable command).
type Command struct {
	Name         string         `json:"name"`
	Status       string         `json:"implementationStatus"`
	OperationID  string         `json:"operationId"`
	Risk         string         `json:"risk"`
	Reason       string         `json:"reason"`
	Positional   string         `json:"positional,omitempty"`
	InputSchema  map[string]any `json:"inputSchema"`
	OutputSchema any            `json:"outputSchema"`
}

func Commands() []Command {
	return describeCommands(Capabilities())
}

func describeCommands(capabilities []Capability) []Command {
	var commands []Command
	seen := map[string]bool{}
	for _, capability := range capabilities {
		for _, name := range capability.Commands {
			if seen[name] {
				continue
			}
			seen[name] = true
			schema, _ := capability.InputSchema.(map[string]any)
			properties := map[string]any{}
			required := []string{}
			if schema != nil {
				p, _ := schema["properties"].(map[string]any)
				for k, v := range p {
					properties[k] = v
				}
				if r, ok := schema["required"].([]string); ok {
					required = append(required, r...)
				}
			}
			if name == "info" || name == "toc" {
				delete(properties, "section")
				delete(properties, "resource")
				delete(properties, "direction")
			}
			if name == "inspect" {
				required = append(required, "section")
			}
			positional := ""
			if _, ok := properties["book"]; ok {
				positional = "book"
			}
			if strings.HasPrefix(name, "task ") && name != "task run" {
				positional = "task"
			}
			if name == "workspace export" {
				positional = "workspace"
			}
			for _, key := range []string{"json", "no-input", "help"} {
				properties[key] = map[string]any{"type": "boolean"}
			}
			input := map[string]any{"type": "object", "additionalProperties": false, "properties": properties, "required": required}
			if name == "inspect" {
				input["dependentRequired"] = map[string]any{"direction": []string{"resource"}}
				input["allOf"] = []any{map[string]any{"if": map[string]any{"anyOf": []any{map[string]any{"required": []string{"resource"}}, map[string]any{"required": []string{"direction"}}}}, "then": map[string]any{"properties": map[string]any{"section": map[string]any{"const": "references"}}, "required": []string{"section"}}}}
			}
			commands = append(commands, Command{Name: name, Status: capability.Status, OperationID: capability.ID, Risk: capability.Risk, Reason: capability.Reason, Positional: positional, InputSchema: input, OutputSchema: capability.OutputSchema})
		}
	}
	return commands
}

func Help(name string) any {
	commands := Commands()
	if name != "" {
		filtered := []Command{}
		for _, c := range commands {
			if c.Name == name || strings.HasPrefix(c.Name, name+" ") {
				filtered = append(filtered, c)
			}
		}
		commands = filtered
	}
	return map[string]any{"usage": "kepub COMMAND [TARGET] [--option VALUE]; -- ends options; -o aliases --output; -h aliases --help", "commands": commands}
}
