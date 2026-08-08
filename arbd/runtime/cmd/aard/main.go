package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := dispatch(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if !isReportedError(err) {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		os.Exit(1)
	}
}

func dispatch(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		printRootUsage(stderr)
		return fmt.Errorf("subcommand is required")
	}
	switch args[0] {
	case "case":
		return runCase(ctx, args[1:], stdout, stderr)
	case "case-packet":
		return runCasePacket(ctx, args[1:], stdout, stderr)
	case "complain":
		return runComplain(args[1:], stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "verify-certificate":
		return runVerifyCertificate(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		if len(args) == 1 {
			printRootUsage(stdout)
			return nil
		}
		switch args[1] {
		case "case":
			return runCase(ctx, []string{"-h"}, stdout, stderr)
		case "case-packet":
			return runCasePacket(ctx, []string{"-h"}, stdout, stderr)
		case "complain":
			return runComplain([]string{"-h"}, stdout, stderr)
		case "validate":
			return runValidate([]string{"-h"}, stdout, stderr)
		case "verify-certificate":
			return runVerifyCertificate([]string{"-h"}, stdout, stderr)
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
	fmt.Fprintln(w, "Usage: aard <subcommand> [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Subcommands:")
	fmt.Fprintln(w, "  case       Initialize an arbitration case from a complaint")
	fmt.Fprintln(w, "  case-packet  Build a deterministic case packet")
	fmt.Fprintln(w, "  complain   Draft complaint.md from a situation markdown file")
	fmt.Fprintln(w, "  validate   Validate a complaint file")
	fmt.Fprintln(w, "  verify-certificate  Verify certificate.json against state.json")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Use 'aard help <subcommand>' for subcommand flags.")
}

type reportedError struct {
	err error
}

func (e *reportedError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *reportedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func isReportedError(err error) bool {
	var reported *reportedError
	return errors.As(err, &reported)
}
