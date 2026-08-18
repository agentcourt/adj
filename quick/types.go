package quick

import (
	"time"

	"github.com/jsmorph/adj/common/documents"
	"github.com/jsmorph/adj/common/modelrequest"
	openaiapi "github.com/jsmorph/adj/common/openai"
)

const (
	Procedure               = "quick"
	ResultSchemaVersion     = "adj.quick.result.v1"
	DefaultCaseAPIAddr      = "127.0.0.1:0"
	DefaultLawyerTimeout    = 15 * time.Minute
	DefaultCouncilTimeout   = 3 * time.Minute
	DefaultMaxResponseBytes = 1 << 20
	DefaultMaxArgumentChars = 40_000
	DefaultInvalidAttempts  = 1
	DefaultProviderAttempts = 1
)

type Options struct {
	Proposition            string
	DocumentsDir           string
	OutputDir              string
	CouncilPoolPath        string
	CouncilSize            int
	RequiredVotes          int
	EvidenceStandard       string
	CaseAPIAddr            string
	CaseID                 string
	RunID                  string
	LawyerTimeout          time.Duration
	CouncilTimeout         time.Duration
	MaxResponseBytes       int
	MaxArgumentChars       int
	InvalidAttemptLimit    int
	MaxDocumentFiles       int
	MaxDocumentFileBytes   int64
	MaxDocumentsTotal      int64
	CouncilRequestAttempts int
	ParallelCouncil        bool
	AllowAPIKey            bool
}

type Config struct {
	Proposition            string
	DocumentsDir           string
	OutputDir              string
	CouncilPoolPath        string
	CouncilSize            int
	RequiredVotes          int
	EvidenceStandard       string
	CaseAPIAddr            string
	CaseID                 string
	RunID                  string
	LawyerTimeout          time.Duration
	CouncilTimeout         time.Duration
	MaxResponseBytes       int
	MaxArgumentChars       int
	InvalidAttemptLimit    int
	DocumentLimits         documents.Limits
	CouncilRequestAttempts int
	ParallelCouncil        bool
	AllowAPIKey            bool
}

type Argument struct {
	Role          string    `json:"role"`
	Text          string    `json:"text"`
	SubmittedAt   time.Time `json:"submitted_at"`
	OpportunityID string    `json:"opportunity_id"`
}

type CouncilMember struct {
	MemberID    string             `json:"member_id"`
	Model       string             `json:"model"`
	PersonaFile string             `json:"persona_file"`
	RequestSpec *modelrequest.Spec `json:"-"`
	PersonaText string             `json:"-"`
}

type Vote struct {
	MemberID              string           `json:"member_id"`
	Model                 string           `json:"model"`
	PersonaFile           string           `json:"persona_file"`
	Vote                  string           `json:"vote"`
	Rationale             string           `json:"rationale"`
	ResponseID            string           `json:"response_id,omitempty"`
	ProviderUsage         *openaiapi.Usage `json:"provider_usage,omitempty"`
	ProviderCostUSD       *float64         `json:"provider_cost_usd,omitempty"`
	ProviderMetadataError string           `json:"provider_metadata_error,omitempty"`
	SubmittedAt           time.Time        `json:"submitted_at"`
}

type Event struct {
	Sequence  int            `json:"sequence"`
	Timestamp time.Time      `json:"timestamp"`
	Type      string         `json:"type"`
	Role      string         `json:"role,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type Transcript struct {
	SchemaVersion string     `json:"schema_version"`
	CaseID        string     `json:"case_id"`
	Proposition   string     `json:"proposition"`
	Arguments     []Argument `json:"arguments"`
	Votes         []Vote     `json:"votes"`
}

type Result struct {
	SchemaVersion    string             `json:"schema_version"`
	Procedure        string             `json:"procedure"`
	CaseID           string             `json:"case_id"`
	RunID            string             `json:"run_id"`
	Status           string             `json:"status"`
	Phase            string             `json:"phase"`
	Proposition      string             `json:"proposition"`
	Resolution       string             `json:"resolution,omitempty"`
	CouncilSize      int                `json:"council_size"`
	RequiredVotes    int                `json:"required_votes"`
	EvidenceStandard string             `json:"evidence_standard"`
	VotesFor         int                `json:"votes_for"`
	VotesAgainst     int                `json:"votes_against"`
	Error            string             `json:"error,omitempty"`
	ErrorClass       string             `json:"error_class,omitempty"`
	CaseAPIBase      string             `json:"case_api_base,omitempty"`
	StartedAt        time.Time          `json:"started_at"`
	FinishedAt       time.Time          `json:"finished_at,omitempty"`
	Documents        documents.Manifest `json:"documents"`
	Council          []CouncilMember    `json:"council"`
	Arguments        []Argument         `json:"arguments"`
	Votes            []Vote             `json:"votes"`
	Events           []Event            `json:"events"`
	CouncilUsage     *openaiapi.Usage   `json:"council_usage,omitempty"`
	CouncilCostUSD   *float64           `json:"council_cost_usd,omitempty"`
}

type inputRecord struct {
	SchemaVersion          string           `json:"schema_version"`
	Procedure              string           `json:"procedure"`
	Proposition            string           `json:"proposition"`
	CaseID                 string           `json:"case_id"`
	RunID                  string           `json:"run_id"`
	DocumentsSource        string           `json:"documents_source,omitempty"`
	CouncilPoolPath        string           `json:"council_pool_path"`
	CouncilSize            int              `json:"council_size"`
	RequiredVotes          int              `json:"required_votes"`
	EvidenceStandard       string           `json:"evidence_standard"`
	CaseAPIAddr            string           `json:"case_api_addr"`
	LawyerTimeout          string           `json:"lawyer_timeout"`
	CouncilTimeout         string           `json:"council_timeout"`
	MaxResponseBytes       int              `json:"max_response_bytes"`
	MaxArgumentChars       int              `json:"max_argument_chars"`
	InvalidAttemptLimit    int              `json:"invalid_attempt_limit"`
	DocumentLimits         documents.Limits `json:"document_limits"`
	CouncilRequestAttempts int              `json:"council_request_attempts"`
	ParallelCouncil        bool             `json:"parallel_council"`
	DirectAPIKeyAuthorized bool             `json:"direct_api_key_authorized"`
}

type runtimeRecord struct {
	SchemaVersion string    `json:"schema_version"`
	Procedure     string    `json:"procedure"`
	CaseID        string    `json:"case_id"`
	RunID         string    `json:"run_id"`
	CaseAPIBase   string    `json:"case_api_base"`
	StartedAt     time.Time `json:"started_at"`
}
