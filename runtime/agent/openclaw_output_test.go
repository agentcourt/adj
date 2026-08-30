package agent

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseOpenClawOutputAcceptsSetupLinesAndSuccessfulResult(t *testing.T) {
	stream := `Applied 14 config update(s). Restart the gateway to apply.
Saved MCP server "quick-case-plaintiff" to /home/node/.openclaw/openclaw.json.
{
  "payloads": [
    {
      "text": "The argument was accepted.",
      "mediaUrl": null
    }
  ],
  "meta": {
    "durationMs": 218159,
    "agentMeta": {
      "sessionId": "session-success",
      "sessionFile": "/home/node/.openclaw/agents/quick/sessions/session-success.jsonl",
      "provider": "anthropic",
      "model": "claude-opus-4-8",
      "usage": {
        "input": 18,
        "output": 14112,
        "cacheRead": 399324,
        "cacheWrite": 66378,
        "total": 66692
      }
    },
    "finalAssistantVisibleText": "The argument was accepted.",
    "finalAssistantRawText": "The argument was accepted.",
    "stopReason": "stop",
    "executionTrace": {
      "winnerProvider": "anthropic",
      "winnerModel": "claude-opus-4-8",
      "attempts": [
        {
          "provider": "anthropic",
          "model": "claude-opus-4-8",
          "result": "success",
          "stage": "assistant"
        }
      ],
      "fallbackUsed": false,
      "runner": "embedded"
    },
    "completion": {
      "stopReason": "stop",
      "finishReason": "stop"
    }
  }
}`
	usage, err := parseOpenClawOutput(strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	if usage != nil {
		t.Fatalf("usage = %#v, want nil", usage)
	}
}

func TestParseOpenClawOutputReturnsStructuredRefusal(t *testing.T) {
	stream := `Applied 14 config update(s). Restart the gateway to apply.
Saved MCP server "quick-case-defendant" to /home/node/.openclaw/openclaw.json.
{
  "payloads": [
    {
      "text": "Payload refusal explanation.",
      "mediaUrl": null
    }
  ],
  "meta": {
    "durationMs": 31142,
    "agentMeta": {
      "sessionId": "session-refusal",
      "sessionFile": "/home/node/.openclaw/agents/quick/sessions/session-refusal.jsonl",
      "provider": "anthropic",
      "model": "claude-opus-4-8"
    },
    "finalAssistantVisibleText": "Provider refusal explanation.",
    "finalAssistantRawText": "Raw provider refusal explanation.",
    "stopReason": "refusal",
    "executionTrace": {
      "winnerProvider": "anthropic",
      "winnerModel": "claude-opus-4-8",
      "attempts": [
        {
          "provider": "anthropic",
          "model": "claude-opus-4-8",
          "result": "success",
          "stage": "assistant"
        }
      ],
      "fallbackUsed": false,
      "runner": "embedded"
    },
    "completion": {
      "stopReason": "refusal",
      "finishReason": "refusal",
      "refusal": true
    }
  }
}`
	usage, err := parseOpenClawOutput(strings.NewReader(stream))
	var refusal *ProviderRefusalError
	if !errors.As(err, &refusal) {
		t.Fatalf("error = %v, want ProviderRefusalError", err)
	}
	if usage != nil {
		t.Fatalf("usage = %#v, want nil", usage)
	}
	if refusal.Provider != "anthropic" || refusal.Model != "claude-opus-4-8" || refusal.SessionID != "session-refusal" {
		t.Fatalf("refusal identity = %#v", refusal)
	}
	if refusal.StopReason != "refusal" || refusal.RawStopReason != "refusal" || refusal.WillRetry {
		t.Fatalf("refusal terminal fields = %#v", refusal)
	}
	if refusal.Explanation != "Provider refusal explanation." {
		t.Fatalf("explanation = %q", refusal.Explanation)
	}
}

func TestParseOpenClawOutputUsesPayloadExplanation(t *testing.T) {
	stream := `setup line one
setup line two
{
  "payloads": [
    {"text": "First explanation."},
    {"text": "Second explanation."}
  ],
  "meta": {
    "agentMeta": {
      "sessionId": "session-payload",
      "provider": "openai",
      "model": "gpt-5.6"
    },
    "stopReason": "provider_refusal",
    "completion": {
      "finishReason": "refusal",
      "refusal": true
    }
  }
}`
	_, err := parseOpenClawOutput(strings.NewReader(stream))
	var refusal *ProviderRefusalError
	if !errors.As(err, &refusal) {
		t.Fatalf("error = %v, want ProviderRefusalError", err)
	}
	if refusal.StopReason != "provider_refusal" || refusal.RawStopReason != "refusal" {
		t.Fatalf("refusal terminal fields = %#v", refusal)
	}
	if refusal.Explanation != "First explanation.\n\nSecond explanation." {
		t.Fatalf("explanation = %q", refusal.Explanation)
	}
}

func TestParseOpenClawOutputDoesNotInferRefusal(t *testing.T) {
	stream := `setup line one
setup line two
{
  "payloads": [{"text": "The provider reported a refusal."}],
  "meta": {
    "finalAssistantVisibleText": "The provider reported a refusal.",
    "stopReason": "refusal",
    "executionTrace": {
      "attempts": [{"result": "error", "reason": "refusal"}]
    },
    "completion": {
      "stopReason": "refusal",
      "finishReason": "refusal",
      "refusal": false
    }
  }
}`
	usage, err := parseOpenClawOutput(strings.NewReader(stream))
	if err != nil {
		t.Fatal(err)
	}
	if usage != nil {
		t.Fatalf("usage = %#v, want nil", usage)
	}
}

func TestInspectOpenClawOutputFilePreservesTypedRefusal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "openclaw.stdout")
	writeTestFile(t, path, `first setup line
second setup line
{"meta":{"agentMeta":{"sessionId":"session-file","provider":"anthropic","model":"claude-opus-4-8"},"finalAssistantRawText":"file explanation","completion":{"stopReason":"refusal","finishReason":"refusal","refusal":true}}}`)
	usage, err := InspectOpenClawOutputFile(path)
	var refusal *ProviderRefusalError
	if !errors.As(err, &refusal) {
		t.Fatalf("error = %v, want ProviderRefusalError", err)
	}
	if usage != nil || refusal.SessionID != "session-file" || refusal.Explanation != "file explanation" {
		t.Fatalf("usage = %#v, refusal = %#v", usage, refusal)
	}
}

func TestParseOpenClawOutputRequiresTerminalJSONObject(t *testing.T) {
	if _, err := parseOpenClawOutput(strings.NewReader("first setup line\nsecond setup line\n")); err == nil || !strings.Contains(err.Error(), "no terminal JSON object") {
		t.Fatalf("error = %v", err)
	}
}
