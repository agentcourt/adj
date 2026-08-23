package proceeding

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

type directCouncilClient struct {
	timeout time.Duration
	mu      sync.Mutex
	clients map[string]*openaiapi.Client
}

func newDirectCouncilClient(timeout time.Duration) *directCouncilClient {
	return &directCouncilClient{
		timeout: timeout,
		clients: map[string]*openaiapi.Client{},
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
	return client.CreateResponseWithRequestSpec(ctx, spec, inputItems, tools, previousResponseID)
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
	c.clients[endpoint] = client
	return client, nil
}
