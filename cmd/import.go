package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Fracizz/sshctl/internal/config"
)

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Restore a local inventory backup (keeps ciphertext)",
	Long: `Replace the current servers.json with a backup file.

The backup is validated as inventory JSON. If a live inventory already exists,
it is snapshot to ~/.sshctl/backups/ first. Ciphertext is copied as-is.`,
	Example: `  sshctl import ~/.sshctl/backups/servers-20260912-154800.json
  sshctl import D:\safe\servers.json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dest := config.ResolvePath(cfgPath)
		imported, snapshot, err := config.ImportFile(args[0], dest)
		if err != nil {
			return err
		}
		fmt.Printf("imported %s -> %s\n", args[0], imported)
		if snapshot != "" {
			fmt.Printf("previous inventory saved to %s\n", snapshot)
		}
		return nil
	},
}
