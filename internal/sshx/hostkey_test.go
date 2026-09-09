package sshx

import (
	"strings"
	"testing"
)

func TestHostKeyCallbackInsecure(t *testing.T) {
	cb, err := hostKeyCallback(true)
	if err != nil || cb == nil {
		t.Fatalf("insecure callback: %v", err)
	}
}

func TestHostKeyCallbackMissingFile(t *testing.T) {
	t.Setenv("USERPROFILE", t.TempDir()) // windows
	t.Setenv("HOME", t.TempDir())
	_, err := hostKeyCallback(false)
	if err == nil {
		t.Fatal("expected error when known_hosts missing")
	}
	if !strings.Contains(err.Error(), "known_hosts") {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(err.Error(), "omit --secure") {
		t.Fatalf("expected omit --secure hint, got: %v", err)
	}
}
