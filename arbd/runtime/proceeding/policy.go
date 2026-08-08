package proceeding

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const DefaultCouncilBackend = "direct"

func DefaultPolicy() Policy {
	return Policy{
		CouncilSize:                        5,
		JudgmentStandard:                   "Answer with one integer from 0 through 100. Base the answer on the record. Explain the score briefly.",
		MaxOpeningChars:                    5000,
		MaxArgumentChars:                   6000,
		MaxRebuttalChars:                   4000,
		MaxSurrebuttalChars:                4000,
		MaxClosingChars:                    5000,
		MaxExhibitsPerFiling:               9,
		MaxExhibitsPerSide:                 12,
		MaxExhibitBytes:                    64 * 1024 * 1024,
		MaxReportsPerFiling:                3,
		MaxReportsPerSide:                  4,
		MaxReportTitleBytes:                256,
		MaxReportSummaryBytes:              8192,
		MaxSubmittedEvidencePerSide:        8,
		MaxSubmittedEvidenceBytes:          64 * 1024 * 1024,
		MaxDirectSubmittedEvidenceBytes:    128 * 1024,
		MaxEvidenceUploadBytes:             64 * 1024 * 1024,
		MaxEvidenceChunkBytes:              64 * 1024,
		MaxEvidenceReadBytes:               64 * 1024,
		MaxEvidenceReadsPerOpportunity:     32,
		MaxEvidenceReadBytesPerOpportunity: 512 * 1024,
	}
}

func DefaultRuntimeLimits() RuntimeLimits {
	return RuntimeLimits{
		CouncilLLMTimeoutSeconds: 240,
		LawyerTurnTimeoutSeconds: 900,
		MaxResponseBytes:         128 * 1024,
		InvalidAttemptLimit:      3,
		CouncilMaxOutputTokens:   4096,
	}
}

func LoadPolicyFile(path string) (Policy, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Policy{}, fmt.Errorf("read policy: %w", err)
	}
	policy := DefaultPolicy()
	if err := json.Unmarshal(raw, &policy); err != nil {
		return Policy{}, fmt.Errorf("parse policy: %w", err)
	}
	if err := ValidatePolicy(policy); err != nil {
		return Policy{}, err
	}
	return policy, nil
}

func ValidatePolicy(policy Policy) error {
	switch {
	case policy.CouncilSize <= 0:
		return fmt.Errorf("policy.council_size must be positive")
	case strings.TrimSpace(policy.JudgmentStandard) == "":
		return fmt.Errorf("policy.judgment_standard is required")
	case policy.MaxOpeningChars <= 0:
		return fmt.Errorf("policy.max_opening_chars must be positive")
	case policy.MaxArgumentChars <= 0:
		return fmt.Errorf("policy.max_argument_chars must be positive")
	case policy.MaxRebuttalChars <= 0:
		return fmt.Errorf("policy.max_rebuttal_chars must be positive")
	case policy.MaxSurrebuttalChars <= 0:
		return fmt.Errorf("policy.max_surrebuttal_chars must be positive")
	case policy.MaxClosingChars <= 0:
		return fmt.Errorf("policy.max_closing_chars must be positive")
	case policy.MaxExhibitsPerFiling < 0:
		return fmt.Errorf("policy.max_exhibits_per_filing must be non-negative")
	case policy.MaxExhibitsPerSide < 0:
		return fmt.Errorf("policy.max_exhibits_per_side must be non-negative")
	case policy.MaxExhibitsPerFiling > policy.MaxExhibitsPerSide:
		return fmt.Errorf("policy.max_exhibits_per_filing %d exceeds max_exhibits_per_side %d", policy.MaxExhibitsPerFiling, policy.MaxExhibitsPerSide)
	case policy.MaxExhibitBytes <= 0:
		return fmt.Errorf("policy.max_exhibit_bytes must be positive")
	case policy.MaxReportsPerFiling < 0:
		return fmt.Errorf("policy.max_reports_per_filing must be non-negative")
	case policy.MaxReportsPerSide < 0:
		return fmt.Errorf("policy.max_reports_per_side must be non-negative")
	case policy.MaxReportsPerFiling > policy.MaxReportsPerSide:
		return fmt.Errorf("policy.max_reports_per_filing %d exceeds max_reports_per_side %d", policy.MaxReportsPerFiling, policy.MaxReportsPerSide)
	case policy.MaxReportTitleBytes <= 0:
		return fmt.Errorf("policy.max_report_title_bytes must be positive")
	case policy.MaxReportSummaryBytes <= 0:
		return fmt.Errorf("policy.max_report_summary_bytes must be positive")
	case policy.MaxSubmittedEvidencePerSide < 0:
		return fmt.Errorf("policy.max_submitted_evidence_per_side must be non-negative")
	case policy.MaxSubmittedEvidenceBytes <= 0:
		return fmt.Errorf("policy.max_submitted_evidence_bytes must be positive")
	case policy.MaxDirectSubmittedEvidenceBytes <= 0:
		return fmt.Errorf("policy.max_direct_submitted_evidence_bytes must be positive")
	case policy.MaxDirectSubmittedEvidenceBytes > policy.MaxSubmittedEvidenceBytes:
		return fmt.Errorf("policy.max_direct_submitted_evidence_bytes %d exceeds max_submitted_evidence_bytes %d", policy.MaxDirectSubmittedEvidenceBytes, policy.MaxSubmittedEvidenceBytes)
	case policy.MaxEvidenceUploadBytes <= 0:
		return fmt.Errorf("policy.max_evidence_upload_bytes must be positive")
	case policy.MaxEvidenceChunkBytes <= 0:
		return fmt.Errorf("policy.max_evidence_chunk_bytes must be positive")
	case policy.MaxEvidenceUploadBytes > policy.MaxSubmittedEvidenceBytes:
		return fmt.Errorf("policy.max_evidence_upload_bytes %d exceeds max_submitted_evidence_bytes %d", policy.MaxEvidenceUploadBytes, policy.MaxSubmittedEvidenceBytes)
	case policy.MaxEvidenceChunkBytes > policy.MaxEvidenceUploadBytes:
		return fmt.Errorf("policy.max_evidence_chunk_bytes %d exceeds max_evidence_upload_bytes %d", policy.MaxEvidenceChunkBytes, policy.MaxEvidenceUploadBytes)
	case policy.MaxEvidenceReadBytes <= 0:
		return fmt.Errorf("policy.max_evidence_read_bytes must be positive")
	case policy.MaxEvidenceReadsPerOpportunity <= 0:
		return fmt.Errorf("policy.max_evidence_reads_per_opportunity must be positive")
	case policy.MaxEvidenceReadBytesPerOpportunity <= 0:
		return fmt.Errorf("policy.max_evidence_read_bytes_per_opportunity must be positive")
	case policy.MaxEvidenceReadBytes > policy.MaxEvidenceReadBytesPerOpportunity:
		return fmt.Errorf("policy.max_evidence_read_bytes %d exceeds max_evidence_read_bytes_per_opportunity %d", policy.MaxEvidenceReadBytes, policy.MaxEvidenceReadBytesPerOpportunity)
	default:
		return nil
	}
}

