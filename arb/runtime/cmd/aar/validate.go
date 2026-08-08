package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jsmorph/adj/arb/runtime/spec"
)

func runValidate(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	complaintPath := fs.String("complaint", "", "Complaint markdown file")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: aar validate --complaint FILE\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
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
