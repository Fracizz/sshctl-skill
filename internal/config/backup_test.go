package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fracizz/sshctl/internal/config"
	"github.com/Fracizz/sshctl/internal/crypto"
)

func TestBackupAndImportPreservesCiphertext(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	src := filepath.Join(home, "live.json")
	f := &config.File{}
	if _, err := f.Add(config.Server{
		Name:        "lab",
		Host:        "192.0.2.10",
		User:        "root",
		Password:    "secret",
		Description: "keep this",
		OS:          "Linux",
	}); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(src, f); err != nil {
		t.Fatal(err)
	}
	cipher := f.Servers[0].Password
	if !crypto.IsEncrypted(cipher) {
		t.Fatal("expected ciphertext")
	}

	bak, err := config.Backup(src, "")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(bak)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), cipher) || !strings.Contains(string(raw), "keep this") {
		t.Fatalf("backup lost history or ciphertext: %s", raw)
	}

	other := filepath.Join(home, "other.json")
	empty := &config.File{}
	if _, err := empty.Add(config.Server{Host: "198.51.100.1", User: "u", Password: "x", Description: "replace me"}); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(other, empty); err != nil {
		t.Fatal(err)
	}

	dest, snapshot, err := config.ImportFile(bak, other)
	if err != nil {
		t.Fatal(err)
	}
	if dest != other || snapshot == "" {
		t.Fatalf("dest=%q snapshot=%q", dest, snapshot)
	}
	loaded, err := config.Load(other)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Servers) != 1 || loaded.Servers[0].Host != "192.0.2.10" || loaded.Servers[0].Description != "keep this" {
		t.Fatalf("import: %#v", loaded.Servers)
	}
	if loaded.Servers[0].Password != cipher {
		t.Fatalf("ciphertext changed: %q vs %q", loaded.Servers[0].Password, cipher)
	}
	plain, err := loaded.Servers[0].PlainPassword()
	if err != nil || plain != "secret" {
		t.Fatalf("decrypt after import: %v %q", err, plain)
	}
}

func TestImportRejectsEmptyInventory(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(src, []byte(`{"servers":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := config.ImportFile(src, filepath.Join(dir, "dest.json")); err == nil {
		t.Fatal("expected empty inventory error")
	}
}
