package proceeding

import (
	"sync"

	"github.com/agentcourt/adj/arb/runtime/lean"
	"github.com/agentcourt/adj/arb/runtime/spec"
	"github.com/agentcourt/adj/common/modelrequest"
	openaiapi "github.com/agentcourt/adj/common/openai"
)

type Policy struct {
	CouncilSize                        int    `json:"council_size"`
	EvidenceStandard                   string `json:"evidence_standard"`
	RequiredVotesForDecision           int    `json:"required_votes_for_decision"`
	MaxDeliberationRounds              int    `json:"max_deliberation_rounds"`
	MaxOpeningChars                    int    `json:"max_opening_chars"`
	MaxArgumentChars                   int    `json:"max_argument_chars"`
	MaxRebuttalChars                   int    `json:"max_rebuttal_chars"`
	MaxSurrebuttalChars                int    `json:"max_surrebuttal_chars"`
	MaxClosingChars                    int    `json:"max_closing_chars"`
	MaxExhibitsPerFiling               int    `json:"max_exhibits_per_filing"`
	MaxExhibitsPerSide                 int    `json:"max_exhibits_per_side"`
	MaxExhibitBytes                    int    `json:"max_exhibit_bytes"`
	MaxReportsPerFiling                int    `json:"max_reports_per_filing"`
	MaxReportsPerSide                  int    `json:"max_reports_per_side"`
	MaxReportTitleBytes                int    `json:"max_report_title_bytes"`
	MaxReportSummaryBytes              int    `json:"max_report_summary_bytes"`
	MaxSubmittedEvidencePerSide        int    `json:"max_submitted_evidence_per_side"`
	MaxSubmittedEvidenceBytes          int    `json:"max_submitted_evidence_bytes"`
	MaxDirectSubmittedEvidenceBytes    int    `json:"max_direct_submitted_evidence_bytes"`
	MaxEvidenceUploadBytes             int    `json:"max_evidence_upload_bytes"`
	MaxEvidenceChunkBytes              int    `json:"max_evidence_chunk_bytes"`
	MaxEvidenceReadBytes               int    `json:"max_evidence_read_bytes"`
	MaxEvidenceReadsPerOpportunity     int    `json:"max_evidence_reads_per_opportunity"`
	MaxEvidenceReadBytesPerOpportunity int    `json:"max_evidence_read_bytes_per_opportunity"`
}

type RuntimeLimits struct {
	CouncilLLMTimeoutSeconds int   `json:"council_llm_timeout_seconds"`
	CouncilRequestAttempts   int   `json:"council_request_attempts"`
	LawyerTurnTimeoutSeconds int   `json:"lawyer_turn_timeout_seconds"`
	EngineCallTimeoutSeconds int   `json:"engine_call_timeout_seconds"`
	MaxResponseBytes         int   `json:"max_response_bytes"`
	InvalidAttemptLimit      int   `json:"invalid_attempt_limit"`
	CouncilMaxOutputTokens   int64 `json:"council_max_output_tokens"`
}

type AttorneyRunInfo struct {
	Role      string `json:"role"`
	Interface string `json:"interface"`
}

type Options struct {
	ComplaintPath            string
	CaseFiles                []string
	OutputDir                string
	PolicyPath               string
	CouncilSize              int
	RequiredVotes            int
	EvidenceStandard         string
	PromptDir                string
	PromptFiles              map[string]string
	CommonRoot               string
	CouncilPoolPath          string
	CaseAPIAddr              string
	CouncilBackend           string
	CouncilTimeoutSeconds    int
	CouncilRequestAttempts   int
	LawyerTimeoutSeconds     int
	EngineCallTimeoutSeconds int
	MaxResponseBytes         int
	InvalidAttemptLimit      int
	EnginePath               string
	RunID                    string
	CaseID                   string
}

type Config struct {
	CaseID           string
	RunID            string
	ComplaintPath    string
	CaseFilePaths    []string
	OutputDir        string
	CommonRoot       string
	CouncilPoolPath  string
	PromptDir        string
	PromptFiles      map[string]string
	CaseAPIAddr      string
	Policy           Policy
	Runtime          RuntimeLimits
	CouncilBackend   string
	Engine           lean.Engine
	promptSources    map[string]string
	toolDescriptions map[string]string
	observerPrompt   string
}

type Result struct {
	CaseID            string                  `json:"case_id"`
	RunID             string                  `json:"run_id"`
	StartedAt         string                  `json:"started_at"`
	FinishedAt        string                  `json:"finished_at"`
	Status            string                  `json:"status"`
	Error             string                  `json:"error,omitempty"`
	ErrorClass        string                  `json:"error_class,omitempty"`
	Failure           map[string]any          `json:"failure,omitempty"`
	Phase             string                  `json:"phase"`
	Resolution        string                  `json:"resolution"`
	Complaint         spec.Complaint          `json:"complaint"`
	EvidenceStandard  string                  `json:"evidence_standard"`
	CouncilBackend    string                  `json:"council_backend"`
	Attorneys         []AttorneyRunInfo       `json:"attorneys"`
	CaseFiles         []CaseFileMeta          `json:"case_files"`
	SubmittedEvidence []SubmittedEvidenceMeta `json:"submitted_evidence,omitempty"`
	Evidence          []EvidenceMeta          `json:"evidence,omitempty"`
	Council           []CouncilSeat           `json:"council"`
	Events            []Event                 `json:"events"`
	FinalState        map[string]any          `json:"final_state"`
	FinalReason       string                  `json:"final_reason"`
	Provider          openaiapi.Accounting    `json:"provider"`
}

