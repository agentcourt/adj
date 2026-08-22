package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	adcprompts "github.com/jsmorph/adj/adc/runtime/prompts"
	"github.com/jsmorph/adj/adc/runtime/spec"
	"github.com/jsmorph/adj/common/modelrequest"
	openaiapi "github.com/jsmorph/adj/common/openai"
)

func (r *Runner) executeTurn(
	ctx context.Context,
	turnIndex int,
	role spec.RoleSpec,
	turn spec.TurnSpec,
	allowed []string,
) (TurnLog, error) {
	transcript := make([]map[string]any, 0)
	if turn.DeterministicAction != nil {
		steps, err := r.executeDeterministicTurn(turnIndex, role.Name, turn.DeterministicAction, &transcript)
		if err != nil {
			return TurnLog{}, err
		}
		if turn.RequireSuccess && !hasSuccessfulResult(transcript) {
			return TurnLog{}, fmt.Errorf(
				"required deterministic turn failed turn=%d role=%s action=%s response=%s",
				turnIndex,
				role.Name,
				turn.DeterministicAction.ActionType,
				marshalString(transcript),
			)
		}
		return TurnLog{Role: role.Name, Prompt: turn.Prompt, Steps: steps, Transcript: transcript}, nil
	}
	if r.cfg.Offline {
		return TurnLog{}, fmt.Errorf("turn requires LLM in offline mode turn=%d role=%s", turnIndex, role.Name)
	}
	if r.client == nil {
		return TurnLog{}, fmt.Errorf("llm client is nil")
	}
	view, err := r.lean.View(r.state, role.Name)
	if err != nil {
		return TurnLog{}, err
	}
	systemPrompt, err := r.buildSystemPrompt(role, view)
	if err != nil {
		return TurnLog{}, err
	}
	turnPrompt, err := r.buildTurnPrompt(role.Name, turn.Prompt, allowed)
	if err != nil {
		return TurnLog{}, err
	}
	conversation := []map[string]any{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": turnPrompt},
	}
	tools, err := r.buildTools(allowed)
	if err != nil {
		return TurnLog{}, err
	}
	inputItems := append([]map[string]any{}, conversation...)
	prevID := ""
	steps := 0
	invalidAttempts := 0
	invalidAttemptReasons := make([]string, 0)
	maxInvalidAttemptsPerTurn := r.cfg.Runtime.Normalized().InvalidAttemptLimit
	successfulCalls := map[string]bool{}
	for steps < turn.MaxSteps {
		steps++
		resp, err := r.client.CreateResponse(ctx, r.effectiveRoleModel(role), inputItems, tools, prevID, r.effectiveRoleTemperature(role))
		if err != nil {
			return TurnLog{}, err
		}
		if err := enforceResponseSizeLimit(resp, r.cfg.Runtime.MaxResponseBytes); err != nil {
			return TurnLog{}, err
		}
		prevID = resp.ResponseID
		if len(resp.ToolCalls) == 0 {
			transcript = append(transcript, map[string]any{"assistant_text": resp.Text})
			break
		}
		callOutputs := make([]map[string]any, 0, len(resp.ToolCalls))
		followupInputItems := make([]map[string]any, 0)
		issues := make([]correctionIssue, 0, len(resp.ToolCalls))
		for _, call := range resp.ToolCalls {
			if strings.TrimSpace(call.ArgumentsError) != "" {
				issue, err := r.issueFromMalformedToolCall(call)
				if err != nil {
					return TurnLog{}, err
				}
				out := malformedToolCallOutput(issue)
				callOutputs = append(callOutputs, map[string]any{
					"type":    "function_call_output",
					"call_id": call.CallID,
					"output":  marshalString(out),
				})
				transcript = append(transcript, map[string]any{
					"action":          call.Name,
					"raw_arguments":   call.RawArguments,
					"arguments_error": call.ArgumentsError,
					"result":          out,
				})
				issues = append(issues, issue)
				continue
			}
			if !contains(allowed, call.Name) {
				out := map[string]any{"ok": false, "error": "tool not allowed", "tool": call.Name}
				callOutputs = append(callOutputs, map[string]any{
					"type":    "function_call_output",
					"call_id": call.CallID,
					"output":  marshalString(out),
				})
				transcript = append(transcript, map[string]any{"action": call.Name, "result": out})
				issues = append(issues, correctionIssue{
					Tool:  call.Name,
					Error: fmt.Sprintf("tool %s is not allowed in this turn", call.Name),
				})
				continue
			}
			if successfulCalls[call.Name] {
				duplicate := map[string]any{
					"ok":                true,
					"duplicate_ignored": true,
					"tool":              call.Name,
				}
				callOutputs = append(callOutputs, map[string]any{
					"type":    "function_call_output",
					"call_id": call.CallID,
					"output":  marshalString(duplicate),
				})
				transcript = append(transcript, map[string]any{"action": call.Name, "arguments": call.Arguments, "result": duplicate})
				continue
			}
			execRes, err := r.executeAction(turnIndex, steps, role.Name, call.Name, call.Arguments)
			if err != nil {
				return TurnLog{}, err
			}
			res := execRes.Result
			transcript = append(transcript, map[string]any{"action": call.Name, "arguments": call.Arguments, "result": res})
			callOutputs = append(callOutputs, map[string]any{
				"type":    "function_call_output",
				"call_id": call.CallID,
				"output":  marshalString(res),
			})
			followupInputItems = append(followupInputItems, execRes.FollowupInputItems...)
			if ok, _ := res["ok"].(bool); ok {
				successfulCalls[call.Name] = true
				continue
			}
			if redundantAfterSuccess(call.Name, res, successfulCalls) {
				continue
			}
			issues = append(issues, issueFromResult(call.Name, res))
		}
		inputItems = callOutputs
		if len(followupInputItems) > 0 {
			inputItems = append(inputItems, followupInputItems...)
		}
		if len(issues) > 0 {
			issueTexts := make([]string, 0, len(issues))
			for _, issue := range issues {
				issueTexts = append(issueTexts, formatIssue(issue))
			}
			invalidAttemptReasons = append(invalidAttemptReasons, strings.Join(issueTexts, "; "))
			invalidAttempts++
			if _, err := fmt.Fprintf(
				os.Stderr,
				"agent correction turn=%d role=%s invalid_attempt=%d/%d reason=%s\n",
				turnIndex,
				role.Name,
				invalidAttempts,
				maxInvalidAttemptsPerTurn,
				strings.Join(issueTexts, "; "),
			); err != nil {
				return TurnLog{}, fmt.Errorf("write agent correction: %w", err)
			}
			if invalidAttempts >= maxInvalidAttemptsPerTurn {
				return TurnLog{}, formatInvalidAttemptLimitError(fmt.Sprintf("agent turn=%d role=%s", turnIndex, role.Name), invalidAttemptReasons)
			}
			correctionPrompt, err := r.buildCorrectionPrompt(role.Name, issues, allowed, false)
			if err != nil {
				return TurnLog{}, err
			}
			inputItems = append(
				inputItems,
				map[string]any{
					"role":    "user",
					"content": correctionPrompt,
				},
			)
		}
	}
	if turn.RequireSuccess && !hasSuccessfulResult(transcript) {
		return TurnLog{}, fmt.Errorf(
			"required turn failed turn=%d role=%s transcript=%s",
			turnIndex,
			role.Name,
			marshalString(transcript),
		)
	}
	return TurnLog{Role: role.Name, Prompt: turn.Prompt, Steps: steps, Transcript: transcript}, nil
}

