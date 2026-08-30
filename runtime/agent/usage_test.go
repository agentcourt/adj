package agent

import (
	"errors"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentcourt/adj/runtime/runstate"
)

func TestParseCodexUsage(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"thread.started","thread_id":"thread"}`,
		`{"type":"turn.completed","usage":{"input_tokens":100,"cached_input_tokens":40,"cache_write_input_tokens":5,"output_tokens":20,"reasoning_output_tokens":7}}`,
	}, "\n")
	usage, err := parseUsage(RunnerCodex, strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	if usage == nil || usage.InputTokens != 100 || usage.CachedInputTokens != 40 || usage.CacheWriteInputTokens != 5 || usage.OutputTokens != 20 || usage.ReasoningTokens != 7 || usage.TotalTokens != 120 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestParseCodexUsageRejectsMultipleTerminalEvents(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"turn.completed","usage":{"input_tokens":100,"output_tokens":20}}`,
		`{"type":"turn.completed","usage":{"input_tokens":30,"output_tokens":4}}`,
	}, "\n")
	usage, err := parseUsage(RunnerCodex, strings.NewReader(stream))
	if err == nil || !strings.Contains(err.Error(), "more than one turn.completed") || usage == nil || usage.TotalTokens != 120 {
		t.Fatalf("usage = %#v, error = %v", usage, err)
	}
}

func TestFinishCodexUsageRecordsInvocationDelta(t *testing.T) {
	dir := t.TempDir()
	checkpointPath := filepath.Join(dir, "usage.json")
	firstPath := filepath.Join(dir, "first.stdout")
	writeTestFile(t, firstPath, `{"type":"turn.completed","usage":{"input_tokens":100,"cached_input_tokens":40,"output_tokens":20,"reasoning_output_tokens":5}}`)
	first := Invocation{usageCheckpoint: checkpointPath}
	usage, err := first.FinishUsage(RunnerCodex, firstPath)
	if err != nil {
		t.Fatal(err)
	}
	if usage == nil || usage.InputTokens != 100 || usage.CachedInputTokens != 40 || usage.OutputTokens != 20 || usage.TotalTokens != 120 {
		t.Fatalf("first usage = %#v", usage)
	}
	checkpoint, err := readUsageCheckpoint(checkpointPath)
	if err != nil {
		t.Fatal(err)
	}
	secondPath := filepath.Join(dir, "second.stdout")
	writeTestFile(t, secondPath, `{"type":"turn.completed","usage":{"input_tokens":250,"cached_input_tokens":190,"output_tokens":35,"reasoning_output_tokens":8}}`)
	second := Invocation{Resumed: true, usageCheckpoint: checkpointPath, usageBaseline: &checkpoint}
	usage, err = second.FinishUsage(RunnerCodex, secondPath)
	if err != nil {
		t.Fatal(err)
	}
	if usage == nil || usage.InputTokens != 150 || usage.CachedInputTokens != 150 || usage.OutputTokens != 15 || usage.ReasoningTokens != 3 || usage.TotalTokens != 165 {
		t.Fatalf("second usage = %#v", usage)
	}
	checkpoint, err = readUsageCheckpoint(checkpointPath)
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint.Usage.InputTokens != 250 || checkpoint.Usage.TotalTokens != 285 {
		t.Fatalf("checkpoint = %#v", checkpoint)
	}
}

func TestFinishCodexUsageRejectsDecreasedCounter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stdout")
	writeTestFile(t, path, `{"type":"turn.completed","usage":{"input_tokens":9,"output_tokens":2}}`)
	invocation := Invocation{
		Resumed:         true,
		usageCheckpoint: filepath.Join(dir, "usage.json"),
		usageBaseline: &usageCheckpoint{Usage: runstate.TokenUsage{
			InputTokens:  10,
			OutputTokens: 2,
			TotalTokens:  12,
		}},
	}
	usage, err := invocation.FinishUsage(RunnerCodex, path)
	if err == nil || !strings.Contains(err.Error(), "input tokens decreased") || usage == nil || usage.InputTokens != 9 {
		t.Fatalf("usage = %#v, error = %v", usage, err)
	}
}

