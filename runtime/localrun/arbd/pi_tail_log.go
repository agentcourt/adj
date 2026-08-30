package localrun

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
)

const repeatedMessageUpdateLogFilterMessage = "earlier repeated message_update events dropped"

type piTailLogWriter struct {
	mu     sync.Mutex
	dst    io.Writer
	buf    []byte
	filter piMessageUpdateTailFilter
}

func newPiTailLogWriter(dst io.Writer) *piTailLogWriter {
	return &piTailLogWriter{dst: dst}
}

func (w *piTailLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	written := 0
	for len(p) > 0 {
		index := bytes.IndexByte(p, '\n')
		if index < 0 {
			w.buf = append(w.buf, p...)
			written += len(p)
			return written, nil
		}
		w.buf = append(w.buf, p[:index]...)
		written += index + 1
		if err := w.writeBufferedLine(true); err != nil {
			return written, err
		}
		p = p[index+1:]
	}
	return written, nil
}

func (w *piTailLogWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.buf) == 0 {
		return nil
	}
	return w.writeBufferedLine(false)
}

func (w *piTailLogWriter) writeBufferedLine(newline bool) error {
	line := w.filter.filterLine(w.buf)
	w.buf = nil
	if _, err := w.dst.Write(line); err != nil {
		return err
	}
	if newline {
		_, err := w.dst.Write([]byte("\n"))
		return err
	}
	return nil
}

type piMessageUpdateTailFilter struct {
	previous map[string]string
}

type piAccumulatedContent struct {
	responseID   string
	contentIndex int
	field        string
	value        string
}

func (f *piMessageUpdateTailFilter) filterLine(line []byte) []byte {
	var event map[string]any
	if err := json.Unmarshal(line, &event); err != nil {
		return line
	}
	if piMapString(event["type"]) != "message_update" {
		return line
	}

	content, ok := piMessageUpdateContent(event)
	if !ok {
		return line
	}
	if f.previous == nil {
		f.previous = map[string]string{}
	}
	key := content.key()
	previous, found := f.previous[key]
	f.previous[key] = content.value
	if !found || previous == "" || !strings.HasPrefix(content.value, previous) {
		return line
	}

	tail := content.value[len(previous):]
	replaced := replacePiAccumulatedContent(event, content, tail)
	if replaced == 0 {
		return line
	}
	event["aard_log_filter"] = map[string]any{
		"message":              repeatedMessageUpdateLogFilterMessage,
		"response_id":          content.responseID,
		"content_index":        content.contentIndex,
		"field":                content.field,
		"dropped_prefix_bytes": len(previous),
		"tail_bytes":           len(tail),
		"replaced_fields":      replaced,
	}
	out, err := json.Marshal(event)
	if err != nil {
		return line
	}
	return out
}

func (c piAccumulatedContent) key() string {
	return fmt.Sprintf("%s:%d:%s", c.responseID, c.contentIndex, c.field)
}

func piMessageUpdateContent(event map[string]any) (piAccumulatedContent, bool) {
	assistantEvent, ok := piMapValue(event["assistantMessageEvent"])
	if !ok {
		return piAccumulatedContent{}, false
	}
	contentIndex, ok := piMapInt(assistantEvent["contentIndex"])
	if !ok {
		return piAccumulatedContent{}, false
	}
	partial, ok := piMapValue(assistantEvent["partial"])
	if !ok {
		return piAccumulatedContent{}, false
	}
	responseID := piMapString(partial["responseId"])
	if responseID == "" {
		message, ok := piMapValue(event["message"])
		if ok {
			responseID = piMapString(message["responseId"])
		}
	}
	if responseID == "" {
		return piAccumulatedContent{}, false
	}
	field, value, ok := piContentString(partial, contentIndex)
	if !ok {
		return piAccumulatedContent{}, false
	}
	return piAccumulatedContent{
		responseID:   responseID,
		contentIndex: contentIndex,
		field:        field,
		value:        value,
	}, true
}

func replacePiAccumulatedContent(event map[string]any, content piAccumulatedContent, tail string) int {
	replaced := 0
	assistantEvent, ok := piMapValue(event["assistantMessageEvent"])
	if ok {
		partial, ok := piMapValue(assistantEvent["partial"])
		if ok {
			replaced += replacePiContentString(partial, content, tail)
		}
		message, ok := piMapValue(assistantEvent["message"])
		if ok {
			replaced += replacePiContentString(message, content, tail)
		}
	}
	message, ok := piMapValue(event["message"])
	if ok {
		replaced += replacePiContentString(message, content, tail)
	}
	return replaced
}

func piContentString(message map[string]any, contentIndex int) (string, string, bool) {
	item, ok := piContentItem(message, contentIndex)
	if !ok {
		return "", "", false
	}
	if value, ok := item["thinking"].(string); ok {
		return "thinking", value, true
	}
	if value, ok := item["text"].(string); ok {
		return "text", value, true
	}
	return "", "", false
}

func replacePiContentString(message map[string]any, content piAccumulatedContent, tail string) int {
	responseID := piMapString(message["responseId"])
	if responseID != "" && responseID != content.responseID {
		return 0
	}
	item, ok := piContentItem(message, content.contentIndex)
	if !ok {
		return 0
	}
	value, ok := item[content.field].(string)
	if !ok || value != content.value {
		return 0
	}
	item[content.field] = tail
	return 1
}

func piContentItem(message map[string]any, contentIndex int) (map[string]any, bool) {
	content, ok := message["content"].([]any)
	if !ok || contentIndex < 0 || contentIndex >= len(content) {
		return nil, false
	}
	item, ok := content[contentIndex].(map[string]any)
	return item, ok
}

func piMapValue(value any) (map[string]any, bool) {
	out, ok := value.(map[string]any)
	return out, ok
}

func piMapString(value any) string {
	out, _ := value.(string)
	return out
}

func piMapInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		if int64(int(v)) != v {
			return 0, false
		}
		return int(v), true
	case float64:
		out := int(v)
		if float64(out) != v {
			return 0, false
		}
		return out, true
	default:
		return 0, false
	}
}