func (r *Runner) executeOpportunityTurn(
	ctx context.Context,
	turnIndex int,
	role spec.RoleSpec,
	opportunity leanOpportunity,
	rolesPayload []map[string]any,
	stateVersion int,
) (TurnLog, error) {
	if r.cfg.Offline {
		return TurnLog{}, fmt.Errorf("opportunity requires LLM in offline mode turn=%d role=%s", turnIndex, role.Name)
	}
	if r.client == nil {
		return TurnLog{}, fmt.Errorf("llm client is nil")
	}
	view, err := r.lean.View(r.state, role.Name)
	if err != nil {
		return TurnLog{}, err
	}
	transcript := make([]map[string]any, 0)
	systemPrompt, err := r.buildSystemPrompt(role, view)
	if err != nil {
		return TurnLog{}, err
	}
	activeModel := r.effectiveRoleModel(role)
	responseClient := r.client
	var activeRequestSpec *modelrequest.Spec
	if role.Name == "juror" {
		caseObj, _ := r.state["case"].(map[string]any)
		jurorModel, jurorPersona := r.jurorOpportunityPromptContext(opportunity)
		systemPrompt, err = r.buildJurorSystemPrompt(role, opportunity, jurorPersona, caseObj)
		if err != nil {
			return TurnLog{}, err
		}
		if strings.TrimSpace(jurorModel) != "" {
			activeModel = jurorModel
		}
		activeRequestSpec = r.jurorOpportunityRequestSpec(opportunity)
		responseClient, err = r.jurorResponseClient(activeModel)
		if err != nil {
			return TurnLog{}, err
		}
	}
	opportunityPrompt, err := r.buildOpportunityPrompt(role, opportunity)
	if err != nil {
		return TurnLog{}, err
	}
	turnPrompt, err := r.buildTurnPrompt(role.Name, opportunityPrompt, opportunityCallableTools(role, opportunity))
	if err != nil {
		return TurnLog{}, err
	}
	conversation := []map[string]any{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": turnPrompt},
	}
	referenceTools := referenceToolsForRole(role)
	callableTools := opportunityCallableTools(role, opportunity)
	tools, err := r.buildOpportunityTools(opportunity.AllowedTools, referenceTools, opportunity.MayPass)
	if err != nil {
		return TurnLog{}, err
	}
	inputItems := append([]map[string]any{}, conversation...)
	prevID := ""
	steps := 0
	decisionSteps := 0
	supportSteps := 0
	supportBudget := supportToolBudget(r.state)
	invalidAttempts := 0
	invalidAttemptReasons := make([]string, 0)
	agentEventSeq := 0
	maxInvalidAttemptsPerTurn := r.cfg.Runtime.Normalized().InvalidAttemptLimit
	recordInvalidAttempt := func(issue correctionIssue) error {
		if _, err := fmt.Fprintf(
			os.Stderr,
			"agent correction turn=%d role=%s invalid_attempt=%d/%d reason=%s\n",
			turnIndex,
			role.Name,
			invalidAttempts+1,
			maxInvalidAttemptsPerTurn,
			formatIssue(issue),
		); err != nil {
			return fmt.Errorf("write agent correction: %w", err)
		}
		invalidAttemptReasons = append(invalidAttemptReasons, formatIssue(issue))
		invalidAttempts++
		if invalidAttempts >= maxInvalidAttemptsPerTurn {
			return formatInvalidAttemptLimitError(fmt.Sprintf("agent turn=%d role=%s", turnIndex, role.Name), invalidAttemptReasons)
		}
		return nil
	}
	recordCompletionResult := func(resp openaiapi.Response, status string, issue *correctionIssue, invalidAttempt int) error {
		agentEventSeq++
		return r.persistAgentCompletionResult(
			turnIndex,
			agentEventSeq,
			role.Name,
			completionResultPayload(activeModel, opportunity, resp, status, issue, invalidAttempt),
		)
	}
	for decisionSteps < opportunity.StepBudget {
		steps++
		var resp openaiapi.Response
		if activeRequestSpec != nil {
			resp, err = responseClient.CreateResponseWithRequestSpec(ctx, *activeRequestSpec, inputItems, tools, prevID)
		} else {
			resp, err = responseClient.CreateResponse(ctx, activeModel, inputItems, tools, prevID, r.effectiveRoleTemperature(role))
		}
		if err != nil {
			if timeoutLog, handled, handleErr := r.handleOpportunityResponseError(turnIndex, role, opportunity, activeModel, err); handled {
				if handleErr != nil {
					return TurnLog{}, handleErr
				}
				return timeoutLog, nil
			}
			return TurnLog{}, err
		}
		if err := enforceResponseSizeLimit(resp, r.cfg.Runtime.MaxResponseBytes); err != nil {
			return TurnLog{}, err
		}
		prevID = resp.ResponseID
		resp = recoverLiteralToolCall(resp, appendOpportunityAllowedTools(opportunity.AllowedTools, referenceTools, opportunity.MayPass))
		if len(resp.ToolCalls) == 0 {
			decisionSteps++
			actorMessage, err := r.prompts.Render(adcprompts.RuntimeCorrectionNoToolID, map[string]string{
				"{{PASS_ACTION}}": passAction(opportunity.MayPass),
			})
			if err != nil {
				return TurnLog{}, err
			}
			issue := correctionIssue{
				Tool:         "none",
				Error:        "no tool call returned",
				ActorMessage: actorMessage,
			}
			if err := recordCompletionResult(resp, "rejected", &issue, invalidAttempts+1); err != nil {
				return TurnLog{}, err
			}
			if supportSteps == 0 {
				prevID = ""
				inputItems, err = r.restartOpportunityCorrection(conversation, role.Name, issue, opportunity, referenceTools)
			} else {
				inputItems, err = r.appendOpportunityCorrection(role.Name, nil, issue, opportunity, referenceTools)
			}
			if err != nil {
				return TurnLog{}, err
			}
			if err := recordInvalidAttempt(issue); err != nil {
				return TurnLog{}, err
			}
			continue
		}
		if len(resp.ToolCalls) != 1 {
			decisionSteps++
			actorMessage, err := r.prompts.Text(adcprompts.RuntimeCorrectionMultipleID)
			if err != nil {
				return TurnLog{}, err
			}
			issue := correctionIssue{
				Tool:         "multiple",
				Error:        "multiple tool calls returned",
				ActorMessage: actorMessage,
			}
			callOutputs := make([]map[string]any, 0, len(resp.ToolCalls))
			for _, call := range resp.ToolCalls {
				out := map[string]any{
					"ok":            false,
					"error":         "multiple tool calls are not allowed",
					"actor_message": actorMessage,
					"tool":          call.Name,
				}
				callOutputs = append(callOutputs, map[string]any{
					"type":    "function_call_output",
					"call_id": call.CallID,
					"output":  marshalString(out),
				})
			}
			if err := recordCompletionResult(resp, "rejected", &issue, invalidAttempts+1); err != nil {
				return TurnLog{}, err
			}
			if supportSteps == 0 {
				prevID = ""
				inputItems, err = r.restartOpportunityCorrection(conversation, role.Name, issue, opportunity, referenceTools)
			} else {
				inputItems = nextInputItems(callOutputs)
				inputItems, err = r.appendOpportunityCorrection(role.Name, inputItems, issue, opportunity, referenceTools)
			}
			if err != nil {
				return TurnLog{}, err
			}
			if err := recordInvalidAttempt(issue); err != nil {
				return TurnLog{}, err
			}
			continue
		}
		call := resp.ToolCalls[0]
		if strings.TrimSpace(call.ArgumentsError) != "" {
			decisionSteps++
			issue, err := r.issueFromMalformedToolCall(call)
			if err != nil {
				return TurnLog{}, err
			}
			if err := recordCompletionResult(resp, "rejected", &issue, invalidAttempts+1); err != nil {
				return TurnLog{}, err
			}
			if supportSteps == 0 {
				prevID = ""
				inputItems, err = r.restartOpportunityCorrection(conversation, role.Name, issue, opportunity, referenceTools)
			} else {
				callOutput := map[string]any{
					"type":    "function_call_output",
					"call_id": call.CallID,
					"output":  marshalString(malformedToolCallOutput(issue)),
				}
				inputItems = nextInputItems([]map[string]any{callOutput})
				inputItems, err = r.appendOpportunityCorrection(role.Name, inputItems, issue, opportunity, referenceTools)
			}
			if err != nil {
				return TurnLog{}, err
			}
			if err := recordInvalidAttempt(issue); err != nil {
				return TurnLog{}, err
			}
			continue
		}
		if !contains(callableTools, call.Name) {
			decisionSteps++
			actorMessage, err := r.prompts.Text(adcprompts.RuntimeCorrectionDisallowedID)
			if err != nil {
				return TurnLog{}, err
			}
			out := map[string]any{"ok": false, "error": "tool not allowed", "tool": call.Name, "actor_message": actorMessage}
			issue := correctionIssue{
				Tool:         call.Name,
				Error:        fmt.Sprintf("tool %s is not allowed in this opportunity", call.Name),
				ActorMessage: actorMessage,
			}
			if err := recordCompletionResult(resp, "rejected", &issue, invalidAttempts+1); err != nil {
				return TurnLog{}, err
			}
			if supportSteps == 0 {
				prevID = ""
				inputItems, err = r.restartOpportunityCorrection(conversation, role.Name, issue, opportunity, referenceTools)
			} else {
				callOutput := map[string]any{
					"type":    "function_call_output",
					"call_id": call.CallID,
					"output":  marshalString(out),
				}
				inputItems = nextInputItems([]map[string]any{callOutput})
				inputItems, err = r.appendOpportunityCorrection(role.Name, inputItems, issue, opportunity, referenceTools)
			}
			if err != nil {
				return TurnLog{}, err
			}
			if err := recordInvalidAttempt(issue); err != nil {
				return TurnLog{}, err
			}
			continue
		}
		if isReferenceTool(call.Name) {
			if supportSteps >= supportBudget {
				actorMessage, err := r.prompts.Text(adcprompts.RuntimeCorrectionSupportID)
				if err != nil {
					return TurnLog{}, err
				}
				issue := correctionIssue{
					Tool:         call.Name,
					Error:        "support-tool budget exhausted",
					ActorMessage: actorMessage,
				}
				if err := recordCompletionResult(resp, "rejected", &issue, invalidAttempts+1); err != nil {
					return TurnLog{}, err
				}
				callOutput := map[string]any{
					"type":    "function_call_output",
					"call_id": call.CallID,
					"output": marshalString(map[string]any{
						"ok":            false,
						"error":         issue.Error,
						"actor_message": issue.ActorMessage,
					}),
				}
				inputItems = nextInputItems([]map[string]any{callOutput})
				inputItems, err = r.appendOpportunityCorrection(role.Name, inputItems, issue, opportunity, referenceTools)
				if err != nil {
					return TurnLog{}, err
				}
				if err := recordInvalidAttempt(issue); err != nil {
					return TurnLog{}, err
				}
				continue
			}
			supportSteps++
			execRes, err := r.executeAction(turnIndex, steps, role.Name, call.Name, call.Arguments)
			if err != nil {
				return TurnLog{}, err
			}
			res := execRes.Result
			transcript = append(transcript, map[string]any{"action": call.Name, "arguments": call.Arguments, "result": res})
			callOutput := map[string]any{
				"type":    "function_call_output",
				"call_id": call.CallID,
				"output":  marshalString(res),
			}
			inputItems = nextInputItems([]map[string]any{callOutput}, execRes.FollowupInputItems)
			if ok, _ := res["ok"].(bool); ok {
				if err := recordCompletionResult(resp, "accepted", nil, 0); err != nil {
					return TurnLog{}, err
				}
				continue
			}
			issue := issueFromResult(call.Name, res)
			if err := recordCompletionResult(resp, "rejected", &issue, invalidAttempts+1); err != nil {
				return TurnLog{}, err
			}
			inputItems, err = r.appendOpportunityCorrection(role.Name, inputItems, issue, opportunity, referenceTools)
			if err != nil {
				return TurnLog{}, err
			}
			if err := recordInvalidAttempt(issue); err != nil {
				return TurnLog{}, err
			}
			continue
		}
		decisionSteps++
		var decision map[string]any
		if call.Name == "pass_turn" {
			decision = map[string]any{
				"kind":   "pass",
				"reason": strings.TrimSpace(stringOrDefault(call.Arguments["reason"], "")),
			}
		} else {
			payload, issue, err := r.applyOpportunityPayloadDefaults(call.Name, call.Arguments, opportunity)
			if err != nil {
				return TurnLog{}, err
			}
			if issue != nil {
				if err := recordCompletionResult(resp, "rejected", issue, invalidAttempts+1); err != nil {
					return TurnLog{}, err
				}
				if supportSteps == 0 {
					prevID = ""
					inputItems, err = r.restartOpportunityCorrection(conversation, role.Name, *issue, opportunity, referenceTools)
				} else {
					callOutput := map[string]any{
						"type":    "function_call_output",
						"call_id": call.CallID,
						"output":  marshalString(map[string]any{"ok": false, "error": issue.Error, "actor_message": issue.ActorMessage}),
					}
					inputItems = nextInputItems([]map[string]any{callOutput})
					inputItems, err = r.appendOpportunityCorrection(role.Name, inputItems, *issue, opportunity, referenceTools)
				}
				if err != nil {
					return TurnLog{}, err
				}
				if err := recordInvalidAttempt(*issue); err != nil {
					return TurnLog{}, err
				}
				continue
			}
			decision = map[string]any{
				"kind":      "tool",
				"tool_name": call.Name,
				"payload":   payload,
			}
		}
		acceptResp, err := r.lean.ApplyDecision(r.state, stateVersion, opportunity.OpportunityID, role.Name, decision, rolesPayload, opportunity.StepBudget)
		if err != nil {
			return TurnLog{}, err
		}
		transcript = append(transcript, map[string]any{"decision": decision, "acceptance": acceptResp})
		if ok, _ := acceptResp["ok"].(bool); !ok {
			issue := issueFromResult(call.Name, acceptResp)
			if err := recordCompletionResult(resp, "rejected", &issue, invalidAttempts+1); err != nil {
				return TurnLog{}, err
			}
			if supportSteps == 0 {
				prevID = ""
				inputItems, err = r.restartOpportunityCorrection(conversation, role.Name, issue, opportunity, referenceTools)
			} else {
				callOutput := map[string]any{
					"type":    "function_call_output",
					"call_id": call.CallID,
					"output":  marshalString(acceptResp),
				}
				inputItems = nextInputItems([]map[string]any{callOutput})
				inputItems, err = r.appendOpportunityCorrection(role.Name, inputItems, issue, opportunity, referenceTools)
			}
			if err != nil {
				return TurnLog{}, err
			}
			if err := recordInvalidAttempt(issue); err != nil {
				return TurnLog{}, err
			}
			continue
		}
		resultKind := strings.TrimSpace(stringOrDefault(acceptResp["result_kind"], ""))
		if resultKind == "pass_recorded" {
			state, _ := acceptResp["state"].(map[string]any)
			if state == nil {
				return TurnLog{}, fmt.Errorf("lean apply_decision pass_recorded missing state")
			}
			if err := r.recordApplyDecisionForCertificate(stateVersion, opportunity.OpportunityID, role.Name, decision, rolesPayload, opportunity.StepBudget); err != nil {
				return TurnLog{}, err
			}
			r.state = mergeLocalCaseExtensions(r.state, state)
			if err := recordCompletionResult(resp, "accepted", nil, 0); err != nil {
				return TurnLog{}, err
			}
			if err := r.persistActionEvent(turnIndex, 1, role.Name, "pass_turn", call.Arguments, acceptResp); err != nil {
				return TurnLog{}, err
			}
			transcript = append(transcript, map[string]any{"action": "pass_turn", "arguments": call.Arguments, "result": acceptResp})
			return TurnLog{Role: role.Name, Prompt: opportunity.Objective, Steps: steps, Transcript: transcript}, nil
		}
		if resultKind != "execute_tool" {
			return TurnLog{}, fmt.Errorf("lean apply_decision returned unsupported result_kind: %s", resultKind)
		}
		action, _ := acceptResp["action"].(map[string]any)
		if action == nil {
			return TurnLog{}, fmt.Errorf("lean apply_decision execute_tool missing action")
		}
		actionType := strings.TrimSpace(stringOrDefault(action["action_type"], ""))
		actorRole := strings.TrimSpace(stringOrDefault(action["actor_role"], role.Name))
		payload, _ := action["payload"].(map[string]any)
		if payload == nil {
			payload = map[string]any{}
		}
		execRes, err := r.executeAction(turnIndex, 1, actorRole, actionType, payload)
		if err != nil {
			return TurnLog{}, err
		}
		res := execRes.Result
		transcript = append(transcript, map[string]any{"action": actionType, "arguments": payload, "result": res})
		if ok, _ := res["ok"].(bool); !ok {
			issue := issueFromResult(actionType, res)
			if err := recordCompletionResult(resp, "rejected", &issue, invalidAttempts+1); err != nil {
				return TurnLog{}, err
			}
			if supportSteps == 0 {
				prevID = ""
				inputItems, err = r.restartOpportunityCorrection(conversation, role.Name, issue, opportunity, referenceTools)
			} else {
				callOutput := map[string]any{
					"type":    "function_call_output",
					"call_id": call.CallID,
					"output":  marshalString(res),
				}
				inputItems = nextInputItems([]map[string]any{callOutput})
				inputItems, err = r.appendOpportunityCorrection(role.Name, inputItems, issue, opportunity, referenceTools)
			}
			if err != nil {
				return TurnLog{}, err
			}
			if err := recordInvalidAttempt(issue); err != nil {
				return TurnLog{}, err
			}
			continue
		}
		if err := recordCompletionResult(resp, "accepted", nil, 0); err != nil {
			return TurnLog{}, err
		}
		return TurnLog{Role: role.Name, Prompt: opportunity.Objective, Steps: steps, Transcript: transcript}, nil
	}
	return TurnLog{}, fmt.Errorf("opportunity exhausted decision budget turn=%d role=%s opportunity_id=%s", turnIndex, role.Name, opportunity.OpportunityID)
}