func TestParseClaudeUsage(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"system","subtype":"init"}`,
		`{"type":"assistant","message":{"usage":{"input_tokens":1}}}`,
		`{"type":"result","subtype":"success","usage":{"input_tokens":11,"cache_creation_input_tokens":13,"cache_read_input_tokens":17,"output_tokens":19}}`,
	}, "\n")
	usage, err := parseUsage(RunnerClaude, strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	if usage == nil || usage.InputTokens != 11 || usage.CachedInputTokens != 17 || usage.CacheWriteInputTokens != 13 || usage.OutputTokens != 19 || usage.TotalTokens != 60 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestParseClaudeUsageReturnsProviderRefusalWithRetainedUsage(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"system","subtype":"model_refusal_no_fallback","original_model":"claude-opus-4-8","request_id":"req_011CeJo55jSnPxXHgaRoxh88","api_refusal_category":"cyber","api_refusal_explanation":"This request triggered cyber-related safeguards. To learn about the Cyber Verification Program and apply for access, visit our help center: https://support.claude.com/en/articles/14604842-real-time-cyber-safeguards-on-claude. API integrators: you can reduce refusals for your users by configuring a fallback model — see https://platform.claude.com/docs/en/build-with-claude/refusals-and-fallback","refused_user_message_uuid":"7c2b9cf8-92d0-4a75-8ea1-188519f56228","content":"","session_id":"175e6341-5588-40f4-b982-d5430e659c0f","uuid":"4eedd89c-ef15-4823-ac0b-00020814c56f"}`,
		`{"is_error":true,"stop_reason":"refusal","session_id":"175e6341-5588-40f4-b982-d5430e659c0f","usage":{"input_tokens":10,"cache_creation_input_tokens":39354,"cache_read_input_tokens":121731,"output_tokens":2734},"terminal_reason":"api_error","subtype":"success","type":"result"}`,
	}, "\n")
	usage, err := parseUsage(RunnerClaude, strings.NewReader(stream))
	var refusal *ProviderRefusalError
	if !errors.As(err, &refusal) {
		t.Fatalf("error = %v, want ProviderRefusalError", err)
	}
	if usage == nil || usage.InputTokens != 10 || usage.CachedInputTokens != 121731 || usage.CacheWriteInputTokens != 39354 || usage.OutputTokens != 2734 || usage.TotalTokens != 163829 {
		t.Fatalf("usage = %#v", usage)
	}
	if refusal.Provider != "" || refusal.Model != "claude-opus-4-8" || refusal.StopReason != "refusal" || refusal.RawStopReason != "" || refusal.WillRetry || refusal.Category != "cyber" || refusal.SessionID != "175e6341-5588-40f4-b982-d5430e659c0f" || refusal.ResponseID != "" || refusal.RequestID != "req_011CeJo55jSnPxXHgaRoxh88" || refusal.RefusedMessageID != "7c2b9cf8-92d0-4a75-8ea1-188519f56228" {
		t.Fatalf("refusal = %#v", refusal)
	}
	if !strings.HasPrefix(refusal.Explanation, "This request triggered cyber-related safeguards.") {
		t.Fatalf("explanation = %q", refusal.Explanation)
	}
	for _, value := range []string{"claude-opus-4-8", "cyber", "175e6341-5588-40f4-b982-d5430e659c0f", "req_011CeJo55jSnPxXHgaRoxh88", "7c2b9cf8-92d0-4a75-8ea1-188519f56228"} {
		if !strings.Contains(refusal.Error(), value) {
			t.Fatalf("error %q does not contain %q", refusal.Error(), value)
		}
	}
}

