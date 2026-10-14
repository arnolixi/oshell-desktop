// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"context"
	"os"

	"github.com/pkg/sftp"
)

type RemoteFileSystem interface {
	RealPath(string) (string, error)
	ReadDir(string) ([]os.FileInfo, error)
	Lstat(string) (os.FileInfo, error)
	Mkdir(string) error
	Rename(string, string) error
	Remove(string) error
	RemoveDirectory(string) error
	Close() error
	Upload(context.Context, string, string, func(TransferProgress)) error
	Download(context.Context, string, string, bool, func(TransferProgress)) error
}
type SFTPFileSystem struct{ *sftp.Client }

func (s *SFTPFileSystem) Upload(ctx context.Context, local, remote string, p func(TransferProgress)) error {
	return Upload(ctx, s.Client, local, remote, p)
}
func (s *SFTPFileSystem) Download(ctx context.Context, remote, local string, replace bool, p func(TransferProgress)) error {
	return Download(ctx, s.Client, remote, local, replace, p)
}
