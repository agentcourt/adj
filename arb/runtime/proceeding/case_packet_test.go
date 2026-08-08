package proceeding

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
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
