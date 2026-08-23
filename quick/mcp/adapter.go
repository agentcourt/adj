package mcp

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/agentcourt/adj/common/mcpbridge"
)

const lawyerAPIPath = "/lawyerapi/v1"

type adapter struct {
	client  *mcpbridge.APIClient
	prompts *mcpbridge.PromptCatalog
}

func (a *adapter) OpenSession(assignment mcpbridge.Assignment) (mcpbridge.Profile, error) {
	caseID := assignment.CaseID
	roleID := assignment.PrincipalID
	if assignment.Audience != "quick" || !validRole(roleID) {
		return mcpbridge.Profile{}, fmt.Errorf("invalid Quick MCP assignment")
	}
	assignmentType := quickAssignmentType(roleID)
	if assignment.AssignmentType != assignmentType {
		return mcpbridge.Profile{}, fmt.Errorf("invalid Quick MCP assignment type %q for %s", assignment.AssignmentType, roleID)
	}
	instructions, err := a.prompts.Render(PromptSessionInstructions, map[string]string{
		"{{CASE_ID}}":         caseID,
		"{{ASSIGNMENT_TYPE}}": assignmentType,
		"{{PRINCIPAL_ID}}":    roleID,
	})
	if err != nil {
		return mcpbridge.Profile{}, err
	}
	return mcpbridge.Profile{
		CaseID:         caseID,
		AssignmentType: assignmentType,
		PrincipalID:    roleID,
		Instructions:   instructions,
	}, nil
}

func NewAssignment(caseID, roleID string) (mcpbridge.Assignment, error) {
	caseID = strings.TrimSpace(caseID)
	roleID = strings.TrimSpace(roleID)
	if caseID == "" {
		return mcpbridge.Assignment{}, fmt.Errorf("case ID is required")
	}
	if !validRole(roleID) {
		return mcpbridge.Assignment{}, fmt.Errorf("role ID must be plaintiff, defendant, or observer")
	}
	return mcpbridge.Assignment{
		Audience:       "quick",
		CaseID:         caseID,
		AssignmentType: quickAssignmentType(roleID),
		PrincipalID:    roleID,
	}, nil
}

func quickAssignmentType(roleID string) string {
	if roleID == "observer" {
		return "observer"
	}
	return "lawyer"
}

func validRole(roleID string) bool {
	switch roleID {
	case "plaintiff", "defendant", "observer":
		return true
	default:
		return false
	}
}

func (a *adapter) Tools(profile mcpbridge.Profile) ([]mcpbridge.Tool, error) {
	roleID := profileRole(profile)
	if roleID == "" {
		return nil, fmt.Errorf("quick MCP session has no valid role")
	}
	definitions := []toolDefinition{
		{name: "get_current_opportunity", schema: mcpbridge.EmptyObjectSchema(), readOnly: true},
		{name: waitToolName, schema: waitForOpportunitySchema(), readOnly: true},
		{name: "case_status", schema: mcpbridge.EmptyObjectSchema(), readOnly: true},
		{name: "get_case", schema: mcpbridge.EmptyObjectSchema(), readOnly: true},
		{name: "get_case_result", schema: mcpbridge.EmptyObjectSchema(), readOnly: true},
		{name: "list_evidence", schema: mcpbridge.EmptyObjectSchema(), readOnly: true},
		{name: "stat_evidence", schema: evidenceIDSchema(), readOnly: true},
		{name: "read_evidence_range", schema: evidenceRangeSchema(), readOnly: true},
	}
	if roleID != "observer" {
		definitions = append(definitions,
			toolDefinition{name: "send_work_notes", schema: workNotesSchema()},
			toolDefinition{name: "submit_decision", schema: submitArgumentSchema()},
		)
	}
	return a.tools(definitions)
}

func (a *adapter) CallTool(ctx context.Context, profile mcpbridge.Profile, name string, arguments map[string]any) (mcpbridge.CallResult, error) {
	roleID := profileRole(profile)
	if roleID == "" {
		return mcpbridge.CallResult{}, fmt.Errorf("quick MCP session has no valid role")
	}
	if !toolAvailable(name, roleID) {
		return mcpbridge.CallResult{}, fmt.Errorf("tool %q is unavailable for quick role %s", name, roleID)
	}
	switch name {
	case "get_current_opportunity":
		return a.get(ctx, profile, "/get")
	case waitToolName:
		return a.waitForOpportunity(ctx, profile, arguments)
	case "case_status":
		return a.get(ctx, profile, "/status")
	case "get_case_result":
		return a.get(ctx, profile, "/result")
	default:
		return a.postTool(ctx, profile, name, arguments)
	}
}

func profileRole(profile mcpbridge.Profile) string {
	roleID := profile.PrincipalID
	if validRole(roleID) {
		return roleID
	}
	return ""
}

func toolAvailable(name, roleID string) bool {
	switch name {
	case "get_current_opportunity", waitToolName, "case_status", "get_case", "get_case_result", "list_evidence", "stat_evidence", "read_evidence_range":
		return true
	case "send_work_notes", "submit_decision":
		return roleID != "observer"
	default:
		return false
	}
}

func (a *adapter) get(ctx context.Context, profile mcpbridge.Profile, endpoint string) (mcpbridge.CallResult, error) {
	value, err := a.client.Get(ctx, lawyerAPIPath+endpoint, profileQuery(profile))
	return result(value, err), nil
}

