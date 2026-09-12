package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Fracizz/sshctl/internal/config"
	"github.com/Fracizz/sshctl/internal/skillpack"
)

var skillsInstallZip string

var skillsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Upgrade-install the sshctl skill without wiping inventory",
	Long: `Overlay SKILL.md and bin/ from a skill zip into existing agent skill directories.

Never deletes extra files in the skill folder (notes, local copies, servers.json).
Never writes ~/.sshctl/servers.json. Do not delete the skill directory before running this.`,
	Example: `  sshctl skills install --zip sshctl-skill.zip`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if skillsInstallZip == "" {
			return fmt.Errorf("--zip is required")
		}
		if _, err := os.Stat(skillsInstallZip); err != nil {
			return fmt.Errorf("zip %s: %w", skillsInstallZip, err)
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		inv := config.ResolvePath(cfgPath)
		if _, err := os.Stat(inv); err == nil {
			bak, err := config.Backup(inv, "")
			if err != nil {
				return fmt.Errorf("backup inventory before skill upgrade: %w", err)
			}
			fmt.Printf("inventory backup %s\n", bak)
		}
		dests := skillpack.DefaultSkillDirs(home)
		if len(dests) == 0 {
			return fmt.Errorf("no ~/.claude/skills, ~/.cursor/skills, or ~/.codex/skills directory found")
		}
		for _, dest := range dests {
			res, err := skillpack.OverlayZip(skillsInstallZip, dest)
			if err != nil {
				return err
			}
			fmt.Printf("updated %s (wrote %d, preserved %d)\n", res.Dest, len(res.Wrote), len(res.Preserved))
		}
		fmt.Println("inventory unchanged: ~/.sshctl/servers.json")
		return nil
	},
}

func init() {
	skillsInstallCmd.Flags().StringVar(&skillsInstallZip, "zip", "", "sshctl-skill.zip or library download")
	skillsCmd.AddCommand(skillsInstallCmd)
}
