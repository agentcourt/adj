package casepacket

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/agentcourt/adj/adc/runtime/casegen"
)

type Options struct {
	ComplaintPath string
	PacketPath    string
	ManifestPath  string
}

type Summary struct {
	SchemaVersion string     `json:"schema_version"`
	Complaint     string     `json:"complaint"`
	Files         []FileMeta `json:"files"`
	Control       Control    `json:"control"`
	PacketSHA384  string     `json:"packet_sha384,omitempty"`
	PacketBytes   int64      `json:"packet_bytes,omitempty"`
}

type FileMeta struct {
	ArchivePath string `json:"archive_path"`
	SourceName  string `json:"source_name"`
	Role        string `json:"role"`
	SizeBytes   int64  `json:"size_bytes"`
	SHA384      string `json:"sha384"`
}

type Control struct {
	CaseArgs       string `json:"case_args"`
	CaseArgsSHA384 string `json:"case_args_sha384"`
}

type entry struct {
	archivePath string
	sourcePath  string
	role        string
}

func Write(opts Options) (Summary, error) {
	complaintPath := strings.TrimSpace(opts.ComplaintPath)
	packetPath := strings.TrimSpace(opts.PacketPath)
	manifestPath := strings.TrimSpace(opts.ManifestPath)
	if complaintPath == "" || packetPath == "" || manifestPath == "" {
		return Summary{}, fmt.Errorf("complaint, packet, and manifest paths are required")
	}
	complaint, err := casegen.LoadComplaint(complaintPath)
	if err != nil {
		return Summary{}, err
	}
	complaintInfo, err := requireRegularFile(complaint.OriginalPath, "complaint")
	if err != nil {
		return Summary{}, err
	}
	complaintName := filepath.Base(complaint.OriginalPath)
	if err := validateName(complaintName, "complaint file name"); err != nil {
		return Summary{}, err
	}
	complaintArchivePath := filepath.ToSlash(filepath.Join("case", complaintName))
	entries := []entry{{archivePath: complaintArchivePath, sourcePath: complaint.OriginalPath, role: "complaint"}}
	baseDir := filepath.Dir(complaint.OriginalPath)
	seenArchive := map[string]string{complaintArchivePath: complaint.OriginalPath}
	for _, linked := range complaint.LinkedFiles {
		rel, err := linkedArchivePath(baseDir, linked.OriginalPath)
		if err != nil {
			return Summary{}, err
		}
		archivePath := filepath.ToSlash(filepath.Join("case", rel))
		if prior := seenArchive[archivePath]; prior != "" {
			if prior == linked.OriginalPath {
				continue
			}
			return Summary{}, fmt.Errorf("case packet path conflict: %s", archivePath)
		}
		seenArchive[archivePath] = linked.OriginalPath
		entries = append(entries, entry{archivePath: archivePath, sourcePath: linked.OriginalPath, role: "linked_file"})
	}
	slices.SortFunc(entries, func(a, b entry) int {
		return strings.Compare(a.archivePath, b.archivePath)
	})
	if err := validateOutputPaths(packetPath, manifestPath, entries); err != nil {
		return Summary{}, err
	}
	control, err := controlBytes(complaintArchivePath)
	if err != nil {
		return Summary{}, err
	}
	manifest := Summary{
		SchemaVersion: "adc.case-packet.v0",
		Complaint:     complaintArchivePath,
		Control: Control{
			CaseArgs:       "control/case-args.txt",
			CaseArgsSHA384: sha384Bytes(control),
		},
	}
	for _, item := range entries {
		info := complaintInfo
		if item.sourcePath != complaint.OriginalPath {
			info, err = requireRegularFile(item.sourcePath, "linked file")
			if err != nil {
				return Summary{}, err
			}
		}
		sum, err := sha384File(item.sourcePath)
		if err != nil {
			return Summary{}, err
		}
		manifest.Files = append(manifest.Files, FileMeta{
			ArchivePath: item.archivePath,
			SourceName:  filepath.Base(item.sourcePath),
			Role:        item.role,
			SizeBytes:   info.Size(),
			SHA384:      sum,
		})
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Summary{}, fmt.Errorf("marshal case packet manifest: %w", err)
	}
	manifestBytes = append(manifestBytes, '\n')
	if err := writeOutputs(packetPath, manifestPath, entries, control, manifestBytes); err != nil {
		return Summary{}, err
	}
	packetInfo, err := os.Stat(packetPath)
	if err != nil {
		return Summary{}, fmt.Errorf("stat case packet: %w", err)
	}
	packetSHA384, err := sha384File(packetPath)
	if err != nil {
		return Summary{}, err
	}
	manifest.PacketSHA384 = packetSHA384
	manifest.PacketBytes = packetInfo.Size()
	return manifest, nil
}