func ValidateRuntimeLimits(limits RuntimeLimits) error {
	switch {
	case limits.CouncilLLMTimeoutSeconds <= 0:
		return fmt.Errorf("runtime.council_llm_timeout_seconds must be positive")
	case limits.LawyerTurnTimeoutSeconds <= 0:
		return fmt.Errorf("runtime.lawyer_turn_timeout_seconds must be positive")
	case limits.MaxResponseBytes <= 0:
		return fmt.Errorf("runtime.max_response_bytes must be positive")
	case limits.InvalidAttemptLimit <= 0:
		return fmt.Errorf("runtime.invalid_attempt_limit must be positive")
	case limits.CouncilMaxOutputTokens <= 0:
		return fmt.Errorf("runtime.council_max_output_tokens must be positive")
	default:
		return nil
	}
}

func ValidateCouncilBackend(backend string) error {
	switch strings.TrimSpace(strings.ToLower(backend)) {
	case "", DefaultCouncilBackend, councilBackendAPI:
		return nil
	default:
		return fmt.Errorf("council backend must be direct or councilapi")
	}
}

func NormalizeCouncilBackend(backend string) string {
	backend = strings.TrimSpace(strings.ToLower(backend))
	if backend == "" {
		return DefaultCouncilBackend
	}
	return backend
}

func (limits RuntimeLimits) CouncilTimeout() time.Duration {
	return time.Duration(limits.CouncilLLMTimeoutSeconds) * time.Second
}

func (limits RuntimeLimits) CouncilRequestTimeout() time.Duration {
	total := limits.CouncilTimeout()
	if total <= 90*time.Second {
		return total
	}
	return 90 * time.Second
}

func (limits RuntimeLimits) LawyerTurnTimeout() time.Duration {
	return time.Duration(limits.LawyerTurnTimeoutSeconds) * time.Second
}

func (policy Policy) StateMap() map[string]any {
	return map[string]any{
		"council_size":                            policy.CouncilSize,
		"judgment_standard":                       strings.TrimSpace(policy.JudgmentStandard),
		"max_opening_chars":                       policy.MaxOpeningChars,
		"max_argument_chars":                      policy.MaxArgumentChars,
		"max_rebuttal_chars":                      policy.MaxRebuttalChars,
		"max_surrebuttal_chars":                   policy.MaxSurrebuttalChars,
		"max_closing_chars":                       policy.MaxClosingChars,
		"max_exhibits_per_filing":                 policy.MaxExhibitsPerFiling,
		"max_exhibits_per_side":                   policy.MaxExhibitsPerSide,
		"max_exhibit_bytes":                       policy.MaxExhibitBytes,
		"max_reports_per_filing":                  policy.MaxReportsPerFiling,
		"max_reports_per_side":                    policy.MaxReportsPerSide,
		"max_report_title_bytes":                  policy.MaxReportTitleBytes,
		"max_report_summary_bytes":                policy.MaxReportSummaryBytes,
		"max_submitted_evidence_per_side":         policy.MaxSubmittedEvidencePerSide,
		"max_submitted_evidence_bytes":            policy.MaxSubmittedEvidenceBytes,
		"max_direct_submitted_evidence_bytes":     policy.MaxDirectSubmittedEvidenceBytes,
		"max_evidence_upload_bytes":               policy.MaxEvidenceUploadBytes,
		"max_evidence_chunk_bytes":                policy.MaxEvidenceChunkBytes,
		"max_evidence_read_bytes":                 policy.MaxEvidenceReadBytes,
		"max_evidence_reads_per_opportunity":      policy.MaxEvidenceReadsPerOpportunity,
		"max_evidence_read_bytes_per_opportunity": policy.MaxEvidenceReadBytesPerOpportunity,
	}
}
