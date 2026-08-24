package runner

import (
	"fmt"
	"strings"

	"github.com/agentcourt/adj/adc/runtime/courts"
	adcprompts "github.com/agentcourt/adj/adc/runtime/prompts"
	"github.com/agentcourt/adj/adc/runtime/spec"
)

type PromptRendererOptions struct {
	Court       courts.Profile
	PromptDir   string
	PromptFiles map[string]string
}

type PromptOpportunity struct {
	ActorMessage string
	Objective    string
	Phase        string
	AllowedTools []string
	Constraints  map[string]any
	MayPass      bool
}

type PromptRenderer struct {
	prompts            *adcprompts.Catalog
	schemaDescriptions map[string]map[string]string
	court              courts.Profile
	resolveCourt       func(string) (courts.Profile, error)
}

func NewPromptRenderer(opts PromptRendererOptions) (*PromptRenderer, error) {
	court := opts.Court
	if courtProfileProvided(court) {
		if err := court.Validate(); err != nil {
			return nil, err
		}
	}
	promptCatalog, err := adcprompts.Load(adcprompts.Options{
		PromptDir:   opts.PromptDir,
		PromptFiles: opts.PromptFiles,
	})
	if err != nil {
		return nil, err
	}
	schemaDescriptions, err := loadSchemaPropertyDescriptions(promptCatalog)
	if err != nil {
		return nil, err
	}
	return newPromptRenderer(promptCatalog, schemaDescriptions, court), nil
}

func courtProfileProvided(court courts.Profile) bool {
	return strings.TrimSpace(court.Name) != "" ||
		strings.TrimSpace(court.RulesMarkdown) != "" ||
		strings.TrimSpace(court.RulesMarkdownFile) != "" ||
		court.JurisdictionScreen ||
		len(court.AllowedJurisdictionBases) > 0 ||
		strings.TrimSpace(court.PreferredJurisdictionBasis) != "" ||
		court.RequireJurisdictionStatement ||
		court.RequireDiversityCitizenship ||
		court.RequireAmountInControversy ||
		court.MinimumAmountInControversy != 0
}

func (p *PromptRenderer) effectiveCourt() (courts.Profile, error) {
	if courtProfileProvided(p.court) {
		return p.court, nil
	}
	return p.resolveCourt("")
}

func newPromptRenderer(promptCatalog *adcprompts.Catalog, schemaDescriptions map[string]map[string]string, court courts.Profile) *PromptRenderer {
	return &PromptRenderer{
		prompts:            promptCatalog,
		schemaDescriptions: schemaDescriptions,
		court:              court,
		resolveCourt:       courts.Resolve,
	}
}

func PromptOpportunityFromMap(payload map[string]any) PromptOpportunity {
	return PromptOpportunity{
		ActorMessage: strings.TrimSpace(stringOrDefault(payload["actor_message"], "")),
		Objective:    strings.TrimSpace(stringOrDefault(payload["objective"], "")),
		Phase:        strings.TrimSpace(stringOrDefault(payload["phase"], "")),
		AllowedTools: promptStringSlice(payload["allowed_tools"]),
		Constraints:  mapOrEmpty(payload["constraints"]),
		MayPass:      boolFromAny(payload["may_pass"]),
	}
}

func promptOpportunityFromLean(opportunity leanOpportunity) PromptOpportunity {
	return PromptOpportunity{
		ActorMessage: opportunity.ActorMessage,
		Objective:    opportunity.Objective,
		Phase:        opportunity.Phase,
		AllowedTools: append([]string(nil), opportunity.AllowedTools...),
		Constraints:  cloneJSONMap(opportunity.Constraints),
		MayPass:      opportunity.MayPass,
	}
}

