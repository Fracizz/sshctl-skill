package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Fracizz/sshctl/internal/crypto"
)

// Server is one SSH endpoint entry.
type Server struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	Password    string `json:"password"`
	OS          string `json:"os"`
	KeyFile     string `json:"key_file,omitempty"`
}

// File is the on-disk JSON document.
type File struct {
	Servers []Server `json:"servers"`
}

// ResolvePath picks config path: flag > SSHCTL_CONFIG > SSHFRAC_CONFIG > INVOSSH_CONFIG > default.
// Default path auto-migrates legacy ~/.sshfrac or ~/.invossh inventories to ~/.sshctl.
func ResolvePath(flagPath string) string {
	if flagPath != "" {
		return flagPath
	}
	for _, key := range []string{"SSHCTL_CONFIG", "SSHFRAC_CONFIG", "INVOSSH_CONFIG"} {
		if env := os.Getenv(key); env != "" {
			return env
		}
	}
	_, _ = MigrateLegacy()
	return PrimaryConfigPath()
}

// Load reads JSON, encrypts any plaintext passwords, and rewrites the file when needed.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	changed := false
	for i := range f.Servers {
		s := &f.Servers[i]
		if s.Port == 0 {
			s.Port = 22
		}
		if s.Password == "" || crypto.IsEncrypted(s.Password) {
			continue
		}
		enc, err := crypto.Encrypt(s.Password)
		if err != nil {
			return nil, fmt.Errorf("encrypt password for %s: %w", s.Name, err)
		}
		s.Password = enc
		changed = true
	}
	if err := f.ValidateUniqueHosts(); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	if changed {
		if err := Save(path, &f); err != nil {
			return nil, err
		}
	}
	return &f, nil
}

// ValidateUniqueHosts reports duplicate host/IP entries in the inventory.
func (f *File) ValidateUniqueHosts() error {
	seen := make(map[string]string, len(f.Servers))
	for _, s := range f.Servers {
		host := normalizeHost(s.Host)
		if host == "" {
			return fmt.Errorf("server %q has empty host", s.Name)
		}
		if prev, ok := seen[host]; ok {
			return fmt.Errorf("duplicate host %s (%q and %q)", s.Host, prev, s.Name)
		}
		seen[host] = s.Name
	}
	return nil
}

func normalizeHost(host string) string {
	return strings.ToLower(strings.TrimSpace(host))
}

// Save writes JSON with indentation.
func Save(path string, f *File) error {
	if err := f.ValidateUniqueHosts(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Find locates a server by name, host, or "user@host".
// Exact match first; if none, case-insensitive substring on name/host (must be unique).
func (f *File) Find(query string) (*Server, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("empty server query")
	}
	userHint := ""
	hostHint := q
	if strings.Contains(q, "@") {
		parts := strings.SplitN(q, "@", 2)
		userHint, hostHint = parts[0], parts[1]
	}
	for i := range f.Servers {
		s := &f.Servers[i]
		if s.Name == q || s.Host == q || s.Host == hostHint {
			if userHint != "" && s.User != "" && s.User != userHint {
				continue
			}
			return s, nil
		}
		if s.User+"@"+s.Host == q {
			return s, nil
		}
	}
	hits := f.Search(q)
	if userHint != "" {
		filtered := hits[:0]
		for _, s := range hits {
			if s.User == userHint {
				filtered = append(filtered, s)
			}
		}
		hits = filtered
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return nil, fmt.Errorf("server not found: %s", query)
	default:
		return nil, fmt.Errorf("ambiguous server %q: %d matches (use sshctl search -s %q)", query, len(hits), query)
	}
}

// Search returns servers whose name, host, or description contains keyword (case-insensitive).
func (f *File) Search(keyword string) []*Server {
	k := strings.ToLower(strings.TrimSpace(keyword))
	if k == "" {
		out := make([]*Server, 0, len(f.Servers))
		for i := range f.Servers {
			out = append(out, &f.Servers[i])
		}
		return out
	}
	var out []*Server
	for i := range f.Servers {
		s := &f.Servers[i]
		if strings.Contains(strings.ToLower(s.Name), k) ||
			strings.Contains(strings.ToLower(s.Host), k) ||
			strings.Contains(strings.ToLower(s.Description), k) {
			out = append(out, s)
		}
	}
	return out
}

// PlainPassword decrypts the stored password for runtime use.
func (s *Server) PlainPassword() (string, error) {
	return crypto.Decrypt(s.Password)
}

// LookupHost returns the inventory entry for host/IP, or nil.
func (f *File) LookupHost(host string) *Server {
	target := normalizeHost(host)
	if target == "" {
		return nil
	}
	for i := range f.Servers {
		if normalizeHost(f.Servers[i].Host) == target {
			return &f.Servers[i]
		}
	}
	return nil
}

func mergeServerUpdate(old, neu Server) Server {
	if neu.Name == "" {
		neu.Name = old.Name
	}
	if neu.Description == "" {
		neu.Description = old.Description
	}
	if neu.User == "" {
		neu.User = old.User
	}
	if neu.Password == "" {
		neu.Password = old.Password
	}
	if neu.OS == "" {
		neu.OS = old.OS
	}
	if neu.KeyFile == "" {
		neu.KeyFile = old.KeyFile
	}
	if neu.Port == 0 {
		neu.Port = old.Port
	}
	if neu.Host == "" {
		neu.Host = old.Host
	}
	return neu
}

// Add inserts or updates the server for a host. Each host/IP may appear only once.
// Empty fields on update keep the previous values (description, ciphertext, key, …).
// Returns true when an existing host entry was updated.
func (f *File) Add(s Server) (bool, error) {
	if normalizeHost(s.Host) == "" {
		return false, fmt.Errorf("host is required")
	}
	if s.Password != "" && !crypto.IsEncrypted(s.Password) {
		enc, err := crypto.Encrypt(s.Password)
		if err != nil {
			return false, err
		}
		s.Password = enc
	}
	target := normalizeHost(s.Host)
	for i := range f.Servers {
		if normalizeHost(f.Servers[i].Host) == target {
			s = mergeServerUpdate(f.Servers[i], s)
			if s.Port == 0 {
				s.Port = 22
			}
			f.Servers[i] = s
			return true, nil
		}
	}
	if s.Port == 0 {
		s.Port = 22
	}
	f.Servers = append(f.Servers, s)
	return false, nil
}
