package spec

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jsmorph/adj/adc/runtime/courts"
)

type ClaimSpec struct {
	ClaimID         string   `json:"claim_id"`
	Label           string   `json:"label"`
	LegalTheory     string   `json:"legal_theory"`
	StandardOfProof string   `json:"standard_of_proof"`
	BurdenHolder    string   `json:"burden_holder"`
	Elements        []string `json:"elements"`
	Defenses        []string `json:"defenses"`
	DamagesQuestion string   `json:"damages_question"`
}

type RoleSpec struct {
	Name               string   `json:"name"`
	Model              string   `json:"model,omitempty"`
	Temperature        *float64 `json:"temperature,omitempty"`
	Instructions       string   `json:"instructions"`
	PromptPreamble     string   `json:"prompt_preamble"`
	PromptPreambleFile string   `json:"prompt_preamble_file"`
	AllowedActions     []string `json:"allowed_actions"`
	AllowedTools       []string `json:"allowed_tools"`
}

type DeterministicAction struct {
	Kind       string         `json:"kind"`
	ActionType string         `json:"action_type"`
	Payload    map[string]any `json:"payload"`
}

type ComplaintAttachmentSpec struct {
	FileID         string `json:"file_id"`
	Label          string `json:"label"`
	OriginalName   string `json:"original_name"`
	StorageRelPath string `json:"storage_relpath"`
	Sha256         string `json:"sha256"`
	SizeBytes      int    `json:"size_bytes"`
}

type CaseInitializationSpec struct {
	ComplaintSummary         string                    `json:"complaint_summary"`
	FiledBy                  string                    `json:"filed_by"`
	JuryDemandedOn           string                    `json:"jury_demanded_on,omitempty"`
	JurisdictionBasis        string                    `json:"jurisdiction_basis,omitempty"`
	JurisdictionalStatement  string                    `json:"jurisdictional_statement,omitempty"`
	InjuryStatement          string                    `json:"injury_statement,omitempty"`
	CausationStatement       string                    `json:"causation_statement,omitempty"`
	RedressabilityStatement  string                    `json:"redressability_statement,omitempty"`
	RipenessStatement        string                    `json:"ripeness_statement,omitempty"`
	LiveControversyStatement string                    `json:"live_controversy_statement,omitempty"`
	PlaintiffCitizenship     string                    `json:"plaintiff_citizenship,omitempty"`
	DefendantCitizenship     string                    `json:"defendant_citizenship,omitempty"`
	AmountInControversy      string                    `json:"amount_in_controversy,omitempty"`
	Attachments              []ComplaintAttachmentSpec `json:"attachments,omitempty"`
}

type TurnSpec struct {
	Role                string               `json:"role"`
	Prompt              string               `json:"prompt"`
	MaxSteps            int                  `json:"max_steps"`
	AllowedActions      []string             `json:"allowed_actions"`
	AllowedTools        []string             `json:"allowed_tools"`
	DeterministicAction *DeterministicAction `json:"deterministic_action"`
	RequireSuccess      bool                 `json:"require_success"`
}

type AssertionSpec struct {
	Type        string  `json:"type"`
	CaseIndex   int     `json:"case_index"`
	Equals      string  `json:"equals"`
	OfferID     string  `json:"offer_id"`
	OfferIndex  *int    `json:"offer_index"`
	Truth       *bool   `json:"truth"`
	Count       *int    `json:"count"`
	MotionIndex *int    `json:"motion_index"`
	MinCount    int     `json:"min_count"`
	Action      string  `json:"action"`
	Citation    string  `json:"citation"`
	Votes       int     `json:"votes"`
	Amount      float64 `json:"amount"`
}

type FormalScenario struct {
	Name         string                  `json:"name"`
	CourtName    string                  `json:"court_name"`
	Court        *courts.Profile         `json:"court,omitempty"`
	Model        string                  `json:"model"`
	Temperature  *float64                `json:"temperature"`
	InitialCase  map[string]any          `json:"initial_case"`
	InitialCases []map[string]any        `json:"initial_cases"`
	Claims       []ClaimSpec             `json:"claims"`
	Pleadings    map[string]any          `json:"pleadings"`
	Policy       map[string]any          `json:"policy"`
	CaseInit     *CaseInitializationSpec `json:"case_init,omitempty"`
	Roles        []RoleSpec              `json:"roles"`
	Turns        []TurnSpec              `json:"turns"`
	LoopPolicy   *LoopPolicySpec         `json:"loop_policy"`
	Assertions   []AssertionSpec         `json:"assertions"`
}

type LoopPolicySpec struct {
	Type             string `json:"type"`
	MaxStepsPerTurn  int    `json:"max_steps_per_turn"`
	MaxTurns         int    `json:"max_turns"`
	StopOnCaseStatus string `json:"stop_on_case_status"`
	StopCaseIndex    int    `json:"stop_case_index"`
}

func (r RoleSpec) EffectiveAllowedActions() []string {
	if len(r.AllowedActions) > 0 {
		return r.AllowedActions
	}
	return r.AllowedTools
}

func (t TurnSpec) EffectiveAllowedActions(role RoleSpec) []string {
	if len(t.AllowedActions) > 0 {
		return t.AllowedActions
	}
	if len(t.AllowedTools) > 0 {
		return t.AllowedTools
	}
	return role.EffectiveAllowedActions()
}

func Load(path string) (FormalScenario, error) {
	var out FormalScenario
	raw, err := os.ReadFile(path)
	if err != nil {
		return out, fmt.Errorf("read scenario: %w", err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("parse scenario: %w", err)
	}
	if out.Name == "" {
		return out, fmt.Errorf("scenario missing name")
	}
	if out.Court != nil {
		if err := out.Court.Validate(); err != nil {
			return out, err
		}
		if out.CourtName == "" {
			out.CourtName = out.Court.Name
		}
	}
	if out.CourtName == "" {
		return out, fmt.Errorf("scenario missing court_name")
	}
	if len(out.Roles) == 0 {
		return out, fmt.Errorf("scenario missing roles")
	}
	if len(out.Claims) > 1 {
		return out, fmt.Errorf("only one claim is supported in this initial Go runner")
	}
	for i := range out.Turns {
		if out.Turns[i].MaxSteps <= 0 {
			out.Turns[i].MaxSteps = 4
		}
	}
	if out.LoopPolicy != nil {
		if out.LoopPolicy.MaxStepsPerTurn <= 0 {
			out.LoopPolicy.MaxStepsPerTurn = 4
		}
		if out.LoopPolicy.MaxTurns <= 0 {
			out.LoopPolicy.MaxTurns = 40
		}
		if out.LoopPolicy.StopCaseIndex < 0 {
			out.LoopPolicy.StopCaseIndex = 0
		}
	}
	return out, nil
}
