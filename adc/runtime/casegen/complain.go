package casegen

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jsmorph/adj/adc/runtime/courts"
	adcprompts "github.com/jsmorph/adj/adc/runtime/prompts"
	"github.com/jsmorph/adj/common/openai"
)

const maxComplaintDraftAttempts = 3

var requiredComplaintHeadings = []string{
	"# Complaint",
	"## Parties",
	"## Jurisdiction",
	"## Facts",
	"## Claim",
	"## Relief Requested",
}

type ComplaintDraftOptions struct {
	Temperature *float64
	PromptDir   string
	PromptFiles map[string]string
}

func buildComplaintDraftPrompt(source ComplaintInput, court courts.Profile) (string, error) {
	promptCatalog, err := adcprompts.Load(adcprompts.Options{})
	if err != nil {
		return "", err
	}
	return buildComplaintDraftPromptWithCatalog(promptCatalog, source, court)
}

func buildComplaintDraftPromptWithCatalog(promptCatalog *adcprompts.Catalog, source ComplaintInput, court courts.Profile) (string, error) {
	courtContext, err := renderCourtContext(court)
	if err != nil {
		return "", err
	}
	return promptCatalog.Render(adcprompts.ComplaintUserID, map[string]string{
		"{{COURT}}":        courtContext,
		"{{SOURCE}}":       source.Markdown,
		"{{LINKED_FILES}}": renderLinkedFileContext(source.LinkedFiles),
	})
}

func DraftComplaint(
	ctx context.Context,
	client *openai.Client,
	model string,
	source ComplaintInput,
	court courts.Profile,
	temperature *float64,
) (string, error) {
	return DraftComplaintWithOptions(ctx, client, model, source, court, ComplaintDraftOptions{Temperature: temperature})
}

func DraftComplaintWithOptions(
	ctx context.Context,
	client *openai.Client,
	model string,
	source ComplaintInput,
	court courts.Profile,
	opts ComplaintDraftOptions,
) (string, error) {
	if client == nil {
		return "", fmt.Errorf("complaint client is nil")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return "", fmt.Errorf("complaint model is required")
	}
	promptCatalog, err := adcprompts.Load(adcprompts.Options{PromptDir: opts.PromptDir, PromptFiles: opts.PromptFiles})
	if err != nil {
		return "", err
	}
	prompt, err := buildComplaintDraftPromptWithCatalog(promptCatalog, source, court)
	if err != nil {
		return "", err
	}
	systemPrompt, err := promptCatalog.Text(adcprompts.ComplaintSystemID)
	if err != nil {
		return "", err
	}
	baseMessages := []map[string]any{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": prompt},
	}
	messages := append([]map[string]any(nil), baseMessages...)
	for attempt := 1; attempt <= maxComplaintDraftAttempts; attempt++ {
		resp, err := client.CreateResponse(ctx, model, messages, nil, "", opts.Temperature)
		if err != nil {
			return "", fmt.Errorf("draft complaint: %w", err)
		}
		text := strings.TrimSpace(resp.Text)
		if text == "" {
			return "", fmt.Errorf("draft complaint: empty response")
		}
		if err := validateComplaintDraft(text, source, court); err == nil {
			return text + "\n", nil
		} else if attempt == maxComplaintDraftAttempts {
			return "", fmt.Errorf("draft complaint: %w", err)
		} else {
			correctionPrompt, renderErr := promptCatalog.Render(adcprompts.ComplaintRepairID, map[string]string{"{{ERROR}}": err.Error()})
			if renderErr != nil {
				return "", renderErr
			}
			messages = append(
				append([]map[string]any(nil), baseMessages...),
				map[string]any{"role": "assistant", "content": text},
				map[string]any{"role": "user", "content": correctionPrompt},
			)
		}
	}
	return "", fmt.Errorf("draft complaint: exhausted draft attempts")
}

func validateComplaintDraft(text string, source ComplaintInput, court courts.Profile) error {
	if strings.Contains(text, "```") {
		return fmt.Errorf("complaint draft must not use code fences")
	}
	for _, heading := range requiredComplaintHeadings {
		if !strings.Contains(text, heading) {
			return fmt.Errorf("complaint draft missing heading %q", heading)
		}
	}
	links, err := extractLinkedFiles(text, filepath.Dir(source.OriginalPath))
	if err != nil {
		return err
	}
	if err := validateComplaintDraftLinks(source.LinkedFiles, links); err != nil {
		return err
	}
	if err := validateComplaintDraftJurisdiction(text, source, court); err != nil {
		return err
	}
	return nil
}

