package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/agentcourt/adj/adc/runtime/lean"
	"github.com/agentcourt/adj/adc/runtime/runner"
)

func RunVerifyCertificate(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("verify-certificate", stderr, func() {
		fmt.Fprintf(fs.Output(), "Usage: adc verify-certificate --dir DIR\n\n")
		fs.PrintDefaults()
	})
	packetDir := fs.String("dir", "", "ADC output packet directory")
	certificatePath := fs.String("certificate", "", "Certificate JSON path. Default: DIR/certificate.json")
	statePath := fs.String("state", "", "Final state JSON path. Default: DIR/state.json")
	engineCommand := fs.String("engine", defaultEngineCommand(), "Engine command string")
	help, parseErr := parseFlagSet(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc verify-certificate accepts no positional arguments")
	}
	dir := strings.TrimSpace(*packetDir)
	cert := strings.TrimSpace(*certificatePath)
	state := strings.TrimSpace(*statePath)
	if dir != "" {
		if cert == "" {
			cert = filepath.Join(dir, runner.ReplayCertificateFileName)
		}
		if state == "" {
			state = filepath.Join(dir, "state.json")
		}
	}
	if cert == "" || state == "" {
		return fmt.Errorf("--dir or both --certificate and --state are required")
	}
	result, err := runner.VerifyReplayCertificate(runner.VerifyReplayCertificateOptions{
		CertificatePath: cert,
		StatePath:       state,
		Engine:          lean.New(strings.Fields(strings.TrimSpace(*engineCommand))),
	})
	if err != nil {
		return err
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal verification result: %w", err)
	}
	_, err = fmt.Fprintf(stdout, "%s\n", raw)
	return err
}
