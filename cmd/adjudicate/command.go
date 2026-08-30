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
	"os"
	"strings"
	"time"

	"github.com/agentcourt/adj/common/cliio"
	service "github.com/agentcourt/adj/runtime/adjudicate"
)

const randomIDBytes = 16

type adjudicationEngine interface {
	Run(context.Context, service.Request) (service.Result, error)
}

type commandDependencies struct {
	newEngine func() (adjudicationEngine, error)
	random    io.Reader
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return runWithDependencies(ctx, args, stdout, stderr, commandDependencies{
		newEngine: newDefaultEngine,
		random:    rand.Reader,
	})
}

func runWithDependencies(ctx context.Context, args []string, stdout, stderr io.Writer, dependencies commandDependencies) error {
	fs := flag.NewFlagSet("adjudicate", flag.ContinueOnError)
	flagOutput := cliio.NewErrorWriter(stderr)
	fs.SetOutput(flagOutput)
	procedure := fs.String("proc", "", "Procedure: simple, quick, arbd, arb, or adc")
	proposition := fs.String("proposition", "", "Proposition presented for adjudication")
	documents := fs.String("documents", "", "Optional document root")
	settings := fs.String("settings", "", "Settings JSON file")
	caseID := fs.String("case-id", "", "Case identifier. Default: generated")
	runID := fs.String("run-id", "", "Run identifier. Default: generated")
	outDir := fs.String("out-dir", "", "Optional output directory")
	fs.Usage = func() {
		fmt.Fprintf(flagOutput, "Usage: adjudicate --proc PROCEDURE --proposition TEXT --settings FILE [options]\n\n")
		fs.PrintDefaults()
	}
	help, err := cliio.Parse(fs, args, flagOutput)
	if err != nil {
		return err
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adjudicate accepts no positional arguments")
	}
	procedureValue := service.Procedure(strings.TrimSpace(*procedure))
	if procedureValue == "" {
		return fmt.Errorf("--proc is required")
	}
	if strings.TrimSpace(*proposition) == "" {
		return fmt.Errorf("--proposition is required")
	}
	settingsValue := strings.TrimSpace(*settings)
	if settingsValue == "" {
		return fmt.Errorf("--settings is required")
	}
	if dependencies.newEngine == nil {
		return fmt.Errorf("engine constructor is required")
	}
	if dependencies.random == nil {
		return fmt.Errorf("random source is required")
	}
	caseIDValue := strings.TrimSpace(*caseID)
	if caseIDValue == "" {
		caseIDValue, err = randomID(dependencies.random, "case")
		if err != nil {
			return fmt.Errorf("generate case ID: %w", err)
		}
	}
	runIDValue := strings.TrimSpace(*runID)
	if runIDValue == "" {
		runIDValue, err = randomID(dependencies.random, "run")
		if err != nil {
			return fmt.Errorf("generate run ID: %w", err)
		}
	}
	request := service.Request{
		SchemaVersion: service.RequestSchemaVersion,
		Procedure:     procedureValue,
		CaseID:        caseIDValue,
		RunID:         runIDValue,
		Proposition:   *proposition,
		Documents:     service.DocumentInput{Root: strings.TrimSpace(*documents)},
		SettingsFile:  settingsValue,
		OutDir:        strings.TrimSpace(*outDir),
	}
	engine, err := dependencies.newEngine()
	if err != nil {
		operationErr := fmt.Errorf("create adjudication engine: %w", err)
		return errors.Join(operationErr, writeResult(stdout, commandErrorResult(request, "engine_create", operationErr)))
	}
	if engine == nil {
		operationErr := fmt.Errorf("engine constructor returned nil")
		return errors.Join(operationErr, writeResult(stdout, commandErrorResult(request, "engine_create", operationErr)))
	}
	result, runErr := engine.Run(ctx, request)
	if result.SchemaVersion == "" {
		resultErr := errors.New("adjudication engine returned no result")
		result = commandErrorResult(request, "engine_result", errors.Join(runErr, resultErr))
		runErr = errors.Join(runErr, resultErr)
	}
	if writeErr := writeResult(stdout, result); writeErr != nil {
		return errors.Join(runErr, writeErr)
	}
	return runErr
}

func commandErrorResult(request service.Request, class string, operationErr error) service.Result {
	now := time.Now().UTC()
	return service.Result{
		SchemaVersion: service.ResultSchemaVersion,
		Procedure:     request.Procedure,
		CaseID:        request.CaseID,
		RunID:         request.RunID,
		StartedAt:     now,
		FinishedAt:    &now,
		Status:        service.StatusError,
		Phase:         "error",
		ErrorClass:    class,
		Error:         operationErr.Error(),
		Management: service.Management{
			UpdatedAt: now,
			Processes: []service.ManagedProcess{{
				Name:       "adjudicate",
				Kind:       "controller",
				PID:        os.Getpid(),
				State:      service.ProcessFailed,
				StartedAt:  now,
				FinishedAt: &now,
				Error:      operationErr.Error(),
			}},
		},
	}
}

func newDefaultEngine() (adjudicationEngine, error) {
	registry, err := defaultRegistry()
	if err != nil {
		return nil, err
	}
	return service.NewEngine(registry), nil
}

func defaultRegistry() (service.Registry, error) {
	registrations := map[service.Procedure]service.Registration{
		service.ProcedureARBD: {
			Capabilities: service.Capabilities{Documents: true, ParticipantAPI: true, LeanReplay: true, Sessions: true, Council: true},
			Runner:       service.ARBDRunner{},
		},
		service.ProcedureARB: {
			Capabilities: service.Capabilities{Documents: true, ParticipantAPI: true, LeanReplay: true, Sessions: true, Council: true},
			Runner:       service.ARBRunner{},
		},
		service.ProcedureADC: {
			Capabilities: service.Capabilities{Documents: true, ParticipantAPI: true, LeanReplay: true, Sessions: true, Council: true},
			Runner:       service.ADCRunner{},
		},
		service.ProcedureSimple: {
			Capabilities: service.Capabilities{Documents: true},
			Runner:       service.SimpleRunner{},
		},
		service.ProcedureQuick: {
			Capabilities: service.Capabilities{Documents: true, ParticipantAPI: true, Sessions: true, Council: true},
			Runner:       service.QuickRunner{},
		},
	}
	return service.NewRegistry(registrations)
}

func randomID(source io.Reader, prefix string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "", fmt.Errorf("ID prefix is required")
	}
	raw := make([]byte, randomIDBytes)
	if _, err := io.ReadFull(source, raw); err != nil {
		return "", err
	}
	return prefix + "-" + hex.EncodeToString(raw), nil
}

func writeResult(w io.Writer, result service.Result) error {
	wire, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal adjudication result: %w", err)
	}
	wire = append(wire, '\n')
	written, writeErr := w.Write(wire)
	if written != len(wire) {
		writeErr = errors.Join(writeErr, io.ErrShortWrite)
	}
	if writeErr != nil {
		return fmt.Errorf("write adjudication result: %w", writeErr)
	}
	return nil
}
