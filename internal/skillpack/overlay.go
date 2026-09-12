package skillpack

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var userDataNames = map[string]struct{}{
	"servers.json":           {},
	"servers.json.bak":       {},
	".env":                   {},
	"user.json":              {},
	"sites.json":             {},
	"token.json":             {},
	".cs-skills-deploy.json": {},
}

// Result is a dry summary of an overlay install.
type Result struct {
	Dest      string
	Wrote     []string
	Skipped   []string
	Preserved []string
}

// OverlayZip writes SKILL.md and bin/* from zip into dest without deleting
// other files. Inventory and secret filenames are never taken from the zip.
func OverlayZip(zipPath, dest string) (Result, error) {
	var out Result
	out.Dest = dest
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return out, fmt.Errorf("open skill zip: %w", err)
	}
	defer r.Close()

	prefix, err := detectPrefix(r.File)
	if err != nil {
		return out, err
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return out, fmt.Errorf("create skill dir %s: %w", dest, err)
	}

	existing := map[string]struct{}{}
	_ = filepath.Walk(dest, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, relErr := filepath.Rel(dest, p)
		if relErr == nil {
			existing[filepath.ToSlash(rel)] = struct{}{}
		}
		return nil
	})

	wrote := map[string]struct{}{}
	for _, f := range r.File {
		rel, ok := packageRel(f.Name, prefix)
		if !ok {
			continue
		}
		if !allowedPackagePath(rel) {
			out.Skipped = append(out.Skipped, rel)
			continue
		}
		if err := writeZipFile(f, filepath.Join(dest, filepath.FromSlash(rel))); err != nil {
			return out, err
		}
		out.Wrote = append(out.Wrote, rel)
		wrote[rel] = struct{}{}
	}
	if len(wrote) == 0 {
		return out, fmt.Errorf("zip has no SKILL.md or bin/ to install")
	}
	for rel := range existing {
		if _, ok := wrote[rel]; !ok {
			out.Preserved = append(out.Preserved, rel)
		}
	}
	return out, nil
}

func detectPrefix(files []*zip.File) (string, error) {
	for _, f := range files {
		name := strings.TrimPrefix(filepath.ToSlash(f.Name), "/")
		base := path.Base(name)
		if strings.EqualFold(base, "SKILL.md") && !strings.HasPrefix(name, "__MACOSX/") {
			dir := path.Dir(name)
			if dir == "." || dir == "" {
				return "", nil
			}
			if path.Base(dir) == "sshctl" {
				return dir + "/", nil
			}
			return dir + "/", nil
		}
	}
	return "", fmt.Errorf("zip missing SKILL.md")
}

func packageRel(name, prefix string) (string, bool) {
	name = strings.TrimPrefix(filepath.ToSlash(name), "/")
	if strings.HasPrefix(name, "__MACOSX/") || strings.HasSuffix(name, "/") {
		return "", false
	}
	if prefix != "" {
		if !strings.HasPrefix(name, prefix) {
			return "", false
		}
		name = strings.TrimPrefix(name, prefix)
	}
	if name == "" || strings.Contains(name, "..") {
		return "", false
	}
	return name, true
}

func allowedPackagePath(rel string) bool {
	rel = strings.ToLower(path.Clean(rel))
	base := path.Base(rel)
	if _, skip := userDataNames[base]; skip {
		return false
	}
	if rel == "skill.md" {
		return true
	}
	return strings.HasPrefix(rel, "bin/") && rel != "bin"
}

func writeZipFile(f *zip.File, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		return err
	}
	return out.Close()
}

// DefaultSkillDirs returns existing agent skill roots plus sshctl dest paths.
func DefaultSkillDirs(home string) []string {
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return nil
		}
	}
	var out []string
	for _, agent := range []string{".claude", ".cursor", ".codex"} {
		root := filepath.Join(home, agent, "skills")
		if _, err := os.Stat(root); err != nil {
			continue
		}
		out = append(out, filepath.Join(root, "sshctl"))
	}
	return out
}
