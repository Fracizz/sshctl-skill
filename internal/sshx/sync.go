package sshx

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SyncOptions controls incremental local-to-remote synchronization.
type SyncOptions struct {
	Delete   bool
	DryRun   bool
	Checksum bool
	Progress io.Writer
}

// SyncStats summarizes one synchronization run.
type SyncStats struct {
	Scanned          int   `json:"scanned"`
	Uploaded         int   `json:"uploaded"`
	Skipped          int   `json:"skipped"`
	Deleted          int   `json:"deleted"`
	BytesTransferred int64 `json:"bytes_transferred"`
}

type localSyncEntry struct {
	path string
	info os.FileInfo
}

// SyncUpload incrementally copies a local file or directory to a remote path over SFTP.
// Directory contents are placed directly under remotePath, matching rsync's trailing-slash use.
func SyncUpload(client *ssh.Client, localPath, remotePath string, opts SyncOptions) (SyncStats, error) {
	var stats SyncStats
	info, err := os.Lstat(localPath)
	if err != nil {
		return stats, fmt.Errorf("stat local source: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return stats, fmt.Errorf("local source symlink is not supported: %s", localPath)
	}
	if opts.Delete && !info.IsDir() {
		return stats, fmt.Errorf("--delete requires a directory source")
	}
	if opts.Delete {
		if err := validateDeleteTarget(remotePath); err != nil {
			return stats, err
		}
	}

	c, err := sftp.NewClient(client)
	if err != nil {
		return stats, err
	}
	defer c.Close()

	if !info.IsDir() {
		stats.Scanned = 1
		changed, err := remoteFileChanged(c, localPath, remotePath, info, opts.Checksum)
		if err != nil {
			return stats, err
		}
		if !changed {
			stats.Skipped = 1
			return stats, nil
		}
		action := "upload"
		if opts.DryRun {
			action = "would-upload"
		}
		writeProgress(opts.Progress, action, filepath.Base(localPath), info.Size())
		if opts.DryRun {
			stats.Uploaded = 1
			stats.BytesTransferred = info.Size()
			return stats, nil
		}
		if err := uploadFileAtomic(c, localPath, remotePath, info); err != nil {
			return stats, err
		}
		stats.Uploaded = 1
		stats.BytesTransferred = info.Size()
		return stats, nil
	}

	entries, localSet, err := collectLocalEntries(localPath)
	if err != nil {
		return stats, err
	}
	if remoteInfo, statErr := c.Stat(remotePath); statErr == nil && !remoteInfo.IsDir() {
		return stats, fmt.Errorf("remote destination exists and is not a directory: %s", remotePath)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return stats, fmt.Errorf("stat remote root %s: %w", remotePath, statErr)
	}
	if !opts.DryRun {
		if err := c.MkdirAll(remotePath); err != nil {
			return stats, fmt.Errorf("create remote root %s: %w", remotePath, err)
		}
	}
	for _, entry := range entries {
		stats.Scanned++
		remote := remoteJoin(remotePath, entry.path)
		if entry.info.IsDir() {
			if !opts.DryRun {
				if err := c.MkdirAll(remote); err != nil {
					return stats, fmt.Errorf("create remote directory %s: %w", remote, err)
				}
			}
			continue
		}
		changed, err := remoteFileChanged(c, filepath.Join(localPath, filepath.FromSlash(entry.path)), remote, entry.info, opts.Checksum)
		if err != nil {
			return stats, err
		}
		if !changed {
			stats.Skipped++
			continue
		}
		action := "upload"
		if opts.DryRun {
			action = "would-upload"
		}
		writeProgress(opts.Progress, action, entry.path, entry.info.Size())
		if !opts.DryRun {
			if err := uploadFileAtomic(c, filepath.Join(localPath, filepath.FromSlash(entry.path)), remote, entry.info); err != nil {
				return stats, err
			}
		}
		stats.Uploaded++
		stats.BytesTransferred += entry.info.Size()
	}

	if opts.Delete {
		deleted, err := deleteRemoteExtras(c, remotePath, localSet, opts)
		if err != nil {
			return stats, err
		}
		stats.Deleted = deleted
	}
	return stats, nil
}

func collectLocalEntries(root string) ([]localSyncEntry, map[string]bool, error) {
	entries := make([]localSyncEntry, 0)
	set := make(map[string]bool)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("local symlink is not supported: %s", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		entries = append(entries, localSyncEntry{path: rel, info: info})
		set[rel] = info.IsDir()
		return nil
	})
	return entries, set, err
}

