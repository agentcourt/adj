package quick

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agentcourt/adj/common/casemanifest"
	"github.com/agentcourt/adj/common/councilsample"
	"github.com/agentcourt/adj/common/documents"
	"github.com/agentcourt/adj/common/modelgateway"
	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
	"github.com/agentcourt/adj/common/persona"
	"github.com/agentcourt/adj/common/recordio"
)

type runner struct {
	cfg       Config
	records   records
	documents documents.Manifest
	council   []CouncilMember
	client    responseClient
	startedAt time.Time

	mu          sync.Mutex
	cond        *sync.Cond
	version     uint64
	phase       string
	active      *lawyerTurn
	terminal    bool
	terminalErr error
	transcript  Transcript
	events      []Event
	sequence    int
}

func Run(ctx context.Context, opts Options) (Result, error) {
	cfg, err := configure(opts)
	if err != nil {
		return Result{}, err
	}
	client := newDirectClient(cfg.CouncilTimeout, cfg.CouncilRequestAttempts)
	return runConfigured(ctx, cfg, client)
}

func runConfigured(ctx context.Context, cfg Config, client responseClient) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil {
		return Result{}, fmt.Errorf("council response client is required")
	}
	if err := validateConfigured(cfg); err != nil {
		return Result{}, err
	}
	prompts, err := loadQuickPrompts(cfg)
	if err != nil {
		return Result{}, err
	}
	cfg.prompts = prompts
	if err := prepareOutputDir(cfg.OutputDir); err != nil {
		return Result{}, err
	}
	r := &runner{
		cfg:       cfg,
		records:   records{dir: cfg.OutputDir},
		client:    client,
		startedAt: time.Now().UTC(),
		phase:     "initializing",
	}
	r.cond = sync.NewCond(&r.mu)
	r.transcript = Transcript{
		SchemaVersion:   transcriptSchema,
		CaseID:          cfg.CaseID,
		Proposition:     cfg.Proposition,
		Arguments:       []Argument{},
		Votes:           []Vote{},
		CouncilFailures: []CouncilMemberFailure{},
	}
	manifest := casemanifest.New(Procedure, cfg.CaseID, cfg.RunID, r.startedAt)
	if err := casemanifest.WriteAtomic(cfg.OutputDir, manifest); err != nil {
		err = sanitizeError(err)
		return r.resultForError(err), err
	}
	if err := r.initializeFiles(); err != nil {
		return r.finishWithoutAPI(err)
	}
	if err := r.constituteCouncil(ctx); err != nil {
		return r.finishWithoutAPI(err)
	}
	initial := r.result("", nil)
	initial.Status = "running"
	initial.Phase = "initializing"
	if err := r.records.writeRun(initial); err != nil {
		return r.finishWithoutAPI(err)
	}

	api, err := startCaseAPI(r)
	if err != nil {
		return r.finishWithoutAPI(err)
	}
	manifest.CaseAPIBase = api.baseURL
	if err := casemanifest.WriteAtomic(cfg.OutputDir, manifest); err != nil {
		return r.finishWithAPI(api, err)
	}
	if err := r.records.writeRuntime(runtimeRecord{
		SchemaVersion: runtimeSchema,
		Procedure:     Procedure,
		CaseID:        cfg.CaseID,
		RunID:         cfg.RunID,
		CaseAPIBase:   api.baseURL,
		StartedAt:     r.startedAt,
	}); err != nil {
		return r.finishWithAPI(api, err)
	}
	if err := r.recordEvent("run_started", "system", map[string]any{"case_api_base": api.baseURL}); err != nil {
		return r.finishWithAPI(api, err)
	}

	runErr := sanitizeError(r.execute(ctx))
	r.setTerminal(runErr)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	closeErr := api.Close(shutdownCtx)
	cancel()
	runErr = sanitizeError(errors.Join(runErr, closeErr))
	runErr = r.recordTerminalEvent(runErr)
	r.setTerminal(runErr)
	result := r.result(api.baseURL, runErr)
	writeErr := r.records.writeRun(result)
	return result, errors.Join(runErr, writeErr)
}

