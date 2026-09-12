package proceeding

import (
	"context"
	"errors"
	"time"

	"github.com/agentcourt/adj/common/modelgateway"
	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

type directCouncilClient struct {
	executor *modelgateway.Executor
	initErr  error
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
	executor, err := modelgateway.New(timeout, maxAttempts)
	return &directCouncilClient{executor: executor, initErr: err}
}

func (c *directCouncilClient) CreateResponseWithRequestSpec(
	ctx context.Context,
	spec modelrequest.Spec,
	inputItems []map[string]any,
	tools []map[string]any,
	previousResponseID string,
) (openaiapi.Response, error) {
	if c.initErr != nil {
		return openaiapi.Response{}, c.initErr
	}
	resp, err := c.executor.CreateResponseWithRequestSpec(ctx, spec, inputItems, tools, previousResponseID)
	if err != nil {
		return openaiapi.Response{}, err
	}
	return resp, nil
}

func (c *directCouncilClient) Accounting() openaiapi.Accounting {
	if c.executor == nil {
		return openaiapi.Accounting{}
	}
	return c.executor.Accounting()
}
