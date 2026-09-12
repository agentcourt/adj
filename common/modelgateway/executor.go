package modelgateway

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/agentcourt/adj/common/modelapi"
	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

type providerClient interface {
	CreateResponseWithRequestSpec(
		context.Context,
		modelrequest.Spec,
		[]map[string]any,
		[]map[string]any,
		string,
	) (modelapi.Response, error)
}

type endpointConfig struct {
	name           string
	credentialName string
	baseURL        string
	protocol       string
}

var endpointConfigs = []endpointConfig{
	{name: "anthropic", credentialName: "ANTHROPIC_API_KEY", protocol: "anthropic"},
	{name: "deepseek", credentialName: "DEEPSEEK_API_KEY", protocol: "deepseek"},
	{name: "google", credentialName: "GEMINI_API_KEY", protocol: "google"},
	{name: "huggingface", credentialName: "HF_TOKEN", baseURL: "https://router.huggingface.co/v1", protocol: "openai"},
	{name: "openai", credentialName: "OPENAI_API_KEY", baseURL: "https://api.openai.com/v1", protocol: "openai"},
	{name: "openrouter", credentialName: "OPENROUTER_API_KEY", baseURL: "https://openrouter.ai/api/v1", protocol: "openai"},
	{name: "xai", credentialName: "XAI_API_KEY", baseURL: "https://api.x.ai/v1", protocol: "openai"},
}

type responseState struct {
	endpoint   string
	model      string
	inputCount int
}

type Executor struct {
	timeout     time.Duration
	maxAttempts int
	environment map[string]string

	mu      sync.Mutex
	clients map[string]providerClient
	states  map[string]responseState

	accounting modelapi.AccountingRecorder
}

type EndpointCredentialError struct {
	Endpoint string
	Err      error
}

func (e *EndpointCredentialError) Error() string { return e.Err.Error() }
func (e *EndpointCredentialError) Unwrap() error { return e.Err }

func CredentialFailureEndpoint(err error) (string, bool) {
	var credentialErr *EndpointCredentialError
	if !errors.As(err, &credentialErr) {
		return "", false
	}
	endpoint := normalizedEndpoint(credentialErr.Endpoint)
	return endpoint, endpoint != ""
}

func New(timeout time.Duration, maxAttempts int) (*Executor, error) {
	return NewWithEnvironment(timeout, maxAttempts, os.Environ())
}

func NewWithEnvironment(timeout time.Duration, maxAttempts int, environment []string) (*Executor, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("model request timeout must be positive")
	}
	if maxAttempts < 1 || maxAttempts > 4 {
		return nil, fmt.Errorf("provider attempts must be between 1 and 4")
	}
	return &Executor{
		timeout:     timeout,
		maxAttempts: maxAttempts,
		environment: environmentMap(environment),
		clients:     map[string]providerClient{},
		states:      map[string]responseState{},
	}, nil
}

func (e *Executor) CheckEndpoint(endpoint string) error {
	_, err := e.clientForEndpoint(endpoint)
	return err
}

func (e *Executor) CreateResponse(
	ctx context.Context,
	model string,
	inputItems []map[string]any,
	tools []map[string]any,
	previousResponseID string,
	temperature *float64,
) (modelapi.Response, error) {
	ref, err := modelrequest.ParseModelRef(model)
	if err != nil {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: err}
	}
	return e.CreateResponseWithRequestSpec(ctx, modelrequest.Spec{
		Endpoint: ref.Endpoint,
		Model:    ref.Model,
		Request:  modelrequest.RequestParameters{Temperature: temperature},
	}, inputItems, tools, previousResponseID)
}

func (e *Executor) CreateResponseWithRequestSpec(
	ctx context.Context,
	spec modelrequest.Spec,
	inputItems []map[string]any,
	tools []map[string]any,
	previousResponseID string,
) (response modelapi.Response, err error) {
	if err := validateRequestSpec(spec); err != nil {
		return modelapi.Response{}, &modelapi.ProviderError{Class: modelapi.ProviderErrorRequest, Err: err}
	}
	client, err := e.clientForEndpoint(spec.Endpoint)
	if err != nil {
		return modelapi.Response{}, err
	}
	delta, err := e.requestDelta(spec, inputItems, previousResponseID)
	if err != nil {
		return modelapi.Response{}, err
	}
	defer func() {
		e.accounting.Record(response)
	}()
	response, err = client.CreateResponseWithRequestSpec(ctx, spec, delta, tools, previousResponseID)
	if err != nil {
		return modelapi.Response{}, err
	}
	if strings.TrimSpace(response.ResponseID) == "" {
		return modelapi.Response{}, &modelapi.ProviderError{
			Class: modelapi.ProviderErrorProtocol,
			Err:   fmt.Errorf("model response omitted an id"),
		}
	}
	e.mu.Lock()
	e.states[response.ResponseID] = responseState{
		endpoint:   normalizedEndpoint(spec.Endpoint),
		model:      spec.UpstreamModel(),
		inputCount: len(inputItems),
	}
	e.mu.Unlock()
	return response, nil
}

