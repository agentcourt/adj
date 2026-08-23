package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/agentcourt/adj/common/cliio"
)

func newCommandFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(cliio.NewErrorWriter(stderr))
	return fs
}

func parseCommandFlags(fs *flag.FlagSet, args []string) (bool, error) {
	output, ok := fs.Output().(*cliio.ErrorWriter)
	if !ok {
		return false, fmt.Errorf("flag output is not an error-tracking writer")
	}
	return cliio.Parse(fs, args, output)
}
