package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/LeviTK/Kepub/internal/app"
	"github.com/LeviTK/Kepub/internal/fault"
	"github.com/LeviTK/Kepub/internal/publication"
	"github.com/LeviTK/Kepub/internal/validation"
)

type options struct {
	command, book, section, output, rootfile string
	action, workspace, operations, plan      string
	resource, direction                      string
	selectValue, before, afterRevision       string
	afterTask                                string
	json, help                               bool
	strict, draft, emitRequest               bool
	timeout                                  time.Duration
	content                                  publication.ContentOptions
	seen                                     map[string]bool
}
type envelope struct {
	SchemaVersion int          `json:"schemaVersion"`
	OK            bool         `json:"ok"`
	Command       string       `json:"command"`
	RequestID     string       `json:"requestId"`
	Data          any          `json:"data"`
	Error         *fault.Error `json:"error"`
}

func parse(args []string) (o options, err error) {
	positional := []string{}
	defer func() {
		if len(positional) > 0 {
			o.command = positional[0]
		}
		if o.command == "workspace" || o.command == "task" || o.command == "fix" {
			o.book = ""
			if len(positional) > 1 {
				o.action = positional[1]
			}
			if len(positional) > 2 {
				o.book = positional[2]
			}
		} else if len(positional) > 1 {
			o.book = positional[1]
		}
	}()
	seen := map[string]bool{}
	o.seen = seen
	end := false
	for i := 0; i < len(args); i++ {
		s := args[i]
		if end {
			positional = append(positional, s)
			continue
		}
		if s == "--" {
			end = true
			continue
		}
		if s == "--json" {
			if seen[s] {
				return o, fault.New(2, "INVALID_ARGUMENT", "duplicate option %s", s)
			}
			seen[s] = true
			o.json = true
			continue
		}
		if s == "--no-input" {
			if seen[s] {
				return o, fault.New(2, "INVALID_ARGUMENT", "duplicate option %s", s)
			}
			seen[s] = true
			continue
		}
		if s == "--emit-request" {
			if seen[s] {
				return o, fault.New(2, "INVALID_ARGUMENT", "duplicate option %s", s)
			}
			seen[s] = true
			o.emitRequest = true
			continue
		}
		if s == "--strict" || s == "--draft" {
			if seen[s] {
				return o, fault.New(2, "INVALID_ARGUMENT", "duplicate option %s", s)
			}
			seen[s] = true
			if s == "--strict" {
				o.strict = true
			} else {
				o.draft = true
			}
			continue
		}
		if s == "--help" || s == "-h" {
			if seen["--help"] {
				return o, fault.New(2, "INVALID_ARGUMENT", "duplicate help")
			}
			seen["--help"] = true
			o.help = true
			continue
		}
		if strings.HasPrefix(s, "-") {
			key, val, has := strings.Cut(s, "=")
			if key == "-o" {
				key = "--output"
			}
			if !valueOption(key) {
				return o, fault.New(2, "INVALID_ARGUMENT", "unknown option %q", s)
			}
			if seen[key] {
				return o, fault.New(2, "INVALID_ARGUMENT", "duplicate option %s", key)
			}
			seen[key] = true
			if !has {
				i++
				if i >= len(args) {
					return o, fault.New(2, "INVALID_ARGUMENT", "missing option value for %s", key)
				}
				val = args[i]
			}
			if val == "" {
				return o, fault.New(2, "INVALID_ARGUMENT", "empty option value")
			}
			switch key {
			case "--workspace":
				o.workspace = val
			case "--operations":
				o.operations = val
			case "--plan":
				o.plan = val
			case "--select":
				o.selectValue = val
			case "--before":
				o.before = val
			case "--after-revision":
				o.afterRevision = val
			case "--after-task":
				o.afterTask = val
			case "--timeout":
				n, e := strconv.ParseUint(val, 10, 32)
				if e != nil || n == 0 {
					return o, fault.New(2, "INVALID_ARGUMENT", "--timeout requires positive integer seconds")
				}
				o.timeout = time.Duration(n) * time.Second
			case "--section":
				o.section = val
			case "--rootfile":
				o.rootfile = val
			case "--resource":
				o.resource = val
			case "--query":
				o.content.Query = &val
			case "--limit":
				n, e := strconv.Atoi(val)
				if e != nil {
					return o, fault.New(2, "INVALID_ARGUMENT", "--limit requires an integer between 1 and 200")
				}
				o.content.Limit = &n
			case "--direction":
				o.direction = val
			default:
				o.output = val
			}
			continue
		}
		positional = append(positional, s)
	}
	if len(positional) > 0 {
		o.command = positional[0]
	}
	if len(positional) > 1 {
		o.book = positional[1]
	}
	max := 2
	if len(positional) > 0 && (positional[0] == "workspace" || positional[0] == "task") {
		max = 3
	}
	if len(positional) > max {
		return o, fault.New(2, "INVALID_ARGUMENT", "too many positional arguments")
	}
	return o, nil
}