func validateRequestSpec(spec modelrequest.Spec) error {
	endpoint := normalizedEndpoint(spec.Endpoint)
	config, ok := endpointConfiguration(endpoint)
	if !ok {
		return fmt.Errorf("unsupported model endpoint %q", endpoint)
	}
	if strings.TrimSpace(spec.UpstreamModel()) == "" {
		return fmt.Errorf("model is required for endpoint %s", endpoint)
	}
	if endpoint != "openrouter" && spec.ProviderBody() != nil {
		return fmt.Errorf("provider routing constraints require the openrouter endpoint")
	}
	for name := range spec.Headers {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "authorization", "host", "x-api-key", "x-goog-api-key":
			return fmt.Errorf("request header %q is controlled by the %s endpoint", name, endpoint)
		}
	}
	if config.protocol != "openai" {
		if len(spec.Headers) > 0 {
			return fmt.Errorf("request headers are unsupported for the %s endpoint", endpoint)
		}
		if spec.MaxToolCalls() != nil {
			return fmt.Errorf("max_tool_calls is unsupported for the %s endpoint", endpoint)
		}
	}
	return nil
}

func (e *Executor) Accounting() modelapi.Accounting {
	return e.accounting.Snapshot()
}

func (e *Executor) requestDelta(spec modelrequest.Spec, inputItems []map[string]any, previousResponseID string) ([]map[string]any, error) {
	if strings.TrimSpace(previousResponseID) == "" {
		return inputItems, nil
	}
	e.mu.Lock()
	state, ok := e.states[previousResponseID]
	e.mu.Unlock()
	if !ok {
		return nil, &modelapi.ProviderError{
			Class: modelapi.ProviderErrorRequest,
			Err:   fmt.Errorf("previous model response %q is unknown", previousResponseID),
		}
	}
	if state.endpoint != normalizedEndpoint(spec.Endpoint) || state.model != spec.UpstreamModel() {
		return nil, &modelapi.ProviderError{
			Class: modelapi.ProviderErrorRequest,
			Err:   fmt.Errorf("previous model response belongs to another model configuration"),
		}
	}
	if len(inputItems) < state.inputCount {
		return nil, &modelapi.ProviderError{
			Class: modelapi.ProviderErrorRequest,
			Err:   fmt.Errorf("continued model request removed prior input items"),
		}
	}
	return inputItems[state.inputCount:], nil
}

func (e *Executor) clientForEndpoint(endpoint string) (providerClient, error) {
	endpoint = normalizedEndpoint(endpoint)
	e.mu.Lock()
	defer e.mu.Unlock()
	if client := e.clients[endpoint]; client != nil {
		return client, nil
	}
	var (
		client providerClient
		err    error
	)
	config, ok := endpointConfiguration(endpoint)
	if !ok {
		return nil, &modelapi.ProviderError{
			Class: modelapi.ProviderErrorRequest,
			Err:   fmt.Errorf("unsupported model endpoint %q", endpoint),
		}
	}
	apiKey := strings.TrimSpace(e.environment[config.credentialName])
	if apiKey == "" {
		return nil, &EndpointCredentialError{
			Endpoint: endpoint,
			Err: &modelapi.ProviderError{
				Class: modelapi.ProviderErrorAuthentication,
				Err:   fmt.Errorf("%s is required for %s models", config.credentialName, endpoint),
			},
		}
	}
	switch config.protocol {
	case "openai":
		var openAIClient *openaiapi.Client
		openAIClient, err = openaiapi.New(apiKey, config.baseURL, false, e.timeout)
		if err == nil {
			err = openAIClient.SetMaxAttempts(e.maxAttempts)
			client = openAIClient
		}
	case "anthropic":
		client, err = newAnthropicClient(apiKey, e.timeout, e.maxAttempts)
	case "google":
		client, err = newGoogleClient(apiKey, e.timeout, e.maxAttempts)
	case "deepseek":
		client, err = newDeepSeekClient(apiKey, e.timeout, e.maxAttempts)
	}
	if err != nil {
		return nil, err
	}
	e.clients[endpoint] = client
	return client, nil
}

func normalizedEndpoint(endpoint string) string {
	return strings.ToLower(strings.TrimSpace(endpoint))
}

func SupportedEndpoints() []string {
	endpoints := make([]string, len(endpointConfigs))
	for index, config := range endpointConfigs {
		endpoints[index] = config.name
	}
	return endpoints
}

func CredentialEnvironmentName(endpoint string) (string, bool) {
	config, ok := endpointConfiguration(normalizedEndpoint(endpoint))
	return config.credentialName, ok
}

func CredentialEnvironmentNames() []string {
	names := make([]string, len(endpointConfigs))
	for index, config := range endpointConfigs {
		names[index] = config.credentialName
	}
	return names
}

func endpointConfiguration(endpoint string) (endpointConfig, bool) {
	for _, config := range endpointConfigs {
		if config.name == endpoint {
			return config, true
		}
	}
	return endpointConfig{}, false
}

func environmentMap(environment []string) map[string]string {
	values := make(map[string]string, len(environment))
	for _, entry := range environment {
		name, value, ok := strings.Cut(entry, "=")
		if ok {
			values[name] = value
		}
	}
	return values
}
