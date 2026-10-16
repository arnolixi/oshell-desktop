package core

import (
	"golang.org/x/sys/windows"
	"os"
)

// Move within the destination volume, without hard links (FAT/exFAT have none).
// Omitting REPLACE_EXISTING makes the absence check atomic with publication.
func publishDownload(staged, destination string, directory, replace bool) error {
	from, err := windows.UTF16PtrFromString(staged)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	flags := uint32(windows.MOVEFILE_WRITE_THROUGH)
	if replace && !directory {
		flags |= windows.MOVEFILE_REPLACE_EXISTING
	}
	if err := windows.MoveFileEx(from, to, flags); err != nil {
		return &os.LinkError{Op: "publish-download", Old: staged, New: destination, Err: err}
	}
	return nil
}
