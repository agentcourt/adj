package mcpcli

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/agentcourt/adj/common/cliio"
	"github.com/agentcourt/adj/common/mcpbridge"
)

type Origins struct {
	values []string
}

func (o *Origins) String() string {
	values := append([]string(nil), o.values...)
	sort.Strings(values)
	return strings.Join(values, ",")
}

func (o *Origins) Set(value string) error {
	origin := strings.TrimSpace(value)
	if origin == "" {
		return fmt.Errorf("origin must not be empty")
	}
	o.values = append(o.values, origin)
	return nil
}

func (o *Origins) Values() []string {
	return append([]string(nil), o.values...)
}

func SplitMode(command string, args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("usage: %s {serve|keygen|issue} [options]", command)
	}
	switch args[0] {
	case "serve", "keygen", "issue":
		return args[0], args[1:], nil
	default:
		return "", nil, fmt.Errorf("unknown %s mode %q; expected serve, keygen, or issue", command, args[0])
	}
}

func RunKeygen(command string, args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet(command+" keygen", flag.ContinueOnError)
	flagOutput := cliio.NewErrorWriter(stderr)
	fs.SetOutput(flagOutput)
	signingKeyFile := fs.String("signing-key-file", "", "Create the signing key at PATH")
	fs.Usage = func() {
		fmt.Fprintf(flagOutput, "Usage: %s keygen --signing-key-file PATH\n\n", command)
		fs.PrintDefaults()
	}
	help, err := cliio.Parse(fs, args, flagOutput)
	if err != nil {
		return err
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("%s keygen accepts no positional arguments", command)
	}
	return mcpbridge.GenerateSigningKeyFile(strings.TrimSpace(*signingKeyFile))
}

func PrintCapability(stdout io.Writer, signingKeyFile string, assignment mcpbridge.Assignment) error {
	key, err := mcpbridge.LoadSigningKey(signingKeyFile)
	if err != nil {
		return err
	}
	token, err := mcpbridge.IssueCapability(key, assignment)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(stdout, token); err != nil {
		return fmt.Errorf("write MCP capability: %w", err)
	}
	return nil
}
