package core

import (
	"github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"os"
	"testing"
	"time"
)

type attributesInfo struct {
	mode os.FileMode
	stat any
}

func (i attributesInfo) Name() string       { return "fixture" }
func (i attributesInfo) Size() int64        { return 10 }
func (i attributesInfo) Mode() os.FileMode  { return i.mode }
func (i attributesInfo) ModTime() time.Time { return time.Time{} }
func (i attributesInfo) IsDir() bool        { return i.mode.IsDir() }
func (i attributesInfo) Sys() any           { return i.stat }
func TestRemoteFilePermissionAndOwnership(t *testing.T) {
	for _, tc := range []struct {
		mode          os.FileMode
		symbol, octal string
	}{
		{os.ModeDir | 0755, "drwxr-xr-x", "0755"}, {0640, "-rw-r-----", "0640"},
		{0755 | os.ModeSetuid, "-rwsr-xr-x", "4755"}, {0640 | os.ModeSetgid, "-rw-r-S---", "2640"},
		{os.ModeDir | 0777 | os.ModeSticky, "drwxrwxrwt", "1777"},
	} {
		a := RemoteFileAttributes(attributesInfo{tc.mode, &sftp.FileStat{Mode: 0100000, UID: 1000, GID: 100}})
		if a.Permissions != tc.symbol || a.Octal != tc.octal || a.User != "1000" || a.Group != "100" {
			t.Fatalf("wrong metadata: %+v", a)
		}
	}
	root := RemoteFileAttributes(attributesInfo{0600, &sftp.FileStat{Mode: 0100600}})
	if root.User != "0" || root.Group != "0" {
		t.Fatal("zero UID/GID lost")
	}
	unknown := RemoteFileAttributes(attributesInfo{0644, nil})
	if unknown.User != "—" || unknown.Group != "—" {
		t.Fatal("invented ownership")
	}
}
func TestFTPDoesNotExposeSyntheticPermissions(t *testing.T) {
	a := RemoteFileAttributes(ftpInfo{entry: &ftp.Entry{Name: "fixture", Type: ftp.EntryTypeFile}})
	if a.Permissions != "—" || a.User != "—" || a.Group != "—" {
		t.Fatal("synthetic FTP defaults shown as remote attributes", a)
	}
}