func closeCaseAPI(api *caseAPI) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return api.Close(ctx)
}

func (r *runner) finishWithoutAPI(runErr error) (Result, error) {
	runErr = sanitizeError(runErr)
	r.setTerminal(runErr)
	runErr = r.recordTerminalEvent(runErr)
	r.setTerminal(runErr)
	result := r.result("", runErr)
	writeErr := r.records.writeRun(result)
	return result, errors.Join(runErr, writeErr)
}

func (r *runner) finishWithAPI(api *caseAPI, runErr error) (Result, error) {
	runErr = sanitizeError(runErr)
	r.setTerminal(runErr)
	closeErr := closeCaseAPI(api)
	runErr = sanitizeError(errors.Join(runErr, closeErr))
	runErr = r.recordTerminalEvent(runErr)
	r.setTerminal(runErr)
	result := r.result(api.baseURL, runErr)
	writeErr := r.records.writeRun(result)
	return result, errors.Join(runErr, writeErr)
}

func (r *runner) recordTerminalEvent(runErr error) error {
	if runErr != nil {
		eventErr := r.recordEvent("run_failed", "system", map[string]any{"error": runErr.Error()})
		return sanitizeError(errors.Join(runErr, eventErr))
	}
	if eventErr := r.recordEvent("run_completed", "system", map[string]any{"resolution": r.resolution()}); eventErr != nil {
		runErr = sanitizeError(eventErr)
		failureEventErr := r.recordEvent("run_failed", "system", map[string]any{"error": runErr.Error()})
		return sanitizeError(errors.Join(runErr, failureEventErr))
	}
	return nil
}

func validateConfigured(cfg Config) error {
	if strings.TrimSpace(cfg.Proposition) == "" || strings.TrimSpace(cfg.OutputDir) == "" || strings.TrimSpace(cfg.CouncilPoolPath) == "" {
		return fmt.Errorf("proposition, output directory, and council pool path are required")
	}
	if strings.TrimSpace(cfg.CaseID) == "" || strings.TrimSpace(cfg.RunID) == "" || strings.TrimSpace(cfg.CaseAPIAddr) == "" {
		return fmt.Errorf("case ID, run ID, and case API address are required")
	}
	if strings.TrimSpace(cfg.LawyerAPIBearerToken) == "" {
		return fmt.Errorf("lawyer API bearer token is required")
	}
	if cfg.CouncilSize <= 0 || cfg.RequiredVotes <= cfg.CouncilSize/2 || cfg.RequiredVotes > cfg.CouncilSize {
		return fmt.Errorf("invalid council size or required votes")
	}
	if strings.TrimSpace(cfg.EvidenceStandard) == "" {
		return fmt.Errorf("evidence standard is required")
	}
	if !cfg.AllowAPIKey {
		return fmt.Errorf("direct council provider calls require explicit API-key authorization")
	}
	if cfg.LawyerTimeout <= 0 || cfg.CouncilTimeout <= 0 {
		return fmt.Errorf("lawyer and council timeouts must be positive")
	}
	if cfg.MaxResponseBytes <= 0 || cfg.MaxArgumentChars <= 0 || cfg.InvalidAttemptLimit <= 0 {
		return fmt.Errorf("response, argument, and attempt limits must be positive")
	}
	if cfg.DocumentLimits.MaxFiles <= 0 || cfg.DocumentLimits.MaxFileBytes <= 0 || cfg.DocumentLimits.MaxTotalBytes <= 0 {
		return fmt.Errorf("document limits must be positive")
	}
	if cfg.DocumentLimits.MaxFileBytes > cfg.DocumentLimits.MaxTotalBytes {
		return fmt.Errorf("document file byte limit must not exceed total byte limit")
	}
	if cfg.CouncilRequestAttempts < 1 || cfg.CouncilRequestAttempts > 4 {
		return fmt.Errorf("council request attempts must be between 1 and 4")
	}
	return nil
}

