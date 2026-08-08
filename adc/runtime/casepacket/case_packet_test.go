package casepacket

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestWriteIncludesComplaintAndLinkedFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "evidence"), 0o755); err != nil {
		t.Fatalf("mkdir evidence: %v", err)
	}
	complaint := filepath.Join(dir, "complaint.md")
	if err := os.WriteFile(complaint, []byte("# Complaint\n\nSee [record](evidence/record.txt).\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "evidence", "record.txt"), []byte("record text\n"), 0o644); err != nil {
		t.Fatalf("write evidence: %v", err)
	}
	packet := filepath.Join(dir, "case.tar.gz")
	manifest := filepath.Join(dir, "case-packet.json")

	summary, err := Write(Options{
		ComplaintPath: complaint,
		PacketPath:    packet,
		ManifestPath:  manifest,
	})
	if err != nil {
		t.Fatalf("write case packet: %v", err)
	}
	if summary.SchemaVersion != "adc.case-packet.v0" {
		t.Fatalf("schema = %q", summary.SchemaVersion)
	}
	if summary.Complaint != "case/complaint.md" {
		t.Fatalf("complaint = %q", summary.Complaint)
	}
	gotNames := archiveNames(t, packet)
	wantNames := []string{
		"case/complaint.md",
		"case/evidence/record.txt",
		"control/case-args.txt",
		"control/case-packet.json",
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("archive names = %#v, want %#v", gotNames, wantNames)
	}
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var saved Summary
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if len(saved.Files) != 2 || saved.Files[0].ArchivePath != "case/complaint.md" || saved.Files[1].ArchivePath != "case/evidence/record.txt" {
		t.Fatalf("manifest files = %#v", saved.Files)
	}
}

func TestWriteRejectsLinkedFilesOutsideComplaintDir(t *testing.T) {
	dir := t.TempDir()
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "record.txt")
	if err := os.WriteFile(outside, []byte("record text\n"), 0o644); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	complaint := filepath.Join(dir, "complaint.md")
	if err := os.WriteFile(complaint, []byte("# Complaint\n\nSee [record]("+filepath.ToSlash(outside)+").\n"), 0o644); err != nil {
		t.Fatalf("write complaint: %v", err)
	}
	_, err := Write(Options{
		ComplaintPath: complaint,
		PacketPath:    filepath.Join(dir, "case.tar.gz"),
		ManifestPath:  filepath.Join(dir, "case-packet.json"),
	})
	if err == nil || !strings.Contains(err.Error(), "linked file must be under the complaint directory") {
		t.Fatalf("error = %v", err)
	}
}

func archiveNames(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("open gzip: %v", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		names = append(names, hdr.Name)
	}
	slices.Sort(names)
	return names
}
