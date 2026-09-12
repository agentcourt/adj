package proceeding

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/agentcourt/adj/arbd/runtime/lean"
	"github.com/agentcourt/adj/arbd/runtime/spec"
)

func Run(ctx context.Context, opts Options) (Result, error) {
	if strings.TrimSpace(opts.ComplaintPath) == "" || strings.TrimSpace(opts.OutputDir) == "" {
		return Result{}, fmt.Errorf("complaint path and output dir are required")
	}
	if opts.EngineCallTimeoutSeconds < 0 {
		return Result{}, fmt.Errorf("engine call timeout must be positive when set")
	}
	raw, err := os.ReadFile(opts.ComplaintPath)
	if err != nil {
		return Result{}, fmt.Errorf("read complaint: %w", err)
	}
	complaint, err := spec.ParseComplaintMarkdown(string(raw))
	if err != nil {
		return Result{}, err
	}
	commonRootValue := strings.TrimSpace(opts.CommonRoot)
	if commonRootValue == "" {
		commonRootValue = DefaultCommonRoot()
	}
	commonRootResolved, err := filepath.Abs(commonRootValue)
	if err != nil {
		return Result{}, fmt.Errorf("resolve common root: %w", err)
	}
	policy, err := loadCasePolicy(opts.PolicyPath)
	if err != nil {
		return Result{}, err
	}
	resolvedPromptDir, err := resolvePromptDir(opts.PromptDir)
	if err != nil {
		return Result{}, err
	}
	resolvedPromptFiles, err := resolvePromptFiles(opts.PromptFiles)
	if err != nil {
		return Result{}, err
	}
	if opts.CouncilSize > 0 {
		policy.CouncilSize = opts.CouncilSize
	}
	if strings.TrimSpace(opts.JudgmentStandard) != "" {
		policy.JudgmentStandard = strings.TrimSpace(opts.JudgmentStandard)
	}
	if err := ValidatePolicy(policy); err != nil {
		return Result{}, err
	}
	runtimeLimits := DefaultRuntimeLimits()
	if opts.CouncilTimeoutSeconds > 0 {
		runtimeLimits.CouncilLLMTimeoutSeconds = opts.CouncilTimeoutSeconds
	}
	if opts.LawyerTimeoutSeconds > 0 {
		runtimeLimits.LawyerTurnTimeoutSeconds = opts.LawyerTimeoutSeconds
	}
	if opts.EngineCallTimeoutSeconds > 0 {
		runtimeLimits.EngineCallTimeoutSeconds = opts.EngineCallTimeoutSeconds
	}
	if opts.MaxResponseBytes > 0 {
		runtimeLimits.MaxResponseBytes = opts.MaxResponseBytes
	}
	if opts.InvalidAttemptLimit > 0 {
		runtimeLimits.InvalidAttemptLimit = opts.InvalidAttemptLimit
	}
	if err := ValidateRuntimeLimits(runtimeLimits); err != nil {
		return Result{}, err
	}
	if err := ValidateCouncilBackend(opts.CouncilBackend); err != nil {
		return Result{}, err
	}
	councilPoolPath := strings.TrimSpace(opts.CouncilPoolPath)
	if councilPoolPath == "" {
		councilPoolPath = defaultCouncilPoolPath(commonRootResolved)
	}
	effectiveRunID := strings.TrimSpace(opts.RunID)
	if effectiveRunID == "" {
		effectiveRunID = fmt.Sprintf("run-%d", time.Now().UTC().UnixNano())
	}
	explicitCaseFiles, err := resolveExplicitCaseFiles(opts.CaseFiles)
	if err != nil {
		return Result{}, err
	}
	enginePath := strings.TrimSpace(opts.EnginePath)
	if enginePath == "" {
		enginePath = DefaultEnginePath()
	}
	cfg := Config{
		CaseID:                  strings.TrimSpace(opts.CaseID),
		RunID:                   effectiveRunID,
		ComplaintPath:           opts.ComplaintPath,
		CaseFilePaths:           explicitCaseFiles,
		OutputDir:               opts.OutputDir,
		CommonRoot:              commonRootResolved,
		CouncilPoolPath:         councilPoolPath,
		CouncilAllowedEndpoints: normalizeCouncilEndpoints(opts.CouncilAllowedEndpoints),
		CouncilMinEndpoints:     opts.CouncilMinEndpoints,
		PromptDir:               resolvedPromptDir,
		PromptFiles:             resolvedPromptFiles,
		CaseAPIAddr:             strings.TrimSpace(opts.CaseAPIAddr),
		Policy:                  policy,
		Runtime:                 runtimeLimits,
		CouncilBackend:          NormalizeCouncilBackend(opts.CouncilBackend),
		Engine:                  lean.New([]string{enginePath}),
	}
	if cfg.CouncilMinEndpoints < 0 || cfg.CouncilMinEndpoints > cfg.Policy.CouncilSize {
		return Result{}, fmt.Errorf("minimum distinct council endpoints must be between 0 and council size")
	}
	return runConfigured(ctx, cfg, complaint)
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

func DefaultEnginePath() string {
	exe, err := os.Executable()
	if err == nil {
		return filepath.Join(filepath.Dir(exe), "aardengine")
	}
	return filepath.FromSlash(".bin/aardengine")
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

func resolveExplicitCaseFiles(patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(patterns))
	seen := map[string]struct{}{}
	for _, pattern := range patterns {
		matches, err := expandExplicitCaseFilePattern(pattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			if err := validateExplicitCaseFilePath(match); err != nil {
				return nil, err
			}
			absMatch, err := filepath.Abs(match)
			if err != nil {
				return nil, fmt.Errorf("resolve case file %s: %w", match, err)
			}
			if _, ok := seen[absMatch]; ok {
				continue
			}
			seen[absMatch] = struct{}{}
			out = append(out, absMatch)
		}
	}
	return out, nil
}

func expandExplicitCaseFilePattern(pattern string) ([]string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("expand case file pattern %q: %w", pattern, err)
	}
	if len(matches) != 0 {
		slices.Sort(matches)
		return matches, nil
	}
	if strings.ContainsAny(pattern, "*?[") {
		return nil, fmt.Errorf("case file pattern %q matched no files", pattern)
	}
	return []string{pattern}, nil
}

