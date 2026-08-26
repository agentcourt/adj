package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/agentcourt/adj/common/mcpbridge"
	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
	"github.com/agentcourt/adj/internal/pimodel"
)

const (
	directMaxOutputTokens = 1024
	piMaxOutputTokens     = 4096
	piMCPServer           = "aar"
)

type options struct {
	specPath      string
	outputDir     string
	directTimeout time.Duration
	piTimeout     time.Duration
	maxAttempts   int
	podmanCommand string
	piImage       string
	piMCPAdapter  string
	piMCPHost     string
}

type checkResult struct {
	Status               string             `json:"status"`
	InfrastructureError  bool               `json:"infrastructure_error,omitempty"`
	Error                string             `json:"error,omitempty"`
	ErrorClass           string             `json:"error_class,omitempty"`
	ElapsedSeconds       float64            `json:"elapsed_seconds"`
	ResponseID           string             `json:"response_id,omitempty"`
	ResponseIDs          []string           `json:"response_ids,omitempty"`
	Vote                 string             `json:"vote,omitempty"`
	Rationale            string             `json:"rationale,omitempty"`
	Usage                *openaiapi.Usage   `json:"usage,omitempty"`
	CostUSD              *float64           `json:"cost_usd,omitempty"`
	CostObservationCount int                `json:"cost_observation_count"`
	CostSource           string             `json:"cost_source,omitempty"`
	MetadataError        string             `json:"metadata_error,omitempty"`
	ExitCode             *int               `json:"exit_code,omitempty"`
	ContainerName        string             `json:"container_name,omitempty"`
	ToolCalls            []recordedToolCall `json:"tool_calls,omitempty"`
}

type result struct {
	CreatedAt            string      `json:"created_at"`
	SpecPath             string      `json:"spec_path"`
	EndpointVariantID    string      `json:"endpoint_variant_id,omitempty"`
	Model                string      `json:"model"`
	ProviderRoute        []string    `json:"provider_route,omitempty"`
	Passed               bool        `json:"passed"`
	Direct               checkResult `json:"direct"`
	PiMCP                checkResult `json:"pi_mcp"`
	ObservedCostUSD      float64     `json:"observed_cost_usd"`
	CostObservationCount int         `json:"cost_observation_count"`
}

type recordedToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
	Accepted  bool           `json:"accepted"`
	Error     string         `json:"error,omitempty"`
}

type vote struct {
	Vote      string
	Rationale string
}

func main() {
	os.Exit(runMain(os.Args[1:]))
}

