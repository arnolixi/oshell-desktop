package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"oshell-desktop/internal/bundled"
)

// Run by the single-file build while its real embedded payload is available.
// Copy only the binary to an empty directory and start it with no adjacent
// library, no marker and a deliberately invalid external library override.
func TestSingleFileResourcesLoad(t *testing.T) {
	if !bundled.Enabled() {
		t.Skip("requires the single-file build's embedded payload")
	}
	if os.Getenv("OSHELL_EMBEDDED_CHILD") != "1" {
		dir := t.TempDir()
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(executable)
		if err != nil {
			t.Fatal(err)
		}
		copy := filepath.Join(dir, "独立终端 测试.exe")
		if err := os.WriteFile(copy, data, 0700); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(copy, "-test.run=^TestSingleFileResourcesLoad$", "-test.timeout=30s")
		cmd.Dir = dir
		// The parent removes this cache only after the child has exited and
		// Windows has released its loaded DLL.
		cmd.Env = append(os.Environ(), "OSHELL_EMBEDDED_CHILD=1", "LOCALAPPDATA="+t.TempDir())
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("standalone process failed: %v\n%s", err, out)
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 1 {
			t.Fatal("resources escaped cache into executable directory", entries, err)
		}
		return
	}
	t.Setenv("MYGO_GHOSTTY_VT", "invalid-external-library.dll")
	lib, err := LibraryPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(lib, os.Getenv("LOCALAPPDATA")+string(os.PathSeparator)) {
		t.Fatal("library did not use cache", lib)
	}
	for _, name := range []string{"LICENSE", "THIRD_PARTY_NOTICES.md", "licenses/ghostty-LICENSE"} {
		if _, err := bundled.Path(name); err != nil {
			t.Fatal(err)
		}
	}
	term, err := New(Options{Conn: newPipe(), NoBlink: true})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	term.write([]byte("embedded-terminal-ready"))
	if !strings.Contains(term.Text(), "embedded-terminal-ready") {
		t.Fatal("embedded engine failed to parse terminal output")
	}
}
