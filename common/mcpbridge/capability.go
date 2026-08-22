package mcpbridge

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	CapabilityVersion = "adj.mcp.capability.v1"
	capabilityPrefix  = "adjmcp1"
	signingKeyPrefix  = "adjmcpkey1."
	minimumKeyBytes   = 32
)

type Assignment struct {
	Version        string `json:"version"`
	Audience       string `json:"audience"`
	CaseID         string `json:"case_id"`
	AssignmentType string `json:"assignment_type"`
	PrincipalID    string `json:"principal_id"`
}

func IssueCapability(key []byte, assignment Assignment) (string, error) {
	if err := validateSigningKey(key); err != nil {
		return "", err
	}
	assignment.Version = CapabilityVersion
	if err := validateAssignment(assignment); err != nil {
		return "", err
	}
	payload, err := json.Marshal(assignment)
	if err != nil {
		return "", fmt.Errorf("marshal MCP capability payload: %w", err)
	}
	payloadSegment := base64.RawURLEncoding.EncodeToString(payload)
	signature, err := capabilitySignature(key, payloadSegment)
	if err != nil {
		return "", err
	}
	return capabilityPrefix + "." + payloadSegment + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func VerifyCapability(key []byte, token, audience string) (Assignment, error) {
	if err := validateSigningKey(key); err != nil {
		return Assignment{}, err
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != capabilityPrefix || parts[1] == "" || parts[2] == "" {
		return Assignment{}, fmt.Errorf("invalid MCP capability format")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != sha256.Size || base64.RawURLEncoding.EncodeToString(signature) != parts[2] {
		return Assignment{}, fmt.Errorf("invalid MCP capability signature encoding")
	}
	expected, err := capabilitySignature(key, parts[1])
	if err != nil {
		return Assignment{}, err
	}
	if !hmac.Equal(signature, expected) {
		return Assignment{}, fmt.Errorf("invalid MCP capability signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Assignment{}, fmt.Errorf("decode MCP capability payload: %w", err)
	}
	if base64.RawURLEncoding.EncodeToString(payload) != parts[1] {
		return Assignment{}, fmt.Errorf("invalid MCP capability payload encoding")
	}
	var assignment Assignment
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&assignment); err != nil {
		return Assignment{}, fmt.Errorf("decode MCP capability payload: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return Assignment{}, fmt.Errorf("decode MCP capability payload: %w", err)
	}
	canonical, err := json.Marshal(assignment)
	if err != nil {
		return Assignment{}, fmt.Errorf("marshal verified MCP capability payload: %w", err)
	}
	if !bytes.Equal(payload, canonical) {
		return Assignment{}, fmt.Errorf("MCP capability payload is not canonical")
	}
	if err := validateAssignment(assignment); err != nil {
		return Assignment{}, err
	}
	if assignment.Audience != strings.TrimSpace(audience) {
		return Assignment{}, fmt.Errorf("MCP capability audience %q does not match %q", assignment.Audience, strings.TrimSpace(audience))
	}
	return assignment, nil
}

func LoadSigningKey(path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("signing key file is required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read signing key file %s: %w", path, err)
	}
	if !bytes.HasSuffix(raw, []byte("\n")) || bytes.Count(raw, []byte("\n")) != 1 {
		return nil, fmt.Errorf("signing key file %s has an invalid format", path)
	}
	encoded := string(raw[:len(raw)-1])
	if !strings.HasPrefix(encoded, signingKeyPrefix) {
		return nil, fmt.Errorf("signing key file %s has an invalid prefix", path)
	}
	keySegment := strings.TrimPrefix(encoded, signingKeyPrefix)
	key, err := base64.RawURLEncoding.DecodeString(keySegment)
	if err != nil {
		return nil, fmt.Errorf("decode signing key file %s: %w", path, err)
	}
	if base64.RawURLEncoding.EncodeToString(key) != keySegment {
		return nil, fmt.Errorf("signing key file %s has an invalid encoding", path)
	}
	if err := validateSigningKey(key); err != nil {
		return nil, fmt.Errorf("signing key file %s: %w", path, err)
	}
	return key, nil
}

func GenerateSigningKeyFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("signing key file is required")
	}
	key := make([]byte, minimumKeyBytes)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return fmt.Errorf("generate MCP signing key: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create signing key file %s: %w", path, err)
	}
	content := signingKeyPrefix + base64.RawURLEncoding.EncodeToString(key) + "\n"
	written, writeErr := io.WriteString(file, content)
	if writeErr == nil && written != len(content) {
		writeErr = io.ErrShortWrite
	}
	closeErr := file.Close()
	if writeErr == nil && closeErr == nil {
		return nil
	}
	removeErr := os.Remove(path)
	return errors.Join(
		wrapError(writeErr, "write signing key file %s", path),
		wrapError(closeErr, "close signing key file %s", path),
		wrapError(removeErr, "remove incomplete signing key file %s", path),
	)
}

func capabilitySignature(key []byte, payloadSegment string) ([]byte, error) {
	mac := hmac.New(sha256.New, key)
	if _, err := io.WriteString(mac, CapabilityVersion+"\n"+payloadSegment); err != nil {
		return nil, fmt.Errorf("compute MCP capability signature: %w", err)
	}
	return mac.Sum(nil), nil
}

func validateSigningKey(key []byte) error {
	if len(key) < minimumKeyBytes {
		return fmt.Errorf("MCP signing key must contain at least %d bytes", minimumKeyBytes)
	}
	return nil
}

func validateAssignment(assignment Assignment) error {
	if assignment.Version != CapabilityVersion {
		return fmt.Errorf("unsupported MCP capability version %q", assignment.Version)
	}
	if err := validateAudience(assignment.Audience); err != nil {
		return err
	}
	fields := []struct {
		name  string
		value string
	}{
		{name: "case_id", value: assignment.CaseID},
		{name: "assignment_type", value: assignment.AssignmentType},
		{name: "principal_id", value: assignment.PrincipalID},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("MCP capability %s is required", field.name)
		}
		if field.value != strings.TrimSpace(field.value) {
			return fmt.Errorf("MCP capability %s contains surrounding whitespace", field.name)
		}
	}
	return nil
}

func validateAudience(audience string) error {
	switch audience {
	case "quick", "aar", "aard", "adc":
		return nil
	default:
		return fmt.Errorf("unsupported MCP capability audience %q", audience)
	}
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("payload contains more than one JSON value")
		}
		return err
	}
	return nil
}

func wrapError(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}
