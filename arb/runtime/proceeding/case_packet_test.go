package proceeding

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWriteCasePacketAutomaticUsesProceedingCaseFiles(t *testing.T) {
	dir := t.TempDir()
	writePacketTestFile(t, filepath.Join(dir, "complaint.md"), "# Proposition\n\nP\n")
	writePacketTestFile(t, filepath.Join(dir, "evidence.txt"), "evidence\n")
	writePacketTestFile(t, filepath.Join(dir, "situation.md"), "skip\n")
	writePacketTestFile(t, filepath.Join(dir, "README.md"), "skip\n")
	packet := filepath.Join(dir, "case.tar.gz")
	manifest := filepath.Join(dir, "case-packet.json")

	summary, err := WriteCasePacket(CasePacketOptions{
		ComplaintPath: filepath.Join(dir, "complaint.md"),
		PacketPath:    packet,
		ManifestPath:  manifest,
	})
	if err != nil {
		t.Fatalf("WriteCasePacket returned error: %v", err)
	}
	if summary.CaseFileMode != "auto" {
		t.Fatalf("CaseFileMode = %q, want auto", summary.CaseFileMode)
	}
	if summary.PacketSHA384 == "" || summary.PacketBytes == 0 {
		t.Fatalf("packet summary missing hash or size: %#v", summary)
	}
	packetBytes, err := os.ReadFile(packet)
	if err != nil {
		t.Fatalf("read case packet: %v", err)
	}
	if summary.PacketBytes != int64(len(packetBytes)) || summary.PacketSHA384 != sha384Bytes(packetBytes) {
		t.Fatalf("packet summary = %d, %s; want %d, %s", summary.PacketBytes, summary.PacketSHA384, len(packetBytes), sha384Bytes(packetBytes))
	}
	gotNames := packetMemberNames(t, packet)
	wantNames := []string{"case/complaint.md", "case/evidence.txt", "control/case-args.txt", "control/case-packet.json"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("packet names = %#v, want %#v", gotNames, wantNames)
	}
	staged := readPacketManifest(t, manifest)
	if staged.CaseFileMode != "auto" || staged.Complaint != "case/complaint.md" {
		t.Fatalf("manifest = %#v", staged)
	}
	if len(staged.CaseFiles) != 0 {
		t.Fatalf("manifest CaseFiles = %#v, want empty", staged.CaseFiles)
	}
}

func TestWriteCasePacketAutomaticExcludesCustomComplaintIdentity(t *testing.T) {
	dir := t.TempDir()
	complaint := filepath.Join(dir, "claim.md")
	writePacketTestFile(t, complaint, "# Proposition\n\nP\n")
	if err := os.Link(complaint, filepath.Join(dir, "claim-copy.md")); err != nil {
		t.Fatalf("link complaint alias: %v", err)
	}
	writePacketTestFile(t, filepath.Join(dir, "evidence.txt"), "evidence\n")
	packet := filepath.Join(dir, "case.tar.gz")
	manifest := filepath.Join(dir, "case-packet.json")

	_, err := WriteCasePacket(CasePacketOptions{
		ComplaintPath: complaint,
		PacketPath:    packet,
		ManifestPath:  manifest,
	})
	if err != nil {
		t.Fatalf("WriteCasePacket returned error: %v", err)
	}
	wantNames := []string{"case/claim.md", "case/evidence.txt", "control/case-args.txt", "control/case-packet.json"}
	if got := packetMemberNames(t, packet); !reflect.DeepEqual(got, wantNames) {
		t.Fatalf("packet names = %#v, want %#v", got, wantNames)
	}
}

func TestWriteCasePacketExplicitUsesResolvedFiles(t *testing.T) {
	dir := t.TempDir()
	writePacketTestFile(t, filepath.Join(dir, "complaint.md"), "# Proposition\n\nP\n")
	writePacketTestFile(t, filepath.Join(dir, "b.txt"), "b\n")
	writePacketTestFile(t, filepath.Join(dir, "a.txt"), "a\n")
	packet := filepath.Join(dir, "case.tar.gz")
	manifest := filepath.Join(dir, "case-packet.json")

	summary, err := WriteCasePacket(CasePacketOptions{
		ComplaintPath: filepath.Join(dir, "complaint.md"),
		CaseFiles:     []string{filepath.Join(dir, "*.txt")},
		PacketPath:    packet,
		ManifestPath:  manifest,
	})
	if err != nil {
		t.Fatalf("WriteCasePacket returned error: %v", err)
	}
	if summary.CaseFileMode != "explicit" {
		t.Fatalf("CaseFileMode = %q, want explicit", summary.CaseFileMode)
	}
	wantCaseFiles := []string{"case-files/000001/a.txt", "case-files/000002/b.txt"}
	if !reflect.DeepEqual(summary.CaseFiles, wantCaseFiles) {
		t.Fatalf("CaseFiles = %#v, want %#v", summary.CaseFiles, wantCaseFiles)
	}
	gotNames := packetMemberNames(t, packet)
	wantNames := []string{"case-files/000001/a.txt", "case-files/000002/b.txt", "case/complaint.md", "control/case-args.txt", "control/case-packet.json"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("packet names = %#v, want %#v", gotNames, wantNames)
	}
}

