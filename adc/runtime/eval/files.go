package eval

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentcourt/adj/adc/runtime/courts"
	"github.com/agentcourt/adj/adc/runtime/lean"
	"github.com/agentcourt/adj/adc/runtime/runner"
	"github.com/agentcourt/adj/adc/runtime/spec"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

type JudgeEvalProvenance struct {
	ExecutionMode         string            `json:"execution_mode"`
	SummaryMode           string            `json:"summary_mode"`
	Court                 *courts.Profile   `json:"court,omitempty"`
	PromptDir             string            `json:"prompt_dir,omitempty"`
	PromptFiles           map[string]string `json:"prompt_files,omitempty"`
	CandidatePromptSource string            `json:"candidate_prompt_source"`
	CandidatePromptName   string            `json:"candidate_prompt_name"`
	CandidatePromptPath   string            `json:"candidate_prompt_path,omitempty"`
	Temperature           *float64          `json:"temperature,omitempty"`
	Online                *bool             `json:"online,omitempty"`
	DryRun                bool              `json:"dry_run"`
	Engine                []string          `json:"engine,omitempty"`
	SourceResultsPath     string            `json:"source_results_path,omitempty"`
	SourceSummaryPath     string            `json:"source_summary_path,omitempty"`
	SourceGeneratedAt     string            `json:"source_generated_at,omitempty"`
}

type judgeEvalSourceSummary struct {
	RunID               string              `json:"run_id"`
	Evaluation          string              `json:"evaluation"`
	Model               string              `json:"model"`
	DryRun              bool                `json:"dry_run"`
	Provenance          JudgeEvalProvenance `json:"provenance"`
	PromptSource        string              `json:"prompt_source"`
	PromptName          string              `json:"prompt_name"`
	PromptPath          string              `json:"prompt_path,omitempty"`
	ExecutionMode       string              `json:"execution_mode,omitempty"`
	CounterfactualModel bool                `json:"counterfactual_model,omitempty"`
	Total               int                 `json:"total"`
	GeneratedAt         string              `json:"generated_at"`
}

type judgeEvalResultIdentity struct {
	RunID               string
	Evaluation          string
	ID                  string
	Model               string
	DryRun              bool
	PromptSource        string
	PromptName          string
	PromptPath          string
	ExecutionMode       string
	CounterfactualModel bool
}

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

func newJudgeEvalProvenance(
	court courts.Profile,
	promptDir string,
	promptFiles map[string]string,
	candidateSource string,
	candidateName string,
	candidatePath string,
	temperature *float64,
	online bool,
	dryRun bool,
	engine lean.Engine,
	executionMode string,
) JudgeEvalProvenance {
	courtCopy := court
	onlineCopy := online
	return JudgeEvalProvenance{
		ExecutionMode:         executionMode,
		SummaryMode:           "run",
		Court:                 &courtCopy,
		PromptDir:             promptDir,
		PromptFiles:           cloneStringMap(promptFiles),
		CandidatePromptSource: candidateSource,
		CandidatePromptName:   candidateName,
		CandidatePromptPath:   candidatePath,
		Temperature:           cloneFloat64Pointer(temperature),
		Online:                &onlineCopy,
		DryRun:                dryRun,
		Engine:                append([]string(nil), engine.Command...),
	}
}

func newJudgeEvalRescoreProvenance(source judgeEvalSourceSummary, resultsPath string, summaryPath string) JudgeEvalProvenance {
	provenance := source.Provenance
	provenance.SummaryMode = "rescore"
	provenance.SourceResultsPath = resultsPath
	provenance.SourceSummaryPath = summaryPath
	provenance.SourceGeneratedAt = source.GeneratedAt
	return provenance
}

func newJudgeEvalRunID() (string, error) {
	return generateJudgeEvalRunID(rand.Reader)
}

func generateJudgeEvalRunID(source io.Reader) (string, error) {
	var value [16]byte
	if _, err := io.ReadFull(source, value[:]); err != nil {
		return "", fmt.Errorf("generate judge eval run id: %w", err)
	}
	return "judge-eval-" + hex.EncodeToString(value[:]), nil
}

func defaultJudgeEvalSourceSummaryPath(resultsPath string, explicitPath string) string {
	if path := strings.TrimSpace(explicitPath); path != "" {
		return path
	}
	return filepath.Join(filepath.Dir(resultsPath), "summary.json")
}

