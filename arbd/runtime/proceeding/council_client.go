package proceeding

import (
	"context"
	"time"

	"github.com/agentcourt/adj/common/modelgateway"
	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

type directCouncilClient struct {
	executor *modelgateway.Executor
	initErr  error
}

func newDirectCouncilClient(timeout time.Duration) *directCouncilClient {
	executor, err := modelgateway.New(timeout, 4)
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
	return c.executor.CreateResponseWithRequestSpec(ctx, spec, inputItems, tools, previousResponseID)
}
