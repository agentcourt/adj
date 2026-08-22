package quick

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/jsmorph/adj/common/documents"
	"github.com/jsmorph/adj/common/modelinput"
	openaiapi "github.com/jsmorph/adj/common/openai"
)

func (r *runner) requestVote(ctx context.Context, member CouncilMember) (Vote, error) {
	if member.RequestSpec == nil {
		return Vote{}, fmt.Errorf("request specification is required")
	}
	input, err := r.councilInput(member)
	if err != nil {
		return Vote{}, err
	}
	tools := councilTools(r.toolPrompt("council_vote"))
	requestCtx, cancel := context.WithTimeout(ctx, r.cfg.CouncilTimeout)
	defer cancel()
	previousResponseID := ""
	invalidReasons := make([]string, 0, r.cfg.InvalidAttemptLimit)
	for attempt := 0; attempt < r.cfg.InvalidAttemptLimit; attempt++ {
		response, err := r.client.CreateResponseWithRequestSpec(requestCtx, *member.RequestSpec, input, tools, previousResponseID)
		if err != nil {
			return Vote{}, err
		}
		previousResponseID = response.ResponseID
		vote, err := parseVote(member, response, r.cfg.MaxResponseBytes)
		if err == nil {
			return vote, nil
		}
		invalidReasons = append(invalidReasons, err.Error())
		repair, renderErr := r.renderPrompt("council.repair", "{{ERROR}}", err.Error())
		if renderErr != nil {
			return Vote{}, renderErr
		}
		input = append(input, map[string]any{
			"role":    "user",
			"content": repair,
		})
	}
	return Vote{}, &openaiapi.ProviderError{
		Class: openaiapi.ProviderErrorProtocol,
		Err:   fmt.Errorf("invalid response limit reached: %s", strings.Join(invalidReasons, "; ")),
	}
}

func parseVote(member CouncilMember, response openaiapi.Response, maxResponseBytes int) (Vote, error) {
	responseSize := len([]byte(response.RawJSON))
	if responseSize == 0 {
		wire, err := json.Marshal(response)
		if err != nil {
			return Vote{}, fmt.Errorf("measure council response: %w", err)
		}
		responseSize = len(wire)
	}
	if responseSize > maxResponseBytes {
		return Vote{}, fmt.Errorf("response has %d bytes; limit is %d", responseSize, maxResponseBytes)
	}
	if len(response.ToolCalls) != 1 {
		return Vote{}, fmt.Errorf("response must contain one tool call")
	}
	call := response.ToolCalls[0]
	if call.ArgumentsError != "" {
		return Vote{}, fmt.Errorf("parse submit_council_vote arguments: %s", call.ArgumentsError)
	}
	if call.Name != "submit_council_vote" {
		return Vote{}, fmt.Errorf("tool must be submit_council_vote")
	}
	if len(call.Arguments) != 2 {
		return Vote{}, fmt.Errorf("submit_council_vote accepts only vote and rationale")
	}
	vote := strings.TrimSpace(stringValue(call.Arguments["vote"]))
	if vote != "demonstrated" && vote != "not_demonstrated" {
		return Vote{}, fmt.Errorf("vote must be demonstrated or not_demonstrated")
	}
	rationale := strings.TrimSpace(stringValue(call.Arguments["rationale"]))
	if rationale == "" {
		return Vote{}, fmt.Errorf("rationale must be non-empty")
	}
	return Vote{
		MemberID:              member.MemberID,
		Model:                 member.Model,
		PersonaFile:           member.PersonaFile,
		Vote:                  vote,
		Rationale:             rationale,
		ResponseID:            response.ResponseID,
		ProviderUsage:         response.TokenUsage(),
		ProviderCostUSD:       response.CostUSD(),
		ProviderMetadataError: response.OpenRouterGenerationError,
		SubmittedAt:           time.Now().UTC(),
	}, nil
}

func (r *runner) councilInput(member CouncilMember) ([]map[string]any, error) {
	r.mu.Lock()
	arguments := append([]Argument(nil), r.transcript.Arguments...)
	r.mu.Unlock()
	if len(arguments) != 2 {
		return nil, fmt.Errorf("council input requires two lawyer arguments")
	}
	instruction, err := r.councilPrompt(member)
	if err != nil {
		return nil, err
	}
	casePrompt, err := r.renderPrompt(
		"council.case",
		"{{PROPOSITION}}", r.cfg.Proposition,
		"{{EVIDENCE_STANDARD}}", r.cfg.EvidenceStandard,
		"{{PROPONENT_ARGUMENT}}", arguments[0].Text,
		"{{OPPONENT_ARGUMENT}}", arguments[1].Text,
	)
	if err != nil {
		return nil, err
	}
	content := []map[string]any{{"type": "input_text", "text": casePrompt + "\n"}}
	if len(r.documents.Files) == 0 {
		noDocuments, err := r.renderPrompt("council.no_documents")
		if err != nil {
			return nil, err
		}
		content = append(content, map[string]any{"type": "input_text", "text": noDocuments})
	} else {
		for _, document := range r.documents.Files {
			documentRoot := filepath.Join(r.cfg.OutputDir, "documents")
			raw, err := documents.ReadVerified(documentRoot, document)
			if err != nil {
				return nil, fmt.Errorf("read council document %s: %w", document.Path, err)
			}
			metadata, err := r.renderPrompt(
				"council.document",
				"{{DOCUMENT_PATH}}", document.Path,
				"{{MEDIA_TYPE}}", document.MediaType,
				"{{SIZE_BYTES}}", fmt.Sprintf("%d", document.SizeBytes),
				"{{SHA256}}", document.SHA256,
			)
			if err != nil {
				return nil, err
			}
			content = append(content, map[string]any{"type": "input_text", "text": metadata})
			item, err := modelinput.DocumentContentItem(document, raw)
			if err != nil {
				return nil, err
			}
			content = append(content, item)
		}
	}
	submit, err := r.renderPrompt("council.submit")
	if err != nil {
		return nil, err
	}
	content = append(content, map[string]any{
		"type": "input_text",
		"text": submit,
	})
	return []map[string]any{
		{"role": "system", "content": instruction},
		{"role": "user", "content_items": content},
	}, nil
}

func validateCouncilDocuments(root string, manifest documents.Manifest) error {
	for _, document := range manifest.Files {
		raw, err := documents.ReadVerified(root, document)
		if err != nil {
			return fmt.Errorf("read council document %s: %w", document.Path, err)
		}
		if _, err := modelinput.DocumentContentItem(document, raw); err != nil {
			return err
		}
	}
	return nil
}

func councilTools(description string) []map[string]any {
	return []map[string]any{
		{
			"type":        "function",
			"name":        "submit_council_vote",
			"description": description,
			"strict":      true,
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"vote":      map[string]any{"type": "string", "enum": []string{"demonstrated", "not_demonstrated"}},
					"rationale": map[string]any{"type": "string"},
				},
				"required":             []string{"vote", "rationale"},
				"additionalProperties": false,
			},
		},
	}
}
