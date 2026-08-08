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

	"github.com/jsmorph/adj/adc/runtime/casegen"
	"github.com/jsmorph/adj/adc/runtime/courts"
	"github.com/jsmorph/adj/common/openai"
)

func RunComplain(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("complain", stderr, func() {
		fmt.Fprintf(stderr, "Usage: adc complain --situation <markdown> [options]\n\n")
		fs.PrintDefaults()
	})
	situationPath := fs.String("situation", "", "Path to situation markdown")
	outputPath := fs.String("out", "", "Output complaint path. Default: complaint.md beside the situation file")
	courtRef := fs.String("court", courts.DefaultCourtName, "Court profile name or JSON path")
	model := fs.String("model", casegen.DefaultPlannerModel(), "Model for complaint drafting")
	timeoutSeconds := fs.Int("timeout-seconds", defaultLLMTimeoutSeconds, "LLM HTTP timeout in seconds")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if strings.TrimSpace(*situationPath) == "" {
		return fmt.Errorf("--situation is required")
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
	complaintMarkdown, err := casegen.DraftComplaint(ctx, client, modelName, source, court, &temp)
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
