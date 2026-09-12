package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Fracizz/sshctl/internal/config"
	"github.com/Fracizz/sshctl/internal/crypto"
)

func TestSearchCaseInsensitiveContains(t *testing.T) {
	f := &config.File{Servers: []config.Server{
		{Name: "Lab-Alpha", Host: "192.0.2.10", Description: "Primary LAB"},
		{Name: "other", Host: "198.51.100.1", Description: "unused"},
	}}
	hits := f.Search("lab")
	if len(hits) != 1 || hits[0].Host != "192.0.2.10" {
		t.Fatalf("search lab: got %#v", hits)
	}
	hits = f.Search("192.0.2")
	if len(hits) != 1 {
		t.Fatalf("search ip fragment: got %d", len(hits))
	}
}

func TestFindExactThenFuzzy(t *testing.T) {
	f := &config.File{Servers: []config.Server{
		{Name: "web", Host: "192.0.2.10", User: "root"},
		{Name: "db", Host: "192.0.2.20", User: "root"},
	}}
	s, err := f.Find("web")
	if err != nil || s.Host != "192.0.2.10" {
		t.Fatalf("exact: %v %#v", err, s)
	}
	s, err = f.Find("192.0.2.20")
	if err != nil || s.Name != "db" {
		t.Fatalf("host: %v %#v", err, s)
	}
	if _, err := f.Find("192.0.2"); err == nil {
		t.Fatal("expected ambiguous error")
	}
}

func TestEncryptRoundTripOnSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	f := &config.File{}
	if _, err := f.Add(config.Server{Name: "t", Host: "192.0.2.10", User: "root", Password: "secret", OS: "Linux"}); err != nil {
		t.Fatal(err)
	}
	if !crypto.IsEncrypted(f.Servers[0].Password) {
		t.Fatal("expected encrypted password after Add")
	}
	if err := config.Save(path, f); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := loaded.Servers[0].PlainPassword()
	if err != nil || plain != "secret" {
		t.Fatalf("decrypt: %v %q", err, plain)
	}
}

func TestAddReplacesDuplicateHost(t *testing.T) {
	f := &config.File{}
	if updated, err := f.Add(config.Server{Name: "a", Host: "192.0.2.10", User: "root", Password: "one", OS: "Linux"}); err != nil || updated {
		t.Fatalf("first add: updated=%v err=%v", updated, err)
	}
	if updated, err := f.Add(config.Server{Name: "b", Host: "192.0.2.10", User: "admin", Password: "two", OS: "Windows"}); err != nil || !updated {
		t.Fatalf("second add: updated=%v err=%v", updated, err)
	}
	if len(f.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(f.Servers))
	}
	if f.Servers[0].Name != "b" || f.Servers[0].User != "admin" {
		t.Fatalf("unexpected server: %#v", f.Servers[0])
	}
	if err := f.ValidateUniqueHosts(); err != nil {
		t.Fatal(err)
	}
}

func TestAddUpdateKeepsOmittedPasswordAndDescription(t *testing.T) {
	f := &config.File{}
	if _, err := f.Add(config.Server{
		Name:        "lab",
		Host:        "192.0.2.10",
		User:        "root",
		Password:    "secret",
		OS:          "Windows",
		Port:        2222,
		Description: "build agent",
		KeyFile:     "C:/keys/lab.pem",
	}); err != nil {
		t.Fatal(err)
	}
	oldCipher := f.Servers[0].Password
	if !crypto.IsEncrypted(oldCipher) {
		t.Fatal("expected ciphertext after first add")
	}

	updated, err := f.Add(config.Server{Host: "192.0.2.10", User: "administrator"})
	if err != nil || !updated {
		t.Fatalf("update: updated=%v err=%v", updated, err)
	}
	got := f.Servers[0]
	if got.Password != oldCipher {
		t.Fatalf("password ciphertext overwritten: %q", got.Password)
	}
	if got.Description != "build agent" || got.Name != "lab" || got.OS != "Windows" || got.Port != 2222 || got.KeyFile != "C:/keys/lab.pem" {
		t.Fatalf("history fields lost: %#v", got)
	}
	if got.User != "administrator" {
		t.Fatalf("user not updated: %#v", got)
	}
	plain, err := got.PlainPassword()
	if err != nil || plain != "secret" {
		t.Fatalf("decrypt after merge: %v %q", err, plain)
	}
}

