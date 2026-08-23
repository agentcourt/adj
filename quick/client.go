package quick

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

type responseClient interface {
	CreateResponseWithRequestSpec(
		context.Context,
		modelrequest.Spec,
		[]map[string]any,
		[]map[string]any,
		string,
	) (openaiapi.Response, error)
	Accounting() openaiapi.Accounting
}

type councilCandidatePreflighter interface {
	PreflightCouncilCandidate(context.Context, CouncilMember, []map[string]any, []map[string]any) (openaiapi.Response, error)
}

const councilPreflightMaxOutputTokens int64 = 1024

type endpointCredentialError struct {
	endpoint string
	err      error
}

func (e *endpointCredentialError) Error() string { return e.err.Error() }
func (e *endpointCredentialError) Unwrap() error { return e.err }

type directClient struct {
	timeout     time.Duration
	maxAttempts int
	mu          sync.Mutex
	clients     map[string]*openaiapi.Client
	accounting  openaiapi.AccountingRecorder
}

func newDirectClient(timeout time.Duration, maxAttempts int) *directClient {
	return &directClient{
		timeout:     timeout,
		maxAttempts: maxAttempts,
		clients:     make(map[string]*openaiapi.Client),
	}
}

func (c *directClient) CreateResponseWithRequestSpec(
	ctx context.Context,
	spec modelrequest.Spec,
	input []map[string]any,
	tools []map[string]any,
	previousResponseID string,
) (openaiapi.Response, error) {
	client, err := c.clientForEndpoint(spec.Endpoint)
	if err != nil {
		if openaiapi.ErrorClass(err) == openaiapi.ProviderErrorAuthentication {
			return openaiapi.Response{}, &endpointCredentialError{
				endpoint: strings.ToLower(strings.TrimSpace(spec.Endpoint)),
				err:      err,
			}
		}
		return openaiapi.Response{}, err
	}
	response, err := client.CreateResponseWithRequestSpec(ctx, spec, input, tools, previousResponseID)
	c.accounting.Record(response)
	if err != nil {
		return openaiapi.Response{}, err
	}
	return response, nil
}

func (c *directClient) recordResponse(response openaiapi.Response) {
	c.accounting.Record(response)
}

func (c *directClient) clientForEndpoint(endpoint string) (*openaiapi.Client, error) {
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))
	c.mu.Lock()
	defer c.mu.Unlock()
	if client := c.clients[endpoint]; client != nil {
		return client, nil
	}
	client, err := openaiapi.NewForEndpoint(endpoint, false, c.timeout)
	if err != nil {
		return nil, err
	}
	if err := client.SetMaxAttempts(c.maxAttempts); err != nil {
		return nil, fmt.Errorf("configure provider attempts: %w", err)
	}
	c.clients[endpoint] = client
	return client, nil
}

func (c *directClient) Accounting() openaiapi.Accounting {
	return c.accounting.Snapshot()
}

func (c *directClient) PreflightCouncilCandidate(ctx context.Context, member CouncilMember, input []map[string]any, tools []map[string]any) (openaiapi.Response, error) {
	if member.RequestSpec == nil {
		return openaiapi.Response{}, fmt.Errorf("council member %s has no request specification", member.MemberID)
	}
	if supported, known := member.RequestSpec.SupportsParameter("tools"); known && !supported {
		return openaiapi.Response{}, &openaiapi.ProviderError{
			Class: openaiapi.ProviderErrorRequest,
			Err:   fmt.Errorf("council member %s model %s metadata omits required parameter tools", member.MemberID, member.Model),
		}
	}
	spec := councilPreflightRequestSpec(*member.RequestSpec)
	return c.CreateResponseWithRequestSpec(ctx, spec, input, tools, "")
}

func councilPreflightRequestSpec(spec modelrequest.Spec) modelrequest.Spec {
	return spec.WithMaxOutputTokens(councilPreflightMaxOutputTokens)
}
