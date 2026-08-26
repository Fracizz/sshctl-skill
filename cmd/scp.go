package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Fracizz/sshctl/internal/config"
	"github.com/Fracizz/sshctl/internal/sshx"
)

var (
	scpTimeout       time.Duration
	scpArtifactCache string
)

var scpCmd = &cobra.Command{
	Use:   "scp <src> <dst> [dst...]",
	Short: "Copy files via SFTP (scp-compatible paths: server:path)",
	Long: `Copy files between local and remote hosts.

Remote-to-remote copies are staged once in the local ArtifactCache. Multiple
remote destinations are supported, so one download can fan out to many hosts.

Examples:
  sshctl scp ./a.txt example-host:/tmp/a.txt
  sshctl scp example-host:/etc/hosts ./hosts
  sshctl scp ./dir example-host:/tmp/dir
  sshctl scp source:/tmp/app.tar target-a:/tmp/app.tar target-b:/tmp/app.tar
`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, destinations := args[0], args[1:]
		srcRemote, dstRemote, err := validateScpEndpoints(src, destinations)
		if err != nil {
			return err
		}

		path := config.ResolvePath(cfgPath)
		f, err := config.Load(path)
		if err != nil {
			return err
		}

		if srcRemote && len(destinations) == 1 && !dstRemote[0] {
			serverQuery, remotePath, err := splitRemote(src)
			if err != nil {
				return err
			}
			s, err := f.Find(serverQuery)
			if err != nil {
				return err
			}
			client, err := sshx.Dial(s, sshx.DialOptions{Timeout: scpTimeout, Insecure: insecure})
			if err != nil {
				return err
			}
			defer client.Close()
			return sshx.Download(client, remotePath, destinations[0])
		}

		if srcRemote {
			return copyRemoteToRemotes(f, src, destinations)
		}

		for _, dst := range destinations {
			if err := uploadToRemote(f, src, dst); err != nil {
				return err
			}
		}
		return nil
	},
}

func uploadToRemote(f *config.File, localPath, destination string) error {
	serverQuery, remotePath, err := splitRemote(destination)
	if err != nil {
		return err
	}
	s, err := f.Find(serverQuery)
	if err != nil {
		return err
	}
	client, err := sshx.Dial(s, sshx.DialOptions{Timeout: scpTimeout, Insecure: insecure})
	if err != nil {
		return fmt.Errorf("connect destination %q: %w", serverQuery, err)
	}
	defer client.Close()
	if err := sshx.Upload(client, localPath, remotePath); err != nil {
		return fmt.Errorf("upload to %q: %w", destination, err)
	}
	return nil
}

func copyRemoteToRemotes(f *config.File, source string, destinations []string) error {
	serverQuery, remotePath, err := splitRemote(source)
	if err != nil {
		return err
	}
	s, err := f.Find(serverQuery)
	if err != nil {
		return err
	}
	client, err := sshx.Dial(s, sshx.DialOptions{Timeout: scpTimeout, Insecure: insecure})
	if err != nil {
		return fmt.Errorf("connect source %q: %w", serverQuery, err)
	}

	cacheRoot, err := artifactCacheRoot()
	if err != nil {
		client.Close()
		return err
	}
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		client.Close()
		return fmt.Errorf("create ArtifactCache %q: %w", cacheRoot, err)
	}
	stageDir, err := os.MkdirTemp(cacheRoot, "scp-")
	if err != nil {
		client.Close()
		return fmt.Errorf("create ArtifactCache staging directory: %w", err)
	}
	defer os.RemoveAll(stageDir)

	stagedPath := filepath.Join(stageDir, "payload")
	if err := sshx.Download(client, remotePath, stagedPath); err != nil {
		client.Close()
		return fmt.Errorf("download %q to ArtifactCache: %w", source, err)
	}
	if err := client.Close(); err != nil {
		return fmt.Errorf("close source connection: %w", err)
	}

	for _, dst := range destinations {
		if err := uploadToRemote(f, stagedPath, dst); err != nil {
			return err
		}
	}
	return nil
}

func artifactCacheRoot() (string, error) {
	if scpArtifactCache != "" {
		return filepath.Abs(scpArtifactCache)
	}
	if configured := os.Getenv("SSHCTL_ARTIFACT_CACHE"); configured != "" {
		return filepath.Abs(configured)
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve local cache directory: %w", err)
	}
	return filepath.Join(cacheDir, "sshctl", "ArtifactCache"), nil
}

func init() {
	scpCmd.Flags().DurationVar(&scpTimeout, "timeout", 15*time.Second, "SSH dial timeout")
	scpCmd.Flags().StringVar(&scpArtifactCache, "artifact-cache", "", "local staging root for remote-to-remote copies (or SSHCTL_ARTIFACT_CACHE)")
}

func isWindowsDrive(p string) bool {
	return len(p) >= 2 && p[1] == ':' && ((p[0] >= 'a' && p[0] <= 'z') || (p[0] >= 'A' && p[0] <= 'Z'))
}

func isRemotePath(p string) bool {
	return strings.Contains(p, ":") && !isWindowsDrive(p)
}

func validateScpEndpoints(source string, destinations []string) (bool, []bool, error) {
	srcRemote := isRemotePath(source)
	dstRemote := make([]bool, len(destinations))
	for i, dst := range destinations {
		dstRemote[i] = isRemotePath(dst)
	}

	if !srcRemote {
		for i, remote := range dstRemote {
			if !remote {
				return false, nil, fmt.Errorf("destination %q must be remote (server:path)", destinations[i])
			}
		}
	} else if len(destinations) > 1 {
		for i, remote := range dstRemote {
			if !remote {
				return false, nil, fmt.Errorf("multiple destinations must all be remote; destination %q is local", destinations[i])
			}
		}
	}
	return srcRemote, dstRemote, nil
}

func splitRemote(spec string) (server, remotePath string, err error) {
	// Prefer last colon so names/IPv6-ish forms still work with path.
	idx := strings.LastIndex(spec, ":")
	if idx <= 0 || idx == len(spec)-1 {
		return "", "", fmt.Errorf("invalid remote path %q (want server:path)", spec)
	}
	return spec[:idx], spec[idx+1:], nil
}