func TestAddUpdateReplacesExplicitPassword(t *testing.T) {
	f := &config.File{}
	if _, err := f.Add(config.Server{Host: "192.0.2.10", User: "root", Password: "old"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Add(config.Server{Host: "192.0.2.10", Password: "new"}); err != nil {
		t.Fatal(err)
	}
	plain, err := f.Servers[0].PlainPassword()
	if err != nil || plain != "new" {
		t.Fatalf("decrypt: %v %q", err, plain)
	}
	if f.Servers[0].User != "root" {
		t.Fatalf("user should be kept: %#v", f.Servers[0])
	}
}

func TestLoadRejectsDuplicateHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "servers.json")
	raw := `{
  "servers": [
    {"name":"a","host":"192.0.2.10","port":22,"user":"root","password":"","os":"Linux"},
    {"name":"b","host":"192.0.2.10","port":22,"user":"admin","password":"","os":"Windows"}
  ]
}
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(path); err == nil {
		t.Fatal("expected duplicate host error")
	}
}

func TestDefaultConfigPathOutsideCwd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	p := config.DefaultConfigPath()
	if filepath.Base(p) != "servers.json" {
		t.Fatalf("base: %s", p)
	}
	dir := filepath.Base(filepath.Dir(p))
	if dir != ".sshctl" {
		t.Fatalf("dir: %s", p)
	}
}

func TestMigrateLegacyFromSshfrac(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	legacyDir := filepath.Join(home, ".sshfrac")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(legacyDir, "servers.json")
	raw := `{"servers":[{"name":"lab","host":"192.0.2.10","port":22,"user":"root","password":"","os":"Linux"}]}`
	if err := os.WriteFile(legacyPath, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	from, err := config.MigrateLegacy()
	if err != nil {
		t.Fatal(err)
	}
	if from != legacyPath {
		t.Fatalf("from: %q", from)
	}
	primary := config.PrimaryConfigPath()
	if _, err := os.Stat(primary); err != nil {
		t.Fatalf("primary missing: %v", err)
	}
	if _, err := os.Stat(legacyPath); err == nil {
		t.Fatal("legacy file should be renamed")
	}
	if _, err := os.Stat(legacyPath + ".bak"); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	_, err = config.MigrateLegacy()
	if err != nil {
		t.Fatal(err)
	}
}

func TestResolvePathPriority(t *testing.T) {
	t.Setenv("SSHCTL_CONFIG", "")
	t.Setenv("SSHFRAC_CONFIG", "")
	t.Setenv("INVOSSH_CONFIG", "")
	if got := config.ResolvePath("/tmp/custom.json"); got != "/tmp/custom.json" {
		t.Fatalf("flag: %s", got)
	}
	t.Setenv("SSHCTL_CONFIG", "/primary/servers.json")
	if got := config.ResolvePath(""); got != "/primary/servers.json" {
		t.Fatalf("primary env: %s", got)
	}
	t.Setenv("SSHCTL_CONFIG", "")
	t.Setenv("SSHFRAC_CONFIG", "/legacy/sshfrac.json")
	if got := config.ResolvePath(""); got != "/legacy/sshfrac.json" {
		t.Fatalf("legacy sshfrac env: %s", got)
	}
	t.Setenv("SSHFRAC_CONFIG", "")
	t.Setenv("INVOSSH_CONFIG", "/legacy/invossh.json")
	if got := config.ResolvePath(""); got != "/legacy/invossh.json" {
		t.Fatalf("legacy invossh env: %s", got)
	}
}
