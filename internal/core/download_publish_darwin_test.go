//go:build darwin

package core

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestDownloadPublicationNeverReplacesExistingDestinations(t *testing.T) {
	for _, directory := range []bool{false, true} {
		for _, existing := range []string{"file", "directory", "symlink"} {
			t.Run(map[bool]string{false: "file", true: "directory"}[directory]+"/"+existing, func(t *testing.T) {
				root := t.TempDir()
				staged, dest := filepath.Join(root, "staged"), filepath.Join(root, "destination")
				if directory {
					if err := os.Mkdir(staged, 0700); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.WriteFile(staged, []byte("new-download"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				var err error
				switch existing {
				case "file":
					err = os.WriteFile(dest, []byte("existing-file"), 0600)
				case "directory":
					err = os.Mkdir(dest, 0700)
				case "symlink":
					err = os.Symlink("unrelated-target", dest)
				}
				if err != nil {
					t.Fatal(err)
				}
				before, err := os.Lstat(dest)
				if err != nil {
					t.Fatal(err)
				}
				err = publishDownload(staged, dest, directory, false)
				if !errors.Is(err, os.ErrExist) {
					t.Fatal("expected atomic collision", err)
				}
				after, err := os.Lstat(dest)
				if err != nil || !os.SameFile(before, after) {
					t.Fatal("existing destination changed", err)
				}
				if _, err = os.Lstat(staged); err != nil {
					t.Fatal("rejected staged item lost", err)
				}
			})
		}
	}
}

func TestConcurrentDirectoryDownloadsPublishOnlyOneWinner(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "destination")
	var wins atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		stage, err := os.MkdirTemp(root, "stage-")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(stage, "payload"), []byte(stage), 0600); err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			err := publishDownload(stage, dest, true, false)
			if err == nil {
				wins.Add(1)
			} else if !errors.Is(err, os.ErrExist) {
				t.Error(err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatal("multiple directory downloads replaced each other", wins.Load())
	}
	entries, err := os.ReadDir(dest)
	if err != nil || len(entries) != 1 {
		t.Fatal("published directory incomplete", err)
	}
}

func TestExplicitRegularFileReplacementStillWorks(t *testing.T) {
	root := t.TempDir()
	stage, dest := filepath.Join(root, "stage"), filepath.Join(root, "dest")
	if err := os.WriteFile(stage, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := publishDownload(stage, dest, false, true); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "replacement" {
		t.Fatal("explicit replacement failed", err)
	}
}
