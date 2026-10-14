//go:build darwin

package core

import (
	"golang.org/x/sys/unix"
	"os"
)

// publishDownload atomically installs a staged file or directory. RENAME_EXCL
// checks absence in the filesystem operation, closing the preflight/rename race.
func publishDownload(staged, destination string, directory, replace bool) error {
	if replace && !directory {
		return os.Rename(staged, destination)
	}
	if err := unix.RenamexNp(staged, destination, unix.RENAME_EXCL); err != nil {
		return &os.LinkError{Op: "rename-no-replace", Old: staged, New: destination, Err: err}
	}
	return nil
}
