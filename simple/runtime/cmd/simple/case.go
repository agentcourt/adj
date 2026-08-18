package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	openaiapi "github.com/jsmorph/adj/common/openai"
	"github.com/jsmorph/adj/simple/runtime/proceeding"
)

type caseErrorSummary struct {
	SchemaVersion string `json:"schema_version"`
	Procedure     string `json:"procedure"`
	Status        string `json:"status"`
	Error         string `json:"error"`
	ErrorClass    string `json:"error_class,omitempty"`
}

var runProcedure = proceeding.Run

func runCase(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("case", flag.ContinueOnError)
	fs.SetOutput(stderr)
	proposition := fs.String("proposition", "", "Proposition to decide")
	documentsRoot := fs.String("documents", "", "Optional document root")
	outDir := fs.String("out-dir", "", "Output directory")
	caseID := fs.String("case-id", "", "Case ID")
	runID := fs.String("run-id", "", "Run ID")
	requestSpec := fs.String("request-spec", "", "Model request-spec JSON file")
	model := fs.String("model", "", "Model as endpoint://model")
	evidenceStandard := fs.String("evidence-standard", "", "Evidence standard applied to the proposition")
	allowAPIKey := fs.Bool("allow-api-key", false, "Allow provider authentication with an API key")
	maxDocuments := fs.Int("max-documents", 0, "Maximum number of documents; required")
	maxDocumentBytes := fs.Int64("max-document-bytes", 0, "Maximum bytes in one document; required")
	maxDocumentsBytes := fs.Int64("max-documents-bytes", 0, "Maximum bytes across all documents; required")
	timeoutSeconds := fs.Int("timeout-seconds", proceeding.DefaultTimeoutSeconds, "Provider request timeout in seconds")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: simple case --proposition TEXT --evidence-standard TEXT --out-dir DIR (--request-spec FILE | --model endpoint://model) --allow-api-key --max-documents N --max-document-bytes N --max-documents-bytes N\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return reportCaseError(stdout, err)
	}
	if fs.NArg() != 0 {
		return reportCaseError(stdout, fmt.Errorf("simple case accepts no positional arguments"))
	}
	result, err := runProcedure(ctx, proceeding.Options{
		Proposition:       *proposition,
		DocumentsRoot:     *documentsRoot,
		OutputDir:         *outDir,
		CaseID:            *caseID,
		RunID:             *runID,
		RequestSpecPath:   *requestSpec,
		Model:             *model,
		EvidenceStandard:  *evidenceStandard,
		AllowAPIKey:       *allowAPIKey,
		MaxDocuments:      *maxDocuments,
		MaxDocumentBytes:  *maxDocumentBytes,
		MaxDocumentsBytes: *maxDocumentsBytes,
		TimeoutSeconds:    *timeoutSeconds,
	})
	if result.SchemaVersion != "" {
		if writeErr := writeJSONLine(stdout, result); writeErr != nil {
			return writeErr
		}
		if err != nil {
			return &reportedError{err: err}
		}
		return nil
	}
	if err != nil {
		return reportCaseError(stdout, err)
	}
	return fmt.Errorf("simple case returned an empty result")
}

func reportCaseError(stdout io.Writer, err error) error {
	summary := caseErrorSummary{
		SchemaVersion: "simple.run.v1",
		Procedure:     "simple",
		Status:        "error",
		Error:         strings.TrimSpace(err.Error()),
		ErrorClass:    string(openaiapi.ErrorClass(err)),
	}
	if writeErr := writeJSONLine(stdout, summary); writeErr != nil {
		return writeErr
	}
	return &reportedError{err: err}
}

func writeJSONLine(w io.Writer, value any) error {
	wire, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal command result: %w", err)
	}
	if _, err := fmt.Fprintln(w, string(wire)); err != nil {
		return fmt.Errorf("write command result: %w", err)
	}
	return nil
}
