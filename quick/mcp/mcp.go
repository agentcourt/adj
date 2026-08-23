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
	DefaultListenAddr             = "127.0.0.1:19760"
	DefaultSessionTTL             = mcpbridge.DefaultSessionTTL
	DefaultSessionCleanupInterval = mcpbridge.DefaultSessionCleanupInterval

	waitToolName       = "wait_for_opportunity"
	waitToolDefault    = 30 * time.Second
	waitToolMax        = 30 * time.Second
	waitToolHTTPMargin = 2 * time.Second
)

type Options struct {
	ListenAddr             string
	CaseAPIBase            string
	SigningKey             []byte
	CaseAPIBearerToken     string
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
		Procedure:   "quick",
		PromptDir:   opts.PromptDir,
		PromptFiles: opts.PromptFiles,
	}, promptDefinitions())
	if err != nil {
		return err
	}
	caseAPIBearerToken := strings.TrimSpace(opts.CaseAPIBearerToken)
	if caseAPIBearerToken == "" {
		return fmt.Errorf("case API bearer token is required")
	}
	client, err := mcpbridge.NewAPIClient(opts.CaseAPIBase, caseAPIBearerToken, waitToolMax+waitToolHTTPMargin)
	if err != nil {
		return fmt.Errorf("configure quick MCP case API: %w", err)
	}
	return mcpbridge.Run(ctx, mcpbridge.Options{
		ListenAddr:             listenAddr,
		Audience:               "quick",
		SigningKey:             opts.SigningKey,
		SessionTTL:             opts.SessionTTL,
		DisableSessionExpiry:   opts.DisableSessionExpiry,
		SessionCleanupInterval: opts.SessionCleanupInterval,
		AllowedOrigins:         opts.AllowedOrigins,
		Log:                    opts.Log,
		ListenerReady:          opts.ListenerReady,
		ServerName:             "quick",
		ServerVersion:          "0.1.0",
		Adapter: &adapter{
			client:  client,
			prompts: catalog,
		},
	})
}
