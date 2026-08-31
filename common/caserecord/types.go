package caserecord

import "time"

const SchemaVersion = "adj.case-record.v1"

const (
	SourcePathBaseAbsolute = "absolute"
	SourcePathBaseIndex    = "index"
)

type Options struct {
	Dir              string
	IncludeWorkNotes bool
	IncludeSessions  bool
}

type Record struct {
	SchemaVersion string       `json:"schema_version"`
	GeneratedAt   time.Time    `json:"generated_at"`
	Procedure     string       `json:"procedure"`
	CaseID        string       `json:"case_id"`
	RunID         string       `json:"run_id"`
	Sources       []Source     `json:"sources"`
	Docket        []DocketItem `json:"docket"`
	Artifacts     []Artifact   `json:"artifacts"`
}

type Source struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Path      string `json:"path"`
	PathBase  string `json:"path_base"`
	Role      string `json:"role,omitempty"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

type Reference struct {
	SourceID    string `json:"source_id"`
	Path        string `json:"path"`
	JSONPointer string `json:"json_pointer,omitempty"`
	Line        int    `json:"line,omitempty"`
}

type DocketItem struct {
	ID              string      `json:"id"`
	Sequence        int         `json:"sequence"`
	Timestamp       time.Time   `json:"timestamp"`
	TimestampSource string      `json:"timestamp_source"`
	Phase           string      `json:"phase,omitempty"`
	Actor           string      `json:"actor,omitempty"`
	Kind            string      `json:"kind"`
	Title           string      `json:"title"`
	Description     string      `json:"description,omitempty"`
	Access          string      `json:"access"`
	Source          Reference   `json:"source"`
	ArtifactRefs    []Reference `json:"artifact_refs,omitempty"`
	order           int
}

type Artifact struct {
	SourceID        string    `json:"source_id"`
	Path            string    `json:"path"`
	MediaType       string    `json:"media_type"`
	SizeBytes       int64     `json:"size_bytes"`
	Category        string    `json:"category"`
	Access          string    `json:"access"`
	Timestamp       time.Time `json:"timestamp"`
	TimestampSource string    `json:"timestamp_source"`
	RecordedSHA256  string    `json:"recorded_sha256,omitempty"`
}
