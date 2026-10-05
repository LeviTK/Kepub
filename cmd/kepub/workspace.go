package main

import (
	"context"

	"github.com/LeviTK/Kepub/internal/app"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/validation"
)

func executeWorkspace(ctx context.Context, o options) (any, error) {
	// Syntax and planned status have already been checked by validateCommand.
	v := validation.Options{Strict: o.strict, Draft: o.draft, Timeout: o.timeout}
	switch o.command {
	case "workspace":
		switch o.action {
		case "open":
			return app.OpenWorkspace(o.book, o.output, o.rootfile)
		case "export":
			return app.ExportWorkspace(ctx, o.book, o.output, v)
		}
	case "plan":
		return app.PlanWorkspace(o.workspace, o.operations, o.output)
	case "apply":
		return app.ApplyWorkspace(o.workspace, o.plan)
	case "task":
		return app.WorkspaceTask(ctx, o.workspace, o.book, o.action, v)
	}
	return nil, fault.New(2, "INVALID_ARGUMENT", "unknown workspace command")
}