func loadJudgeEvalSourceSummary(resultsPath string, explicitSummaryPath string, expectedEvaluation string) (judgeEvalSourceSummary, string, error) {
	summaryPath := defaultJudgeEvalSourceSummaryPath(resultsPath, explicitSummaryPath)
	raw, err := os.ReadFile(summaryPath)
	if err != nil {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("read source summary %s: %w", summaryPath, err)
	}
	fields, err := judgeEvalJSONObjectFields(raw)
	if err != nil {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("parse source summary %s: %w", summaryPath, err)
	}
	if err := requireJudgeEvalFields(fields, "run_id", "evaluation", "model", "dry_run", "execution_mode", "counterfactual_model", "provenance", "prompt_source", "prompt_name", "total", "generated_at"); err != nil {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s: %w", summaryPath, err)
	}
	var provenanceFields map[string]json.RawMessage
	if err := json.Unmarshal(fields["provenance"], &provenanceFields); err != nil {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("parse source summary %s provenance: %w", summaryPath, err)
	}
	if err := requireJudgeEvalFields(provenanceFields, "execution_mode", "summary_mode", "candidate_prompt_source", "candidate_prompt_name", "dry_run"); err != nil {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s provenance: %w", summaryPath, err)
	}
	var source judgeEvalSourceSummary
	if err := json.Unmarshal(raw, &source); err != nil {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("parse source summary %s: %w", summaryPath, err)
	}
	if strings.TrimSpace(source.RunID) == "" {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s is missing run_id", summaryPath)
	}
	if source.Evaluation != expectedEvaluation {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s evaluation is %q, want %q", summaryPath, source.Evaluation, expectedEvaluation)
	}
	if strings.TrimSpace(source.Model) == "" {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s is missing model", summaryPath)
	}
	if source.Total < 1 {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s has invalid total %d", summaryPath, source.Total)
	}
	if strings.TrimSpace(source.GeneratedAt) == "" {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s is missing generated_at", summaryPath)
	}
	if source.Provenance.SummaryMode != "run" {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s provenance summary_mode is %q, want %q", summaryPath, source.Provenance.SummaryMode, "run")
	}
	if source.Provenance.SourceResultsPath != "" || source.Provenance.SourceSummaryPath != "" || source.Provenance.SourceGeneratedAt != "" {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s run provenance contains rescore source fields", summaryPath)
	}
	if strings.TrimSpace(source.Provenance.ExecutionMode) == "" {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s provenance is missing execution_mode", summaryPath)
	}
	if source.Provenance.Court == nil {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s provenance is missing court", summaryPath)
	}
	if source.Provenance.Online == nil {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s provenance is missing online", summaryPath)
	}
	if len(source.Provenance.Engine) == 0 {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s provenance is missing engine", summaryPath)
	}
	if strings.TrimSpace(source.Provenance.CandidatePromptSource) == "" || strings.TrimSpace(source.Provenance.CandidatePromptName) == "" {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s provenance is missing candidate prompt identity", summaryPath)
	}
	if source.Provenance.DryRun != source.DryRun {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s dry_run disagrees with provenance", summaryPath)
	}
	if source.Provenance.CandidatePromptSource != source.PromptSource || source.Provenance.CandidatePromptName != source.PromptName || source.Provenance.CandidatePromptPath != source.PromptPath {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s prompt fields disagree with provenance", summaryPath)
	}
	if source.ExecutionMode != source.Provenance.ExecutionMode {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s execution_mode disagrees with provenance", summaryPath)
	}
	if source.CounterfactualModel != (source.ExecutionMode == "counterfactual_model") {
		return judgeEvalSourceSummary{}, summaryPath, fmt.Errorf("source summary %s counterfactual_model disagrees with execution_mode", summaryPath)
	}
	return source, summaryPath, nil
}

func validateJudgeEvalRescoreRows[T any](
	resultsPath string,
	source judgeEvalSourceSummary,
	results []T,
	identity func(T) judgeEvalResultIdentity,
	validateFixture func(T) error,
) error {
	if len(results) == 0 {
		return fmt.Errorf("no results loaded from %s", resultsPath)
	}
	if len(results) != source.Total {
		return fmt.Errorf("source results %s contain %d rows, source summary records %d", resultsPath, len(results), source.Total)
	}
	seen := make(map[string]int, len(results))
	for index, result := range results {
		lineNo := index + 1
		row := identity(result)
		if strings.TrimSpace(row.ID) == "" {
			return fmt.Errorf("source results %s line %d is missing fixture id", resultsPath, lineNo)
		}
		if firstLine, exists := seen[row.ID]; exists {
			return fmt.Errorf("source results %s line %d duplicates fixture id %q from line %d", resultsPath, lineNo, row.ID, firstLine)
		}
		seen[row.ID] = lineNo
		if row.RunID != source.RunID {
			return fmt.Errorf("source results %s line %d run_id is %q, want %q", resultsPath, lineNo, row.RunID, source.RunID)
		}
		if row.Evaluation != source.Evaluation {
			return fmt.Errorf("source results %s line %d evaluation is %q, want %q", resultsPath, lineNo, row.Evaluation, source.Evaluation)
		}
		if row.Model != source.Model {
			return fmt.Errorf("source results %s line %d model is %q, want %q", resultsPath, lineNo, row.Model, source.Model)
		}
		if row.DryRun != source.DryRun {
			return fmt.Errorf("source results %s line %d dry_run disagrees with source summary", resultsPath, lineNo)
		}
		if row.PromptSource != source.PromptSource || row.PromptName != source.PromptName || row.PromptPath != source.PromptPath {
			return fmt.Errorf("source results %s line %d prompt identity disagrees with source summary", resultsPath, lineNo)
		}
		rowExecutionMode := row.ExecutionMode
		if rowExecutionMode == "" {
			rowExecutionMode = "production"
		}
		if rowExecutionMode != source.ExecutionMode || row.CounterfactualModel != source.CounterfactualModel {
			return fmt.Errorf("source results %s line %d execution mode disagrees with source summary", resultsPath, lineNo)
		}
		if err := validateFixture(result); err != nil {
			return fmt.Errorf("source results %s line %d: %w", resultsPath, lineNo, err)
		}
	}
	return nil
}

