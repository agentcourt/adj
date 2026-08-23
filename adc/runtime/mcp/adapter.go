package mcp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/agentcourt/adj/common/mcpbridge"
)

const roleAPIPath = "/roleapi/v1"

type adapter struct {
	client  *mcpbridge.APIClient
	prompts *mcpbridge.PromptCatalog
}

func (a *adapter) OpenSession(assignment mcpbridge.Assignment) (mcpbridge.Profile, error) {
	caseID := assignment.CaseID
	assignmentType := assignment.AssignmentType
	effectivePrincipal := assignment.PrincipalID
	if assignment.Audience != "adc" {
		return mcpbridge.Profile{}, fmt.Errorf("invalid ADC MCP audience %q", assignment.Audience)
	}
	roleID, _, err := profileIdentity(mcpbridge.Profile{AssignmentType: assignmentType, PrincipalID: effectivePrincipal})
	if err != nil {
		return mcpbridge.Profile{}, err
	}
	instructions, err := a.prompts.Render(PromptSessionInstructions, map[string]string{
		"{{CASE_ID}}":         caseID,
		"{{ASSIGNMENT_TYPE}}": assignmentType,
		"{{ROLE_ID}}":         roleID,
		"{{PRINCIPAL_ID}}":    effectivePrincipal,
	})
	if err != nil {
		return mcpbridge.Profile{}, err
	}
	return mcpbridge.Profile{
		CaseID:         caseID,
		AssignmentType: assignmentType,
		PrincipalID:    effectivePrincipal,
		Instructions:   instructions,
	}, nil
}

func NewAssignment(caseID, roleID, principalID string) (mcpbridge.Assignment, error) {
	caseID = strings.TrimSpace(caseID)
	roleID = strings.TrimSpace(roleID)
	principalID = strings.TrimSpace(principalID)
	if caseID == "" {
		return mcpbridge.Assignment{}, fmt.Errorf("case ID is required")
	}
	if !validRole(roleID) {
		return mcpbridge.Assignment{}, fmt.Errorf("role ID must be plaintiff, defendant, juror, or observer")
	}
	if roleID == "juror" && principalID == "" {
		return mcpbridge.Assignment{}, fmt.Errorf("principal ID is required for a juror")
	}
	if roleID != "juror" && principalID != "" {
		return mcpbridge.Assignment{}, fmt.Errorf("principal ID is allowed only for a juror")
	}
	assignmentType := "lawyer"
	effectivePrincipal := roleID
	if roleID == "juror" {
		assignmentType = "juror"
		effectivePrincipal = principalID
	} else if roleID == "observer" {
		assignmentType = "observer"
	}
	return mcpbridge.Assignment{
		Audience:       "adc",
		CaseID:         caseID,
		AssignmentType: assignmentType,
		PrincipalID:    effectivePrincipal,
	}, nil
}

func validRole(roleID string) bool {
	switch roleID {
	case "plaintiff", "defendant", "juror", "observer":
		return true
	default:
		return false
	}
}

type toolDefinition struct {
	name     string
	schema   map[string]any
	readOnly bool
}

func (a *adapter) Tools(profile mcpbridge.Profile) ([]mcpbridge.Tool, error) {
	if _, _, err := profileIdentity(profile); err != nil {
		return nil, err
	}
	definitions := []toolDefinition{
		{name: "get_current_opportunity", schema: mcpbridge.EmptyObjectSchema(), readOnly: true},
		{name: waitToolName, schema: waitSchema(), readOnly: true},
		{name: "case_status", schema: mcpbridge.EmptyObjectSchema(), readOnly: true},
	}
	if profile.AssignmentType == "observer" {
		definitions = append(definitions, toolDefinition{name: "get_case_result", schema: mcpbridge.EmptyObjectSchema(), readOnly: true})
	} else {
		definitions = append(definitions,
			toolDefinition{name: "get_case", schema: mcpbridge.EmptyObjectSchema(), readOnly: true},
			toolDefinition{name: "get_case_result", schema: mcpbridge.EmptyObjectSchema(), readOnly: true},
			toolDefinition{name: "explain_decisions", schema: mcpbridge.EmptyObjectSchema(), readOnly: true},
			toolDefinition{name: "list_case_files", schema: mcpbridge.EmptyObjectSchema(), readOnly: true},
			toolDefinition{name: "read_case_text_file", schema: fileIDSchema(), readOnly: true},
			toolDefinition{name: "request_case_file", schema: fileIDSchema(), readOnly: true},
			toolDefinition{name: "read_case_file_bytes", schema: fileIDSchema(), readOnly: true},
			toolDefinition{name: "get_juror_context", schema: jurorIDSchema(), readOnly: true},
			toolDefinition{name: "send_work_notes", schema: workNotesSchema()},
			toolDefinition{name: "submit_decision", schema: submitDecisionSchema()},
			toolDefinition{name: "report_failure", schema: failureSchema()},
		)
	}
	tools := make([]mcpbridge.Tool, 0, len(definitions))
	for _, definition := range definitions {
		description, err := a.prompts.Text("mcp.tool." + definition.name)
		if err != nil {
			return nil, err
		}
		tools = append(tools, mcpbridge.Tool{
			Name:        definition.name,
			Description: description,
			InputSchema: definition.schema,
			ReadOnly:    definition.readOnly,
		})
	}
	return tools, nil
}