func linkedArchivePath(baseDir string, sourcePath string) (string, error) {
	rel, err := filepath.Rel(baseDir, sourcePath)
	if err != nil {
		return "", fmt.Errorf("resolve linked file path %s: %w", sourcePath, err)
	}
	if filepath.IsAbs(rel) || rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return "", fmt.Errorf("linked file must be under the complaint directory: %s", sourcePath)
	}
	rel = filepath.Clean(rel)
	if err := validatePacketPath(filepath.ToSlash(rel)); err != nil {
		return "", err
	}
	return rel, nil
}

func controlBytes(complaint string) ([]byte, error) {
	if err := validatePacketPath(complaint); err != nil {
		return nil, err
	}
	return []byte(strings.Join([]string{
		"case_file_mode=auto",
		"complaint=" + complaint,
	}, "\n") + "\n"), nil
}

func validateOutputPaths(packetPath string, manifestPath string, entries []entry) error {
	packetResolved, err := canonicalWritePath(packetPath)
	if err != nil {
		return fmt.Errorf("resolve case packet output: %w", err)
	}
	manifestResolved, err := canonicalWritePath(manifestPath)
	if err != nil {
		return fmt.Errorf("resolve case packet manifest output: %w", err)
	}
	if packetResolved == manifestResolved {
		return fmt.Errorf("case packet output paths conflict: packet and manifest both resolve to %s", packetResolved)
	}
	for _, item := range entries {
		sourceResolved, err := canonicalExistingPath(item.sourcePath)
		if err != nil {
			return err
		}
		switch sourceResolved {
		case packetResolved:
			return fmt.Errorf("case packet output path conflicts with source file %s", item.sourcePath)
		case manifestResolved:
			return fmt.Errorf("case packet manifest path conflicts with source file %s", item.sourcePath)
		}
	}
	return nil
}

func canonicalExistingPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve case packet source path %s: %w", path, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve case packet source path %s: %w", path, err)
	}
	return filepath.Clean(resolved), nil
}

func canonicalWritePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve output path %s: %w", path, err)
	}
	if _, err := os.Lstat(abs); err == nil {
		resolved, err := filepath.EvalSymlinks(abs)
		if err != nil {
			return "", fmt.Errorf("resolve output path %s: %w", path, err)
		}
		return filepath.Clean(resolved), nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat output path %s: %w", path, err)
	}
	dir := filepath.Dir(abs)
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("resolve output directory %s: %w", dir, err)
	}
	return filepath.Clean(filepath.Join(resolvedDir, filepath.Base(abs))), nil
}

func writeOutputs(packetPath string, manifestPath string, entries []entry, control []byte, manifestBytes []byte) (err error) {
	manifestTemp, err := createTempPath(filepath.Dir(manifestPath), ".case-packet-manifest-*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		err = removeTemp(manifestTemp, err)
	}()
	packetTemp, err := createTempPath(filepath.Dir(packetPath), ".case-packet-*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		err = removeTemp(packetTemp, err)
	}()
	if err := os.WriteFile(manifestTemp, manifestBytes, 0o644); err != nil {
		return fmt.Errorf("write case packet manifest temp file: %w", err)
	}
	if err := writeArchive(packetTemp, entries, control, manifestBytes); err != nil {
		return err
	}
	if err := os.Rename(packetTemp, packetPath); err != nil {
		return fmt.Errorf("publish case packet: %w", err)
	}
	packetTemp = ""
	if err := os.Rename(manifestTemp, manifestPath); err != nil {
		return fmt.Errorf("publish case packet manifest: %w", err)
	}
	manifestTemp = ""
	return nil
}

func createTempPath(dir string, pattern string) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", fmt.Errorf("create case packet temp file in %s: %w", dir, err)
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return "", fmt.Errorf("close case packet temp file %s: %w; remove temp file: %v", path, err, removeErr)
		}
		return "", fmt.Errorf("close case packet temp file %s: %w", path, err)
	}
	return path, nil
}