func runMain(argv []string) int {
	opts, err := parseOptions(argv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	opts.outputDir, err = filepath.Abs(opts.outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve output directory: %v\n", err)
		return 2
	}
	if err := prepareOutputDir(opts.outputDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	raw, err := os.ReadFile(opts.specPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read model configuration: %v\n", err)
		return 2
	}
	spec, err := modelrequest.ParseJSON(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse model configuration: %v\n", err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	direct := runDirectCheck(ctx, opts, spec)
	pi := runPiCheck(ctx, opts, spec)
	out := result{
		CreatedAt:         time.Now().UTC().Format(time.RFC3339),
		SpecPath:          opts.specPath,
		EndpointVariantID: metadataString(spec, "endpoint_variant_id"),
		Model:             spec.RuntimeModel(),
		Direct:            direct,
		PiMCP:             pi,
		Passed:            direct.Status == "passed" && pi.Status == "passed",
	}
	if spec.Provider != nil {
		out.ProviderRoute = append([]string(nil), spec.Provider.Only...)
	}
	for _, check := range []checkResult{direct, pi} {
		if check.CostUSD != nil {
			out.ObservedCostUSD += *check.CostUSD
		}
		out.CostObservationCount += check.CostObservationCount
	}
	if err := writeJSON(filepath.Join(opts.outputDir, "result.json"), out, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fmt.Fprintf(os.Stderr, "write result: %v\n", err)
		return 2
	}
	if out.Passed {
		return 0
	}
	if direct.InfrastructureError || pi.InfrastructureError {
		return 2
	}
	return 1
}

func parseOptions(argv []string) (options, error) {
	fs := flag.NewFlagSet("model-config-screen", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var opts options
	fs.StringVar(&opts.specPath, "spec", "", "model configuration JSON")
	fs.StringVar(&opts.outputDir, "out", "", "output directory")
	fs.DurationVar(&opts.directTimeout, "direct-timeout", 20*time.Second, "Quick-style request timeout")
	fs.DurationVar(&opts.piTimeout, "pi-timeout", 5*time.Minute, "Pi process timeout")
	fs.IntVar(&opts.maxAttempts, "max-attempts", 3, "provider request attempts")
	fs.StringVar(&opts.podmanCommand, "podman-command", "podman", "Podman command")
	fs.StringVar(&opts.piImage, "pi-image", "agentcourt-pi-sandbox", "Pi container image")
	fs.StringVar(&opts.piMCPAdapter, "pi-mcp-adapter", "/opt/pi-extensions/pi-mcp-adapter/node_modules/pi-mcp-adapter", "Pi MCP adapter path")
	fs.StringVar(&opts.piMCPHost, "pi-mcp-host", "127.0.0.1", "host used by Pi to reach MCP")
	if err := fs.Parse(argv); err != nil {
		return options{}, err
	}
	if fs.NArg() != 0 {
		return options{}, fmt.Errorf("unexpected positional arguments")
	}
	if strings.TrimSpace(opts.specPath) == "" || strings.TrimSpace(opts.outputDir) == "" {
		return options{}, fmt.Errorf("--spec and --out are required")
	}
	if opts.directTimeout <= 0 || opts.piTimeout <= 0 {
		return options{}, fmt.Errorf("timeouts must be positive")
	}
	if opts.maxAttempts < 1 || opts.maxAttempts > 4 {
		return options{}, fmt.Errorf("--max-attempts must be between 1 and 4")
	}
	for name, value := range map[string]string{
		"--podman-command": opts.podmanCommand,
		"--pi-image":       opts.piImage,
		"--pi-mcp-adapter": opts.piMCPAdapter,
		"--pi-mcp-host":    opts.piMCPHost,
	} {
		if strings.TrimSpace(value) == "" {
			return options{}, fmt.Errorf("%s must be non-empty", name)
		}
	}
	return opts, nil
}

func prepareOutputDir(path string) error {
	entries, err := os.ReadDir(path)
	if err == nil {
		if len(entries) != 0 {
			return fmt.Errorf("output directory %s must be empty", path)
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read output directory: %w", err)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	return nil
}

func runDirectCheck(parent context.Context, opts options, spec modelrequest.Spec) (result checkResult) {
	started := time.Now()
	result = checkResult{Status: "failed"}
	defer func() { result.ElapsedSeconds = time.Since(started).Seconds() }()
	if supported, known := spec.SupportsParameter("tools"); known && !supported {
		result.Error = "endpoint metadata omits required parameter tools"
		result.ErrorClass = string(openaiapi.ProviderErrorRequest)
		return result
	}
	client, err := openaiapi.NewForEndpoint(spec.Endpoint, false, opts.directTimeout)
	if err != nil {
		result.Error = err.Error()
		result.ErrorClass = string(openaiapi.ErrorClass(err))
		result.InfrastructureError = openaiapi.ErrorClass(err) == openaiapi.ProviderErrorAuthentication
		return result
	}
	if err := client.SetMaxAttempts(opts.maxAttempts); err != nil {
		result.Error = err.Error()
		result.InfrastructureError = true
		return result
	}
	ctx, cancel := context.WithTimeout(parent, opts.directTimeout)
	defer cancel()
	prompt := fmt.Sprintf(
		"Availability check for council seat C1, model %s, persona file model-pool-screen.md. Call submit_council_vote exactly once with vote=demonstrated and rationale=READY.",
		spec.RuntimeModel(),
	)
	response, err := client.CreateResponseWithRequestSpec(
		ctx,
		spec.WithMaxOutputTokens(directMaxOutputTokens),
		[]map[string]any{{"role": "user", "content": prompt}},
		councilTools(),
		"",
	)
	if response.RawJSON != "" {
		if writeErr := os.WriteFile(filepath.Join(opts.outputDir, "direct-response.json"), []byte(response.RawJSON+"\n"), 0o644); writeErr != nil {
			result.Error = fmt.Sprintf("write direct response: %v", writeErr)
			result.InfrastructureError = true
			return result
		}
	}
	result.ResponseID = response.ResponseID
	result.Usage = response.TokenUsage()
	result.CostUSD = response.CostUSD()
	if result.CostUSD != nil {
		result.CostObservationCount = 1
		result.CostSource = "openrouter"
	}
	result.MetadataError = response.OpenRouterGenerationError
	if err != nil {
		result.Error = err.Error()
		result.ErrorClass = string(openaiapi.ErrorClass(err))
		result.InfrastructureError = openaiapi.ErrorClass(err) == openaiapi.ProviderErrorAuthentication
		return result
	}
	parsed, err := parseCouncilVote(response)
	if err != nil {
		result.Error = err.Error()
		result.ErrorClass = string(openaiapi.ProviderErrorProtocol)
		return result
	}
	result.Status = "passed"
	result.Vote = parsed.Vote
	result.Rationale = parsed.Rationale
	return result
}

func councilTools() []map[string]any {
	return []map[string]any{{
		"type":        "function",
		"name":        "submit_council_vote",
		"description": "Submit this council member's vote. Use demonstrated only when the proposition satisfies the stated evidence standard; use not_demonstrated otherwise. The rationale must support the selected vote.",
		"strict":      true,
		"parameters":  councilVoteSchema(),
	}}
}

func councilVoteSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"vote":      map[string]any{"type": "string", "enum": []string{"demonstrated", "not_demonstrated"}},
			"rationale": map[string]any{"type": "string"},
		},
		"required":             []string{"vote", "rationale"},
		"additionalProperties": false,
	}
}

func parseCouncilVote(response openaiapi.Response) (vote, error) {
	if len(response.ToolCalls) != 1 {
		return vote{}, fmt.Errorf("response must contain one tool call")
	}
	call := response.ToolCalls[0]
	if call.ArgumentsError != "" {
		return vote{}, fmt.Errorf("parse submit_council_vote arguments: %s", call.ArgumentsError)
	}
	if call.Name != "submit_council_vote" {
		return vote{}, fmt.Errorf("tool must be submit_council_vote")
	}
	if len(call.Arguments) != 2 {
		return vote{}, fmt.Errorf("submit_council_vote accepts only vote and rationale")
	}
	choice, _ := call.Arguments["vote"].(string)
	choice = strings.TrimSpace(choice)
	if choice != "demonstrated" && choice != "not_demonstrated" {
		return vote{}, fmt.Errorf("vote must be demonstrated or not_demonstrated")
	}
	rationale, _ := call.Arguments["rationale"].(string)
	rationale = strings.TrimSpace(rationale)
	if rationale == "" {
		return vote{}, fmt.Errorf("rationale must be non-empty")
	}
	return vote{Vote: choice, Rationale: rationale}, nil
}

type screenAdapter struct {
	mu        sync.Mutex
	submitted bool
	vote      vote
	calls     []recordedToolCall
}

func (a *screenAdapter) OpenSession(assignment mcpbridge.Assignment) (mcpbridge.Profile, error) {
	if assignment.AssignmentType != "council" || assignment.PrincipalID != "C1" {
		return mcpbridge.Profile{}, fmt.Errorf("screen requires council assignment C1")
	}
	return mcpbridge.Profile{
		CaseID:         assignment.CaseID,
		AssignmentType: assignment.AssignmentType,
		PrincipalID:    assignment.PrincipalID,
		Instructions:   "This session presents one ARB council opportunity. Wait for the opportunity, then submit one vote.",
	}, nil
}

func (a *screenAdapter) Tools(mcpbridge.Profile) ([]mcpbridge.Tool, error) {
	return []mcpbridge.Tool{
		{
			Name:        "wait_for_opportunity",
			Description: "Wait for the assigned ARB council opportunity.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"after_opportunity_id": map[string]any{"type": "string"},
					"after_version":        map[string]any{"type": "integer", "minimum": 0},
					"timeout_ms":           map[string]any{"type": "integer", "minimum": 1, "maximum": 30000},
				},
				"additionalProperties": false,
			},
			ReadOnly: true,
		},
		{
			Name:        "submit_council_vote",
			Description: "Submit one council vote for the current deliberation opportunity.",
			InputSchema: councilVoteSchema(),
		},
	}, nil
}

func (a *screenAdapter) CallTool(_ context.Context, _ mcpbridge.Profile, name string, arguments map[string]any) (mcpbridge.CallResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	call := recordedToolCall{Name: name, Arguments: cloneMap(arguments)}
	switch name {
	case "wait_for_opportunity":
		if err := onlyKeys(arguments, "after_opportunity_id", "after_version", "timeout_ms"); err != nil {
			call.Error = err.Error()
			a.calls = append(a.calls, call)
			return mcpbridge.CallResult{StructuredContent: map[string]any{"ok": false, "state": "error", "error": err.Error()}, IsError: true}, nil
		}
		call.Accepted = true
		a.calls = append(a.calls, call)
		if a.submitted {
			return mcpbridge.CallResult{StructuredContent: map[string]any{
				"ok": true, "status": "done", "state": "done", "after_version": 2,
			}}, nil
		}
		return mcpbridge.CallResult{StructuredContent: map[string]any{
			"ok": true, "status": "ready", "state": "ready", "after_version": 1,
			"after_opportunity_id": "deliberation:1:C1",
			"instructions":         "Call submit_council_vote exactly once with vote=demonstrated and rationale=READY.",
			"turn": map[string]any{
				"phase": "deliberation", "opportunity_id": "deliberation:1:C1",
				"allowed_tools": []string{"submit_council_vote"},
			},
		}}, nil
	case "submit_council_vote":
		if a.submitted {
			call.Error = "council vote was already submitted"
			a.calls = append(a.calls, call)
			return mcpbridge.CallResult{StructuredContent: map[string]any{"ok": false, "error": call.Error}, IsError: true}, nil
		}
		parsed, err := parseVoteArguments(arguments)
		if err != nil {
			call.Error = err.Error()
			a.calls = append(a.calls, call)
			return mcpbridge.CallResult{StructuredContent: map[string]any{"ok": false, "error": err.Error()}, IsError: true}, nil
		}
		a.submitted = true
		a.vote = parsed
		call.Accepted = true
		a.calls = append(a.calls, call)
		return mcpbridge.CallResult{StructuredContent: map[string]any{
			"ok": true, "status": "accepted", "vote": parsed.Vote, "rationale": parsed.Rationale,
		}}, nil
	default:
		return mcpbridge.CallResult{}, fmt.Errorf("unknown tool %q", name)
	}
}

func (a *screenAdapter) snapshot() (bool, vote, []recordedToolCall) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.submitted, a.vote, append([]recordedToolCall(nil), a.calls...)
}