func (r *Runner) appendOpportunityCorrection(
	roleName string,
	inputItems []map[string]any,
	issue correctionIssue,
	opportunity leanOpportunity,
	referenceTools []string,
) ([]map[string]any, error) {
	correctionPrompt, err := r.buildCorrectionPrompt(roleName, []correctionIssue{issue}, appendOpportunityAllowedTools(opportunity.AllowedTools, referenceTools, opportunity.MayPass), opportunity.MayPass)
	if err != nil {
		return nil, err
	}
	return append(
		inputItems,
		map[string]any{
			"role":    "user",
			"content": correctionPrompt,
		},
	), nil
}

func nextInputItems(items ...[]map[string]any) []map[string]any {
	var out []map[string]any
	for _, itemSet := range items {
		if len(itemSet) == 0 {
			continue
		}
		out = append(out, itemSet...)
	}
	return out
}

func (r *Runner) restartOpportunityCorrection(
	conversation []map[string]any,
	roleName string,
	issue correctionIssue,
	opportunity leanOpportunity,
	referenceTools []string,
) ([]map[string]any, error) {
	return r.appendOpportunityCorrection(
		roleName,
		append([]map[string]any{}, conversation...),
		issue,
		opportunity,
		referenceTools,
	)
}

func appendOpportunityAllowedTools(allowed []string, reference []string, mayPass bool) []string {
	out := append([]string{}, allowed...)
	for _, name := range reference {
		out = appendIfMissing(out, name)
	}
	if mayPass {
		out = append(out, "pass_turn")
	}
	return out
}

