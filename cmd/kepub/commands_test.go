package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/LeviTK/Kepub/internal/app"
)

func TestCommandOptionMatrix(t *testing.T) {
	// This expectation is independent of the schemas: an extra accepted option
	// or an advertised but unparseable option both fail the cross-product.
	want := map[string]string{
		"capabilities": "", "version": "", "doctor": "",
		"info": "rootfile", "toc": "rootfile",
		"inspect": "rootfile section resource direction",
		"unpack":  "rootfile output", "validate": "rootfile strict timeout", "pack": "rootfile output strict draft timeout",
		"workspace open": "rootfile output", "workspace export": "output strict draft timeout",
		"plan": "workspace operations output", "apply": "workspace plan",
		"task diff": "workspace", "task accept": "workspace strict timeout", "task reject": "workspace",
		"workspace list": "", "preview": "", "serve": "", "amp": "", "task run": "",
	}
	samples := map[string]string{"rootfile": "EPUB/package.opf", "section": "references", "resource": "EPUB/chapter.xhtml", "direction": "incoming", "output": "out", "strict": "", "draft": "", "timeout": "2", "workspace": "ws", "operations": "ops.json", "plan": "plan.json"}
	for _, c := range app.Commands() {
		allowed, ok := want[c.Name]
		if !ok {
			t.Fatalf("command without independent expectation: %s", c.Name)
		}
		delete(want, c.Name)
		properties := c.InputSchema["properties"].(map[string]any)
		for key, val := range samples {
			args := append(strings.Fields(c.Name), "--help", "--"+key)
			if val != "" {
				args = append(args, val)
			}
			if c.Name == "inspect" && key == "direction" {
				args = append(args, "--section", "references", "--resource", "EPUB/chapter.xhtml")
			}
			if c.Name == "inspect" && key == "resource" {
				args = append(args, "--section", "references")
			}
			if c.Name == "inspect" && key != "section" && key != "resource" && key != "direction" {
				args = append(args, "--section", "references")
			}
			isAllowed := strings.Contains(" "+allowed+" ", " "+key+" ")
			_, advertised := properties[key]
			if key == c.Positional {
				advertised = false
			}
			if advertised != isAllowed {
				t.Fatalf("%s schema mismatch for %s", c.Name, key)
			}
			exit := 2
			if isAllowed {
				exit = 0
			}
			invoke(t, append(args, "--json"), exit)
		}
		invoke(t, append(strings.Fields(c.Name), "--help", "--json"), 0)
	}
	if len(want) != 0 {
		t.Fatalf("missing commands: %v", want)
	}
}

func TestHelpDoesNotHideInvalidInput(t *testing.T) {
	for _, args := range [][]string{
		{"unknown", "--help"}, {"task", "unknown", "--help"}, {"workspace", "unknown", "--help"},
		{"info", "--help", "--unknown"}, {"version", "target", "--help"},
		{"inspect", "--help", "--section", "invalid"}, {"info", "missing", "--output", "out"},
		{"inspect", "--help", "--direction", "sideways"},
		{"inspect", "--help", "--direction", "incoming"},
		{"inspect", "--help", "--resource", "../bad"},
		{"inspect", "--help", "--resource", "EPUB/chapter.xhtml"},
		{"task", "diff", "missing", "--workspace", "missing", "--rootfile", "x"},
		{"--help", "--output", "x"}, {"doctor", "--timeout", "1"},
		{"info", "missing", "--rootfile", "../package.opf"},
		{"version", "--json", "--json"}, {"info", "--help", "-h"}, {"info", "--help", "--timeout", "0"},
	} {
		invoke(t, append(args, "--json"), 2)
	}
	invoke(t, []string{"inspect", "--help", "--json"}, 0)
	for _, name := range []string{"workspace list", "task run", "preview", "serve", "amp"} {
		invoke(t, append(strings.Fields(name), "--json"), 3)
	}
}

