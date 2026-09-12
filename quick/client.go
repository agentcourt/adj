package quick

import (
	"context"
	"fmt"
	"time"

	"github.com/agentcourt/adj/common/modelgateway"
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

type directClient struct {
	executor *modelgateway.Executor
	initErr  error
}

func newDirectClient(timeout time.Duration, maxAttempts int) *directClient {
	executor, err := modelgateway.New(timeout, maxAttempts)
	return &directClient{executor: executor, initErr: err}
}

func (c *directClient) CreateResponseWithRequestSpec(
	ctx context.Context,
	spec modelrequest.Spec,
	input []map[string]any,
	tools []map[string]any,
	previousResponseID string,
) (openaiapi.Response, error) {
	if c.initErr != nil {
		return openaiapi.Response{}, c.initErr
	}
	if err := c.executor.CheckEndpoint(spec.Endpoint); err != nil {
		return openaiapi.Response{}, err
	}
	response, err := c.executor.CreateResponseWithRequestSpec(ctx, spec, input, tools, previousResponseID)
	if err != nil {
		return openaiapi.Response{}, err
	}
	return response, nil
}

func (c *directClient) Accounting() openaiapi.Accounting {
	if c.executor == nil {
		return openaiapi.Accounting{}
	}
	return c.executor.Accounting()
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
