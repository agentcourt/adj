package quick

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentcourt/adj/common/councilsample"
	"github.com/agentcourt/adj/common/documents"
	"github.com/agentcourt/adj/common/modelgateway"
	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

const testLawyerAPIBearerToken = "test-quick-lawyer-api-token"

func TestCaseAPIHealthIdentifiesRun(t *testing.T) {
	api := &caseAPI{runner: &runner{cfg: Config{CaseID: "case-1", RunID: "run-1"}}}
	response := httptest.NewRecorder()
	api.handleHealth(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"case_id":"case-1"`) || !strings.Contains(response.Body.String(), `"run_id":"run-1"`) {
		t.Fatalf("health response: %d %s", response.Code, response.Body.String())
	}
}

func TestCaseAPIWaitReturnsAtTimeoutWithoutStateChange(t *testing.T) {
	const wait = 20 * time.Millisecond
	runner := newLawyerTestRunner(t, time.Minute)
	runner.cfg.CaseID = "case"
	runner.phase = "arguments"
	runner.version = 7
	runner.active = &lawyerTurn{
		role:              "plaintiff",
		opportunityID:     "arguments:plaintiff",
		deadline:          time.Now().Add(time.Minute),
		attemptsMax:       1,
		attemptsRemaining: 1,
		done:              make(chan error, 1),
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/lawyerapi/v1/wait?case_id=case&role_id=defendant&after_version=7&timeout_ms=%d", wait.Milliseconds()),
		nil,
	)
	done := make(chan struct{})
	started := time.Now()
	go func() {
		(&caseAPI{runner: runner}).handleWait(response, request)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		runner.mu.Lock()
		runner.version++
		runner.cond.Broadcast()
		runner.mu.Unlock()
		<-done
		t.Fatal("wait handler did not return after its timeout")
	}
	if elapsed := time.Since(started); elapsed < wait {
		t.Fatalf("wait handler returned after %s, before its %s timeout", elapsed, wait)
	}
	if response.Code != http.StatusOK {
		t.Fatalf("wait status = %d, body = %s", response.Code, response.Body.String())
	}
	var body struct {
		Status       string `json:"status"`
		StateVersion uint64 `json:"state_version"`
		Wait         struct {
			Reason       string `json:"reason"`
			Version      uint64 `json:"version"`
			StateVersion uint64 `json:"state_version"`
		} `json:"wait"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode wait response: %v", err)
	}
	if body.Status != "waiting" || body.StateVersion != 7 || body.Wait.Reason != "timeout" || body.Wait.Version != 7 || body.Wait.StateVersion != 7 {
		t.Fatalf("wait response = %#v", body)
	}
}

func TestCaseResultIncludesCouncilFailures(t *testing.T) {
	runner := newLawyerTestRunner(t, time.Minute)
	runner.cfg.CouncilSize = 3
	runner.cfg.RequiredVotes = 2
	runner.phase = "complete"
	runner.transcript.Votes = []Vote{
		{MemberID: "C1", Vote: "not_demonstrated"},
		{MemberID: "C3", Vote: "not_demonstrated"},
	}
	runner.transcript.CouncilFailures = []CouncilMemberFailure{{
		MemberID:      "C2",
		Model:         "openrouter://model-2",
		Status:        "failed",
		FailureReason: councilFailureRequestFailed,
		Message:       "provider unavailable",
		ErrorClass:    string(openaiapi.ProviderErrorTransient),
		FailedAt:      time.Now().UTC(),
	}}
	result, err := (&caseAPI{runner: runner}).executeTool(doRequest{Tool: "get_case_result"})
	if err != nil {
		t.Fatal(err)
	}
	if result["resolution"] != "not_demonstrated" {
		t.Fatalf("result = %#v", result)
	}
	failures, ok := result["council_failures"].([]CouncilMemberFailure)
	if !ok || len(failures) != 1 || failures[0].MemberID != "C2" {
		t.Fatalf("council failures = %#v", result["council_failures"])
	}
}

type capturedRequest struct {
	Spec               modelrequest.Spec
	Input              []map[string]any
	Tools              []map[string]any
	PreviousResponseID string
}

type fakeResponseClient struct {
	mu         sync.Mutex
	responses  []openaiapi.Response
	outcomes   []councilClientOutcome
	requests   []capturedRequest
	accounting openaiapi.AccountingRecorder
}

func (c *fakeResponseClient) CreateResponseWithRequestSpec(
	_ context.Context,
	spec modelrequest.Spec,
	input []map[string]any,
	tools []map[string]any,
	previousResponseID string,
) (openaiapi.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests = append(c.requests, capturedRequest{Spec: spec, Input: input, Tools: tools, PreviousResponseID: previousResponseID})
	if len(c.outcomes) > 0 {
		outcome := c.outcomes[0]
		c.outcomes = c.outcomes[1:]
		c.accounting.Record(outcome.response)
		return outcome.response, outcome.err
	}
	if len(c.responses) == 0 {
		return openaiapi.Response{}, fmt.Errorf("unexpected council request")
	}
	response := c.responses[0]
	c.responses = c.responses[1:]
	c.accounting.Record(response)
	return response, nil
}

func (c *fakeResponseClient) Accounting() openaiapi.Accounting { return c.accounting.Snapshot() }

type timedCouncilClient struct {
	mu        sync.Mutex
	active    int
	maxActive int
	models    []string
	delay     time.Duration
}

func (c *timedCouncilClient) CreateResponseWithRequestSpec(
	ctx context.Context,
	spec modelrequest.Spec,
	_ []map[string]any,
	_ []map[string]any,
	_ string,
) (openaiapi.Response, error) {
	c.mu.Lock()
	c.active++
	if c.active > c.maxActive {
		c.maxActive = c.active
	}
	c.models = append(c.models, spec.Model)
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.active--
		c.mu.Unlock()
	}()
	select {
	case <-time.After(c.delay):
		return voteResponse("response-"+spec.Model, "demonstrated", "rationale for "+spec.Model), nil
	case <-ctx.Done():
		return openaiapi.Response{}, ctx.Err()
	}
}

func (c *timedCouncilClient) Accounting() openaiapi.Accounting { return openaiapi.Accounting{} }

type successAfterContextClient struct {
	started chan struct{}
}

func (c *successAfterContextClient) CreateResponseWithRequestSpec(
	ctx context.Context,
	_ modelrequest.Spec,
	_ []map[string]any,
	_ []map[string]any,
	_ string,
) (openaiapi.Response, error) {
	close(c.started)
	<-ctx.Done()
	return voteResponse("response-after-context", "demonstrated", "late response"), nil
}

func (c *successAfterContextClient) Accounting() openaiapi.Accounting {
	return openaiapi.Accounting{}
}

type councilTimeoutTestError struct{}

func (councilTimeoutTestError) Error() string   { return "transport timed out" }
func (councilTimeoutTestError) Timeout() bool   { return true }
func (councilTimeoutTestError) Temporary() bool { return true }

type controlledCouncilClient struct {
	started   chan string
	completed chan string
	release   map[string]chan struct{}
	failModel string
	fail      chan struct{}
	failErr   error
	canceled  chan string
}

func (c *controlledCouncilClient) CreateResponseWithRequestSpec(
	ctx context.Context,
	spec modelrequest.Spec,
	_ []map[string]any,
	_ []map[string]any,
	_ string,
) (openaiapi.Response, error) {
	c.started <- spec.Model
	if spec.Model == c.failModel {
		select {
		case <-c.fail:
			if c.failErr != nil {
				return openaiapi.Response{}, c.failErr
			}
			return openaiapi.Response{}, &openaiapi.ProviderError{
				Class: openaiapi.ProviderErrorRequest,
				Err:   fmt.Errorf("request rejected for %s", spec.Model),
			}
		case <-ctx.Done():
			return openaiapi.Response{}, ctx.Err()
		}
	}
	select {
	case <-c.release[spec.Model]:
		c.completed <- spec.Model
		return voteResponse("response-"+spec.Model, "demonstrated", "rationale for "+spec.Model), nil
	case <-ctx.Done():
		c.canceled <- spec.Model
		return openaiapi.Response{}, ctx.Err()
	}
}

func (c *controlledCouncilClient) Accounting() openaiapi.Accounting { return openaiapi.Accounting{} }

type councilClientOutcome struct {
	response openaiapi.Response
	err      error
}

type scriptedCouncilClient struct {
	mu       sync.Mutex
	outcomes map[string]councilClientOutcome
	models   []string
}

func (c *scriptedCouncilClient) CreateResponseWithRequestSpec(
	_ context.Context,
	spec modelrequest.Spec,
	_ []map[string]any,
	_ []map[string]any,
	_ string,
) (openaiapi.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.models = append(c.models, spec.Model)
	outcome, ok := c.outcomes[spec.Model]
	if !ok {
		return openaiapi.Response{}, fmt.Errorf("no scripted outcome for %s", spec.Model)
	}
	return outcome.response, outcome.err
}

func (c *scriptedCouncilClient) Accounting() openaiapi.Accounting {
	return openaiapi.Accounting{}
}

