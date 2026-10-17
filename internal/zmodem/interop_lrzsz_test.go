package zmodem

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"oshell-desktop/internal/zmodem/testutil"
	"path/filepath"
	"testing"
)

func lrzszPeer(t *testing.T, dir, name string, args ...string) (net.Conn, func()) {
	t.Helper()
	return testutil.LrzszPeer(t, dir, name, args...)
}
func TestLrzszBatchInterop(t *testing.T) {
	sizes := []int{0, 1, 6, 128, 4096, 65536, 12993548}
	for _, upload := range []bool{true, false} {
		t.Run(fmt.Sprintf("upload=%v", upload), func(t *testing.T) {
			dir := t.TempDir()
			data := map[string][]byte{}
			var files []SendFile
			var names []string
			for _, size := range sizes {
				name := fmt.Sprintf("file-%d.bin", size)
				b := make([]byte, size)
				for i := range b {
					b[i] = byte(i*37 + (i >> 3))
				}
				data[name] = b
				names = append(names, name)
				files = append(files, SendFile{Name: name, Size: int64(size), Reader: bytes.NewReader(b)})
				if !upload {
					if e := os.WriteFile(filepath.Join(dir, name), b, 0600); e != nil {
						t.Fatal(e)
					}
				}
			}
			if upload {
				conn, wait := lrzszPeer(t, dir, "rz", "-b", "-y")
				if e := SendFiles(conn, files, nil); e != nil {
					t.Fatal(e)
				}
				wait()
				for name, want := range data {
					got, e := os.ReadFile(filepath.Join(dir, name))
					if e != nil || !bytes.Equal(got, want) {
						t.Fatalf("upload %s: %v", name, e)
					}
				}
			} else {
				conn, wait := lrzszPeer(t, dir, "sz", append([]string{"-b", "--"}, names...)...)
				received := map[string]*interopBuffer{}
				e := ReceiveFiles(conn, func(file ReceiveFile) (io.WriteCloser, error) {
					b := &interopBuffer{}
					received[file.Name] = b
					return b, nil
				}, nil)
				if e != nil {
					t.Fatal(e)
				}
				wait()
				for name, want := range data {
					got := received[name]
					if got == nil || !bytes.Equal(got.Bytes(), want) {
						t.Fatalf("download %s differs", name)
					}
				}
			}
		})
	}
}

func TestLrzszDownloadModes(t *testing.T) {
	for _, args := range [][]string{{"-b"}, {"-b", "-e"}, {"-b", "-o"}, {"-b", "-w", "512"}} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			dir := t.TempDir()
			want := make([]byte, 65539)
			for i := range want {
				want[i] = byte(i)
			}
			if err := os.WriteFile(filepath.Join(dir, "data.bin"), want, 0600); err != nil {
				t.Fatal(err)
			}
			conn, wait := lrzszPeer(t, dir, "sz", append(args, "--", "data.bin")...)
			got := &interopBuffer{}
			if err := ReceiveFiles(conn, func(ReceiveFile) (io.WriteCloser, error) { return got, nil }, nil); err != nil {
				t.Fatal(err)
			}
			wait()
			if !bytes.Equal(want, got.Bytes()) {
				t.Fatal("download differs")
			}
		})
	}
}

type interopBuffer struct{ bytes.Buffer }

func (*interopBuffer) Close() error { return nil }
