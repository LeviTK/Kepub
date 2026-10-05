package main

import (
	"strings"

	"github.com/LeviTK/Kepub/internal/app"
	"github.com/LeviTK/Kepub/internal/bookpath"
	"github.com/LeviTK/Kepub/internal/fault"
)

func valueOption(option string) bool {
	key := strings.TrimPrefix(option, "--")
	if key == option {
		return false
	}
	for _, c := range app.Commands() {
		if key == c.Positional {
			continue
		}
		if s, ok := c.InputSchema["properties"].(map[string]any)[key].(map[string]any); ok && s["type"] != "boolean" {
			return true
		}
	}
	return false
}

func validateCommand(o options) (string, error) {
	name := o.command
	if o.action != "" {
		name += " " + o.action
	}
	bad := func(message string) (string, error) { return name, fault.New(2, "INVALID_ARGUMENT", "%s", message) }
	if name == "" && o.help {
		for key := range o.seen {
			if key != "--help" && key != "--json" && key != "--no-input" {
				return bad("option requires a command")
			}
		}
		return name, nil
	}
	for _, c := range app.Commands() {
		if c.Name != name {
			continue
		}
		properties := c.InputSchema["properties"].(map[string]any)
		for key := range o.seen {
			k := strings.TrimPrefix(key, "--")
			if k == c.Positional {
				return bad("target must be positional")
			}
			if _, ok := properties[k]; !ok {
				return bad("option " + key + " is not supported by " + name)
			}
		}
		if c.Positional == "" && o.book != "" {
			return bad("unexpected positional target")
		}
		if o.rootfile != "" {
			if _, err := bookpath.Parse(o.rootfile); err != nil {
				return bad("rootfile must be a canonical BookPath")
			}
		}
		if !o.help {
			values := map[string]string{"book": o.book, "task": o.book, "workspace": o.workspace, "operations": o.operations, "plan": o.plan, "output": o.output, "section": o.section}
			if c.Positional == "workspace" {
				values["workspace"] = o.book
			}
			for _, key := range c.InputSchema["required"].([]string) {
				if values[key] == "" {
					return bad("missing required " + key)
				}
			}
		}
		if name == "inspect" && (o.section != "" || !o.help) {
			if err := app.ValidateInspect(o.section, o.resource, o.direction); err != nil {
				return name, err
			}
		}
		if c.Status == "planned" && !o.help {
			return name, fault.New(3, "CAPABILITY_UNAVAILABLE", "%s is not implemented", name)
		}
		return name, nil
	}
	// Groups have overview help, but never accept arguments as a hidden command.
	if (name == "workspace" || name == "task") && o.help && o.book == "" {
		for key := range o.seen {
			if key != "--help" && key != "--json" && key != "--no-input" {
				return bad("option requires a subcommand")
			}
		}
		return name, nil
	}
	return bad("unknown/missing command " + name)
}