func TestParseClaudeUsageDoesNotInferRefusalFromResultText(t *testing.T) {
	stream := `{"type":"result","subtype":"success","stop_reason":"end_turn","result":"API refusal prose","usage":{"input_tokens":3,"output_tokens":2}}`
	usage, err := parseUsage(RunnerClaude, strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	if usage == nil || usage.TotalTokens != 5 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestParseClaudeUsageRequiresRefusalSystemEvent(t *testing.T) {
	stream := `{"type":"result","subtype":"success","stop_reason":"refusal","usage":{"input_tokens":3,"output_tokens":2}}`
	usage, err := parseUsage(RunnerClaude, strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	if usage == nil || usage.TotalTokens != 5 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestParseClaudeUsageRejectsContradictoryRefusalResult(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"system","subtype":"model_refusal_no_fallback","original_model":"claude-opus-4-8"}`,
		`{"type":"result","stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":2}}`,
	}, "\n")
	usage, err := parseUsage(RunnerClaude, strings.NewReader(stream))
	var refusal *ProviderRefusalError
	if err == nil || errors.As(err, &refusal) || !strings.Contains(err.Error(), `terminal stop reason "end_turn"`) {
		t.Fatalf("usage = %#v, error = %v", usage, err)
	}
	if usage == nil || usage.TotalTokens != 5 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestParsePiUsage(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"session","version":3}`,
		`{"type":"message_end","message":{"role":"assistant","usage":{"input":10,"output":2,"cacheRead":3,"cacheWrite":4,"totalTokens":19}}}`,
		`{"type":"message_end","message":{"role":"toolResult"}}`,
		`{"type":"message_end","message":{"role":"assistant","usage":{"input":5,"output":1,"cacheRead":2,"cacheWrite":0,"totalTokens":8}}}`,
		`{"type":"agent_end","messages":[{"role":"assistant","usage":{"input":15,"output":3,"cacheRead":5,"cacheWrite":4,"totalTokens":27}}]}`,
	}, "\n")
	usage, err := parseUsage(RunnerPi, strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	if usage == nil || usage.InputTokens != 15 || usage.CachedInputTokens != 5 || usage.CacheWriteInputTokens != 4 || usage.OutputTokens != 3 || usage.TotalTokens != 27 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestParsePiUsageReturnsSettledProviderRefusalWithUsage(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"session","id":"session-1","version":3}`,
		`{"type":"message_end","message":{"role":"assistant","provider":"anthropic","model":"claude-opus-4-8","stopReason":"toolUse","rawStopReason":"tool_use","responseId":"response-1","usage":{"input":10,"output":2,"cacheRead":3,"cacheWrite":4,"totalTokens":19}}}`,
		`{"type":"message_end","message":{"role":"assistant","provider":"anthropic","model":"claude-opus-4-8","stopReason":"error","rawStopReason":"refusal","errorMessage":"provider explanation","responseId":"response-2","usage":{"input":5,"output":0,"cacheRead":2,"cacheWrite":1,"totalTokens":8}}}`,
		`{"type":"agent_end","messages":[{"role":"assistant","provider":"anthropic","model":"claude-opus-4-8","stopReason":"toolUse","rawStopReason":"tool_use","responseId":"response-1"},{"role":"toolResult"},{"role":"assistant","provider":"anthropic","model":"claude-opus-4-8","stopReason":"error","rawStopReason":"refusal","errorMessage":"provider explanation","responseId":"response-2"}],"willRetry":false}`,
		`{"type":"agent_settled"}`,
	}, "\n")
	usage, err := parseUsage(RunnerPi, strings.NewReader(stream))
	var refusal *ProviderRefusalError
	if !errors.As(err, &refusal) {
		t.Fatalf("error = %v, want ProviderRefusalError", err)
	}
	if usage == nil || usage.InputTokens != 15 || usage.CachedInputTokens != 5 || usage.CacheWriteInputTokens != 5 || usage.OutputTokens != 2 || usage.TotalTokens != 27 {
		t.Fatalf("usage = %#v", usage)
	}
	if refusal.Provider != "anthropic" || refusal.Model != "claude-opus-4-8" || refusal.StopReason != "error" || refusal.RawStopReason != "refusal" || refusal.WillRetry || refusal.Explanation != "provider explanation" || refusal.SessionID != "session-1" || refusal.ResponseID != "response-2" {
		t.Fatalf("refusal = %#v", refusal)
	}
	if refusal.ErrorClass() != "provider_refusal" {
		t.Fatalf("error class = %q", refusal.ErrorClass())
	}
	for _, value := range []string{"anthropic", "claude-opus-4-8", "provider explanation", "session-1", "response-2"} {
		if !strings.Contains(refusal.Error(), value) {
			t.Fatalf("error %q does not contain %q", refusal.Error(), value)
		}
	}
}

func TestParsePiUsageUsesFinalSettledTerminalEvent(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"message_end","message":{"role":"assistant","usage":{"input":1,"output":1,"cacheRead":0,"cacheWrite":0,"totalTokens":2}}}`,
		`{"type":"agent_end","messages":[{"role":"assistant","provider":"anthropic","model":"claude-opus-4-8","stopReason":"error","rawStopReason":"refusal","errorMessage":"first attempt"}],"willRetry":true}`,
		`{"type":"agent_end","messages":[{"role":"assistant","provider":"anthropic","model":"claude-opus-4-8","stopReason":"stop","rawStopReason":"end_turn"}],"willRetry":false}`,
		`{"type":"agent_settled"}`,
	}, "\n")
	usage, err := parseUsage(RunnerPi, strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	if usage == nil || usage.TotalTokens != 2 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestParsePiUsageRequiresSettledRefusal(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"message_end","message":{"role":"assistant","usage":{"input":1,"output":1,"cacheRead":0,"cacheWrite":0,"totalTokens":2}}}`,
		`{"type":"agent_end","messages":[{"role":"assistant","provider":"anthropic","model":"claude-opus-4-8","stopReason":"error","rawStopReason":"refusal","errorMessage":"unfinished"}],"willRetry":false}`,
	}, "\n")
	usage, err := parseUsage(RunnerPi, strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	if usage == nil || usage.TotalTokens != 2 {
		t.Fatalf("usage = %#v", usage)
	}
}

func TestReadPiUsageFilePreservesTypedRefusal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pi.stdout")
	writeTestFile(t, path, strings.Join([]string{
		`{"type":"session","id":"session-file"}`,
		`{"type":"message_end","message":{"role":"assistant","usage":{"input":1,"output":1,"cacheRead":0,"cacheWrite":0,"totalTokens":2}}}`,
		`{"type":"agent_end","messages":[{"role":"assistant","provider":"anthropic","model":"claude-opus-4-8","stopReason":"error","rawStopReason":"refusal","errorMessage":"file refusal","responseId":"response-file"}],"willRetry":false}`,
		`{"type":"agent_settled"}`,
	}, "\n"))
	usage, err := ReadUsageFile(RunnerPi, path)
	var refusal *ProviderRefusalError
	if !errors.As(err, &refusal) {
		t.Fatalf("error = %v, want ProviderRefusalError", err)
	}
	if usage == nil || usage.TotalTokens != 2 {
		t.Fatalf("usage = %#v", usage)
	}
	if refusal.SessionID != "session-file" || refusal.ResponseID != "response-file" {
		t.Fatalf("refusal = %#v", refusal)
	}
}

func TestParseUsagePreservesPartialUsageAndErrors(t *testing.T) {
	stream := `{"type":"turn.completed","usage":{"input_tokens":2,"output_tokens":3}}` + "\n{"
	usage, err := parseUsage(RunnerCodex, strings.NewReader(stream))
	if err == nil || usage == nil || usage.TotalTokens != 5 {
		t.Fatalf("usage = %#v, error = %v", usage, err)
	}

	_, err = parseUsage(RunnerPi, strings.NewReader(`{"type":"message_end","message":{"role":"assistant","usage":{"input":1,"output":2,"cacheRead":3,"cacheWrite":4,"totalTokens":9}}}`))
	if err == nil || !strings.Contains(err.Error(), "component sum is 10") {
		t.Fatalf("Pi total error = %v", err)
	}

	_, err = parseUsage(RunnerClaude, strings.NewReader(`{"type":"result","usage":{"input_tokens":-1}}`))
	if err == nil || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("negative usage error = %v", err)
	}
}

func TestParseUsageRejectsOverflowAndUnsupportedRunner(t *testing.T) {
	stream := `{"type":"turn.completed","usage":{"input_tokens":` + "9223372036854775807" + `,"output_tokens":1}}`
	if _, err := parseUsage(RunnerCodex, strings.NewReader(stream)); err == nil || !strings.Contains(err.Error(), "overflows") {
		t.Fatalf("overflow error = %v", err)
	}
	if _, err := addNonnegative(math.MaxInt64, 1); err == nil {
		t.Fatal("direct overflow was accepted")
	}
	if _, err := parseUsage(Runner("unknown"), strings.NewReader("")); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unsupported-runner error = %v", err)
	}
}

func TestParseUsageRequiresTerminalUsageEvent(t *testing.T) {
	for _, runner := range []Runner{RunnerCodex, RunnerClaude, RunnerPi} {
		if _, err := parseUsage(runner, strings.NewReader(`{"type":"system"}`)); err == nil || !strings.Contains(err.Error(), "usage stream has no") {
			t.Fatalf("%s missing-event error = %v", runner, err)
		}
	}
}