func (r *Runner) applyOpportunityPayloadDefaults(toolName string, arguments map[string]any, opportunity leanOpportunity) (map[string]any, *correctionIssue, error) {
	defaults := mapOrEmpty(opportunity.Constraints["payload_defaults"])
	if len(defaults) == 0 {
		return clonePayload(arguments), nil, nil
	}
	merged := clonePayload(arguments)
	conflicts := make([]string, 0)
	for key, want := range defaults {
		got, ok := merged[key]
		if !ok {
			merged[key] = want
			continue
		}
		if !jsonValueEqual(got, want) {
			conflicts = append(conflicts, fmt.Sprintf("%s=%s", key, marshalString(want)))
		}
	}
	if len(conflicts) == 0 {
		return merged, nil, nil
	}
	sort.Strings(conflicts)
	fixedFields := strings.Join(conflicts, ", ")
	actorMessage, err := r.prompts.Render(adcprompts.RuntimeCorrectionFixedID, map[string]string{
		"{{FIXED_FIELDS}}": fixedFields,
	})
	if err != nil {
		return nil, nil, err
	}
	return nil, &correctionIssue{
		Tool:         toolName,
		Error:        "fixed opportunity field set incorrectly: " + fixedFields,
		ActorMessage: actorMessage,
	}, nil
}

