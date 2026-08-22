package mcpbridge

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	arbitrationLawyerAPIPath  = "/lawyerapi/v1"
	arbitrationCouncilAPIPath = "/councilapi/v1"
	arbitrationWaitTool       = "wait_for_opportunity"
	arbitrationWaitDefault    = 30 * time.Second
	arbitrationWaitMax        = 30 * time.Second
	arbitrationWaitMargin     = 2 * time.Second
)

type ArbitrationAdapterOptions struct {
	Client                  *APIClient
	Prompts                 *PromptCatalog
	CouncilSubmissionTool   string
	CouncilSubmissionSchema map[string]any
}

func NewArbitrationAdapter(opts ArbitrationAdapterOptions) (Adapter, error) {
	if opts.Client == nil {
		return nil, fmt.Errorf("arbitration MCP case API client is required")
	}
	if opts.Prompts == nil {
		return nil, fmt.Errorf("arbitration MCP prompt catalog is required")
	}
	councilTool := strings.TrimSpace(opts.CouncilSubmissionTool)
	if councilTool == "" || opts.CouncilSubmissionSchema == nil {
		return nil, fmt.Errorf("arbitration MCP council submission tool and schema are required")
	}
	return &arbitrationAdapter{
		client:                  opts.Client,
		prompts:                 opts.Prompts,
		councilSubmissionTool:   councilTool,
		councilSubmissionSchema: opts.CouncilSubmissionSchema,
	}, nil
}

func ArbitrationHTTPTimeout() time.Duration {
	return arbitrationWaitMax + arbitrationWaitMargin
}

type arbitrationAdapter struct {
	client                  *APIClient
	prompts                 *PromptCatalog
	councilSubmissionTool   string
	councilSubmissionSchema map[string]any
}

func (a *arbitrationAdapter) OpenSession(assignment Assignment) (Profile, error) {
	caseID := assignment.CaseID
	assignmentType := assignment.AssignmentType
	principalID := assignment.PrincipalID
	switch assignmentType {
	case "lawyer":
		if !validArbitrationRole(principalID) || principalID == "observer" {
			return Profile{}, fmt.Errorf("invalid arbitration lawyer assignment")
		}
	case "observer":
		if principalID != "observer" {
			return Profile{}, fmt.Errorf("invalid arbitration observer assignment")
		}
	case "council":
		if strings.TrimSpace(principalID) == "" {
			return Profile{}, fmt.Errorf("invalid arbitration council assignment")
		}
	default:
		return Profile{}, fmt.Errorf("invalid arbitration assignment type %q", assignmentType)
	}
	instructions, err := a.prompts.Render("mcp.session.instructions", map[string]string{
		"{{CASE_ID}}":         caseID,
		"{{ASSIGNMENT_TYPE}}": assignmentType,
		"{{PRINCIPAL_ID}}":    principalID,
	})
	if err != nil {
		return Profile{}, err
	}
	return Profile{
		CaseID:         caseID,
		AssignmentType: assignmentType,
		PrincipalID:    principalID,
		Instructions:   instructions,
	}, nil
}

func NewArbitrationAssignment(audience, caseID, roleID, memberID string) (Assignment, error) {
	audience = strings.TrimSpace(audience)
	caseID = strings.TrimSpace(caseID)
	roleID = strings.TrimSpace(roleID)
	memberID = strings.TrimSpace(memberID)
	if audience != "aar" && audience != "aard" {
		return Assignment{}, fmt.Errorf("arbitration audience must be aar or aard")
	}
	if caseID == "" {
		return Assignment{}, fmt.Errorf("case ID is required")
	}
	if (roleID == "") == (memberID == "") {
		return Assignment{}, fmt.Errorf("exactly one of role ID or member ID is required")
	}
	if memberID != "" {
		return Assignment{Audience: audience, CaseID: caseID, AssignmentType: "council", PrincipalID: memberID}, nil
	}
	if !validArbitrationRole(roleID) {
		return Assignment{}, fmt.Errorf("role ID must be plaintiff, defendant, or observer")
	}
	assignmentType := "lawyer"
	if roleID == "observer" {
		assignmentType = "observer"
	}
	return Assignment{Audience: audience, CaseID: caseID, AssignmentType: assignmentType, PrincipalID: roleID}, nil
}

