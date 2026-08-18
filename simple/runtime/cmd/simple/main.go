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

func dispatch(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.Join(fmt.Errorf("subcommand is required"), printRootUsage(stderr))
	}
	switch args[0] {
	case "case":
		return runCase(ctx, args[1:], stdout, stderr)
	case "help", "-h", "--help":
		if len(args) == 1 {
			return printRootUsage(stdout)
		}
		if args[1] == "case" {
			return runCase(ctx, []string{"-h"}, stdout, stderr)
		}
		return errors.Join(fmt.Errorf("unknown help topic %q", args[1]), printRootUsage(stderr))
	default:
		return errors.Join(fmt.Errorf("unknown subcommand %q", args[0]), printRootUsage(stderr))
	}
}

func printRootUsage(w io.Writer) error {
	_, err := fmt.Fprint(w, `Usage: simple <subcommand> [options]

Subcommands:
  case  Decide one proposition with one model request

Use 'simple help case' for subcommand flags.
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