type CaseFile struct {
	EvidenceID   string
	Name         string
	Path         string
	MimeType     string
	TextReadable bool
	SizeBytes    int
	Text         string
}

type CaseFileMeta struct {
	EvidenceID   string `json:"evidence_id"`
	Name         string `json:"name"`
	MimeType     string `json:"mime_type"`
	TextReadable bool   `json:"text_readable"`
}

type EvidenceCommitment struct {
	EvidenceID string `json:"evidence_id"`
	SHA256     string `json:"sha256"`
	SizeBytes  int    `json:"size_bytes"`
}

type SubmittedEvidenceMeta struct {
	Phase              string `json:"phase"`
	Role               string `json:"role"`
	EvidenceID         string `json:"evidence_id"`
	Name               string `json:"name"`
	Title              string `json:"title"`
	SourceURL          string `json:"source_url,omitempty"`
	SourceDescription  string `json:"source_description,omitempty"`
	MimeType           string `json:"mime_type"`
	RetrievalTimestamp string `json:"retrieval_timestamp"`
	Relevance          string `json:"relevance"`
	SHA256             string `json:"sha256"`
	SizeBytes          int    `json:"size_bytes"`
	ParentEvidenceID   string `json:"parent_evidence_id,omitempty"`
	ParentSHA256       string `json:"parent_sha256,omitempty"`
	DerivationMethod   string `json:"derivation_method,omitempty"`
}

type EvidenceUploadSession struct {
	UploadID           string
	Role               string
	Phase              string
	Title              string
	MimeType           string
	ExpectedSizeBytes  int
	ExpectedSHA256     string
	SourceURL          string
	SourceDescription  string
	RetrievalTimestamp string
	Relevance          string
	ParentEvidenceID   string
	ParentSHA256       string
	DerivationMethod   string
	Path               string
	ReceivedBytes      int
}

type EvidenceMeta struct {
	EvidenceID          string `json:"evidence_id"`
	SHA256              string `json:"sha256"`
	SizeBytes           int    `json:"size_bytes"`
	MimeType            string `json:"mime_type"`
	StorageName         string `json:"storage_name"`
	CreatedAt           string `json:"created_at"`
	AdmissibilityStatus string `json:"admissibility_status"`
	RecordVisibility    string `json:"record_visibility"`
	Title               string `json:"title,omitempty"`
	OriginalName        string `json:"original_name,omitempty"`
	SourceURL           string `json:"source_url,omitempty"`
	SourceDescription   string `json:"source_description,omitempty"`
	RetrievalTimestamp  string `json:"retrieval_timestamp,omitempty"`
	SubmittedByRole     string `json:"submitted_by_role,omitempty"`
	SubmittedPhase      string `json:"submitted_phase,omitempty"`
	ParentEvidenceID    string `json:"parent_evidence_id,omitempty"`
	ParentSHA256        string `json:"parent_sha256,omitempty"`
	DerivationMethod    string `json:"derivation_method,omitempty"`
	Relevance           string `json:"relevance,omitempty"`
	TextReadable        bool   `json:"text_readable"`
}

type CouncilSeat struct {
	MemberID    string             `json:"member_id"`
	Model       string             `json:"model"`
	PersonaFile string             `json:"persona_file"`
	RequestSpec *modelrequest.Spec `json:"request_spec,omitempty"`
	PersonaText string             `json:"-"`
}

type Opportunity struct {
	ID           string
	StateVersion int
	Role         string
	Phase        string
	MemberID     string
	MayPass      bool
	Objective    string
	AllowedTools []string
}

type Event struct {
	Timestamp string         `json:"timestamp"`
	Turn      int            `json:"turn"`
	Role      string         `json:"role"`
	Phase     string         `json:"phase"`
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type WorkNote struct {
	Timestamp     string `json:"timestamp"`
	Turn          int    `json:"turn"`
	Role          string `json:"role"`
	Phase         string `json:"phase"`
	OpportunityID string `json:"opportunity_id"`
	CallID        string `json:"call_id,omitempty"`
	Notes         string `json:"notes"`
}

type runContext struct {
	// mu protects mutable case data and both role API turn records. Methods ending
	// in Locked require it. Provider calls and HTTP writes run without it.
	mu                 sync.Mutex
	cfg                Config
	complaint          spec.Complaint
	state              map[string]any
	certificateInit    ReplayInitializeRequest
	certificateActions []ReplayAction
	caseFiles          []CaseFile
	fileByID           map[string]CaseFile
	submittedEvidence  []SubmittedEvidenceMeta
	evidence           []EvidenceMeta
	evidenceByID       map[string]EvidenceMeta
	evidenceStoreDir   string
	uploadSessions     map[string]*EvidenceUploadSession
	council            []CouncilSeat
	attorneys          map[string]AttorneyRunInfo
	lawyerAPI          *lawyerAPIServer
	councilAPI         *councilAPIServer
	workProductDirs    map[string]string
	events             []Event
	turn               int
	providerErrorClass string
	terminal           bool
	terminalReason     string
	responseErrMu      sync.Mutex
	responseErr        error
}
