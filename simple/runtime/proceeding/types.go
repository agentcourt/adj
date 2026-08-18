package proceeding

import (
	"context"
	"time"

	"github.com/jsmorph/adj/common/documents"
	"github.com/jsmorph/adj/common/modelrequest"
	openaiapi "github.com/jsmorph/adj/common/openai"
)

const (
	RunSchemaVersion      = "simple.run.v1"
	InputSchemaVersion    = "simple.input.v1"
	RuntimeSchemaVersion  = "simple.runtime.v1"
	DecisionSchemaVersion = "simple.decision.v1"
	StateSchemaVersion    = "simple.state.v1"
	RequestSchemaVersion  = "simple.model-request.v1"
	ResponseSchemaVersion = "simple.model-response.v1"
	EventSchemaVersion    = "simple.event.v1"

	DefaultCaseID          = "simple-1"
	DefaultTimeoutSeconds  = 240
	DefaultMaxOutputTokens = int64(4096)
)

type Options struct {
	Proposition       string
	DocumentsRoot     string
	OutputDir         string
	CaseID            string
	RunID             string
	RequestSpecPath   string
	Model             string
	EvidenceStandard  string
	AllowAPIKey       bool
	MaxDocuments      int
	MaxDocumentBytes  int64
	MaxDocumentsBytes int64
	TimeoutSeconds    int
}

type Runtime struct {
	SchemaVersion          string `json:"schema_version"`
	EvidenceStandard       string `json:"evidence_standard"`
	AllowAPIKey            bool   `json:"allow_api_key"`
	MaxDocuments           int    `json:"max_documents"`
	MaxDocumentBytes       int64  `json:"max_document_bytes"`
	MaxDocumentsBytes      int64  `json:"max_documents_bytes"`
	ProviderTimeoutSeconds int    `json:"provider_timeout_seconds"`
	ProviderAttempts       int    `json:"provider_attempts"`
}

type Decision struct {
	Value     string `json:"value"`
	Rationale string `json:"rationale"`
}

type Result struct {
	SchemaVersion    string               `json:"schema_version"`
	Procedure        string               `json:"procedure"`
	CaseID           string               `json:"case_id"`
	RunID            string               `json:"run_id"`
	StartedAt        string               `json:"started_at"`
	FinishedAt       string               `json:"finished_at"`
	Status           string               `json:"status"`
	Phase            string               `json:"phase"`
	Proposition      string               `json:"proposition"`
	EvidenceStandard string               `json:"evidence_standard"`
	Decision         *Decision            `json:"decision,omitempty"`
	Error            string               `json:"error,omitempty"`
	ErrorClass       string               `json:"error_class,omitempty"`
	ResponseID       string               `json:"response_id,omitempty"`
	Provider         openaiapi.Accounting `json:"provider"`
	Documents        documents.Manifest   `json:"documents"`
}

type ResponseClient interface {
	CreateResponseWithRequestSpec(
		ctx context.Context,
		spec modelrequest.Spec,
		inputItems []map[string]any,
		tools []map[string]any,
		previousResponseID string,
	) (openaiapi.Response, error)
}

type ClientFactory interface {
	New(spec modelrequest.Spec, timeout time.Duration) (ResponseClient, error)
}

type ClientFactoryFunc func(spec modelrequest.Spec, timeout time.Duration) (ResponseClient, error)

func (f ClientFactoryFunc) New(spec modelrequest.Spec, timeout time.Duration) (ResponseClient, error) {
	return f(spec, timeout)
}

type InputRecord struct {
	SchemaVersion    string `json:"schema_version"`
	CaseID           string `json:"case_id"`
	RunID            string `json:"run_id"`
	Proposition      string `json:"proposition"`
	EvidenceStandard string `json:"evidence_standard"`
	DocumentsPresent bool   `json:"documents_present"`
	DocumentsRoot    string `json:"documents_root,omitempty"`
}

type DecisionRecord struct {
	SchemaVersion string    `json:"schema_version"`
	Status        string    `json:"status"`
	Decision      *Decision `json:"decision,omitempty"`
	Error         string    `json:"error,omitempty"`
	ErrorClass    string    `json:"error_class,omitempty"`
}

type StateRecord struct {
	SchemaVersion    string             `json:"schema_version"`
	CaseID           string             `json:"case_id"`
	RunID            string             `json:"run_id"`
	Status           string             `json:"status"`
	Phase            string             `json:"phase"`
	Proposition      string             `json:"proposition"`
	EvidenceStandard string             `json:"evidence_standard"`
	Documents        documents.Manifest `json:"documents"`
	Decision         *Decision          `json:"decision,omitempty"`
	Error            string             `json:"error,omitempty"`
	ErrorClass       string             `json:"error_class,omitempty"`
}

type ModelRequestRecord struct {
	SchemaVersion    string            `json:"schema_version"`
	RequestSpec      modelrequest.Spec `json:"request_spec"`
	DeveloperPrompt  string            `json:"developer_prompt"`
	Proposition      string            `json:"proposition"`
	EvidenceStandard string            `json:"evidence_standard"`
	Documents        []documents.File  `json:"documents"`
	Tools            []map[string]any  `json:"tools"`
	Error            string            `json:"error,omitempty"`
}

type ModelResponseRecord struct {
	SchemaVersion           string           `json:"schema_version"`
	Status                  string           `json:"status"`
	ResponseID              string           `json:"response_id,omitempty"`
	Text                    string           `json:"text,omitempty"`
	ToolCalls               []ToolCallRecord `json:"tool_calls,omitempty"`
	RawResponse             string           `json:"raw_response,omitempty"`
	ProviderMetadata        map[string]any   `json:"provider_metadata,omitempty"`
	ProviderGeneration      map[string]any   `json:"provider_generation,omitempty"`
	ProviderGenerationError string           `json:"provider_generation_error,omitempty"`
	ProviderUsage           *openaiapi.Usage `json:"provider_usage,omitempty"`
	RecoveredCostUSD        *float64         `json:"recovered_cost_usd,omitempty"`
	Error                   string           `json:"error,omitempty"`
	ErrorClass              string           `json:"error_class,omitempty"`
}

type ToolCallRecord struct {
	CallID         string         `json:"call_id,omitempty"`
	Name           string         `json:"name"`
	Arguments      map[string]any `json:"arguments,omitempty"`
	RawArguments   string         `json:"raw_arguments"`
	ArgumentsError string         `json:"arguments_error,omitempty"`
}

type Event struct {
	SchemaVersion string         `json:"schema_version"`
	Timestamp     string         `json:"timestamp"`
	Sequence      int            `json:"sequence"`
	Type          string         `json:"type"`
	Payload       map[string]any `json:"payload,omitempty"`
}
