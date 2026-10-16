package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	marker := os.Getenv("OSHELL_SSH_EDIT_STARTED")
	if marker == "" || !strings.EqualFold(filepath.Base(os.Args[0]), "ssh.exe") {
		return
	}
	if err := os.WriteFile(marker, []byte("started"), 0600); err != nil {
		os.Exit(71)
	}
	time.Sleep(30 * time.Second)
	os.Exit(0)
}
