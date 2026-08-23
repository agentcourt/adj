package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	adcmcp "github.com/agentcourt/adj/adc/runtime/mcp"
	"github.com/agentcourt/adj/common/cliio"
	"github.com/agentcourt/adj/common/mcpbridge"
	"github.com/agentcourt/adj/common/mcpcli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if _, writeErr := fmt.Fprintf(os.Stderr, "error: %v\n", err); writeErr != nil {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	mode, modeArgs, err := mcpcli.SplitMode("adc-mcp", args)
	if err != nil {
		return err
	}
	switch mode {
	case "serve":
		return runServe(ctx, modeArgs, stderr)
	case "keygen":
		return mcpcli.RunKeygen("adc-mcp", modeArgs, stderr)
	case "issue":
		return runIssue(modeArgs, stdout, stderr)
	default:
		return fmt.Errorf("unknown adc-mcp mode %q", mode)
	}
}

func runServe(ctx context.Context, args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("adc-mcp serve", flag.ContinueOnError)
	flagOutput := cliio.NewErrorWriter(stderr)
	fs.SetOutput(flagOutput)
	var origins mcpcli.Origins
	var promptFiles mcpbridge.PromptFiles
	listenAddr := fs.String("listen", adcmcp.DefaultListenAddr, "MCP listen address")
	caseAPIBase := fs.String("caseapi-base", "", "Base URL for the ADC case API")
	signingKeyFile := fs.String("signing-key-file", "", "Read the MCP capability signing key from PATH")
	apiBearerToken := fs.String("api-bearer-token", "", "Optional bearer token sent to the case API")
	sessionTTL := fs.Duration("session-ttl", adcmcp.DefaultSessionTTL, "Idle MCP session TTL; 0 disables expiry")
	cleanupInterval := fs.Duration("session-cleanup-interval", adcmcp.DefaultSessionCleanupInterval, "Interval for deleting expired MCP sessions")
	readyFile := fs.String("ready-file", "", "Publish the selected listen address as JSON after binding")
	promptDir := fs.String("prompt-dir", "", "Complete ADC prompt directory; MCP prompts are read below mcp/")
	fs.Var(&origins, "allow-origin", "Allowed HTTP Origin; repeat for non-local browser origins")
	fs.Var(&promptFiles, "prompt-file", "MCP prompt override as ID=PATH; repeat for a partial set")
	fs.Usage = func() {
		fmt.Fprintln(flagOutput, "Usage: adc-mcp serve --caseapi-base URL --signing-key-file PATH [options]")
		fmt.Fprintln(flagOutput)
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
		return fmt.Errorf("adc-mcp serve accepts no positional arguments")
	}
	signingKey, err := mcpbridge.LoadSigningKey(strings.TrimSpace(*signingKeyFile))
	if err != nil {
		return err
	}
	publication, err := mcpcli.NewReadyPublication(strings.TrimSpace(*readyFile))
	if err != nil {
		return err
	}
	runErr := adcmcp.Run(ctx, adcmcp.Options{
		ListenAddr:             strings.TrimSpace(*listenAddr),
		CaseAPIBase:            strings.TrimSpace(*caseAPIBase),
		SigningKey:             signingKey,
		APIBearerToken:         strings.TrimSpace(*apiBearerToken),
		SessionTTL:             *sessionTTL,
		DisableSessionExpiry:   *sessionTTL == 0,
		SessionCleanupInterval: *cleanupInterval,
		AllowedOrigins:         origins.Values(),
		Log:                    stderr,
		ListenerReady:          publication.Publish,
		PromptDir:              strings.TrimSpace(*promptDir),
		PromptFiles:            promptFiles.Map(),
	})
	return errors.Join(runErr, publication.Remove())
}

func runIssue(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("adc-mcp issue", flag.ContinueOnError)
	flagOutput := cliio.NewErrorWriter(stderr)
	fs.SetOutput(flagOutput)
	signingKeyFile := fs.String("signing-key-file", "", "Read the MCP capability signing key from PATH")
	caseID := fs.String("case-id", "", "Case identifier")
	roleID := fs.String("role-id", "", "ADC role: plaintiff, defendant, juror, or observer")
	principalID := fs.String("principal-id", "", "Juror identifier")
	fs.Usage = func() {
		fmt.Fprintln(flagOutput, "Usage: adc-mcp issue --signing-key-file PATH --case-id ID --role-id ROLE [--principal-id ID]")
		fmt.Fprintln(flagOutput)
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
		return fmt.Errorf("adc-mcp issue accepts no positional arguments")
	}
	assignment, err := adcmcp.NewAssignment(*caseID, *roleID, *principalID)
	if err != nil {
		return err
	}
	return mcpcli.PrintCapability(stdout, strings.TrimSpace(*signingKeyFile), assignment)
}
