package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jsmorph/adj/arbd/runtime/proceeding"
)

func runCasePacket(_ context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("case-packet", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var caseFiles explicitFileList
	complaintPath := fs.String("complaint", "", "Complaint markdown file")
	fs.Var(&caseFiles, "file", "Explicit case file path or glob. May be repeated. Overrides automatic complaint-directory scanning")
	packetPath := fs.String("packet", "", "Output case packet tar.gz")
	manifestPath := fs.String("manifest", "", "Output case packet manifest JSON")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: aard case-packet --complaint FILE --packet case.tar.gz --manifest case-packet.json\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if strings.TrimSpace(*complaintPath) == "" || strings.TrimSpace(*packetPath) == "" || strings.TrimSpace(*manifestPath) == "" {
		return fmt.Errorf("--complaint, --packet, and --manifest are required")
	}
	summary, err := proceeding.WriteCasePacket(proceeding.CasePacketOptions{
		ComplaintPath: *complaintPath,
		CaseFiles:     caseFiles.values,
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
