package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
)

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.Join(fmt.Errorf("subcommand is required"), printRootUsage(stderr))
	}
	switch args[0] {
	case "case":
		return RunCase(ctx, args[1:], stdout, stderr)
	case "case-packet":
		return RunCasePacket(args[1:], stdout, stderr)
	case "complain":
		return RunComplain(args[1:], stdout, stderr)
	case "scenario":
		return RunScenarioCase(ctx, args[1:], stdout, stderr)
	case "pacer":
		return RunPacer(args[1:], stdout, stderr)
	case "validate":
		return RunValidate(args[1:], stdout, stderr)
	case "verify-certificate":
		return RunVerifyCertificate(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		if len(args) == 1 {
			return printRootUsage(stdout)
		}
		switch args[1] {
		case "case":
			return RunCase(ctx, []string{"-h"}, stdout, stderr)
		case "case-packet":
			return RunCasePacket([]string{"-h"}, stdout, stderr)
		case "complain":
			return RunComplain([]string{"-h"}, stdout, stderr)
		case "scenario":
			return RunScenarioCase(ctx, []string{"-h"}, stdout, stderr)
		case "pacer":
			return RunPacer([]string{"-h"}, stdout, stderr)
		case "validate":
			return RunValidate([]string{"-h"}, stdout, stderr)
		case "verify-certificate":
			return RunVerifyCertificate([]string{"-h"}, stdout, stderr)
		default:
			return errors.Join(fmt.Errorf("unknown help topic %q", args[1]), printRootUsage(stderr))
		}
	default:
		return errors.Join(fmt.Errorf("unknown subcommand %q", args[0]), printRootUsage(stderr))
	}
}

func printRootUsage(w io.Writer) error {
	_, err := fmt.Fprint(w, `Usage: adc <subcommand> [options]

Subcommands:
  case       Read a complaint, plan both sides, and run the case
  case-packet  Build a deterministic complaint packet
  complain   Draft complaint.md from a situation markdown file
  scenario   Run an existing scenario JSON without starting agents
  pacer      List or fetch PACER-style documents from sqlite
  validate   Validate a scenario file for the Go runner
  verify-certificate  Verify certificate.json against state.json

Use 'adc help <subcommand>' for subcommand flags.
`)
	return err
}