func TestWriteCasePacketRejectsDuplicateExplicitBaseNames(t *testing.T) {
	dir := t.TempDir()
	left := filepath.Join(dir, "left")
	right := filepath.Join(dir, "right")
	if err := os.Mkdir(left, 0o755); err != nil {
		t.Fatalf("mkdir left: %v", err)
	}
	if err := os.Mkdir(right, 0o755); err != nil {
		t.Fatalf("mkdir right: %v", err)
	}
	writePacketTestFile(t, filepath.Join(dir, "complaint.md"), "# Proposition\n\nP\n")
	writePacketTestFile(t, filepath.Join(left, "evidence.txt"), "left\n")
	writePacketTestFile(t, filepath.Join(right, "evidence.txt"), "right\n")

	_, err := WriteCasePacket(CasePacketOptions{
		ComplaintPath: filepath.Join(dir, "complaint.md"),
		CaseFiles: []string{
			filepath.Join(left, "evidence.txt"),
			filepath.Join(right, "evidence.txt"),
		},
		PacketPath:   filepath.Join(dir, "case.tar.gz"),
		ManifestPath: filepath.Join(dir, "case-packet.json"),
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate case file name") {
		t.Fatalf("WriteCasePacket error = %v, want duplicate name error", err)
	}
}

func TestWriteCasePacketRejectsOutputPathOverSource(t *testing.T) {
	dir := t.TempDir()
	complaint := filepath.Join(dir, "complaint.md")
	original := "# Proposition\n\nP\n"
	writePacketTestFile(t, complaint, original)

	_, err := WriteCasePacket(CasePacketOptions{
		ComplaintPath: complaint,
		PacketPath:    complaint,
		ManifestPath:  filepath.Join(dir, "case-packet.json"),
	})
	if err == nil || !strings.Contains(err.Error(), "conflicts with source file") {
		t.Fatalf("WriteCasePacket error = %v, want source conflict error", err)
	}
	raw, readErr := os.ReadFile(complaint)
	if readErr != nil {
		t.Fatalf("read complaint after failed packet: %v", readErr)
	}
	if string(raw) != original {
		t.Fatalf("complaint changed after failed packet: %q", string(raw))
	}
}

func TestWriteCasePacketRejectsOutputPathCollision(t *testing.T) {
	dir := t.TempDir()
	complaint := filepath.Join(dir, "complaint.md")
	writePacketTestFile(t, complaint, "# Proposition\n\nP\n")
	output := filepath.Join(dir, "case-output")

	_, err := WriteCasePacket(CasePacketOptions{
		ComplaintPath: complaint,
		PacketPath:    output,
		ManifestPath:  output,
	})
	if err == nil || !strings.Contains(err.Error(), "packet and manifest") {
		t.Fatalf("WriteCasePacket error = %v, want output collision error", err)
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatalf("output exists after failed packet, stat error = %v", statErr)
	}
}

func TestWriteCasePacketDoesNotPublishManifestWhenPacketCannotBeCreated(t *testing.T) {
	dir := t.TempDir()
	complaint := filepath.Join(dir, "complaint.md")
	writePacketTestFile(t, complaint, "# Proposition\n\nP\n")
	manifest := filepath.Join(dir, "case-packet.json")

	_, err := WriteCasePacket(CasePacketOptions{
		ComplaintPath: complaint,
		PacketPath:    filepath.Join(dir, "missing", "case.tar.gz"),
		ManifestPath:  manifest,
	})
	if err == nil {
		t.Fatalf("WriteCasePacket returned nil error for missing packet directory")
	}
	if _, statErr := os.Stat(manifest); !os.IsNotExist(statErr) {
		t.Fatalf("manifest exists after failed packet, stat error = %v", statErr)
	}
}

func TestWriteCasePacketRejectsPreexistingOutput(t *testing.T) {
	for _, test := range []struct {
		name     string
		existing string
	}{
		{name: "packet", existing: "packet"},
		{name: "manifest", existing: "manifest"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			complaint := filepath.Join(dir, "complaint.md")
			evidence := filepath.Join(dir, "evidence.txt")
			packet := filepath.Join(dir, "case.tar.gz")
			manifest := filepath.Join(dir, "case-packet.json")
			writePacketTestFile(t, complaint, "# Proposition\n\nP\n")
			writePacketTestFile(t, evidence, "evidence\n")
			existingPath := packet
			otherPath := manifest
			if test.existing == "manifest" {
				existingPath = manifest
				otherPath = packet
			}
			const original = "existing output\n"
			writePacketTestFile(t, existingPath, original)

			_, err := WriteCasePacket(CasePacketOptions{
				ComplaintPath: complaint,
				CaseFiles:     []string{evidence},
				PacketPath:    packet,
				ManifestPath:  manifest,
			})
			if err == nil || !errors.Is(err, os.ErrExist) {
				t.Fatalf("WriteCasePacket error = %v, want existing-output error", err)
			}
			raw, readErr := os.ReadFile(existingPath)
			if readErr != nil {
				t.Fatalf("read preexisting output: %v", readErr)
			}
			if string(raw) != original {
				t.Fatalf("preexisting output = %q, want %q", string(raw), original)
			}
			if _, statErr := os.Stat(otherPath); !os.IsNotExist(statErr) {
				t.Fatalf("other output exists after failed publication, stat error = %v", statErr)
			}
		})
	}
}

