package cliio

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
)

type Assignments struct {
	values map[string]string
}

type StringList []string

func (values *StringList) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("value must not be empty")
	}
	*values = append(*values, value)
	return nil
}

func (values *StringList) String() string {
	return strings.Join(*values, ",")
}

func (values *StringList) Values() []string {
	return append([]string(nil), *values...)
}

func (a *Assignments) Set(value string) error {
	key, path, ok := strings.Cut(value, "=")
	key = strings.TrimSpace(key)
	path = strings.TrimSpace(path)
	if !ok || key == "" || path == "" {
		return fmt.Errorf("assignment must have the form ID=PATH")
	}
	if a.values == nil {
		a.values = make(map[string]string)
	}
	if _, exists := a.values[key]; exists {
		return fmt.Errorf("assignment ID %q was repeated", key)
	}
	a.values[key] = path
	return nil
}

func (a *Assignments) String() string {
	values := make([]string, 0, len(a.values))
	for key, path := range a.values {
		values = append(values, key+"="+path)
	}
	sort.Strings(values)
	return strings.Join(values, ",")
}

func (a *Assignments) Map() map[string]string {
	result := make(map[string]string, len(a.values))
	for key, path := range a.values {
		result[key] = path
	}
	return result
}

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
