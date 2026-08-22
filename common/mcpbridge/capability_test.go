package mcpbridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	GoldenCapabilityKeyFile = "adjmcpkey1.AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8\n"
	GoldenCapabilityToken   = "adjmcp1.eyJ2ZXJzaW9uIjoiYWRqLm1jcC5jYXBhYmlsaXR5LnYxIiwiYXVkaWVuY2UiOiJxdWljayIsImNhc2VfaWQiOiJjYXNlLWFscGhhIiwiYXNzaWdubWVudF90eXBlIjoibGF3eWVyIiwicHJpbmNpcGFsX2lkIjoicGxhaW50aWZmIn0.9W98_OD1jQN6xF0fcfUKmTWOgr2QFLYa2tQyxkalUCg"
)

var GoldenCapabilityAssignment = Assignment{
	Version:        CapabilityVersion,
	Audience:       "quick",
	CaseID:         "case-alpha",
	AssignmentType: "lawyer",
	PrincipalID:    "plaintiff",
}

func TestCapabilityGoldenVector(t *testing.T) {
	key := goldenCapabilityKey()
	token, err := IssueCapability(key, GoldenCapabilityAssignment)
	if err != nil {
		t.Fatal(err)
	}
	if token != GoldenCapabilityToken {
		t.Fatalf("token = %q", token)
	}
	assignment, err := VerifyCapability(key, token, "quick")
	if err != nil {
		t.Fatal(err)
	}
	if assignment != GoldenCapabilityAssignment {
		t.Fatalf("assignment = %#v", assignment)
	}
}

func TestCapabilityRejectsTamperingAndWrongAudience(t *testing.T) {
	key := goldenCapabilityKey()
	token := GoldenCapabilityToken
	tampered := token[:len(token)-1] + "A"
	if _, err := VerifyCapability(key, tampered, "quick"); err == nil {
		t.Fatal("tampered capability succeeded")
	}
	if _, err := VerifyCapability(key, token, "adc"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("wrong-audience error = %v", err)
	}
}

func TestSigningKeyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.key")
	if err := GenerateSigningKeyFile(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("key mode = %o", info.Mode().Perm())
	}
	key, err := LoadSigningKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(key) != 32 {
		t.Fatalf("key length = %d", len(key))
	}
	if err := GenerateSigningKeyFile(path); err == nil {
		t.Fatal("key generation replaced an existing file")
	}
	invalidPath := filepath.Join(t.TempDir(), "invalid.key")
	if err := os.WriteFile(invalidPath, []byte(strings.TrimSuffix(GoldenCapabilityKeyFile, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSigningKey(invalidPath); err == nil {
		t.Fatal("key without final newline succeeded")
	}
}

func goldenCapabilityKey() []byte {
	key := make([]byte, 32)
	for index := range key {
		key[index] = byte(index)
	}
	return key
}
