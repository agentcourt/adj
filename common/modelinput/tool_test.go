package modelinput

import (
	"strings"
	"testing"

	"github.com/agentcourt/adj/common/modelapi"
)

func TestRejectedToolCallOutputs(t *testing.T) {
	outputs, err := RejectedToolCallOutputs([]modelapi.ToolCall{
		{CallID: "call-1", Name: "submit_council_vote"},
		{CallID: "call-2", Name: "submit_council_vote"},
	}, "call the tool once")
	if err != nil {
		t.Fatal(err)
	}
	if len(outputs) != 2 || outputs[0]["call_id"] != "call-1" || outputs[1]["call_id"] != "call-2" {
		t.Fatalf("outputs = %#v", outputs)
	}
	if output, _ := outputs[0]["output"].(string); !strings.Contains(output, `"ok":false`) || !strings.Contains(output, `"error":"call the tool once"`) {
		t.Fatalf("output = %q", output)
	}
}

func TestRejectedToolCallOutputsRequiresCallID(t *testing.T) {
	_, err := RejectedToolCallOutputs([]modelapi.ToolCall{{Name: "submit_council_vote"}}, "invalid")
	if err == nil || !strings.Contains(err.Error(), "has no call id") {
		t.Fatalf("error = %v", err)
	}
}
