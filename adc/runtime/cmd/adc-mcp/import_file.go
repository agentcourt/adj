package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentcourt/adj/common/cliio"
	"github.com/agentcourt/adj/common/mcpbridge"
)

func runImportFile(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("adc-mcp import-file", flag.ContinueOnError)
	flagOutput := cliio.NewErrorWriter(stderr)
	fs.SetOutput(flagOutput)
	file := fs.String("file", "", "Read the file bytes from PATH on this machine")
	label := fs.String("label", "", "Description and source context for the file")
	endpoint := fs.String("mcp-url", os.Getenv("ADJ_MCP_URL"), "ADC MCP URL; defaults to ADJ_MCP_URL")
	tokenFile := fs.String("token-file", "", "Read the lawyer MCP capability from PATH; otherwise use ADJ_MCP_BEARER_TOKEN")
	timeout := fs.Duration("timeout", 90*time.Second, "Upload timeout")
	fs.Usage = func() {
		fmt.Fprintln(flagOutput, "Usage: adc-mcp import-file --file PATH [--label TEXT] [--mcp-url URL --token-file PATH]")
		fs.PrintDefaults()
	}
	help, err := cliio.Parse(fs, args, flagOutput)
	if err != nil || help {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*file) == "" {
		return fmt.Errorf("import-file requires --file PATH and accepts no positional arguments")
	}
	token := os.Getenv("ADJ_MCP_BEARER_TOKEN")
	if *tokenFile != "" {
		raw, err := os.ReadFile(*tokenFile)
		if err != nil {
			return fmt.Errorf("read MCP capability: %w", err)
		}
		token = string(raw)
	}
	if strings.TrimSpace(*endpoint) == "" || strings.TrimSpace(token) == "" {
		return fmt.Errorf("ADC MCP URL and lawyer capability are required")
	}
	if *timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	f, err := os.Open(*file)
	if err != nil {
		return err
	}
	raw, readErr := io.ReadAll(io.LimitReader(f, mcpbridge.MaxRequestBytes))
	if err := errors.Join(readErr, f.Close()); err != nil {
		return err
	}
	if base64.StdEncoding.EncodedLen(len(raw)) >= mcpbridge.MaxRequestBytes {
		return fmt.Errorf("file exceeds the %d-byte MCP request limit after base64 encoding", mcpbridge.MaxRequestBytes)
	}
	ctx, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	response, callErr := mcpbridge.CallHTTPTool(ctx, strings.TrimSpace(*endpoint), strings.TrimSpace(token), "submit_decision", map[string]any{
		"kind": "tool", "tool_name": "import_case_file",
		"payload": map[string]any{
			"original_name": filepath.Base(*file), "content_base64": base64.StdEncoding.EncodeToString(raw), "label": *label,
		},
	})
	if response == nil || response["ok"] != true {
		if callErr != nil {
			return callErr
		}
		return fmt.Errorf("MCP import returned no acceptance")
	}
	decision, _ := response["result"].(map[string]any)
	result, _ := decision["result"].(map[string]any)
	metadata, _ := result["file"].(map[string]any)
	if metadata == nil || metadata["file_id"] == nil {
		return errors.Join(callErr, fmt.Errorf("accepted import returned no file metadata; inspect the case before retrying"))
	}
	return errors.Join(callErr, json.NewEncoder(stdout).Encode(map[string]any{"ok": true, "file": metadata}))
}