func validateJudgeEvalRescoreOutputPaths(resultsPath string, sourceSummaryPath string, outputDir string) error {
	sources := []struct {
		name string
		path string
	}{
		{name: "source results", path: resultsPath},
		{name: "source summary", path: sourceSummaryPath},
	}
	targets := []struct {
		name string
		path string
	}{
		{name: "rescore results", path: filepath.Join(outputDir, "results.jsonl")},
		{name: "rescore summary", path: filepath.Join(outputDir, "summary.json")},
	}
	for _, target := range targets {
		targetPath, err := resolveJudgeEvalPath(target.path)
		if err != nil {
			return fmt.Errorf("resolve %s path %s: %w", target.name, target.path, err)
		}
		for _, source := range sources {
			sourcePath, err := resolveJudgeEvalPath(source.path)
			if err != nil {
				return fmt.Errorf("resolve %s path %s: %w", source.name, source.path, err)
			}
			if targetPath == sourcePath {
				return fmt.Errorf("%s path %s resolves to %s path %s", target.name, target.path, source.name, source.path)
			}
		}
	}
	return nil
}

func resolveJudgeEvalPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absPath = filepath.Clean(absPath)
	ancestor := absPath
	suffix := make([]string, 0)
	for {
		if _, err := os.Lstat(ancestor); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", fmt.Errorf("no existing ancestor")
		}
		suffix = append(suffix, filepath.Base(ancestor))
		ancestor = parent
	}
	resolved, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	for index := len(suffix) - 1; index >= 0; index-- {
		resolved = filepath.Join(resolved, suffix[index])
	}
	return filepath.Clean(resolved), nil
}

func readJudgeEvalJSONL[T any](path string, requiredFields ...string) (resultValue []T, returnErr error) {
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
		raw := []byte(line)
		fields, err := judgeEvalJSONObjectFields(raw)
		if err != nil {
			return nil, fmt.Errorf("parse results %s line %d: %w", path, lineNo, err)
		}
		if err := requireJudgeEvalFields(fields, requiredFields...); err != nil {
			return nil, fmt.Errorf("parse results %s line %d: %w", path, lineNo, err)
		}
		var result T
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&result); err != nil {
			return nil, fmt.Errorf("parse results %s line %d: %w", path, lineNo, err)
		}
		if err := requireJudgeEvalJSONEOF(decoder); err != nil {
			return nil, fmt.Errorf("parse results %s line %d: %w", path, lineNo, err)
		}
		out = append(out, result)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan results %s: %w", path, err)
	}
	return out, nil
}

func judgeEvalJSONObjectFields(raw []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := rejectJudgeEvalDuplicateKeys(decoder); err != nil {
		return nil, err
	}
	if err := requireJudgeEvalJSONEOF(decoder); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("JSON value must be an object")
	}
	return fields, nil
}

func rejectJudgeEvalDuplicateKeys(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key has type %T", keyToken)
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate object key %q", key)
			}
			seen[key] = struct{}{}
			if err := rejectJudgeEvalDuplicateKeys(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return fmt.Errorf("object ended with %v", end)
		}
	case '[':
		for decoder.More() {
			if err := rejectJudgeEvalDuplicateKeys(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return fmt.Errorf("array ended with %v", end)
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	return nil
}

func requireJudgeEvalJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("unexpected trailing JSON value")
}

func requireJudgeEvalFields(fields map[string]json.RawMessage, names ...string) error {
	for _, name := range names {
		if _, ok := fields[name]; !ok {
			return fmt.Errorf("missing required field %s", name)
		}
	}
	return nil
}

func judgeEvalRequiredResultFields(suiteFields ...string) []string {
	common := []string{
		"run_id",
		"evaluation",
		"id",
		"model",
		"dry_run",
		"execution_mode",
		"counterfactual_model",
		"prompt_source",
		"prompt_name",
		"state",
		"view",
		"opportunity",
		"input",
		"raw_response",
		"turn_log",
		"provider",
		"outcome_correct",
		"reason_correct",
		"lean_accepted",
		"step_accepted",
	}
	return append(common, suiteFields...)
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
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

func judgeEvalResponseClient(dryRun bool, client *openaiapi.Client, response openaiapi.Response) runner.ResponseClient {
	if dryRun {
		script := make([]ScriptedResponse, runner.DefaultInvalidAttemptLimit)
		for index := range script {
			script[index] = ScriptedResponse{Response: response}
		}
		return NewScriptedResponseClient(script)
	}
	return client
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
