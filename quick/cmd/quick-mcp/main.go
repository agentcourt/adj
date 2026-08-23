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

	"github.com/agentcourt/adj/common/cliio"
	"github.com/agentcourt/adj/common/mcpbridge"
	"github.com/agentcourt/adj/common/mcpcli"
	quickmcp "github.com/agentcourt/adj/quick/mcp"
)

var runServer = quickmcp.Run

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
	mode, modeArgs, err := mcpcli.SplitMode("quick-mcp", args)
	if err != nil {
		return err
	}
	switch mode {
	case "serve":
		return runServe(ctx, modeArgs, stderr)
	case "keygen":
		return mcpcli.RunKeygen("quick-mcp", modeArgs, stderr)
	case "issue":
		return runIssue(modeArgs, stdout, stderr)
	default:
		return fmt.Errorf("unknown quick-mcp mode %q", mode)
	}
}

func runServe(ctx context.Context, args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("quick-mcp serve", flag.ContinueOnError)
	flagOutput := cliio.NewErrorWriter(stderr)
	fs.SetOutput(flagOutput)
	var origins mcpcli.Origins
	var promptFiles mcpbridge.PromptFiles
	listenAddr := fs.String("listen", quickmcp.DefaultListenAddr, "MCP listen address")
	caseAPIBase := fs.String("caseapi-base", "", "Base URL for the Quick case API")
	signingKeyFile := fs.String("signing-key-file", "", "Read the MCP capability signing key from PATH")
	caseAPIBearerTokenFile := fs.String("caseapi-bearer-token-file", "", "Read the private case API bearer token from PATH")
	sessionTTL := fs.Duration("session-ttl", quickmcp.DefaultSessionTTL, "Idle MCP session TTL; 0 disables expiry")
	cleanupInterval := fs.Duration("session-cleanup-interval", quickmcp.DefaultSessionCleanupInterval, "Interval for deleting expired MCP sessions")
	readyFile := fs.String("ready-file", "", "Publish the selected listen address as JSON after binding")
	promptDir := fs.String("prompt-dir", "", "Complete Quick prompt directory; MCP prompts are read below mcp/")
	fs.Var(&origins, "allow-origin", "Allowed HTTP Origin; repeat for non-local browser origins")
	fs.Var(&promptFiles, "prompt-file", "MCP prompt override as ID=PATH; repeat for a partial set")
	fs.Usage = func() {
		fmt.Fprintln(flagOutput, "Usage: quick-mcp serve --caseapi-base URL --signing-key-file PATH --caseapi-bearer-token-file PATH [options]")
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
		return fmt.Errorf("quick-mcp serve accepts no positional arguments")
	}
	signingKey, err := mcpbridge.LoadSigningKey(strings.TrimSpace(*signingKeyFile))
	if err != nil {
		return err
	}
	caseAPIBearerToken, err := readBearerTokenFile(*caseAPIBearerTokenFile)
	if err != nil {
		return err
	}
	publication, err := mcpcli.NewReadyPublication(strings.TrimSpace(*readyFile))
	if err != nil {
		return err
	}
	runErr := runServer(ctx, quickmcp.Options{
		ListenAddr:             strings.TrimSpace(*listenAddr),
		CaseAPIBase:            strings.TrimSpace(*caseAPIBase),
		SigningKey:             signingKey,
		CaseAPIBearerToken:     caseAPIBearerToken,
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

func readBearerTokenFile(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("case API bearer-token file is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat case API bearer-token file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("case API bearer-token file must be a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("case API bearer-token file permissions must exclude group and other access")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read case API bearer-token file: %w", err)
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return "", fmt.Errorf("case API bearer-token file is empty")
	}
	return token, nil
}

func runIssue(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("quick-mcp issue", flag.ContinueOnError)
	flagOutput := cliio.NewErrorWriter(stderr)
	fs.SetOutput(flagOutput)
	signingKeyFile := fs.String("signing-key-file", "", "Read the MCP capability signing key from PATH")
	caseID := fs.String("case-id", "", "Case identifier")
	roleID := fs.String("role-id", "", "Quick role: plaintiff, defendant, or observer")
	fs.Usage = func() {
		fmt.Fprintln(flagOutput, "Usage: quick-mcp issue --signing-key-file PATH --case-id ID --role-id ROLE")
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
		return fmt.Errorf("quick-mcp issue accepts no positional arguments")
	}
	assignment, err := quickmcp.NewAssignment(*caseID, *roleID)
	if err != nil {
		return err
	}
	return mcpcli.PrintCapability(stdout, strings.TrimSpace(*signingKeyFile), assignment)
}
