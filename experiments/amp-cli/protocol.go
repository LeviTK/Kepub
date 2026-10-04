package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"unicode/utf8"
)

const maxLine = 1 << 20

// Start is the experiment-v1 message, not Kepub's production API.
type Start struct {
	SchemaVersion int      `json:"schemaVersion"`
	Type          string   `json:"type"`
	RequestID     string   `json:"requestId"`
	WorkspaceID   string   `json:"workspaceId"`
	TaskID        string   `json:"taskId"`
	BaseRevision  string   `json:"baseRevision"`
	CWD           string   `json:"cwd"`
	Prompt        string   `json:"prompt"`
	ThreadID      string   `json:"threadId,omitempty"`
	BookPath      string   `json:"bookPath,omitempty"`
	Fragment      string   `json:"fragment,omitempty"`
	Progression   *float64 `json:"progression,omitempty"`
	SelectedText  string   `json:"selectedText,omitempty"`
	Generation    *int64   `json:"generation"`
}

type Binding struct {
	ThreadID     string `json:"threadId"`
	WorkspaceID  string `json:"workspaceId"`
	TaskID       string `json:"taskId"`
	BaseRevision string `json:"baseRevision"`
	CWD          string `json:"cwd"`
}

var threadPattern = regexp.MustCompile(`^T-[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func (s Start) validate(binding *Binding) error {
	if s.SchemaVersion != 1 || s.Type != "start" || s.RequestID == "" || len(s.RequestID) > 256 || s.WorkspaceID == "" || s.TaskID == "" || s.BaseRevision == "" || s.Prompt == "" {
		return errors.New("start requires schemaVersion 1 and explicit request/workspace/task/revision/prompt")
	}
	if !filepath.IsAbs(s.CWD) || filepath.Clean(s.CWD) != s.CWD {
		return errors.New("cwd must be an explicit clean absolute directory")
	}
	info, err := os.Stat(s.CWD)
	if err != nil || !info.IsDir() {
		return errors.New("cwd must exist and be a directory")
	}
	if s.Progression != nil && (*s.Progression < 0 || *s.Progression > 1) || s.Generation != nil && *s.Generation < 0 {
		return errors.New("invalid progression or generation")
	}
	if s.ThreadID != "" {
		if !threadPattern.MatchString(s.ThreadID) || binding == nil || *binding != (Binding{s.ThreadID, s.WorkspaceID, s.TaskID, s.BaseRevision, s.CWD}) {
			return errors.New("explicit threadId does not match the trusted host binding")
		}
	}
	return nil
}

func ampArgs(s Start) []string {
	args := []string{"--execute", "--stream-json", "--stream-json-input", "--executor", "local", "--visibility", "private", "--mode", "ultra", "--no-ide", "--no-color"}
	if s.ThreadID != "" {
		args = append(args, "threads", "continue", s.ThreadID)
	}
	return args
}

func ampInput(s Start) []byte {
	// Explicit supplied context only: no file reads, attachments or argument interpolation.
	context, _ := json.Marshal(s)
	value := map[string]any{"type": "user", "requestId": s.RequestID, "message": map[string]any{
		"role": "user", "content": []any{map[string]any{"type": "text", "text": "Kepub task context (JSON):\n" + string(context)}},
	}}
	line, _ := json.Marshal(value)
	return append(line, '\n')
}

type Event struct {
	SchemaVersion int            `json:"schemaVersion"`
	RequestID     string         `json:"requestId"`
	WorkspaceID   string         `json:"workspaceId"`
	TaskID        string         `json:"taskId"`
	Sequence      int            `json:"sequence"`
	Generation    *int64         `json:"generation"`
	Type          string         `json:"type"`
	Data          map[string]any `json:"data"`
}

type emitter struct {
	s   Start
	seq int
	out *json.Encoder
	err error
}

func (e *emitter) send(kind string, data map[string]any) {
	if e.err != nil {
		return
	}
	e.seq++
	e.err = e.out.Encode(Event{1, e.s.RequestID, e.s.WorkspaceID, e.s.TaskID, e.seq, e.s.Generation, kind, data})
}

type frame struct {
	line []byte
	err  error
}

// ReadSlice bounds memory before JSON decoding and preserves split UTF-8 bytes.
// A final non-newline-terminated record is deliberately an abnormal EOF.
func readFrame(r *bufio.Reader, limit int) ([]byte, error) {
	var line []byte
	for {
		part, err := r.ReadSlice('\n')
		if len(line)+len(part) > limit {
			return nil, errors.New("message exceeds byte limit")
		}
		line = append(line, part...)
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if err != nil {
			if errors.Is(err, io.EOF) && len(line) > 0 {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}
		if !utf8.Valid(line) {
			return nil, errors.New("invalid UTF-8")
		}
		return line, nil // Preserve whitespace in aggregate byte accounting.
	}
}

func pump(r io.Reader, limit int, done <-chan struct{}) <-chan frame {
	ch := make(chan frame, 1)
	go func() {
		defer close(ch)
		reader := bufio.NewReaderSize(r, 4096)
		for {
			line, err := readFrame(reader, limit)
			select {
			case ch <- frame{line, err}:
			case <-done:
				return
			}
			if err != nil {
				return
			}
		}
	}()
	return ch
}

type streamState struct {
	threadID string
	result   bool
	success  bool
	text     string
}

var errAmpResult = errors.New("Amp execution failed")

func (s *streamState) consume(line []byte, emit *emitter) error {
	var msg struct {
		Type      string  `json:"type"`
		Subtype   string  `json:"subtype"`
		SessionID string  `json:"session_id"`
		CWD       string  `json:"cwd"`
		IsError   *bool   `json:"is_error"`
		Result    *string `json:"result"`
		Message   struct {
			Content []json.RawMessage `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(line, &msg); err != nil || msg.Type == "" {
		return errors.New("malformed Amp JSON event")
	}
	if s.result {
		return errors.New("event after terminal Amp result")
	}
	if msg.SessionID != "" {
		if !threadPattern.MatchString(msg.SessionID) || s.threadID != "" && s.threadID != msg.SessionID {
			return errors.New("Amp thread ID mismatch")
		}
		s.threadID = msg.SessionID
	}
	switch msg.Type {
	case "system":
		if msg.Subtype == "init" && (msg.SessionID == "" || msg.CWD != emit.s.CWD) {
			return errors.New("Amp init context mismatch")
		}
	case "assistant", "user":
		for _, raw := range msg.Message.Content {
			var block struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if err := json.Unmarshal(raw, &block); err != nil {
				return errors.New("malformed Amp content block")
			}
			switch block.Type {
			case "text":
				if msg.Type == "assistant" {
					emit.send("assistant", map[string]any{"text": block.Text, "threadId": s.threadID})
				}
			case "tool_use", "tool_result":
				emit.send("tool", map[string]any{"content": raw, "threadId": s.threadID})
			}
		}
	case "result":
		if msg.SessionID == "" || msg.IsError == nil {
			return errors.New("Amp result missing session_id or is_error")
		}
		s.result, s.success = true, msg.Subtype == "success" && !*msg.IsError
		if !s.success {
			return fmt.Errorf("%w (%s)", errAmpResult, msg.Subtype)
		}
		if msg.Result == nil {
			return errors.New("Amp success missing result text")
		}
		s.text = *msg.Result
	}
	// Unknown fields, event types and content blocks are forward-compatible.
	return emit.err
}