func (r *runner) initializeFiles() error {
	if err := r.records.writeInput(r.cfg); err != nil {
		return err
	}
	if err := createEmptyEvents(filepath.Join(r.cfg.OutputDir, "events.ndjson")); err != nil {
		return err
	}
	destination := filepath.Join(r.cfg.OutputDir, "documents")
	if r.cfg.DocumentsDir == "" {
		if err := os.Mkdir(destination, 0o755); err != nil {
			return fmt.Errorf("create documents directory: %w", err)
		}
		r.documents = documents.Manifest{SchemaVersion: documents.SchemaVersion, Files: []documents.File{}}
	} else {
		manifest, err := documents.Import(r.cfg.DocumentsDir, destination, r.cfg.DocumentLimits)
		if err != nil {
			return err
		}
		r.documents = manifest
	}
	if err := validateCouncilDocuments(destination, r.documents); err != nil {
		return err
	}
	if err := recordio.WriteJSON(filepath.Join(r.cfg.OutputDir, "documents.json"), r.documents); err != nil {
		return err
	}
	if err := r.records.writeTranscript(r.transcript); err != nil {
		return err
	}
	return nil
}

type councilCandidate struct {
	spec  persona.Spec
	model modelrequest.ModelRef
}

type councilCandidateRejection struct {
	MemberID    string
	Unavailable CouncilMember
	Replacement *CouncilMember
	Cause       string
	ErrorClass  string
	err         error
}

func loadEligibleCouncilCandidates(path string) ([]councilCandidate, int, error) {
	specs, err := persona.LoadRecordsFile(path, filepath.Dir(path))
	if err != nil {
		return nil, 0, fmt.Errorf("load council pool: %w", sanitizeError(err))
	}
	candidates := make([]councilCandidate, 0, len(specs))
	incompatible := 0
	for index, spec := range specs {
		if spec.RequestSpec == nil {
			return nil, 0, fmt.Errorf("council pool record %d must be a JSON request specification", index+1)
		}
		modelRef, err := modelrequest.ParseModelRef(spec.Model)
		if err != nil {
			return nil, 0, fmt.Errorf("parse council pool record %d model: %w", index+1, sanitizeError(err))
		}
		if supported, known := spec.RequestSpec.SupportsParameter("tools"); known && !supported {
			incompatible++
			continue
		}
		candidates = append(candidates, councilCandidate{spec: spec, model: modelRef})
	}
	return candidates, incompatible, nil
}

func councilCandidates(path string) ([]CouncilMember, error) {
	candidates, _, err := loadEligibleCouncilCandidates(path)
	if err != nil {
		return nil, err
	}
	council := make([]CouncilMember, 0, len(candidates))
	for _, candidate := range candidates {
		council = append(council, councilMemberFromCandidate(candidate, ""))
	}
	return council, nil
}

func councilMemberFromCandidate(candidate councilCandidate, memberID string) CouncilMember {
	requestSpec := *candidate.spec.RequestSpec
	member := CouncilMember{
		MemberID:          memberID,
		Model:             candidate.model.Endpoint + "://" + candidate.model.Model,
		PersonaFile:       candidate.spec.File,
		EndpointVariantID: requestSpecMetadataString(requestSpec, "endpoint_variant_id"),
		ProviderName:      requestSpecMetadataString(requestSpec, "provider_name"),
		EndpointTag:       requestSpecMetadataString(requestSpec, "endpoint_tag"),
		Quantization:      requestSpecMetadataString(requestSpec, "quantization"),
		RequestSpec:       &requestSpec,
		PersonaText:       candidate.spec.Text,
	}
	if requestSpec.Provider != nil {
		member.ProviderOnly = append([]string(nil), requestSpec.Provider.Only...)
		member.ProviderQuantizations = append([]string(nil), requestSpec.Provider.Quantizations...)
		member.ProviderAllowFallbacks = copyBool(requestSpec.Provider.AllowFallbacks)
		member.ProviderRequireParameters = copyBool(requestSpec.Provider.RequireParameters)
	}
	return member
}

func copyBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	result := *value
	return &result
}

func requestSpecMetadataString(spec modelrequest.Spec, key string) string {
	value, _ := spec.VariantMetadata[key].(string)
	return strings.TrimSpace(value)
}

func (r *runner) constituteCouncil(ctx context.Context) error {
	preflighter, ok := r.client.(councilCandidatePreflighter)
	if !ok {
		council, err := loadCouncilWithOptions(r.cfg.CouncilPoolPath, councilsample.Options{
			Count:                    r.cfg.CouncilSize,
			AllowedEndpoints:         r.cfg.CouncilAllowedEndpoints,
			MinimumDistinctEndpoints: r.cfg.CouncilMinEndpoints,
		})
		if err != nil {
			return err
		}
		r.council = council
		return nil
	}
	candidates, err := councilCandidates(r.cfg.CouncilPoolPath)
	if err != nil {
		return err
	}
	check := func(ctx context.Context, member CouncilMember) error {
		prompt, err := r.renderPrompt(
			"council.preflight",
			"{{MEMBER_ID}}", member.MemberID,
			"{{MODEL}}", member.Model,
			"{{PERSONA_FILE}}", member.PersonaFile,
		)
		if err != nil {
			return err
		}
		requestCtx, cancel := context.WithTimeout(ctx, councilPreflightTimeout(r.cfg.CouncilTimeout))
		defer cancel()
		response, err := preflighter.PreflightCouncilCandidate(
			requestCtx,
			member,
			[]map[string]any{{"role": "user", "content": prompt}},
			councilTools(r.toolPrompt("council_vote")),
		)
		if err != nil {
			return err
		}
		if _, err := parseVote(member, response, r.cfg.MaxResponseBytes); err != nil {
			return &openaiapi.ProviderError{Class: openaiapi.ProviderErrorProtocol, Err: fmt.Errorf("invalid council preflight response: %w", err)}
		}
		return nil
	}
	council, rejections, err := selectAvailableCouncilWithOptions(ctx, candidates, councilsample.Options{
		Count:                    r.cfg.CouncilSize,
		AllowedEndpoints:         r.cfg.CouncilAllowedEndpoints,
		MinimumDistinctEndpoints: r.cfg.CouncilMinEndpoints,
	}, check)
	r.council = council
	for _, rejection := range rejections {
		if recordErr := r.recordEvent("council_candidate_rejected", "system", councilCandidateRejectionPayload(rejection)); recordErr != nil {
			return errors.Join(err, recordErr)
		}
	}
	return err
}

func loadCouncilWithOptions(path string, opts councilsample.Options) ([]CouncilMember, error) {
	candidates, incompatible, err := loadEligibleCouncilCandidates(path)
	if err != nil {
		return nil, err
	}
	endpoints := make([]string, len(candidates))
	for index, candidate := range candidates {
		endpoints[index] = candidate.model.Endpoint
	}
	selector, err := councilsample.New(endpoints, opts)
	if err != nil {
		if len(candidates) == 0 && incompatible > 0 {
			return nil, fmt.Errorf("council pool has no compatible records; %d record(s) omit required parameter tools", incompatible)
		}
		return nil, err
	}
	council := make([]CouncilMember, 0, opts.Count)
	for seat := 1; seat <= opts.Count; seat++ {
		candidateIndex, err := selector.Draw()
		if err != nil {
			return nil, err
		}
		if err := selector.Accept(candidateIndex); err != nil {
			return nil, err
		}
		council = append(council, councilMemberFromCandidate(candidates[candidateIndex], fmt.Sprintf("C%d", seat)))
	}
	if err := selector.Validate(); err != nil {
		return nil, err
	}
	return council, nil
}