func validateComplaintDraftLinks(source []LinkedFile, draft []LinkedFile) error {
	if len(source) == 0 {
		return nil
	}
	if len(draft) == 0 {
		return fmt.Errorf("complaint draft must cite linked files with exact markdown link syntax")
	}
	sourceSet := map[string]bool{}
	for _, file := range source {
		path := strings.TrimSpace(file.ReferencePath)
		if path == "" {
			continue
		}
		sourceSet[path] = true
	}
	draftSet := map[string]bool{}
	for _, file := range draft {
		path := strings.TrimSpace(file.ReferencePath)
		if path == "" {
			continue
		}
		draftSet[path] = true
		if !sourceSet[path] {
			return fmt.Errorf("complaint draft cited file not present in the situation references: %s", path)
		}
	}
	for path := range sourceSet {
		if !draftSet[path] {
			return fmt.Errorf("complaint draft omitted referenced file: %s", path)
		}
	}
	return nil
}

var citizenOfRE = regexp.MustCompile(`(?im)\b[A-Z][A-Za-z .'-]*\bis a citizen of\s+([A-Z][A-Za-z .'-]+)\b`)

var dollarAmountRE = regexp.MustCompile(`\$([0-9][0-9,]*(?:\.[0-9]{2})?)`)

func validateComplaintDraftJurisdiction(text string, source ComplaintInput, court courts.Profile) error {
	lower := strings.ToLower(text)
	if !court.JurisdictionScreen {
		if strings.Contains(lower, "lacks a basis for federal subject-matter jurisdiction") ||
			strings.Contains(lower, "lacks federal subject-matter jurisdiction") ||
			strings.Contains(lower, "no basis for federal subject-matter jurisdiction") {
			return fmt.Errorf("complaint draft used federal subject-matter jurisdiction screening in %s", court.Name)
		}
		preferred := strings.TrimSpace(court.PreferredJurisdictionBasis)
		preferredHuman := strings.ReplaceAll(preferred, "_", " ")
		if preferred != "" && !strings.Contains(lower, preferred) && !strings.Contains(lower, preferredHuman) {
			return fmt.Errorf("complaint draft must plead %s jurisdiction in %s", court.PreferredJurisdictionBasis, court.Name)
		}
		return nil
	}
	if !sourceSupportsDiversityJurisdiction(source.Markdown, court.MinimumAmountInControversy) {
		return nil
	}
	if strings.Contains(lower, "lacks a basis for federal subject-matter jurisdiction") ||
		strings.Contains(lower, "lacks federal subject-matter jurisdiction") ||
		strings.Contains(lower, "no basis for federal subject-matter jurisdiction") {
		return fmt.Errorf("complaint draft denied federal jurisdiction even though the situation alleges complete diversity and more than %s", formatWholeDollarAmount(court.MinimumAmountInControversy))
	}
	if !strings.Contains(lower, "diversity") {
		return fmt.Errorf("complaint draft must plead diversity jurisdiction when the situation alleges complete diversity and more than %s", formatWholeDollarAmount(court.MinimumAmountInControversy))
	}
	threshold := formatWholeDollarAmount(court.MinimumAmountInControversy)
	if !strings.Contains(lower, strings.ToLower(threshold)) && !strings.Contains(lower, strings.TrimPrefix(strings.ToLower(threshold), "$")) {
		return fmt.Errorf("complaint draft must state that the amount in controversy exceeds %s when the situation alleges diversity jurisdiction", threshold)
	}
	return nil
}

func sourceSupportsDiversityJurisdiction(markdown string, threshold int) bool {
	if !hasCompleteDiversityAllegation(markdown) {
		return false
	}
	return int(maxDollarAmount(markdown)) > threshold
}

func hasCompleteDiversityAllegation(markdown string) bool {
	matches := citizenOfRE.FindAllStringSubmatch(markdown, -1)
	if len(matches) < 2 {
		return false
	}
	states := map[string]bool{}
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		state := normalizeJurisdictionToken(match[1])
		if state == "" {
			continue
		}
		states[state] = true
	}
	return len(states) >= 2
}

func maxDollarAmount(markdown string) float64 {
	matches := dollarAmountRE.FindAllStringSubmatch(markdown, -1)
	max := 0.0
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		raw := strings.ReplaceAll(match[1], ",", "")
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			continue
		}
		if value > max {
			max = value
		}
	}
	return max
}

func normalizeJurisdictionToken(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}
