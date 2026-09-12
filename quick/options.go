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
	lawyerWebSearch := true
	if opts.LawyerWebSearch != nil {
		lawyerWebSearch = *opts.LawyerWebSearch
	}
	commonRoot := strings.TrimSpace(opts.CommonRoot)
	if commonRoot == "" {
		commonRoot = DefaultCommonRoot()
	}
	commonRoot, err := filepath.Abs(commonRoot)
	if err != nil {
		return Config{}, fmt.Errorf("resolve common root: %w", err)
	}
	councilPoolPath := strings.TrimSpace(opts.CouncilPoolPath)
	if councilPoolPath == "" {
		councilPoolPath = defaultCouncilPoolPath(commonRoot)
	}
	cfg := Config{
		Proposition:             strings.TrimSpace(opts.Proposition),
		DocumentsDir:            strings.TrimSpace(opts.DocumentsDir),
		OutputDir:               strings.TrimSpace(opts.OutputDir),
		CouncilPoolPath:         councilPoolPath,
		CouncilAllowedEndpoints: normalizeCouncilEndpoints(opts.CouncilAllowedEndpoints),
		CouncilMinEndpoints:     opts.CouncilMinEndpoints,
		CouncilSize:             opts.CouncilSize,
		RequiredVotes:           opts.RequiredVotes,
		EvidenceStandard:        strings.TrimSpace(opts.EvidenceStandard),
		PromptDir:               strings.TrimSpace(opts.PromptDir),
		LawyerWebSearchEnabled:  lawyerWebSearch,
		CaseAPIAddr:             strings.TrimSpace(opts.CaseAPIAddr),
		LawyerAPIBearerToken:    strings.TrimSpace(opts.LawyerAPIBearerToken),
		CaseID:                  strings.TrimSpace(opts.CaseID),
		RunID:                   strings.TrimSpace(opts.RunID),
		LawyerTimeout:           opts.LawyerTimeout,
		CouncilTimeout:          opts.CouncilTimeout,
		MaxResponseBytes:        opts.MaxResponseBytes,
		MaxArgumentChars:        opts.MaxArgumentChars,
		InvalidAttemptLimit:     opts.InvalidAttemptLimit,
		CouncilRequestAttempts:  opts.CouncilRequestAttempts,
		ParallelCouncil:         opts.ParallelCouncil,
		AllowAPIKey:             opts.AllowAPIKey,
	}
	if cfg.Proposition == "" {
		return Config{}, fmt.Errorf("proposition is required")
	}
	if cfg.OutputDir == "" {
		return Config{}, fmt.Errorf("output directory is required")
	}
	if cfg.CouncilSize <= 0 {
		return Config{}, fmt.Errorf("council size must be positive")
	}
	if cfg.CouncilMinEndpoints < 0 || cfg.CouncilMinEndpoints > cfg.CouncilSize {
		return Config{}, fmt.Errorf("minimum distinct council endpoints must be between 0 and council size")
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
	if cfg.LawyerAPIBearerToken == "" {
		return Config{}, fmt.Errorf("lawyer API bearer token is required")
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
	promptOverrides := make(map[string]string, len(opts.PromptFiles))
	for rawID, rawPath := range opts.PromptFiles {
		id := strings.TrimSpace(rawID)
		if _, exists := promptOverrides[id]; exists {
			return Config{}, fmt.Errorf("prompt ID %q is repeated after trimming", id)
		}
		promptOverrides[id] = strings.TrimSpace(rawPath)
	}
	if cfg.PromptDir != "" {
		cfg.PromptDir, err = absolutePath(cfg.PromptDir)
		if err != nil {
			return Config{}, fmt.Errorf("resolve prompt directory: %w", err)
		}
	}
	for id, path := range promptOverrides {
		if path == "" {
			continue
		}
		promptOverrides[id], err = absolutePath(path)
		if err != nil {
			return Config{}, fmt.Errorf("resolve prompt file %q: %w", id, err)
		}
	}
	cfg.PromptFiles = promptOverrides
	return cfg, nil
}

func normalizeCouncilEndpoints(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.ToLower(strings.TrimSpace(raw))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func DefaultCommonRoot() string {
	cwd, err := os.Getwd()
	if err == nil {
		return locateCommonRootFrom(cwd)
	}
	return filepath.FromSlash("../common")
}

func defaultCouncilPoolPath(commonRoot string) string {
	if cwd, err := os.Getwd(); err == nil {
		localPool := filepath.Join(cwd, "pool.jsonl")
		if fileExists(localPool) {
			return localPool
		}
	}
	return filepath.Join(commonRoot, "data", "personas", "pool.jsonl")
}

func locateCommonRootFrom(start string) string {
	base := filepath.Clean(strings.TrimSpace(start))
	if base == "" {
		return filepath.FromSlash("../common")
	}
	if !filepath.IsAbs(base) {
		if absolute, err := filepath.Abs(base); err == nil {
			base = absolute
		}
	}
	for {
		candidate := filepath.Join(base, "common")
		if fileExists(filepath.Join(candidate, "etc", "personas.csv")) || fileExists(filepath.Join(candidate, "data", "personas", "pool.jsonl")) {
			return candidate
		}
		if filepath.Base(base) == "common" && (fileExists(filepath.Join(base, "etc", "personas.csv")) || fileExists(filepath.Join(base, "data", "personas", "pool.jsonl"))) {
			return base
		}
		next := filepath.Dir(base)
		if next == base {
			break
		}
		base = next
	}
	return filepath.Clean(filepath.Join(start, filepath.FromSlash("../common")))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
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