func TestRunQuickCase(t *testing.T) {
	root := t.TempDir()
	documentsDir := filepath.Join(root, "source-documents")
	if err := os.Mkdir(documentsDir, 0o755); err != nil {
		t.Fatalf("create documents: %v", err)
	}
	if err := os.WriteFile(filepath.Join(documentsDir, "record.txt"), []byte("The measured value is blue.\n"), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}
	poolPath := writeCouncilPool(t, root, 3)
	outputDir := filepath.Join(root, "out")
	responses := []openaiapi.Response{
		voteResponse("response-1", "demonstrated", "The record supports the proposition."),
		voteResponse("response-2", "not_demonstrated", "The opponent identifies uncertainty."),
		voteResponse("response-3", "demonstrated", "The evidence meets the stated standard."),
	}
	for index := range responses {
		responses[index].Usage = openaiapi.Usage{InputTokens: int64(index + 1), OutputTokens: 1, TotalTokens: int64(index + 2)}
		responses[index].UsageKnown = true
		responses[index].OpenRouterCostUSD = float64(index+1) / 10_000
		responses[index].OpenRouterCostKnown = true
	}
	client := &fakeResponseClient{responses: responses}
	cfg := Config{
		Proposition:            "The sky is blue.",
		DocumentsDir:           documentsDir,
		OutputDir:              outputDir,
		CouncilPoolPath:        poolPath,
		CouncilSize:            3,
		RequiredVotes:          2,
		EvidenceStandard:       "preponderance of the evidence",
		CaseAPIAddr:            "127.0.0.1:0",
		LawyerAPIBearerToken:   testLawyerAPIBearerToken,
		CaseID:                 "quick-test",
		RunID:                  "run-test",
		LawyerTimeout:          5 * time.Second,
		CouncilTimeout:         5 * time.Second,
		MaxResponseBytes:       1 << 20,
		MaxArgumentChars:       10_000,
		InvalidAttemptLimit:    1,
		DocumentLimits:         documents.Limits{MaxFiles: 4, MaxFileBytes: 1024, MaxTotalBytes: 4096},
		CouncilRequestAttempts: 1,
		AllowAPIKey:            true,
	}

	type runOutcome struct {
		result Result
		err    error
	}
	done := make(chan runOutcome, 1)
	go func() {
		result, err := runConfigured(context.Background(), cfg, client)
		done <- runOutcome{result: result, err: err}
	}()

	runtime := waitForRuntime(t, filepath.Join(outputDir, "runtime.json"))
	unauthorized, err := http.Get(runtime.CaseAPIBase + "/lawyerapi/v1/get?case_id=" + cfg.CaseID + "&role_id=plaintiff")
	if err != nil {
		t.Fatal(err)
	}
	if closeErr := unauthorized.Body.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if unauthorized.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated lawyer API status = %d", unauthorized.StatusCode)
	}
	wait := getJSON(t, runtime.CaseAPIBase+"/lawyerapi/v1/wait?case_id="+cfg.CaseID+"&role_id=plaintiff&timeout_ms=100")
	if wait["status"] != "ready" {
		t.Fatalf("lawyer wait = %#v", wait)
	}
	status := getJSON(t, runtime.CaseAPIBase+"/lawyerapi/v1/status?case_id="+cfg.CaseID+"&role_id=observer")
	if status["status"] != "observing" {
		t.Fatalf("observer status = %#v", status)
	}
	pending := getJSON(t, runtime.CaseAPIBase+"/lawyerapi/v1/result?case_id="+cfg.CaseID+"&role_id=observer")
	if pending["status"] != "pending" {
		t.Fatalf("pending result = %#v", pending)
	}
	submitLawyerArgument(t, runtime.CaseAPIBase, cfg.CaseID, "plaintiff", "arguments:plaintiff", "The document reports blue.")
	submitLawyerArgument(t, runtime.CaseAPIBase, cfg.CaseID, "defendant", "arguments:defendant", "The report may not establish the whole sky.")

	select {
	case outcome := <-done:
		if outcome.err != nil {
			t.Fatalf("run quick case: %v", outcome.err)
		}
		if outcome.result.Status != "ok" || outcome.result.Resolution != "demonstrated" {
			t.Fatalf("result = %#v", outcome.result)
		}
		if outcome.result.VotesFor != 2 || outcome.result.VotesAgainst != 1 {
			t.Fatalf("vote counts = %d/%d", outcome.result.VotesFor, outcome.result.VotesAgainst)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("quick case did not finish")
	}

	for _, name := range []string{"input.json", "runtime.json", "documents.json", "events.ndjson", "transcript.json", "run.json", "case-manifest.json"} {
		if _, err := os.Stat(filepath.Join(outputDir, name)); err != nil {
			t.Errorf("stat %s: %v", name, err)
		}
	}
	client.mu.Lock()
	requests := append([]capturedRequest(nil), client.requests...)
	client.mu.Unlock()
	if len(requests) != 3 {
		t.Fatalf("council requests = %d, want 3", len(requests))
	}
	for index, request := range requests {
		if len(request.Input) != 2 || request.Input[0]["role"] != "system" || request.Input[1]["role"] != "user" {
			t.Errorf("request %d input = %#v", index, request.Input)
		}
		promptBytes, err := json.Marshal(request.Input)
		if err != nil {
			t.Fatal(err)
		}
		prompt := string(promptBytes)
		for _, required := range []string{
			"The sky is blue.",
			"The document reports blue.",
			"The report may not establish the whole sky.",
			"preponderance of the evidence",
			"Use demonstrated only when the proposition satisfies that standard for every required part",
			"Use vote=demonstrated only if the proposition satisfies the stated evidence standard",
			"record.txt",
			"The measured value is blue.",
		} {
			if !strings.Contains(prompt, required) {
				t.Errorf("request %d prompt omits %q", index, required)
			}
		}
		if !reflect.DeepEqual(request.Tools, councilTools(defaultCouncilVoteToolPrompt)) {
			t.Errorf("request %d tools = %#v", index, request.Tools)
		}
		description, ok := request.Tools[0]["description"].(string)
		if !ok || !strings.Contains(description, "Use demonstrated only when the proposition satisfies the stated evidence standard") {
			t.Errorf("request %d council tool description = %#v", index, request.Tools[0]["description"])
		}
		if strict, ok := request.Tools[0]["strict"].(bool); !ok || !strict {
			t.Errorf("request %d council tool strict = %#v", index, request.Tools[0]["strict"])
		}
		if request.PreviousResponseID != "" {
			t.Errorf("request %d previous response = %q", index, request.PreviousResponseID)
		}
	}
	assertRecordsDoNotContain(t, outputDir, "secret-header-value")
	assertRecordsDoNotContain(t, outputDir, testLawyerAPIBearerToken)
	runRecord, err := os.ReadFile(filepath.Join(outputDir, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(runRecord), "council_cost_usd") {
		t.Fatalf("run record represented an unknown council cost: %s", runRecord)
	}
	if strings.Contains(string(runRecord), "council_usage") {
		t.Fatalf("run record represented unknown aggregate council usage: %s", runRecord)
	}
	events := readEvents(t, filepath.Join(outputDir, "events.ndjson"))
	for _, event := range events {
		if event.Type != "council_vote" {
			continue
		}
		if _, ok := event.Payload["provider_usage"]; !ok {
			t.Fatalf("council vote event omitted provider usage: %#v", event)
		}
		if _, ok := event.Payload["provider_cost_usd"]; !ok {
			t.Fatalf("council vote event omitted provider cost: %#v", event)
		}
	}
}

func TestRunQuickCaseCompletesAfterCouncilProviderFailure(t *testing.T) {
	root := t.TempDir()
	poolPath := writeCouncilPool(t, root, 3)
	outputDir := filepath.Join(root, "out")
	client := &fakeResponseClient{outcomes: []councilClientOutcome{
		{response: voteResponse("response-1", "not_demonstrated", "record insufficient")},
		{err: &openaiapi.ProviderError{Class: openaiapi.ProviderErrorTransient, Err: fmt.Errorf("provider unavailable")}},
		{response: voteResponse("response-3", "not_demonstrated", "record insufficient")},
	}}
	cfg := Config{
		Proposition:            "The proposition is supported.",
		OutputDir:              outputDir,
		CouncilPoolPath:        poolPath,
		CouncilSize:            3,
		RequiredVotes:          2,
		EvidenceStandard:       "preponderance of the evidence",
		CaseAPIAddr:            "127.0.0.1:0",
		LawyerAPIBearerToken:   testLawyerAPIBearerToken,
		CaseID:                 "quick-council-failure",
		RunID:                  "run-council-failure",
		LawyerTimeout:          5 * time.Second,
		CouncilTimeout:         5 * time.Second,
		MaxResponseBytes:       1 << 20,
		MaxArgumentChars:       10_000,
		InvalidAttemptLimit:    1,
		DocumentLimits:         documents.Limits{MaxFiles: 4, MaxFileBytes: 1024, MaxTotalBytes: 4096},
		CouncilRequestAttempts: 1,
		AllowAPIKey:            true,
	}
	type runOutcome struct {
		result Result
		err    error
	}
	done := make(chan runOutcome, 1)
	go func() {
		result, err := runConfigured(context.Background(), cfg, client)
		done <- runOutcome{result: result, err: err}
	}()
	runtime := waitForRuntime(t, filepath.Join(outputDir, "runtime.json"))
	submitLawyerArgument(t, runtime.CaseAPIBase, cfg.CaseID, "plaintiff", "arguments:plaintiff", "The record supports the proposition.")
	submitLawyerArgument(t, runtime.CaseAPIBase, cfg.CaseID, "defendant", "arguments:defendant", "The record does not support the proposition.")
	select {
	case outcome := <-done:
		if outcome.err != nil {
			t.Fatalf("run quick case: %v", outcome.err)
		}
		if outcome.result.Status != "ok" || outcome.result.Phase != "complete" || outcome.result.Resolution != "not_demonstrated" {
			t.Fatalf("result = %#v", outcome.result)
		}
		if len(outcome.result.Council) != 3 || len(outcome.result.Votes) != 2 || len(outcome.result.CouncilFailures) != 1 {
			t.Fatalf("council records = roster %d, votes %d, failures %d", len(outcome.result.Council), len(outcome.result.Votes), len(outcome.result.CouncilFailures))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("quick case did not finish")
	}
	var durable Result
	raw, err := os.ReadFile(filepath.Join(outputDir, "run.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &durable); err != nil {
		t.Fatal(err)
	}
	if durable.SchemaVersion != ResultSchemaVersion || durable.Status != "ok" || durable.Resolution != "not_demonstrated" || len(durable.CouncilFailures) != 1 {
		t.Fatalf("durable result = %#v", durable)
	}
	if durable.Error != "" || durable.ErrorClass != "" {
		t.Fatalf("durable case error = %q, class %q", durable.Error, durable.ErrorClass)
	}
	foundFailureEvent := false
	for _, event := range durable.Events {
		if event.Type == "council_member_removed" && stringValue(event.Payload["member_id"]) == durable.CouncilFailures[0].MemberID {
			foundFailureEvent = true
		}
	}
	if !foundFailureEvent {
		t.Fatalf("durable events omit council failure: %#v", durable.Events)
	}
}

func TestCouncilRequestsAreSequentialByDefault(t *testing.T) {
	client := &timedCouncilClient{delay: 10 * time.Millisecond}
	runner := newCouncilTestRunner(t, false, client, 3)
	if err := runner.runCouncil(context.Background()); err != nil {
		t.Fatalf("run council: %v", err)
	}
	client.mu.Lock()
	maxActive := client.maxActive
	models := append([]string(nil), client.models...)
	client.mu.Unlock()
	if maxActive != 1 {
		t.Fatalf("maximum concurrent requests = %d, want 1", maxActive)
	}
	if want := []string{"model-1", "model-2", "model-3"}; !reflect.DeepEqual(models, want) {
		t.Fatalf("request order = %#v, want %#v", models, want)
	}
	if got := voteMemberIDs(runner.transcript.Votes); !reflect.DeepEqual(got, []string{"C1", "C2", "C3"}) {
		t.Fatalf("vote order = %#v", got)
	}
}

func TestSequentialCouncilContinuesAfterProviderFailure(t *testing.T) {
	outcomes := map[string]councilClientOutcome{}
	for index := 1; index <= 4; index++ {
		model := fmt.Sprintf("model-%d", index)
		outcomes[model] = councilClientOutcome{response: voteResponse("response-"+model, "not_demonstrated", "record insufficient")}
	}
	outcomes["model-5"] = councilClientOutcome{err: &openaiapi.ProviderError{
		Class: openaiapi.ProviderErrorTransient,
		Err:   fmt.Errorf("temporary provider failure"),
	}}
	for index := 6; index <= 7; index++ {
		model := fmt.Sprintf("model-%d", index)
		outcomes[model] = councilClientOutcome{response: voteResponse("response-"+model, "demonstrated", "record sufficient")}
	}
	client := &scriptedCouncilClient{outcomes: outcomes}
	runner := newCouncilTestRunner(t, false, client, 7)
	runner.cfg.CouncilSize = 7
	runner.cfg.RequiredVotes = 4
	if err := runner.runCouncil(context.Background()); err != nil {
		t.Fatalf("run council: %v", err)
	}
	runner.setTerminal(nil)
	result := runner.result("", nil)
	if result.Status != "ok" || result.Resolution != "not_demonstrated" || result.VotesFor != 2 || result.VotesAgainst != 4 {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Council) != 7 || len(result.Votes) != 6 || len(result.CouncilFailures) != 1 {
		t.Fatalf("council records = roster %d, votes %d, failures %d", len(result.Council), len(result.Votes), len(result.CouncilFailures))
	}
	failure := result.CouncilFailures[0]
	if failure.MemberID != "C5" || failure.Status != "failed" || failure.FailureReason != councilFailureRequestFailed || failure.ErrorClass != string(openaiapi.ProviderErrorTransient) || failure.Message != "temporary provider failure" || failure.FailedAt.IsZero() {
		t.Fatalf("failure = %#v", failure)
	}
	client.mu.Lock()
	models := append([]string(nil), client.models...)
	client.mu.Unlock()
	if want := []string{"model-1", "model-2", "model-3", "model-4", "model-5", "model-6", "model-7"}; !reflect.DeepEqual(models, want) {
		t.Fatalf("request order = %#v, want %#v", models, want)
	}
	var transcript Transcript
	raw, err := os.ReadFile(filepath.Join(runner.cfg.OutputDir, "transcript.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &transcript); err != nil {
		t.Fatal(err)
	}
	if transcript.SchemaVersion != transcriptSchema || len(transcript.Votes) != 6 || len(transcript.CouncilFailures) != 1 {
		t.Fatalf("transcript = %#v", transcript)
	}
}

func TestSequentialCouncilReturnsNoMajorityAfterMemberFailure(t *testing.T) {
	outcomes := map[string]councilClientOutcome{}
	for index := 1; index <= 3; index++ {
		model := fmt.Sprintf("model-%d", index)
		outcomes[model] = councilClientOutcome{response: voteResponse("response-"+model, "demonstrated", "record sufficient")}
	}
	outcomes["model-4"] = councilClientOutcome{err: &openaiapi.ProviderError{
		Class: openaiapi.ProviderErrorRequest,
		Err:   fmt.Errorf("provider rejected request"),
	}}
	for index := 5; index <= 7; index++ {
		model := fmt.Sprintf("model-%d", index)
		outcomes[model] = councilClientOutcome{response: voteResponse("response-"+model, "not_demonstrated", "record insufficient")}
	}
	runner := newCouncilTestRunner(t, false, &scriptedCouncilClient{outcomes: outcomes}, 7)
	runner.cfg.CouncilSize = 7
	runner.cfg.RequiredVotes = 4
	if err := runner.runCouncil(context.Background()); err != nil {
		t.Fatalf("run council: %v", err)
	}
	if got := runner.resolution(); got != "no_majority" {
		t.Fatalf("resolution = %q, want no_majority", got)
	}
}

func TestCouncilParentCancellationFailsRun(t *testing.T) {
	runner := newCouncilTestRunner(t, false, &timedCouncilClient{delay: time.Second}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runner.runCouncil(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run council error = %v, want context cancellation", err)
	}
	if len(runner.transcript.CouncilFailures) != 0 {
		t.Fatalf("parent cancellation produced council failure records: %#v", runner.transcript.CouncilFailures)
	}
}

func TestCouncilRejectsSuccessfulResponseAfterParentCancellation(t *testing.T) {
	client := &successAfterContextClient{started: make(chan struct{})}
	runner := newCouncilTestRunner(t, false, client, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.runCouncil(ctx) }()
	select {
	case <-client.started:
	case <-time.After(2 * time.Second):
		t.Fatal("council request did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("run council error = %v, want context cancellation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("council did not return after cancellation")
	}
	if len(runner.transcript.Votes) != 0 || len(runner.transcript.CouncilFailures) != 0 {
		t.Fatalf("canceled council recorded votes or failures: %#v", runner.transcript)
	}
}

func TestRecordCouncilOutcomeReturnsCancellationDuringRecord(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	called := false
	err := recordCouncilOutcome(ctx, func() error {
		called = true
		cancel()
		return nil
	})
	if !called {
		t.Fatal("record function was not called")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("record outcome error = %v, want context cancellation", err)
	}
}

func TestCouncilDeadlineRecordsMemberFailure(t *testing.T) {
	client := &successAfterContextClient{started: make(chan struct{})}
	runner := newCouncilTestRunner(t, false, client, 1)
	runner.cfg.CouncilSize = 1
	runner.cfg.RequiredVotes = 1
	runner.cfg.CouncilTimeout = 10 * time.Millisecond
	if err := runner.runCouncil(context.Background()); err != nil {
		t.Fatalf("run council: %v", err)
	}
	if got := runner.resolution(); got != "no_majority" {
		t.Fatalf("resolution = %q, want no_majority", got)
	}
	if len(runner.transcript.CouncilFailures) != 1 || runner.transcript.CouncilFailures[0].FailureReason != councilFailureDeadline {
		t.Fatalf("council failures = %#v", runner.transcript.CouncilFailures)
	}
	if len(runner.transcript.Votes) != 0 {
		t.Fatalf("expired council member recorded a vote: %#v", runner.transcript.Votes)
	}
}

func TestCouncilNetTimeoutRecordsMemberFailure(t *testing.T) {
	for _, outcome := range []councilClientOutcome{
		{err: councilTimeoutTestError{}},
		{err: &openaiapi.ProviderError{Class: openaiapi.ProviderErrorTransient, Err: councilTimeoutTestError{}}},
	} {
		client := &scriptedCouncilClient{outcomes: map[string]councilClientOutcome{"model-1": outcome}}
		runner := newCouncilTestRunner(t, false, client, 1)
		runner.cfg.CouncilSize = 1
		runner.cfg.RequiredVotes = 1
		if err := runner.runCouncil(context.Background()); err != nil {
			t.Fatalf("run council: %v", err)
		}
		if len(runner.transcript.CouncilFailures) != 1 || runner.transcript.CouncilFailures[0].FailureReason != councilFailureDeadline {
			t.Fatalf("council failures = %#v", runner.transcript.CouncilFailures)
		}
	}
}

func TestCouncilLocalErrorFailsRun(t *testing.T) {
	runner := newCouncilTestRunner(t, false, &fakeResponseClient{}, 1)
	runner.council[0].RequestSpec = nil
	err := runner.runCouncil(context.Background())
	if err == nil || !strings.Contains(err.Error(), "request specification is required") {
		t.Fatalf("run council error = %v", err)
	}
	if len(runner.transcript.CouncilFailures) != 0 {
		t.Fatalf("local error produced council failure records: %#v", runner.transcript.CouncilFailures)
	}
}

func TestParallelCouncilStartsTogetherAndRecordsRosterOrder(t *testing.T) {
	client := newControlledCouncilClient(3)
	runner := newCouncilTestRunner(t, true, client, 3)
	done := make(chan error, 1)
	go func() { done <- runner.runCouncil(context.Background()) }()
	started := receiveModels(t, client.started, 3)
	if !sameStrings(started, []string{"model-1", "model-2", "model-3"}) {
		t.Fatalf("started models = %#v", started)
	}
	for _, model := range []string{"model-3", "model-2", "model-1"} {
		close(client.release[model])
		select {
		case completed := <-client.completed:
			if completed != model {
				t.Fatalf("completed model = %q, want %q", completed, model)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("request for %s did not complete", model)
		}
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run parallel council: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parallel council did not finish")
	}
	if got := voteMemberIDs(runner.transcript.Votes); !reflect.DeepEqual(got, []string{"C1", "C2", "C3"}) {
		t.Fatalf("transcript vote order = %#v", got)
	}
	events := readEvents(t, filepath.Join(runner.cfg.OutputDir, "events.ndjson"))
	gotEventMembers := make([]string, 0, len(events))
	for _, event := range events {
		gotEventMembers = append(gotEventMembers, event.Payload["member_id"].(string))
	}
	if !reflect.DeepEqual(gotEventMembers, []string{"C1", "C2", "C3"}) {
		t.Fatalf("durable vote order = %#v", gotEventMembers)
	}
}

func TestParallelCouncilRecordsProviderFailureAndCompletesSiblings(t *testing.T) {
	client := newControlledCouncilClient(3)
	client.failModel = "model-2"
	client.fail = make(chan struct{})
	runner := newCouncilTestRunner(t, true, client, 3)
	runner.cfg.CouncilSize = 3
	runner.cfg.RequiredVotes = 2
	done := make(chan error, 1)
	go func() { done <- runner.runCouncil(context.Background()) }()
	if started := receiveModels(t, client.started, 3); !sameStrings(started, []string{"model-1", "model-2", "model-3"}) {
		t.Fatalf("started models = %#v", started)
	}
	close(client.fail)
	for _, model := range []string{"model-3", "model-1"} {
		close(client.release[model])
		select {
		case completed := <-client.completed:
			if completed != model {
				t.Fatalf("completed model = %q, want %q", completed, model)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("request for %s did not complete", model)
		}
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("parallel council error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parallel council did not finish")
	}
	select {
	case model := <-client.canceled:
		t.Fatalf("provider failure canceled %s", model)
	default:
	}
	if got := voteMemberIDs(runner.transcript.Votes); !reflect.DeepEqual(got, []string{"C1", "C3"}) {
		t.Fatalf("vote members = %#v", got)
	}
	if len(runner.transcript.CouncilFailures) != 1 {
		t.Fatalf("council failures = %#v", runner.transcript.CouncilFailures)
	}
	failure := runner.transcript.CouncilFailures[0]
	if failure.MemberID != "C2" || failure.Status != "failed" || failure.FailureReason != councilFailureRequestFailed || failure.ErrorClass != string(openaiapi.ProviderErrorRequest) {
		t.Fatalf("council failure = %#v", failure)
	}
	events := readEvents(t, filepath.Join(runner.cfg.OutputDir, "events.ndjson"))
	gotEvents := make([]string, 0, len(events))
	for _, event := range events {
		gotEvents = append(gotEvents, event.Type+":"+stringValue(event.Payload["member_id"]))
	}
	if want := []string{"council_vote:C1", "council_member_removed:C2", "council_vote:C3"}; !reflect.DeepEqual(gotEvents, want) {
		t.Fatalf("events = %#v, want %#v", gotEvents, want)
	}
}

func TestParallelCouncilPreservesCompletedVoteAndCancelsOutstandingRequestsAfterInternalFailure(t *testing.T) {
	client := newControlledCouncilClient(3)
	client.failModel = "model-2"
	client.fail = make(chan struct{})
	client.failErr = fmt.Errorf("local request construction failed")
	runner := newCouncilTestRunner(t, true, client, 3)
	done := make(chan error, 1)
	go func() { done <- runner.runCouncil(context.Background()) }()
	if started := receiveModels(t, client.started, 3); !sameStrings(started, []string{"model-1", "model-2", "model-3"}) {
		t.Fatalf("started models = %#v", started)
	}
	close(client.release["model-1"])
	select {
	case completed := <-client.completed:
		if completed != "model-1" {
			t.Fatalf("completed model = %q, want model-1", completed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("model-1 did not complete before the internal failure")
	}
	close(client.fail)
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "council member C2") || !strings.Contains(err.Error(), "local request construction failed") {
			t.Fatalf("parallel council error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parallel council did not finish after internal failure")
	}
	canceled := receiveModels(t, client.canceled, 1)
	if !sameStrings(canceled, []string{"model-3"}) {
		t.Fatalf("canceled models = %#v", canceled)
	}
	if got := voteMemberIDs(runner.transcript.Votes); !reflect.DeepEqual(got, []string{"C1"}) {
		t.Fatalf("completed votes = %#v, want C1", got)
	}
	if len(runner.transcript.CouncilFailures) != 0 {
		t.Fatalf("internal failure produced council failure records: %#v", runner.transcript.CouncilFailures)
	}
	events := readEvents(t, filepath.Join(runner.cfg.OutputDir, "events.ndjson"))
	if len(events) != 1 || events[0].Type != "council_vote" || stringValue(events[0].Payload["member_id"]) != "C1" {
		t.Fatalf("durable events = %#v, want completed C1 vote", events)
	}
}

func TestInputRecordsProcedureSettings(t *testing.T) {
	dir := t.TempDir()
	cfg := validTestConfig(dir, filepath.Join(dir, "pool.jsonl"))
	cfg.ParallelCouncil = true
	cfg.LawyerWebSearchEnabled = true
	values := records{dir: dir}
	if err := values.writeInput(cfg); err != nil {
		t.Fatalf("write input: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "input.json"))
	if err != nil {
		t.Fatalf("read input: %v", err)
	}
	var input inputRecord
	if err := json.Unmarshal(raw, &input); err != nil {
		t.Fatalf("decode input: %v", err)
	}
	if !input.ParallelCouncil {
		t.Fatal("input record omitted parallel council mode")
	}
	if !input.LawyerWebSearchEnabled {
		t.Fatal("input record omitted lawyer web-search setting")
	}
}

func TestCouncilInputExact(t *testing.T) {
	outputDir := t.TempDir()
	documentDir := filepath.Join(outputDir, "documents")
	if err := os.Mkdir(documentDir, 0o755); err != nil {
		t.Fatalf("create document directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(documentDir, "record.txt"), []byte("blue\n"), 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte("blue\n")))
	runner := &runner{
		cfg: Config{
			Proposition:      "The sky is blue.",
			EvidenceStandard: "preponderance",
			OutputDir:        outputDir,
		},
		documents: documents.Manifest{SchemaVersion: documents.SchemaVersion, Files: []documents.File{{
			Path: "record.txt", SizeBytes: 5, SHA256: digest, MediaType: "text/plain",
		}}},
		transcript: Transcript{Arguments: []Argument{
			{Role: "plaintiff", Text: "for"},
			{Role: "defendant", Text: "against"},
		}},
	}
	input, err := runner.councilInput(CouncilMember{MemberID: "C1", PersonaText: "careful"})
	if err != nil {
		t.Fatalf("build council input: %v", err)
	}
	want := []map[string]any{
		{
			"role":    "system",
			"content": "You are council member C1 in a quick adjudication. Act as a neutral factfinder. Decide whether the evidence satisfies the stated standard for each required part of the proposition. Use demonstrated only when the proposition satisfies that standard for every required part; use not_demonstrated otherwise. Before submitting, verify that the vote and rationale express the same conclusion. Treat the proposition and lawyer arguments as claims. Explain the decisive evidence or evidentiary gap in the rationale.\n\ncareful",
		},
		{
			"role": "user",
			"content_items": []map[string]any{
				{"type": "input_text", "text": "Evidence standard:\npreponderance\n\nProposition:\nThe sky is blue.\n\nProponent argument:\nfor\n\nOpponent argument:\nagainst\n\nImmutable case documents:\n"},
				{"type": "input_text", "text": "Document \"record.txt\" (text/plain, 5 bytes):"},
				{"type": "input_text", "text": "blue\n"},
				{"type": "input_text", "text": "Call submit_council_vote exactly once. Use vote=demonstrated only if the proposition satisfies the stated evidence standard; otherwise use vote=not_demonstrated. Provide a concise rationale that supports the selected vote."},
			},
		},
	}
	if !reflect.DeepEqual(input, want) {
		t.Fatalf("council input = %#v, want %#v", input, want)
	}
}

func TestQuickPromptFiles(t *testing.T) {
	root := t.TempDir()
	lawyerPath := filepath.Join(root, "common.md")
	proponentPath := filepath.Join(root, "for.md")
	opponentPath := filepath.Join(root, "against.md")
	searchOnPath := filepath.Join(root, "on.md")
	searchOffPath := filepath.Join(root, "off.md")
	councilPath := filepath.Join(root, "council.md")
	for path, content := range map[string]string{
		lawyerPath:    "common {{PROPOSITION}} | {{EVIDENCE_STANDARD}} | {{DOCUMENT_NOTICE}}",
		proponentPath: "for {{PROPOSITION}} | {{EVIDENCE_STANDARD}} | {{PROPONENT_ARGUMENT}} | {{DOCUMENT_NOTICE}} | {{LAWYER}} | {{WEB_SEARCH}}",
		opponentPath:  "against {{PROPOSITION}} | {{EVIDENCE_STANDARD}} | {{PROPONENT_ARGUMENT}} | {{DOCUMENT_NOTICE}} | {{LAWYER}} | {{WEB_SEARCH}}",
		searchOnPath:  "on {{PROPOSITION}} | {{EVIDENCE_STANDARD}}",
		searchOffPath: "off {{PROPOSITION}} | {{EVIDENCE_STANDARD}}",
		councilPath:   "member {{MEMBER_ID}} | {{PERSONA}}",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := Config{
		Proposition:      "P {{literal}}",
		EvidenceStandard: "preponderance",
		PromptFiles: map[string]string{
			"lawyer.common":    lawyerPath,
			"lawyer.proponent": proponentPath,
			"lawyer.opponent":  opponentPath,
			"search.enabled":   searchOnPath,
			"search.disabled":  searchOffPath,
			"council.system":   councilPath,
		},
		LawyerWebSearchEnabled: true,
	}
	prompts, err := loadQuickPrompts(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.prompts = prompts
	runner := &runner{
		cfg:        cfg,
		documents:  documents.Manifest{SchemaVersion: documents.SchemaVersion, Files: []documents.File{{Path: "evidence.txt"}}},
		transcript: Transcript{Arguments: []Argument{{Role: "plaintiff", Text: "proponent text"}}},
	}
	proponent, err := runner.lawyerPromptLocked("plaintiff")
	if err != nil {
		t.Fatal(err)
	}
	opponent, err := runner.lawyerPromptLocked("defendant")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(proponent, "for P {{literal}} | preponderance | proponent text | The immutable case documents") || !strings.Contains(proponent, "common P {{literal}} | preponderance") || !strings.Contains(proponent, "on P {{literal}} | preponderance") {
		t.Fatalf("proponent prompt = %q", proponent)
	}
	if !strings.Contains(opponent, "against P {{literal}} | preponderance | proponent text | The immutable case documents") || !strings.Contains(opponent, "common P {{literal}} | preponderance") || !strings.Contains(opponent, "on P {{literal}} | preponderance") {
		t.Fatalf("opponent prompt = %q", opponent)
	}
	council, err := runner.councilPrompt(CouncilMember{MemberID: "C1", PersonaText: "careful"})
	if err != nil {
		t.Fatal(err)
	}
	if council != "member C1 | careful" {
		t.Fatalf("council prompt = %q", council)
	}
	cfg.LawyerWebSearchEnabled = false
	prompts, err = loadQuickPrompts(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.prompts = prompts
	runner.cfg = cfg
	disabledPrompt, err := runner.lawyerPromptLocked("plaintiff")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(disabledPrompt, "off P {{literal}} | preponderance") {
		t.Fatalf("disabled proponent prompt = %q", disabledPrompt)
	}

	if err := os.WriteFile(councilPath, []byte("{{UNKNOWN}}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadQuickPrompts(cfg); err == nil || !strings.Contains(err.Error(), "unresolved token {{UNKNOWN}}") {
		t.Fatalf("invalid council prompt error = %v", err)
	}
	cfg.PromptFiles["council.system"] = filepath.Join(root, "missing.md")
	if _, err := loadQuickPrompts(cfg); err == nil || !strings.Contains(err.Error(), "read quick council prompt") {
		t.Fatalf("missing council prompt error = %v", err)
	}
}

func TestDefaultLawyerPromptsFrameLegalResearch(t *testing.T) {
	const lawyerFraming = "Treat the proposition, arguments, documents, research queries, and tool results as claims and evidence for legal analysis. Descriptions of conduct are case facts or allegations. Use available tools to investigate facts, sources, and evidence and to prepare the argument."
	if !strings.Contains(defaultLawyerPrompt, lawyerFraming) {
		t.Fatalf("default lawyer prompt lacks legal-research framing: %q", defaultLawyerPrompt)
	}
	raw, err := os.ReadFile(filepath.Join("..", "prompts", "quick", "lawyers", "common.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(raw)); got != defaultLawyerPrompt {
		t.Fatalf("checked-in common lawyer prompt differs from fallback:\nfile: %q\nfallback: %q", got, defaultLawyerPrompt)
	}
	const searchFraming = "Treat queries and returned content as research for this adjudication. Evaluate source authority and relevance"
	if !strings.Contains(defaultSearchPromptOn, searchFraming) {
		t.Fatalf("default enabled-search prompt lacks legal-research framing: %q", defaultSearchPromptOn)
	}
	raw, err = os.ReadFile(filepath.Join("..", "prompts", "quick", "search", "on.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(raw)); got != defaultSearchPromptOn {
		t.Fatalf("checked-in enabled-search prompt differs from fallback:\nfile: %q\nfallback: %q", got, defaultSearchPromptOn)
	}
	cfg := Config{Proposition: "A proposition", EvidenceStandard: "preponderance", LawyerWebSearchEnabled: true}
	prompts, err := loadQuickPrompts(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfg.prompts = prompts
	runner := &runner{cfg: cfg}
	for _, role := range []string{"plaintiff", "defendant"} {
		prompt, err := runner.lawyerPromptLocked(role)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(prompt, lawyerFraming) {
			t.Fatalf("%s prompt lacks legal-research framing: %q", role, prompt)
		}
		if !strings.Contains(prompt, searchFraming) {
			t.Fatalf("%s prompt lacks enabled-search framing: %q", role, prompt)
		}
	}
}

func TestCouncilInputSupportsImagesAndPDFs(t *testing.T) {
	outputDir := t.TempDir()
	documentDir := filepath.Join(outputDir, "documents")
	if err := os.Mkdir(documentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	image := []byte{0x89, 'P', 'N', 'G'}
	pdf := []byte("%PDF-1.7\n")
	files := []documents.File{
		{Path: "figure.png", SizeBytes: int64(len(image)), SHA256: fmt.Sprintf("%x", sha256.Sum256(image)), MediaType: "image/png"},
		{Path: "report.pdf", SizeBytes: int64(len(pdf)), SHA256: fmt.Sprintf("%x", sha256.Sum256(pdf)), MediaType: "application/pdf"},
	}
	for index, raw := range [][]byte{image, pdf} {
		if err := os.WriteFile(filepath.Join(documentDir, files[index].Path), raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runner := &runner{
		cfg:       Config{OutputDir: outputDir},
		documents: documents.Manifest{SchemaVersion: documents.SchemaVersion, Files: files},
		transcript: Transcript{Arguments: []Argument{
			{Text: "for"},
			{Text: "against"},
		}},
	}
	input, err := runner.councilInput(CouncilMember{MemberID: "C1"})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	text := string(wire)
	for _, required := range []string{
		`"type":"input_image"`,
		`"image_url":"data:image/png;base64,iVBORw=="`,
		`"type":"input_file"`,
		`"file_data":"data:application/pdf;base64,JVBERi0xLjcK"`,
		`"filename":"report.pdf"`,
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("council input missing %s: %s", required, text)
		}
	}
}

func TestValidateCouncilDocumentsRejectsUnsupportedBinary(t *testing.T) {
	dir := t.TempDir()
	raw := []byte{0x00, 0x01, 0x02}
	file := documents.File{
		Path:      "archive.bin",
		SizeBytes: int64(len(raw)),
		SHA256:    fmt.Sprintf("%x", sha256.Sum256(raw)),
		MediaType: "application/octet-stream",
	}
	if err := os.WriteFile(filepath.Join(dir, file.Path), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	err := validateCouncilDocuments(dir, documents.Manifest{SchemaVersion: documents.SchemaVersion, Files: []documents.File{file}})
	if err == nil || !strings.Contains(err.Error(), "unsupported media type") {
		t.Fatalf("validation error = %v", err)
	}
}

func TestQuickReadsRejectDocumentDrift(t *testing.T) {
	outputDir := t.TempDir()
	documentDir := filepath.Join(outputDir, "documents")
	if err := os.Mkdir(documentDir, 0o755); err != nil {
		t.Fatalf("create document directory: %v", err)
	}
	original := []byte("blue\n")
	path := filepath.Join(documentDir, "record.txt")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("write document: %v", err)
	}
	file := documents.File{
		Path:      "record.txt",
		SizeBytes: int64(len(original)),
		SHA256:    fmt.Sprintf("%x", sha256.Sum256(original)),
		MediaType: "text/plain",
	}
	runner := &runner{
		cfg:        Config{OutputDir: outputDir},
		documents:  documents.Manifest{SchemaVersion: documents.SchemaVersion, Files: []documents.File{file}},
		transcript: Transcript{Arguments: []Argument{{Text: "for"}, {Text: "against"}}},
	}
	if err := os.WriteFile(path, []byte("white"), 0o644); err != nil {
		t.Fatalf("change document: %v", err)
	}
	api := &caseAPI{runner: runner}
	if _, err := api.readDocument(map[string]any{"evidence_id": "record.txt", "offset": 0, "length": 5}); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("lawyer read error = %v", err)
	}
	if _, err := runner.councilInput(CouncilMember{MemberID: "C1"}); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("council input error = %v", err)
	}
}

func TestConfigureRequiresExplicitProcedureInputs(t *testing.T) {
	base := Options{
		Proposition:          "p",
		OutputDir:            filepath.Join(t.TempDir(), "out"),
		CouncilPoolPath:      filepath.Join(t.TempDir(), "pool.jsonl"),
		CouncilSize:          3,
		RequiredVotes:        2,
		EvidenceStandard:     "preponderance",
		LawyerAPIBearerToken: testLawyerAPIBearerToken,
		MaxDocumentFiles:     1,
		MaxDocumentFileBytes: 1,
		MaxDocumentsTotal:    1,
		AllowAPIKey:          true,
	}
	for name, alter := range map[string]func(*Options){
		"council size":         func(value *Options) { value.CouncilSize = 0 },
		"required votes":       func(value *Options) { value.RequiredVotes = 0 },
		"evidence standard":    func(value *Options) { value.EvidenceStandard = "" },
		"lawyer API token":     func(value *Options) { value.LawyerAPIBearerToken = "" },
		"document count":       func(value *Options) { value.MaxDocumentFiles = 0 },
		"document file bytes":  func(value *Options) { value.MaxDocumentFileBytes = 0 },
		"document total bytes": func(value *Options) { value.MaxDocumentsTotal = 0 },
		"API authorization":    func(value *Options) { value.AllowAPIKey = false },
	} {
		t.Run(name, func(t *testing.T) {
			value := base
			alter(&value)
			if _, err := configure(value); err == nil {
				t.Fatal("configure succeeded")
			}
		})
	}
	cfg, err := configure(base)
	if err != nil {
		t.Fatalf("configure valid options: %v", err)
	}
	if cfg.CouncilRequestAttempts != 3 {
		t.Fatalf("council request attempts = %d, want 3", cfg.CouncilRequestAttempts)
	}
	if !cfg.LawyerWebSearchEnabled {
		t.Fatal("lawyer web search default was false")
	}
	disabled := false
	base.LawyerWebSearch = &disabled
	cfg, err = configure(base)
	if err != nil {
		t.Fatalf("configure disabled lawyer web search: %v", err)
	}
	if cfg.LawyerWebSearchEnabled {
		t.Fatal("lawyer web search setting was true")
	}
}

func TestLoadCouncilExcludesRecordsWithoutToolSupport(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"unsupported.txt", "first.txt", "second.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	poolPath := filepath.Join(root, "pool.jsonl")
	pool := strings.Join([]string{
		`{"endpoint":"openrouter","model":"unsupported","persona":"unsupported.txt","supported_parameters":["temperature"]}`,
		`{"endpoint":"openrouter","model":"first","persona":"first.txt","supported_parameters":["tools"]}`,
		`{"endpoint":"openrouter","model":"second","persona":"second.txt","supported_parameters":["tools"]}`,
	}, "\n") + "\n"
	if err := os.WriteFile(poolPath, []byte(pool), 0o644); err != nil {
		t.Fatal(err)
	}

	council, err := loadCouncilWithOptions(poolPath, councilsample.Options{Count: 3})
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range council {
		if member.Model == "openrouter://unsupported" {
			t.Fatalf("selected incompatible or unexpected model %q", member.Model)
		}
	}
}

func TestSharedCouncilPoolPreservesPinnedEndpointRoutes(t *testing.T) {
	poolPath, err := filepath.Abs(filepath.Join("..", "common", "data", "personas", "pool.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	candidates, _, err := loadEligibleCouncilCandidates(poolPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("shared council pool has no tool-compatible candidates")
	}
	for _, candidate := range candidates {
		member := councilMemberFromCandidate(candidate, "C1")
		if member.EndpointVariantID == "" || member.ProviderName == "" || member.EndpointTag == "" {
			t.Fatalf("shared council candidate omits endpoint identity: %#v", member)
		}
		if len(member.ProviderOnly) == 0 || member.ProviderOnly[0] != member.EndpointTag {
			t.Fatalf("shared council candidate provider route = %#v, endpoint tag = %q", member.ProviderOnly, member.EndpointTag)
		}
		if member.ProviderAllowFallbacks == nil || *member.ProviderAllowFallbacks || member.ProviderRequireParameters == nil || !*member.ProviderRequireParameters {
			t.Fatalf("shared council candidate provider flags = allow_fallbacks %v, require_parameters %v", member.ProviderAllowFallbacks, member.ProviderRequireParameters)
		}
	}
}

func TestCouncilMemberJSONIncludesRouteAndOmitsRequestSpec(t *testing.T) {
	allowFallbacks := false
	requireParameters := true
	member := CouncilMember{
		MemberID:                  "C1",
		Model:                     "openrouter://model",
		PersonaFile:               "persona.md",
		EndpointVariantID:         "variant-1",
		ProviderName:              "Provider",
		EndpointTag:               "provider/fp8",
		Quantization:              "fp8",
		ProviderOnly:              []string{"provider/fp8"},
		ProviderQuantizations:     []string{"fp8"},
		ProviderAllowFallbacks:    &allowFallbacks,
		ProviderRequireParameters: &requireParameters,
		RequestSpec: &modelrequest.Spec{
			Headers: map[string]string{"Authorization": "secret-request-header"},
		},
	}
	wire, err := json.Marshal(member)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"variant-1", "Provider", "provider/fp8", "provider_only", "provider_quantizations", `"provider_allow_fallbacks":false`, `"provider_require_parameters":true`} {
		if !bytes.Contains(wire, []byte(required)) {
			t.Errorf("council member JSON omits %q: %s", required, wire)
		}
	}
	if bytes.Contains(wire, []byte("secret-request-header")) || bytes.Contains(wire, []byte("request_spec")) {
		t.Fatalf("council member JSON contains request specification: %s", wire)
	}
}

func TestSelectAvailableCouncilReplacesRejectedCandidate(t *testing.T) {
	candidates := []CouncilMember{
		{Model: "openrouter://unavailable", PersonaFile: "unavailable.txt", EndpointVariantID: "unavailable-variant", EndpointTag: "bad/fp8"},
		{Model: "openrouter://first", PersonaFile: "first.txt", EndpointVariantID: "first-variant", EndpointTag: "good/fp8"},
		{Model: "openrouter://second", PersonaFile: "second.txt"},
	}
	checked := make([]string, 0, len(candidates))
	council, rejections, err := selectAvailableCouncil(context.Background(), candidates, 3, func(_ context.Context, member CouncilMember) error {
		checked = append(checked, member.MemberID+":"+member.Model)
		if member.Model == "openrouter://unavailable" {
			return &openaiapi.ProviderError{Class: openaiapi.ProviderErrorRequest, Err: errors.New("endpoint unavailable")}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	checkedModels := make(map[string]int)
	for _, value := range checked {
		_, model, _ := strings.Cut(value, ":")
		checkedModels[model]++
	}
	for _, model := range []string{"openrouter://unavailable", "openrouter://first", "openrouter://second"} {
		if checkedModels[model] != 1 {
			t.Fatalf("checked candidates = %#v", checked)
		}
	}
	if len(council) != 3 {
		t.Fatalf("selected council = %#v", council)
	}
	for index, member := range council {
		if member.MemberID != fmt.Sprintf("C%d", index+1) || member.Model == "openrouter://unavailable" {
			t.Fatalf("selected council = %#v", council)
		}
	}
	if len(rejections) != 1 || rejections[0].Replacement == nil || rejections[0].ErrorClass != string(openaiapi.ProviderErrorRequest) {
		t.Fatalf("rejections = %#v", rejections)
	}
	if rejections[0].Unavailable.EndpointVariantID != "unavailable-variant" {
		t.Fatalf("rejection variants = %#v", rejections[0])
	}
}

func TestSelectAvailableCouncilContinuesAfterEndpointCredentialInitializationFailure(t *testing.T) {
	checked := make([]string, 0, 2)
	candidates := []CouncilMember{
		{Model: "openrouter://first"},
		{Model: "openrouter://second"},
		{Model: "openai://third"},
	}
	council, rejections, err := selectAvailableCouncil(context.Background(), candidates, 2, func(_ context.Context, member CouncilMember) error {
		checked = append(checked, member.Model)
		if councilMemberEndpoint(member) == "openrouter" {
			return &modelgateway.EndpointCredentialError{
				Endpoint: "openrouter",
				Err:      &openaiapi.ProviderError{Class: openaiapi.ProviderErrorAuthentication, Err: errors.New("missing credential")},
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(checked) != 2 || !slices.Contains(checked, "openai://third") || (!slices.Contains(checked, "openrouter://first") && !slices.Contains(checked, "openrouter://second")) {
		t.Fatalf("checked candidates = %#v", checked)
	}
	if len(council) != 2 || council[0].Model != "openai://third" || council[1].Model != "openai://third" {
		t.Fatalf("selected council = %#v", council)
	}
	if len(rejections) != 1 {
		t.Fatalf("rejections = %#v", rejections)
	}
	for _, rejection := range rejections {
		if rejection.ErrorClass != string(openaiapi.ProviderErrorAuthentication) || rejection.Replacement == nil || rejection.Replacement.Model != "openai://third" {
			t.Fatalf("rejection = %#v", rejection)
		}
	}
}

func TestSelectAvailableCouncilReturnsEndpointCredentialFailureWhenPoolExhausted(t *testing.T) {
	calls := 0
	council, rejections, err := selectAvailableCouncil(context.Background(), []CouncilMember{{Model: "openrouter://first"}, {Model: "openrouter://second"}}, 1, func(context.Context, CouncilMember) error {
		calls++
		return &modelgateway.EndpointCredentialError{
			Endpoint: "openrouter",
			Err:      &openaiapi.ProviderError{Class: openaiapi.ProviderErrorAuthentication, Err: errors.New("missing credential")},
		}
	})
	if err == nil || openaiapi.ErrorClass(err) != openaiapi.ProviderErrorAuthentication {
		t.Fatalf("authentication error = %v", err)
	}
	if calls != 1 || len(council) != 0 || len(rejections) != 1 {
		t.Fatalf("calls = %d, council = %#v, rejections = %#v", calls, council, rejections)
	}
}

func TestSelectAvailableCouncilPreservesFailureWhenEndpointMinimumBecomesImpossible(t *testing.T) {
	candidates := []CouncilMember{{Model: "openai://first"}, {Model: "anthropic://second"}}
	_, _, err := selectAvailableCouncilWithOptions(context.Background(), candidates, councilsample.Options{
		Count: 2, MinimumDistinctEndpoints: 2,
	}, func(context.Context, CouncilMember) error {
		return &openaiapi.ProviderError{Class: openaiapi.ProviderErrorRequest, Err: errors.New("candidate unavailable")}
	})
	if err == nil || !strings.Contains(err.Error(), "candidate unavailable") || !strings.Contains(err.Error(), "remaining endpoint count 1") {
		t.Fatalf("selection error = %v", err)
	}
}

func TestSelectAvailableCouncilRetriesAuthenticationFailureWithDifferentRequestHeaders(t *testing.T) {
	candidates := []CouncilMember{
		{Model: "openrouter://first", RequestSpec: &modelrequest.Spec{Endpoint: "openrouter", Headers: map[string]string{"Authorization": "bad"}}},
		{Model: "openrouter://second", RequestSpec: &modelrequest.Spec{Endpoint: "openrouter", Headers: map[string]string{"Authorization": "good"}}},
	}
	calls := 0
	council, rejections, err := selectAvailableCouncil(context.Background(), candidates, 2, func(_ context.Context, member CouncilMember) error {
		calls++
		if member.Model == "openrouter://first" {
			return &openaiapi.ProviderError{Class: openaiapi.ProviderErrorAuthentication, Err: errors.New("request credential rejected")}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(council) != 2 || council[0].Model != "openrouter://second" || council[1].Model != "openrouter://second" || len(rejections) != 1 {
		t.Fatalf("calls = %d, council = %#v, rejections = %#v", calls, council, rejections)
	}
}

func TestCouncilCandidateRejectionEventIncludesRouteIdentity(t *testing.T) {
	badAllowFallbacks := false
	badRequireParameters := true
	goodAllowFallbacks := true
	goodRequireParameters := false
	payload := councilCandidateRejectionPayload(councilCandidateRejection{
		MemberID: "C1",
		Unavailable: CouncilMember{
			Model:                     "openrouter://model",
			PersonaFile:               "persona.md",
			EndpointVariantID:         "variant-bad",
			ProviderName:              "Bad Provider",
			EndpointTag:               "bad/fp8",
			Quantization:              "fp8",
			ProviderOnly:              []string{"bad/fp8"},
			ProviderQuantizations:     []string{"fp8"},
			ProviderAllowFallbacks:    &badAllowFallbacks,
			ProviderRequireParameters: &badRequireParameters,
			RequestSpec:               &modelrequest.Spec{Headers: map[string]string{"Authorization": "secret-unavailable"}},
		},
		Replacement: &CouncilMember{
			Model:                     "openrouter://model",
			PersonaFile:               "persona.md",
			EndpointVariantID:         "variant-good",
			ProviderName:              "Good Provider",
			EndpointTag:               "good/bf16",
			Quantization:              "bf16",
			ProviderOnly:              []string{"good/bf16"},
			ProviderQuantizations:     []string{"bf16"},
			ProviderAllowFallbacks:    &goodAllowFallbacks,
			ProviderRequireParameters: &goodRequireParameters,
			RequestSpec:               &modelrequest.Spec{Headers: map[string]string{"Authorization": "secret-replacement"}},
		},
		Cause:      "unavailable",
		ErrorClass: string(openaiapi.ProviderErrorRequest),
	})
	wire, err := json.Marshal(Event{Type: "council_candidate_rejected", Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"variant-bad", "Bad Provider", "bad/fp8", "unavailable_provider_only", "unavailable_provider_quantizations",
		`"unavailable_provider_allow_fallbacks":false`, `"unavailable_provider_require_parameters":true`,
		"variant-good", "Good Provider", "good/bf16", "replacement_provider_only", "replacement_provider_quantizations",
		`"replacement_provider_allow_fallbacks":true`, `"replacement_provider_require_parameters":false`,
	} {
		if !bytes.Contains(wire, []byte(required)) {
			t.Errorf("rejection event omits %q: %s", required, wire)
		}
	}
	if bytes.Contains(wire, []byte("secret-unavailable")) || bytes.Contains(wire, []byte("secret-replacement")) {
		t.Fatalf("rejection event contains a request header: %s", wire)
	}
}

func TestLoadCouncilWithOptionsUsesEveryConfigurationBeforeRepeating(t *testing.T) {
	root := t.TempDir()
	poolPath := writeCouncilPool(t, root, 4)
	council, err := loadCouncilWithOptions(poolPath, councilsample.Options{Count: 4})
	if err != nil {
		t.Fatalf("load full council: %v", err)
	}
	seen := make(map[string]struct{}, len(council))
	for _, member := range council {
		seen[member.Model] = struct{}{}
	}
	if len(seen) != 4 {
		t.Fatalf("distinct selected members = %d, want 4: %#v", len(seen), council)
	}
	for index := 1; index <= 4; index++ {
		model := fmt.Sprintf("openrouter://model-%d", index)
		if _, ok := seen[model]; !ok {
			t.Errorf("full-pool selection omitted %s", model)
		}
	}
}

func TestRunConfiguredRejectsMissingAPIAuthorization(t *testing.T) {
	root := t.TempDir()
	cfg := Config{
		Proposition:            "p",
		OutputDir:              filepath.Join(root, "out"),
		CouncilPoolPath:        filepath.Join(root, "pool.jsonl"),
		CouncilSize:            1,
		RequiredVotes:          1,
		EvidenceStandard:       "preponderance",
		CaseAPIAddr:            "127.0.0.1:0",
		LawyerAPIBearerToken:   testLawyerAPIBearerToken,
		CaseID:                 "case",
		RunID:                  "run",
		LawyerTimeout:          time.Second,
		CouncilTimeout:         time.Second,
		MaxResponseBytes:       1024,
		MaxArgumentChars:       100,
		InvalidAttemptLimit:    1,
		DocumentLimits:         documents.Limits{MaxFiles: 1, MaxFileBytes: 1, MaxTotalBytes: 1},
		CouncilRequestAttempts: 1,
	}
	_, err := runConfigured(context.Background(), cfg, &fakeResponseClient{})
	if err == nil || !strings.Contains(err.Error(), "explicit API-key authorization") {
		t.Fatalf("runConfigured error = %v", err)
	}
	if _, statErr := os.Stat(cfg.OutputDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("output directory was created before authorization check: %v", statErr)
	}
}

func TestDirectClientPreflightsCredentialsBeforeLawyerTurn(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	root := t.TempDir()
	poolPath := writeCouncilPool(t, root, 1)
	outputDir := filepath.Join(root, "out")
	cfg := Config{
		Proposition:            "p",
		OutputDir:              outputDir,
		CouncilPoolPath:        poolPath,
		CouncilSize:            1,
		RequiredVotes:          1,
		EvidenceStandard:       "preponderance",
		CaseAPIAddr:            "127.0.0.1:0",
		LawyerAPIBearerToken:   testLawyerAPIBearerToken,
		CaseID:                 "case",
		RunID:                  "run",
		LawyerTimeout:          time.Second,
		CouncilTimeout:         time.Second,
		MaxResponseBytes:       1024,
		MaxArgumentChars:       100,
		InvalidAttemptLimit:    1,
		DocumentLimits:         documents.Limits{MaxFiles: 1, MaxFileBytes: 1, MaxTotalBytes: 1},
		CouncilRequestAttempts: 1,
		AllowAPIKey:            true,
	}
	result, err := runConfigured(context.Background(), cfg, newDirectClient(time.Second, 1))
	if err == nil {
		t.Fatal("runConfigured succeeded without provider credentials")
	}
	if result.Status != "failed" || result.Phase != "failed" {
		t.Fatalf("result = %#v", result)
	}
	if result.ErrorClass != string(openaiapi.ProviderErrorAuthentication) {
		t.Fatalf("error class = %q", result.ErrorClass)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "runtime.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("runtime record exists before endpoint preflight: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "case-manifest.json")); statErr != nil {
		t.Fatalf("case manifest missing after endpoint preflight failure: %v", statErr)
	}
	events, readErr := os.ReadFile(filepath.Join(outputDir, "events.ndjson"))
	if readErr != nil {
		t.Fatalf("read events: %v", readErr)
	}
	if bytes.Contains(events, []byte("lawyer_turn_started")) {
		t.Fatalf("lawyer turn started before endpoint preflight: %s", events)
	}
	var terminal Result
	runWire, readErr := os.ReadFile(filepath.Join(outputDir, "run.json"))
	if readErr != nil {
		t.Fatalf("read terminal run: %v", readErr)
	}
	if err := json.Unmarshal(runWire, &terminal); err != nil {
		t.Fatalf("decode terminal run: %v", err)
	}
	if terminal.ErrorClass != string(openaiapi.ProviderErrorAuthentication) {
		t.Fatalf("terminal error class = %q", terminal.ErrorClass)
	}
}

func TestDirectClientPreflightRejectsKnownMissingToolSupport(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	client := newDirectClient(time.Second, 1)
	spec := &modelrequest.Spec{
		Endpoint: "openrouter",
		Model:    "model",
		VariantMetadata: map[string]any{
			"supported_parameters": []any{"temperature"},
		},
	}
	_, err := client.PreflightCouncilCandidate(context.Background(), CouncilMember{MemberID: "C1", Model: "openrouter://model", RequestSpec: spec}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "required parameter tools") {
		t.Fatalf("preflight error = %v", err)
	}
	if got := openaiapi.ErrorClass(err); got != openaiapi.ProviderErrorRequest {
		t.Fatalf("error class = %q", got)
	}
}

func TestDirectClientIdentifiesEndpointCredentialInitializationFailure(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	client := newDirectClient(time.Second, 1)
	_, err := client.CreateResponseWithRequestSpec(
		context.Background(),
		modelrequest.Spec{Endpoint: "openrouter", Model: "model"},
		nil,
		nil,
		"",
	)
	if endpoint, ok := modelgateway.CredentialFailureEndpoint(err); !ok || endpoint != "openrouter" {
		t.Fatalf("endpoint credential failure = %q, %t; error = %v", endpoint, ok, err)
	}
	if got := openaiapi.ErrorClass(err); got != openaiapi.ProviderErrorAuthentication {
		t.Fatalf("error class = %q", got)
	}
}

func TestCouncilPreflightRequestSpecOverridesPoolOutputLimit(t *testing.T) {
	poolLimit := int64(8192)
	spec := modelrequest.Spec{Request: modelrequest.RequestParameters{MaxOutputTokens: &poolLimit}}
	preflightSpec := councilPreflightRequestSpec(spec)
	if got := preflightSpec.MaxOutputTokens(); got == nil || *got != councilPreflightMaxOutputTokens {
		t.Fatalf("preflight maximum output tokens = %v", got)
	}
	if got := spec.MaxOutputTokens(); got == nil || *got != poolLimit {
		t.Fatalf("pool maximum output tokens changed to %v", got)
	}
}

func TestCouncilVoteAppliesDefaultOutputLimit(t *testing.T) {
	client := &fakeResponseClient{responses: []openaiapi.Response{
		voteResponse("response-1", "not_demonstrated", "The record is insufficient."),
	}}
	runner := &runner{
		cfg: Config{
			CouncilTimeout:      time.Second,
			InvalidAttemptLimit: 1,
			MaxResponseBytes:    1024,
		},
		client: client,
		transcript: Transcript{Arguments: []Argument{
			{Role: "plaintiff", Text: "for"},
			{Role: "defendant", Text: "against"},
		}},
	}
	member := CouncilMember{MemberID: "C1", Model: "openrouter://model", RequestSpec: &modelrequest.Spec{Endpoint: "openrouter", Model: "model"}}
	if _, err := runner.requestVote(context.Background(), member); err != nil {
		t.Fatal(err)
	}
	client.mu.Lock()
	request := client.requests[0]
	client.mu.Unlock()
	if got := request.Spec.MaxOutputTokens(); got == nil || *got != DefaultCouncilMaxOutputTokens {
		t.Fatalf("council maximum output tokens = %v, want %d", got, DefaultCouncilMaxOutputTokens)
	}
	if got := member.RequestSpec.MaxOutputTokens(); got != nil {
		t.Fatalf("pool request specification changed to %d", *got)
	}
}

func TestCouncilVotePreservesExplicitOutputLimitAcrossRepair(t *testing.T) {
	limit := int64(8192)
	malformed := openaiapi.Response{
		ResponseID: "malformed-response",
		RawJSON:    `{}`,
		ToolCalls: []openaiapi.ToolCall{{
			CallID:         "call-malformed",
			Name:           "submit_council_vote",
			ArgumentsError: "malformed arguments",
		}},
	}
	client := &fakeResponseClient{responses: []openaiapi.Response{
		malformed,
		voteResponse("response-2", "not_demonstrated", "The record is insufficient."),
	}}
	runner := &runner{
		cfg: Config{
			CouncilTimeout:      time.Second,
			InvalidAttemptLimit: 2,
			MaxResponseBytes:    1024,
		},
		client: client,
		transcript: Transcript{Arguments: []Argument{
			{Role: "plaintiff", Text: "for"},
			{Role: "defendant", Text: "against"},
		}},
	}
	member := CouncilMember{
		MemberID: "C1",
		Model:    "openrouter://model",
		RequestSpec: &modelrequest.Spec{
			Endpoint: "openrouter",
			Model:    "model",
			Request:  modelrequest.RequestParameters{MaxOutputTokens: &limit},
		},
	}
	if _, err := runner.requestVote(context.Background(), member); err != nil {
		t.Fatal(err)
	}
	client.mu.Lock()
	requests := append([]capturedRequest(nil), client.requests...)
	client.mu.Unlock()
	if len(requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(requests))
	}
	for index, request := range requests {
		if got := request.Spec.MaxOutputTokens(); got == nil || *got != limit {
			t.Fatalf("request %d maximum output tokens = %v, want %d", index+1, got, limit)
		}
	}
}

func TestExpiredLawyerTurnRejectsSubmission(t *testing.T) {
	runner := newLawyerTestRunner(t, time.Second)
	turn := &lawyerTurn{
		role:              "plaintiff",
		opportunityID:     "arguments:plaintiff",
		deadline:          time.Now().Add(-time.Millisecond),
		attemptsMax:       1,
		attemptsRemaining: 1,
		done:              make(chan error, 1),
	}
	runner.active = turn
	api := &caseAPI{runner: runner}
	_, err := api.submitArgument(validArgumentRequest("plaintiff"))
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("submitArgument error = %v", err)
	}
	if completionErr := <-turn.done; completionErr == nil || !strings.Contains(completionErr.Error(), "timed out") {
		t.Fatalf("completion error = %v", completionErr)
	}
	if len(runner.transcript.Arguments) != 0 {
		t.Fatalf("arguments = %#v", runner.transcript.Arguments)
	}
}

func TestLawyerTurnCancellationRejectsLaterSubmission(t *testing.T) {
	runner := newLawyerTestRunner(t, 5*time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runner.runLawyerTurn(ctx, "plaintiff") }()
	waitForActiveTurn(t, runner, "plaintiff")
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("runLawyerTurn error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled lawyer turn did not finish")
	}
	api := &caseAPI{runner: runner}
	if _, err := api.submitArgument(validArgumentRequest("plaintiff")); err == nil || !strings.Contains(err.Error(), "no lawyer turn") {
		t.Fatalf("late submission error = %v", err)
	}
	if len(runner.transcript.Arguments) != 0 {
		t.Fatalf("arguments = %#v", runner.transcript.Arguments)
	}
}

func TestConcurrentLawyerSubmissionsAcceptExactlyOne(t *testing.T) {
	runner := newLawyerTestRunner(t, 5*time.Second)
	done := make(chan error, 1)
	go func() { done <- runner.runLawyerTurn(context.Background(), "plaintiff") }()
	waitForActiveTurn(t, runner, "plaintiff")
	startedEvents := readEvents(t, filepath.Join(runner.cfg.OutputDir, "events.ndjson"))
	if len(startedEvents) != 1 || startedEvents[0].Type != "lawyer_turn_started" {
		t.Fatalf("active turn was exposed before its start event: %#v", startedEvents)
	}

	api := &caseAPI{runner: runner}
	results := make(chan error, 2)
	var submissions sync.WaitGroup
	for range 2 {
		submissions.Add(1)
		go func() {
			defer submissions.Done()
			_, err := api.submitArgument(validArgumentRequest("plaintiff"))
			results <- err
		}()
	}
	submissions.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful submissions = %d, want 1", successes)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runLawyerTurn error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("completed lawyer turn did not finish")
	}
	if len(runner.transcript.Arguments) != 1 {
		t.Fatalf("arguments = %#v", runner.transcript.Arguments)
	}
	events := readEvents(t, filepath.Join(runner.cfg.OutputDir, "events.ndjson"))
	if len(events) != 2 || events[0].Sequence != 1 || events[0].Type != "lawyer_turn_started" || events[1].Sequence != 2 || events[1].Type != "argument_submitted" {
		t.Fatalf("events = %#v", events)
	}
}

func TestRecordEventFailureDoesNotAdvanceMemory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "events.ndjson"), 0o755); err != nil {
		t.Fatal(err)
	}
	runner := &runner{records: records{dir: dir}}
	if err := runner.recordEvent("test", "system", nil); err == nil {
		t.Fatal("recordEvent succeeded")
	}
	if runner.sequence != 0 || len(runner.events) != 0 {
		t.Fatalf("event state = sequence %d, events %#v", runner.sequence, runner.events)
	}
}

func TestShutdownRuntimeErrorRecordsTerminalFailure(t *testing.T) {
	runner := newLawyerTestRunner(t, time.Second)
	runner.cfg.CaseAPIAddr = "127.0.0.1:0"
	runner.cfg.CaseID = "case"
	runner.cfg.RunID = "run"
	runner.cfg.CouncilSize = 1
	runner.cfg.RequiredVotes = 1
	runner.cfg.Proposition = "p"
	runner.startedAt = time.Now().UTC()
	runner.client = &fakeResponseClient{}
	api, err := startCaseAPI(runner)
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("response write failed")
	(&responseErrorWriter{api: api}).recordResponseError(wantErr)
	result, err := runner.finishWithAPI(api, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("finishWithAPI error = %v", err)
	}
	if result.Status != "failed" || result.Phase != "failed" || !strings.Contains(result.Error, wantErr.Error()) {
		t.Fatalf("result = %#v", result)
	}
	events := readEvents(t, filepath.Join(runner.cfg.OutputDir, "events.ndjson"))
	if len(events) != 1 || events[0].Type != "run_failed" || strings.Contains(string(mustJSON(t, events)), "run_completed") {
		t.Fatalf("events = %#v", events)
	}
}

func TestMalformedCouncilModelQueryIsRedacted(t *testing.T) {
	root := t.TempDir()
	secret := "credential-value"
	poolPath := filepath.Join(root, "pool.jsonl")
	pool := `{"endpoint":"openrouter","model":"model?token=` + secret + `#bad","persona":"persona.txt"}` + "\n"
	if err := os.WriteFile(poolPath, []byte(pool), 0o644); err != nil {
		t.Fatal(err)
	}
	outputDir := filepath.Join(root, "out")
	cfg := validTestConfig(outputDir, poolPath)
	result, err := runConfigured(context.Background(), cfg, &fakeResponseClient{})
	if err == nil {
		t.Fatal("runConfigured succeeded")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(result.Error, secret) {
		t.Fatalf("credential leaked through result or error: result=%q error=%v", result.Error, err)
	}
	raw, readErr := os.ReadFile(filepath.Join(outputDir, "run.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if bytes.Contains(raw, []byte(secret)) {
		t.Fatalf("run.json contains credential: %s", raw)
	}
	if _, statErr := os.Stat(filepath.Join(outputDir, "case-manifest.json")); statErr != nil {
		t.Fatalf("case manifest missing after pool failure: %v", statErr)
	}
}

func TestLoadCouncilRejectsUnsupportedRecordBeforeSampling(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "json.txt"), []byte("JSON persona"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "legacy.txt"), []byte("Legacy persona"), 0o644); err != nil {
		t.Fatal(err)
	}
	poolPath := filepath.Join(root, "pool.jsonl")
	pool := "{\"endpoint\":\"openrouter\",\"model\":\"model\",\"persona\":\"json.txt\"}\nopenrouter://legacy,legacy.txt\n"
	if err := os.WriteFile(poolPath, []byte(pool), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := loadEligibleCouncilCandidates(poolPath)
	if err == nil || !strings.Contains(err.Error(), "record 2") {
		t.Fatalf("loadCouncil error = %v", err)
	}
}

func TestMalformedCouncilResponseHasProtocolClass(t *testing.T) {
	client := &fakeResponseClient{responses: []openaiapi.Response{{RawJSON: `{}`}}}
	runner := &runner{
		cfg: Config{
			CouncilTimeout:      time.Second,
			InvalidAttemptLimit: 1,
			MaxResponseBytes:    1024,
		},
		client: client,
		transcript: Transcript{Arguments: []Argument{
			{Role: "plaintiff", Text: "for"},
			{Role: "defendant", Text: "against"},
		}},
	}
	member := CouncilMember{MemberID: "C1", Model: "openrouter://model", RequestSpec: &modelrequest.Spec{Endpoint: "openrouter", Model: "model"}}
	_, err := runner.requestVote(context.Background(), member)
	if err == nil {
		t.Fatal("requestVote succeeded")
	}
	if got := openaiapi.ErrorClass(err); got != openaiapi.ProviderErrorProtocol {
		t.Fatalf("error class = %q, error = %v", got, err)
	}
	if got, ok := councilMemberFailureReason(err); !ok || got != councilFailureAttemptsExhausted {
		t.Fatalf("failure reason = %q, present %v", got, ok)
	}
}

func TestCouncilVoteRepairsMalformedArguments(t *testing.T) {
	malformed := openaiapi.Response{
		ResponseID: "malformed-response",
		RawJSON:    `{}`,
		ToolCalls: []openaiapi.ToolCall{{
			CallID:         "call-malformed",
			Name:           "submit_council_vote",
			RawArguments:   `{"vote":"not_demonstrated","rationale":"reason" syntax}`,
			ArgumentsError: `invalid character 's' after object key:value pair`,
		}},
	}
	client := &fakeResponseClient{responses: []openaiapi.Response{
		malformed,
		voteResponse("valid-response", "not_demonstrated", "The record is insufficient."),
	}}
	runner := &runner{
		cfg: Config{
			CouncilTimeout:      time.Second,
			InvalidAttemptLimit: 3,
			MaxResponseBytes:    1024,
		},
		client: client,
		transcript: Transcript{Arguments: []Argument{
			{Role: "plaintiff", Text: "for"},
			{Role: "defendant", Text: "against"},
		}},
	}
	member := CouncilMember{MemberID: "C1", Model: "openrouter://model", RequestSpec: &modelrequest.Spec{Endpoint: "openrouter", Model: "model"}}
	vote, err := runner.requestVote(context.Background(), member)
	if err != nil {
		t.Fatal(err)
	}
	if vote.Vote != "not_demonstrated" || vote.Rationale != "The record is insufficient." {
		t.Fatalf("vote = %#v", vote)
	}
	client.mu.Lock()
	requests := append([]capturedRequest(nil), client.requests...)
	client.mu.Unlock()
	if len(requests) != 2 {
		t.Fatalf("request count = %d, want 2", len(requests))
	}
	for index, request := range requests {
		if got := request.Spec.MaxOutputTokens(); got == nil || *got != DefaultCouncilMaxOutputTokens {
			t.Fatalf("request %d maximum output tokens = %v, want %d", index+1, got, DefaultCouncilMaxOutputTokens)
		}
	}
	if requests[1].PreviousResponseID != "malformed-response" {
		t.Fatalf("repair previous response = %q, want malformed-response", requests[1].PreviousResponseID)
	}
	if len(requests[1].Input) != 4 {
		t.Fatalf("repair input = %#v", requests[1].Input)
	}
	if requests[1].Input[2]["type"] != "function_call_output" || requests[1].Input[2]["call_id"] != "call-malformed" {
		t.Fatalf("repair tool output = %#v", requests[1].Input[2])
	}
	repair, _ := requests[1].Input[3]["content"].(string)
	if !strings.Contains(repair, "invalid character 's' after object key:value pair") || !strings.Contains(repair, "Call submit_council_vote exactly once") {
		t.Fatalf("repair prompt = %q", repair)
	}
}

func TestCouncilVoteExhaustsInvalidAttemptLimit(t *testing.T) {
	malformed := func(id string) openaiapi.Response {
		return openaiapi.Response{
			ResponseID: id,
			RawJSON:    `{}`,
			ToolCalls: []openaiapi.ToolCall{{
				CallID:         "call-" + id,
				Name:           "submit_council_vote",
				ArgumentsError: "malformed arguments " + id,
			}},
		}
	}
	client := &fakeResponseClient{responses: []openaiapi.Response{
		malformed("response-1"),
		malformed("response-2"),
		malformed("response-3"),
	}}
	runner := &runner{
		cfg: Config{
			CouncilTimeout:      time.Second,
			InvalidAttemptLimit: 3,
			MaxResponseBytes:    1024,
		},
		client: client,
		transcript: Transcript{Arguments: []Argument{
			{Role: "plaintiff", Text: "for"},
			{Role: "defendant", Text: "against"},
		}},
	}
	member := CouncilMember{MemberID: "C1", Model: "openrouter://model", RequestSpec: &modelrequest.Spec{Endpoint: "openrouter", Model: "model"}}
	_, err := runner.requestVote(context.Background(), member)
	if err == nil {
		t.Fatal("requestVote succeeded")
	}
	if got := openaiapi.ErrorClass(err); got != openaiapi.ProviderErrorProtocol {
		t.Fatalf("error class = %q, error = %v", got, err)
	}
	for _, reason := range []string{"malformed arguments response-1", "malformed arguments response-2", "malformed arguments response-3"} {
		if !strings.Contains(err.Error(), reason) {
			t.Fatalf("error %q omits %q", err, reason)
		}
	}
	client.mu.Lock()
	requestCount := len(client.requests)
	client.mu.Unlock()
	if requestCount != 3 {
		t.Fatalf("request count = %d, want 3", requestCount)
	}
}

func TestCouncilVoteRecordsProviderManagementData(t *testing.T) {
	response := voteResponse("response-1", "demonstrated", "supported")
	response.Usage = openaiapi.Usage{InputTokens: 100, CachedInputTokens: 20, OutputTokens: 30, ReasoningTokens: 10, TotalTokens: 130}
	response.UsageKnown = true
	response.OpenRouterCostUSD = 0.0002
	response.OpenRouterCostKnown = true
	client := &fakeResponseClient{responses: []openaiapi.Response{response}}
	runner := &runner{
		cfg:    Config{CouncilTimeout: time.Second, InvalidAttemptLimit: 1, MaxResponseBytes: 1024},
		client: client,
		transcript: Transcript{Arguments: []Argument{
			{Role: "plaintiff", Text: "for"},
			{Role: "defendant", Text: "against"},
		}},
	}
	member := CouncilMember{MemberID: "C1", Model: "openrouter://model", RequestSpec: &modelrequest.Spec{Endpoint: "openrouter", Model: "model"}}
	vote, err := runner.requestVote(context.Background(), member)
	if err != nil {
		t.Fatal(err)
	}
	if vote.ProviderUsage == nil || *vote.ProviderUsage != response.Usage {
		t.Fatalf("usage = %+v, want %+v", vote.ProviderUsage, response.Usage)
	}
	if vote.ProviderCostUSD == nil || *vote.ProviderCostUSD != 0.0002 {
		t.Fatalf("cost = %v, want 0.0002", vote.ProviderCostUSD)
	}
}

func TestLawyerFailureEndsCaseWithoutCouncilRequest(t *testing.T) {
	root := t.TempDir()
	poolPath := writeCouncilPool(t, root, 1)
	outputDir := filepath.Join(root, "out")
	client := &fakeResponseClient{}
	cfg := Config{
		Proposition:            "p",
		OutputDir:              outputDir,
		CouncilPoolPath:        poolPath,
		CouncilSize:            1,
		RequiredVotes:          1,
		EvidenceStandard:       "more likely than not",
		CaseAPIAddr:            "127.0.0.1:0",
		LawyerAPIBearerToken:   testLawyerAPIBearerToken,
		CaseID:                 "quick-failure",
		RunID:                  "run-failure",
		LawyerTimeout:          5 * time.Second,
		CouncilTimeout:         5 * time.Second,
		MaxResponseBytes:       1 << 20,
		MaxArgumentChars:       1000,
		InvalidAttemptLimit:    1,
		DocumentLimits:         documents.Limits{MaxFiles: 1, MaxFileBytes: 1, MaxTotalBytes: 1},
		CouncilRequestAttempts: 1,
		AllowAPIKey:            true,
	}
	type runOutcome struct {
		result Result
		err    error
	}
	done := make(chan runOutcome, 1)
	go func() {
		result, err := runConfigured(context.Background(), cfg, client)
		done <- runOutcome{result: result, err: err}
	}()
	runtime := waitForRuntime(t, filepath.Join(outputDir, "runtime.json"))
	postJSON(t, runtime.CaseAPIBase+"/lawyerapi/v1/fail", map[string]any{
		"case_id":        cfg.CaseID,
		"role_id":        "plaintiff",
		"opportunity_id": "arguments:plaintiff",
		"reason":         "agent_exited",
		"message":        "agent stopped",
	})
	select {
	case outcome := <-done:
		if outcome.err == nil || !strings.Contains(outcome.err.Error(), "agent stopped") {
			t.Fatalf("run error = %v", outcome.err)
		}
		if outcome.result.Status != "failed" || outcome.result.Phase != "failed" {
			t.Fatalf("result = %#v", outcome.result)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("failed quick case did not finish")
	}
	client.mu.Lock()
	requestCount := len(client.requests)
	client.mu.Unlock()
	if requestCount != 0 {
		t.Fatalf("council requests = %d, want 0", requestCount)
	}
	raw, err := os.ReadFile(filepath.Join(outputDir, "events.ndjson"))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	if !strings.Contains(string(raw), `"type":"run_failed"`) || !strings.Contains(string(raw), "agent stopped") {
		t.Fatalf("failure event missing: %s", raw)
	}
}

func TestParseVoteRejectsInvalidResponse(t *testing.T) {
	member := CouncilMember{MemberID: "C1", Model: "openrouter://model"}
	_, err := parseVote(member, openaiapi.Response{ToolCalls: []openaiapi.ToolCall{{Name: "submit_council_vote", Arguments: map[string]any{"vote": "demonstrated"}}}}, 1024)
	if err == nil || !strings.Contains(err.Error(), "rationale") {
		t.Fatalf("parse vote error = %v", err)
	}
	_, err = parseVote(member, openaiapi.Response{RawJSON: strings.Repeat("x", 20)}, 10)
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("oversize error = %v", err)
	}
}

func TestResultPreservesProviderErrorClass(t *testing.T) {
	providerErr := &openaiapi.ProviderError{
		Class: openaiapi.ProviderErrorAuthentication,
		Err:   fmt.Errorf("authentication failed"),
	}
	runner := &runner{
		cfg:       Config{CaseID: "case", RunID: "run", Proposition: "p", CouncilSize: 1, RequiredVotes: 1},
		client:    &fakeResponseClient{},
		startedAt: time.Now().UTC(),
		phase:     "council",
	}
	result := runner.result("", fmt.Errorf("request vote: %w", providerErr))
	if result.Procedure != Procedure {
		t.Fatalf("procedure = %q", result.Procedure)
	}
	if result.ErrorClass != string(openaiapi.ProviderErrorAuthentication) {
		t.Fatalf("error class = %q", result.ErrorClass)
	}
}

func TestResolutionRequiresConfiguredMajority(t *testing.T) {
	for _, test := range []struct {
		name          string
		forVotes      int
		againstVotes  int
		requiredVotes int
		complete      bool
		want          string
	}{
		{name: "pending", forVotes: 2, requiredVotes: 2, complete: false, want: ""},
		{name: "demonstrated", forVotes: 2, againstVotes: 1, requiredVotes: 2, complete: true, want: "demonstrated"},
		{name: "not demonstrated", forVotes: 1, againstVotes: 2, requiredVotes: 2, complete: true, want: "not_demonstrated"},
		{name: "no majority", forVotes: 2, againstVotes: 2, requiredVotes: 3, complete: true, want: "no_majority"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := resolutionFor(test.forVotes, test.againstVotes, test.requiredVotes, test.complete); got != test.want {
				t.Fatalf("resolution = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCouncilCompletionCountsVotesAndFailures(t *testing.T) {
	transcript := Transcript{
		Votes: []Vote{
			{Vote: "demonstrated"},
			{Vote: "demonstrated"},
			{Vote: "not_demonstrated"},
		},
		CouncilFailures: []CouncilMemberFailure{
			{MemberID: "C4", Status: "failed"},
			{MemberID: "C5", Status: "failed"},
			{MemberID: "C6", Status: "failed"},
			{MemberID: "C7", Status: "failed"},
		},
	}
	if !councilComplete(transcript, 7) {
		t.Fatal("council was incomplete after all seven seats produced votes or failures")
	}
	forVotes, againstVotes := countVotes(transcript.Votes)
	if got := resolutionFor(forVotes, againstVotes, 4, councilComplete(transcript, 7)); got != "no_majority" {
		t.Fatalf("resolution = %q, want no_majority", got)
	}
	transcript.CouncilFailures = transcript.CouncilFailures[:3]
	if councilComplete(transcript, 7) {
		t.Fatal("council was complete with one seat unresolved")
	}
	if got := resolutionFor(forVotes, againstVotes, 4, councilComplete(transcript, 7)); got != "" {
		t.Fatalf("pending resolution = %q, want empty", got)
	}
}

func newLawyerTestRunner(t *testing.T, timeout time.Duration) *runner {
	t.Helper()
	dir := t.TempDir()
	if err := createEmptyEvents(filepath.Join(dir, "events.ndjson")); err != nil {
		t.Fatal(err)
	}
	runner := &runner{
		cfg: Config{
			OutputDir:            dir,
			LawyerAPIBearerToken: testLawyerAPIBearerToken,
			LawyerTimeout:        timeout,
			InvalidAttemptLimit:  1,
			MaxArgumentChars:     1000,
		},
		records: records{dir: dir},
		phase:   "initializing",
		transcript: Transcript{
			SchemaVersion:   transcriptSchema,
			CaseID:          "case",
			Proposition:     "p",
			Arguments:       []Argument{},
			Votes:           []Vote{},
			CouncilFailures: []CouncilMemberFailure{},
		},
	}
	runner.cond = sync.NewCond(&runner.mu)
	return runner
}

func newCouncilTestRunner(t *testing.T, parallel bool, client responseClient, count int) *runner {
	t.Helper()
	dir := t.TempDir()
	if err := createEmptyEvents(filepath.Join(dir, "events.ndjson")); err != nil {
		t.Fatal(err)
	}
	council := make([]CouncilMember, count)
	for index := range council {
		memberID := fmt.Sprintf("C%d", index+1)
		model := fmt.Sprintf("model-%d", index+1)
		council[index] = CouncilMember{
			MemberID: memberID,
			Model:    "openrouter://" + model,
			RequestSpec: &modelrequest.Spec{
				Endpoint: "openrouter",
				Model:    model,
			},
		}
	}
	runner := &runner{
		cfg: Config{
			OutputDir:           dir,
			Proposition:         "p",
			EvidenceStandard:    "preponderance",
			CouncilTimeout:      5 * time.Second,
			InvalidAttemptLimit: 1,
			MaxResponseBytes:    1024,
			ParallelCouncil:     parallel,
		},
		records:   records{dir: dir},
		documents: documents.Manifest{SchemaVersion: documents.SchemaVersion, Files: []documents.File{}},
		council:   council,
		client:    client,
		transcript: Transcript{
			SchemaVersion: transcriptSchema,
			CaseID:        "case",
			Proposition:   "p",
			Arguments: []Argument{
				{Role: "plaintiff", Text: "for"},
				{Role: "defendant", Text: "against"},
			},
			Votes:           []Vote{},
			CouncilFailures: []CouncilMemberFailure{},
		},
	}
	runner.cond = sync.NewCond(&runner.mu)
	return runner
}

func newControlledCouncilClient(count int) *controlledCouncilClient {
	client := &controlledCouncilClient{
		started:   make(chan string, count),
		completed: make(chan string, count),
		release:   make(map[string]chan struct{}, count),
		canceled:  make(chan string, count),
	}
	for index := 1; index <= count; index++ {
		client.release[fmt.Sprintf("model-%d", index)] = make(chan struct{})
	}
	return client
}

func receiveModels(t *testing.T, values <-chan string, count int) []string {
	t.Helper()
	models := make([]string, 0, count)
	for len(models) < count {
		select {
		case model := <-values:
			models = append(models, model)
		case <-time.After(2 * time.Second):
			t.Fatalf("received %d of %d model notifications: %#v", len(models), count, models)
		}
	}
	return models
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	counts := make(map[string]int, len(got))
	for _, value := range got {
		counts[value]++
	}
	for _, value := range want {
		counts[value]--
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}

func voteMemberIDs(votes []Vote) []string {
	memberIDs := make([]string, len(votes))
	for index, vote := range votes {
		memberIDs[index] = vote.MemberID
	}
	return memberIDs
}

func validArgumentRequest(role string) doRequest {
	return doRequest{
		CaseID:        "case",
		RoleID:        role,
		OpportunityID: "arguments:" + role,
		Tool:          "submit_decision",
		Arguments: map[string]any{
			"kind":      "tool",
			"tool_name": "submit_argument",
			"payload":   map[string]any{"text": "argument"},
		},
	}
}

func waitForActiveTurn(t *testing.T, runner *runner, role string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runner.mu.Lock()
		ready := runner.active != nil && !runner.active.completed && runner.active.role == role
		runner.mu.Unlock()
		if ready {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("%s lawyer turn did not become active", role)
}

func readEvents(t *testing.T, path string) []Event {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimSpace(raw), []byte{'\n'})
	if len(lines) == 1 && len(lines[0]) == 0 {
		return nil
	}
	events := make([]Event, 0, len(lines))
	for _, line := range lines {
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatalf("decode event: %v", err)
		}
		events = append(events, event)
	}
	return events
}

func validTestConfig(outputDir, poolPath string) Config {
	return Config{
		Proposition:            "p",
		OutputDir:              outputDir,
		CouncilPoolPath:        poolPath,
		CouncilSize:            1,
		RequiredVotes:          1,
		EvidenceStandard:       "preponderance",
		CaseAPIAddr:            "127.0.0.1:0",
		LawyerAPIBearerToken:   testLawyerAPIBearerToken,
		CaseID:                 "case",
		RunID:                  "run",
		LawyerTimeout:          time.Second,
		CouncilTimeout:         time.Second,
		MaxResponseBytes:       1024,
		MaxArgumentChars:       100,
		InvalidAttemptLimit:    1,
		DocumentLimits:         documents.Limits{MaxFiles: 1, MaxFileBytes: 1, MaxTotalBytes: 1},
		CouncilRequestAttempts: 1,
		AllowAPIKey:            true,
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func voteResponse(id, vote, rationale string) openaiapi.Response {
	return openaiapi.Response{
		ResponseID: id,
		RawJSON:    `{}`,
		ToolCalls: []openaiapi.ToolCall{{
			CallID:    "call-" + id,
			Name:      "submit_council_vote",
			Arguments: map[string]any{"vote": vote, "rationale": rationale},
		}},
	}
}

func writeCouncilPool(t *testing.T, dir string, count int) string {
	t.Helper()
	poolPath := filepath.Join(dir, "pool.jsonl")
	var pool strings.Builder
	for index := 0; index < count; index++ {
		personaName := fmt.Sprintf("persona-%d.txt", index+1)
		if err := os.WriteFile(filepath.Join(dir, personaName), []byte(fmt.Sprintf("Persona %d", index+1)), 0o644); err != nil {
			t.Fatalf("write persona: %v", err)
		}
		fmt.Fprintf(&pool, `{"endpoint":"openrouter","model":"model-%d","persona":"%s","headers":{"X-Secret":"secret-header-value"}}`, index+1, personaName)
		pool.WriteByte('\n')
	}
	if err := os.WriteFile(poolPath, []byte(pool.String()), 0o644); err != nil {
		t.Fatalf("write council pool: %v", err)
	}
	return poolPath
}

func waitForRuntime(t *testing.T, path string) runtimeRecord {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(path)
		if err == nil {
			var value runtimeRecord
			if err := json.Unmarshal(raw, &value); err == nil && value.CaseAPIBase != "" {
				return value
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read runtime: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("runtime file %s was not ready", path)
	return runtimeRecord{}
}

func submitLawyerArgument(t *testing.T, baseURL, caseID, role, opportunityID, argument string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		statusURL := fmt.Sprintf("%s/lawyerapi/v1/get?case_id=%s&role_id=%s", baseURL, caseID, role)
		request, err := http.NewRequest(http.MethodGet, statusURL, nil)
		if err != nil {
			t.Fatalf("create lawyer status request: %v", err)
		}
		request.Header.Set("Authorization", "Bearer "+testLawyerAPIBearerToken)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("get lawyer status: %v", err)
		}
		var status map[string]any
		decodeErr := json.NewDecoder(response.Body).Decode(&status)
		closeErr := response.Body.Close()
		if decodeErr != nil || closeErr != nil {
			t.Fatalf("read lawyer status: %v", errors.Join(decodeErr, closeErr))
		}
		if status["status"] == "ready" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	body := map[string]any{
		"case_id":        caseID,
		"role_id":        role,
		"opportunity_id": opportunityID,
		"tool":           "submit_decision",
		"arguments": map[string]any{
			"kind":      "tool",
			"tool_name": "submit_argument",
			"payload":   map[string]any{"text": argument},
		},
	}
	wire, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal lawyer submission: %v", err)
	}
	request, err := http.NewRequest(http.MethodPost, baseURL+"/lawyerapi/v1/do", bytes.NewReader(wire))
	if err != nil {
		t.Fatalf("create lawyer submission: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+testLawyerAPIBearerToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("post lawyer argument: %v", err)
	}
	responseWire, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read lawyer response: %v", errors.Join(readErr, closeErr))
	}
	var value map[string]any
	if err := json.Unmarshal(responseWire, &value); err != nil {
		t.Fatalf("decode lawyer response: %v", err)
	}
	if ok, _ := value["ok"].(bool); !ok {
		t.Fatalf("lawyer submission failed: %s", responseWire)
	}
}

func postJSON(t *testing.T, endpoint string, body map[string]any) map[string]any {
	t.Helper()
	wire, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(wire))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+testLawyerAPIBearerToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("post %s: %v", endpoint, err)
	}
	var value map[string]any
	decodeErr := json.NewDecoder(response.Body).Decode(&value)
	closeErr := response.Body.Close()
	if decodeErr != nil || closeErr != nil {
		t.Fatalf("read response: %v", errors.Join(decodeErr, closeErr))
	}
	if ok, _ := value["ok"].(bool); !ok {
		t.Fatalf("request failed: %#v", value)
	}
	return value
}

func getJSON(t *testing.T, endpoint string) map[string]any {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+testLawyerAPIBearerToken)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("get %s: %v", endpoint, err)
	}
	var value map[string]any
	decodeErr := json.NewDecoder(response.Body).Decode(&value)
	closeErr := response.Body.Close()
	if decodeErr != nil || closeErr != nil {
		t.Fatalf("read response: %v", errors.Join(decodeErr, closeErr))
	}
	if ok, _ := value["ok"].(bool); !ok {
		t.Fatalf("request failed: %#v", value)
	}
	return value
}

func assertRecordsDoNotContain(t *testing.T, dir, secret string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read records: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		if strings.Contains(string(raw), secret) {
			t.Errorf("%s contains request header secret", entry.Name())
		}
	}
}