func councilCandidateRejectionPayload(rejection councilCandidateRejection) map[string]any {
	payload := map[string]any{
		"member_id":                rejection.MemberID,
		"unavailable_model":        rejection.Unavailable.Model,
		"unavailable_persona_file": rejection.Unavailable.PersonaFile,
		"cause":                    rejection.Cause,
		"error_class":              rejection.ErrorClass,
	}
	addCouncilRoutePayload(payload, "unavailable", rejection.Unavailable)
	if rejection.Replacement != nil {
		payload["replacement_model"] = rejection.Replacement.Model
		payload["replacement_persona_file"] = rejection.Replacement.PersonaFile
		addCouncilRoutePayload(payload, "replacement", *rejection.Replacement)
	}
	return payload
}

func addCouncilRoutePayload(payload map[string]any, prefix string, member CouncilMember) {
	key := func(name string) string {
		if prefix == "" {
			return name
		}
		return prefix + "_" + name
	}
	optional := map[string]string{
		key("endpoint_variant_id"): member.EndpointVariantID,
		key("provider_name"):       member.ProviderName,
		key("endpoint_tag"):        member.EndpointTag,
		key("quantization"):        member.Quantization,
	}
	for key, value := range optional {
		if value != "" {
			payload[key] = value
		}
	}
	if len(member.ProviderOnly) > 0 {
		payload[key("provider_only")] = append([]string(nil), member.ProviderOnly...)
	}
	if len(member.ProviderQuantizations) > 0 {
		payload[key("provider_quantizations")] = append([]string(nil), member.ProviderQuantizations...)
	}
	if member.ProviderAllowFallbacks != nil {
		payload[key("provider_allow_fallbacks")] = *member.ProviderAllowFallbacks
	}
	if member.ProviderRequireParameters != nil {
		payload[key("provider_require_parameters")] = *member.ProviderRequireParameters
	}
}

func selectAvailableCouncil(
	ctx context.Context,
	candidates []CouncilMember,
	size int,
	check func(context.Context, CouncilMember) error,
) ([]CouncilMember, []councilCandidateRejection, error) {
	return selectAvailableCouncilWithOptions(ctx, candidates, councilsample.Options{Count: size}, check)
}

func selectAvailableCouncilWithOptions(
	ctx context.Context,
	candidates []CouncilMember,
	opts councilsample.Options,
	check func(context.Context, CouncilMember) error,
) ([]CouncilMember, []councilCandidateRejection, error) {
	if check == nil {
		return nil, nil, fmt.Errorf("council candidate check is required")
	}
	endpoints := make([]string, len(candidates))
	for index, candidate := range candidates {
		endpoints[index] = councilMemberEndpoint(candidate)
	}
	selector, err := councilsample.New(endpoints, opts)
	if err != nil {
		return nil, nil, err
	}
	seated := make([]CouncilMember, 0, opts.Count)
	rejections := make([]councilCandidateRejection, 0)
	available := make(map[int]bool)
	for seat := 1; seat <= opts.Count; seat++ {
		if err := ctx.Err(); err != nil {
			return seated, rejections, err
		}
		memberID := fmt.Sprintf("C%d", seat)
		firstRejection := len(rejections)
		for {
			candidateIndex, drawErr := selector.Draw()
			if drawErr != nil {
				if len(rejections) == firstRejection {
					return seated, rejections, fmt.Errorf("council preflight could not seat %s: %w", memberID, drawErr)
				}
				last := rejections[len(rejections)-1]
				return seated, rejections, fmt.Errorf("council preflight could not seat %s after %d rejected candidate(s): %w", memberID, len(rejections)-firstRejection, last.err)
			}
			candidate := candidates[candidateIndex]
			candidate.MemberID = memberID
			if available[candidateIndex] {
				if err := selector.Accept(candidateIndex); err != nil {
					return seated, rejections, err
				}
				seated = append(seated, candidate)
				replacement := candidate
				for index := firstRejection; index < len(rejections); index++ {
					rejections[index].Replacement = &replacement
				}
				break
			}
			candidateErr := check(ctx, candidate)
			if candidateErr != nil {
				err := sanitizeError(candidateErr)
				rejections = append(rejections, councilCandidateRejection{
					MemberID:    memberID,
					Unavailable: candidate,
					Cause:       err.Error(),
					ErrorClass:  string(openaiapi.ErrorClass(err)),
					err:         err,
				})
				if ctxErr := ctx.Err(); ctxErr != nil {
					return seated, rejections, ctxErr
				}
				if failedEndpoint, ok := modelgateway.CredentialFailureEndpoint(candidateErr); ok {
					if err := selector.RejectEndpoint(failedEndpoint); err != nil {
						return seated, rejections, err
					}
				} else if err := selector.Reject(candidateIndex); err != nil {
					return seated, rejections, err
				}
				continue
			}
			available[candidateIndex] = true
			if err := selector.Accept(candidateIndex); err != nil {
				return seated, rejections, err
			}
			seated = append(seated, candidate)
			replacement := candidate
			for index := firstRejection; index < len(rejections); index++ {
				rejections[index].Replacement = &replacement
			}
			break
		}
	}
	if err := selector.Validate(); err != nil {
		return seated, rejections, err
	}
	return seated, rejections, nil
}

