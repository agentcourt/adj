package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jsmorph/adj/adc/runtime/runner"
)

func TestRenderJurorRoundsIncludesRoundSummaries(t *testing.T) {
	t.Parallel()

	caseObj := map[string]any{
		"jurors": []any{
			map[string]any{"juror_id": "J1", "status": "sworn", "model": "openai://gpt-5-mini", "persona_filename": "personas/persons/a.txt"},
			map[string]any{"juror_id": "J2", "status": "sworn", "model": "openai://gpt-5-mini", "persona_filename": "personas/persons/b.txt"},
		},
		"jury_configuration": map[string]any{"minimum_concurring": 2},
		"juror_votes": []any{
			map[string]any{"juror_id": "J1", "round": 1, "vote": "plaintiff", "damages": 10000.0, "confidence": "medium", "explanation": "first round"},
			map[string]any{"juror_id": "J2", "round": 1, "vote": "defendant", "damages": 0.0, "confidence": "medium", "explanation": "first round"},
			map[string]any{"juror_id": "J1", "round": 2, "vote": "plaintiff", "damages": 12000.0, "confidence": "medium", "explanation": "second round"},
			map[string]any{"juror_id": "J2", "round": 2, "vote": "plaintiff", "damages": 12000.0, "confidence": "medium", "explanation": "second round"},
		},
		"docket": []any{
			map[string]any{"title": "Jury supplemental instruction", "description": "Review the evidence and the instructions again."},
		},
		"jury_verdict": map[string]any{
			"verdict_for":       "plaintiff",
			"required_votes":    2,
			"votes_for_verdict": 2,
			"damages":           12000.0,
		},
	}

	rendered := renderJurorRounds(caseObj)
	want := []string{
		"### Round 1",
		"Summary: 1 for plaintiff, 1 for defendant, required votes 2. No verdict was reached in this round. Deliberation continued.",
		"### Round 2",
		"Supplemental instruction:",
		"Changes from round 1: 1 vote changes, 1 damages changes.",
		"This round produced a verdict for plaintiff with derived damages 12000.",
	}
	for _, needle := range want {
		if !strings.Contains(rendered, needle) {
			t.Fatalf("rendered digest missing %q\n%s", needle, rendered)
		}
	}
}

func TestRenderVoirDireSectionIncludesRulings(t *testing.T) {
	t.Parallel()

	caseObj := map[string]any{
		"jurors": []any{
			map[string]any{"juror_id": "J1", "status": "candidate", "model": "openai://gpt-5-mini", "persona_filename": "personas/persons/a.txt"},
		},
		"juror_questionnaire":           []any{},
		"juror_questionnaire_responses": []any{},
		"voir_dire_exchanges": []any{
			map[string]any{
				"juror_id":      "J1",
				"asked_by":      "plaintiff",
				"question":      "Could you follow the preponderance standard?",
				"judge_allowed": false,
				"ruling_reason": "This asks the juror to precommit on evidentiary sufficiency.",
				"response":      "",
			},
			map[string]any{
				"juror_id":      "J1",
				"asked_by":      "plaintiff",
				"question":      "Do you distrust authenticated digital business records as a category?",
				"judge_allowed": true,
				"ruling_reason": "This tests bias toward documentary evidence without arguing the merits.",
				"response":      "No.",
			},
		},
		"for_cause_challenges": []any{},
	}

	rendered := renderVoirDireSection(map[string]any{"policy": map[string]any{"skip_voir_dire": 0}}, caseObj, nil)
	want := []string{
		"| # | Juror ID | Asked by | Ruling | Question | Judge reason | Answer |",
		"| 1 | J1 | plaintiff | disallowed | Could you follow the preponderance standard? | This asks the juror to precommit on evidentiary sufficiency. | n/a |",
		"| 2 | J1 | plaintiff | allowed | Do you distrust authenticated digital business records as a category? | This tests bias toward documentary evidence without arguing the merits. | No. |",
	}
	for _, needle := range want {
		if !strings.Contains(rendered, needle) {
			t.Fatalf("rendered voir dire section missing %q\n%s", needle, rendered)
		}
	}
}

