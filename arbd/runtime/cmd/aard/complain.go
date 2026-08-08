package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jsmorph/adj/arbd/runtime/spec"
)

func runComplain(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("complain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	situationPath := fs.String("situation", "", "Situation markdown file")
	outPath := fs.String("out", "", "Output complaint markdown file")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: aard complain --situation FILE --out FILE\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
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
