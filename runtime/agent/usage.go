package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/agentcourt/adj/runtime/runstate"
)

const usageCheckpointSchemaVersion = "adj.agent.codex-usage.v1"

type usageCheckpoint struct {
	SchemaVersion string              `json:"schema_version"`
	Usage         runstate.TokenUsage `json:"usage"`
}

// FinishUsage reads one invocation's structured output and records the Codex
// cumulative counter needed to measure a later resumed invocation.
func (i *Invocation) FinishUsage(runner Runner, path string) (*runstate.TokenUsage, error) {
	usage, err := ReadUsageFile(runner, path)
	if err != nil || runner != RunnerCodex {
		return usage, err
	}
	if i == nil || i.usageCheckpoint == "" {
		return usage, errors.New("Codex invocation has no usage checkpoint path")
	}
	invocationUsage := usage
	if i.Resumed {
		if i.usageBaseline == nil {
			return usage, errors.New("resumed Codex invocation has no usage baseline")
		}
		var deltaErr error
		invocationUsage, deltaErr = subtractUsage(usage, &i.usageBaseline.Usage)
		if deltaErr != nil {
			return usage, fmt.Errorf("compute resumed Codex usage: %w", deltaErr)
		}
	}
	checkpoint := usageCheckpoint{SchemaVersion: usageCheckpointSchemaVersion, Usage: *usage}
	data, err := json.Marshal(checkpoint)
	if err != nil {
		return invocationUsage, fmt.Errorf("encode Codex usage checkpoint: %w", err)
	}
	if err := writePrivateFile(i.usageCheckpoint, append(data, '\n')); err != nil {
		return invocationUsage, fmt.Errorf("write Codex usage checkpoint: %w", err)
	}
	return invocationUsage, nil
}

func ReadUsageFile(runner Runner, path string) (usage *runstate.TokenUsage, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s usage stream %q: %w", runner, path, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close %s usage stream %q: %w", runner, path, err))
		}
	}()
	usage, err = parseUsage(runner, file)
	if err != nil {
		return usage, fmt.Errorf("parse %s usage stream %q: %w", runner, path, err)
	}
	return usage, nil
}

func parseUsage(runner Runner, reader io.Reader) (*runstate.TokenUsage, error) {
	switch runner {
	case RunnerCodex:
		return parseCodexUsage(reader)
	case RunnerClaude:
		return parseClaudeUsage(reader)
	case RunnerPi:
		return parsePiUsage(reader)
	default:
		return nil, fmt.Errorf("unsupported usage runner %q", runner)
	}
}

func parseCodexUsage(reader io.Reader) (*runstate.TokenUsage, error) {
	decoder := json.NewDecoder(reader)
	var total *runstate.TokenUsage
	for {
		var event struct {
			Type  string `json:"type"`
			Usage *struct {
				InputTokens           int64 `json:"input_tokens"`
				CachedInputTokens     int64 `json:"cached_input_tokens"`
				CacheWriteInputTokens int64 `json:"cache_write_input_tokens"`
				OutputTokens          int64 `json:"output_tokens"`
				ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
			} `json:"usage"`
		}
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				if total == nil {
					return nil, errors.New("usage stream has no turn.completed event")
				}
				return total, nil
			}
			return total, err
		}
		if event.Type != "turn.completed" {
			continue
		}
		if total != nil {
			return total, errors.New("usage stream contains more than one turn.completed event")
		}
		if event.Usage == nil {
			return total, errors.New("turn.completed event has no usage")
		}
		usage := runstate.TokenUsage{
			InputTokens:           event.Usage.InputTokens,
			CachedInputTokens:     event.Usage.CachedInputTokens,
			CacheWriteInputTokens: event.Usage.CacheWriteInputTokens,
			OutputTokens:          event.Usage.OutputTokens,
			ReasoningTokens:       event.Usage.ReasoningOutputTokens,
		}
		if err := validateUsageComponents(usage); err != nil {
			return total, err
		}
		value, err := addNonnegative(usage.InputTokens, usage.OutputTokens)
		if err != nil {
			return total, fmt.Errorf("compute Codex total tokens: %w", err)
		}
		usage.TotalTokens = value
		total = &usage
	}
}

