package proceeding

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jsmorph/adj/arb/runtime/lean"
	"github.com/jsmorph/adj/arb/runtime/spec"
	"github.com/jsmorph/adj/common/casemanifest"
)

const (
	DefaultCaseID          = "arb-1"
	outputDirClaimFileName = ".aar-output-claim"
)

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
	cfg, err = cfg.preparePrompts()
	if err != nil {
		return Result{}, err
	}
	outputDirClaimPath, err := prepareOutputDir(cfg.OutputDir)
	if err != nil {
		return Result{}, err
	}
	startedAt := time.Now().UTC()
	manifest := casemanifest.New(casemanifest.ProcedureARB, cfg.CaseID, cfg.RunID, startedAt)
	if err := writeInitialCaseManifest(cfg.OutputDir, outputDirClaimPath, manifest); err != nil {
		return Result{}, err
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
		accounting := llmClient.Accounting()
		result.Provider = accounting
		if err != nil && accounting.RequestCount > 0 {
			err = &councilAccountingError{accounting: accounting, err: err}
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
		caseFiles, err = loadCaseFiles(caseDir, cfg.ComplaintPath)
		if err != nil {
			return Result{}, err
		}
	}
	evidenceStoreDir := filepath.Join(cfg.OutputDir, "evidence-store")
	rc := &runContext{
		cfg:               cfg,
		complaint:         complaint,
		caseFiles:         caseFiles,
		submittedEvidence: []SubmittedEvidenceMeta{},
		evidenceByID:      map[string]EvidenceMeta{},
		evidenceStoreDir:  evidenceStoreDir,
		uploadSessions:    map[string]*EvidenceUploadSession{},
		attorneys:         attorneyMap,
		workProductDirs:   map[string]string{},
	}
	if err := rc.initializeEvidenceRegistry(); err != nil {
		return Result{}, err
	}
	council, councilReplacements, err := sampleAvailableCouncil(ctx, cfg, llmClient)
	if err != nil {
		return Result{}, err
	}
	initialState := initialState(cfg.Policy, cfg.CaseID, rc.initialEvidenceCommitments())
	councilMembers, err := councilSeatMaps(council)
	if err != nil {
		return Result{}, fmt.Errorf("prepare council members: %w", err)
	}
	certificateInit, err := newReplayInitializeRequest(initialState, complaint.Proposition, councilMembers)
	if err != nil {
		return Result{}, err
	}
	initCtx, cancelInit := context.WithTimeout(ctx, cfg.Runtime.EngineCallTimeout())
	initResp, err := cfg.Engine.InitializeCase(initCtx, initialState, complaint.Proposition, councilMembers)
	cancelInit()
	if err != nil {
		return Result{}, err
	}
	if ok, _ := initResp["ok"].(bool); !ok {
		return Result{}, fmt.Errorf("initialize_case rejected: %s", mapString(initResp["error"]))
	}
	rc.state = mapAny(initResp["state"])
	rc.certificateInit = certificateInit
	rc.council = council
	caseAPI, err := startCaseAPIServer(ctx, rc, cfg.CouncilBackend == councilBackendAPI)
	if err != nil {
		return Result{}, err
	}
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
		rc.mu.Lock()
		eventErr := rc.recordEventLocked("council_member_replaced", "system", currentPhase(rc.state), map[string]any{
			"member_id":                    replacement.MemberID,
			"unavailable_model":            replacement.UnavailableModel,
			"unavailable_persona_filename": replacement.UnavailablePersonaFile,
			"replacement_model":            replacement.ReplacementModel,
			"replacement_persona_filename": replacement.ReplacementPersonaFile,
			"cause":                        replacement.Cause,
		})
		rc.mu.Unlock()
		if eventErr != nil {
			return Result{}, eventErr
		}
	}
	rc.mu.Lock()
	eventErr := rc.recordEventLocked("run_initialized", "system", currentPhase(rc.state), map[string]any{
		"complaint":                      complaint,
		"evidence_standard":              cfg.Policy.EvidenceStandard,
		"council_backend":                cfg.CouncilBackend,
		"attorneys":                      attorneys,
		"council":                        council,
		"council_preflight_replacements": councilReplacements,
	})
	rc.mu.Unlock()
	if eventErr != nil {
		return Result{}, eventErr
	}
	for {
		rc.mu.Lock()
		opportunity, terminal, reason, err := nextOpportunity(ctx, cfg.Engine, cfg.Runtime.EngineCallTimeout(), rc.state)
		rc.mu.Unlock()
		if err != nil {
			return Result{}, err
		}
		if terminal {
			rc.mu.Lock()
			rc.setRoleAPIsTerminalLocked(reason)
			finalState, snapshotErr := cloneMapJSON(rc.state)
			if snapshotErr != nil {
				rc.mu.Unlock()
				return Result{}, fmt.Errorf("clone final case state: %w", snapshotErr)
			}
			events, snapshotErr := cloneEvents(rc.events)
			if snapshotErr != nil {
				rc.mu.Unlock()
				return Result{}, snapshotErr
			}
			outputSnapshot, snapshotErr := rc.finalOutputSnapshotLocked()
			if snapshotErr != nil {
				rc.mu.Unlock()
				return Result{}, snapshotErr
			}
			finishedAt := time.Now().UTC()
			caseObj := mapAny(finalState["case"])
			status := "ok"
			var failure map[string]any
			errorMessage := ""
			if mapString(caseObj["status"]) == "failed" {
				status = "failed"
				failure = caseFailure(finalState)
				errorMessage = caseFailureError(finalState)
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
				Phase:             currentPhase(finalState),
				Resolution:        currentResolution(finalState),
				Complaint:         complaint,
				EvidenceStandard:  currentEvidenceStandard(finalState, cfg.Policy),
				CouncilBackend:    cfg.CouncilBackend,
				Attorneys:         append([]AttorneyRunInfo(nil), attorneys...),
				CaseFiles:         caseFileMetas(rc.caseFiles),
				SubmittedEvidence: append([]SubmittedEvidenceMeta(nil), rc.submittedEvidence...),
				Evidence:          rc.listVisibleEvidence(),
				Council:           append([]CouncilSeat(nil), council...),
				Events:            events,
				FinalState:        finalState,
				FinalReason:       reason,
				Provider:          llmClient.Accounting(),
			}
			rc.mu.Unlock()
			if err := writeEvidence(cfg, result, outputSnapshot); err != nil {
				return Result{}, err
			}
			return result, nil
		}
		rc.mu.Lock()
		rc.turn++
		rc.mu.Unlock()
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

func prepareOutputDir(path string) (string, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return "", fmt.Errorf("create output directory: %w", err)
		}
		info, err = os.Stat(path)
	}
	if err != nil {
		return "", fmt.Errorf("stat output directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("output path is not a directory: %s", path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("read output directory: %w", err)
	}
	if len(entries) != 0 {
		return "", fmt.Errorf("output directory is not empty: %s", path)
	}
	return claimOutputDir(path)
}

