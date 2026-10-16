//go:build !windows

package core

import "os"

func replaceStateFile(staged, destination string) error {
	return os.Rename(staged, destination)
}