func (a *adapter) CallTool(ctx context.Context, profile mcpbridge.Profile, name string, arguments map[string]any) (mcpbridge.CallResult, error) {
	available, err := a.toolAvailable(profile, name)
	if err != nil {
		return mcpbridge.CallResult{}, err
	}
	if !available {
		return mcpbridge.CallResult{}, fmt.Errorf("tool %q is unavailable for ADC %s %s", name, profile.AssignmentType, profile.PrincipalID)
	}
	body, err := baseBody(profile)
	if err != nil {
		return mcpbridge.CallResult{}, err
	}
	switch name {
	case "get_current_opportunity":
		return a.postRoleAPI(ctx, "/get", body)
	case waitToolName:
		return a.waitForOpportunity(ctx, profile, arguments)
	case "case_status":
		return a.callRoleTool(ctx, profile, "", "case_status", map[string]any{})
	case "get_case_result":
		return a.postRoleAPI(ctx, "/result", body)
	case "report_failure":
		body["message"] = strings.TrimSpace(mcpbridge.String(arguments["message"]))
		return a.postRoleAPI(ctx, "/fail", body)
	default:
		status, err := a.client.Post(ctx, roleAPIPath+"/get", body)
		if err != nil {
			return adcErrorResult(err), nil
		}
		if ok, exists := status["ok"].(bool); exists && !ok {
			return mcpbridge.CallResult{StructuredContent: status, IsError: true}, nil
		}
		return a.callRoleTool(ctx, profile, currentOpportunityID(status), name, arguments)
	}
}