func remoteFileChanged(c *sftp.Client, localPath, remotePath string, local os.FileInfo, checksum bool) (bool, error) {
	remote, err := c.Stat(remotePath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("stat remote file %s: %w", remotePath, err)
	}
	if remote.IsDir() {
		return false, fmt.Errorf("remote path is a directory but local source is a file: %s", remotePath)
	}
	if remote.Size() != local.Size() {
		return true, nil
	}
	if checksum {
		localDigest, err := hashLocalFile(localPath)
		if err != nil {
			return false, err
		}
		remoteDigest, err := hashRemoteFile(c, remotePath)
		if err != nil {
			return false, err
		}
		return localDigest != remoteDigest, nil
	}
	return local.ModTime().Unix() != remote.ModTime().Unix(), nil
}

func uploadFileAtomic(c *sftp.Client, localPath, remotePath string, info os.FileInfo) error {
	if err := c.MkdirAll(filepath.ToSlash(filepath.Dir(remotePath))); err != nil {
		return err
	}
	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()
	tmp := fmt.Sprintf("%s.sshctl-part-%d", remotePath, os.Getpid())
	dst, err := c.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	_, copyErr := io.CopyBuffer(dst, src, make([]byte, copyBufSize))
	closeErr := dst.Close()
	if copyErr != nil {
		_ = c.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = c.Remove(tmp)
		return closeErr
	}
	_ = c.Chmod(tmp, info.Mode())
	_ = c.Chtimes(tmp, info.ModTime(), info.ModTime())
	if err := c.PosixRename(tmp, remotePath); err != nil {
		_ = c.Remove(remotePath)
		if renameErr := c.Rename(tmp, remotePath); renameErr != nil {
			_ = c.Remove(tmp)
			return fmt.Errorf("replace remote file %s: %w", remotePath, renameErr)
		}
	}
	return nil
}

func deleteRemoteExtras(c *sftp.Client, remoteRoot string, localSet map[string]bool, opts SyncOptions) (int, error) {
	if _, err := c.Stat(remoteRoot); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	walker := c.Walk(remoteRoot)
	files := make([]string, 0)
	dirs := make([]string, 0)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return 0, err
		}
		if walker.Path() == remoteRoot {
			continue
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(filepath.ToSlash(walker.Path()), filepath.ToSlash(remoteRoot)), "/")
		if _, ok := localSet[rel]; ok {
			continue
		}
		if walker.Stat().IsDir() {
			dirs = append(dirs, walker.Path())
		} else {
			files = append(files, walker.Path())
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return strings.Count(dirs[i], "/") > strings.Count(dirs[j], "/") })
	deleted := 0
	action := "delete"
	if opts.DryRun {
		action = "would-delete"
	}
	for _, path := range files {
		writeProgress(opts.Progress, action, strings.TrimPrefix(path, remoteRoot+"/"), 0)
		if !opts.DryRun {
			if err := c.Remove(path); err != nil {
				return deleted, err
			}
		}
		deleted++
	}
	for _, path := range dirs {
		writeProgress(opts.Progress, action, strings.TrimPrefix(path, remoteRoot+"/"), 0)
		if !opts.DryRun {
			if err := c.RemoveDirectory(path); err != nil && !os.IsNotExist(err) {
				return deleted, err
			}
		}
		deleted++
	}
	return deleted, nil
}

func validateDeleteTarget(remotePath string) error {
	clean := strings.TrimSpace(filepath.ToSlash(remotePath))
	clean = pathpkg.Clean(clean)
	isDriveRoot := len(clean) >= 2 && clean[1] == ':' && (len(clean) == 2 || (len(clean) == 3 && clean[2] == '/'))
	isShallowAbsolute := strings.HasPrefix(clean, "/") && !strings.Contains(strings.Trim(clean, "/"), "/")
	if clean == "" || clean == "." || clean == ".." || clean == "/" || clean == "~" ||
		strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "~/") || isDriveRoot || isShallowAbsolute {
		return fmt.Errorf("refusing --delete for unsafe remote path %q", remotePath)
	}
	return nil
}

func remoteJoin(root, rel string) string {
	return strings.TrimRight(filepath.ToSlash(root), "/") + "/" + strings.TrimLeft(rel, "/")
}

func hashLocalFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return hashReader(f)
}

func hashRemoteFile(c *sftp.Client, path string) (string, error) {
	f, err := c.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return hashReader(f)
}

func hashReader(r io.Reader) (string, error) {
	digest := sha256.New()
	if _, err := io.CopyBuffer(digest, r, make([]byte, copyBufSize)); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func writeProgress(w io.Writer, action, path string, size int64) {
	if w == nil {
		return
	}
	if size > 0 {
		_, _ = fmt.Fprintf(w, "[%s] %s (%d bytes)\n", action, path, size)
		return
	}
	_, _ = fmt.Fprintf(w, "[%s] %s\n", action, path)
}