func parseVoteArguments(arguments map[string]any) (vote, error) {
	if err := onlyKeys(arguments, "vote", "rationale"); err != nil {
		return vote{}, err
	}
	if len(arguments) != 2 {
		return vote{}, fmt.Errorf("submit_council_vote requires vote and rationale")
	}
	choice, _ := arguments["vote"].(string)
	choice = strings.TrimSpace(choice)
	if choice != "demonstrated" && choice != "not_demonstrated" {
		return vote{}, fmt.Errorf("vote must be demonstrated or not_demonstrated")
	}
	rationale, _ := arguments["rationale"].(string)
	rationale = strings.TrimSpace(rationale)
	if rationale == "" {
		return vote{}, fmt.Errorf("rationale must be non-empty")
	}
	return vote{Vote: choice, Rationale: rationale}, nil
}

func onlyKeys(arguments map[string]any, allowed ...string) error {
	set := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		set[key] = struct{}{}
	}
	for key := range arguments {
		if _, ok := set[key]; !ok {
			return fmt.Errorf("unexpected argument %q", key)
		}
	}
	return nil
}

func cloneMap(value map[string]any) map[string]any {
	copy := make(map[string]any, len(value))
	for key, item := range value {
		copy[key] = item
	}
	return copy
}

func runPiCheck(parent context.Context, opts options, spec modelrequest.Spec) (result checkResult) {
	started := time.Now()
	result = checkResult{Status: "failed"}
	defer func() { result.ElapsedSeconds = time.Since(started).Seconds() }()
	if spec.Endpoint != "openrouter" {
		result.Error = fmt.Sprintf("Pi council screening requires openrouter endpoint; got %s", spec.Endpoint)
		result.ErrorClass = string(openaiapi.ProviderErrorRequest)
		return result
	}
	if strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")) == "" {
		result.Error = "OPENROUTER_API_KEY is required"
		result.ErrorClass = string(openaiapi.ProviderErrorAuthentication)
		result.InfrastructureError = true
		return result
	}

	adapter := &screenAdapter{}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		result.Error = fmt.Sprintf("generate MCP signing key: %v", err)
		result.InfrastructureError = true
		return result
	}
	capability, err := mcpbridge.IssueCapability(key, mcpbridge.Assignment{
		Audience: "aar", CaseID: "model-config-screen", AssignmentType: "council", PrincipalID: "C1",
	})
	if err != nil {
		result.Error = err.Error()
		result.InfrastructureError = true
		return result
	}
	mcpLog, err := os.Create(filepath.Join(opts.outputDir, "mcp.log"))
	if err != nil {
		result.Error = fmt.Sprintf("create MCP log: %v", err)
		result.InfrastructureError = true
		return result
	}
	defer mcpLog.Close()
	serverCtx, cancelServer := context.WithCancel(parent)
	defer cancelServer()
	ready := make(chan string, 1)
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- mcpbridge.Run(serverCtx, mcpbridge.Options{
			ListenAddr: "127.0.0.1:0", Audience: "aar", SigningKey: key,
			DisableSessionExpiry: true, ServerName: "model-config-screen", Adapter: adapter,
			Log: mcpLog, ListenerReady: func(addr string) error { ready <- addr; return nil },
		})
	}()
	var addr string
	select {
	case addr = <-ready:
	case err := <-serverDone:
		result.Error = fmt.Sprintf("start MCP server: %v", err)
		result.InfrastructureError = true
		return result
	case <-parent.Done():
		result.Error = parent.Err().Error()
		result.InfrastructureError = true
		return result
	}

	home := filepath.Join(opts.outputDir, "pi-home")
	if err := writePiConfig(home, spec, "http://"+mcpAddress(opts.piMCPHost, addr)+"/mcp", capability); err != nil {
		result.Error = err.Error()
		result.InfrastructureError = true
		return result
	}
	containerName, err := newContainerName()
	if err != nil {
		result.Error = err.Error()
		result.InfrastructureError = true
		return result
	}
	result.ContainerName = containerName
	stdoutPath := filepath.Join(opts.outputDir, "pi.stdout.jsonl")
	stderrPath := filepath.Join(opts.outputDir, "pi.stderr.log")
	exitCode, runErr := runPiProcess(parent, opts, home, spec.UpstreamModel(), containerName, stdoutPath, stderrPath)
	result.ExitCode = &exitCode
	cancelServer()
	if err := <-serverDone; err != nil && !errors.Is(err, context.Canceled) {
		result.Error = fmt.Sprintf("MCP server: %v", err)
		result.InfrastructureError = true
		return result
	}
	submitted, submittedVote, calls := adapter.snapshot()
	result.ToolCalls = calls
	result.Vote = submittedVote.Vote
	result.Rationale = submittedVote.Rationale
	usage, responseIDs, cost, costCount, err := readPiTranscript(stdoutPath, pimodel.OpenRouterCost(spec.VariantMetadata) != nil)
	if err != nil {
		result.Error = err.Error()
		result.InfrastructureError = true
		return result
	}
	result.Usage = usage
	result.ResponseIDs = responseIDs
	result.CostUSD = cost
	result.CostObservationCount = costCount
	if cost != nil {
		result.CostSource = "pi_catalog"
	}
	if runErr != nil {
		result.Error = runErr.Error()
		result.InfrastructureError = exitCode < 0 || exitCode == 125
		return result
	}
	if exitCode != 0 {
		result.Error = fmt.Sprintf("Pi exited with code %d", exitCode)
		return result
	}
	waitedBeforeSubmission := false
	for _, call := range calls {
		if call.Name == "wait_for_opportunity" && call.Accepted {
			waitedBeforeSubmission = true
		}
		if call.Name == "submit_council_vote" && call.Accepted {
			break
		}
	}
	if !waitedBeforeSubmission {
		result.Error = "Pi exited without an accepted wait_for_opportunity call before submit_council_vote"
		return result
	}
	if !submitted {
		result.Error = "Pi exited without an accepted submit_council_vote call"
		return result
	}
	result.Status = "passed"
	return result
}

