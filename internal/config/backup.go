package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// BackupDir is ~/.sshctl/backups (same home as the primary inventory).
func BackupDir() string {
	return filepath.Join(filepath.Dir(PrimaryConfigPath()), "backups")
}

// DefaultBackupPath returns a timestamped backup file under BackupDir.
func DefaultBackupPath() string {
	dir := BackupDir()
	stamp := time.Now().Format("20060102-150405.000")
	dest := filepath.Join(dir, "servers-"+stamp+".json")
	if _, err := os.Stat(dest); err != nil {
		return dest
	}
	return filepath.Join(dir, fmt.Sprintf("servers-%s-%d.json", stamp, time.Now().UnixNano()))
}

// Backup copies the inventory file as-is (ciphertext and descriptions preserved).
// dest empty uses DefaultBackupPath().
func Backup(src, dest string) (string, error) {
	if src == "" {
		src = PrimaryConfigPath()
	}
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("inventory not found: %s", src)
	}
	if dest == "" {
		dest = DefaultBackupPath()
	}
	if err := copyFile(src, dest, 0o600); err != nil {
		return "", err
	}
	return dest, nil
}

// ImportFile validates src as inventory JSON, snapshots dest if it exists, then copies src over dest.
func ImportFile(src, dest string) (string, string, error) {
	if src == "" {
		return "", "", fmt.Errorf("import path is required")
	}
	if dest == "" {
		dest = PrimaryConfigPath()
	}
	loaded, err := Load(src)
	if err != nil {
		return "", "", fmt.Errorf("import %s: %w", src, err)
	}
	if len(loaded.Servers) == 0 {
		return "", "", fmt.Errorf("import %s: inventory has no servers", src)
	}
	var snapshot string
	if _, err := os.Stat(dest); err == nil {
		snapshot, err = Backup(dest, "")
		if err != nil {
			return "", "", fmt.Errorf("snapshot current inventory: %w", err)
		}
	}
	if err := copyFile(src, dest, 0o600); err != nil {
		return "", snapshot, err
	}
	return dest, snapshot, nil
}

func copyFile(src, dest string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("read %s: %w", src, err)
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("copy %s -> %s: %w", src, dest, err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
