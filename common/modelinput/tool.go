package modelinput

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentcourt/adj/common/modelapi"
)

func RejectedToolCallOutputs(calls []modelapi.ToolCall, reason string) ([]map[string]any, error) {
	outputs := make([]map[string]any, 0, len(calls))
	for _, call := range calls {
		if strings.TrimSpace(call.CallID) == "" {
			return nil, fmt.Errorf("rejected tool call %q has no call id", call.Name)
		}
		wire, err := json.Marshal(map[string]any{
			"ok":    false,
			"error": reason,
			"tool":  call.Name,
		})
		if err != nil {
			return nil, fmt.Errorf("encode rejected tool call %q output: %w", call.Name, err)
		}
		outputs = append(outputs, map[string]any{
			"type":    "function_call_output",
			"call_id": call.CallID,
			"output":  string(wire),
		})
	}
	return outputs, nil
}