func validArbitrationRole(roleID string) bool {
	switch roleID {
	case "plaintiff", "defendant", "observer":
		return true
	default:
		return false
	}
}

type arbitrationToolDefinition struct {
	name     string
	schema   map[string]any
	readOnly bool
}

func (a *arbitrationAdapter) Tools(profile Profile) ([]Tool, error) {
	definitions, err := a.toolDefinitions(profile)
	if err != nil {
		return nil, err
	}
	tools := make([]Tool, 0, len(definitions))
	for _, definition := range definitions {
		description, err := a.prompts.Text("mcp.tool." + definition.name)
		if err != nil {
			return nil, err
		}
		tools = append(tools, Tool{
			Name:        definition.name,
			Description: description,
			InputSchema: definition.schema,
			ReadOnly:    definition.readOnly,
		})
	}
	return tools, nil
}

func (a *arbitrationAdapter) toolDefinitions(profile Profile) ([]arbitrationToolDefinition, error) {
	base := []arbitrationToolDefinition{
		{name: "get_current_opportunity", schema: EmptyObjectSchema(), readOnly: true},
		{name: arbitrationWaitTool, schema: arbitrationWaitSchema(), readOnly: true},
	}
	switch profile.AssignmentType {
	case "council":
		return append(base,
			arbitrationToolDefinition{name: "get_case", schema: EmptyObjectSchema(), readOnly: true},
			arbitrationToolDefinition{name: "list_evidence", schema: EmptyObjectSchema(), readOnly: true},
			arbitrationToolDefinition{name: "stat_evidence", schema: arbitrationEvidenceIDSchema(), readOnly: true},
			arbitrationToolDefinition{name: "read_evidence_range", schema: arbitrationEvidenceRangeSchema(), readOnly: true},
			arbitrationToolDefinition{name: a.councilSubmissionTool, schema: a.councilSubmissionSchema},
		), nil
	case "observer":
		return append(base,
			arbitrationToolDefinition{name: "case_status", schema: EmptyObjectSchema(), readOnly: true},
			arbitrationToolDefinition{name: "get_case", schema: EmptyObjectSchema(), readOnly: true},
			arbitrationToolDefinition{name: "get_case_result", schema: EmptyObjectSchema(), readOnly: true},
			arbitrationToolDefinition{name: "get_turn", schema: EmptyObjectSchema(), readOnly: true},
			arbitrationToolDefinition{name: "list_events", schema: arbitrationListEventsSchema(), readOnly: true},
			arbitrationToolDefinition{name: "list_evidence", schema: EmptyObjectSchema(), readOnly: true},
			arbitrationToolDefinition{name: "stat_evidence", schema: arbitrationEvidenceIDSchema(), readOnly: true},
			arbitrationToolDefinition{name: "read_evidence_range", schema: arbitrationEvidenceRangeSchema(), readOnly: true},
		), nil
	case "lawyer":
		if !validArbitrationRole(profile.PrincipalID) || profile.PrincipalID == "observer" {
			return nil, fmt.Errorf("invalid arbitration lawyer profile")
		}
		return append(base,
			arbitrationToolDefinition{name: "case_status", schema: EmptyObjectSchema(), readOnly: true},
			arbitrationToolDefinition{name: "get_case", schema: EmptyObjectSchema(), readOnly: true},
			arbitrationToolDefinition{name: "get_case_result", schema: EmptyObjectSchema(), readOnly: true},
			arbitrationToolDefinition{name: "send_work_notes", schema: arbitrationWorkNotesSchema()},
			arbitrationToolDefinition{name: "list_evidence", schema: EmptyObjectSchema(), readOnly: true},
			arbitrationToolDefinition{name: "stat_evidence", schema: arbitrationEvidenceIDSchema(), readOnly: true},
			arbitrationToolDefinition{name: "read_evidence_range", schema: arbitrationEvidenceRangeSchema(), readOnly: true},
			arbitrationToolDefinition{name: "begin_evidence_upload", schema: arbitrationBeginUploadSchema()},
			arbitrationToolDefinition{name: "write_evidence_chunk", schema: arbitrationWriteChunkSchema()},
			arbitrationToolDefinition{name: "commit_evidence_upload", schema: arbitrationCommitUploadSchema()},
			arbitrationToolDefinition{name: "submit_evidence", schema: arbitrationSubmitEvidenceSchema()},
			arbitrationToolDefinition{name: "submit_decision", schema: arbitrationSubmitDecisionSchema()},
		), nil
	default:
		return nil, fmt.Errorf("unknown arbitration assignment type %q", profile.AssignmentType)
	}
}

