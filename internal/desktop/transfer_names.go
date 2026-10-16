package desktop

import (
	"context"
	"fmt"
	"os"
	"oshell-desktop/internal/core"
	"path"
	"path/filepath"
)

func uniqueTransferName(name string, existing map[string]bool) string {
	next := name
	for index := 1; existing[next]; index++ {
		next = fmt.Sprintf("%s.%d", name, index)
	}
	existing[next] = true
	return next
}
func uploadUnique(ctx context.Context, client core.RemoteFileSystem, paths []string, directory string, progress func(core.TransferProgress)) error {
	entries, err := client.ReadDir(directory)
	if err != nil {
		return err
	}
	names := map[string]bool{}
	for _, entry := range entries {
		names[entry.Name()] = true
	}
	for _, source := range paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := filepath.Base(source)
		if !core.SafeTransferName(name) {
			return fmt.Errorf("文件名包含不支持的字符")
		}
		target := path.Join(directory, uniqueTransferName(name, names))
		if err := client.Upload(ctx, source, target, progress); err != nil {
			return err
		}
	}
	return nil
}
func downloadUnique(ctx context.Context, client core.RemoteFileSystem, remote, directory string, progress func(core.TransferProgress)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name := path.Base(remote)
	if !core.SafeTransferName(name) {
		return fmt.Errorf("远程文件名无效")
	}
	target, err := uniqueLocalTransferPath(directory, name)
	if err != nil {
		return err
	}
	return client.Download(ctx, remote, target, false, progress)
}

// Ask the destination filesystem: it may be case-insensitive, or a Windows
// directory explicitly configured as case-sensitive. Remote names are untouched.
func uniqueLocalTransferPath(directory, name string) (string, error) {
	for index := 0; ; index++ {
		next := name
		if index > 0 {
			next = fmt.Sprintf("%s.%d", name, index)
		}
		target := filepath.Join(directory, next)
		if _, err := os.Lstat(target); os.IsNotExist(err) {
			return target, nil
		} else if err != nil {
			return "", err
		}
	}
}
