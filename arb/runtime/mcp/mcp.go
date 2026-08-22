package mcp

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jsmorph/adj/common/mcpbridge"
)

const (
	DefaultListenAddr             = "127.0.0.1:19780"
	DefaultSessionTTL             = mcpbridge.DefaultSessionTTL
	DefaultSessionCleanupInterval = mcpbridge.DefaultSessionCleanupInterval
)

type Options struct {
	ListenAddr             string
	CaseAPIBase            string
	SigningKey             []byte
	APIBearerToken         string
	SessionTTL             time.Duration
	DisableSessionExpiry   bool
	SessionCleanupInterval time.Duration
	AllowedOrigins         []string
	Log                    io.Writer
	ListenerReady          func(string) error
	PromptDir              string
	PromptFiles            map[string]string
}

func Run(ctx context.Context, opts Options) error {
	listenAddr := strings.TrimSpace(opts.ListenAddr)
	if listenAddr == "" {
		listenAddr = DefaultListenAddr
	}
	catalog, err := mcpbridge.LoadPromptCatalog(mcpbridge.PromptCatalogOptions{
		Procedure:   "arb",
		PromptDir:   opts.PromptDir,
		PromptFiles: opts.PromptFiles,
	}, promptDefinitions())
	if err != nil {
		return err
	}
	client, err := mcpbridge.NewAPIClient(opts.CaseAPIBase, opts.APIBearerToken, mcpbridge.ArbitrationHTTPTimeout())
	if err != nil {
		return fmt.Errorf("configure ARB MCP case API: %w", err)
	}
	adapter, err := mcpbridge.NewArbitrationAdapter(mcpbridge.ArbitrationAdapterOptions{
		Client:                  client,
		Prompts:                 catalog,
		CouncilSubmissionTool:   "submit_council_vote",
		CouncilSubmissionSchema: councilVoteSchema(),
	})
	if err != nil {
		return err
	}
	return mcpbridge.Run(ctx, mcpbridge.Options{
		ListenAddr:             listenAddr,
		Audience:               "aar",
		SigningKey:             opts.SigningKey,
		SessionTTL:             opts.SessionTTL,
		DisableSessionExpiry:   opts.DisableSessionExpiry,
		SessionCleanupInterval: opts.SessionCleanupInterval,
		AllowedOrigins:         opts.AllowedOrigins,
		Log:                    opts.Log,
		ListenerReady:          opts.ListenerReady,
		ServerName:             "aar",
		ServerVersion:          "0.1.0",
		Adapter:                adapter,
	})
}

func councilVoteSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"vote":      map[string]any{"type": "string", "enum": []string{"demonstrated", "not_demonstrated"}},
			"rationale": map[string]any{"type": "string"},
		},
		"required":             []string{"vote", "rationale"},
		"additionalProperties": false,
	}
}