func clonePayload(arguments map[string]any) map[string]any {
	if arguments == nil {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(arguments))
	for key, value := range arguments {
		cloned[key] = value
	}
	return cloned
}

func enforceResponseSizeLimit(resp openaiapi.Response, maxBytes int) error {
	if maxBytes <= 0 {
		return nil
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response for size check: %w", err)
	}
	if len(raw) > maxBytes {
		return fmt.Errorf("response exceeded byte limit of %d", maxBytes)
	}
	return nil
}

func completionResultPayload(
	model string,
	opportunity leanOpportunity,
	resp openaiapi.Response,
	status string,
	issue *correctionIssue,
	invalidAttempt int,
) map[string]any {
	payload := map[string]any{
		"model":          strings.TrimSpace(model),
		"opportunity_id": opportunity.OpportunityID,
		"phase":          opportunity.Phase,
		"status":         status,
	}
	if invalidAttempt > 0 {
		payload["invalid_attempt"] = invalidAttempt
	}
	if issue != nil {
		payload["rejection_reason"] = formatIssue(*issue)
		if strings.TrimSpace(issue.Error) != "" {
			payload["error"] = strings.TrimSpace(issue.Error)
		}
		if strings.TrimSpace(issue.Code) != "" {
			payload["code"] = strings.TrimSpace(issue.Code)
		}
	}
	if text := strings.TrimSpace(resp.Text); text != "" {
		payload["response_text"] = text
	}
	if len(resp.ToolCalls) > 0 {
		toolCalls := make([]map[string]any, 0, len(resp.ToolCalls))
		for _, call := range resp.ToolCalls {
			callPayload := map[string]any{
				"call_id":   call.CallID,
				"name":      call.Name,
				"arguments": call.Arguments,
			}
			if strings.TrimSpace(call.RawArguments) != "" {
				callPayload["raw_arguments"] = call.RawArguments
			}
			if strings.TrimSpace(call.ArgumentsError) != "" {
				callPayload["arguments_error"] = call.ArgumentsError
			}
			toolCalls = append(toolCalls, callPayload)
		}
		payload["tool_calls"] = toolCalls
	}
	return payload
}

