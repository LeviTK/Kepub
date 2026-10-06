package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/app"
)

// Terminal summaries intentionally abbreviate long lists/text; JSON remains the
// complete machine contract. Untrusted text cannot emit terminal control codes.
func humanSummary(command string, data any) (string, error) {
	if s, ok := data.(string); ok {
		return s, nil
	} // already escaped actual-file diff
	var out strings.Builder
	fmt.Fprintf(&out, "Kepub · %s\n", command)
	if cs, ok := data.([]app.Capability); ok {
		for _, c := range cs {
			fmt.Fprintf(&out, "%s v%d · %s · %s\n", c.ID, c.Version, c.Status, strconv.QuoteToGraphic(c.Reason))
		}
		return out.String(), nil
	}
	if help, ok := data.(map[string]any); ok {
		if cs, ok := help["commands"].([]app.Command); ok {
			fmt.Fprintln(&out, help["usage"])
			for _, c := range cs {
				fmt.Fprintf(&out, "  %s %s · %s\n", c.Name, c.Positional, c.Status)
				keys := []string{}
				for k := range c.InputSchema["properties"].(map[string]any) {
					if k != c.Positional {
						keys = append(keys, "--"+k)
					}
				}
				sort.Strings(keys)
				fmt.Fprintf(&out, "    %s\n", strings.Join(keys, " "))
			}
			return out.String(), nil
		}
	}
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var v any
	if err := d.Decode(&v); err != nil {
		return "", err
	}
	var render func(any, int)
	render = func(v any, depth int) {
		pad := strings.Repeat("  ", depth)
		if depth > 5 {
			out.WriteString("(nested details omitted; use --json)\n")
			return
		}
		switch v := v.(type) {
		case map[string]any:
			out.WriteByte('\n')
			keys := []string{}
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(&out, "%s%s: ", pad, k)
				render(v[k], depth+1)
			}
		case []any:
			fmt.Fprintf(&out, "%d item(s)\n", len(v))
			for _, item := range v[:min(len(v), 10)] {
				fmt.Fprintf(&out, "%s- ", pad)
				render(item, depth+1)
			}
			if len(v) > 10 {
				fmt.Fprintf(&out, "%s%d more omitted; use --json\n", pad, len(v)-10)
			}
		case string:
			if len(v) > 1024 {
				end := 1024
				for !utf8.RuneStart(v[end]) {
					end--
				}
				v = v[:end] + "… (text omitted; use --json)"
			}
			fmt.Fprintln(&out, strconv.QuoteToGraphic(v))
		case nil:
			out.WriteString("none\n")
		default:
			fmt.Fprintln(&out, v)
		}
	}
	render(v, 0)
	return out.String(), nil
}