func validateExplicitCaseFilePath(path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".gitignore", ".sh", ".sig":
		return fmt.Errorf("explicit case file %s uses prohibited extension %q", path, ext)
	default:
		return nil
	}
}

func loadCasePolicy(pathValue string) (Policy, error) {
	path := strings.TrimSpace(pathValue)
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return Policy{}, fmt.Errorf("resolve current working directory: %w", err)
		}
		path = filepath.Join(cwd, "etc", "policy.json")
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return DefaultPolicy(), nil
			}
			return Policy{}, fmt.Errorf("stat default policy: %w", err)
		}
	}
	policy, err := LoadPolicyFile(path)
	if err != nil {
		return Policy{}, fmt.Errorf("load policy %s: %w", path, err)
	}
	return policy, nil
}

func resolvePromptFiles(files map[string]string) (map[string]string, error) {
	if err := validatePromptFileIDs(files); err != nil {
		return nil, err
	}
	resolved := make(map[string]string, len(files))
	ids := make([]string, 0, len(files))
	for id := range files {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		path, err := resolvePromptFile(id+" prompt", files[id])
		if err != nil {
			return nil, err
		}
		resolved[id] = path
	}
	return resolved, nil
}

func resolvePromptDir(pathValue string) (string, error) {
	path := strings.TrimSpace(pathValue)
	if path == "" {
		return "", nil
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve prompt dir %s: %w", path, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat prompt dir %s: %w", resolved, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("prompt dir %s must be a directory", resolved)
	}
	return resolved, nil
}

func resolvePromptFile(label string, pathValue string) (string, error) {
	path := strings.TrimSpace(pathValue)
	if path == "" {
		return "", nil
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s %s: %w", label, path, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat %s %s: %w", label, resolved, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s %s must be a file", label, resolved)
	}
	return resolved, nil
}

func locateCommonRootFrom(start string) string {
	base := filepath.Clean(strings.TrimSpace(start))
	if base == "" {
		return filepath.FromSlash("../common")
	}
	if !filepath.IsAbs(base) {
		if absBase, err := filepath.Abs(base); err == nil {
			base = absBase
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