func (a *arbitrationAdapter) CallTool(ctx context.Context, profile Profile, name string, arguments map[string]any) (CallResult, error) {
	available, err := a.toolAvailable(profile, name)
	if err != nil {
		return CallResult{}, err
	}
	if !available {
		return CallResult{}, fmt.Errorf("tool %q is unavailable for %s %s", name, profile.AssignmentType, profile.PrincipalID)
	}
	switch name {
	case "get_current_opportunity":
		return a.get(ctx, profile, "/get")
	case arbitrationWaitTool:
		return a.waitForOpportunity(ctx, profile, arguments)
	case "case_status":
		return a.getFromLawyerAPI(ctx, profile, "/status")
	case "get_case_result":
		return a.getFromLawyerAPI(ctx, profile, "/result")
	default:
		return a.postTool(ctx, profile, name, arguments)
	}
}

func (a *arbitrationAdapter) toolAvailable(profile Profile, name string) (bool, error) {
	definitions, err := a.toolDefinitions(profile)
	if err != nil {
		return false, err
	}
	for _, definition := range definitions {
		if definition.name == name {
			return true, nil
		}
	}
	return false, nil
}

func (a *arbitrationAdapter) get(ctx context.Context, profile Profile, endpoint string) (CallResult, error) {
	value, err := a.client.Get(ctx, a.apiPath(profile)+endpoint, a.query(profile))
	return arbitrationResult(value, err), nil
}

func (a *arbitrationAdapter) getFromLawyerAPI(ctx context.Context, profile Profile, endpoint string) (CallResult, error) {
	value, err := a.client.Get(ctx, arbitrationLawyerAPIPath+endpoint, a.query(profile))
	return arbitrationResult(value, err), nil
}

func (a *arbitrationAdapter) waitForOpportunity(ctx context.Context, profile Profile, arguments map[string]any) (CallResult, error) {
	if err := arbitrationRequireKeys(arguments, "after_opportunity_id", "after_version", "timeout_ms"); err != nil {
		return arbitrationErrorResult(err), nil
	}
	timeout, err := arbitrationWaitTimeout(arguments["timeout_ms"])
	if err != nil {
		return arbitrationErrorResult(err), nil
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout+arbitrationWaitMargin)
	defer cancel()
	query := a.query(profile)
	query.Set("timeout_ms", strconv.FormatInt(timeout.Milliseconds(), 10))
	if after := String(arguments["after_opportunity_id"]); after != "" {
		query.Set("after", after)
	}
	if version := NumberString(arguments["after_version"]); version != "" {
		query.Set("after_version", version)
	}
	value, err := a.client.Get(waitCtx, a.apiPath(profile)+"/wait", query)
	if err != nil {
		return arbitrationErrorResult(err), nil
	}
	state := arbitrationWaitState(value)
	value["state"] = state
	if version := arbitrationWaitVersion(value); version != nil {
		value["after_version"] = version
	}
	if opportunityID := arbitrationOpportunityID(value); opportunityID != "" {
		value["after_opportunity_id"] = opportunityID
	}
	message, err := a.prompts.Text(arbitrationWaitPromptID(state))
	if err != nil {
		return CallResult{}, err
	}
	value["message"] = message
	return CallResult{StructuredContent: value, IsError: state == "error"}, nil
}