func promptStringSlice(value any) []string {
	var raw []any
	switch values := value.(type) {
	case []any:
		raw = values
	case []string:
		out := make([]string, 0, len(values))
		for _, value := range values {
			if value = strings.TrimSpace(value); value != "" {
				out = append(out, value)
			}
		}
		return out
	default:
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, value := range raw {
		text := strings.TrimSpace(stringOrDefault(value, ""))
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func (p *PromptRenderer) JudgeRole(allowedTools []string) (spec.RoleSpec, error) {
	if p == nil || p.prompts == nil {
		return spec.RoleSpec{}, fmt.Errorf("ADC prompt renderer is nil")
	}
	court, err := p.effectiveCourt()
	if err != nil {
		return spec.RoleSpec{}, err
	}
	instructions, err := p.prompts.Text(adcprompts.JudgeInstructionsID)
	if err != nil {
		return spec.RoleSpec{}, err
	}
	preamble, err := p.prompts.Render(adcprompts.JudgeRuntimeID, map[string]string{
		"{{COURT_RULES}}": strings.TrimSpace(court.RulesMarkdown),
	})
	if err != nil {
		return spec.RoleSpec{}, err
	}
	return spec.RoleSpec{
		Name:           "judge",
		Instructions:   instructions,
		PromptPreamble: preamble,
		AllowedTools:   append([]string(nil), allowedTools...),
	}, nil
}

func (p *PromptRenderer) RenderJurorProbeIdentity(persona string) (string, error) {
	if p == nil || p.prompts == nil {
		return "", fmt.Errorf("ADC prompt renderer is nil")
	}
	return p.prompts.Render(adcprompts.ProbeJurorIdentityID, map[string]string{
		"{{PERSONA}}": strings.TrimSpace(persona),
	})
}

func (p *PromptRenderer) JurorProbeToolCheck() (string, error) {
	if p == nil || p.prompts == nil {
		return "", fmt.Errorf("ADC prompt renderer is nil")
	}
	return p.prompts.Text(adcprompts.ProbeJurorToolCheckID)
}

func (p *PromptRenderer) RenderSystemPrompt(role spec.RoleSpec, view map[string]any) (string, error) {
	if p == nil || p.prompts == nil {
		return "", fmt.Errorf("ADC prompt renderer is nil")
	}
	return p.prompts.Render(adcprompts.RuntimeSystemID, map[string]string{
		"{{ROLE}}":            strings.TrimSpace(role.Name),
		"{{PREAMBLE}}":        promptValue(role.PromptPreamble),
		"{{INSTRUCTIONS}}":    promptValue(role.Instructions),
		"{{ALLOWED_ACTIONS}}": promptList(role.EffectiveAllowedActions()),
		"{{VIEW}}":            marshalString(view),
	})
}

func (p *PromptRenderer) RenderOpportunityPrompt(role spec.RoleSpec, opportunity PromptOpportunity) (string, error) {
	if p == nil || p.prompts == nil {
		return "", fmt.Errorf("ADC prompt renderer is nil")
	}
	return p.prompts.Render(adcprompts.RuntimeOpportunityID, map[string]string{
		"{{ACTOR_MESSAGE}}":   promptValue(opportunity.ActorMessage),
		"{{OBJECTIVE}}":       promptValue(opportunity.Objective),
		"{{PHASE}}":           promptValue(opportunity.Phase),
		"{{ALLOWED_ACTIONS}}": promptList(opportunity.AllowedTools),
		"{{REFERENCE_TOOLS}}": promptList(referenceToolsForRole(role)),
		"{{CONSTRAINTS}}":     promptJSON(opportunity.Constraints),
		"{{PASS_ACTION}}":     passAction(opportunity.MayPass),
	})
}

func (p *PromptRenderer) RenderTurnPrompt(roleName string, basePrompt string, allowedTools []string) (string, error) {
	if p == nil || p.prompts == nil {
		return "", fmt.Errorf("ADC prompt renderer is nil")
	}
	schemaLines, err := toolSchemaPromptLinesWithErrorResolver(allowedTools, p.toolSchema)
	if err != nil {
		return "", err
	}
	cards, err := p.collectToolCards(roleName, allowedTools)
	if err != nil {
		return "", err
	}
	return p.prompts.Render(adcprompts.RuntimeTurnID, map[string]string{
		"{{BASE_PROMPT}}":   promptValue(basePrompt),
		"{{TOOL_SCHEMAS}}":  promptSections(schemaLines),
		"{{TOOL_GUIDANCE}}": promptSections(cards),
	})
}

func (p *PromptRenderer) RenderOpportunityInput(role spec.RoleSpec, view map[string]any, opportunity PromptOpportunity) ([]map[string]any, error) {
	systemPrompt, err := p.RenderSystemPrompt(role, view)
	if err != nil {
		return nil, err
	}
	opportunityPrompt, err := p.RenderOpportunityPrompt(role, opportunity)
	if err != nil {
		return nil, err
	}
	turnPrompt, err := p.RenderTurnPrompt(role.Name, opportunityPrompt, opportunityCallableToolNames(role, opportunity))
	if err != nil {
		return nil, err
	}
	return []map[string]any{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": turnPrompt},
	}, nil
}

func opportunityCallableToolNames(role spec.RoleSpec, opportunity PromptOpportunity) []string {
	names := append([]string(nil), opportunity.AllowedTools...)
	for _, name := range referenceToolsForRole(role) {
		names = appendIfMissing(names, name)
	}
	if opportunity.MayPass {
		names = appendIfMissing(names, "pass_turn")
	}
	return names
}

func (p *PromptRenderer) OpportunityTools(role spec.RoleSpec, opportunity PromptOpportunity) ([]map[string]any, error) {
	return p.BuildTools(opportunityCallableToolNames(role, opportunity))
}

func (p *PromptRenderer) collectToolCards(roleName string, allowedTools []string) ([]string, error) {
	roleName = strings.TrimSpace(roleName)
	seen := map[string]bool{}
	cards := make([]string, 0, len(allowedTools))
	for _, toolName := range allowedTools {
		toolName = strings.TrimSpace(toolName)
		if toolName == "" || seen[toolName] {
			continue
		}
		seen[toolName] = true
		card, err := p.prompts.ToolCard(roleName, toolName)
		if err != nil {
			return nil, err
		}
		cards = append(cards, fmt.Sprintf("Tool `%s`:\n%s", toolName, card))
	}
	return cards, nil
}

func rule12Grounds(court courts.Profile) []string {
	if court.JurisdictionScreen {
		return []string{
			"lack_subject_matter_jurisdiction",
			"no_standing",
			"not_ripe",
			"moot",
			"failure_to_state_a_claim",
		}
	}
	return []string{
		"no_standing",
		"not_ripe",
		"moot",
		"failure_to_state_a_claim",
	}
}

func (p *PromptRenderer) toolSchema(name string) (map[string]any, error) {
	base := toolSchema(name)
	if base == nil {
		return nil, nil
	}
	schema := toolSchemaWithDescriptions(name, base, p.schemaDescriptions)
	switch name {
	case "file_rule12_motion", "decide_rule12_motion":
		court, err := p.effectiveCourt()
		if err != nil {
			return nil, err
		}
		properties, _ := schema["properties"].(map[string]any)
		ground, _ := properties["ground"].(map[string]any)
		if properties == nil || ground == nil {
			return schema, nil
		}
		grounds := rule12Grounds(court)
		enumVals := make([]any, 0, len(grounds))
		for _, groundName := range grounds {
			enumVals = append(enumVals, groundName)
		}
		ground["enum"] = enumVals
		properties["ground"] = ground
		schema["properties"] = properties
	}
	return schema, nil
}

func (p *PromptRenderer) BuildTools(allowed []string) ([]map[string]any, error) {
	if p == nil || p.prompts == nil {
		return nil, fmt.Errorf("ADC prompt renderer is nil")
	}
	tools := make([]map[string]any, 0, len(allowed))
	missing := make([]string, 0)
	for _, name := range allowed {
		params, err := p.toolSchema(name)
		if err != nil {
			return nil, err
		}
		if params == nil {
			missing = append(missing, name)
			continue
		}
		descriptionID, ok := adcprompts.DirectToolDescriptionID(name)
		if !ok {
			return nil, fmt.Errorf("missing direct tool description prompt for %q", name)
		}
		description, err := p.prompts.Text(descriptionID)
		if err != nil {
			return nil, err
		}
		tools = append(tools, map[string]any{
			"type":        "function",
			"name":        name,
			"description": description,
			"parameters":  params,
		})
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing tool schemas for actions: %s", strings.Join(missing, ", "))
	}
	return tools, nil
}
