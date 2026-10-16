//go:build !darwin && !windows

package core

import "os"

// Preserve the existing non-macOS implementation until platform acceptance.
func publishDownload(staged, destination string, directory, replace bool) error {
	if !directory && !replace {
		return os.Link(staged, destination)
	}
	return os.Rename(staged, destination)
}
