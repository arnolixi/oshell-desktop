package core

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestProxyCommandQuotesIdentityAndClosesProcess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX quoting; Windows validates metacharacters separately")
	}
	p := NewProfile()
	p.Host = "host'with\"quotes"
	p.Username = "$(echo must-not-run)`literal`"
	p.Port = 2222
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, e := dialProxyCommand(ctx, `printf '%%s|%%s|%%s' '%h' "%r" %p`, p)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	got, e := io.ReadAll(c)
	if e != nil {
		t.Fatal(e)
	}
	want := p.Host + "|" + p.Username + "|2222"
	if string(got) != want {
		t.Fatalf("%q != %q", got, want)
	}
	c, e = dialProxyCommand(ctx, "cat", p)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Write([]byte("roundtrip\n")); e != nil {
		t.Fatal(e)
	}
	b := make([]byte, 10)
	if _, e = io.ReadFull(c, b); e != nil || string(b) != "roundtrip\n" {
		t.Fatal(string(b), e)
	}
	c.Close()
	select {
	case <-c.(*commandConn).done:
	case <-time.After(time.Second):
		t.Fatal("process not reaped")
	}
}
func TestProxyCommandRejectsUnsupportedExpansion(t *testing.T) {
	p := NewProfile()
	p.Host = "valid.example"
	p.Username = "valid"
	if runtime.GOOS == "windows" {
		p.Host = "bad&command"
		if _, e := expandProxyCommand("nc %h %p", p); e == nil {
			t.Fatal("accepted shell metacharacter")
		}
		return
	}
	if _, e := expandProxyCommand("nc %x %p", p); e == nil {
		t.Fatal("unknown token")
	}
}
func TestSCPRecursiveTransfers(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("OpenSSH Windows SCP server path semantics need manual acceptance")
	}
	if _, e := exec.LookPath("scp"); e != nil {
		t.Skip("OpenSSH scp unavailable")
	}
	server := startSSH(t, "test")
	p := testProfile(server.address)
	conn, e := Dial(context.Background(), p, Credentials{Password: "test"}, testTrust(t), acceptHost)
	if e != nil {
		t.Fatal(e)
	}
	defer conn.Close()
	source := filepath.Join(t.TempDir(), "tree")
	os.MkdirAll(filepath.Join(source, "sub"), 0700)
	os.WriteFile(filepath.Join(source, "sub", "中文.txt"), []byte(strings.Repeat("payload", 10000)), 0600)
	remote := filepath.Join(t.TempDir(), "remote-tree")
	if e = conn.SCPUpload(context.Background(), source, remote, nil); e != nil {
		t.Fatal(e)
	}
	dest := filepath.Join(t.TempDir(), "download")
	if e = conn.SCPDownload(context.Background(), remote, dest, false, nil); e != nil {
		t.Fatal(e)
	}
	got, e := os.ReadFile(filepath.Join(dest, "sub", "中文.txt"))
	if e != nil || string(got) != strings.Repeat("payload", 10000) {
		t.Fatal("recursive content differs", e)
	}
}
