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
		if _, writeErr := fmt.Fprintf(os.Stderr, "error: %v\n", err); writeErr != nil {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func dispatch(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.Join(fmt.Errorf("subcommand is required"), printUsage(stderr))
	}
	switch args[0] {
	case "case":
		return runCase(ctx, args[1:], stdout, stderr)
	case "help", "-h", "--help":
		if len(args) == 1 {
			return printUsage(stdout)
		}
		if args[1] == "case" {
			return runCase(ctx, []string{"-h"}, stdout, stderr)
		}
		return errors.Join(fmt.Errorf("unknown help topic %q", args[1]), printUsage(stderr))
	default:
		return errors.Join(fmt.Errorf("unknown subcommand %q", args[0]), printUsage(stderr))
	}
}

func printUsage(w io.Writer) error {
	usage := `Usage: quick <subcommand> [options]

Subcommands:
  case  Run one quick adjudication case

Use 'quick help case' for case flags.
`
	_, err := exactWriter{Writer: w}.Write([]byte(usage))
	return err
}
