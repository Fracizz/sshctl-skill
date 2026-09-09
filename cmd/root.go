package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Fracizz/sshctl/internal/crypto"
)

var (
	cfgPath        string
	insecure       bool
	secure         bool
	masterPassword string
	bindMachine    bool
	// Version is overwritten by -ldflags at build time.
	Version = "0.4.0"
)

var rootCmd = &cobra.Command{
	Use:   "sshctl",
	Short: "AI-friendly SSH/SFTP sync CLI with encrypted server inventory",
	Long: `sshctl is a cross-platform SSH/SFTP sync CLI designed primarily for AI agents.

Exit codes:
  0   success
  1   local runtime error (dial, decrypt, I/O, …)
  2   usage / config error
  N   remote command exit status (sshctl exec only, when available)

Master password (recommended on shared machines):
  --master-password / SSHCTL_MASTER_PASSWORD  → enc:v2 (Argon2id + AES-GCM)
  --bind-machine / SSHCTL_BIND_MACHINE=1      → also bind v2 keys to this machine
  Without a master password, new secrets use legacy enc:v1 (machine-derived).

Skill / agent workflow (preferred):
  put binary next to SKILL.md as bin/sshctl.exe (see skills/sshctl/)
  local build: powershell -File scripts/build.ps1

Optional system PATH install (advanced, Administrator):
  sshctl install

Shell completion:
  sshctl completion bash|zsh|fish|powershell`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       Version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		insecure = resolveInsecure(insecure, secure)
		if masterPassword != "" {
			crypto.SetMasterPassword(masterPassword)
		}
		if cmd.Flags().Changed("bind-machine") {
			crypto.SetBindMachine(bindMachine)
		}
	},
}

// Execute runs the root command.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return err
	}
	return nil
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "", "path to servers JSON (default: ~/.sshctl/servers.json or $SSHCTL_CONFIG)")
	rootCmd.PersistentFlags().BoolVar(&insecure, "insecure", true, "skip SSH host key verification (default)")
	rootCmd.PersistentFlags().BoolVar(&secure, "secure", false, "verify SSH host key against ~/.ssh/known_hosts")
	rootCmd.MarkFlagsMutuallyExclusive("insecure", "secure")
	rootCmd.PersistentFlags().StringVar(&masterPassword, "master-password", "", "master password for enc:v2 (or set SSHCTL_MASTER_PASSWORD)")
	rootCmd.PersistentFlags().BoolVar(&bindMachine, "bind-machine", false, "mix machine identity into enc:v2 KDF (or SSHCTL_BIND_MACHINE=1)")

	rootCmd.SetVersionTemplate("sshctl {{.Version}}\n")
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(shellCmd)
	rootCmd.AddCommand(scpCmd)
	rootCmd.AddCommand(rsyncCmd)
	rootCmd.AddCommand(initCmd)
}

func resolveInsecure(insecureFlag, secureFlag bool) bool {
	if secureFlag {
		return false
	}
	return insecureFlag
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("sshctl %s\n", Version)
	},
}
