package core

import (
	"os"
	"path/filepath"
)

// AtomicWrite stages beside the destination. Root.Rename uses Windows POSIX
// rename semantics where supported, so open readers retain the old snapshot
// without preventing publication. MoveFileEx (os.Rename) cannot do this.
func replaceStateFile(staged, destination string) error {
	root, err := os.OpenRoot(filepath.Dir(destination))
	if err != nil {
		return err
	}
	defer root.Close()
	return root.Rename(filepath.Base(staged), filepath.Base(destination))
}