func mcpAddress(host, listenerAddress string) string {
	_, port, err := splitHostPort(listenerAddress)
	if err != nil {
		return listenerAddress
	}
	return strings.TrimSpace(host) + ":" + port
}

func splitHostPort(address string) (string, string, error) {
	last := strings.LastIndex(address, ":")
	if last < 0 {
		return "", "", fmt.Errorf("address has no port")
	}
	return address[:last], address[last+1:], nil
}

func writePiConfig(home string, spec modelrequest.Spec, mcpURL, capability string) error {
	settingsDir := filepath.Join(home, ".pi", "agent")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		return fmt.Errorf("create Pi settings directory: %w", err)
	}
	if err := writeJSON(filepath.Join(settingsDir, "settings.json"), map[string]any{
		"defaultProvider": "openrouter", "defaultModel": spec.UpstreamModel(), "quietStartup": true,
	}, 0o644); err != nil {
		return err
	}
	modelEntry := map[string]any{
		"id": spec.UpstreamModel(), "name": "Model pool screen " + spec.UpstreamModel(),
	}
	spec = spec.WithFallbackMaxOutputTokens(piMaxOutputTokens)
	if maxTokens := spec.MaxOutputTokens(); maxTokens != nil {
		modelEntry["maxTokens"] = *maxTokens
	}
	sampling := map[string]any{}
	if spec.Request.Temperature != nil {
		sampling["temperature"] = *spec.Request.Temperature
	}
	if spec.Request.TopP != nil {
		sampling["top_p"] = *spec.Request.TopP
	}
	if len(sampling) > 0 {
		modelEntry["samplingParams"] = sampling
	}
	if cost := pimodel.OpenRouterCost(spec.VariantMetadata); len(cost) > 0 {
		modelEntry["cost"] = cost
	}
	if compat := pimodel.OpenRouterCompat(spec.ProviderBody(), spec.VariantMetadata); len(compat) > 0 {
		modelEntry["compat"] = compat
	}
	provider := map[string]any{
		"baseUrl": "https://openrouter.ai/api/v1", "apiKey": "$OPENROUTER_API_KEY",
		"api": "openai-completions", "models": []map[string]any{modelEntry},
	}
	if len(spec.Headers) > 0 {
		provider["headers"] = spec.Headers
	}
	if err := writeJSON(filepath.Join(settingsDir, "models.json"), map[string]any{
		"providers": map[string]any{"openrouter": provider},
	}, 0o644); err != nil {
		return err
	}
	return writeJSON(filepath.Join(home, ".mcp.json"), map[string]any{
		"mcpServers": map[string]any{piMCPServer: map[string]any{
			"url": mcpURL, "transport": "streamable-http", "lifecycle": "keep-alive",
			"headers": map[string]string{"Authorization": "Bearer " + capability},
		}},
	}, 0o600)
}