func recoverLiteralToolCall(resp openaiapi.Response, allowed []string) openaiapi.Response {
	if len(resp.ToolCalls) > 0 {
		return resp
	}
	name, arguments, ok := parseLiteralToolCall(strings.TrimSpace(resp.Text), allowed)
	if !ok {
		return resp
	}
	resp.ToolCalls = []openaiapi.ToolCall{{
		CallID:    "synthetic_" + name,
		Name:      name,
		Arguments: arguments,
	}}
	return resp
}

func parseLiteralToolCall(text string, allowed []string) (string, map[string]any, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil, false
	}
	names := append([]string(nil), allowed...)
	sort.Slice(names, func(i, j int) bool {
		return len(names[i]) > len(names[j])
	})
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || !strings.HasPrefix(text, name) {
			continue
		}
		rest := text[len(name):]
		if !literalToolCallBoundary(rest) {
			continue
		}
		arguments, ok := parseLiteralToolArguments(rest)
		if !ok {
			continue
		}
		return name, arguments, true
	}
	return "", nil, false
}

func literalToolCallBoundary(rest string) bool {
	if rest == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_'
}

func parseLiteralToolArguments(rest string) (map[string]any, bool) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return map[string]any{}, true
	}
	open := strings.Index(rest, "{")
	if open < 0 {
		return nil, false
	}
	var arguments map[string]any
	decoder := json.NewDecoder(strings.NewReader(rest[open:]))
	if err := decoder.Decode(&arguments); err != nil {
		return nil, false
	}
	if strings.TrimSpace(rest[open+int(decoder.InputOffset()):]) != "" {
		return nil, false
	}
	if arguments == nil {
		return map[string]any{}, true
	}
	return arguments, true
}

func jsonValueEqual(lhs any, rhs any) bool {
	left, err := json.Marshal(lhs)
	if err != nil {
		return false
	}
	right, err := json.Marshal(rhs)
	if err != nil {
		return false
	}
	return string(left) == string(right)
}

func redundantAfterSuccess(toolName string, result map[string]any, successfulCalls map[string]bool) bool {
	if !successfulCalls[toolName] {
		return false
	}
	errText := strings.ToLower(strings.TrimSpace(stringOrDefault(result["error"], "")))
	return strings.Contains(errText, "already")
}

