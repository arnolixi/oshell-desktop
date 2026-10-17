package testutil

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// LrzszPeer runs a real peer over byte-preserving pipes, including on Windows.
// Tests opt in by putting rz/sz on PATH; CI can require them explicitly.
func LrzszPeer(t *testing.T, dir, name string, args ...string) (net.Conn, func()) {
	t.Helper()
	return lrzszPeer(t, dir, name, nil, args...)
}

// LrzszShellPeer leaves the transport open and prints a prompt after the peer
// exits, like sz/rz running inside an existing SSH shell.
func LrzszShellPeer(t *testing.T, dir, name string, args ...string) (net.Conn, func()) {
	t.Helper()
	return lrzszPeer(t, dir, name, []byte("\r\nshell-ready$ "), args...)
}

// LrzszSessionPeer executes several real commands in one persistent shell,
// preserving its input/output streams across rz and sz invocations.
func LrzszSessionPeer(t *testing.T, dir, script string) (net.Conn, func()) {
	t.Helper()
	for _, name := range []string{"rz", "sz"} {
		if _, err := exec.LookPath(name); err != nil {
			if os.Getenv("OSHELL_REQUIRE_LRZSZ") == "1" {
				t.Fatal(err)
			}
			t.Skip("lrzsz not installed")
		}
	}
	shell := "sh"
	if runtime.GOOS == "windows" {
		git, err := exec.LookPath("git")
		if err != nil {
			t.Fatal(err)
		}
		shell = filepath.Join(filepath.Dir(filepath.Dir(git)), "usr", "bin", "bash.exe")
	}
	return lrzszPeer(t, dir, shell, nil, "-c", script)
}

func lrzszPeer(t *testing.T, dir, name string, tail []byte, args ...string) (net.Conn, func()) {
	t.Helper()
	executable, err := exec.LookPath(name)
	if err != nil {
		if os.Getenv("OSHELL_REQUIRE_LRZSZ") == "1" {
			t.Fatal(err)
		}
		t.Skip("lrzsz not installed")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	local, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { local.Close() })
	peer, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { peer.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, executable, args...)
	hidePeerWindow(cmd)
	cmd.Dir = dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { io.Copy(stdin, peer); stdin.Close() }()
	finished := make(chan struct{})
	var exitErr error
	go func() {
		io.Copy(peer, stdout)
		exitErr = cmd.Wait()
		if tail == nil {
			peer.Close()
		} else {
			peer.Write(tail)
		}
		close(finished)
	}()
	t.Cleanup(func() { cancel(); peer.Close(); <-finished })
	return local, func() {
		t.Helper()
		<-finished
		if exitErr != nil {
			t.Fatalf("%s: %v, %s", name, exitErr, stderr.String())
		}
	}
}