func (a *adapter) toolAvailable(profile mcpbridge.Profile, name string) (bool, error) {
	tools, err := a.Tools(profile)
	if err != nil {
		return false, err
	}
	for _, tool := range tools {
		if tool.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (a *adapter) waitForOpportunity(ctx context.Context, profile mcpbridge.Profile, arguments map[string]any) (mcpbridge.CallResult, error) {
	if err := requireKeys(arguments, "timeout_ms"); err != nil {
		return adcErrorResult(err), nil
	}
	timeout, err := waitTimeout(arguments["timeout_ms"])
	if err != nil {
		return adcErrorResult(err), nil
	}
	body, err := baseBody(profile)
	if err != nil {
		return mcpbridge.CallResult{}, err
	}
	body["timeout_ms"] = int(timeout / time.Millisecond)
	waitCtx, cancel := context.WithTimeout(ctx, timeout+waitToolHTTPMargin)
	defer cancel()
	value, err := a.client.Post(waitCtx, roleAPIPath+"/wait_for_opportunity", body)
	if err != nil {
		return adcErrorResult(err), nil
	}
	state := waitState(value)
	value["state"] = state
	if opportunityID := currentOpportunityID(value); opportunityID != "" {
		value["after_opportunity_id"] = opportunityID
	}
	message, err := a.prompts.Text(waitPromptID(state))
	if err != nil {
		return mcpbridge.CallResult{}, err
	}
	value["message"] = message
	return mcpbridge.CallResult{StructuredContent: value, IsError: state == "error"}, nil
}

func (a *adapter) callRoleTool(ctx context.Context, profile mcpbridge.Profile, opportunityID, name string, arguments map[string]any) (mcpbridge.CallResult, error) {
	body, err := baseBody(profile)
	if err != nil {
		return mcpbridge.CallResult{}, err
	}
	body["tool"] = name
	body["arguments"] = arguments
	if opportunityID != "" {
		body["opportunity_id"] = opportunityID
	}
	return a.postRoleAPI(ctx, "/do", body)
}

func (a *adapter) postRoleAPI(ctx context.Context, endpoint string, body map[string]any) (mcpbridge.CallResult, error) {
	value, err := a.client.Post(ctx, roleAPIPath+endpoint, body)
	if err != nil {
		return adcErrorResult(err), nil
	}
	isError := false
	if ok, exists := value["ok"].(bool); exists {
		isError = !ok
	}
	return mcpbridge.CallResult{StructuredContent: value, IsError: isError}, nil
}

func profileIdentity(profile mcpbridge.Profile) (roleID, principalID string, err error) {
	switch profile.AssignmentType {
	case "lawyer":
		if profile.PrincipalID != "plaintiff" && profile.PrincipalID != "defendant" {
			return "", "", fmt.Errorf("invalid ADC lawyer profile")
		}
		return profile.PrincipalID, "", nil
	case "juror":
		if strings.TrimSpace(profile.PrincipalID) == "" {
			return "", "", fmt.Errorf("invalid ADC juror profile")
		}
		return "juror", profile.PrincipalID, nil
	case "observer":
		if profile.PrincipalID != "observer" {
			return "", "", fmt.Errorf("invalid ADC observer profile")
		}
		return "observer", "", nil
	default:
		return "", "", fmt.Errorf("unknown ADC assignment type %q", profile.AssignmentType)
	}
}

func baseBody(profile mcpbridge.Profile) (map[string]any, error) {
	roleID, principalID, err := profileIdentity(profile)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"case_id": profile.CaseID, "role_id": roleID}
	if principalID != "" {
		body["principal_id"] = principalID
	}
	return body, nil
}

func adcErrorResult(err error) mcpbridge.CallResult {
	return mcpbridge.CallResult{StructuredContent: map[string]any{"ok": false, "error": err.Error()}, IsError: true}
}

func waitTimeout(value any) (time.Duration, error) {
	if value == nil {
		return waitToolDefault, nil
	}
	raw := mcpbridge.NumberString(value)
	if raw == "" {
		return 0, fmt.Errorf("timeout_ms must be an integer")
	}
	milliseconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || milliseconds <= 0 {
		return 0, fmt.Errorf("timeout_ms must be a positive integer")
	}
	timeout := time.Duration(milliseconds) * time.Millisecond
	if timeout > waitToolMax {
		return waitToolMax, nil
	}
	return timeout, nil
}

func waitState(value map[string]any) string {
	if ok, exists := value["ok"].(bool); exists && !ok {
		return "error"
	}
	switch strings.ToLower(mcpbridge.String(value["status"])) {
	case "active", "ready":
		return "ready"
	case "failed":
		return "failed"
	case "done", "terminal", "complete", "completed":
		return "done"
	default:
		return "waiting"
	}
}

func currentOpportunityID(value map[string]any) string {
	if opportunity, ok := value["opportunity"].(map[string]any); ok {
		if id := mcpbridge.String(opportunity["opportunity_id"]); id != "" {
			return id
		}
	}
	if turn, ok := value["current_turn"].(map[string]any); ok {
		return mcpbridge.String(turn["opportunity_id"])
	}
	return ""
}

func requireKeys(arguments map[string]any, allowed ...string) error {
	set := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		set[key] = struct{}{}
	}
	for key := range arguments {
		if _, ok := set[key]; !ok {
			return fmt.Errorf("unexpected argument %q", key)
		}
	}
	return nil
}

func waitSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"timeout_ms": map[string]any{"type": "integer", "minimum": 1, "maximum": waitToolMax.Milliseconds()},
		},
		"additionalProperties": false,
	}
}

func fileIDSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"file_id": map[string]any{"type": "string"}}, "required": []string{"file_id"}, "additionalProperties": false}
}

func jurorIDSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"juror_id": map[string]any{"type": "string"}}, "required": []string{"juror_id"}, "additionalProperties": false}
}

func workNotesSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"notes": map[string]any{"type": "string"}}, "required": []string{"notes"}, "additionalProperties": false}
}

func failureSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"message": map[string]any{"type": "string"}}, "required": []string{"message"}, "additionalProperties": false}
}

func submitDecisionSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"kind":      map[string]any{"type": "string", "enum": []string{"tool", "pass"}},
			"tool_name": map[string]any{"type": "string"},
			"payload":   map[string]any{"type": "object"},
			"reason":    map[string]any{"type": "string"},
		},
		"required":             []string{"kind"},
		"additionalProperties": false,
	}
}
