package proceeding

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jsmorph/adj/arb/runtime/lean"
	"github.com/jsmorph/adj/arb/runtime/spec"
	"github.com/jsmorph/adj/common/casemanifest"
)

const DefaultCaseID = "arb-1"

func runConfigured(ctx context.Context, cfg Config, complaint spec.Complaint) (result Result, err error) {
	cfg.CaseID = normalizeCaseID(cfg.CaseID)
	if cfg.OutputDir == "" {
		return Result{}, fmt.Errorf("output dir is required")
	}
	if cfg.ComplaintPath == "" {
		return Result{}, fmt.Errorf("complaint path is required")
	}
	if cfg.Engine.Command == nil {
		return Result{}, fmt.Errorf("lean engine command is required")
	}
	if err := ValidatePolicy(cfg.Policy); err != nil {
		return Result{}, err
	}
	if err := ValidateRuntimeLimits(cfg.Runtime); err != nil {
		return Result{}, err
	}
	if err := ValidateCouncilBackend(cfg.CouncilBackend); err != nil {
		return Result{}, err
	}
	cfg.CouncilBackend = NormalizeCouncilBackend(cfg.CouncilBackend)
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create out dir: %w", err)
	}
	startedAt := time.Now().UTC()
	manifest := casemanifest.New(casemanifest.ProcedureARB, cfg.CaseID, cfg.RunID, startedAt)
	if err := casemanifest.WriteAtomic(cfg.OutputDir, manifest); err != nil {
		return Result{}, fmt.Errorf("write case manifest: %w", err)
	}
	attorneys, err := attorneyRunInfos(cfg, cfg.ComplaintPath)
	if err != nil {
		return Result{}, err
	}
	attorneyMap := make(map[string]AttorneyRunInfo, len(attorneys))
	for _, attorney := range attorneys {
		attorneyMap[attorney.Role] = attorney
	}
	llmClient := newDirectCouncilClient(cfg.Runtime.CouncilRequestTimeout(), cfg.Runtime.CouncilRequestAttempts)
	defer func() {
		cost := llmClient.TotalCostUSD()
		result.CouncilCostUSD = cost
		if err != nil && cost > 0 {
			err = &councilCostError{costUSD: cost, err: err}
		}
	}()
	var caseFiles []CaseFile
	if len(cfg.CaseFilePaths) != 0 {
		caseFiles, err = loadCaseFilesFromPaths(cfg.CaseFilePaths)
		if err != nil {
			return Result{}, err
		}
	} else {
		caseDir := filepath.Dir(cfg.ComplaintPath)
		caseFiles, err = loadCaseFiles(caseDir)
		if err != nil {
			return Result{}, err
		}
	}
	evidenceStoreDir := filepath.Join(cfg.OutputDir, "evidence-store")
	council, councilReplacements, err := sampleAvailableCouncil(ctx, cfg, llmClient)
	if err != nil {
		return Result{}, err
	}
	initialState := initialState(cfg.Policy, cfg.CaseID)
	councilMembers := councilSeatMaps(council)
	certificateInit, err := newReplayInitializeRequest(initialState, complaint.Proposition, councilMembers)
	if err != nil {
		return Result{}, err
	}
	initResp, err := cfg.Engine.InitializeCase(initialState, complaint.Proposition, councilMembers)
	if err != nil {
		return Result{}, err
	}
	if ok, _ := initResp["ok"].(bool); !ok {
		return Result{}, fmt.Errorf("initialize_case rejected: %s", mapString(initResp["error"]))
	}
	rc := &runContext{
		cfg:               cfg,
		complaint:         complaint,
		state:             mapAny(initResp["state"]),
		certificateInit:   certificateInit,
		caseFiles:         caseFiles,
		submittedEvidence: []SubmittedEvidenceMeta{},
		evidenceByID:      map[string]EvidenceMeta{},
		evidenceStoreDir:  evidenceStoreDir,
		uploadSessions:    map[string]*EvidenceUploadSession{},
		council:           council,
		attorneys:         attorneyMap,
		workProductDirs:   map[string]string{},
	}
	if err := rc.initializeEvidenceRegistry(); err != nil {
		return Result{}, err
	}
	caseAPI, err := startCaseAPIServer(rc, cfg.CouncilBackend == councilBackendAPI)
	if err != nil {
		return Result{}, err
	}
	rc.lawyerAPI = caseAPI.lawyerAPI
	rc.councilAPI = caseAPI.councilAPI
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if closeErr := caseAPI.Close(shutdownCtx); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	manifest.CaseAPIBase = caseAPI.baseURL
	if err := casemanifest.WriteAtomic(cfg.OutputDir, manifest); err != nil {
		return Result{}, fmt.Errorf("write case manifest with case API address: %w", err)
	}
	if _, err := fmt.Fprintf(os.Stderr, "caseapi listening on %s\n", caseAPI.baseURL); err != nil {
		return Result{}, fmt.Errorf("write case API address: %w", err)
	}
	for _, replacement := range councilReplacements {
		if err := rc.recordEvent("council_member_replaced", "system", currentPhase(rc.state), map[string]any{
			"member_id":                    replacement.MemberID,
			"unavailable_model":            replacement.UnavailableModel,
			"unavailable_persona_filename": replacement.UnavailablePersonaFile,
			"replacement_model":            replacement.ReplacementModel,
			"replacement_persona_filename": replacement.ReplacementPersonaFile,
			"cause":                        replacement.Cause,
		}); err != nil {
			return Result{}, err
		}
	}
	if err := rc.recordEvent("run_initialized", "system", currentPhase(rc.state), map[string]any{
		"complaint":                      complaint,
		"evidence_standard":              cfg.Policy.EvidenceStandard,
		"council_backend":                cfg.CouncilBackend,
		"attorneys":                      attorneys,
		"council":                        council,
		"council_preflight_replacements": councilReplacements,
	}); err != nil {
		return Result{}, err
	}
	for {
		opportunity, terminal, reason, err := nextOpportunity(cfg.Engine, rc.state)
		if err != nil {
			return Result{}, err
		}
		if terminal {
			if rc.lawyerAPI != nil {
				rc.lawyerAPI.setTerminal(reason)
			}
			if rc.councilAPI != nil {
				rc.councilAPI.setTerminal(reason)
			}
			finishedAt := time.Now().UTC()
			caseObj := mapAny(rc.state["case"])
			status := "ok"
			var failure map[string]any
			errorMessage := ""
			if mapString(caseObj["status"]) == "failed" {
				status = "failed"
				failure = caseFailure(rc.state)
				errorMessage = caseFailureError(rc.state)
			}
			result := Result{
				CaseID:            cfg.CaseID,
				RunID:             cfg.RunID,
				StartedAt:         startedAt.Format(time.RFC3339),
				FinishedAt:        finishedAt.Format(time.RFC3339),
				Status:            status,
				Error:             errorMessage,
				ErrorClass:        rc.providerErrorClass,
				Failure:           failure,
				Phase:             currentPhase(rc.state),
				Resolution:        currentResolution(rc.state),
				Complaint:         complaint,
				EvidenceStandard:  currentEvidenceStandard(rc.state, cfg.Policy),
				CouncilBackend:    cfg.CouncilBackend,
				Attorneys:         attorneys,
				CaseFiles:         caseFileMetas(rc.caseFiles),
				SubmittedEvidence: rc.submittedEvidence,
				Evidence:          rc.listVisibleEvidence(),
				Council:           council,
				Events:            rc.events,
				FinalState:        rc.state,
				FinalReason:       reason,
				CouncilCostUSD:    llmClient.TotalCostUSD(),
			}
			if err := writeEvidence(cfg, result, rc); err != nil {
				return Result{}, err
			}
			return result, nil
		}
		rc.turn++
		switch opportunity.Role {
		case "plaintiff", "defendant":
			if err := rc.executeAttorneyOpportunity(ctx, llmClient, opportunity); err != nil {
				return Result{}, err
			}
		case "council":
			if err := rc.executeCouncilOpportunity(ctx, llmClient, opportunity); err != nil {
				return Result{}, err
			}
		default:
			return Result{}, fmt.Errorf("unsupported opportunity role %q", opportunity.Role)
		}
	}
}