func councilMemberEndpoint(member CouncilMember) string {
	if member.RequestSpec != nil {
		if endpoint := strings.ToLower(strings.TrimSpace(member.RequestSpec.Endpoint)); endpoint != "" {
			return endpoint
		}
	}
	model, err := modelrequest.ParseModelRef(member.Model)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(model.Endpoint))
}

func councilPreflightTimeout(councilTimeout time.Duration) time.Duration {
	const maximum = 20 * time.Second
	if councilTimeout <= 0 || councilTimeout > maximum {
		return maximum
	}
	return councilTimeout
}

func (r *runner) execute(ctx context.Context) error {
	if err := r.runLawyerTurn(ctx, "plaintiff"); err != nil {
		return err
	}
	if err := r.runLawyerTurn(ctx, "defendant"); err != nil {
		return err
	}
	r.mu.Lock()
	r.phase = "council"
	r.version++
	r.cond.Broadcast()
	r.mu.Unlock()
	if err := r.recordEvent("council_started", "system", map[string]any{
		"member_count": len(r.council),
		"parallel":     r.cfg.ParallelCouncil,
	}); err != nil {
		return err
	}
	return r.runCouncil(ctx)
}

func (r *runner) runCouncil(ctx context.Context) error {
	if r.cfg.ParallelCouncil {
		return r.runCouncilParallel(ctx)
	}
	for _, member := range r.council {
		vote, err := r.requestVote(ctx, member)
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}
		if err != nil {
			reason, ok := councilMemberFailureReason(err)
			if !ok {
				return fmt.Errorf("council member %s: %w", member.MemberID, err)
			}
			if recordErr := recordCouncilOutcome(ctx, func() error {
				if err := r.recordCouncilFailure(member, reason, err); err != nil {
					return fmt.Errorf("record council member %s failure: %w", member.MemberID, err)
				}
				return nil
			}); recordErr != nil {
				return recordErr
			}
			continue
		}
		if err := recordCouncilOutcome(ctx, func() error { return r.recordCouncilVote(vote) }); err != nil {
			return err
		}
	}
	return context.Cause(ctx)
}

type councilVoteResult struct {
	index int
	vote  Vote
	err   error
}

