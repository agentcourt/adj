package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentcourt/adj/adc/runtime/casegen"
	"github.com/agentcourt/adj/adc/runtime/courts"
	"github.com/agentcourt/adj/common/openai"
)

func RunComplain(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("complain", stderr, func() {
		fmt.Fprintf(fs.Output(), "Usage: adc complain --situation <markdown> [options]\n\n")
		fs.PrintDefaults()
	})
	situationPath := fs.String("situation", "", "Path to situation markdown")
	outputPath := fs.String("out", "", "Output complaint path. Default: complaint.md beside the situation file")
	courtRef := fs.String("court", courts.DefaultCourtName, "Court profile name or JSON path")
	model := fs.String("model", casegen.DefaultPlannerModel(), "Model for complaint drafting")
	promptDir := fs.String("prompt-dir", "", "ADC prompt catalog directory")
	var promptFiles promptFileFlag
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM HTTP timeout in seconds")
	fs.Var(&promptFiles, "prompt-file", "ADC prompt override as ID=PATH; repeat as needed")
	help, parseErr := parseFlagSet(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc complain accepts no positional arguments")
	}
	if strings.TrimSpace(*situationPath) == "" {
		return fmt.Errorf("--situation is required")
	}
	resolvedPromptDir, resolvedPromptFiles, err := resolvePromptOptions(*promptDir, promptFiles)
	if err != nil {
		return err
	}

	source, err := casegen.LoadSourceMarkdown(*situationPath)
	if err != nil {
		return err
	}
	targetPath := strings.TrimSpace(*outputPath)
	if targetPath == "" {
		targetPath = defaultComplaintOutputPath(source.OriginalPath)
	}
	if err := ensureParentDir(targetPath); err != nil {
		return err
	}
	court, err := courts.Resolve(*courtRef)
	if err != nil {
		return err
	}

	timeout := time.Duration(*timeoutSeconds) * time.Second
	modelName := strings.TrimSpace(*model)
	client, err := openai.NewFromEnv(false, timeout)
	if err != nil {
		return err
	}
	temp := 0.2
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	complaintMarkdown, err := casegen.DraftComplaintWithOptions(ctx, client, modelName, source, court, casegen.ComplaintDraftOptions{
		Temperature: &temp,
		PromptDir:   resolvedPromptDir,
		PromptFiles: resolvedPromptFiles,
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(targetPath, []byte(complaintMarkdown), 0o644); err != nil {
		return fmt.Errorf("write complaint: %w", err)
	}
	if _, err := fmt.Fprintln(stdout, targetPath); err != nil {
		return err
	}
	return nil
}

func defaultComplaintOutputPath(sourcePath string) string {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return "complaint.md"
	}
	return filepath.Join(filepath.Dir(sourcePath), "complaint.md")
}
