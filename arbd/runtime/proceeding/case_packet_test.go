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
	"syscall"
	"testing"
	"time"
)

func TestWriteCasePacketAutomaticUsesProceedingCaseFiles(t *testing.T) {
	dir := t.TempDir()
	writePacketTestFile(t, filepath.Join(dir, "complaint.md"), "# Question\n\nQ\n")
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
	if summary.SchemaVersion != "aard.case-packet.v0" || summary.CaseFileMode != "auto" {
		t.Fatalf("case packet summary = %#v", summary)
	}
	packetBytes, err := os.ReadFile(packet)
	if err != nil {
		t.Fatalf("read case packet: %v", err)
	}
	if summary.PacketBytes != int64(len(packetBytes)) || summary.PacketSHA384 != sha384Bytes(packetBytes) {
		t.Fatalf("packet summary = %d, %s; want %d, %s", summary.PacketBytes, summary.PacketSHA384, len(packetBytes), sha384Bytes(packetBytes))
	}
	wantNames := []string{"case/complaint.md", "case/evidence.txt", "control/case-args.txt", "control/case-packet.json"}
	if got := packetMemberNames(t, packet); !reflect.DeepEqual(got, wantNames) {
		t.Fatalf("packet names = %#v, want %#v", got, wantNames)
	}
	staged := readPacketManifest(t, manifest)
	if staged.SchemaVersion != "aard.case-packet.v0" || staged.Complaint != "case/complaint.md" {
		t.Fatalf("manifest = %#v", staged)
	}
}

func TestWriteCasePacketAutomaticExcludesCustomComplaintIdentity(t *testing.T) {
	dir := t.TempDir()
	complaint := filepath.Join(dir, "claim.md")
	writePacketTestFile(t, complaint, "# Question\n\nQ\n")
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

func TestWriteCasePacketRejectsFIFOWithoutOpeningIt(t *testing.T) {
	dir := t.TempDir()
	complaint := filepath.Join(dir, "claim.md")
	writePacketTestFile(t, complaint, "# Question\n\nQ\n")
	if err := syscall.Mkfifo(filepath.Join(dir, "source.fifo"), 0o644); err != nil {
		t.Fatalf("create FIFO: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := WriteCasePacket(CasePacketOptions{
			ComplaintPath: complaint,
			PacketPath:    filepath.Join(dir, "case.tar.gz"),
			ManifestPath:  filepath.Join(dir, "case-packet.json"),
		})
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "not a regular file") {
			t.Fatalf("WriteCasePacket error = %v, want regular-file error", err)
		}
	case <-time.After(time.Second):
		t.Fatal("WriteCasePacket blocked while examining a FIFO")
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
			writePacketTestFile(t, complaint, "# Question\n\nQ\n")
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

func TestCombineCasePacketErrorPreservesBothErrors(t *testing.T) {
	primary := errors.New("primary failure")
	cleanup := errors.New("cleanup failure")
	secondary := &os.PathError{Op: "close", Path: "case source", Err: cleanup}
	err := combineCasePacketError(primary, secondary, "close case packet source")
	if !errors.Is(err, primary) {
		t.Fatalf("combined error = %v, want primary error", err)
	}
	if !errors.Is(err, cleanup) {
		t.Fatalf("combined error = %v, want cleanup error", err)
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr != secondary {
		t.Fatalf("combined error = %v, want cleanup path error", err)
	}
	if !strings.Contains(err.Error(), "close case packet source: close case source: cleanup failure") {
		t.Fatalf("combined error = %q, want cleanup context", err)
	}
}

func TestWriteCasePacketOutputsRejectsChangedSource(t *testing.T) {
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
		SchemaVersion: "aard.case-packet.v0",
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
	writePacketTestFile(t, source, "new-data\n")
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
