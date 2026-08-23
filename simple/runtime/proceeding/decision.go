package proceeding

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	openaiapi "github.com/agentcourt/adj/common/openai"
)

type decisionArguments struct {
	Decision  string `json:"decision"`
	Rationale string `json:"rationale"`
}

func parseDecision(response openaiapi.Response) (Decision, error) {
	if len(response.ToolCalls) != 1 {
		return Decision{}, protocolError(fmt.Errorf("provider returned %d tool calls; expected exactly one submit_simple_decision call", len(response.ToolCalls)))
	}
	call := response.ToolCalls[0]
	if call.Name != "submit_simple_decision" {
		return Decision{}, protocolError(fmt.Errorf("provider called %q; expected submit_simple_decision", call.Name))
	}
	if strings.TrimSpace(call.ArgumentsError) != "" {
		return Decision{}, protocolError(fmt.Errorf("submit_simple_decision arguments are invalid JSON: %s", call.ArgumentsError))
	}
	raw := strings.TrimSpace(call.RawArguments)
	if raw == "" {
		return Decision{}, protocolError(fmt.Errorf("submit_simple_decision arguments are empty"))
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var arguments decisionArguments
	if err := decoder.Decode(&arguments); err != nil {
		return Decision{}, protocolError(fmt.Errorf("decode submit_simple_decision arguments: %w", err))
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Decision{}, protocolError(fmt.Errorf("submit_simple_decision arguments contain trailing JSON"))
		}
		return Decision{}, protocolError(fmt.Errorf("decode trailing submit_simple_decision arguments: %w", err))
	}
	arguments.Decision = strings.TrimSpace(arguments.Decision)
	arguments.Rationale = strings.TrimSpace(arguments.Rationale)
	switch arguments.Decision {
	case "demonstrated", "not_demonstrated":
	default:
		return Decision{}, protocolError(fmt.Errorf("submit_simple_decision decision %q is invalid", arguments.Decision))
	}
	if arguments.Rationale == "" {
		return Decision{}, protocolError(fmt.Errorf("submit_simple_decision rationale is empty"))
	}
	return Decision{Value: arguments.Decision, Rationale: arguments.Rationale}, nil
}

func protocolError(err error) error {
	return &openaiapi.ProviderError{Class: openaiapi.ProviderErrorProtocol, Err: err}
}
