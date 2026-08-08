package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jsmorph/adj/common/modelrequest"
	"github.com/jsmorph/adj/common/openai"
)

func TestLoadJurorPersonaPoolAndSample(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	scenarioBase := filepath.Join(root, "cases", "ex2")
	if err := os.MkdirAll(filepath.Join(root, "etc", "personas", "persons"), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.MkdirAll(scenarioBase, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc", "personas", "persons", "j1.txt"), []byte("skeptical of screenshots"), 0o644); err != nil {
		t.Fatalf("WriteFile j1 error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "etc", "personas", "persons", "j2.txt"), []byte("insists on corroboration"), 0o644); err != nil {
		t.Fatalf("WriteFile j2 error = %v", err)
	}
	poolPath := filepath.Join(root, "etc", "personas", "pool.jsonl")
	poolText := strings.Join([]string{
		`{"openrouter_model_id":"openai/gpt-5","provider":{"only":["openai"],"allow_fallbacks":false,"require_parameters":true},"request":{"temperature":0,"max_tokens":32},"persona":"persons/j1.txt"}`,
		`{"openrouter_model_id":"anthropic/claude-3.7-sonnet","provider":{"only":["anthropic"],"allow_fallbacks":false,"require_parameters":true},"request":{"temperature":0,"max_tokens":32},"persona":"persons/j2.txt"}`,
	}, "\n")
	if err := os.WriteFile(poolPath, []byte(poolText), 0o644); err != nil {
		t.Fatalf("WriteFile pool error = %v", err)
	}

	pool, err := loadJurorPersonaPool("etc/personas/pool.jsonl", scenarioBase)
	if err != nil {
		t.Fatalf("loadJurorPersonaPool error = %v", err)
	}
	if len(pool.pairs) != 2 || len(pool.remaining) != 2 {
		t.Fatalf("pool = %+v", pool)
	}
	for _, pair := range pool.pairs {
		if pair.RequestSpec == nil {
			t.Fatalf("missing request spec in %+v", pair)
		}
		if strings.TrimSpace(pair.PersonaText) == "" {
			t.Fatalf("empty persona text in %+v", pair)
		}
	}

	first, err := pool.samplePair()
	if err != nil {
		t.Fatalf("samplePair first error = %v", err)
	}
	second, err := pool.samplePair()
	if err != nil {
		t.Fatalf("samplePair second error = %v", err)
	}
	if first.PersonaFile == second.PersonaFile {
		t.Fatalf("samplePair repeated persona file: %q", first.PersonaFile)
	}
	if _, err := pool.samplePair(); err == nil {
		t.Fatalf("samplePair exhaustion error = nil")
	}
}

func TestApplyJurorPersonaDefaultsAndOpportunityContext(t *testing.T) {
	t.Parallel()

	pool := &jurorPersonaPool{
		pairs: []jurorPersonaPair{
			{Model: "openrouter://openai/gpt-5", PersonaText: "skeptical of screenshots", PersonaFile: "personas/persons/j1.txt", RequestSpec: &modelrequest.Spec{Endpoint: "openrouter", Model: "openai/gpt-5"}},
			{Model: "openrouter://anthropic/claude-3.7-sonnet", PersonaText: "insists on corroboration", PersonaFile: "personas/persons/j2.txt", RequestSpec: &modelrequest.Spec{Endpoint: "openrouter", Model: "anthropic/claude-3.7-sonnet"}},
		},
		remaining: []int{0, 1},
	}
	r := &Runner{
		jurorPersonaPool:        pool,
		jurorPersonaAssignments: map[string]jurorPersonaPair{},
	}

	payload, err := r.applyJurorPersonaDefaults(map[string]any{"juror_id": "J1"})
	if err != nil {
		t.Fatalf("applyJurorPersonaDefaults error = %v", err)
	}
	model := strings.TrimSpace(stringOrDefault(payload["model"], ""))
	personaFile := strings.TrimSpace(stringOrDefault(payload["persona_filename"], ""))
	if model == "" || personaFile == "" {
		t.Fatalf("payload = %+v", payload)
	}

	payloadAgain, err := r.applyJurorPersonaDefaults(map[string]any{"juror_id": "J1"})
	if err != nil {
		t.Fatalf("applyJurorPersonaDefaults second error = %v", err)
	}
	if payloadAgain["model"] != model || payloadAgain["persona_filename"] != personaFile {
		t.Fatalf("repeat payload = %+v, want same assignment", payloadAgain)
	}

	ctxModel, ctxPrompt := r.jurorOpportunityPromptContext(leanOpportunity{
		ActorMessage: "Juror J1 should answer.",
	})
	if ctxModel != model || !strings.Contains(ctxPrompt, "You are J1.") {
		t.Fatalf("jurorOpportunityPromptContext = (%q, %q)", ctxModel, ctxPrompt)
	}
	reqSpec := r.jurorOpportunityRequestSpec(leanOpportunity{ActorMessage: "Juror J1 should answer."})
	if reqSpec == nil || strings.TrimSpace(reqSpec.RuntimeModel()) == "" {
		t.Fatalf("jurorOpportunityRequestSpec = %+v", reqSpec)
	}
}

func TestJurorHelpers(t *testing.T) {
	t.Parallel()

	if got := targetJurorIDForOpportunity(leanOpportunity{
		Constraints:  map[string]any{"required_payload": map[string]any{"juror_id": "J7"}},
		ActorMessage: "Juror J3 should answer.",
	}); got != "J7" {
		t.Fatalf("targetJurorIDForOpportunity required_payload = %q", got)
	}
	if got := targetJurorIDForOpportunity(leanOpportunity{ActorMessage: "Juror J3 should answer."}); got != "J3" {
		t.Fatalf("targetJurorIDForOpportunity actor message = %q", got)
	}

	state := map[string]any{
		"case": map[string]any{
			"jurors": []any{
				map[string]any{"juror_id": "J2", "status": "candidate"},
				map[string]any{"juror_id": "J7", "status": "candidate"},
				map[string]any{"juror_id": "J8", "status": "sworn"},
			},
		},
	}
	if got := countCandidateJurors(state); got != 2 {
		t.Fatalf("countCandidateJurors = %d, want 2", got)
	}
	if got := nextJurorNumber(state); got != 9 {
		t.Fatalf("nextJurorNumber = %d, want 9", got)
	}
}

func TestJurorResponseClient(t *testing.T) {
	t.Parallel()

	baseClient := &openai.Client{}
	jurorClient := &openai.Client{}
	r := &Runner{client: baseClient, jurorClient: jurorClient}

	got, err := r.jurorResponseClient("")
	if err != nil || got != baseClient {
		t.Fatalf("jurorResponseClient empty = (%v, %v)", got, err)
	}
	got, err = r.jurorResponseClient("openrouter://openai/gpt-5")
	if err != nil || got != jurorClient {
		t.Fatalf("jurorResponseClient request spec model = (%v, %v)", got, err)
	}
	r = &Runner{client: baseClient}
	got, err = r.jurorResponseClient("openrouter://openai/gpt-5")
	if err != nil || got != baseClient {
		t.Fatalf("jurorResponseClient fallback = (%v, %v)", got, err)
	}
}
