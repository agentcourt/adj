package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentcourt/adj/common/caserecord"
	"github.com/agentcourt/adj/common/cliio"
)

func runCaseRecord(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("case-record", flag.ContinueOnError)
	flagOutput := cliio.NewErrorWriter(stderr)
	fs.SetOutput(flagOutput)
	dir := fs.String("dir", "", "Case record directory")
	output := fs.String("output", "", "External JSON output file. Default: standard output")
	publishDir := fs.String("publish-dir", "", "New directory for a portable case record and selected artifacts")
	includeWorkNotes := fs.Bool("include-work-notes", false, "Include participant work notes")
	includeSessions := fs.Bool("include-sessions", false, "Include retained participant sessions and process logs")
	fs.Usage = func() {
		fmt.Fprintf(flagOutput, "Usage: adjudicate case-record --dir RUN_DIR [options]\n\n")
		fs.PrintDefaults()
	}
	help, err := cliio.Parse(fs, args, flagOutput)
	if err != nil {
		return err
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adjudicate case-record accepts no positional arguments")
	}
	dirValue := strings.TrimSpace(*dir)
	if dirValue == "" {
		return fmt.Errorf("--dir is required")
	}
	outputValue := strings.TrimSpace(*output)
	publishValue := strings.TrimSpace(*publishDir)
	if outputValue != "" && publishValue != "" {
		return fmt.Errorf("--output and --publish-dir cannot be used together")
	}
	record, err := caserecord.Build(caserecord.Options{
		Dir:              dirValue,
		IncludeWorkNotes: *includeWorkNotes,
		IncludeSessions:  *includeSessions,
	})
	if err != nil {
		return err
	}
	if publishValue != "" {
		return caserecord.Publish(record, publishValue)
	}
	if outputValue == "" {
		return caserecord.WriteJSON(stdout, record)
	}
	return writeCaseRecordFile(outputValue, record)
}

func writeCaseRecordFile(path string, record caserecord.Record) (returnErr error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve case-record output path: %w", err)
	}
	parent := filepath.Dir(path)
	parentInfo, err := os.Lstat(parent)
	if err != nil {
		return fmt.Errorf("inspect case-record output directory %s: %w", parent, err)
	}
	if parentInfo.Mode()&os.ModeSymlink != 0 || !parentInfo.IsDir() {
		return fmt.Errorf("case-record output parent %s is not a regular directory", parent)
	}
	recordRoot, err := filepath.EvalSymlinks(record.Sources[0].Path)
	if err != nil {
		return fmt.Errorf("resolve case record directory: %w", err)
	}
	outputParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return fmt.Errorf("resolve case-record output directory: %w", err)
	}
	resolvedOutput := filepath.Join(outputParent, filepath.Base(path))
	if pathInside(recordRoot, resolvedOutput) {
		return fmt.Errorf("case-record output must be outside the case record directory")
	}
	file, err := os.OpenFile(resolvedOutput, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create case-record output %s: %w", resolvedOutput, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close case-record output %s: %w", resolvedOutput, closeErr))
		}
	}()
	if err := caserecord.WriteJSON(file, record); err != nil {
		return err
	}
	return nil
}

func pathInside(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