func TestRenderImportantAgentBashExecutionsIncludesOpenSSL(t *testing.T) {
	t.Parallel()

	turnLogs := []runner.TurnLog{
		{
			Role:             "plaintiff",
			OpportunityPhase: "pretrial",
			Transcript: []map[string]any{
				{
					"agent_tool_call": map[string]any{
						"tool_call_id": "call-1",
						"title":        "bash",
						"status":       "in_progress",
						"raw_input":    map[string]any{"cmd": "openssl dgst -sha256 -verify samantha_public.pem -signature confession.sig confession.txt"},
					},
				},
				{
					"agent_tool_update": map[string]any{
						"tool_call_id": "call-1",
						"status":       "completed",
						"raw_output": map[string]any{
							"details": map[string]any{
								"stdout":   "Verified OK\n",
								"exitCode": 0,
							},
						},
					},
				},
			},
		},
	}

	rendered := renderImportantAgentBashExecutions(turnLogs)
	want := []string{
		"| Turn | Role | Phase | Status | Command | Output |",
		"openssl dgst -sha256 -verify samantha_public.pem -signature confession.sig confession.txt",
		"Verified OK",
		"exit code: 0",
	}
	for _, needle := range want {
		if !strings.Contains(rendered, needle) {
			t.Fatalf("rendered bash section missing %q\n%s", needle, rendered)
		}
	}
}

func TestRenderImportantAgentBashExecutionsUsesUpdateRawInput(t *testing.T) {
	t.Parallel()

	turnLogs := []runner.TurnLog{
		{
			Role:             "plaintiff",
			OpportunityPhase: "pretrial",
			Transcript: []map[string]any{
				{
					"agent_tool_call": map[string]any{
						"tool_call_id": "call-1",
						"title":        "bash",
						"status":       "pending",
						"raw_input":    map[string]any{},
					},
				},
				{
					"agent_tool_update": map[string]any{
						"tool_call_id": "call-1",
						"status":       "in_progress",
						"raw_input":    map[string]any{"cmd": "openssl dgst -sha256 -verify samantha_public.pem -signature confession.sig confession.txt"},
					},
				},
				{
					"agent_tool_update": map[string]any{
						"tool_call_id": "call-1",
						"status":       "completed",
						"raw_output": map[string]any{
							"details": map[string]any{
								"stdout":   "Verified OK\n",
								"exitCode": 0,
							},
						},
					},
				},
			},
		},
	}

	rendered := renderImportantAgentBashExecutions(turnLogs)
	for _, needle := range []string{
		"openssl dgst -sha256 -verify samantha_public.pem -signature confession.sig confession.txt",
		"Verified OK",
	} {
		if !strings.Contains(rendered, needle) {
			t.Fatalf("rendered bash section missing %q\n%s", needle, rendered)
		}
	}
}

func TestRenderImportantAgentBashExecutionsTracksLongestStreamedCommand(t *testing.T) {
	t.Parallel()

	turnLogs := []runner.TurnLog{
		{
			Role:             "plaintiff",
			OpportunityPhase: "pretrial",
			Transcript: []map[string]any{
				{
					"agent_tool_call": map[string]any{
						"tool_call_id": "call-1",
						"title":        "bash",
						"status":       "pending",
						"raw_input":    map[string]any{},
					},
				},
				{
					"agent_tool_update": map[string]any{
						"tool_call_id": "call-1",
						"status":       "pending",
						"raw_input":    map[string]any{"command": "set"},
					},
				},
				{
					"agent_tool_update": map[string]any{
						"tool_call_id": "call-1",
						"status":       "pending",
						"raw_input":    map[string]any{"command": "set -euo pipefail\ncd /home/user/case\nbase64 -d confession.sig.b64 > confession.sig"},
					},
				},
				{
					"agent_tool_update": map[string]any{
						"tool_call_id": "call-1",
						"status":       "completed",
						"raw_input": map[string]any{
							"command": "set -euo pipefail\ncd /home/user/case\nbase64 -d confession.sig.b64 > confession.sig\nopenssl dgst -sha256 -verify samantha_public.pem -signature confession.sig confession.txt",
						},
						"raw_output": map[string]any{
							"details": map[string]any{
								"stdout":   "Verified OK\n",
								"exitCode": 0,
							},
						},
					},
				},
			},
		},
	}

	executions := collectAgentBashExecutions(turnLogs)
	if len(executions) != 1 {
		t.Fatalf("got %d executions, want 1", len(executions))
	}
	if got := executions[0].Command; !strings.Contains(got, "base64 -d confession.sig.b64 > confession.sig") || !strings.Contains(got, "openssl dgst -sha256 -verify samantha_public.pem -signature confession.sig confession.txt") {
		t.Fatalf("collector kept wrong command:\n%s", got)
	}
	if got := executions[0].Output; !strings.Contains(got, "Verified OK") {
		t.Fatalf("collector kept wrong output:\n%s", got)
	}
}

