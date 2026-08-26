package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Fracizz/sshctl/internal/config"
	"github.com/Fracizz/sshctl/internal/sshx"
)

var (
	rsyncTimeout  time.Duration
	rsyncDelete   bool
	rsyncDryRun   bool
	rsyncChecksum bool
	rsyncQuiet    bool
	rsyncJSON     bool
)

var rsyncCmd = &cobra.Command{
	Use:   "rsync <local-src> <server:remote-dst>",
	Short: "Incrementally sync local files to a remote host over SFTP",
	Long: `Incrementally synchronize a local file or directory to a remote host.

The command reuses sshctl's encrypted inventory and does not require a local
rsync executable. Files are compared by size and modification time by default.
Use --checksum for SHA-256 comparison and --delete to remove remote extras.

Examples:
  sshctl rsync ./app-store prod:/srv/app-store
  sshctl rsync ./app-store prod:/srv/app-store --delete
  sshctl rsync ./app.tar.gz prod:/tmp/app.tar.gz --checksum
  sshctl rsync ./app-store prod:/srv/app-store --dry-run --json
`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, dst := args[0], args[1]
		if strings.Contains(src, ":") && !isWindowsDrive(src) {
			return fmt.Errorf("remote-to-local rsync is not supported yet")
		}
		serverQuery, remotePath, err := splitRemote(dst)
		if err != nil {
			return err
		}
		path := config.ResolvePath(cfgPath)
		inventory, err := config.Load(path)
		if err != nil {
			return err
		}
		server, err := inventory.Find(serverQuery)
		if err != nil {
			return err
		}
		client, err := sshx.Dial(server, sshx.DialOptions{Timeout: rsyncTimeout, Insecure: insecure})
		if err != nil {
			return err
		}
		defer client.Close()

		var progress *os.File
		if !rsyncQuiet && !rsyncJSON {
			progress = os.Stderr
		}
		stats, err := sshx.SyncUpload(client, src, remotePath, sshx.SyncOptions{
			Delete:   rsyncDelete,
			DryRun:   rsyncDryRun,
			Checksum: rsyncChecksum,
			Progress: progress,
		})
		if err != nil {
			return err
		}
		if rsyncJSON {
			return json.NewEncoder(os.Stdout).Encode(stats)
		}
		fmt.Printf("sync complete: scanned=%d uploaded=%d skipped=%d deleted=%d bytes=%d dry_run=%t\n",
			stats.Scanned, stats.Uploaded, stats.Skipped, stats.Deleted, stats.BytesTransferred, rsyncDryRun)
		return nil
	},
}

func init() {
	rsyncCmd.Flags().DurationVar(&rsyncTimeout, "timeout", 15*time.Second, "SSH dial timeout")
	rsyncCmd.Flags().BoolVar(&rsyncDelete, "delete", false, "delete remote files that are absent locally")
	rsyncCmd.Flags().BoolVar(&rsyncDryRun, "dry-run", false, "show changes without modifying the remote host")
	rsyncCmd.Flags().BoolVar(&rsyncChecksum, "checksum", false, "compare SHA-256 instead of modification time")
	rsyncCmd.Flags().BoolVarP(&rsyncQuiet, "quiet", "q", false, "suppress per-file progress")
	rsyncCmd.Flags().BoolVar(&rsyncJSON, "json", false, "write the final summary as JSON")
}
