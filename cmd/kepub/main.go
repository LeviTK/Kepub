package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/LeviTK/Kepub/internal/app"
	"github.com/LeviTK/Kepub/internal/fault"
)

type options struct {
	command, book, section, output, rootfile string
	json, help                               bool
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
		if len(positional) > 1 {
			o.book = positional[1]
		}
	}()
	seen := map[string]bool{}
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
			o.json = true
			continue
		}
		if s == "--no-input" {
			continue
		}
		if s == "--help" || s == "-h" {
			o.help = true
			continue
		}
		if strings.HasPrefix(s, "-") {
			key, val, has := strings.Cut(s, "=")
			switch key {
			case "--section", "--rootfile", "--output", "-o":
			default:
				return o, fault.New(2, "INVALID_ARGUMENT", "unknown option %q", s)
			}
			if key == "-o" {
				key = "--output"
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
			case "--section":
				o.section = val
			case "--rootfile":
				o.rootfile = val
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
	if len(positional) > 2 {
		return o, fault.New(2, "INVALID_ARGUMENT", "too many positional arguments")
	}
	return o, nil
}

func execute(o options) (any, error) {
	if o.help {
		return map[string]any{"usage": "kepub [--json] capabilities | info BOOK | inspect BOOK --section metadata|manifest|spine|capabilities | unpack BOOK --output DIR; --rootfile BOOK_PATH; -- ends options", "capabilities": app.Capabilities()}, nil
	}
	if o.command == "capabilities" {
		if o.book != "" || o.rootfile != "" || o.section != "" || o.output != "" {
			return nil, fault.New(2, "INVALID_ARGUMENT", "capabilities takes no target/options")
		}
		return app.Capabilities(), nil
	}
	switch o.command {
	case "info", "inspect", "unpack":
	default:
		for _, c := range app.Capabilities() {
			for _, command := range c.Commands {
				if command == o.command && c.Status == "planned" {
					return nil, fault.New(3, "CAPABILITY_UNAVAILABLE", "%s is not implemented", o.command)
				}
			}
		}
		return nil, fault.New(2, "INVALID_ARGUMENT", "unknown/missing command %q", o.command)
	}
	if o.book == "" {
		return nil, fault.New(2, "INVALID_ARGUMENT", "BOOK is required")
	}
	if o.command != "unpack" && o.output != "" {
		return nil, fault.New(2, "INVALID_ARGUMENT", "--output is only for unpack")
	}
	if o.command != "inspect" && o.section != "" {
		return nil, fault.New(2, "INVALID_ARGUMENT", "--section is only for inspect")
	}
	if o.command == "unpack" && o.output == "" {
		return nil, fault.New(2, "INVALID_ARGUMENT", "--output is required")
	}
	if o.command == "inspect" {
		switch o.section {
		case "metadata", "manifest", "spine", "capabilities":
		case "navigation", "references":
			return nil, fault.New(3, "CAPABILITY_UNAVAILABLE", "section %s is not implemented", o.section)
		default:
			return nil, fault.New(2, "INVALID_ARGUMENT", "supported --section is required")
		}
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
	var section any
	switch o.section {
	case "metadata":
		section = p.Metadata
	case "manifest":
		section = p.Manifest
	case "spine":
		section = map[string]any{"items": p.Spine, "attributes": p.SpineAttributes}
	case "capabilities":
		section = app.Capabilities()
	}
	return map[string]any{"rootfile": p.Rootfile, "section": o.section, "value": section, "limitations": p.Limitations}, nil
}

func run(args []string, stdout, stderr io.Writer) int {
	o, err := parse(args)
	// Detect --json even when an earlier malformed option stopped parsing; never
	// reinterpret a filename after -- as an option.
	machine := false
	for _, s := range args {
		if s == "--" {
			break
		}
		if s == "--json" {
			machine = true
		}
	}
	if o.command == "" && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		o.command = args[0]
	}
	var data any
	if err == nil {
		data, err = execute(o)
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
	} else {
		b, e := json.MarshalIndent(data, "", "  ")
		if e != nil {
			return 6
		}
		fmt.Fprintln(stdout, string(b))
	}
	return code
}
func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
