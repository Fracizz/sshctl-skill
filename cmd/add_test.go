package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestServerFromAddFlagsOmitsUnchangedOnUpdate(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().StringVar(&addName, "name", "", "")
	cmd.Flags().StringVar(&addDescription, "desc", "", "")
	cmd.Flags().StringVar(&addHost, "host", "", "")
	cmd.Flags().IntVar(&addPort, "port", 22, "")
	cmd.Flags().StringVar(&addUser, "user", "", "")
	cmd.Flags().StringVar(&addPassword, "password", "", "")
	cmd.Flags().StringVar(&addOS, "os", "Linux", "")
	cmd.Flags().StringVar(&addKeyFile, "key", "", "")
	if err := cmd.ParseFlags([]string{"--host", "192.0.2.10", "--user", "admin"}); err != nil {
		t.Fatal(err)
	}
	s := serverFromAddFlags(cmd, true)
	if s.Host != "192.0.2.10" || s.User != "admin" {
		t.Fatalf("expected host/user, got %#v", s)
	}
	if s.Password != "" || s.Description != "" || s.Name != "" || s.KeyFile != "" {
		t.Fatalf("omitted fields should stay empty for merge: %#v", s)
	}
	if s.Port != 0 {
		t.Fatalf("default port should not apply on update: %#v", s)
	}
	if s.OS != "" {
		t.Fatalf("default OS should not apply on update: %#v", s)
	}
}

func TestServerFromAddFlagsNewHostUsesDefaults(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().StringVar(&addName, "name", "", "")
	cmd.Flags().StringVar(&addDescription, "desc", "", "")
	cmd.Flags().StringVar(&addHost, "host", "", "")
	cmd.Flags().IntVar(&addPort, "port", 22, "")
	cmd.Flags().StringVar(&addUser, "user", "", "")
	cmd.Flags().StringVar(&addPassword, "password", "", "")
	cmd.Flags().StringVar(&addOS, "os", "Linux", "")
	cmd.Flags().StringVar(&addKeyFile, "key", "", "")
	addHost, addUser, addName, addDescription, addPassword, addOS, addKeyFile = "192.0.2.10", "root", "", "", "", "Linux", ""
	addPort = 22
	if err := cmd.ParseFlags([]string{"--host", "192.0.2.10", "--user", "root"}); err != nil {
		t.Fatal(err)
	}
	s := serverFromAddFlags(cmd, false)
	if s.Name != "192.0.2.10" || s.Port != 22 || s.OS != "Linux" || s.User != "root" {
		t.Fatalf("new host defaults: %#v", s)
	}
}
