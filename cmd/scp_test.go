package cmd

import (
	"path/filepath"
	"testing"
)

func TestIsRemotePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "unix remote", path: "source:/srv/app.tar", want: true},
		{name: "windows drive", path: `C:\ArtifactCache\app.tar`, want: false},
		{name: "relative local", path: `ArtifactCache\app.tar`, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRemotePath(tt.path); got != tt.want {
				t.Fatalf("isRemotePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestValidateScpEndpointsSupportsRemoteFanOut(t *testing.T) {
	srcRemote, destinations, err := validateScpEndpoints(
		"source:/srv/app.tar",
		[]string{"target-a:/srv/app.tar", "target-b:/srv/app.tar"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !srcRemote || len(destinations) != 2 || !destinations[0] || !destinations[1] {
		t.Fatalf("unexpected classification: srcRemote=%v destinations=%v", srcRemote, destinations)
	}
}

func TestValidateScpEndpointsRejectsMixedFanOut(t *testing.T) {
	_, _, err := validateScpEndpoints(
		"source:/srv/app.tar",
		[]string{"target:/srv/app.tar", ".\\local-copy.tar"},
	)
	if err == nil {
		t.Fatal("expected mixed remote/local destinations to be rejected")
	}
}

func TestArtifactCacheRootFlagPrecedesEnvironment(t *testing.T) {
	original := scpArtifactCache
	t.Cleanup(func() { scpArtifactCache = original })

	envRoot := filepath.Join(t.TempDir(), "env-cache")
	flagRoot := filepath.Join(t.TempDir(), "flag-cache")
	t.Setenv("SSHCTL_ARTIFACT_CACHE", envRoot)
	scpArtifactCache = flagRoot

	got, err := artifactCacheRoot()
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(flagRoot)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("artifactCacheRoot() = %q, want %q", got, want)
	}
}

func TestArtifactCacheRootUsesEnvironment(t *testing.T) {
	original := scpArtifactCache
	t.Cleanup(func() { scpArtifactCache = original })

	envRoot := filepath.Join(t.TempDir(), "ArtifactCache")
	t.Setenv("SSHCTL_ARTIFACT_CACHE", envRoot)
	scpArtifactCache = ""

	got, err := artifactCacheRoot()
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.Abs(envRoot)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("artifactCacheRoot() = %q, want %q", got, want)
	}
}
