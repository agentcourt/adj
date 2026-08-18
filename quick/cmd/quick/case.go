package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jsmorph/adj/common/cliio"
	"github.com/jsmorph/adj/common/documents"
	openaiapi "github.com/jsmorph/adj/common/openai"
	"github.com/jsmorph/adj/quick"
)

var runProcedure = quick.Run

func runCase(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("case", flag.ContinueOnError)
	flagErrors := cliio.NewErrorWriter(exactWriter{Writer: stderr})
	fs.SetOutput(flagErrors)
	proposition := fs.String("proposition", "", "Proposition to adjudicate")
	documentsDir := fs.String("documents", "", "Directory of immutable case documents")
	outDir := fs.String("out-dir", "", "Empty output directory")
	councilPool := fs.String("council-pool", "", "Council JSONL request-spec pool")
	councilSize := fs.Int("council-size", 0, "Council member count")
	requiredVotes := fs.Int("required-votes", 0, "Votes required for a decision")
	evidenceStandard := fs.String("evidence-standard", "", "Standard the council applies to the proposition")
	caseAPIAddr := fs.String("caseapi-addr", quick.DefaultCaseAPIAddr, "Private lawyer API listen address")
	caseID := fs.String("case-id", "quick-1", "Case identifier")
	runID := fs.String("run-id", "", "Run identifier")
	lawyerTimeout := fs.Duration("lawyer-timeout", quick.DefaultLawyerTimeout, "Maximum duration of each lawyer turn")
	councilTimeout := fs.Duration("council-timeout", quick.DefaultCouncilTimeout, "Maximum duration of each council request")
	maxResponseBytes := fs.Int("max-response-bytes", quick.DefaultMaxResponseBytes, "Maximum lawyer and council response bytes")
	maxArgumentChars := fs.Int("max-argument-chars", quick.DefaultMaxArgumentChars, "Maximum characters in one lawyer argument")
	invalidAttemptLimit := fs.Int("invalid-attempt-limit", quick.DefaultInvalidAttempts, "Maximum invalid submissions per lawyer or council member")
	maxDocumentFiles := fs.Int("max-document-files", 0, "Maximum number of imported documents")
	maxDocumentFileBytes := fs.Int64("max-document-file-bytes", 0, "Maximum bytes in one imported document")
	maxDocumentsTotalBytes := fs.Int64("max-documents-total-bytes", 0, "Maximum total imported document bytes")
	councilRequestAttempts := fs.Int("council-request-attempts", quick.DefaultProviderAttempts, "Provider request attempts per council response: 1 through 4")
	parallelCouncil := fs.Bool("parallel-council", false, "Request all council votes concurrently")
	allowAPIKey := fs.Bool("allow-api-key", false, "Authorize direct provider calls billed through configured API keys")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: quick case --proposition TEXT --out-dir DIR --council-pool FILE --council-size N --required-votes N --evidence-standard TEXT --max-document-files N --max-document-file-bytes N --max-documents-total-bytes N --allow-api-key")
		fmt.Fprintln(fs.Output())
		fs.PrintDefaults()
	}
	help, err := cliio.Parse(fs, args, flagErrors)
	if err != nil {
		startedAt := time.Now().UTC()
		if strings.TrimSpace(*runID) == "" {
			generated, idErr := commandRunID()
			err = errors.Join(err, idErr)
			*runID = generated
		}
		return reportCaseError(stdout, earlyResult(*proposition, *caseID, *runID, *councilSize, *requiredVotes, *evidenceStandard, startedAt), err)
	}
	if help {
		return nil
	}
	startedAt := time.Now().UTC()
	if strings.TrimSpace(*runID) == "" {
		*runID, err = commandRunID()
		if err != nil {
			return reportCaseError(stdout, earlyResult(*proposition, *caseID, "", *councilSize, *requiredVotes, *evidenceStandard, startedAt), err)
		}
	}
	if fs.NArg() != 0 {
		return reportCaseError(stdout, earlyResult(*proposition, *caseID, *runID, *councilSize, *requiredVotes, *evidenceStandard, startedAt), fmt.Errorf("quick case accepts no positional arguments"))
	}
	if strings.TrimSpace(*proposition) == "" || strings.TrimSpace(*outDir) == "" || strings.TrimSpace(*councilPool) == "" || strings.TrimSpace(*evidenceStandard) == "" {
		return reportCaseError(stdout, earlyResult(*proposition, *caseID, *runID, *councilSize, *requiredVotes, *evidenceStandard, startedAt), fmt.Errorf("--proposition, --out-dir, --council-pool, and --evidence-standard are required"))
	}
	result, runErr := runProcedure(ctx, quick.Options{
		Proposition:            *proposition,
		DocumentsDir:           *documentsDir,
		OutputDir:              *outDir,
		CouncilPoolPath:        *councilPool,
		CouncilSize:            *councilSize,
		RequiredVotes:          *requiredVotes,
		EvidenceStandard:       *evidenceStandard,
		CaseAPIAddr:            *caseAPIAddr,
		CaseID:                 *caseID,
		RunID:                  *runID,
		LawyerTimeout:          *lawyerTimeout,
		CouncilTimeout:         *councilTimeout,
		MaxResponseBytes:       *maxResponseBytes,
		MaxArgumentChars:       *maxArgumentChars,
		InvalidAttemptLimit:    *invalidAttemptLimit,
		MaxDocumentFiles:       *maxDocumentFiles,
		MaxDocumentFileBytes:   *maxDocumentFileBytes,
		MaxDocumentsTotal:      *maxDocumentsTotalBytes,
		CouncilRequestAttempts: *councilRequestAttempts,
		ParallelCouncil:        *parallelCouncil,
		AllowAPIKey:            *allowAPIKey,
	})
	if result.SchemaVersion == "" {
		if runErr == nil {
			runErr = fmt.Errorf("quick case returned an empty result")
		}
		result = earlyResult(*proposition, *caseID, *runID, *councilSize, *requiredVotes, *evidenceStandard, startedAt)
		result.Error = runErr.Error()
		result.ErrorClass = string(openaiapi.ErrorClass(runErr))
	}
	writeErr := writeResult(stdout, result)
	return errors.Join(runErr, writeErr)
}

