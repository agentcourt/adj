package eval

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentcourt/adj/adc/runtime/runner"
	"github.com/agentcourt/adj/adc/runtime/spec"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

type judgeEvalDecisionRecord struct {
	ScoringResponse openaiapi.Response
	RawResponse     openaiapi.Response
	Input           []map[string]any
}

func closeEvalFile(file *os.File, path string, returnErr *error) {
	if err := file.Close(); err != nil {
		*returnErr = errors.Join(*returnErr, fmt.Errorf("close %s: %w", path, err))
	}
}

func requireJSONBooleanFields(raw []byte, fields ...string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return err
	}
	for _, field := range fields {
		value, ok := object[field]
		if !ok {
			return fmt.Errorf("missing required boolean field %s", field)
		}
		switch strings.TrimSpace(string(value)) {
		case "true", "false":
		default:
			return fmt.Errorf("field %s must be a boolean", field)
		}
	}
	return nil
}

func validateJudgeEvalRescoreOutputPath(resultsPath string, outputDir string) error {
	sourcePath, err := filepath.Abs(filepath.Clean(resultsPath))
	if err != nil {
		return fmt.Errorf("resolve source results path %s: %w", resultsPath, err)
	}
	outputPath, err := filepath.Abs(filepath.Join(outputDir, "results.jsonl"))
	if err != nil {
		return fmt.Errorf("resolve rescore results path %s: %w", outputDir, err)
	}
	if sourcePath == filepath.Clean(outputPath) {
		return fmt.Errorf("rescore results would overwrite source results %s", resultsPath)
	}
	return nil
}

func readJudgeEvalJSONL[T any](path string) (resultValue []T, returnErr error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open results %s: %w", path, err)
	}
	defer closeEvalFile(f, path, &returnErr)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 32*1024*1024)
	out := make([]T, 0)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var result T
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			return nil, fmt.Errorf("parse results %s line %d: %w", path, lineNo, err)
		}
		out = append(out, result)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan results %s: %w", path, err)
	}
	return out, nil
}

func cloneFloat64Pointer(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func judgeEvalExecutionMode(counterfactualModel bool) string {
	if counterfactualModel {
		return "counterfactual_model"
	}
	return "production"
}

func judgeEvalRoleSpecs(renderer *runner.PromptRenderer, toolName string) ([]spec.RoleSpec, error) {
	if renderer == nil {
		return nil, fmt.Errorf("judge eval prompt renderer is nil")
	}
	role, err := renderer.JudgeRole([]string{toolName})
	if err != nil {
		return nil, err
	}
	return []spec.RoleSpec{role}, nil
}

func judgeEvalDecisionFromExecution(execution judgeOpportunityExecution, toolName string) (judgeEvalDecisionRecord, error) {
	if len(execution.Exchanges) > 0 {
		last := execution.Exchanges[len(execution.Exchanges)-1]
		return judgeEvalDecisionRecord{
			ScoringResponse: last.Response,
			RawResponse:     last.Response,
			Input:           cloneEvalMapList(last.Request.InputItems),
		}, nil
	}
	deterministic, _ := execution.Opportunity["deterministic_action"].(map[string]any)
	actionType := strings.TrimSpace(executionStringField(deterministic, "action_type"))
	payload, _ := deterministic["payload"].(map[string]any)
	if actionType != toolName || payload == nil {
		return judgeEvalDecisionRecord{}, fmt.Errorf("execution recorded no %s decision", toolName)
	}
	response := openaiapi.Response{
		ResponseID: "deterministic-" + strings.TrimSpace(executionStringField(execution.Opportunity, "opportunity_id")),
		ToolCalls: []openaiapi.ToolCall{{
			CallID:    "deterministic",
			Name:      toolName,
			Arguments: cloneEvalMap(payload),
		}},
	}
	return judgeEvalDecisionRecord{ScoringResponse: response, RawResponse: response}, nil
}

func judgeEvalDecisionForScoring(execution judgeOpportunityExecution, toolName string, executionErr error) (judgeEvalDecisionRecord, error) {
	if executionErr == nil {
		return judgeEvalDecisionFromExecution(execution, toolName)
	}
	var proceduralFailure *runner.ProceduralTurnFailure
	if !errors.As(executionErr, &proceduralFailure) {
		return judgeEvalDecisionRecord{}, executionErr
	}
	if len(execution.Exchanges) == 0 {
		return judgeEvalDecisionRecord{}, nil
	}
	last := execution.Exchanges[len(execution.Exchanges)-1]
	return judgeEvalDecisionRecord{
		ScoringResponse: last.Response,
		RawResponse:     last.Response,
		Input:           cloneEvalMapList(last.Request.InputItems),
	}, nil
}

func judgeEvalProceduralInvalidReason(executionErr error) string {
	var proceduralFailure *runner.ProceduralTurnFailure
	if !errors.As(executionErr, &proceduralFailure) {
		return ""
	}
	switch proceduralFailure.Kind {
	case runner.ProceduralFailureInvalidAttemptLimit:
		return "procedural_invalid_attempt_limit"
	case runner.ProceduralFailureDecisionBudget:
		return "procedural_decision_budget_exhausted"
	default:
		return "procedural_failure"
	}
}

func judgeEvalResponseExchangeJSON(exchanges []ResponseExchange) []map[string]any {
	if len(exchanges) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(exchanges))
	for _, exchange := range exchanges {
		request := map[string]any{
			"sequence":             exchange.Request.Sequence,
			"method":               exchange.Request.Method,
			"model":                exchange.Request.Model,
			"input_items":          cloneEvalMapList(exchange.Request.InputItems),
			"tools":                cloneEvalMapList(exchange.Request.Tools),
			"previous_response_id": exchange.Request.PreviousResponseID,
			"temperature":          cloneFloat64Pointer(exchange.Request.Temperature),
		}
		if exchange.Request.RequestSpec != nil {
			request["request_spec"] = *exchange.Request.RequestSpec
		}
		item := map[string]any{
			"request":  request,
			"response": responseJSON(exchange.Response),
		}
		if strings.TrimSpace(exchange.Error) != "" {
			item["error"] = exchange.Error
		}
		result = append(result, item)
	}
	return result
}

func judgeEvalExecutionAcceptance(execution judgeOpportunityExecution, toolName string) (bool, bool) {
	leanAccepted := false
	stepAccepted := false
	_, deterministic := execution.Opportunity["deterministic_action"]
	for _, entry := range execution.TurnLog.Transcript {
		if acceptance, _ := entry["acceptance"].(map[string]any); acceptance != nil {
			if ok, _ := acceptance["ok"].(bool); ok {
				leanAccepted = true
			}
		}
		if strings.TrimSpace(executionStringField(entry, "action")) != toolName {
			continue
		}
		if result, _ := entry["result"].(map[string]any); result != nil {
			if ok, _ := result["ok"].(bool); ok {
				stepAccepted = true
			}
		}
	}
	if deterministic && stepAccepted {
		leanAccepted = true
	}
	return leanAccepted, stepAccepted
}
