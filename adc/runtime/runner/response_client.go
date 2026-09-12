package runner

import (
	"context"

	"github.com/agentcourt/adj/common/modelgateway"
	"github.com/agentcourt/adj/common/modelrequest"
	"github.com/agentcourt/adj/common/openai"
)

// ResponseClient is the model-response API used by the runner.
type ResponseClient interface {
	CreateResponse(
		ctx context.Context,
		model string,
		inputItems []map[string]any,
		tools []map[string]any,
		previousResponseID string,
		temperature *float64,
	) (openai.Response, error)
	CreateResponseWithRequestSpec(
		ctx context.Context,
		spec modelrequest.Spec,
		inputItems []map[string]any,
		tools []map[string]any,
		previousResponseID string,
	) (openai.Response, error)
	Accounting() openai.Accounting
}

var _ ResponseClient = (*openai.Client)(nil)
var _ ResponseClient = (*modelgateway.Executor)(nil)