func (r *runner) runCouncilParallel(ctx context.Context) error {
	requestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan councilVoteResult, len(r.council))
	for index, member := range r.council {
		go func() {
			vote, err := r.requestVote(requestCtx, member)
			results <- councilVoteResult{index: index, vote: vote, err: err}
		}()
	}

	outcomes := make([]councilVoteResult, len(r.council))
	var requestErr error
	for range r.council {
		result := <-results
		outcomes[result.index] = result
		if result.err == nil {
			continue
		}
		if _, ok := councilMemberFailureReason(result.err); ok {
			continue
		}
		memberErr := fmt.Errorf("council member %s: %w", r.council[result.index].MemberID, result.err)
		if requestErr == nil {
			requestErr = memberErr
			cancel()
			continue
		}
		if !errors.Is(result.err, context.Canceled) {
			requestErr = errors.Join(requestErr, memberErr)
		}
	}
	if cause := context.Cause(ctx); cause != nil {
		requestErr = errors.Join(requestErr, cause)
	}
	var recordErr error
	for index, outcome := range outcomes {
		if outcome.err != nil {
			reason, ok := councilMemberFailureReason(outcome.err)
			if !ok {
				continue
			}
			if err := recordCouncilOutcome(ctx, func() error {
				if err := r.recordCouncilFailure(r.council[index], reason, outcome.err); err != nil {
					return fmt.Errorf("record council member %s failure: %w", r.council[index].MemberID, err)
				}
				return nil
			}); err != nil {
				recordErr = errors.Join(recordErr, err)
				break
			}
			continue
		}
		if err := recordCouncilOutcome(ctx, func() error { return r.recordCouncilVote(outcome.vote) }); err != nil {
			recordErr = errors.Join(recordErr, err)
			break
		}
	}
	return errors.Join(requestErr, recordErr, context.Cause(ctx))
}

func recordCouncilOutcome(ctx context.Context, record func() error) error {
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	recordErr := record()
	return errors.Join(recordErr, context.Cause(ctx))
}

func (r *runner) recordCouncilFailure(member CouncilMember, reason string, cause error) error {
	failure := CouncilMemberFailure{
		MemberID:      member.MemberID,
		Model:         member.Model,
		PersonaFile:   member.PersonaFile,
		Status:        "failed",
		FailureReason: strings.TrimSpace(reason),
		Message:       sanitizeText(cause.Error()),
		ErrorClass:    string(openaiapi.ErrorClass(cause)),
		FailedAt:      time.Now().UTC(),
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transcript.CouncilFailures = append(r.transcript.CouncilFailures, failure)
	if err := r.records.writeTranscript(cloneTranscript(r.transcript)); err != nil {
		r.transcript.CouncilFailures = r.transcript.CouncilFailures[:len(r.transcript.CouncilFailures)-1]
		return err
	}
	payload := map[string]any{
		"member_id":      failure.MemberID,
		"model":          failure.Model,
		"persona_file":   failure.PersonaFile,
		"status":         failure.Status,
		"failure_reason": failure.FailureReason,
		"message":        failure.Message,
		"cause":          failure.Message,
		"failed_at":      failure.FailedAt,
	}
	if failure.ErrorClass != "" {
		payload["error_class"] = failure.ErrorClass
	}
	addCouncilRoutePayload(payload, "", member)
	return r.appendEventLocked("council_member_removed", "system", payload)
}

func (r *runner) recordCouncilVote(vote Vote) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transcript.Votes = append(r.transcript.Votes, vote)
	if err := r.records.writeTranscript(cloneTranscript(r.transcript)); err != nil {
		r.transcript.Votes = r.transcript.Votes[:len(r.transcript.Votes)-1]
		return err
	}
	payload := map[string]any{
		"member_id":    vote.MemberID,
		"model":        vote.Model,
		"persona_file": vote.PersonaFile,
		"vote":         vote.Vote,
		"rationale":    vote.Rationale,
		"response_id":  vote.ResponseID,
	}
	if vote.ProviderUsage != nil {
		payload["provider_usage"] = vote.ProviderUsage
	}
	if vote.ProviderCostUSD != nil {
		payload["provider_cost_usd"] = *vote.ProviderCostUSD
	}
	if vote.ProviderMetadataError != "" {
		payload["provider_metadata_error"] = vote.ProviderMetadataError
	}
	return r.appendEventLocked("council_vote", "council", payload)
}

func (r *runner) recordEvent(eventType, role string, payload map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.appendEventLocked(eventType, role, payload)
}

