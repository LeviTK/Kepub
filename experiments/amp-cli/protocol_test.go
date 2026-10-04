//go:build linux || darwin

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestContextPresenceAndWireRequestID(t *testing.T) {
	for _, tc := range []struct {
		name, fields, want string
	}{
		{"omitted", `{}`, ""},
		{"unknown-only", `{"unknown":"do not send"}`, ""},
		{"empty-strings", `{"bookPath":"","fragment":"","selectedText":""}`, `{"bookPath":"","fragment":"","selectedText":""}`},
		{"explicit-nulls", `{"generation":null,"progression":null}`, `{"generation":null,"progression":null}`},
		{"selection", `{"selectedText":"中文🙂选区","progression":0.73,"generation":42,"unknown":123}`, `{"selectedText":"中文🙂选区","progression":0.73,"generation":42}`},
		{"zero", `{"generation":0,"progression":0}`, `{"generation":0,"progression":0}`},
		{"max-int64", `{"generation":9223372036854775807,"progression":1}`, `{"generation":9223372036854775807,"progression":1}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := fixture(t, "normal")
			data := withContext(s, tc.fields)
			var parsed Start
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatal(err)
			}
			if err := parsed.validate(nil); err != nil {
				t.Fatal(err)
			}
			var wire map[string]json.RawMessage
			if err := json.Unmarshal(ampInput(parsed), &wire); err != nil {
				t.Fatal(err)
			}
			var requestID string
			if len(wire) != 3 || json.Unmarshal(wire["request_id"], &requestID) != nil || requestID != s.RequestID || wire["requestId"] != nil {
				t.Fatalf("wrong CLI wire keys: %s", ampInput(parsed))
			}
			var msg struct {
				Role    string
				Content []struct{ Type, Text string }
			}
			if json.Unmarshal(wire["message"], &msg) != nil || msg.Role != "user" || len(msg.Content) != 1 || msg.Content[0].Type != "text" {
				t.Fatal("wrong user message")
			}
			text := msg.Content[0].Text
			if tc.want == "" {
				if text != s.Prompt {
					t.Fatalf("omitted context changed prompt: %q", text)
				}
				return
			}
			prefix := s.Prompt + "\n\nKepub task context (JSON):\n"
			if !strings.HasPrefix(text, prefix) {
				t.Fatalf("wrong prompt/separator: %q", text)
			}
			var got, want map[string]json.RawMessage
			if json.Unmarshal([]byte(strings.TrimPrefix(text, prefix)), &got) != nil || json.Unmarshal([]byte(tc.want), &want) != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("context differs (includes internal/unknown data or loses presence): %s", text)
			}
		})
	}
}

func withContext(s Start, fields string) []byte {
	var input, extra map[string]json.RawMessage
	json.Unmarshal(startJSON(s), &input)
	json.Unmarshal([]byte(fields), &extra)
	for key, value := range extra {
		input[key] = value
	}
	data, _ := json.Marshal(input)
	return append(data, '\n')
}

func TestInvalidContextRejectedBeforeSpawn(t *testing.T) {
	for _, fields := range []string{
		`{"bookPath":null}`, `{"fragment":null}`, `{"selectedText":null}`, `{"selectedText":17}`,
		`{"generation":-1}`, `{"generation":0.5}`, `{"generation":9223372036854775808}`, `{"generation":"1"}`,
		`{"progression":-0.01}`, `{"progression":1.01}`, `{"progression":"0.5"}`,
	} {
		t.Run(fields, func(t *testing.T) {
			s, cfg := fixture(t, "normal")
			var output bytes.Buffer
			if run(context.Background(), cfg, io.NopCloser(bytes.NewReader(withContext(s, fields))), &output, io.Discard) != 1 {
				t.Fatal("invalid context succeeded")
			}
			var event Event
			if json.Unmarshal(output.Bytes(), &event) != nil {
				t.Fatalf("expected only one pre-spawn failure: %s", output.String())
			}
			checkTerminal(t, []Event{event}, "failed", "INVALID_INPUT", true)
			if _, err := os.Stat(filepath.Join(s.CWD, "captured-input")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("invalid context spawned CLI")
			}
		})
	}
}