func (r *Runner) executeDeterministicTurn(
	turnIndex int,
	roleName string,
	deterministic *spec.DeterministicAction,
	transcript *[]map[string]any,
) (int, error) {
	if deterministic == nil {
		return 0, fmt.Errorf("deterministic action is nil")
	}
	kind := strings.TrimSpace(deterministic.Kind)
	if kind == "" || kind == "action" || kind == "single_tool" {
		execRes, err := r.executeAction(turnIndex, 1, roleName, deterministic.ActionType, deterministic.Payload)
		if err != nil {
			return 0, err
		}
		res := execRes.Result
		*transcript = append(*transcript, map[string]any{"action": deterministic.ActionType, "result": res})
		return 1, nil
	}
	if kind == "clerk_panel_setup" {
		caseID := strings.TrimSpace(stringOrDefault(deterministic.Payload["case_id"], ""))
		if caseID == "" {
			caseID = strings.TrimSpace(stringOrDefault(getCaseValue(r.state, "case_id"), ""))
		}
		candidateCount := intFromAny(deterministic.Payload["candidate_count"])
		if candidateCount <= 0 {
			candidateCount = intFromAny(getCaseValue(r.state, "jury_configuration.juror_count"))
		}
		if candidateCount <= 0 {
			candidateCount = 10
		}
		needed := candidateCount - countCandidateJurors(r.state)
		if needed < 0 {
			needed = 0
		}
		nextNumber := nextJurorNumber(r.state)
		step := 0
		for i := 0; i < needed; i++ {
			jurorNumber := nextNumber + i
			step++
			payload := map[string]any{
				"case_id":  caseID,
				"juror_id": fmt.Sprintf("J%d", jurorNumber),
				"name":     fmt.Sprintf("Juror %d", jurorNumber),
			}
			execRes, err := r.executeAction(turnIndex, step, roleName, "add_juror", payload)
			if err != nil {
				return step, err
			}
			res := execRes.Result
			*transcript = append(*transcript, map[string]any{"action": "add_juror", "arguments": payload, "result": res})
			if ok, _ := res["ok"].(bool); !ok {
				return step, nil
			}
		}
		return step, nil
	}
	if kind == "random_empanel_jury" {
		caseID := strings.TrimSpace(stringOrDefault(deterministic.Payload["case_id"], ""))
		if caseID == "" {
			caseID = strings.TrimSpace(stringOrDefault(getCaseValue(r.state, "case_id"), ""))
		}
		jurorCount := intFromAny(deterministic.Payload["juror_count"])
		if jurorCount <= 0 {
			jurorCount = intFromAny(getCaseValue(r.state, "jury_configuration.juror_count"))
		}
		if jurorCount <= 0 {
			return 0, fmt.Errorf("random empanelment requires positive juror_count")
		}
		selected, err := chooseRandomCandidateJurorIDs(rand.New(rand.NewSource(time.Now().UnixNano())), candidateJurorIDs(r.state), jurorCount)
		if err != nil {
			return 0, err
		}
		payload := map[string]any{
			"case_id":   caseID,
			"juror_ids": selected,
		}
		execRes, err := r.executeAction(turnIndex, 1, roleName, "empanel_jury", payload)
		if err != nil {
			return 0, err
		}
		res := execRes.Result
		*transcript = append(*transcript, map[string]any{"action": "empanel_jury", "arguments": payload, "result": res})
		return 1, nil
	}
	return 0, fmt.Errorf("unsupported deterministic kind: %s", kind)
}

func candidateJurorIDs(state map[string]any) []string {
	caseObj, _ := state["case"].(map[string]any)
	if caseObj == nil {
		return nil
	}
	jurors, _ := caseObj["jurors"].([]any)
	out := make([]string, 0, len(jurors))
	for _, raw := range jurors {
		juror, _ := raw.(map[string]any)
		if juror == nil {
			continue
		}
		if strings.TrimSpace(stringOrDefault(juror["status"], "")) != "candidate" {
			continue
		}
		jurorID := strings.TrimSpace(stringOrDefault(juror["juror_id"], ""))
		if jurorID == "" {
			continue
		}
		out = append(out, jurorID)
	}
	return out
}

func chooseRandomCandidateJurorIDs(rng *rand.Rand, ids []string, count int) ([]string, error) {
	if rng == nil {
		return nil, fmt.Errorf("random source is nil")
	}
	if count <= 0 {
		return nil, fmt.Errorf("juror count must be positive")
	}
	if len(ids) < count {
		return nil, fmt.Errorf("need %d candidate jurors, have %d", count, len(ids))
	}
	pool := append([]string(nil), ids...)
	rng.Shuffle(len(pool), func(i, j int) {
		pool[i], pool[j] = pool[j], pool[i]
	})
	selected := append([]string(nil), pool[:count]...)
	sort.Strings(selected)
	return selected, nil
}

func getCaseValue(state map[string]any, dotted string) any {
	caseObj, _ := state["case"].(map[string]any)
	if caseObj == nil {
		return nil
	}
	if dotted == "jury_configuration.juror_count" {
		cfg, _ := caseObj["jury_configuration"].(map[string]any)
		if cfg == nil {
			return nil
		}
		return cfg["juror_count"]
	}
	return caseObj[dotted]
}

type correctionIssue struct {
	Tool         string
	Error        string
	Code         string
	ActorMessage string
	Details      map[string]string
}