func TestRenderImportantAgentBashExecutionsReadsDirectTextOutputBlocks(t *testing.T) {
	t.Parallel()

	turnLogs := []runner.TurnLog{
		{
			Role:             "plaintiff",
			OpportunityPhase: "pretrial",
			Transcript: []map[string]any{
				{
					"agent_tool_call": map[string]any{
						"tool_call_id": "call-1",
						"title":        "bash",
						"status":       "pending",
						"raw_input":    map[string]any{"command": "openssl dgst -sha256 -verify samantha_public.pem -signature confession.sig confession.txt"},
					},
				},
				{
					"agent_tool_update": map[string]any{
						"tool_call_id": "call-1",
						"status":       "completed",
						"raw_output": map[string]any{
							"content": []any{
								map[string]any{"text": "Verified OK\nCommand exited with code 0", "type": "text"},
							},
							"details": map[string]any{},
						},
					},
				},
			},
		},
	}

	rendered := renderImportantAgentBashExecutions(turnLogs)
	for _, needle := range []string{
		"openssl dgst -sha256 -verify samantha_public.pem -signature confession.sig confession.txt",
		"Verified OK",
		"Command exited with code 0",
	} {
		if !strings.Contains(rendered, needle) {
			t.Fatalf("rendered bash section missing %q\n%s", needle, rendered)
		}
	}
}

func TestWriteTranscriptIncludesDeliberationAndJudgment(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "transcript.md")
	result := runner.Result{
		FinalState: map[string]any{
			"court_name": "United States District",
			"case": map[string]any{
				"caption":           "Peter v. Samantha",
				"status":            "judgment_entered",
				"monetary_judgment": 0.0,
				"docket": []any{
					map[string]any{
						"title":       "Technical report - plaintiff",
						"description": "Verified the detached signature with OpenSSL.",
					},
					map[string]any{
						"title":       "Jury instructions delivered",
						"description": "Plaintiff bears the burden to prove each element by a preponderance of the evidence.",
					},
				},
				"jury_configuration": map[string]any{"minimum_concurring": 2},
				"jurors": []any{
					map[string]any{"juror_id": "J1", "status": "sworn", "model": "openai://gpt-5-mini", "persona_filename": "personas/persons/a.txt"},
					map[string]any{"juror_id": "J2", "status": "sworn", "model": "openai://gpt-5-mini", "persona_filename": "personas/persons/b.txt"},
				},
				"juror_votes": []any{
					map[string]any{"juror_id": "J1", "round": 1, "vote": "defendant", "damages": 0.0, "confidence": "high", "explanation": "plaintiff did not prove assent"},
					map[string]any{"juror_id": "J2", "round": 1, "vote": "defendant", "damages": 0.0, "confidence": "high", "explanation": "plaintiff did not prove causation"},
				},
				"jury_verdict": map[string]any{
					"verdict_for":       "defendant",
					"required_votes":    2,
					"votes_for_verdict": 2,
					"damages":           0.0,
				},
			},
		},
	}

	if err := WriteTranscript(path, result); err != nil {
		t.Fatalf("WriteTranscript returned error: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	text := string(raw)
	for _, needle := range []string{
		"### Technical report - plaintiff",
		"### Jury instructions delivered",
		"## Deliberation",
		"### Round 1",
		"## Judgment",
		"| Verdict | defendant |",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("transcript missing %q\n%s", needle, text)
		}
	}
}

func TestShortenPreservesUTF8(t *testing.T) {
	t.Parallel()

	got := shorten("Samantha’s account of the reading-before-drafting condition", 12)
	if !utf8.ValidString(got) {
		t.Fatalf("shorten returned invalid UTF-8: %q", got)
	}
	if got != "Samantha’..." {
		t.Fatalf("got %q, want %q", got, "Samantha’...")
	}
}

func TestShortenAtWordPreservesUTF8(t *testing.T) {
	t.Parallel()

	got := shortenAtWord("and Samantha’s-insistence-on-timing-and-order", 18)
	if !utf8.ValidString(got) {
		t.Fatalf("shortenAtWord returned invalid UTF-8: %q", got)
	}
	if got != "and Samantha’s-..." {
		t.Fatalf("got %q, want %q", got, "and Samantha’s-...")
	}
}