func reportCaseError(stdout io.Writer, result quick.Result, runErr error) error {
	result.Error = strings.TrimSpace(runErr.Error())
	result.ErrorClass = string(openaiapi.ErrorClass(runErr))
	return errors.Join(runErr, writeResult(stdout, result))
}

func earlyResult(proposition, caseID, runID string, councilSize, requiredVotes int, evidenceStandard string, startedAt time.Time) quick.Result {
	return quick.Result{
		SchemaVersion:    quick.ResultSchemaVersion,
		Procedure:        quick.Procedure,
		CaseID:           strings.TrimSpace(caseID),
		RunID:            strings.TrimSpace(runID),
		Status:           "failed",
		Phase:            "failed",
		Proposition:      strings.TrimSpace(proposition),
		CouncilSize:      councilSize,
		RequiredVotes:    requiredVotes,
		EvidenceStandard: strings.TrimSpace(evidenceStandard),
		StartedAt:        startedAt,
		FinishedAt:       time.Now().UTC(),
		Documents:        documents.Manifest{SchemaVersion: documents.SchemaVersion, Files: []documents.File{}},
		Council:          []quick.CouncilMember{},
		Arguments:        []quick.Argument{},
		Votes:            []quick.Vote{},
		Events:           []quick.Event{},
	}
}

func commandRunID() (string, error) {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate run ID: %w", err)
	}
	return "quick-" + hex.EncodeToString(raw[:]), nil
}

func writeResult(w io.Writer, value quick.Result) error {
	wire, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal quick result: %w", err)
	}
	wire = append(wire, '\n')
	written, writeErr := w.Write(wire)
	if written != len(wire) {
		writeErr = errors.Join(writeErr, io.ErrShortWrite)
	}
	if writeErr != nil {
		return fmt.Errorf("write quick result: %w", writeErr)
	}
	return nil
}

type exactWriter struct {
	io.Writer
}

func (w exactWriter) Write(value []byte) (int, error) {
	written, err := w.Writer.Write(value)
	if written != len(value) {
		err = errors.Join(err, io.ErrShortWrite)
	}
	return written, err
}
