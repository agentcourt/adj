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
	timeout     time.Duration
	maxAttempts int
	mu          sync.Mutex
	clients     map[string]*openaiapi.Client
	accounting  openaiapi.AccountingRecorder
}

type councilAccountingError struct {
	accounting openaiapi.Accounting
	err        error
}

func (e *councilAccountingError) Error() string { return e.err.Error() }
func (e *councilAccountingError) Unwrap() error { return e.err }

func CouncilAccounting(err error) openaiapi.Accounting {
	var accountingErr *councilAccountingError
	if errors.As(err, &accountingErr) {
		return accountingErr.accounting
	}
	return openaiapi.Accounting{}
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
	c.accounting.Record(resp)
	if err != nil {
		return openaiapi.Response{}, err
	}
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

func (c *directCouncilClient) Accounting() openaiapi.Accounting {
	return c.accounting.Snapshot()
}
