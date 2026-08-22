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
	DefaultListenAddr             = "127.0.0.1:19880"
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
		Procedure:   "adc",
		PromptDir:   opts.PromptDir,
		PromptFiles: opts.PromptFiles,
	}, promptDefinitions())
	if err != nil {
		return err
	}
	client, err := mcpbridge.NewAPIClient(opts.CaseAPIBase, opts.APIBearerToken, waitToolMax+waitToolHTTPMargin)
	if err != nil {
		return fmt.Errorf("configure ADC MCP case API: %w", err)
	}
	return mcpbridge.Run(ctx, mcpbridge.Options{
		ListenAddr:             listenAddr,
		Audience:               "adc",
		SigningKey:             opts.SigningKey,
		SessionTTL:             opts.SessionTTL,
		DisableSessionExpiry:   opts.DisableSessionExpiry,
		SessionCleanupInterval: opts.SessionCleanupInterval,
		AllowedOrigins:         opts.AllowedOrigins,
		Log:                    opts.Log,
		ListenerReady:          opts.ListenerReady,
		ServerName:             "adc",
		ServerVersion:          "0.1.0",
		Adapter:                &adapter{client: client, prompts: catalog},
	})
}
