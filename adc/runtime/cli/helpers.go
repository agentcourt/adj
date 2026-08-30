package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	adcprompts "github.com/agentcourt/adj/adc/runtime/prompts"
	"github.com/agentcourt/adj/adc/runtime/runner"
	"github.com/agentcourt/adj/common/cliio"
	"github.com/agentcourt/adj/common/promptfile"
)

const (
	defaultRoleAPITimeoutSeconds = runner.DefaultRoleAPITimeoutSeconds
	defaultLLMTimeoutSeconds     = runner.DefaultLLMTimeoutSeconds
)

type stringListFlag []string

func (f *stringListFlag) String() string {
	if f == nil {
		return ""
	}
	return strings.Join([]string(*f), ",")
}

type promptFileFlag struct {
	assignments promptfile.Assignments
}

func (f *promptFileFlag) String() string {
	if f == nil {
		return ""
	}
	return f.assignments.String()
}

func (f *promptFileFlag) Set(value string) error {
	id, path, ok := strings.Cut(value, "=")
	id = strings.TrimSpace(id)
	path = strings.TrimSpace(path)
	if !ok || id == "" || path == "" {
		return fmt.Errorf("prompt file must use ID=PATH")
	}
	if !adcprompts.Known(id) {
		return fmt.Errorf("unknown ADC prompt id %q", id)
	}
	return f.assignments.Set(id + "=" + path)
}

type deferredPromptFileFlag struct {
	assignments promptfile.Assignments
}

func (f *deferredPromptFileFlag) String() string {
	if f == nil {
		return ""
	}
	return f.assignments.String()
}

func (f *deferredPromptFileFlag) Set(value string) error {
	id, path, ok := strings.Cut(value, "=")
	id = strings.TrimSpace(id)
	path = strings.TrimSpace(path)
	if !ok || id == "" || path == "" {
		return fmt.Errorf("prompt file must use ID=PATH")
	}
	return f.assignments.Set(id + "=" + path)
}

func copyPromptFiles(files promptFileFlag) map[string]string {
	return files.assignments.Values()
}

func resolvePromptOptions(promptDir string, files promptFileFlag) (string, map[string]string, error) {
	promptDir = strings.TrimSpace(promptDir)
	promptFiles := copyPromptFiles(files)
	if _, err := adcprompts.Load(adcprompts.Options{PromptDir: promptDir, PromptFiles: promptFiles}); err != nil {
		return "", nil, err
	}
	return promptDir, promptFiles, nil
}

func resolveDeferredPromptOptions(promptDir string, files deferredPromptFileFlag) (string, map[string]string, error) {
	promptDir = strings.TrimSpace(promptDir)
	promptFiles := files.assignments.Values()
	if err := validatePromptFileIDs(promptFiles); err != nil {
		return "", nil, err
	}
	if _, err := adcprompts.Load(adcprompts.Options{PromptDir: promptDir, PromptFiles: promptFiles}); err != nil {
		return "", nil, err
	}
	return promptDir, promptFiles, nil
}

func newProbePromptRenderer(promptDir string, files promptFileFlag) (*runner.PromptRenderer, error) {
	return runner.NewPromptRenderer(runner.PromptRendererOptions{
		PromptDir:   strings.TrimSpace(promptDir),
		PromptFiles: copyPromptFiles(files),
	})
}

func validatePromptFileIDs(promptFiles map[string]string) error {
	for id := range promptFiles {
		if !adcprompts.Known(id) {
			return fmt.Errorf("unknown ADC prompt id %q", id)
		}
	}
	return nil
}

func (f *stringListFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func newFlagSet(name string, stderr io.Writer, usage func()) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(cliio.NewErrorWriter(stderr))
	fs.Usage = usage
	return fs
}

func parseFlagSet(fs *flag.FlagSet, args []string) (bool, error) {
	output, ok := fs.Output().(*cliio.ErrorWriter)
	if !ok {
		return false, fmt.Errorf("flag output is not an error-tracking writer")
	}
	return cliio.Parse(fs, args, output)
}

func loadPromptText(prompt string, inputFile string) (string, error) {
	if strings.TrimSpace(prompt) != "" && strings.TrimSpace(inputFile) != "" {
		return "", fmt.Errorf("--prompt and --input-file are mutually exclusive")
	}
	if strings.TrimSpace(prompt) != "" {
		return prompt, nil
	}
	if strings.TrimSpace(inputFile) == "" {
		return "", nil
	}
	raw, err := os.ReadFile(inputFile)
	if err != nil {
		return "", fmt.Errorf("read prompt file: %w", err)
	}
	return string(raw), nil
}

