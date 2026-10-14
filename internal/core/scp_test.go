package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSCPTransfersUseAuthenticatedChannel(t *testing.T) {
	if _, e := exec.LookPath("scp"); e != nil {
		t.Skip("requires OpenSSH scp as the test server")
	}
	server := startSSH(t, "test")
	p := testProfile(server.address)
	connection, e := Dial(context.Background(), p, Credentials{Password: "test"}, testTrust(t), acceptHost)
	if e != nil {
		t.Fatal(e)
	}
	defer connection.Close()
	source := filepath.Join(t.TempDir(), "a 'quoted' file.txt")
	os.WriteFile(source, []byte("SCP channel data\n中文"), 0600)
	remote := filepath.ToSlash(filepath.Join(t.TempDir(), "remote file.txt"))
	if e = connection.SCPUpload(context.Background(), source, remote, nil); e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(t.TempDir(), "download.txt")
	if e = connection.SCPDownload(context.Background(), remote, target, false, nil); e != nil {
		t.Fatal(e)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "SCP channel data\n中文" {
		t.Fatal(string(got))
	}
	if server.auth.Load() != 1 {
		t.Fatal("SCP reauthenticated")
	}
	if _, ok := scpTestArgs("scp -rf -- 'x'; touch /tmp/invalid"); ok {
		t.Fatal("accepted shell injection")
	}
}
