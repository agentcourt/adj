package proceeding

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jsmorph/adj/common/modelrequest"
	openaiapi "github.com/jsmorph/adj/common/openai"
)

type directCouncilClient struct {
	timeout      time.Duration
	maxAttempts  int
	mu           sync.Mutex
	clients      map[string]*openaiapi.Client
	totalCostUSD float64
}

type councilCostError struct {
	costUSD float64
	err     error
}

func (e *councilCostError) Error() string { return e.err.Error() }
func (e *councilCostError) Unwrap() error { return e.err }

func CouncilCostUSD(err error) float64 {
	var costErr *councilCostError
	if errors.As(err, &costErr) {
		return costErr.costUSD
	}
	return 0
}

func newDirectCouncilClient(timeout time.Duration, maxAttempts int) *directCouncilClient {
	return &directCouncilClient{
		timeout:     timeout,
		maxAttempts: maxAttempts,
		clients:     map[string]*openaiapi.Client{},
	}
}

func (c *directCouncilClient) CreateResponseWithRequestSpec(
	ctx context.Context,
	spec modelrequest.Spec,
	inputItems []map[string]any,
	tools []map[string]any,
	previousResponseID string,
) (openaiapi.Response, error) {
	client, err := c.clientForEndpoint(spec.Endpoint)
	if err != nil {
		return openaiapi.Response{}, err
	}
	resp, err := client.CreateResponseWithRequestSpec(ctx, spec, inputItems, tools, previousResponseID)
	if err != nil {
		return openaiapi.Response{}, err
	}
	c.mu.Lock()
	c.totalCostUSD += resp.OpenRouterCostUSD
	c.mu.Unlock()
	return resp, nil
}

func (c *directCouncilClient) clientForEndpoint(endpoint string) (*openaiapi.Client, error) {
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))
	c.mu.Lock()
	defer c.mu.Unlock()
	if client, ok := c.clients[endpoint]; ok {
		return client, nil
	}
	client, err := openaiapi.NewForEndpoint(endpoint, false, c.timeout)
	if err != nil {
		return nil, err
	}
	if err := client.SetMaxAttempts(c.maxAttempts); err != nil {
		return nil, err
	}
	c.clients[endpoint] = client
	return client, nil
}

func (c *directCouncilClient) TotalCostUSD() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.totalCostUSD
}
