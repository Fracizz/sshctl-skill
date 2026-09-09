package sshx

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/term"

	"github.com/Fracizz/sshctl/internal/config"
)

const copyBufSize = 256 * 1024

// DialOptions controls SSH connection behavior.
type DialOptions struct {
	Timeout  time.Duration
	Insecure bool // skip host key verification (CLI default)
}

// Dial opens an SSH client for the given server entry.
func Dial(s *config.Server, opts DialOptions) (*ssh.Client, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}
	auth, err := authMethods(s)
	if err != nil {
		return nil, err
	}
	hostKeyCallback, err := hostKeyCallback(opts.Insecure)
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            s.User,
		Auth:            auth,
		HostKeyCallback: hostKeyCallback,
		Timeout:         opts.Timeout,
	}
	addr := net.JoinHostPort(s.Host, fmt.Sprintf("%d", s.Port))
	return ssh.Dial("tcp", addr, cfg)
}

func hostKeyCallback(insecure bool) (ssh.HostKeyCallback, error) {
	if insecure {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home for known_hosts: %w", err)
	}
	path := filepath.Join(home, ".ssh", "known_hosts")
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("known_hosts not found at %s (connect once with OpenSSH, or omit --secure): %w", path, err)
	}
	cb, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts: %w", err)
	}
	return cb, nil
}

func authMethods(s *config.Server) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod
	if s.KeyFile != "" {
		key, err := os.ReadFile(s.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("read key %s: %w", s.KeyFile, err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("parse key %s: %w", s.KeyFile, err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if s.Password != "" {
		plain, err := s.PlainPassword()
		if err != nil {
			return nil, err
		}
		// Use password auth only. Do not also register keyboard-interactive: when
		// password fails, Windows OpenSSH may reject keyboard-interactive with
		// USERAUTH_FAILURE (51) before INFO_REQUEST (60), masking the real error.
		methods = append(methods, ssh.Password(plain))
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("no auth method configured for %s", s.Name)
	}
	return methods, nil
}

// Run executes a remote command and streams stdout/stderr to the local process.
func Run(client *ssh.Client, command string) (int, error) {
	session, err := client.NewSession()
	if err != nil {
		return 1, err
	}
	defer session.Close()
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin
	if err := session.Run(command); err != nil {
		if ee, ok := err.(*ssh.ExitError); ok {
			return ee.ExitStatus(), nil
		}
		return 1, err
	}
	return 0, nil
}

// Shell starts an interactive login shell with PTY.
func Shell(client *ssh.Client) error {
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		session.Stdout = os.Stdout
		session.Stderr = os.Stderr
		session.Stdin = os.Stdin
		return session.Shell()
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer term.Restore(fd, oldState)

	w, h, err := term.GetSize(fd)
	if err != nil {
		w, h = 120, 40
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", h, w, modes); err != nil {
		return err
	}
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin
	if err := session.Shell(); err != nil {
		return err
	}
	return session.Wait()
}

// Upload copies localPath to remotePath via SFTP (SCP-compatible usage).
func Upload(client *ssh.Client, localPath, remotePath string) error {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	src, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer src.Close()

	info, err := src.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() {
		return uploadDir(sftpClient, localPath, remotePath)
	}
	return uploadFile(sftpClient, src, remotePath, info.Mode())
}

// Download copies remotePath to localPath via SFTP.
func Download(client *ssh.Client, remotePath, localPath string) error {
	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	info, err := sftpClient.Stat(remotePath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return downloadDir(sftpClient, remotePath, localPath)
	}
	return downloadFile(sftpClient, remotePath, localPath)
}

func uploadFile(c *sftp.Client, src *os.File, remotePath string, mode os.FileMode) error {
	_ = c.MkdirAll(filepath.ToSlash(filepath.Dir(remotePath)))
	dst, err := c.OpenFile(remotePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return err
	}
	defer dst.Close()
	buf := make([]byte, copyBufSize)
	if _, err := io.CopyBuffer(dst, src, buf); err != nil {
		return err
	}
	_ = c.Chmod(remotePath, mode)
	return nil
}

func downloadFile(c *sftp.Client, remotePath, localPath string) error {
	src, err := c.Open(remotePath)
	if err != nil {
		return err
	}
	defer src.Close()
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	dst, err := os.OpenFile(localPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer dst.Close()
	buf := make([]byte, copyBufSize)
	_, err = io.CopyBuffer(dst, src, buf)
	return err
}

func uploadDir(c *sftp.Client, localDir, remoteDir string) error {
	return filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}
		target := filepath.ToSlash(filepath.Join(remoteDir, rel))
		if info.IsDir() {
			return c.MkdirAll(target)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		return uploadFile(c, f, target, info.Mode())
	})
}

func downloadDir(c *sftp.Client, remoteDir, localDir string) error {
	walker := c.Walk(remoteDir)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(remoteDir, walker.Path())
		if err != nil {
			return err
		}
		target := filepath.Join(localDir, rel)
		if walker.Stat().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := downloadFile(c, walker.Path(), target); err != nil {
			return err
		}
	}
	return nil
}
