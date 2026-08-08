package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/jsmorph/adj/arb/runtime/lean"
	"github.com/jsmorph/adj/arb/runtime/proceeding"
)

func runVerifyCertificate(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("verify-certificate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	packetDir := fs.String("dir", "", "AAR output packet directory")
	certificatePath := fs.String("certificate", "", "Certificate JSON path. Default: DIR/certificate.json")
	statePath := fs.String("state", "", "Final state JSON path. Default: DIR/state.json")
	enginePath := fs.String("engine", proceeding.DefaultEnginePath(), "Lean engine binary")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: aar verify-certificate --dir DIR\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
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
	result, err := proceeding.VerifyReplayCertificate(proceeding.VerifyReplayCertificateOptions{
		CertificatePath: cert,
		StatePath:       state,
		Engine:          lean.New([]string{strings.TrimSpace(*enginePath)}),
	})
	if err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal verification result: %w", err)
	}
	fmt.Fprintf(stdout, "%s\n", raw)
	return nil
}
