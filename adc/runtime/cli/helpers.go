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
	return defaultADCPath(".bin", "adcengine")
}

func defaultADCPath(parts ...string) string {
	rel := filepath.Join(parts...)
	cwd, err := os.Getwd()
	if err != nil {
		return rel
	}
	for {
		for _, candidate := range []string{
			filepath.Join(cwd, rel),
			filepath.Join(cwd, "adc", rel),
		} {
			if fileExists(candidate) {
				return candidate
			}
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
		if fileExists(filepath.Join(candidate, "data", "personas", "pool.jsonl")) || fileExists(filepath.Join(candidate, "etc", "personas.csv")) {
			return candidate
		}
		if filepath.Base(base) == "common" && (fileExists(filepath.Join(base, "data", "personas", "pool.jsonl")) || fileExists(filepath.Join(base, "etc", "personas.csv"))) {
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

func resolveDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}
