package main

import (
	"fmt"
	"io"
	"os"

	"github.com/agentcourt/adj/arbd/runtime/spec"
)

func runValidate(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := newCommandFlagSet("validate", stderr)
	complaintPath := fs.String("complaint", "", "Complaint markdown file")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: aard validate --complaint FILE\n\n")
		fs.PrintDefaults()
	}
	help, parseErr := parseCommandFlags(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("aard validate accepts no positional arguments")
	}
	if *complaintPath == "" {
		return fmt.Errorf("--complaint is required")
	}
	raw, err := os.ReadFile(*complaintPath)
	if err != nil {
		return fmt.Errorf("read complaint: %w", err)
	}
	if _, err := spec.ParseComplaintMarkdown(string(raw)); err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, "ok")
	return err
}
