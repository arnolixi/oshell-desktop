package bundled

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func testArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestConcurrentExtractionRepairsCorruptionAndReusesCache(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "版本 缓存")
	data := testArchive(t, map[string]string{"ghostty-vt.dll": "library", "licenses/LICENSE": "license"})
	var workers sync.WaitGroup
	for i := 0; i < 12; i++ {
		workers.Go(func() {
			if err := extract(data, dir); err != nil {
				t.Error(err)
			}
		})
	}
	workers.Wait()
	dll := filepath.Join(dir, "ghostty-vt.dll")
	before, err := os.Stat(dll)
	if err != nil {
		t.Fatal(err)
	}
	if err := extract(data, dir); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(dll)
	if err != nil || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("valid cache was rewritten", err)
	}
	if err := os.WriteFile(dll, []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := extract(data, dir); err != nil {
		t.Fatal(err)
	}
	if !matches(dll, []byte("library")) {
		t.Fatal("corruption not repaired")
	}
	if err := os.Remove(filepath.Join(dir, "licenses/LICENSE")); err != nil {
		t.Fatal(err)
	}
	if err := extract(data, dir); err != nil {
		t.Fatal(err)
	}
	if !matches(filepath.Join(dir, "licenses/LICENSE"), []byte("license")) {
		t.Fatal("missing resource not restored")
	}
}

func TestExtractionRejectsUnsafePaths(t *testing.T) {
	for _, name := range []string{"../escape", "/escape", "C:/escape", "a\\escape", "a/../escape", "file:stream"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if err := extract(testArchive(t, map[string]string{name: "bad"}), filepath.Join(root, "cache")); err == nil {
				t.Fatal("unsafe archive accepted")
			}
			if entries, _ := os.ReadDir(root); len(entries) != 0 {
				t.Fatal("invalid archive wrote files")
			}
		})
	}
}

func TestExtractionRejectsFileDirectoryConflict(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "ghostty-vt.dll"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := extract(testArchive(t, map[string]string{"ghostty-vt.dll": "library"}), dir); err == nil {
		t.Fatal("directory replaced by resource")
	}
}
