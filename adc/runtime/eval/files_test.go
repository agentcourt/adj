package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	openaiapi "github.com/agentcourt/adj/common/openai"
)

func TestRequiredFixtureBooleanFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		field  string
		line   string
		loader func(string) error
	}{
		{
			name:  "voir dire",
			field: "expected_allowed",
			line:  `{"id":"jvd-test","tier":1,"question_family":"proper_bias_probe","case_theme":"theme","asked_by":"plaintiff","juror_id":"J1","question":"Can you be fair?","expected_allowed":true,"expected_reason_tags":["proper_bias_probe"],"severity":1}`,
			loader: func(path string) error {
				_, err := LoadJudgeVoirDireFixtures(path)
				return err
			},
		},
		{
			name:  "for cause",
			field: "expected_granted",
			line:  `{"id":"fc-test","tier":1,"issue_family":"follow_law","case_theme":"Burden refusal","challenged_by":"plaintiff","juror_id":"J1","voir_dire_record":"I would require proof beyond any doubt and could not use preponderance.","challenge_grounds":"Juror refuses to apply the civil burden.","expected_granted":true,"expected_reason_tags":["follow_law"],"severity":5}`,
			loader: func(path string) error {
				_, err := LoadJudgeForCauseFixtures(path)
				return err
			},
		},
		{
			name:  "rule 11",
			field: "expected_granted",
			line:  `{"id":"r11-test","tier":1,"issue_family":"frivolous_legal","case_theme":"Foreclosed claim.","movant":"defendant","target_party":"plaintiff","challenged_filing":"Complaint","filing_text":"Plaintiff alleges a claim barred by an unambiguous release attached to the complaint.","notice_text":"Defendant served notice identifying the release and demanding withdrawal.","notice_served_at":"2026-07-01","motion_filed_at":"2026-07-25","motion_text":"Defendant moves for Rule 11 sanctions after no correction.","opposition_text":"Plaintiff argues the release should be ignored.","expected_granted":true,"expected_sanction_type":"admonition","expected_reason_tags":["frivolous_legal","proportional_sanction"],"severity":5}`,
			loader: func(path string) error {
				_, err := LoadJudgeRule11Fixtures(path)
				return err
			},
		},
		{
			name:  "rule 37",
			field: "expected_granted",
			line:  `{"id":"r37-test","tier":1,"issue_family":"no_response","case_theme":"No interrogatory response.","movant":"plaintiff","target_party":"defendant","discovery_type":"interrogatories","set_index":0,"request_text":"Identify witnesses with knowledge of the delivery failure.","response_text":"No response served by the deadline.","motion_text":"Plaintiff moves to compel complete answers and requests $750 in fees.","opposition_text":"Defendant offers no justification for missing the deadline.","expected_granted":true,"expected_sanction_type":"fees","expected_sanction_amount":750,"expected_reason_tags":["no_response","fees"],"severity":5}`,
			loader: func(path string) error {
				_, err := LoadJudgeRule37Fixtures(path)
				return err
			},
		},
		{
			name:  "rule 60",
			field: "expected_granted",
			line:  `{"id":"r60-test","tier":1,"issue_family":"excusable_neglect","case_theme":"Default judgment after service routing error.","judgment_summary":"Default judgment entered for plaintiff.","motion_ground":"60b1_mistake","motion_text":"Defendant missed the answer deadline because service was routed to a closed mailbox and appeared promptly.","opposition_text":"Plaintiff argues the neglect was careless.","default_judgment":true,"expected_granted":true,"required_concepts":["excusable neglect","prompt action"],"expected_reason_tags":["mistake_excusable_neglect"],"severity":5}`,
			loader: func(path string) error {
				_, err := LoadJudgeRule60Fixtures(path)
				return err
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			missing := strings.Replace(test.line, `"`+test.field+`":true,`, "", 1)
			path := filepath.Join(t.TempDir(), "fixtures.jsonl")
			if err := os.WriteFile(path, []byte(missing+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			err := test.loader(path)
			if err == nil || !strings.Contains(err.Error(), "missing required boolean field "+test.field) {
				t.Fatalf("loader error = %v, want missing %s", err, test.field)
			}
		})
	}
}

func TestLoadJudgeVoirDireFixturesPreservesFalseBoolean(t *testing.T) {
	t.Parallel()

	line := `{"id":"jvd-false","tier":1,"question_family":"improper_commitment","case_theme":"theme","asked_by":"plaintiff","juror_id":"J1","question":"Would you always award damages?","expected_allowed":false,"expected_reason_tags":["improper_commitment"],"severity":1}`
	path := filepath.Join(t.TempDir(), "fixtures.jsonl")
	if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixtures, err := LoadJudgeVoirDireFixtures(path)
	if err != nil {
		t.Fatalf("LoadJudgeVoirDireFixtures error = %v", err)
	}
	if len(fixtures) != 1 {
		t.Fatalf("fixture count = %d, want 1", len(fixtures))
	}
	if fixtures[0].ExpectedAllowed {
		t.Fatal("expected_allowed = true, want false")
	}
}

func TestRequireJSONBooleanFieldsRejectsNonBoolean(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{`{"required":null}`, `{"required":0}`, `{"required":"false"}`} {
		if err := requireJSONBooleanFields([]byte(raw), "required"); err == nil || !strings.Contains(err.Error(), "must be a boolean") {
			t.Fatalf("requireJSONBooleanFields(%s) error = %v, want boolean type error", raw, err)
		}
	}
}

func TestRule58RejectsCandidateOpportunityPromptOutsideCounterfactualMode(t *testing.T) {
	t.Parallel()

	_, err := RunJudgeRule58(nil, JudgeRule58Options{
		FixturesPath:          "unused",
		OutputDir:             "unused",
		OpportunityPromptPath: "candidate.md",
	})
	if err == nil || !strings.Contains(err.Error(), "opportunity prompt file requires counterfactual model mode") {
		t.Fatalf("Run error = %v", err)
	}
}

func TestResponseJSONIncludesCompleteResponse(t *testing.T) {
	t.Parallel()

	response := openaiapi.Response{
		Text: "decision text",
		ToolCalls: []openaiapi.ToolCall{{
			CallID:         "call-1",
			Name:           "decide",
			Arguments:      map[string]any{"granted": true},
			RawArguments:   `{"granted":true}`,
			ArgumentsError: "",
		}},
		WebSearchCalls: []openaiapi.WebSearchCall{{
			ID:      "search-1",
			Status:  "completed",
			Action:  "search",
			Queries: []string{"authority"},
			URL:     "https://example.test/search",
			Pattern: "authority",
			Sources: []openaiapi.WebSearchSource{{Type: "url", URL: "https://example.test/source"}},
		}},
		URLCitations: []openaiapi.URLCitation{{
			URL:        "https://example.test/source",
			Title:      "Authority",
			StartIndex: 4,
			EndIndex:   13,
		}},
		ResponseID: "response-1",
		RawJSON:    `{"id":"response-1"}`,
		Usage: openaiapi.Usage{
			InputTokens:       100,
			CachedInputTokens: 20,
			OutputTokens:      30,
			ReasoningTokens:   10,
			TotalTokens:       130,
		},
		UsageKnown:                true,
		OpenRouterMetadata:        map[string]any{"provider": "test-provider"},
		OpenRouterGeneration:      map[string]any{"data": map[string]any{"total_cost": 1.25}},
		OpenRouterGenerationError: "generation warning",
		OpenRouterCostUSD:         1.25,
		OpenRouterCostKnown:       false,
	}
	got := responseJSON(response)
	wantKeys := []string{
		"text",
		"tool_calls",
		"web_search_calls",
		"url_citations",
		"response_id",
		"raw_json",
		"usage",
		"usage_known",
		"openrouter_metadata",
		"openrouter_generation",
		"openrouter_generation_error",
		"openrouter_cost_usd",
		"openrouter_cost_known",
	}
	if len(got) != len(wantKeys) {
		t.Fatalf("response keys = %#v, want %d keys", got, len(wantKeys))
	}
	for _, key := range wantKeys {
		if _, ok := got[key]; !ok {
			t.Fatalf("response omitted %q: %#v", key, got)
		}
	}
	usage := got["usage"].(map[string]any)
	if usage["input_tokens"] != int64(100) || usage["cached_input_tokens"] != int64(20) || usage["output_tokens"] != int64(30) || usage["reasoning_tokens"] != int64(10) || usage["total_tokens"] != int64(130) {
		t.Fatalf("usage = %#v", usage)
	}
	if got["usage_known"] != true || got["openrouter_cost_known"] != false || got["openrouter_cost_usd"] != 1.25 {
		t.Fatalf("known flags or cost = %#v", got)
	}
	searches := got["web_search_calls"].([]map[string]any)
	if len(searches) != 1 || searches[0]["id"] != "search-1" || searches[0]["queries"].([]string)[0] != "authority" {
		t.Fatalf("web search calls = %#v", searches)
	}
	sources := searches[0]["sources"].([]map[string]any)
	if len(sources) != 1 || sources[0]["url"] != "https://example.test/source" {
		t.Fatalf("web search sources = %#v", sources)
	}
	citations := got["url_citations"].([]map[string]any)
	if len(citations) != 1 || citations[0]["start_index"] != int64(4) || citations[0]["end_index"] != int64(13) {
		t.Fatalf("URL citations = %#v", citations)
	}
	if got["openrouter_metadata"].(map[string]any)["provider"] != "test-provider" {
		t.Fatalf("OpenRouter metadata = %#v", got["openrouter_metadata"])
	}
	generation := got["openrouter_generation"].(map[string]any)
	if generation["data"].(map[string]any)["total_cost"] != 1.25 || got["openrouter_generation_error"] != "generation warning" {
		t.Fatalf("OpenRouter generation = %#v, error = %#v", generation, got["openrouter_generation_error"])
	}
	toolCalls := got["tool_calls"].([]map[string]any)
	if len(toolCalls) != 1 || toolCalls[0]["call_id"] != "call-1" || toolCalls[0]["raw_arguments"] != `{"granted":true}` {
		t.Fatalf("tool calls = %#v", toolCalls)
	}
	if got["raw_json"] != `{"id":"response-1"}` {
		t.Fatalf("raw_json = %#v", got["raw_json"])
	}
}
