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
			if _, writeErr := fmt.Fprintf(os.Stderr, "error: %v\n", err); writeErr != nil {
				os.Exit(2)
			}
		}
		os.Exit(1)
	}
}

func dispatch(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.Join(fmt.Errorf("subcommand is required"), printRootUsage(stderr))
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
		return runVerifyCertificate(ctx, args[1:], stdout, stderr)
	case "help", "-h", "--help":
		if len(args) == 1 {
			return printRootUsage(stdout)
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
			return runVerifyCertificate(ctx, []string{"-h"}, stdout, stderr)
		default:
			return errors.Join(fmt.Errorf("unknown help topic %q", args[1]), printRootUsage(stderr))
		}
	default:
		return errors.Join(fmt.Errorf("unknown subcommand %q", args[0]), printRootUsage(stderr))
	}
}

func printRootUsage(w io.Writer) error {
	_, err := fmt.Fprint(w, `Usage: aar <subcommand> [options]

Subcommands:
  case       Initialize an arbitration case from a complaint
  case-packet  Build a deterministic case packet
  complain   Draft complaint.md from a situation markdown file
  validate   Validate a complaint file
  verify-certificate  Verify certificate.json against state.json

Use 'aar help <subcommand>' for subcommand flags.
`)
	return err
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
