package main

import (
	"bytes"
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/LeviTK/Kepub/internal/app"
)

// terminalQuote renders one untrusted text field, not program layout. Visible
// escapes preserve controls, invalid bytes and bidi formatting without executing
// them; JSON messages and already escaped task diffs must not pass through it.
func terminalQuote(text string) string { return strconv.QuoteToGraphic(text) }

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
			fmt.Fprintf(&out, "%s v%d · %s · %s\n", c.ID, c.Version, c.Status, terminalQuote(c.Reason))
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
	v, err := summaryValue(data)
	if err != nil {
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
				q := terminalQuote(k)
				fmt.Fprintf(&out, "%s%s: ", pad, q[1:len(q)-1])
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
				for i := 0; i < end; {
					_, size := utf8.DecodeRuneInString(v[i:])
					if i+size > end {
						end = i
						break
					}
					i += size
				}
				v = v[:end] + "… (text omitted; use --json)"
			}
			fmt.Fprintln(&out, terminalQuote(v))
		case nil:
			out.WriteString("none\n")
		default:
			fmt.Fprintln(&out, v)
		}
	}
	render(v, 0)
	return out.String(), nil
}

// Keep JSON's field selection, numbers, omission and custom encoders, but recover
// ordinary Go strings before their invalid bytes are lost to JSON replacement.
// This is a display-only copy; the machine encoder still receives the original.
func summaryValue(data any) (any, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var value any
	if err := d.Decode(&value); err != nil {
		return nil, err
	}
	return summaryStrings(reflect.ValueOf(data), value)
}

func summaryStrings(raw reflect.Value, value any) (any, error) {
	if !raw.IsValid() {
		return value, nil
	}
	if raw.CanInterface() {
		switch raw.Interface().(type) {
		case json.Marshaler, encoding.TextMarshaler:
			return value, nil
		}
	}
	switch raw.Kind() {
	case reflect.Interface, reflect.Pointer:
		if !raw.IsNil() {
			return summaryStrings(raw.Elem(), value)
		}
	case reflect.String:
		if _, ok := value.(string); ok {
			return raw.String(), nil
		}
	case reflect.Map:
		if raw.Type().Key().Kind() == reflect.String && !raw.IsNil() {
			m := make(map[string]any, raw.Len())
			iter := raw.MapRange()
			for iter.Next() {
				// Decode each entry independently: distinct raw keys may have
				// collided in the JSON object after UTF-8 replacement.
				v, err := summaryValue(iter.Value().Interface())
				if err != nil {
					return nil, err
				}
				m[iter.Key().String()] = v
			}
			return m, nil
		}
	case reflect.Slice, reflect.Array:
		if items, ok := value.([]any); ok {
			for i := range items {
				v, err := summaryStrings(raw.Index(i), items[i])
				if err != nil {
					return nil, err
				}
				items[i] = v
			}
		}
	case reflect.Struct:
		if fields, ok := value.(map[string]any); ok {
			for i := 0; i < raw.NumField(); i++ {
				field := raw.Type().Field(i)
				if !field.IsExported() {
					continue
				}
				tag := strings.Split(field.Tag.Get("json"), ",")
				name := tag[0]
				if name == "-" || slices.Contains(tag[1:], "string") {
					continue
				}
				if name == "" {
					if field.Anonymous {
						if _, err := summaryStrings(raw.Field(i), fields); err != nil {
							return nil, err
						}
						continue
					}
					name = field.Name
				}
				if v, exists := fields[name]; exists {
					restored, err := summaryStrings(raw.Field(i), v)
					if err != nil {
						return nil, err
					}
					fields[name] = restored
				}
			}
		}
	}
	return value, nil
}