func claimOutputDir(path string) (string, error) {
	claimPath := filepath.Join(path, outputDirClaimFileName)
	claim, err := os.OpenFile(claimPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("claim output directory %s: %w", path, err)
	}
	if err := claim.Close(); err != nil {
		closeErr := fmt.Errorf("close output directory claim %s: %w", claimPath, err)
		return "", errors.Join(closeErr, removeOutputDirClaim(claimPath))
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		readErr := fmt.Errorf("read claimed output directory: %w", err)
		return "", errors.Join(readErr, removeOutputDirClaim(claimPath))
	}
	if len(entries) != 1 || entries[0].Name() != outputDirClaimFileName {
		nonemptyErr := fmt.Errorf("output directory is not empty: %s", path)
		return "", errors.Join(nonemptyErr, removeOutputDirClaim(claimPath))
	}
	return claimPath, nil
}

func writeInitialCaseManifest(outputDir, claimPath string, manifest casemanifest.Manifest) error {
	writeErr := casemanifest.WriteAtomic(outputDir, manifest)
	releaseErr := removeOutputDirClaim(claimPath)
	if writeErr != nil {
		return errors.Join(fmt.Errorf("write case manifest: %w", writeErr), releaseErr)
	}
	return releaseErr
}

func removeOutputDirClaim(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove output directory claim %s: %w", path, err)
	}
	return nil
}

