package core

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pkg/sftp"
)

func TestSFTPOnlyDirectoryTransfersAndLease(t *testing.T) {
	server := startSSH(t, "test")
	p := testProfile(server.address)
	p.Kind = "sftp"
	conn, e := Dial(context.Background(), p, Credentials{Password: "test", SavePassword: true}, testTrust(t), acceptHost)
	if e != nil {
		t.Fatal(e)
	}
	if !conn.PasswordVerified {
		t.Fatal("successful password not verified")
	}
	release, e := conn.Lease()
	if e != nil {
		t.Fatal(e)
	}
	defer release()
	client, e := sftp.NewClient(conn.Client)
	if e != nil {
		t.Fatal(e)
	}
	defer client.Close()
	conn.Close()
	local, remote, download := t.TempDir(), filepath.Join(t.TempDir(), "remote"), filepath.Join(t.TempDir(), "download")
	os.Mkdir(filepath.Join(local, "sub"), 0700)
	os.WriteFile(filepath.Join(local, "sub", "中文.txt"), []byte("file roundtrip"), 0600)
	if e = Upload(context.Background(), client, local, remote, nil); e != nil {
		t.Fatal(e)
	}
	if e = Upload(context.Background(), client, local, remote, nil); e == nil {
		t.Fatal("overwrote existing remote directory")
	}
	if e = Download(context.Background(), client, remote, download, false, nil); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(filepath.Join(download, "sub", "中文.txt"))
	if e != nil || string(b) != "file roundtrip" {
		t.Fatal(string(b), e)
	}
	if e = Download(context.Background(), client, remote, download, false, nil); e == nil {
		t.Fatal("merged an existing directory")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e = Upload(ctx, client, local, filepath.Join(t.TempDir(), "cancelled"), nil); e == nil {
		t.Fatal("ignored cancellation")
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(local, "link")
		os.Symlink(filepath.Join(local, "sub"), link)
		if e = Upload(context.Background(), client, link, filepath.Join(t.TempDir(), "link-copy"), nil); e == nil {
			t.Fatal("followed symlink")
		}
	}
}
func TestCrossPlatformFileNames(t *testing.T) {
	for _, bad := range []string{"..", ".. ", "../x", "/x", `..\x`, "x:y", "NUL.txt", "COM1", "a\x00b"} {
		if safeEntry(bad) {
			t.Fatalf("unsafe filename accepted: %q", bad)
		}
	}
	if !safeEntry("中文 file.txt") {
		t.Fatal("normal filename rejected")
	}
}