func execute(ctx context.Context, o options) (any, error) {
	name, err := validateCommand(o)
	if err != nil {
		return nil, err
	}
	if o.help {
		return app.Help(name), nil
	}
	if o.command == "version" {
		return app.BuildVersion(), nil
	}
	if o.command == "doctor" {
		return app.Doctor(ctx)
	}
	if o.command == "content" {
		return app.ContentWorkspace(o.workspace, o.resource, o.content)
	}
	if o.command == "search" {
		return app.SearchWorkspace(o.workspace, o.content)
	}
	if o.command == "workspace" || o.command == "task" || o.command == "plan" || o.command == "apply" {
		data, err := executeWorkspace(ctx, o)
		if err != nil && errors.Is(ctx.Err(), context.Canceled) {
			err = fault.New(130, "CANCELLED", "request cancelled")
		}
		return data, err
	}
	if o.command == "fix" {
		switch o.action {
		case "propose":
			return app.FixPropose(o.workspace, o.selectValue, o.emitRequest, o.output, o.json)
		case "delta":
			return app.FixDelta(ctx, o.workspace, o.before, o.afterRevision, o.afterTask, o.output, validation.Options{Strict: o.strict, Timeout: o.timeout}, o.json)
		}
	}
	if o.command == "capabilities" {
		return app.Capabilities(), nil
	}
	if o.command == "validate" || o.command == "pack" {
		v := validation.Options{Rootfile: o.rootfile, Strict: o.strict, Draft: o.draft, Timeout: o.timeout}
		var data any
		var err error
		if o.command == "validate" {
			data, err = validation.Validate(ctx, o.book, v)
		} else {
			data, err = app.Pack(ctx, o.book, o.output, v)
		}
		// Successful pack has crossed its atomic commit point. A later signal
		// must not turn that published artifact into a cancellation failure.
		if err != nil && errors.Is(ctx.Err(), context.Canceled) {
			err = fault.New(130, "CANCELLED", "request cancelled")
		}
		return data, err
	}
	a, p, err := app.Read(o.book, o.rootfile)
	if err != nil {
		return nil, err
	}
	defer a.Close()
	if o.command == "unpack" {
		if err = a.Unpack(o.output); err != nil {
			return nil, err
		}
		return map[string]any{"output": o.output, "rootfile": p.Rootfile, "files": len(a.Files), "limitations": p.Limitations}, nil
	}
	if o.command == "info" {
		return p, nil
	}
	if o.command == "toc" {
		o.section = "navigation"
	}
	return app.Inspect(a, p, o.section, o.resource, o.direction)
}

func run(args []string, stdout, stderr io.Writer) int {
	o, err := parse(args)
	machine := o.json
	if err != nil {
		// Find later --json after a parse error using the same token boundaries:
		// option values and filenames after -- are not flags.
		for i := 0; i < len(args); i++ {
			s := args[i]
			if s == "--" {
				break
			}
			if s == "--json" {
				machine = true
			}
			key, _, has := strings.Cut(s, "=")
			if key == "-o" {
				key = "--output"
			}
			if valueOption(key) && !has {
				i++
			}
		}
	}
	if o.command == "" && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		o.command = args[0]
	}
	var data any
	if err == nil {
		ctx := context.Background()
		if o.command == "doctor" || o.command == "validate" || o.command == "pack" || o.command == "workspace" && o.action == "export" || o.command == "task" && o.action == "accept" || o.command == "fix" && o.action == "delta" {
			var stop context.CancelFunc
			ctx, stop = signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
			defer stop()
			if o.timeout != 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, o.timeout)
				defer cancel()
			}
		}
		data, err = execute(ctx, o)
	}
	code := 0
	var fe *fault.Error
	if err != nil {
		if !errors.As(err, &fe) {
			fe = &fault.Error{Code: "IO_ERROR", Message: err.Error(), Exit: 6}
		}
		code = fe.Exit
	}
	if machine {
		id := make([]byte, 16)
		if _, e := rand.Read(id); e != nil {
			fmt.Fprintln(stderr, "request ID generation failed")
			return 6
		}
		e := envelope{1, err == nil, o.command, hex.EncodeToString(id), data, fe}
		if e := json.NewEncoder(stdout).Encode(e); e != nil {
			fmt.Fprintln(stderr, "output:", e)
			return 6
		}
	} else if err != nil {
		fmt.Fprintf(stderr, "%s: %s\n", fe.Code, fe.Message)
	} else if raw, ok := data.(app.RawDocument); ok {
		if _, e := stdout.Write(raw.Bytes); e != nil {
			fmt.Fprintln(stderr, "output:", e)
			return 6
		}
	} else {
		b, e := humanSummary(strings.TrimSpace(o.command+" "+o.action), data)
		if e != nil {
			return 6
		}
		if _, e := io.WriteString(stdout, b); e != nil {
			fmt.Fprintln(stderr, "output:", e)
			return 6
		}
	}
	return code
}
func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
