package main

import (
	"context"

	"github.com/LeviTK/Kepub/internal/app"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/validation"
)

func executeWorkspace(ctx context.Context, o options) (any, error) {
	bad := func() (any, error) {
		return nil, fault.New(2, "INVALID_ARGUMENT", "explicit workspace command target and matching options are required; see --help")
	}
	if o.section != "" || o.resource != "" || o.direction != "" {
		return bad()
	}
	checks := o.command == "workspace" && o.action == "export" || o.command == "task" && o.action == "accept"
	if !checks && (o.strict || o.timeout != 0) || o.draft && !(o.command == "workspace" && o.action == "export") {
		return bad()
	}
	v := validation.Options{Strict: o.strict, Draft: o.draft, Timeout: o.timeout}
	switch o.command {
	case "workspace":
		if o.action == "list" {
			return nil, fault.New(3, "CAPABILITY_UNAVAILABLE", "workspace list/registry is planned; use an explicit directory")
		}
		if o.workspace != "" || o.operations != "" || o.plan != "" || o.book == "" || o.output == "" {
			return bad()
		}
		switch o.action {
		case "open":
			return app.OpenWorkspace(o.book, o.output, o.rootfile)
		case "export":
			if o.rootfile != "" {
				return bad()
			}
			return app.ExportWorkspace(ctx, o.book, o.output, v)
		default:
			return bad()
		}
	case "plan":
		if o.book != "" || o.rootfile != "" || o.workspace == "" || o.operations == "" || o.output == "" || o.plan != "" {
			return bad()
		}
		return app.PlanWorkspace(o.workspace, o.operations, o.output)
	case "apply":
		if o.book != "" || o.rootfile != "" || o.workspace == "" || o.plan == "" || o.operations != "" || o.output != "" {
			return bad()
		}
		return app.ApplyWorkspace(o.workspace, o.plan)
	case "task":
		if o.action == "run" {
			return nil, fault.New(3, "CAPABILITY_UNAVAILABLE", "model task run is planned")
		}
		if o.book == "" || o.workspace == "" || o.output != "" || o.rootfile != "" || o.operations != "" || o.plan != "" {
			return bad()
		}
		if o.action != "diff" && o.action != "accept" && o.action != "reject" {
			return bad()
		}
		return app.WorkspaceTask(ctx, o.workspace, o.book, o.action, v)
	}
	return bad()
}