func writeJSONFile(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func parseOptionalFloat(raw string) (*float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var parsed float64
	if _, err := fmt.Sscanf(raw, "%f", &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseOptionalBool(raw string) (*bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func juryPolicyOverrides(jurorCount int, minimumConcurring int, unanimousRequired *bool) (map[string]any, error) {
	if jurorCount < 0 {
		return nil, fmt.Errorf("--juror-count must be non-negative")
	}
	if jurorCount > 0 && (jurorCount < 6 || jurorCount > 12) {
		return nil, fmt.Errorf("--juror-count must be between 6 and 12")
	}
	if minimumConcurring < 0 {
		return nil, fmt.Errorf("--minimum-concurring must be non-negative")
	}
	if minimumConcurring > 0 && (minimumConcurring < 6 || minimumConcurring > 12) {
		return nil, fmt.Errorf("--minimum-concurring must be between 6 and 12")
	}
	if jurorCount > 0 && minimumConcurring > jurorCount {
		return nil, fmt.Errorf("--minimum-concurring cannot exceed --juror-count")
	}
	out := map[string]any{}
	if jurorCount > 0 {
		out["jury_juror_count"] = jurorCount
	}
	if minimumConcurring > 0 {
		out["jury_minimum_concurring"] = minimumConcurring
	}
	if unanimousRequired != nil {
		if *unanimousRequired {
			out["jury_unanimous_required"] = 1
		} else {
			out["jury_unanimous_required"] = 0
		}
	}
	return out, nil
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(strings.TrimSpace(path))
	if dir == "" || dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", path, err)
	}
	return nil
}

func defaultEngineCommand() string {
	executablePath, err := os.Executable()
	if err != nil {
		executablePath = ""
	}
	return defaultEngineCommandFrom(executablePath, defaultADCPath(".bin", "adcengine"))
}

func defaultEngineCommandFrom(executablePath string, fallback string) string {
	executablePath = strings.TrimSpace(executablePath)
	if executablePath != "" {
		candidate := filepath.Join(filepath.Dir(executablePath), "adcengine")
		if fileExists(candidate) {
			return candidate
		}
	}
	return fallback
}

func defaultADCPath(parts ...string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return filepath.Join(parts...)
	}
	return defaultADCPathFrom(cwd, parts...)
}

func defaultADCPathFrom(start string, parts ...string) string {
	rel := filepath.Join(parts...)
	start = absoluteCleanPath(start)
	moduleRoot := nearestGoModuleRoot(start)
	searchLimit := start
	if moduleRoot != "" {
		searchLimit = moduleRoot
	}
	cwd := start
	for {
		for _, candidate := range []string{
			filepath.Join(cwd, rel),
			filepath.Join(cwd, "adc", rel),
		} {
			if fileExists(candidate) {
				return candidate
			}
		}
		if cwd == searchLimit {
			return rel
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			return rel
		}
		cwd = parent
	}
}

func defaultCommonRoot() string {
	cwd, err := os.Getwd()
	if err == nil {
		return locateCommonRootFrom(cwd)
	}
	return filepath.FromSlash("../common")
}

func defaultCommonPath(parts ...string) string {
	return filepath.Join(append([]string{defaultCommonRoot()}, parts...)...)
}

func defaultCommonPathFrom(baseDir string, parts ...string) string {
	return filepath.Join(append([]string{locateCommonRootFrom(baseDir)}, parts...)...)
}

func firstExistingPath(paths ...string) string {
	for _, path := range paths {
		if fileExists(path) {
			return path
		}
	}
	return ""
}

func defaultPersonaRecordsPathFor(baseDir string) string {
	return defaultCommonPathFrom(baseDir, "data", "personas", "pool.jsonl")
}

func defaultPersonaRecordsPath() string {
	cwd, err := os.Getwd()
	if err == nil {
		return defaultPersonaRecordsPathFor(cwd)
	}
	return defaultCommonPath("data", "personas", "pool.jsonl")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func locateCommonRootFrom(start string) string {
	originalStart := strings.TrimSpace(start)
	if originalStart == "" {
		return filepath.FromSlash("../common")
	}
	base := absoluteCleanPath(originalStart)
	moduleRoot := nearestGoModuleRoot(base)
	searchLimit := base
	if moduleRoot != "" {
		searchLimit = moduleRoot
	}
	for {
		candidate := filepath.Join(base, "common")
		if fileExists(filepath.Join(candidate, "data", "personas", "pool.jsonl")) || fileExists(filepath.Join(candidate, "etc", "personas.csv")) {
			return candidate
		}
		if filepath.Base(base) == "common" && (fileExists(filepath.Join(base, "data", "personas", "pool.jsonl")) || fileExists(filepath.Join(base, "etc", "personas.csv"))) {
			return base
		}
		if base == searchLimit {
			break
		}
		next := filepath.Dir(base)
		if next == base {
			break
		}
		base = next
	}
	return filepath.Clean(filepath.Join(originalStart, filepath.FromSlash("../common")))
}

func nearestGoModuleRoot(start string) string {
	base := absoluteCleanPath(start)
	for {
		if fileExists(filepath.Join(base, "go.mod")) {
			return base
		}
		next := filepath.Dir(base)
		if next == base {
			return ""
		}
		base = next
	}
}

func absoluteCleanPath(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(path) {
		if absolute, err := filepath.Abs(path); err == nil {
			return absolute
		}
	}
	return path
}

func resolveDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}
