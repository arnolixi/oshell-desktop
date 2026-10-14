package terminal

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestToolConnectionStatusChangesForSSHAndTelnetOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix fixture executables")
	}
	for _, name := range []string{"ssh", "telnet", "ping"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			file := filepath.Join(dir, name)
			if err := os.WriteFile(file, []byte("#!/bin/sh\n/bin/sleep 0.2\n"), 0700); err != nil {
				t.Fatal(err)
			}

			console := NewToolsConsole()
			defer console.Close()
			states := make(chan bool, 4)
			console.SetStateHandler(func() { states <- console.ConnectionActive() })
			go io.Copy(io.Discard, console)
			if _, err := console.Write([]byte("\"" + file + "\" 127.0.0.1\r")); err != nil {
				t.Fatal(err)
			}
			select {
			case state := <-states:
				if state != (name != "ping") {
					t.Fatal("wrong active status", state)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("missing process-start notification")
			}
			select {
			case state := <-states:
				if state {
					t.Fatal("status remained connected after exit")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("missing exit notification")
			}
		})
	}
}