func cloneEvents(events []Event) ([]Event, error) {
	cloned := make([]Event, len(events))
	for i, event := range events {
		payload, err := cloneMapJSON(event.Payload)
		if err != nil {
			return nil, fmt.Errorf("clone final event %d: %w", i, err)
		}
		event.Payload = payload
		cloned[i] = event
	}
	return cloned, nil
}

func normalizeCaseID(caseID string) string {
	caseID = strings.TrimSpace(caseID)
	if caseID == "" {
		return DefaultCaseID
	}
	return caseID
}

func initialState(policy Policy, caseID string, evidenceCatalog []EvidenceCommitment) map[string]any {
	evidenceCatalog = append([]EvidenceCommitment(nil), evidenceCatalog...)
	sort.Slice(evidenceCatalog, func(i, j int) bool { return evidenceCatalog[i].EvidenceID < evidenceCatalog[j].EvidenceID })
	if evidenceCatalog == nil {
		evidenceCatalog = []EvidenceCommitment{}
	}
	return map[string]any{
		"schema_version":   "v1",
		"forum_name":       "Agent Arbitration",
		"evidence_catalog": evidenceCatalog,
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

func nextOpportunity(ctx context.Context, engine lean.Engine, engineTimeout time.Duration, state map[string]any) (Opportunity, bool, string, error) {
	if engineTimeout <= 0 {
		return Opportunity{}, false, "", fmt.Errorf("engine timeout must be positive")
	}
	callCtx, cancel := context.WithTimeout(ctx, engineTimeout)
	resp, err := engine.NextOpportunity(callCtx, state)
	cancel()
	if err != nil {
		return Opportunity{}, false, "", err
	}
	if ok, _ := resp["ok"].(bool); !ok {
		return Opportunity{}, false, "", fmt.Errorf("next_opportunity rejected: %s", mapString(resp["error"]))
	}
	stateVersion, err := requiredStateVersion(resp)
	if err != nil {
		return Opportunity{}, false, "", fmt.Errorf("next_opportunity: %w", err)
	}
	if terminal, _ := resp["terminal"].(bool); terminal {
		return Opportunity{}, true, mapString(resp["reason"]), nil
	}
	raw := mapAny(resp["opportunity"])
	if len(raw) == 0 {
		return Opportunity{}, false, "", fmt.Errorf("next_opportunity returned empty opportunity")
	}
	opportunity := Opportunity{
		ID:           mapString(raw["opportunity_id"]),
		StateVersion: stateVersion,
		Role:         mapString(raw["role"]),
		Phase:        mapString(raw["phase"]),
		MemberID:     mapString(raw["member_id"]),
		MayPass:      raw["may_pass"] == true,
		Objective:    mapString(raw["objective"]),
		AllowedTools: stringList(raw["allowed_tools"]),
	}
	if err := validateOpportunityAuthority(OpportunityAuthority{
		OpportunityID:        opportunity.ID,
		ExpectedStateVersion: opportunity.StateVersion,
		Role:                 opportunity.Role,
		Phase:                opportunity.Phase,
		MemberID:             opportunity.MemberID,
	}); err != nil {
		return Opportunity{}, false, "", fmt.Errorf("next_opportunity returned invalid authority: %w", err)
	}
	return opportunity, false, "", nil
}

func requiredStateVersion(values map[string]any) (int, error) {
	version, err := requiredIntParam(values, "state_version")
	if err != nil {
		return 0, err
	}
	if version < 0 {
		return 0, fmt.Errorf("state_version must be nonnegative")
	}
	return version, nil
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