func TestPublishCasePacketOutputsRemovesPacketAfterManifestFailure(t *testing.T) {
	dir := t.TempDir()
	packetTemp := filepath.Join(dir, "packet.tmp")
	writePacketTestFile(t, packetTemp, "packet\n")
	manifestTemp := filepath.Join(dir, "missing-manifest.tmp")
	packet := filepath.Join(dir, "case.tar.gz")
	manifest := filepath.Join(dir, "case-packet.json")

	err := publishCasePacketOutputs(packetTemp, packet, manifestTemp, manifest)
	if err == nil || !strings.Contains(err.Error(), "publish case packet manifest") {
		t.Fatalf("publishCasePacketOutputs error = %v, want manifest publication error", err)
	}
	for _, path := range []string{packet, manifest} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("output %s exists after manifest publication error, stat error = %v", path, statErr)
		}
	}
}

func TestWriteCasePacketOutputsRejectsChangedSource(t *testing.T) {
	for _, test := range []struct {
		name        string
		replacement string
	}{
		{name: "same size", replacement: "new-data\n"},
		{name: "smaller", replacement: "new\n"},
		{name: "larger", replacement: "new-data-with-more-bytes\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "complaint.md")
			writePacketTestFile(t, source, "old-data\n")
			entry := casePacketEntry{
				archivePath: "case/complaint.md",
				sourcePath:  source,
				role:        "complaint",
			}
			meta, err := casePacketFileMetadata(entry)
			if err != nil {
				t.Fatalf("construct source metadata: %v", err)
			}
			control := []byte("case_file_mode=auto\ncomplaint=case/complaint.md\n")
			manifest := CasePacketSummary{
				SchemaVersion: "aar.case-packet.v0",
				CaseFileMode:  "auto",
				Complaint:     entry.archivePath,
				Files:         []CasePacketFileMeta{meta},
				Control: CasePacketControl{
					CaseArgs:       "control/case-args.txt",
					CaseArgsSHA384: sha384Bytes(control),
				},
			}
			manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
			if err != nil {
				t.Fatalf("marshal stale manifest: %v", err)
			}
			manifestBytes = append(manifestBytes, '\n')
			writePacketTestFile(t, source, test.replacement)
			packetPath := filepath.Join(dir, "case.tar.gz")
			manifestPath := filepath.Join(dir, "case-packet.json")

			_, _, err = writeCasePacketOutputs(
				packetPath,
				manifestPath,
				[]casePacketEntry{entry},
				[]CasePacketFileMeta{meta},
				control,
				manifestBytes,
			)
			if err == nil || !strings.Contains(err.Error(), "changed after manifest construction") {
				t.Fatalf("writeCasePacketOutputs error = %v, want source-change error", err)
			}
			for _, path := range []string{packetPath, manifestPath} {
				if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
					t.Fatalf("output %s exists after source-change error, stat error = %v", path, statErr)
				}
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("read output directory: %v", err)
			}
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".case-packet-") {
					t.Fatalf("temporary output remains after source-change error: %s", entry.Name())
				}
			}
		})
	}
}

func writePacketTestFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func packetMemberNames(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open packet: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("open gzip: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var out []string
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("read tar: %v", err)
		}
		out = append(out, header.Name)
	}
	return out
}

func readPacketManifest(t *testing.T, path string) CasePacketSummary {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var out CasePacketSummary
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	return out
}