func (a *adapter) waitForOpportunity(ctx context.Context, profile mcpbridge.Profile, arguments map[string]any) (mcpbridge.CallResult, error) {
	if err := requireKeys(arguments, "after_opportunity_id", "after_version", "timeout_ms"); err != nil {
		return errorResult(err), nil
	}
	timeout, err := waitTimeout(arguments["timeout_ms"])
	if err != nil {
		return errorResult(err), nil
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout+waitToolHTTPMargin)
	defer cancel()
	query := profileQuery(profile)
	query.Set("timeout_ms", strconv.FormatInt(timeout.Milliseconds(), 10))
	if after := mcpbridge.String(arguments["after_opportunity_id"]); after != "" {
		query.Set("after", after)
	}
	if afterVersion := mcpbridge.NumberString(arguments["after_version"]); afterVersion != "" {
		query.Set("after_version", afterVersion)
	}
	value, err := a.client.Get(waitCtx, lawyerAPIPath+"/wait", query)
	if err != nil {
		return errorResult(err), nil
	}
	state := waitState(value)
	value["state"] = state
	if version := waitVersion(value); version != nil {
		value["after_version"] = version
	}
	if opportunityID := currentOpportunityID(value); opportunityID != "" {
		value["after_opportunity_id"] = opportunityID
	}
	message, promptErr := a.prompts.Text(waitPromptID(state))
	if promptErr != nil {
		return mcpbridge.CallResult{}, promptErr
	}
	value["message"] = message
	return mcpbridge.CallResult{StructuredContent: value, IsError: state == "error"}, nil
}

func (a *adapter) postTool(ctx context.Context, profile mcpbridge.Profile, name string, arguments map[string]any) (mcpbridge.CallResult, error) {
	status, err := a.client.Get(ctx, lawyerAPIPath+"/get", profileQuery(profile))
	if err != nil {
		return errorResult(err), nil
	}
	if ok, exists := status["ok"].(bool); exists && !ok {
		return mcpbridge.CallResult{StructuredContent: status, IsError: true}, nil
	}
	body := map[string]any{
		"case_id":   profile.CaseID,
		"role_id":   profile.PrincipalID,
		"tool":      name,
		"arguments": arguments,
	}
	if profile.PrincipalID != "observer" {
		opportunityID := currentOpportunityID(status)
		if opportunityID == "" {
			return errorResult(fmt.Errorf("current lawyer turn has no opportunity_id")), nil
		}
		body["opportunity_id"] = opportunityID
	}
	value, err := a.client.Post(ctx, lawyerAPIPath+"/do", body)
	return result(value, err), nil
}

func profileQuery(profile mcpbridge.Profile) url.Values {
	query := url.Values{}
	query.Set("case_id", profile.CaseID)
	query.Set("role_id", profile.PrincipalID)
	return query
}

func result(value map[string]any, err error) mcpbridge.CallResult {
	if err != nil {
		return errorResult(err)
	}
	isError := false
	if ok, exists := value["ok"].(bool); exists {
		isError = !ok
	}
	return mcpbridge.CallResult{StructuredContent: value, IsError: isError}
}

func errorResult(err error) mcpbridge.CallResult {
	return mcpbridge.CallResult{
		StructuredContent: map[string]any{"ok": false, "error": err.Error()},
		IsError:           true,
	}
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
	case "ready":
		return "ready"
	case "failed":
		return "failed"
	case "done", "terminal", "complete", "completed":
		return "done"
	default:
		return "waiting"
	}
}

func waitVersion(value map[string]any) any {
	wait, _ := value["wait"].(map[string]any)
	return wait["version"]
}

func currentOpportunityID(value map[string]any) string {
	turn, _ := value["turn"].(map[string]any)
	return mcpbridge.String(turn["opportunity_id"])
}

func requireKeys(arguments map[string]any, allowed ...string) error {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = struct{}{}
	}
	for key := range arguments {
		if _, ok := allowedSet[key]; !ok {
			return fmt.Errorf("unexpected argument %q", key)
		}
	}
	return nil
}

func waitForOpportunitySchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"after_opportunity_id": map[string]any{"type": "string"},
			"after_version":        map[string]any{"type": "integer", "minimum": 0},
			"timeout_ms":           map[string]any{"type": "integer", "minimum": 1, "maximum": waitToolMax.Milliseconds()},
		},
		"additionalProperties": false,
	}
}

type toolDefinition struct {
	name     string
	schema   map[string]any
	readOnly bool
}

func (a *adapter) tools(definitions []toolDefinition) ([]mcpbridge.Tool, error) {
	tools := make([]mcpbridge.Tool, 0, len(definitions))
	for _, definition := range definitions {
		description, err := a.prompts.Text(toolPromptID(definition.name))
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

func evidenceIDSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"evidence_id": map[string]any{"type": "string"},
		},
		"required":             []string{"evidence_id"},
		"additionalProperties": false,
	}
}

func evidenceRangeSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"evidence_id": map[string]any{"type": "string"},
			"offset":      map[string]any{"type": "integer", "minimum": 0},
			"length":      map[string]any{"type": "integer", "minimum": 1, "maximum": 256 << 10},
		},
		"required":             []string{"evidence_id", "offset", "length"},
		"additionalProperties": false,
	}
}

func workNotesSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"notes": map[string]any{"type": "string"},
		},
		"required":             []string{"notes"},
		"additionalProperties": false,
	}
}

func submitArgumentSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"kind":      map[string]any{"type": "string", "enum": []string{"tool"}},
			"tool_name": map[string]any{"type": "string", "enum": []string{"submit_argument"}},
			"payload": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"text": map[string]any{"type": "string"},
				},
				"required":             []string{"text"},
				"additionalProperties": false,
			},
		},
		"required":             []string{"kind", "tool_name", "payload"},
		"additionalProperties": false,
	}
}
