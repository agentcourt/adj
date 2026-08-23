package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentcourt/adj/arb/runtime/lean"
	"github.com/agentcourt/adj/arb/runtime/proceeding"
)

func runVerifyCertificate(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	fs := newCommandFlagSet("verify-certificate", stderr)
	packetDir := fs.String("dir", "", "AAR output packet directory")
	certificatePath := fs.String("certificate", "", "Certificate JSON path. Default: DIR/certificate.json")
	statePath := fs.String("state", "", "Final state JSON path. Default: DIR/state.json")
	enginePath := fs.String("engine", proceeding.DefaultEnginePath(), "Lean engine binary")
	engineTimeoutSeconds := fs.Int("engine-timeout-seconds", proceeding.DefaultRuntimeLimits().EngineCallTimeoutSeconds, "Maximum seconds for one Lean engine call")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: aar verify-certificate --dir DIR\n\n")
		fs.PrintDefaults()
	}
	help, parseErr := parseCommandFlags(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("aar verify-certificate accepts no positional arguments")
	}
	dir := strings.TrimSpace(*packetDir)
	cert := strings.TrimSpace(*certificatePath)
	state := strings.TrimSpace(*statePath)
	if dir != "" {
		if cert == "" {
			cert = filepath.Join(dir, proceeding.ReplayCertificateFileName)
		}
		if state == "" {
			state = filepath.Join(dir, "state.json")
		}
	}
	if cert == "" || state == "" {
		return fmt.Errorf("--dir or both --certificate and --state are required")
	}
	if *engineTimeoutSeconds <= 0 {
		return fmt.Errorf("--engine-timeout-seconds must be positive")
	}
	if int64(*engineTimeoutSeconds) > proceeding.MaxRuntimeTimeoutSeconds {
		return fmt.Errorf("--engine-timeout-seconds must be at most %d", proceeding.MaxRuntimeTimeoutSeconds)
	}
	result, err := proceeding.VerifyReplayCertificate(ctx, proceeding.VerifyReplayCertificateOptions{
		CertificatePath: cert,
		StatePath:       state,
		Engine:          lean.New([]string{strings.TrimSpace(*enginePath)}),
		EngineTimeout:   time.Duration(*engineTimeoutSeconds) * time.Second,
	})
	if err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal verification result: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "%s\n", raw); err != nil {
		return fmt.Errorf("write verification result: %w", err)
	}
	return nil
}
