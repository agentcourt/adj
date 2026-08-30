package agent

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/agentcourt/adj/runtime/runstate"
)

// InspectOpenClawOutputFile reads the terminal JSON object emitted by OpenClaw.
func InspectOpenClawOutputFile(path string) (usage *runstate.TokenUsage, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open OpenClaw output %q: %w", path, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close OpenClaw output %q: %w", path, err))
		}
	}()
	usage, err = parseOpenClawOutput(file)
	if err != nil {
		return usage, fmt.Errorf("parse OpenClaw output %q: %w", path, err)
	}
	return usage, nil
}

func parseOpenClawOutput(reader io.Reader) (*runstate.TokenUsage, error) {
	jsonReader, err := openClawJSONReader(reader)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(jsonReader)
	var output struct {
		Payloads []struct {
			Text string `json:"text"`
		} `json:"payloads"`
		Meta struct {
			AgentMeta struct {
				SessionID string `json:"sessionId"`
				Provider  string `json:"provider"`
				Model     string `json:"model"`
			} `json:"agentMeta"`
			FinalAssistantVisibleText string `json:"finalAssistantVisibleText"`
			FinalAssistantRawText     string `json:"finalAssistantRawText"`
			StopReason                string `json:"stopReason"`
			Completion                struct {
				StopReason   string `json:"stopReason"`
				FinishReason string `json:"finishReason"`
				Refusal      bool   `json:"refusal"`
			} `json:"completion"`
		} `json:"meta"`
	}
	if err := decoder.Decode(&output); err != nil {
		return nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("trailing JSON")
		}
		return nil, err
	}
	if !output.Meta.Completion.Refusal {
		return nil, nil
	}
	return nil, &ProviderRefusalError{
		Provider:      strings.TrimSpace(output.Meta.AgentMeta.Provider),
		Model:         strings.TrimSpace(output.Meta.AgentMeta.Model),
		StopReason:    firstOpenClawValue(output.Meta.Completion.StopReason, output.Meta.StopReason, output.Meta.Completion.FinishReason),
		RawStopReason: firstOpenClawValue(output.Meta.Completion.FinishReason, output.Meta.StopReason, output.Meta.Completion.StopReason),
		WillRetry:     false,
		Explanation:   openClawExplanation(output.Meta.FinalAssistantVisibleText, output.Meta.FinalAssistantRawText, output.Payloads),
		SessionID:     strings.TrimSpace(output.Meta.AgentMeta.SessionID),
	}
}

func openClawJSONReader(reader io.Reader) (io.Reader, error) {
	buffered := bufio.NewReader(reader)
	atLineStart := true
	for {
		value, err := buffered.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, errors.New("OpenClaw output has no terminal JSON object")
			}
			return nil, err
		}
		if value == '\n' {
			atLineStart = true
			continue
		}
		if !atLineStart {
			continue
		}
		if value == ' ' || value == '\t' || value == '\r' {
			continue
		}
		if value == '{' {
			return io.MultiReader(strings.NewReader("{"), buffered), nil
		}
		atLineStart = false
	}
}

func firstOpenClawValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func openClawExplanation(visible, raw string, payloads []struct {
	Text string `json:"text"`
}) string {
	if value := strings.TrimSpace(visible); value != "" {
		return value
	}
	if value := strings.TrimSpace(raw); value != "" {
		return value
	}
	var values []string
	for _, payload := range payloads {
		if value := strings.TrimSpace(payload.Text); value != "" {
			values = append(values, value)
		}
	}
	return strings.Join(values, "\n\n")
}
