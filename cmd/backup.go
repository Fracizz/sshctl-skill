package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Fracizz/sshctl/internal/config"
)

var backupOut string

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Copy the local inventory (descriptions + ciphertext) to a backup file",
	Long: `Write a raw copy of the current servers.json. Passwords stay encrypted; descriptions are kept.

Default destination: ~/.sshctl/backups/servers-YYYYmmdd-HHMMSS.json`,
	Example: `  sshctl backup
  sshctl backup -o D:\safe\servers.json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		src := config.ResolvePath(cfgPath)
		dest, err := config.Backup(src, backupOut)
		if err != nil {
			return err
		}
		fmt.Printf("backed up %s -> %s\n", src, dest)
		return nil
	},
}

func init() {
	backupCmd.Flags().StringVarP(&backupOut, "out", "o", "", "backup file (default: ~/.sshctl/backups/servers-<timestamp>.json)")
}