func writeJSON(path string, value any, mode os.FileMode) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, mode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func newContainerName() (string, error) {
	random := make([]byte, 6)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		return "", fmt.Errorf("generate container name: %w", err)
	}
	return fmt.Sprintf("adj-model-screen-%d-%s", os.Getpid(), hex.EncodeToString(random)), nil
}

func piPrompt() string {
	return `You are council member C1 for AAR case model-config-screen. Use the Pi proxy tool named mcp. The proxy exposes each AAR tool under the aar_ prefix.

Every mcp call must contain a tool string and an args string whose value is JSON encoded as a string. Do not pass args as an object or include action, server, or connect. Begin with {"tool":"aar_wait_for_opportunity","args":"{}"}.

When the result has state: ready, follow the returned instructions and submit the requested vote through aar_submit_council_vote. Invoke each AAR tool through mcp; do not emit tool-call JSON as response text.

Stop after the vote is accepted. Report the terminal state. Do not ask the user for another turn, create a scheduled job, or listen for inbound HTTP.`
}

func runPiProcess(parent context.Context, opts options, home, model, containerName, stdoutPath, stderrPath string) (int, error) {
	stdout, err := os.Create(stdoutPath)
	if err != nil {
		return -1, fmt.Errorf("create Pi stdout: %w", err)
	}
	defer stdout.Close()
	stderr, err := os.Create(stderrPath)
	if err != nil {
		return -1, fmt.Errorf("create Pi stderr: %w", err)
	}
	defer stderr.Close()
	ctx, cancel := context.WithTimeout(parent, opts.piTimeout)
	defer cancel()
	args := []string{
		"run", "--rm", "--name", containerName, "--network", "host", "--user", "0:0",
		"-e", "HOME=/home/user", "-e", "TMPDIR=/home/user", "-e", "PI_CODING_AGENT_DIR=/home/user/.pi/agent",
		"-e", "OPENROUTER_API_KEY", "-e", "NODE_OPTIONS", "-v", home + ":/home/user", "-w", "/home/user",
		opts.piImage, "--provider", "openrouter", "--model", model, "-e", opts.piMCPAdapter,
		"--mode", "json", "-p", piPrompt(),
	}
	cmd := exec.Command(opts.podmanCommand, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return -1, fmt.Errorf("start Pi container: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return processExitCode(cmd.ProcessState), err
	case <-ctx.Done():
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		stop := exec.CommandContext(stopCtx, opts.podmanCommand, "stop", "--time", "5", containerName)
		stop.Stdout = stderr
		stop.Stderr = stderr
		stopErr := stop.Run()
		stopCancel()
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			<-done
		}
		return processExitCode(cmd.ProcessState), errors.Join(ctx.Err(), stopErr)
	}
}

