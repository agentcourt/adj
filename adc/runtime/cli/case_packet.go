package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/agentcourt/adj/adc/runtime/casepacket"
)

func RunCasePacket(args []string, stdout io.Writer, stderr io.Writer) error {
	var fs *flag.FlagSet
	fs = newFlagSet("case-packet", stderr, func() {
		fmt.Fprintf(fs.Output(), "Usage: adc case-packet --complaint FILE --packet case.tar.gz --manifest case-packet.json\n\n")
		fs.PrintDefaults()
	})
	complaintPath := fs.String("complaint", "", "Complaint markdown file")
	packetPath := fs.String("packet", "", "Output case packet tar.gz")
	manifestPath := fs.String("manifest", "", "Output case packet manifest JSON")
	help, parseErr := parseFlagSet(fs, args)
	if parseErr != nil {
		return parseErr
	}
	if help {
		return nil
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("adc case-packet accepts no positional arguments")
	}
	if strings.TrimSpace(*complaintPath) == "" || strings.TrimSpace(*packetPath) == "" || strings.TrimSpace(*manifestPath) == "" {
		return fmt.Errorf("--complaint, --packet, and --manifest are required")
	}
	summary, err := casepacket.Write(casepacket.Options{
		ComplaintPath: *complaintPath,
		PacketPath:    *packetPath,
		ManifestPath:  *manifestPath,
	})
	if err != nil {
		return err
	}
	raw, err := json.Marshal(summary)
	if err != nil {
		return fmt.Errorf("marshal case packet summary: %w", err)
	}
	if _, err := fmt.Fprintln(stdout, string(raw)); err != nil {
		return fmt.Errorf("write case packet summary: %w", err)
	}
	return nil
}
