package cliio

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

type ErrorWriter struct {
	dst io.Writer
	err error
}

func NewErrorWriter(dst io.Writer) *ErrorWriter {
	return &ErrorWriter{dst: dst}
}

func (w *ErrorWriter) Write(p []byte) (int, error) {
	n, err := w.dst.Write(p)
	if err != nil {
		w.err = errors.Join(w.err, err)
	}
	return n, err
}

func (w *ErrorWriter) Err() error {
	return w.err
}

func Parse(fs *flag.FlagSet, args []string, output *ErrorWriter) (help bool, err error) {
	err = fs.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		err = nil
		help = true
	}
	if outputErr := output.Err(); outputErr != nil {
		err = errors.Join(err, fmt.Errorf("write command output: %w", outputErr))
	}
	return help, err
}
