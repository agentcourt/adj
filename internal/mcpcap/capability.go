package mcpcap

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	Version       = "adj.mcp.capability.v1"
	tokenPrefix   = "adjmcp1."
	keyFilePrefix = "adjmcpkey1."
	minimumKeyLen = 32
)

type Claims struct {
	Audience       string
	CaseID         string
	AssignmentType string
	PrincipalID    string
}

type payload struct {
	Version        string `json:"version"`
	Audience       string `json:"audience"`
	CaseID         string `json:"case_id"`
	AssignmentType string `json:"assignment_type"`
	PrincipalID    string `json:"principal_id"`
}

func GenerateKey() ([]byte, error) {
	raw := make([]byte, minimumKeyLen)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate MCP signing key: %w", err)
	}
	return raw, nil
}

func ParseKeyFile(contents []byte) ([]byte, error) {
	text := string(contents)
	if !strings.HasSuffix(text, "\n") || strings.Contains(text[:len(text)-1], "\n") {
		return nil, fmt.Errorf("MCP signing-key file must contain one newline-terminated key")
	}
	encoded := strings.TrimSuffix(text, "\n")
	if !strings.HasPrefix(encoded, keyFilePrefix) {
		return nil, fmt.Errorf("MCP signing-key file has invalid prefix")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(encoded, keyFilePrefix))
	if err != nil {
		return nil, fmt.Errorf("decode MCP signing key: %w", err)
	}
	if len(raw) < minimumKeyLen {
		return nil, fmt.Errorf("MCP signing key has %d bytes; need at least %d", len(raw), minimumKeyLen)
	}
	return append([]byte(nil), raw...), nil
}

func EncodeKeyFile(key []byte) ([]byte, error) {
	if len(key) < minimumKeyLen {
		return nil, fmt.Errorf("MCP signing key has %d bytes; need at least %d", len(key), minimumKeyLen)
	}
	encoded := base64.RawURLEncoding.EncodeToString(key)
	return []byte(keyFilePrefix + encoded + "\n"), nil
}

func Issue(key []byte, claims Claims) (string, error) {
	if len(key) < minimumKeyLen {
		return "", fmt.Errorf("MCP signing key has %d bytes; need at least %d", len(key), minimumKeyLen)
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "audience", value: claims.Audience},
		{name: "case_id", value: claims.CaseID},
		{name: "assignment_type", value: claims.AssignmentType},
		{name: "principal_id", value: claims.PrincipalID},
	} {
		trimmed := strings.TrimSpace(field.value)
		if trimmed == "" {
			return "", fmt.Errorf("MCP capability %s is empty", field.name)
		}
		if trimmed != field.value {
			return "", fmt.Errorf("MCP capability %s has surrounding whitespace", field.name)
		}
	}
	if !supportedAudience(claims.Audience) {
		return "", fmt.Errorf("unsupported MCP capability audience %q", claims.Audience)
	}
	if err := validateAssignment(claims); err != nil {
		return "", err
	}
	encodedPayload, err := json.Marshal(payload{
		Version:        Version,
		Audience:       claims.Audience,
		CaseID:         claims.CaseID,
		AssignmentType: claims.AssignmentType,
		PrincipalID:    claims.PrincipalID,
	})
	if err != nil {
		return "", fmt.Errorf("encode MCP capability payload: %w", err)
	}
	payloadSegment := base64.RawURLEncoding.EncodeToString(encodedPayload)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(Version + "\n" + payloadSegment))
	signatureSegment := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return tokenPrefix + payloadSegment + "." + signatureSegment, nil
}

func validateAssignment(claims Claims) error {
	switch claims.AssignmentType {
	case "lawyer":
		if claims.PrincipalID != "plaintiff" && claims.PrincipalID != "defendant" {
			return fmt.Errorf("unsupported MCP lawyer principal %q", claims.PrincipalID)
		}
	case "observer":
		if claims.PrincipalID != "observer" {
			return fmt.Errorf("unsupported MCP observer principal %q", claims.PrincipalID)
		}
	case "council":
		if claims.Audience != "aar" && claims.Audience != "aard" {
			return fmt.Errorf("MCP council assignment does not apply to audience %q", claims.Audience)
		}
	case "juror":
		if claims.Audience != "adc" {
			return fmt.Errorf("MCP juror assignment does not apply to audience %q", claims.Audience)
		}
	default:
		return fmt.Errorf("unsupported MCP assignment type %q", claims.AssignmentType)
	}
	return nil
}

func supportedAudience(audience string) bool {
	switch audience {
	case "quick", "aar", "aard", "adc":
		return true
	default:
		return false
	}
}
