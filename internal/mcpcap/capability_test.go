package mcpcap

import (
	"bytes"
	"testing"
)

func TestCompatibilityVector(t *testing.T) {
	key := make([]byte, 32)
	for index := range key {
		key[index] = byte(index)
	}
	encodedKey, err := EncodeKeyFile(key)
	if err != nil {
		t.Fatal(err)
	}
	const wantKeyFile = "adjmcpkey1.AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8\n"
	if string(encodedKey) != wantKeyFile {
		t.Fatalf("key file = %q", encodedKey)
	}
	parsedKey, err := ParseKeyFile([]byte(wantKeyFile))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(parsedKey, key) {
		t.Fatalf("parsed key = %x", parsedKey)
	}
	if _, err := ParseKeyFile(encodedKey[:len(encodedKey)-1]); err == nil {
		t.Fatal("key file without its terminating newline was accepted")
	}
	token, err := Issue(key, Claims{
		Audience:       "quick",
		CaseID:         "case-alpha",
		AssignmentType: "lawyer",
		PrincipalID:    "plaintiff",
	})
	if err != nil {
		t.Fatal(err)
	}
	const wantToken = "adjmcp1.eyJ2ZXJzaW9uIjoiYWRqLm1jcC5jYXBhYmlsaXR5LnYxIiwiYXVkaWVuY2UiOiJxdWljayIsImNhc2VfaWQiOiJjYXNlLWFscGhhIiwiYXNzaWdubWVudF90eXBlIjoibGF3eWVyIiwicHJpbmNpcGFsX2lkIjoicGxhaW50aWZmIn0.9W98_OD1jQN6xF0fcfUKmTWOgr2QFLYa2tQyxkalUCg"
	if token != wantToken {
		t.Fatalf("token = %q", token)
	}
	for _, claims := range []Claims{
		{Audience: "other", CaseID: "case-alpha", AssignmentType: "lawyer", PrincipalID: "plaintiff"},
		{Audience: "quick", CaseID: " case-alpha", AssignmentType: "lawyer", PrincipalID: "plaintiff"},
		{Audience: "quick", CaseID: "case-alpha", AssignmentType: "council", PrincipalID: "member-1"},
	} {
		if _, err := Issue(key, claims); err == nil {
			t.Fatalf("invalid claims were accepted: %#v", claims)
		}
	}
}
