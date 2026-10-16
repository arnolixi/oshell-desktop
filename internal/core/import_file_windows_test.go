package core

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsImportReaderAllowsAtomicPublication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := AtomicWrite(path, []byte("old snapshot")); err != nil {
		t.Fatal(err)
	}
	reader, err := openImportFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if err := AtomicWrite(path, []byte("new snapshot")); err != nil {
		t.Fatal(err)
	}
	old, err := io.ReadAll(reader)
	if err != nil || string(old) != "old snapshot" {
		t.Fatalf("reader changed: %q %v", old, err)
	}
	current, err := os.ReadFile(path)
	if err != nil || string(current) != "new snapshot" {
		t.Fatalf("publication lost: %q %v", current, err)
	}
}

func TestWindowsObstructedImportIsNotMissing(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "obstruction")
	if err := os.WriteFile(parent, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(parent, "config.json"), filepath.Join(parent, "logs", "sync.json")} {
		file, err := openImportFile(path)
		if file != nil {
			file.Close()
		}
		if err == nil || os.IsNotExist(err) {
			t.Fatalf("obstructed path treated as missing: %v", err)
		}
	}
	if _, err := openImportFile(filepath.Join(dir, "actually-missing", "config.json")); !os.IsNotExist(err) {
		t.Fatalf("genuinely absent path misclassified: %v", err)
	}
}
