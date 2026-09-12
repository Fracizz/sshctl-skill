package skillpack

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOverlayZipKeepsUserData(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "sshctl.zip")
	if err := writeTestZip(zipPath, map[string]string{
		"sshctl/SKILL.md":       "# new skill\n",
		"sshctl/bin/sshctl.exe": "newbin",
		"sshctl/servers.json":   `{"servers":[]}`,
	}); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(dir, "installed")
	if err := os.MkdirAll(filepath.Join(dest, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "SKILL.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "servers.json"), []byte(`{"servers":[{"host":"keep-me","password":"enc:v1:x"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "notes.md"), []byte("history"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "bin", "old.exe"), []byte("oldbin"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := OverlayZip(zipPath, dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Wrote) == 0 {
		t.Fatal("expected writes")
	}

	gotSkill, _ := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	if string(gotSkill) != "# new skill\n" {
		t.Fatalf("SKILL.md: %q", gotSkill)
	}
	gotInv, _ := os.ReadFile(filepath.Join(dest, "servers.json"))
	if !strings.Contains(string(gotInv), "keep-me") {
		t.Fatalf("inventory overwritten: %s", gotInv)
	}
	gotNotes, _ := os.ReadFile(filepath.Join(dest, "notes.md"))
	if string(gotNotes) != "history" {
		t.Fatalf("history lost: %q", gotNotes)
	}
	if _, err := os.Stat(filepath.Join(dest, "bin", "old.exe")); err != nil {
		t.Fatal("existing extra binary was deleted")
	}
	gotBin, _ := os.ReadFile(filepath.Join(dest, "bin", "sshctl.exe"))
	if string(gotBin) != "newbin" {
		t.Fatalf("bin not updated: %q", gotBin)
	}
}

func TestOverlayZipFlatLayout(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "flat.zip")
	if err := writeTestZip(zipPath, map[string]string{
		"SKILL.md":       "flat\n",
		"bin/sshctl.exe": "exe",
	}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "dest")
	if _, err := OverlayZip(zipPath, dest); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	if err != nil || string(b) != "flat\n" {
		t.Fatalf("flat skill: %v %q", err, b)
	}
}

func writeTestZip(path string, files map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	for name, body := range files {
		fw, err := w.Create(name)
		if err != nil {
			return err
		}
		if _, err := fw.Write([]byte(body)); err != nil {
			return err
		}
	}
	return w.Close()
}