func removeTemp(path string, retErr error) error {
	if path == "" {
		return retErr
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		if retErr != nil {
			return fmt.Errorf("%w; remove temp file %s: %v", retErr, path, err)
		}
		return fmt.Errorf("remove temp file %s: %w", path, err)
	}
	return retErr
}

func writeArchive(path string, entries []entry, control []byte, manifest []byte) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create case packet: %w", err)
	}
	fileClosed := false
	defer func() {
		if !fileClosed {
			err = combineError(err, f.Close(), "close case packet file")
		}
	}()
	gz, err := gzip.NewWriterLevel(f, gzip.BestCompression)
	if err != nil {
		return fmt.Errorf("create gzip writer: %w", err)
	}
	gz.Name = ""
	gz.ModTime = time.Unix(0, 0).UTC()
	tw := tar.NewWriter(gz)
	closeWriters := func(retErr error) error {
		retErr = combineError(retErr, tw.Close(), "close case packet tar")
		retErr = combineError(retErr, gz.Close(), "close case packet gzip")
		return retErr
	}
	for _, item := range entries {
		if err := addFile(tw, item.archivePath, item.sourcePath); err != nil {
			return closeWriters(err)
		}
	}
	if err := addBytes(tw, "control/case-args.txt", control); err != nil {
		return closeWriters(err)
	}
	if err := addBytes(tw, "control/case-packet.json", manifest); err != nil {
		return closeWriters(err)
	}
	if closeErr := tw.Close(); closeErr != nil {
		return combineError(fmt.Errorf("close case packet tar: %w", closeErr), gz.Close(), "close case packet gzip")
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("close case packet gzip: %w", err)
	}
	closeErr := f.Close()
	fileClosed = true
	if closeErr != nil {
		return fmt.Errorf("close case packet: %w", closeErr)
	}
	return nil
}

func addFile(tw *tar.Writer, archivePath string, sourcePath string) (err error) {
	if err := validatePacketPath(archivePath); err != nil {
		return err
	}
	info, err := requireRegularFile(sourcePath, "case packet source")
	if err != nil {
		return err
	}
	f, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("open case packet source %s: %w", sourcePath, err)
	}
	defer func() {
		err = combineError(err, f.Close(), "close case packet source")
	}()
	if err := writeHeader(tw, archivePath, info.Size()); err != nil {
		return err
	}
	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("write case packet member %s: %w", archivePath, err)
	}
	return nil
}

func addBytes(tw *tar.Writer, archivePath string, data []byte) error {
	if err := validatePacketPath(archivePath); err != nil {
		return err
	}
	if err := writeHeader(tw, archivePath, int64(len(data))); err != nil {
		return err
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("write case packet member %s: %w", archivePath, err)
	}
	return nil
}

func writeHeader(tw *tar.Writer, name string, size int64) error {
	header := &tar.Header{
		Name:     name,
		Mode:     0o644,
		Uid:      0,
		Gid:      0,
		Size:     size,
		ModTime:  time.Unix(0, 0).UTC(),
		Typeflag: tar.TypeReg,
		Format:   tar.FormatUSTAR,
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write case packet header %s: %w", name, err)
	}
	return nil
}

func validateName(value string, label string) error {
	if value == "" || value == "." || value == ".." || strings.Contains(value, "/") || strings.Contains(value, "\n") {
		return fmt.Errorf("invalid %s: %s", label, value)
	}
	return nil
}

func validatePacketPath(value string) error {
	if value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "\n") {
		return fmt.Errorf("invalid packet path: %s", value)
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("invalid packet path: %s", value)
		}
	}
	return nil
}

func requireRegularFile(path string, label string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s %s: %w", label, path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s %s is not a regular file", label, path)
	}
	return info, nil
}

func sha384File(path string) (sum string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer func() {
		err = combineError(err, f.Close(), "close hashed file")
	}()
	h := sha512.New384()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sha384Bytes(data []byte) string {
	sum := sha512.Sum384(data)
	return hex.EncodeToString(sum[:])
}

func combineError(primary error, secondary error, label string) error {
	if secondary == nil {
		return primary
	}
	secondary = fmt.Errorf("%s: %w", label, secondary)
	if primary == nil {
		return secondary
	}
	return fmt.Errorf("%w; %v", primary, secondary)
}
