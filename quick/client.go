package quick

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jsmorph/adj/common/modelrequest"
	openaiapi "github.com/jsmorph/adj/common/openai"
)

type responseClient interface {
	CreateResponseWithRequestSpec(
		context.Context,
		modelrequest.Spec,
		[]map[string]any,
		[]map[string]any,
		string,
	) (openaiapi.Response, error)
	TotalCostUSD() float64
}

type councilEndpointPreflighter interface {
	PreflightCouncilEndpoints([]CouncilMember) error
}

type directClient struct {
	timeout      time.Duration
	maxAttempts  int
	mu           sync.Mutex
	clients      map[string]*openaiapi.Client
	totalCostUSD float64
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
		return openaiapi.Response{}, err
	}
	response, err := client.CreateResponseWithRequestSpec(ctx, spec, input, tools, previousResponseID)
	if err != nil {
		return openaiapi.Response{}, err
	}
	c.mu.Lock()
	c.totalCostUSD += response.OpenRouterCostUSD
	c.mu.Unlock()
	return response, nil
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

func (c *directClient) TotalCostUSD() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.totalCostUSD
}

func (c *directClient) PreflightCouncilEndpoints(council []CouncilMember) error {
	seen := make(map[string]struct{}, len(council))
	var preflightErr error
	for _, member := range council {
		if member.RequestSpec == nil {
			preflightErr = errors.Join(preflightErr, fmt.Errorf("council member %s has no request specification", member.MemberID))
			continue
		}
		if supported, known := member.RequestSpec.SupportsParameter("tools"); known && !supported {
			preflightErr = errors.Join(preflightErr, &openaiapi.ProviderError{
				Class: openaiapi.ProviderErrorRequest,
				Err:   fmt.Errorf("council member %s model %s metadata omits required parameter tools", member.MemberID, member.Model),
			})
		}
		endpoint := strings.ToLower(strings.TrimSpace(member.RequestSpec.Endpoint))
		if _, ok := seen[endpoint]; ok {
			continue
		}
		seen[endpoint] = struct{}{}
		if _, err := c.clientForEndpoint(endpoint); err != nil {
			preflightErr = errors.Join(preflightErr, fmt.Errorf("initialize council endpoint %s: %w", endpoint, err))
		}
	}
	return preflightErr
}
