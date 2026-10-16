package core

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestHostProbeCommandReportsWithoutFiles(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("remote POSIX probe execution requires a POSIX shell; parser and token checks run on every platform")
	}
	command, e := HostProbeCommand("OSHELL_INFO_test:")
	if e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(shell, "-c", command)
	cmd.Env = append(os.Environ(), "SSH_CONNECTION=192.0.2.1 44000 198.51.100.67 22")
	out, e := cmd.Output()
	if e != nil {
		t.Fatal(e)
	}
	prefix := "\x1b]2;OSHELL_INFO_test:"
	if !strings.HasPrefix(string(out), prefix) || !strings.HasSuffix(string(out), "\x07") {
		t.Fatalf("bad frame %q", out)
	}
	identity, ok := ParseHostIdentity(strings.TrimSuffix(strings.TrimPrefix(string(out), prefix), "\x07"))
	if !ok || identity.Address != "198.51.100.67" {
		t.Fatalf("invalid identity %+v (%v), probe output %q", identity, ok, out)
	}
}
func TestHostProbeRejectsUnsafeToken(t *testing.T) {
	if _, e := HostProbeCommand("';touch bad;"); e == nil {
		t.Fatal("unsafe token")
	}
}
func TestHostProbeEchoFragmentation(t *testing.T) {
	command := " sh -c 'printf test'"
	echo := command + "\r\nprompt> "
	for split := 0; split <= len(echo); split++ {
		parser := NewHostProbeEcho(command)
		out := append(parser.Consume([]byte(echo[:split])), parser.Consume([]byte(echo[split:]))...)
		if string(out) != "\r\x1b[2Kprompt> " {
			t.Fatalf("split %d: %q", split, out)
		}
	}
	for _, unknown := range []string{"server output\r\n", "\x1b[?1049h", command[:8]} {
		parser := NewHostProbeEcho(command)
		out := parser.Consume([]byte(unknown))
		out = append(out, parser.Flush()...)
		if string(out) != unknown {
			t.Fatalf("swallowed output %q -> %q", unknown, out)
		}
	}
	if !IsAuthenticationPrompt("Password: ") || IsAuthenticationPrompt("user@host:~$ ") {
		t.Fatal("authentication guard")
	}
}
