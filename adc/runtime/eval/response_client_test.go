package eval

import (
	"context"
	"fmt"
	"testing"

	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

type testResponseStep struct {
	response openaiapi.Response
	err      error
}

type testResponseClient struct {
	steps      []testResponseStep
	next       int
	accounting openaiapi.AccountingRecorder
}

func newTestResponseClient(steps ...testResponseStep) *testResponseClient {
	return &testResponseClient{steps: steps}
}

func (c *testResponseClient) CreateResponse(context.Context, string, []map[string]any, []map[string]any, string, *float64) (openaiapi.Response, error) {
	return c.respond()
}

func (c *testResponseClient) CreateResponseWithRequestSpec(context.Context, modelrequest.Spec, []map[string]any, []map[string]any, string) (openaiapi.Response, error) {
	return c.respond()
}

func (c *testResponseClient) respond() (openaiapi.Response, error) {
	if c.next >= len(c.steps) {
		return openaiapi.Response{}, fmt.Errorf("test response client exhausted after %d requests", c.next)
	}
	step := c.steps[c.next]
	c.next++
	c.accounting.Record(step.response)
	return step.response, step.err
}

func (c *testResponseClient) Accounting() openaiapi.Accounting {
	return c.accounting.Snapshot()
}

func TestRecordingResponseClientDelegatesAccountingAndRecords(t *testing.T) {
	t.Parallel()

	base := newTestResponseClient(testResponseStep{response: openaiapi.Response{ResponseID: "response-1"}})
	recorder, err := NewRecordingResponseClient(base)
	if err != nil {
		t.Fatalf("NewRecordingResponseClient error = %v", err)
	}
	if _, err := recorder.CreateResponse(context.Background(), "model-1", nil, nil, "", nil); err != nil {
		t.Fatalf("CreateResponse error = %v", err)
	}
	if len(recorder.Exchanges()) != 1 {
		t.Fatalf("recorded exchanges = %#v", recorder.Exchanges())
	}
	if recorder.Accounting().RequestCount != 1 {
		t.Fatalf("Accounting = %#v", recorder.Accounting())
	}
}