func TestVersionAndDoctorWithoutRuntimeDependencies(t *testing.T) {
	bin := t.TempDir()
	marker := filepath.Join(bin, "called")
	// Optional Amp must be discovered without running even --version/login.
	if e := os.WriteFile(filepath.Join(bin, "amp"), []byte("#!/bin/sh\nprintf called > '"+marker+"'\n"), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", bin)
	t.Setenv("KEPUB_EPUBCHECK_JAR", "private-missing-path")
	t.Setenv("AMP_API_KEY", "sensitive-value-must-not-appear")
	v := invoke(t, []string{"version", "--json"}, 0)["data"].(map[string]any)
	if v["goVersion"] == "" || v["os"] == "" || v["arch"] == "" || v["development"] != true {
		t.Fatal(v)
	}
	d := invoke(t, []string{"doctor", "--json"}, 0)["data"].(map[string]any)
	if d["formalValidationAvailable"] != false || d["java"].(map[string]any)["status"] != "unavailable" || d["amp"].(map[string]any)["status"] != "discovered" {
		t.Fatal(d)
	}
	if _, e := os.Stat(marker); !os.IsNotExist(e) {
		t.Fatal("doctor executed Amp", e)
	}
	var out, stderr bytes.Buffer
	if code := run([]string{"doctor", "--json"}, &out, &stderr); code != 0 || strings.Contains(out.String(), "sensitive-value") || strings.Contains(out.String(), "private-missing-path") || stderr.Len() != 0 {
		t.Fatal(code, out.String(), stderr.String())
	}
}

func TestHelpAndCapabilityDescriptionsMatch(t *testing.T) {
	help := invoke(t, []string{"--help", "--json"}, 0)["data"].(map[string]any)["commands"].([]any)
	capabilities := invoke(t, []string{"capabilities", "--json"}, 0)["data"].([]any)
	descriptions := map[string]any{}
	for _, value := range capabilities {
		c := value.(map[string]any)
		if schemas, ok := c["commandSchemas"].([]any); ok {
			for _, schema := range schemas {
				s := schema.(map[string]any)
				descriptions[s["name"].(string)] = s
			}
		}
	}
	for _, h := range help {
		s := h.(map[string]any)
		name := s["name"].(string)
		if !reflect.DeepEqual(descriptions[name], h) {
			t.Fatal("capability/help mismatch", name)
		}
		b, _ := json.Marshal(app.Help(name))
		var single map[string]any
		if e := json.Unmarshal(b, &single); e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(single["commands"].([]any), []any{h}) {
			t.Fatal("command help mismatch", name)
		}
		delete(descriptions, name)
	}
	if len(descriptions) != 0 {
		t.Fatal("unadvertised commands", descriptions)
	}
}

func TestTimeoutSchemaUint32Boundary(t *testing.T) {
	want := map[string]bool{"publication.validate": true, "publication.pack": true, "task.accept": true, "workspace.export": true}
	capabilities := invoke(t, []string{"capabilities", "--json"}, 0)["data"].([]any)
	for _, value := range capabilities {
		c := value.(map[string]any)
		id := c["operationId"].(string)
		if !want[id] {
			continue
		}
		delete(want, id)
		source := c["inputSchema"].(map[string]any)["properties"].(map[string]any)["timeout"].(map[string]any)
		if source["minimum"] != float64(1) || source["maximum"] != float64(4294967295) {
			t.Fatal("timeout source boundary", id, source)
		}
		for _, schema := range c["commandSchemas"].([]any) {
			s := schema.(map[string]any)
			inherited := s["inputSchema"].(map[string]any)["properties"].(map[string]any)["timeout"]
			if !reflect.DeepEqual(source, inherited) {
				t.Fatal("timeout boundary not inherited", s["name"])
			}
			args := append(strings.Fields(s["name"].(string)), "--help", "--timeout", "4294967295", "--json")
			invoke(t, args, 0)
			o, e := parse(args)
			if e != nil || o.timeout != time.Duration(4294967295)*time.Second {
				t.Fatal("maximum timeout parsed incorrectly", o.timeout, e)
			}
			args[len(args)-2] = "4294967296"
			invoke(t, args, 2)
		}
	}
	if len(want) != 0 {
		t.Fatal("missing timeout schema", want)
	}
}
