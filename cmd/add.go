package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Fracizz/sshctl/internal/config"
)

var (
	addName        string
	addDescription string
	addHost        string
	addPort        int
	addUser        string
	addPassword    string
	addOS          string
	addKeyFile     string
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add or update a server (password is encrypted on save)",
	Long: `Add a new host or update an existing one (matched by IP/host).

Unspecified fields are kept on update: description, encrypted password,
user, OS, key, name, and port are not cleared just because a flag was omitted.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if addHost == "" {
			return fmt.Errorf("--host is required")
		}
		path := config.ResolvePath(cfgPath)
		var f *config.File
		if _, err := os.Stat(path); os.IsNotExist(err) {
			f = &config.File{Servers: []config.Server{}}
		} else if err != nil {
			return err
		} else {
			loaded, err := config.Load(path)
			if err != nil {
				return err
			}
			f = loaded
		}
		existing := f.LookupHost(addHost) != nil
		if !existing && addUser == "" {
			return fmt.Errorf("--user is required when adding a new host")
		}
		updated, err := f.Add(serverFromAddFlags(cmd, existing))
		if err != nil {
			return err
		}
		if err := config.Save(path, f); err != nil {
			return err
		}
		saved := f.LookupHost(addHost)
		if saved == nil {
			return fmt.Errorf("save succeeded but host %s not found", addHost)
		}
		verb := "added"
		if updated {
			verb = "updated"
		}
		fmt.Printf("%s %s (%s@%s) -> %s\n", verb, saved.Name, saved.User, saved.Host, path)
		return nil
	},
}

func serverFromAddFlags(cmd *cobra.Command, existing bool) config.Server {
	s := config.Server{Host: addHost}
	if cmd.Flags().Changed("name") {
		s.Name = addName
	} else if !existing {
		s.Name = addHost
	}
	if cmd.Flags().Changed("desc") {
		s.Description = addDescription
	}
	if cmd.Flags().Changed("port") {
		s.Port = addPort
	} else if !existing {
		s.Port = addPort
	}
	if cmd.Flags().Changed("user") || !existing {
		s.User = addUser
	}
	if cmd.Flags().Changed("password") {
		s.Password = addPassword
	}
	if cmd.Flags().Changed("os") {
		s.OS = addOS
	} else if !existing {
		s.OS = addOS
	}
	if cmd.Flags().Changed("key") {
		s.KeyFile = addKeyFile
	}
	return s
}

func init() {
	addCmd.Flags().StringVar(&addName, "name", "", "display name")
	addCmd.Flags().StringVar(&addDescription, "desc", "", "server description")
	addCmd.Flags().StringVar(&addHost, "host", "", "hostname or IP")
	addCmd.Flags().IntVar(&addPort, "port", 22, "SSH port")
	addCmd.Flags().StringVar(&addUser, "user", "", "SSH username")
	addCmd.Flags().StringVar(&addPassword, "password", "", "SSH password (encrypted at rest)")
	addCmd.Flags().StringVar(&addOS, "os", "Linux", "OS label")
	addCmd.Flags().StringVar(&addKeyFile, "key", "", "optional private key path")
}
