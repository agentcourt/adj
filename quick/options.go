package quick

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func configure(opts Options) (Config, error) {
	cfg := Config{
		Proposition:            strings.TrimSpace(opts.Proposition),
		DocumentsDir:           strings.TrimSpace(opts.DocumentsDir),
		OutputDir:              strings.TrimSpace(opts.OutputDir),
		CouncilPoolPath:        strings.TrimSpace(opts.CouncilPoolPath),
		CouncilSize:            opts.CouncilSize,
		RequiredVotes:          opts.RequiredVotes,
		EvidenceStandard:       strings.TrimSpace(opts.EvidenceStandard),
		CaseAPIAddr:            strings.TrimSpace(opts.CaseAPIAddr),
		CaseID:                 strings.TrimSpace(opts.CaseID),
		RunID:                  strings.TrimSpace(opts.RunID),
		LawyerTimeout:          opts.LawyerTimeout,
		CouncilTimeout:         opts.CouncilTimeout,
		MaxResponseBytes:       opts.MaxResponseBytes,
		MaxArgumentChars:       opts.MaxArgumentChars,
		InvalidAttemptLimit:    opts.InvalidAttemptLimit,
		CouncilRequestAttempts: opts.CouncilRequestAttempts,
		ParallelCouncil:        opts.ParallelCouncil,
		AllowAPIKey:            opts.AllowAPIKey,
	}
	if cfg.Proposition == "" {
		return Config{}, fmt.Errorf("proposition is required")
	}
	if cfg.OutputDir == "" {
		return Config{}, fmt.Errorf("output directory is required")
	}
	if cfg.CouncilPoolPath == "" {
		return Config{}, fmt.Errorf("council pool path is required")
	}
	if cfg.CouncilSize <= 0 {
		return Config{}, fmt.Errorf("council size must be positive")
	}
	if cfg.RequiredVotes <= cfg.CouncilSize/2 || cfg.RequiredVotes > cfg.CouncilSize {
		return Config{}, fmt.Errorf("required votes must be a majority between %d and %d", cfg.CouncilSize/2+1, cfg.CouncilSize)
	}
	if cfg.EvidenceStandard == "" {
		return Config{}, fmt.Errorf("evidence standard is required")
	}
	if !cfg.AllowAPIKey {
		return Config{}, fmt.Errorf("direct council provider calls require explicit API-key authorization")
	}
	if cfg.CaseAPIAddr == "" {
		cfg.CaseAPIAddr = DefaultCaseAPIAddr
	}
	if cfg.CaseID == "" {
		cfg.CaseID = "quick-1"
	}
	if cfg.RunID == "" {
		runID, err := randomID("quick")
		if err != nil {
			return Config{}, err
		}
		cfg.RunID = runID
	}
	if cfg.LawyerTimeout == 0 {
		cfg.LawyerTimeout = DefaultLawyerTimeout
	}
	if cfg.LawyerTimeout < 0 {
		return Config{}, fmt.Errorf("lawyer timeout must be positive")
	}
	if cfg.CouncilTimeout == 0 {
		cfg.CouncilTimeout = DefaultCouncilTimeout
	}
	if cfg.CouncilTimeout < 0 {
		return Config{}, fmt.Errorf("council timeout must be positive")
	}
	if cfg.MaxResponseBytes == 0 {
		cfg.MaxResponseBytes = DefaultMaxResponseBytes
	}
	if cfg.MaxResponseBytes < 1 {
		return Config{}, fmt.Errorf("max response bytes must be positive")
	}
	if cfg.MaxArgumentChars == 0 {
		cfg.MaxArgumentChars = DefaultMaxArgumentChars
	}
	if cfg.MaxArgumentChars < 1 {
		return Config{}, fmt.Errorf("max argument characters must be positive")
	}
	if cfg.InvalidAttemptLimit == 0 {
		cfg.InvalidAttemptLimit = DefaultInvalidAttempts
	}
	if cfg.InvalidAttemptLimit < 1 {
		return Config{}, fmt.Errorf("invalid attempt limit must be positive")
	}
	if cfg.CouncilRequestAttempts == 0 {
		cfg.CouncilRequestAttempts = DefaultProviderAttempts
	}
	if cfg.CouncilRequestAttempts < 1 || cfg.CouncilRequestAttempts > 4 {
		return Config{}, fmt.Errorf("council request attempts must be between 1 and 4")
	}
	if opts.MaxDocumentFiles < 1 || opts.MaxDocumentFileBytes < 1 || opts.MaxDocumentsTotal < 1 {
		return Config{}, fmt.Errorf("document file-count, per-file byte, and total byte limits must be positive")
	}
	if opts.MaxDocumentFileBytes > opts.MaxDocumentsTotal {
		return Config{}, fmt.Errorf("document file byte limit must not exceed total byte limit")
	}
	cfg.DocumentLimits.MaxFiles = opts.MaxDocumentFiles
	cfg.DocumentLimits.MaxFileBytes = opts.MaxDocumentFileBytes
	cfg.DocumentLimits.MaxTotalBytes = opts.MaxDocumentsTotal
	var err error
	cfg.OutputDir, err = absolutePath(cfg.OutputDir)
	if err != nil {
		return Config{}, err
	}
	cfg.CouncilPoolPath, err = absolutePath(cfg.CouncilPoolPath)
	if err != nil {
		return Config{}, err
	}
	if cfg.DocumentsDir != "" {
		cfg.DocumentsDir, err = absolutePath(cfg.DocumentsDir)
		if err != nil {
			return Config{}, err
		}
	}
	return cfg, nil
}

func prepareOutputDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("stat output directory %s: %w", path, err)
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("create output directory %s: %w", path, err)
		}
		return nil
	}
	if !info.IsDir() {
		return fmt.Errorf("output path %s is not a directory", path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("read output directory %s: %w", path, err)
	}
	if len(entries) != 0 {
		return fmt.Errorf("output directory %s must be empty", path)
	}
	return nil
}

func absolutePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path %s: %w", path, err)
	}
	return filepath.Clean(abs), nil
}

func randomID(prefix string) (string, error) {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate %s id: %w", prefix, err)
	}
	return prefix + "-" + hex.EncodeToString(raw[:]), nil
}