func issueFromResult(toolName string, result map[string]any) correctionIssue {
	details := map[string]string{}
	if raw, ok := result["details"].(map[string]any); ok {
		for key, value := range raw {
			valueText := strings.TrimSpace(fmt.Sprintf("%v", value))
			if valueText == "" {
				continue
			}
			details[key] = valueText
		}
	}
	return correctionIssue{
		Tool:         toolName,
		Error:        strings.TrimSpace(stringOrDefault(result["error"], "")),
		Code:         strings.TrimSpace(stringOrDefault(result["code"], "")),
		ActorMessage: strings.TrimSpace(stringOrDefault(result["actor_message"], "")),
		Details:      details,
	}
}

func (r *Runner) issueFromMalformedToolCall(call openaiapi.ToolCall) (correctionIssue, error) {
	toolName := strings.TrimSpace(call.Name)
	if toolName == "" {
		toolName = "tool"
	}
	msg := strings.TrimSpace(call.ArgumentsError)
	if msg == "" {
		msg = "invalid JSON arguments"
	}
	actorMessage, err := r.prompts.Text(adcprompts.RuntimeCorrectionMalformedID)
	if err != nil {
		return correctionIssue{}, err
	}
	return correctionIssue{
		Tool:         toolName,
		Error:        "malformed JSON arguments: " + msg,
		ActorMessage: actorMessage,
	}, nil
}

func malformedToolCallOutput(issue correctionIssue) map[string]any {
	return map[string]any{
		"ok":            false,
		"error":         issue.Error,
		"actor_message": issue.ActorMessage,
	}
}

func formatIssue(issue correctionIssue) string {
	if strings.TrimSpace(issue.ActorMessage) != "" {
		return strings.TrimSpace(issue.ActorMessage)
	}
	if issue.Error == "" {
		return fmt.Sprintf("%s failed", issue.Tool)
	}
	return fmt.Sprintf("%s failed: %s", issue.Tool, issue.Error)
}

func (r *Runner) buildCorrectionPrompt(role string, issues []correctionIssue, allowed []string, mayPass bool) (string, error) {
	issueLines := make([]string, 0, len(issues))
	requiredHints := map[string][]string{}
	for _, issue := range issues {
		issueLines = append(issueLines, formatIssue(issue))
		if req := requiredFieldsForTool(issue.Tool); len(req) > 0 {
			requiredHints[issue.Tool] = req
		}
	}
	for index := range issueLines {
		issueLines[index] = "- " + issueLines[index]
	}
	requiredLines := make([]string, 0, len(requiredHints))
	if len(requiredHints) > 0 {
		tools := make([]string, 0, len(requiredHints))
		for tool := range requiredHints {
			tools = append(tools, tool)
		}
		sort.Strings(tools)
		for _, tool := range tools {
			requiredLines = append(requiredLines, fmt.Sprintf("- %s: %s", tool, strings.Join(requiredHints[tool], ", ")))
		}
	}
	guidanceIDs := make([]string, 0, 3)
	if containsIssueCode(issues, "LOCAL_RULE_LIMIT_EXCEEDED") {
		guidanceIDs = append(guidanceIDs, adcprompts.RuntimeCorrectionLocalRuleID)
	}
	if containsIssueText(issues, "property not found") || containsIssueText(issues, "missing required") {
		guidanceIDs = append(guidanceIDs, adcprompts.RuntimeCorrectionFieldsID)
	}
	if containsIssueText(issues, "already") {
		guidanceIDs = append(guidanceIDs, adcprompts.RuntimeCorrectionCompleteID)
	}
	guidance := make([]string, 0, len(guidanceIDs))
	for _, id := range guidanceIDs {
		text, err := r.prompts.Text(id)
		if err != nil {
			return "", err
		}
		guidance = append(guidance, text)
	}
	return r.prompts.Render(adcprompts.RuntimeCorrectionID, map[string]string{
		"{{ROLE}}":            promptValue(role),
		"{{ISSUES}}":          promptSections(issueLines),
		"{{GUIDANCE}}":        promptSections(guidance),
		"{{REQUIRED_FIELDS}}": promptSections(requiredLines),
		"{{ALLOWED_ACTIONS}}": promptList(allowed),
		"{{PASS_ACTION}}":     passAction(mayPass),
	})
}

func containsIssueCode(issues []correctionIssue, code string) bool {
	target := strings.TrimSpace(code)
	for _, issue := range issues {
		if strings.TrimSpace(issue.Code) == target {
			return true
		}
	}
	return false
}

func containsIssueText(issues []correctionIssue, fragment string) bool {
	target := strings.ToLower(strings.TrimSpace(fragment))
	for _, issue := range issues {
		text := strings.ToLower(issue.Error)
		if strings.Contains(text, target) {
			return true
		}
	}
	return false
}

func requiredFieldsForTool(toolName string) []string {
	schema := toolSchema(toolName)
	if schema == nil {
		return nil
	}
	raw, ok := schema["required"].([]any)
	if !ok {
		return nil
	}
	fields := make([]string, 0, len(raw))
	for _, item := range raw {
		field := strings.TrimSpace(fmt.Sprintf("%v", item))
		if field == "" {
			continue
		}
		fields = append(fields, field)
	}
	if len(fields) == 0 {
		return nil
	}
	sort.Strings(fields)
	return fields
}

func hasSuccessfulResult(transcript []map[string]any) bool {
	for i := len(transcript) - 1; i >= 0; i-- {
		resultObj, ok := transcript[i]["result"].(map[string]any)
		if !ok {
			continue
		}
		if flag, _ := resultObj["ok"].(bool); flag {
			return true
		}
	}
	return false
}