func (r *runner) appendEventLocked(eventType, role string, payload map[string]any) error {
	event := Event{Sequence: r.sequence + 1, Timestamp: time.Now().UTC(), Type: eventType, Role: role, Payload: payload}
	if err := r.records.appendEvent(event); err != nil {
		return err
	}
	r.sequence = event.Sequence
	r.events = append(r.events, event)
	return nil
}

func (r *runner) setTerminal(runErr error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.terminal = true
	r.terminalErr = runErr
	if runErr != nil {
		r.phase = "failed"
	} else {
		r.phase = "complete"
	}
	r.active = nil
	r.version++
	r.cond.Broadcast()
}

func (r *runner) resultForError(err error) Result {
	return r.result("", err)
}

func (r *runner) result(caseAPIBase string, runErr error) Result {
	r.mu.Lock()
	defer r.mu.Unlock()
	status := "running"
	finishedAt := time.Time{}
	if runErr != nil {
		status = "failed"
		finishedAt = time.Now().UTC()
	} else if r.terminal || r.phase == "complete" {
		status = "ok"
		finishedAt = time.Now().UTC()
	}
	votesFor, votesAgainst := countVotes(r.transcript.Votes)
	result := Result{
		SchemaVersion:    resultSchema,
		Procedure:        Procedure,
		CaseID:           r.cfg.CaseID,
		RunID:            r.cfg.RunID,
		Status:           status,
		Phase:            r.phase,
		Proposition:      r.cfg.Proposition,
		Resolution:       resolutionFor(votesFor, votesAgainst, r.cfg.RequiredVotes, councilComplete(r.transcript, r.cfg.CouncilSize)),
		CouncilSize:      r.cfg.CouncilSize,
		RequiredVotes:    r.cfg.RequiredVotes,
		EvidenceStandard: r.cfg.EvidenceStandard,
		VotesFor:         votesFor,
		VotesAgainst:     votesAgainst,
		CaseAPIBase:      caseAPIBase,
		StartedAt:        r.startedAt,
		FinishedAt:       finishedAt,
		Documents:        r.documents,
		Council:          append([]CouncilMember(nil), r.council...),
		Arguments:        append([]Argument(nil), r.transcript.Arguments...),
		Votes:            append([]Vote(nil), r.transcript.Votes...),
		CouncilFailures:  append([]CouncilMemberFailure(nil), r.transcript.CouncilFailures...),
		Events:           append([]Event(nil), r.events...),
		Provider:         r.client.Accounting(),
	}
	if runErr != nil {
		result.Error = sanitizeText(runErr.Error())
		result.ErrorClass = string(openaiapi.ErrorClass(runErr))
	}
	return result
}

func (r *runner) resolution() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	forVotes, againstVotes := countVotes(r.transcript.Votes)
	return resolutionFor(forVotes, againstVotes, r.cfg.RequiredVotes, councilComplete(r.transcript, r.cfg.CouncilSize))
}

func councilComplete(transcript Transcript, councilSize int) bool {
	return len(transcript.Votes)+len(transcript.CouncilFailures) == councilSize
}

func countVotes(votes []Vote) (int, int) {
	var forVotes, againstVotes int
	for _, vote := range votes {
		switch vote.Vote {
		case "demonstrated":
			forVotes++
		case "not_demonstrated":
			againstVotes++
		}
	}
	return forVotes, againstVotes
}

func resolutionFor(forVotes, againstVotes, requiredVotes int, complete bool) string {
	if !complete {
		return ""
	}
	if forVotes >= requiredVotes {
		return "demonstrated"
	}
	if againstVotes >= requiredVotes {
		return "not_demonstrated"
	}
	return "no_majority"
}

func cloneTranscript(value Transcript) Transcript {
	value.Arguments = append([]Argument(nil), value.Arguments...)
	value.Votes = append([]Vote(nil), value.Votes...)
	value.CouncilFailures = append([]CouncilMemberFailure(nil), value.CouncilFailures...)
	return value
}