func (a *arbitrationAdapter) postTool(ctx context.Context, profile Profile, name string, arguments map[string]any) (CallResult, error) {
	status, err := a.client.Get(ctx, a.apiPath(profile)+"/get", a.query(profile))
	if err != nil {
		return arbitrationErrorResult(err), nil
	}
	if ok, exists := status["ok"].(bool); exists && !ok {
		return CallResult{StructuredContent: status, IsError: true}, nil
	}
	body := map[string]any{
		"case_id":   profile.CaseID,
		"tool":      name,
		"arguments": arguments,
	}
	if profile.AssignmentType == "council" {
		body["member_id"] = profile.PrincipalID
	} else {
		body["role_id"] = profile.PrincipalID
	}
	if profile.AssignmentType != "observer" {
		opportunityID := arbitrationOpportunityID(status)
		if opportunityID == "" {
			return arbitrationErrorResult(fmt.Errorf("current turn has no opportunity_id")), nil
		}
		body["opportunity_id"] = opportunityID
	}
	value, err := a.client.Post(ctx, a.apiPath(profile)+"/do", body)
	return arbitrationResult(value, err), nil
}

func (a *arbitrationAdapter) apiPath(profile Profile) string {
	if profile.AssignmentType == "council" {
		return arbitrationCouncilAPIPath
	}
	return arbitrationLawyerAPIPath
}

func (a *arbitrationAdapter) query(profile Profile) url.Values {
	query := url.Values{}
	query.Set("case_id", profile.CaseID)
	if profile.AssignmentType == "council" {
		query.Set("member_id", profile.PrincipalID)
	} else {
		query.Set("role_id", profile.PrincipalID)
	}
	return query
}

func arbitrationResult(value map[string]any, err error) CallResult {
	if err != nil {
		return arbitrationErrorResult(err)
	}
	isError := false
	if ok, exists := value["ok"].(bool); exists {
		isError = !ok
	}
	return CallResult{StructuredContent: value, IsError: isError}
}

func arbitrationErrorResult(err error) CallResult {
	return CallResult{StructuredContent: map[string]any{"ok": false, "error": err.Error()}, IsError: true}
}

func arbitrationWaitTimeout(value any) (time.Duration, error) {
	if value == nil {
		return arbitrationWaitDefault, nil
	}
	raw := NumberString(value)
	if raw == "" {
		return 0, fmt.Errorf("timeout_ms must be an integer")
	}
	milliseconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || milliseconds <= 0 {
		return 0, fmt.Errorf("timeout_ms must be a positive integer")
	}
	timeout := time.Duration(milliseconds) * time.Millisecond
	if timeout > arbitrationWaitMax {
		return arbitrationWaitMax, nil
	}
	return timeout, nil
}

func arbitrationWaitState(value map[string]any) string {
	if ok, exists := value["ok"].(bool); exists && !ok {
		return "error"
	}
	switch strings.ToLower(String(value["status"])) {
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

func arbitrationWaitVersion(value map[string]any) any {
	wait, _ := value["wait"].(map[string]any)
	return wait["version"]
}

func arbitrationOpportunityID(value map[string]any) string {
	turn, _ := value["turn"].(map[string]any)
	return String(turn["opportunity_id"])
}

func arbitrationWaitPromptID(state string) string {
	switch state {
	case "ready", "done", "failed", "error":
		return "mcp.wait." + state
	default:
		return "mcp.wait.waiting"
	}
}

func arbitrationRequireKeys(arguments map[string]any, allowed ...string) error {
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

func arbitrationWaitSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"after_opportunity_id": map[string]any{"type": "string"},
			"after_version":        map[string]any{"type": "integer", "minimum": 0},
			"timeout_ms":           map[string]any{"type": "integer", "minimum": 1, "maximum": arbitrationWaitMax.Milliseconds()},
		},
		"additionalProperties": false,
	}
}

func arbitrationEvidenceIDSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"evidence_id": map[string]any{"type": "string"}}, "required": []string{"evidence_id"}, "additionalProperties": false}
}

func arbitrationEvidenceRangeSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"evidence_id": map[string]any{"type": "string"},
			"offset":      map[string]any{"type": "integer", "minimum": 0},
			"length":      map[string]any{"type": "integer", "minimum": 1},
		},
		"required":             []string{"evidence_id", "offset", "length"},
		"additionalProperties": false,
	}
}

func arbitrationWorkNotesSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"notes": map[string]any{"type": "string"}}, "required": []string{"notes"}, "additionalProperties": false}
}

func arbitrationBeginUploadSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title":               map[string]any{"type": "string"},
			"mime_type":           map[string]any{"type": "string"},
			"expected_size_bytes": map[string]any{"type": "integer", "minimum": 1},
			"expected_sha256":     map[string]any{"type": "string"},
			"source_url":          map[string]any{"type": "string"},
			"source_description":  map[string]any{"type": "string"},
			"retrieval_timestamp": map[string]any{"type": "string"},
			"relevance":           map[string]any{"type": "string"},
			"parent_evidence_id":  map[string]any{"type": "string"},
			"derivation_method":   map[string]any{"type": "string"},
		},
		"required":             []string{"title", "mime_type", "expected_size_bytes", "relevance"},
		"additionalProperties": false,
	}
}

func arbitrationWriteChunkSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"upload_id": map[string]any{"type": "string"}, "offset": map[string]any{"type": "integer", "minimum": 0}, "content_base64": map[string]any{"type": "string"}}, "required": []string{"upload_id", "offset", "content_base64"}, "additionalProperties": false}
}

func arbitrationCommitUploadSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"upload_id": map[string]any{"type": "string"}, "expected_sha256": map[string]any{"type": "string"}, "preferred_filename_ext": map[string]any{"type": "string"}}, "required": []string{"upload_id"}, "additionalProperties": false}
}

func arbitrationSubmitEvidenceSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title":                  map[string]any{"type": "string"},
			"source_url":             map[string]any{"type": "string"},
			"source_description":     map[string]any{"type": "string"},
			"retrieval_timestamp":    map[string]any{"type": "string"},
			"mime_type":              map[string]any{"type": "string"},
			"relevance":              map[string]any{"type": "string"},
			"content":                map[string]any{"type": "string"},
			"content_base64":         map[string]any{"type": "string"},
			"preferred_filename_ext": map[string]any{"type": "string"},
		},
		"required":             []string{"title", "mime_type", "relevance"},
		"additionalProperties": false,
	}
}

func arbitrationSubmitDecisionSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"kind":      map[string]any{"type": "string", "enum": []string{"tool", "pass"}},
			"tool_name": map[string]any{"type": "string", "enum": []string{"record_opening_statement", "submit_argument", "submit_rebuttal", "submit_surrebuttal", "deliver_closing_statement", "pass_phase_opportunity"}},
			"payload":   arbitrationAttorneyPayloadSchema(),
		},
		"required":             []string{"kind"},
		"additionalProperties": false,
	}
}

func arbitrationAttorneyPayloadSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":              map[string]any{"type": "string"},
			"offered_evidence":  arbitrationOfferedEvidenceSchema(),
			"technical_reports": arbitrationTechnicalReportsSchema(),
		},
		"additionalProperties": false,
	}
}

func arbitrationOfferedEvidenceSchema() map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"evidence_id": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"}}, "required": []string{"evidence_id", "label"}, "additionalProperties": false}}
}

func arbitrationTechnicalReportsSchema() map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"title": map[string]any{"type": "string"}, "summary": map[string]any{"type": "string"}}, "required": []string{"title", "summary"}, "additionalProperties": false}}
}

func arbitrationListEventsSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"offset": map[string]any{"type": "integer", "minimum": 0}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 1000}}, "additionalProperties": false}
}