func processExitCode(state *os.ProcessState) int {
	if state == nil {
		return -1
	}
	return state.ExitCode()
}

func readPiTranscript(path string, costKnown bool) (*openaiapi.Usage, []string, *float64, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("read Pi transcript: %w", err)
	}
	defer file.Close()
	type piUsage struct {
		Input       int64 `json:"input"`
		Output      int64 `json:"output"`
		CacheRead   int64 `json:"cacheRead"`
		CacheWrite  int64 `json:"cacheWrite"`
		Reasoning   int64 `json:"reasoning"`
		TotalTokens int64 `json:"totalTokens"`
		Cost        struct {
			Total float64 `json:"total"`
		} `json:"cost"`
	}
	type piEvent struct {
		Type    string `json:"type"`
		Message struct {
			Role       string   `json:"role"`
			ResponseID string   `json:"responseId"`
			Usage      *piUsage `json:"usage"`
		} `json:"message"`
	}
	usage := openaiapi.Usage{}
	usageKnown := false
	responseIDs := []string{}
	seenResponseIDs := map[string]struct{}{}
	var totalCost float64
	costCount := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		var event piEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, nil, nil, 0, fmt.Errorf("parse Pi transcript line %d: %w", line, err)
		}
		if event.Type != "message_end" || event.Message.Role != "assistant" {
			continue
		}
		if event.Message.Usage != nil {
			value := event.Message.Usage
			usage.InputTokens += value.Input + value.CacheRead + value.CacheWrite
			usage.CachedInputTokens += value.CacheRead
			usage.OutputTokens += value.Output
			usage.ReasoningTokens += value.Reasoning
			usage.TotalTokens += value.TotalTokens
			usageKnown = true
			if costKnown {
				totalCost += value.Cost.Total
				costCount++
			}
		}
		responseID := strings.TrimSpace(event.Message.ResponseID)
		if responseID == "" {
			continue
		}
		if _, exists := seenResponseIDs[responseID]; exists {
			continue
		}
		seenResponseIDs[responseID] = struct{}{}
		responseIDs = append(responseIDs, responseID)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, nil, 0, fmt.Errorf("read Pi transcript: %w", err)
	}
	var cost *float64
	if costCount > 0 {
		cost = &totalCost
	}
	if !usageKnown {
		return nil, responseIDs, cost, costCount, nil
	}
	return &usage, responseIDs, cost, costCount, nil
}

func metadataString(spec modelrequest.Spec, key string) string {
	value, _ := spec.VariantMetadata[key].(string)
	return strings.TrimSpace(value)
}
