package cli

import (
	"fmt"
	"io"
)

func Run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		printRootUsage(stderr)
		return fmt.Errorf("subcommand is required")
	}
	switch args[0] {
	case "case":
		return RunCase(args[1:], stdout, stderr)
	case "case-packet":
		return RunCasePacket(args[1:], stdout, stderr)
	case "complain":
		return RunComplain(args[1:], stdout, stderr)
	case "scenario":
		return RunScenarioCase(args[1:], stdout, stderr)
	case "pacer":
		return RunPacer(args[1:], stdout, stderr)
	case "validate":
		return RunValidate(args[1:], stdout, stderr)
	case "verify-certificate":
		return RunVerifyCertificate(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		if len(args) == 1 {
			printRootUsage(stdout)
			return nil
		}
		switch args[1] {
		case "case":
			return RunCase([]string{"-h"}, stdout, stderr)
		case "case-packet":
			return RunCasePacket([]string{"-h"}, stdout, stderr)
		case "complain":
			return RunComplain([]string{"-h"}, stdout, stderr)
		case "scenario":
			return RunScenarioCase([]string{"-h"}, stdout, stderr)
		case "pacer":
			return RunPacer([]string{"-h"}, stdout, stderr)
		case "validate":
			return RunValidate([]string{"-h"}, stdout, stderr)
		case "verify-certificate":
			return RunVerifyCertificate([]string{"-h"}, stdout, stderr)
		default:
			printRootUsage(stderr)
			return fmt.Errorf("unknown help topic %q", args[1])
		}
	default:
		printRootUsage(stderr)
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func printRootUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: adc <subcommand> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintln(w, "  case       Read a complaint, plan both sides, and run the case")
	fmt.Fprintln(w, "  case-packet  Build a deterministic complaint packet")
	fmt.Fprintln(w, "  complain   Draft complaint.md from a situation markdown file")
	fmt.Fprintln(w, "  scenario   Run an existing scenario JSON without starting agents")
	fmt.Fprintln(w, "  pacer      List or fetch PACER-style documents from sqlite")
	fmt.Fprintln(w, "  validate   Validate a scenario file for the Go runner")
	fmt.Fprintln(w, "  verify-certificate  Verify certificate.json against state.json")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Use 'adc help <subcommand>' for subcommand flags.")
}
