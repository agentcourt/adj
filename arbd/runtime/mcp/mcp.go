package mcp

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/agentcourt/adj/common/mcpbridge"
)

const (
	DefaultListenAddr             = "127.0.0.1:19800"
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
		Procedure:   "arbd",
		PromptDir:   opts.PromptDir,
		PromptFiles: opts.PromptFiles,
	}, promptDefinitions())
	if err != nil {
		return err
	}
	client, err := mcpbridge.NewAPIClient(opts.CaseAPIBase, opts.APIBearerToken, mcpbridge.ArbitrationHTTPTimeout())
	if err != nil {
		return fmt.Errorf("configure ARBD MCP case API: %w", err)
	}
	adapter, err := mcpbridge.NewArbitrationAdapter(mcpbridge.ArbitrationAdapterOptions{
		Client:                  client,
		Prompts:                 catalog,
		CouncilSubmissionTool:   "submit_council_answer",
		CouncilSubmissionSchema: councilAnswerSchema(),
	})
	if err != nil {
		return err
	}
	return mcpbridge.Run(ctx, mcpbridge.Options{
		ListenAddr:             listenAddr,
		Audience:               "aard",
		SigningKey:             opts.SigningKey,
		SessionTTL:             opts.SessionTTL,
		DisableSessionExpiry:   opts.DisableSessionExpiry,
		SessionCleanupInterval: opts.SessionCleanupInterval,
		AllowedOrigins:         opts.AllowedOrigins,
		Log:                    opts.Log,
		ListenerReady:          opts.ListenerReady,
		ServerName:             "aard",
		ServerVersion:          "0.1.0",
		Adapter:                adapter,
	})
}

func councilAnswerSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"answer":    map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
			"rationale": map[string]any{"type": "string"},
		},
		"required":             []string{"answer", "rationale"},
		"additionalProperties": false,
	}
}
