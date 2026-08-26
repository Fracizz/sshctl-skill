package sshx

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func TestValidateDeleteTargetRejectsUnsafeRoots(t *testing.T) {
	for _, path := range []string{"", ".", "..", "../tmp", "/", "///", "/home", "~", "~/data", "C:/"} {
		if err := validateDeleteTarget(path); err == nil {
			t.Fatalf("validateDeleteTarget(%q) accepted unsafe path", path)
		}
	}
}

func TestValidateDeleteTargetAcceptsScopedDirectory(t *testing.T) {
	for _, path := range []string{"/srv/app-store", "tmp/stage", "C:/staging"} {
		if err := validateDeleteTarget(path); err != nil {
			t.Fatalf("validateDeleteTarget(%q): %v", path, err)
		}
	}
}

func TestRemoteJoinUsesSlashSeparators(t *testing.T) {
	if got := remoteJoin("/srv/app/", "/nested/file.txt"); got != "/srv/app/nested/file.txt" {
		t.Fatalf("remoteJoin() = %q", got)
	}
}

func TestSyncUploadIncrementalChecksumAndDelete(t *testing.T) {
	client := newInMemorySFTPClient(t)
	defer client.Close()

	local := t.TempDir()
	if err := os.WriteFile(filepath.Join(local, "one.txt"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(local, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "nested", "two.txt"), []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}

	const remoteRoot = "/tmp/sync"
	first, err := SyncUpload(client, local, remoteRoot, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if first.Uploaded != 2 || first.BytesTransferred != 6 {
		t.Fatalf("first sync = %+v", first)
	}

	second, err := SyncUpload(client, local, remoteRoot, SyncOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if second.Uploaded != 0 || second.Skipped != 2 {
		t.Fatalf("second sync = %+v", second)
	}

	// Keep the same size and timestamp; --checksum must still detect changed content.
	one := filepath.Join(local, "one.txt")
	before, err := os.Stat(one)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(one, []byte("ONE"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(one, before.ModTime(), before.ModTime()); err != nil {
		t.Fatal(err)
	}
	third, err := SyncUpload(client, local, remoteRoot, SyncOptions{Checksum: true})
	if err != nil {
		t.Fatal(err)
	}
	if third.Uploaded != 1 || third.Skipped != 1 {
		t.Fatalf("checksum sync = %+v", third)
	}

	remote, err := sftp.NewClient(client)
	if err != nil {
		t.Fatal(err)
	}
	extra, err := remote.Create(remoteRoot + "/extra.txt")
	if err != nil {
		remote.Close()
		t.Fatal(err)
	}
	if _, err := extra.Write([]byte("extra")); err != nil {
		extra.Close()
		remote.Close()
		t.Fatal(err)
	}
	if err := extra.Close(); err != nil {
		remote.Close()
		t.Fatal(err)
	}
	remote.Close()

	dryRun, err := SyncUpload(client, local, remoteRoot, SyncOptions{Delete: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if dryRun.Deleted != 1 {
		t.Fatalf("dry-run sync = %+v", dryRun)
	}
	actual, err := SyncUpload(client, local, remoteRoot, SyncOptions{Delete: true})
	if err != nil {
		t.Fatal(err)
	}
	if actual.Deleted != 1 {
		t.Fatalf("delete sync = %+v", actual)
	}
}

func newInMemorySFTPClient(t *testing.T) *ssh.Client {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	serverConfig := &ssh.ServerConfig{NoClientAuth: true}
	serverConfig.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	handlers := sftp.InMemHandler()
	go func() {
		serverConn, err := listener.Accept()
		if err != nil {
			return
		}
		_, channels, requests, err := ssh.NewServerConn(serverConn, serverConfig)
		if err != nil {
			return
		}
		go ssh.DiscardRequests(requests)
		for newChannel := range channels {
			if newChannel.ChannelType() != "session" {
				_ = newChannel.Reject(ssh.UnknownChannelType, "session required")
				continue
			}
			channel, channelRequests, err := newChannel.Accept()
			if err != nil {
				return
			}
			go func() {
				for request := range channelRequests {
					ok := request.Type == "subsystem" && len(request.Payload) >= 4 && string(request.Payload[4:]) == "sftp"
					_ = request.Reply(ok, nil)
				}
			}()
			go func() {
				server := sftp.NewRequestServer(channel, handlers)
				if err := server.Serve(); err != nil && err != io.EOF {
					_ = channel.Close()
				}
				_ = server.Close()
			}()
		}
	}()

	clientConfig := &ssh.ClientConfig{
		User:            "test",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	client, err := ssh.Dial("tcp", listener.Addr().String(), clientConfig)
	if err != nil {
		t.Fatal(err)
	}
	return client
}
