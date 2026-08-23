package main

import (
	"fmt"
	"io"
	"os"

	"github.com/agentcourt/adj/arbd/runtime/spec"
)

func runComplain(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := newCommandFlagSet("complain", stderr)
	situationPath := fs.String("situation", "", "Situation markdown file")
	outPath := fs.String("out", "", "Output complaint markdown file")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: aard complain --situation FILE --out FILE\n\n")
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
		return fmt.Errorf("aard complain accepts no positional arguments")
	}
	if *situationPath == "" || *outPath == "" {
		return fmt.Errorf("--situation and --out are required")
	}
	raw, err := os.ReadFile(*situationPath)
	if err != nil {
		return fmt.Errorf("read situation: %w", err)
	}
	complaint, err := spec.ParseComplaintMarkdown(string(raw))
	if err != nil {
		return fmt.Errorf("parse situation: %w", err)
	}
	if err := os.WriteFile(*outPath, []byte(spec.ComplaintMarkdown(complaint)), 0o644); err != nil {
		return fmt.Errorf("write complaint: %w", err)
	}
	_, err = fmt.Fprintln(stdout, *outPath)
	return err
}
