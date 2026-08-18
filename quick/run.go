package quick

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jsmorph/adj/common/casemanifest"
	"github.com/jsmorph/adj/common/documents"
	"github.com/jsmorph/adj/common/modelrequest"
	openaiapi "github.com/jsmorph/adj/common/openai"
	"github.com/jsmorph/adj/common/persona"
	"github.com/jsmorph/adj/common/recordio"
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
		SchemaVersion: transcriptSchema,
		CaseID:        cfg.CaseID,
		Proposition:   cfg.Proposition,
		Arguments:     []Argument{},
		Votes:         []Vote{},
	}
	manifest := casemanifest.New(Procedure, cfg.CaseID, cfg.RunID, r.startedAt)
	if err := casemanifest.WriteAtomic(cfg.OutputDir, manifest); err != nil {
		err = sanitizeError(err)
		return r.resultForError(err), err
	}
	if err := r.initializeFiles(); err != nil {
		return r.finishWithoutAPI(err)
	}
	if preflighter, ok := client.(councilEndpointPreflighter); ok {
		if err := preflighter.PreflightCouncilEndpoints(r.council); err != nil {
			return r.finishWithoutAPI(fmt.Errorf("preflight selected council endpoints: %w", err))
		}
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
	if err := recordio.WriteJSON(filepath.Join(r.cfg.OutputDir, "documents.json"), r.documents); err != nil {
		return err
	}
	council, err := loadCouncil(r.cfg.CouncilPoolPath, r.cfg.CouncilSize)
	if err != nil {
		return err
	}
	r.council = council
	if err := r.records.writeTranscript(r.transcript); err != nil {
		return err
	}
	initial := r.result("", nil)
	initial.Status = "running"
	initial.Phase = "initializing"
	return r.records.writeRun(initial)
}

type randomIndex func(upperBound int) (int, error)

func loadCouncil(path string, size int) ([]CouncilMember, error) {
	return loadCouncilWithRandomIndex(path, size, cryptoRandomIndex)
}

func loadCouncilWithRandomIndex(path string, size int, choose randomIndex) ([]CouncilMember, error) {
	specs, err := persona.LoadRecordsFile(path, filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("load council pool: %w", sanitizeError(err))
	}
	if choose == nil {
		return nil, fmt.Errorf("council random-index source is required")
	}
	if size <= 0 {
		return nil, fmt.Errorf("council size must be positive")
	}
	if len(specs) < size {
		return nil, fmt.Errorf("council size %d exceeds available pool %d", size, len(specs))
	}
	type councilCandidate struct {
		spec  persona.Spec
		model modelrequest.ModelRef
	}
	candidates := make([]councilCandidate, len(specs))
	for index, spec := range specs {
		if spec.RequestSpec == nil {
			return nil, fmt.Errorf("council pool record %d must be a JSON request specification", index+1)
		}
		modelRef, err := modelrequest.ParseModelRef(spec.Model)
		if err != nil {
			return nil, fmt.Errorf("parse council pool record %d model: %w", index+1, sanitizeError(err))
		}
		candidates[index] = councilCandidate{spec: spec, model: modelRef}
	}
	council := make([]CouncilMember, 0, size)
	for index := 0; index < size; index++ {
		candidateIndex, err := choose(len(candidates))
		if err != nil {
			return nil, fmt.Errorf("sample council member %d: %w", index+1, err)
		}
		if candidateIndex < 0 || candidateIndex >= len(candidates) {
			return nil, fmt.Errorf("sample council member %d: random index %d outside [0,%d)", index+1, candidateIndex, len(candidates))
		}
		candidate := candidates[candidateIndex]
		candidates = append(candidates[:candidateIndex], candidates[candidateIndex+1:]...)
		spec := candidate.spec
		requestSpec := *spec.RequestSpec
		council = append(council, CouncilMember{
			MemberID:    fmt.Sprintf("C%d", index+1),
			Model:       candidate.model.Endpoint + "://" + candidate.model.Model,
			PersonaFile: spec.File,
			RequestSpec: &requestSpec,
			PersonaText: spec.Text,
		})
	}
	return council, nil
}

func cryptoRandomIndex(upperBound int) (int, error) {
	if upperBound <= 0 {
		return 0, fmt.Errorf("random index upper bound must be positive")
	}
	value, err := rand.Int(rand.Reader, big.NewInt(int64(upperBound)))
	if err != nil {
		return 0, fmt.Errorf("read cryptographic randomness: %w", err)
	}
	return int(value.Int64()), nil
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
		if err != nil {
			return fmt.Errorf("council member %s: %w", member.MemberID, err)
		}
		if err := r.recordCouncilVote(vote); err != nil {
			return err
		}
	}
	return nil
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

	votes := make([]*Vote, len(r.council))
	var requestErr error
	for range r.council {
		result := <-results
		if result.err == nil {
			vote := result.vote
			votes[result.index] = &vote
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
	var recordErr error
	for _, vote := range votes {
		if vote == nil {
			continue
		}
		if err := r.recordCouncilVote(*vote); err != nil {
			recordErr = errors.Join(recordErr, err)
			break
		}
	}
	return errors.Join(requestErr, recordErr)
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
		Resolution:       resolutionFor(votesFor, votesAgainst, r.cfg.RequiredVotes, len(r.transcript.Votes) == r.cfg.CouncilSize),
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
	return resolutionFor(forVotes, againstVotes, r.cfg.RequiredVotes, len(r.transcript.Votes) == r.cfg.CouncilSize)
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
	return value
}
