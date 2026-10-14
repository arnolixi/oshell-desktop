package zmodem

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func lrzszPeer(t *testing.T, dir, name string, args ...string) (net.Conn, func()) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("lrzsz fixture uses Unix stdio sockets")
	}
	executable, e := exec.LookPath(name)
	if e != nil {
		if os.Getenv("OSHELL_REQUIRE_LRZSZ") == "1" {
			t.Fatal(e)
		}
		t.Skip("lrzsz not installed")
	}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer listener.Close()
	local, e := net.Dial("tcp", listener.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { local.Close() })
	peer, e := listener.Accept()
	if e != nil {
		t.Fatal(e)
	}
	file, e := peer.(*net.TCPConn).File()
	peer.Close()
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = dir
	cmd.Stdin = file
	cmd.Stdout = file
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if e = cmd.Start(); e != nil {
		file.Close()
		t.Fatal(e)
	}
	file.Close()
	finished := make(chan error, 1)
	go func() { finished <- cmd.Wait() }()
	t.Cleanup(func() { cancel() })
	return local, func() {
		t.Helper()
		if e := <-finished; e != nil {
			t.Fatalf("%s: %v, %s", name, e, stderr.String())
		}
	}
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

type interopBuffer struct{ bytes.Buffer }

func (*interopBuffer) Close() error { return nil }