func normalizeCaseID(caseID string) string {
	caseID = strings.TrimSpace(caseID)
	if caseID == "" {
		return DefaultCaseID
	}
	return caseID
}

func initialState(policy Policy, caseID string) map[string]any {
	return map[string]any{
		"schema_version": "v1",
		"forum_name":     "Agent Arbitration",
		"case": map[string]any{
			"case_id":            normalizeCaseID(caseID),
			"caption":            "Claimant v. Respondent",
			"proposition":        "",
			"status":             "draft",
			"phase":              "draft",
			"council_members":    []map[string]any{},
			"openings":           []map[string]any{},
			"arguments":          []map[string]any{},
			"rebuttals":          []map[string]any{},
			"surrebuttals":       []map[string]any{},
			"closings":           []map[string]any{},
			"offered_evidence":   []map[string]any{},
			"technical_reports":  []map[string]any{},
			"submitted_evidence": []map[string]any{},
			"deliberation_round": 1,
			"council_votes":      []map[string]any{},
			"resolution":         "",
		},
		"policy":        policy.StateMap(),
		"state_version": 0,
	}
}

func nextOpportunity(engine lean.Engine, state map[string]any) (Opportunity, bool, string, error) {
	resp, err := engine.NextOpportunity(state)
	if err != nil {
		return Opportunity{}, false, "", err
	}
	if ok, _ := resp["ok"].(bool); !ok {
		return Opportunity{}, false, "", fmt.Errorf("next_opportunity rejected: %s", mapString(resp["error"]))
	}
	if terminal, _ := resp["terminal"].(bool); terminal {
		return Opportunity{}, true, mapString(resp["reason"]), nil
	}
	raw := mapAny(resp["opportunity"])
	if len(raw) == 0 {
		return Opportunity{}, false, "", fmt.Errorf("next_opportunity returned empty opportunity")
	}
	return Opportunity{
		ID:           mapString(raw["opportunity_id"]),
		Role:         mapString(raw["role"]),
		Phase:        mapString(raw["phase"]),
		MayPass:      raw["may_pass"] == true,
		Objective:    mapString(raw["objective"]),
		AllowedTools: stringList(raw["allowed_tools"]),
	}, false, "", nil
}

func currentPhase(state map[string]any) string {
	return mapString(mapAny(state["case"])["phase"])
}

func currentEvidenceStandard(state map[string]any, policy Policy) string {
	value := mapString(mapAny(state["policy"])["evidence_standard"])
	if value != "" {
		return value
	}
	return strings.TrimSpace(policy.EvidenceStandard)
}

func currentResolution(state map[string]any) string {
	return mapString(mapAny(state["case"])["resolution"])
}

func stringList(value any) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, raw := range v {
			s := mapString(raw)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