func readUsageCheckpoint(path string) (usageCheckpoint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return usageCheckpoint{}, fmt.Errorf("read usage checkpoint %q: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var checkpoint usageCheckpoint
	if err := decoder.Decode(&checkpoint); err != nil {
		return usageCheckpoint{}, fmt.Errorf("decode usage checkpoint %q: %w", path, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("trailing JSON")
		}
		return usageCheckpoint{}, fmt.Errorf("decode usage checkpoint %q: %w", path, err)
	}
	if checkpoint.SchemaVersion != usageCheckpointSchemaVersion {
		return usageCheckpoint{}, fmt.Errorf("usage checkpoint %q has schema version %q", path, checkpoint.SchemaVersion)
	}
	if err := validateUsage(checkpoint.Usage); err != nil {
		return usageCheckpoint{}, fmt.Errorf("validate usage checkpoint %q: %w", path, err)
	}
	return checkpoint, nil
}

func subtractUsage(current, previous *runstate.TokenUsage) (*runstate.TokenUsage, error) {
	if current == nil || previous == nil {
		return nil, errors.New("usage counter is absent")
	}
	result := runstate.TokenUsage{}
	fields := []struct {
		name     string
		current  int64
		previous int64
		result   *int64
	}{
		{"input tokens", current.InputTokens, previous.InputTokens, &result.InputTokens},
		{"cached input tokens", current.CachedInputTokens, previous.CachedInputTokens, &result.CachedInputTokens},
		{"cache-write input tokens", current.CacheWriteInputTokens, previous.CacheWriteInputTokens, &result.CacheWriteInputTokens},
		{"output tokens", current.OutputTokens, previous.OutputTokens, &result.OutputTokens},
		{"reasoning tokens", current.ReasoningTokens, previous.ReasoningTokens, &result.ReasoningTokens},
		{"total tokens", current.TotalTokens, previous.TotalTokens, &result.TotalTokens},
	}
	for _, field := range fields {
		if field.current < field.previous {
			return nil, fmt.Errorf("%s decreased from %d to %d", field.name, field.previous, field.current)
		}
		*field.result = field.current - field.previous
	}
	if err := validateUsage(result); err != nil {
		return nil, err
	}
	return &result, nil
}

func parseClaudeUsage(reader io.Reader) (*runstate.TokenUsage, error) {
	decoder := json.NewDecoder(reader)
	var total *runstate.TokenUsage
	var refusal *ProviderRefusalError
	var resultStopReason string
	for {
		var event struct {
			Type                   string `json:"type"`
			Subtype                string `json:"subtype"`
			OriginalModel          string `json:"original_model"`
			RequestID              string `json:"request_id"`
			APIRefusalCategory     string `json:"api_refusal_category"`
			APIRefusalExplanation  string `json:"api_refusal_explanation"`
			RefusedUserMessageUUID string `json:"refused_user_message_uuid"`
			SessionID              string `json:"session_id"`
			StopReason             string `json:"stop_reason"`
			Usage                  *struct {
				InputTokens              int64 `json:"input_tokens"`
				CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
				CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
				OutputTokens             int64 `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				if refusal != nil {
					if resultStopReason != "" && !strings.EqualFold(resultStopReason, "refusal") {
						return total, fmt.Errorf("Claude model_refusal_no_fallback event has terminal stop reason %q", resultStopReason)
					}
					refusal.StopReason = resultStopReason
					return total, refusal
				}
				if total == nil {
					return nil, errors.New("usage stream has no result event")
				}
				return total, nil
			}
			return total, err
		}
		if event.Type == "system" && event.Subtype == "model_refusal_no_fallback" {
			if refusal != nil {
				return total, errors.New("usage stream contains more than one model_refusal_no_fallback event")
			}
			refusal = &ProviderRefusalError{
				Model:            strings.TrimSpace(event.OriginalModel),
				WillRetry:        false,
				Category:         strings.TrimSpace(event.APIRefusalCategory),
				Explanation:      strings.TrimSpace(event.APIRefusalExplanation),
				SessionID:        strings.TrimSpace(event.SessionID),
				RequestID:        strings.TrimSpace(event.RequestID),
				RefusedMessageID: strings.TrimSpace(event.RefusedUserMessageUUID),
			}
			continue
		}
		if event.Type != "result" {
			continue
		}
		if total != nil {
			return total, errors.New("usage stream contains more than one result event")
		}
		if event.Usage == nil {
			return nil, errors.New("result event has no usage")
		}
		resultStopReason = strings.TrimSpace(event.StopReason)
		usage := runstate.TokenUsage{
			InputTokens:           event.Usage.InputTokens,
			CachedInputTokens:     event.Usage.CacheReadInputTokens,
			CacheWriteInputTokens: event.Usage.CacheCreationInputTokens,
			OutputTokens:          event.Usage.OutputTokens,
		}
		if err := validateUsageComponents(usage); err != nil {
			return nil, err
		}
		value, err := sumNonnegative(usage.InputTokens, usage.CachedInputTokens, usage.CacheWriteInputTokens, usage.OutputTokens)
		if err != nil {
			return nil, fmt.Errorf("compute Claude total tokens: %w", err)
		}
		usage.TotalTokens = value
		total = &usage
	}
}

func parsePiUsage(reader io.Reader) (*runstate.TokenUsage, error) {
	decoder := json.NewDecoder(reader)
	var total *runstate.TokenUsage
	var pendingTerminal *piTerminal
	var settledTerminal *piTerminal
	var sessionID string
	for {
		var event struct {
			Type    string `json:"type"`
			ID      string `json:"id"`
			Message *struct {
				Role  string `json:"role"`
				Usage *struct {
					Input       int64 `json:"input"`
					Output      int64 `json:"output"`
					CacheRead   int64 `json:"cacheRead"`
					CacheWrite  int64 `json:"cacheWrite"`
					TotalTokens int64 `json:"totalTokens"`
				} `json:"usage"`
			} `json:"message"`
			Messages  []piTerminalMessage `json:"messages"`
			WillRetry bool                `json:"willRetry"`
		}
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				if refusal := settledTerminal.refusal(); refusal != nil {
					return total, refusal
				}
				if total == nil {
					return nil, errors.New("usage stream has no assistant message_end event")
				}
				return total, nil
			}
			return total, err
		}
		switch event.Type {
		case "session":
			sessionID = strings.TrimSpace(event.ID)
		case "agent_end":
			pendingTerminal = &piTerminal{SessionID: sessionID, Messages: event.Messages, WillRetry: event.WillRetry}
		case "agent_settled":
			if pendingTerminal != nil {
				settledTerminal = pendingTerminal
				pendingTerminal = nil
			}
		}
		if event.Type != "message_end" || event.Message == nil || event.Message.Role != "assistant" {
			continue
		}
		if event.Message.Usage == nil {
			return total, errors.New("assistant message_end event has no usage")
		}
		usage := runstate.TokenUsage{
			InputTokens:           event.Message.Usage.Input,
			CachedInputTokens:     event.Message.Usage.CacheRead,
			CacheWriteInputTokens: event.Message.Usage.CacheWrite,
			OutputTokens:          event.Message.Usage.Output,
			TotalTokens:           event.Message.Usage.TotalTokens,
		}
		if err := validateUsageComponents(usage); err != nil {
			return total, err
		}
		computed, err := sumNonnegative(usage.InputTokens, usage.CachedInputTokens, usage.CacheWriteInputTokens, usage.OutputTokens)
		if err != nil {
			return total, fmt.Errorf("compute Pi total tokens: %w", err)
		}
		if usage.TotalTokens != computed {
			return total, fmt.Errorf("Pi totalTokens is %d; component sum is %d", usage.TotalTokens, computed)
		}
		if err := accumulateUsage(&total, usage); err != nil {
			return total, err
		}
	}
}

type piTerminal struct {
	SessionID string
	Messages  []piTerminalMessage
	WillRetry bool
}

type piTerminalMessage struct {
	Role          string `json:"role"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	StopReason    string `json:"stopReason"`
	RawStopReason string `json:"rawStopReason"`
	ErrorMessage  string `json:"errorMessage"`
	ResponseID    string `json:"responseId"`
}

func (t *piTerminal) refusal() error {
	if t == nil {
		return nil
	}
	for index := len(t.Messages) - 1; index >= 0; index-- {
		message := t.Messages[index]
		if message.Role != "assistant" {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(message.StopReason), "refusal") && !strings.EqualFold(strings.TrimSpace(message.RawStopReason), "refusal") {
			return nil
		}
		return &ProviderRefusalError{
			Provider:      strings.TrimSpace(message.Provider),
			Model:         strings.TrimSpace(message.Model),
			StopReason:    strings.TrimSpace(message.StopReason),
			RawStopReason: strings.TrimSpace(message.RawStopReason),
			WillRetry:     t.WillRetry,
			Explanation:   strings.TrimSpace(message.ErrorMessage),
			SessionID:     strings.TrimSpace(t.SessionID),
			ResponseID:    strings.TrimSpace(message.ResponseID),
		}
	}
	return nil
}

// ProviderRefusalError records a structured provider refusal.
type ProviderRefusalError struct {
	Provider         string
	Model            string
	StopReason       string
	RawStopReason    string
	WillRetry        bool
	Category         string
	Explanation      string
	SessionID        string
	ResponseID       string
	RequestID        string
	RefusedMessageID string
}

func (e *ProviderRefusalError) Error() string {
	message := fmt.Sprintf("provider %q model %q refused the request: stop reason %q; raw stop reason %q; will retry %t; category %q; session ID %q; response ID %q; request ID %q; refused message ID %q", e.Provider, e.Model, e.StopReason, e.RawStopReason, e.WillRetry, e.Category, e.SessionID, e.ResponseID, e.RequestID, e.RefusedMessageID)
	if e.Explanation != "" {
		message += ": " + e.Explanation
	}
	return message
}

func (e *ProviderRefusalError) ErrorClass() string { return "provider_refusal" }

func validateUsageComponents(usage runstate.TokenUsage) error {
	if usage.InputTokens < 0 || usage.CachedInputTokens < 0 || usage.CacheWriteInputTokens < 0 || usage.OutputTokens < 0 || usage.ReasoningTokens < 0 || usage.TotalTokens < 0 {
		return errors.New("usage contains a negative token count")
	}
	return nil
}

func validateUsage(usage runstate.TokenUsage) error {
	if err := validateUsageComponents(usage); err != nil {
		return err
	}
	total, err := addNonnegative(usage.InputTokens, usage.OutputTokens)
	if err != nil {
		return fmt.Errorf("compute total tokens: %w", err)
	}
	if usage.TotalTokens != total {
		return fmt.Errorf("total tokens is %d; input and output sum is %d", usage.TotalTokens, total)
	}
	return nil
}

func accumulateUsage(total **runstate.TokenUsage, usage runstate.TokenUsage) error {
	if *total == nil {
		copy := usage
		*total = &copy
		return nil
	}
	fields := []struct {
		name  string
		left  *int64
		right int64
	}{
		{"input tokens", &(*total).InputTokens, usage.InputTokens},
		{"cached input tokens", &(*total).CachedInputTokens, usage.CachedInputTokens},
		{"cache-write input tokens", &(*total).CacheWriteInputTokens, usage.CacheWriteInputTokens},
		{"output tokens", &(*total).OutputTokens, usage.OutputTokens},
		{"reasoning tokens", &(*total).ReasoningTokens, usage.ReasoningTokens},
		{"total tokens", &(*total).TotalTokens, usage.TotalTokens},
	}
	for _, field := range fields {
		value, err := addNonnegative(*field.left, field.right)
		if err != nil {
			return fmt.Errorf("sum %s: %w", field.name, err)
		}
		*field.left = value
	}
	return nil
}

func sumNonnegative(values ...int64) (int64, error) {
	var total int64
	for _, value := range values {
		var err error
		total, err = addNonnegative(total, value)
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}

func addNonnegative(left, right int64) (int64, error) {
	if left < 0 || right < 0 {
		return 0, errors.New("token count is negative")
	}
	if right > math.MaxInt64-left {
		return 0, errors.New("token count overflows int64")
	}
	return left + right, nil
}
