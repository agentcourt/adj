package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type PiToolLog struct {
	Participant string
	Path        string
}

type piToolCall struct {
	Name    string
	Command string
	Output  string
	Status  string
}

// AppendPiToolActivity appends activity from closed process logs to a completed digest.
func AppendPiToolActivity(digestPath string, logs []PiToolLog) error {
	if len(logs) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("\n## Pi Tool Activity\n\n")
	b.WriteString("Counts reflect Pi's reported tool outcomes, with missing results marked \"No result.\"  Bash excerpts follow tool-start order within each process, and call numbers count all tools in that process.  The logs provide no court-turn association.\n")
	for _, log := range logs {
		calls, err := readPiToolCalls(log.Path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(filepath.Dir(digestPath), log.Path)
		if err != nil {
			return fmt.Errorf("locate Pi activity log: %w", err)
		}
		fmt.Fprintf(&b, "\n### %s\n\nSource: `%s`\n\n", log.Participant, rel)
		if len(calls) == 0 {
			b.WriteString("No tool execution events were recorded in this log.\n")
			continue
		}
		counts := map[string][3]int{}
		var names []string
		for _, call := range calls {
			count, seen := counts[call.Name]
			if !seen {
				names = append(names, call.Name)
			}
			switch call.Status {
			case "succeeded":
				count[0]++
			case "failed":
				count[1]++
			default:
				count[2]++
			}
			counts[call.Name] = count
		}
		b.WriteString("| Tool | Succeeded | Failed | No result |\n|---|---|---|---|\n")
		for _, name := range names {
			count := counts[name]
			fmt.Fprintf(&b, "| %s | %d | %d | %d |\n", escapePipe(name), count[0], count[1], count[2])
		}
		if _, hasBash := counts["bash"]; hasBash {
			b.WriteString("\n| Call | Bash status | Command excerpt | Output excerpt |\n|---|---|---|---|\n")
			for i, call := range calls {
				if call.Name == "bash" {
					fmt.Fprintf(&b, "| %d | %s | %s | %s |\n", i+1, call.Status, toolExcerpt(call.Command), toolExcerpt(call.Output))
				}
			}
		}
	}
	f, err := os.OpenFile(digestPath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("open digest for Pi tool activity: %w", err)
	}
	_, writeErr := io.WriteString(f, b.String())
	if err := errors.Join(writeErr, f.Close()); err != nil {
		return fmt.Errorf("append Pi tool activity: %w", err)
	}
	return nil
}

func readPiToolCalls(path string) (calls []piToolCall, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open Pi tool log: %w", err)
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	decoder := json.NewDecoder(f)
	byID := map[string]int{}
	for {
		var event struct {
			Type       string `json:"type"`
			ToolCallID string `json:"toolCallId"`
			ToolName   string `json:"toolName"`
			IsError    bool   `json:"isError"`
			Args       struct {
				Command string `json:"command"`
				Tool    string `json:"tool"`
			} `json:"args"`
			Result struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				return calls, nil
			}
			return nil, fmt.Errorf("read Pi tool log %s at byte %d: %w", path, decoder.InputOffset(), err)
		}
		switch event.Type {
		case "tool_execution_start":
			if event.ToolCallID == "" {
				return nil, fmt.Errorf("Pi tool start in %s lacks toolCallId", path)
			}
			if _, exists := byID[event.ToolCallID]; exists {
				return nil, fmt.Errorf("duplicate Pi tool start %s in %s", event.ToolCallID, path)
			}
			name := event.ToolName
			if name == "" {
				name = "(empty tool name)"
			}
			if name == "mcp" && event.Args.Tool != "" {
				name += ": " + event.Args.Tool
			}
			byID[event.ToolCallID] = len(calls)
			calls = append(calls, piToolCall{Name: name, Command: event.Args.Command, Status: "no result"})
		case "tool_execution_end":
			index, exists := byID[event.ToolCallID]
			if !exists {
				return nil, fmt.Errorf("Pi tool result %s in %s has no start", event.ToolCallID, path)
			}
			call := &calls[index]
			delete(byID, event.ToolCallID)
			call.Status = "succeeded"
			if event.IsError {
				call.Status = "failed"
			}
			if call.Name == "bash" {
				var parts []string
				for _, content := range event.Result.Content {
					if content.Type == "text" {
						parts = append(parts, content.Text)
					}
				}
				call.Output = shortenAtWord(strings.Join(parts, "\n"), 160)
			}
		}
	}
}

func toolExcerpt(text string) string {
	return escapePipe(emptyNA(shortenAtWord(strings.Join(strings.Fields(text), " "), 160)))
}
