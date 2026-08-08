package persona

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/jsmorph/adj/common/modelrequest"
)

type Spec struct {
	Model       string
	File        string
	Text        string
	FilePath    string
	RequestSpec *modelrequest.Spec
}

func ParseRecord(record string, baseDir string) (Spec, error) {
	line := strings.TrimSpace(record)
	if strings.HasPrefix(line, "{") {
		return parseJSONRecord(line, baseDir)
	}
	model, fileRef, ok := strings.Cut(line, ",")
	if !ok {
		return Spec{}, fmt.Errorf("invalid persona record: %s", line)
	}
	model = strings.TrimSpace(model)
	fileRef = strings.TrimSpace(fileRef)
	if model == "" || fileRef == "" {
		return Spec{}, fmt.Errorf("invalid persona record: %s", line)
	}
	if _, err := modelrequest.ParseModelRef(model); err != nil {
		return Spec{}, fmt.Errorf("invalid persona model %q: %w", model, err)
	}
	return loadSpecPersona(model, fileRef, baseDir, nil)
}

func parseJSONRecord(line string, baseDir string) (Spec, error) {
	requestSpec, err := modelrequest.ParseJSON([]byte(line))
	if err != nil {
		return Spec{}, fmt.Errorf("invalid persona JSON record: %w", err)
	}
	fileRef := strings.TrimSpace(requestSpec.Persona)
	if fileRef == "" {
		return Spec{}, fmt.Errorf("invalid persona JSON record: persona is required")
	}
	return loadSpecPersona(requestSpec.RuntimeModel(), fileRef, baseDir, &requestSpec)
}

func loadSpecPersona(model string, fileRef string, baseDir string, requestSpec *modelrequest.Spec) (Spec, error) {
	filePath := resolvePersonaFilePath(fileRef, baseDir)
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return Spec{}, fmt.Errorf("read persona text %s: %w", fileRef, err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return Spec{}, fmt.Errorf("empty persona text: %s", fileRef)
	}
	return Spec{
		Model:       model,
		File:        fileRef,
		Text:        text,
		FilePath:    filePath,
		RequestSpec: requestSpec,
	}, nil
}

func resolvePersonaFilePath(fileRef string, baseDir string) string {
	if filepath.IsAbs(fileRef) {
		return filepath.Clean(fileRef)
	}
	candidates := []string{
		filepath.Join(baseDir, fileRef),
		filepath.Join(baseDir, "..", "..", "etc", fileRef),
	}
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return filepath.Clean(filepath.Join(baseDir, fileRef))
}

func LoadRecordsFile(path string, baseDir string) ([]Spec, error) {
	filePath := path
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(baseDir, path)
	}
	filePath = filepath.Clean(filePath)
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read persona records %s: %w", path, err)
	}
	recordBaseDir := filepath.Dir(filePath)
	lines := strings.Split(string(raw), "\n")
	specs := make([]Spec, 0, len(lines))
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		spec, err := ParseRecord(line, recordBaseDir)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("persona records file contains no usable entries: %s", path)
	}
	return specs, nil
}

func SampleRecordFile(path string, baseDir string) (Spec, error) {
	specs, err := LoadRecordsFile(path, baseDir)
	if err != nil {
		return Spec{}, err
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(specs))))
	if err != nil {
		return Spec{}, fmt.Errorf("sample persona record: %w", err)
	}
	return specs[int(n.Int64())], nil
}

func JurorPrompt(jurorID string, personaText string) string {
	personaText = strings.TrimSpace(personaText)
	if personaText == "" {
		return ""
	}
	if strings.TrimSpace(jurorID) == "" {
		return "This juror identity is yours for this prompt. Treat it as true of yourself, including any bias, skepticism, hardship, or limits it implies:\n" + personaText
	}
	return "You are " + strings.TrimSpace(jurorID) + ". This juror identity is yours for this prompt. Treat it as true of yourself, including any bias, skepticism, hardship, or limits it implies:\n" + personaText
}

func VoteExplanationPrompt(jurorID string, personaText string) string {
	return JurorPrompt(jurorID, personaText)
}
