package desktop

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"oshell-desktop/internal/core"
	"oshell-desktop/internal/terminal"
	"oshell-desktop/internal/zmodem"
	"oshell-desktop/internal/zmodem/testutil"
)

type fragmentedZmodemConn struct {
	net.Conn
	mu            sync.Mutex
	read, written []byte
}

func (c *fragmentedZmodemConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b[:min(len(b), 7)])
	c.mu.Lock()
	c.read = append(c.read, b[:n]...)
	if len(c.read) > 512 {
		c.read = c.read[len(c.read)-512:]
	}
	c.mu.Unlock()
	return n, err
}
func (c *fragmentedZmodemConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	c.written = append(c.written, b...)
	if len(c.written) > 512 {
		c.written = c.written[len(c.written)-512:]
	}
	c.mu.Unlock()
	return c.Conn.Write(b)
}

func TestTerminalRealSzDownloadsToWindowsDirectory(t *testing.T) {
	for _, mode := range []string{"default", "escape", "crc16", "window", "collision"} {
		t.Run(mode, func(t *testing.T) {
			source, destination := t.TempDir(), t.TempDir()
			payload := make([]byte, 65539)
			for i := range payload {
				payload[i] = byte(i)
			}
			names := []string{"中文 空格.bin", "empty.txt"}
			var secondPayload []byte
			// The Windows lrzsz 0.12.21rc port crashes on an empty file with
			// -o. Cover CRC16 here with a one-byte file; default mode covers
			// real empty files, and protocol fixtures cover empty CRC16 files.
			if runtime.GOOS == "windows" && mode == "crc16" {
				secondPayload = []byte{0}
			}
			for i, name := range names {
				data := payload
				if i == 1 {
					data = secondPayload
				}
				if err := os.WriteFile(filepath.Join(source, name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"-b"}
			switch mode {
			case "escape":
				args = append(args, "-e")
			case "crc16":
				args = append(args, "-o")
			case "window":
				args = append(args, "-w", "512")
			case "collision":
				if err := os.WriteFile(filepath.Join(destination, names[0]), []byte("existing"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			args = append(append(args, "--"), names...)
			conn, wait := testutil.LrzszShellPeer(t, source, "sz", args...)
			a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
			p := &pane{}
			p.owner.Store(a)
			fragmented := &fragmentedZmodemConn{Conn: conn}
			opts := terminal.Options{Conn: fragmented, NoBlink: true}
			a.configureTerminalTransferWithPicker(p, &opts, true, func(ctx context.Context, _ *mygo.Window, upload bool) ([]string, error) {
				if upload {
					return nil, fmt.Errorf("sz was detected as upload")
				}
				select {
				case <-time.After(100 * time.Millisecond):
					return []string{destination}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			})
			term, err := terminal.New(opts)
			if err != nil {
				t.Fatal(err)
			}
			p.term = term
			defer term.Close()
			defer func() {
				if t.Failed() {
					fragmented.mu.Lock()
					defer fragmented.mu.Unlock()
					t.Logf("transfer=%+v terminal=%q read=%x written=%x", p.transfer, term.Text(), fragmented.read, fragmented.written)
				}
			}()
			waitOperator(t, a, func() bool { return p.transfer.visible && !p.transfer.active })
			if mode == "collision" {
				if !strings.Contains(p.transfer.status, "跳过") {
					t.Fatal(p.transfer.status)
				}
			} else if p.transfer.status != "ZMODEM · 传输完成" {
				t.Fatal(p.transfer.status)
			}
			wait()
			waitOperator(t, a, func() bool { return term.Available() && strings.Contains(term.Text(), "shell-ready$") })
			for i, name := range names {
				want := payload
				if i == 1 {
					want = secondPayload
				} else if mode == "collision" {
					want = []byte("existing")
				}
				got, err := os.ReadFile(filepath.Join(destination, name))
				if err != nil || !bytes.Equal(got, want) {
					t.Fatalf("%s corrupt/missing: %v", name, err)
				}
			}
			entries, err := os.ReadDir(destination)
			if err != nil || len(entries) != len(names) {
				t.Fatalf("partial downloads remain: %v %v", entries, err)
			}
		})
	}
}

func TestTerminalRealRzBatchUpload(t *testing.T) {
	for _, escape := range []bool{false, true} {
		t.Run(fmt.Sprint(escape), func(t *testing.T) {
			source, destination := t.TempDir(), t.TempDir()
			payload := bytes.Repeat([]byte{0, 1, 0x11, 0x18, 0xff, 'x'}, 22000)
			var paths []string
			for _, name := range []string{"中文 空格.bin", "second.bin"} {
				path := filepath.Join(source, name)
				if err := os.WriteFile(path, payload, 0600); err != nil {
					t.Fatal(err)
				}
				paths = append(paths, path)
			}
			args := []string{"-b", "-y"}
			if escape {
				args = append(args, "-e")
			}
			conn, wait := testutil.LrzszShellPeer(t, destination, "rz", args...)
			a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
			p := &pane{}
			p.owner.Store(a)
			opts := terminal.Options{Conn: &fragmentedZmodemConn{Conn: conn}, NoBlink: true}
			a.configureTerminalTransferWithPicker(p, &opts, true, func(context.Context, *mygo.Window, bool) ([]string, error) { return paths, nil })
			term, err := terminal.New(opts)
			if err != nil {
				t.Fatal(err)
			}
			p.term = term
			defer term.Close()
			waitOperator(t, a, func() bool { return p.transfer.visible && !p.transfer.active })
			if p.transfer.status != "ZMODEM · 传输完成" {
				t.Fatal(p.transfer.status)
			}
			wait()
			waitOperator(t, a, func() bool { return term.Available() && strings.Contains(term.Text(), "shell-ready$") })
			for _, path := range paths {
				got, err := os.ReadFile(filepath.Join(destination, filepath.Base(path)))
				if err != nil || !bytes.Equal(got, payload) {
					t.Fatalf("upload corrupted: %v", err)
				}
			}
		})
	}
}

func TestTerminalRealSzCancelWhileChoosingDirectory(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "data.bin"), []byte("cancelled"), 0600); err != nil {
		t.Fatal(err)
	}
	conn, _ := testutil.LrzszShellPeer(t, source, "sz", "-b", "--", "data.bin")
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p := &pane{}
	p.owner.Store(a)
	opts := terminal.Options{Conn: conn, NoBlink: true}
	choosing := make(chan struct{})
	a.configureTerminalTransferWithPicker(p, &opts, true, func(ctx context.Context, _ *mygo.Window, _ bool) ([]string, error) {
		close(choosing)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	term, err := terminal.New(opts)
	if err != nil {
		t.Fatal(err)
	}
	p.term = term
	defer term.Close()
	waitOperator(t, a, func() bool {
		select {
		case <-choosing:
			return true
		default:
			return false
		}
	})
	term.CancelTransfer()
	waitOperator(t, a, func() bool { return !p.transfer.active && term.Available() })
	if p.transfer.status != "ZMODEM · 已取消" {
		t.Fatal(p.transfer.status)
	}
	waitOperator(t, a, func() bool { return strings.Contains(term.Text(), "shell-ready$") })
}

type slowZmodemConn struct{ net.Conn }

func (c slowZmodemConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p[:min(len(p), 2048)])
	if n > 0 {
		time.Sleep(time.Millisecond)
	}
	return n, err
}

func TestTerminalCancelDownloadRemovesPartialFile(t *testing.T) {
	local, remote := net.Pipe()
	defer remote.Close()
	destination := t.TempDir()
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p := &pane{}
	p.owner.Store(a)
	opts := terminal.Options{Conn: slowZmodemConn{local}, NoBlink: true}
	a.configureTerminalTransferWithPicker(p, &opts, true, func(context.Context, *mygo.Window, bool) ([]string, error) { return []string{destination}, nil })
	term, err := terminal.New(opts)
	if err != nil {
		t.Fatal(err)
	}
	p.term = term
	defer term.Close()
	payload := bytes.Repeat([]byte("payload"), 1<<19)
	done := make(chan error, 1)
	go func() {
		done <- zmodem.SendFiles(remote, []zmodem.SendFile{{Name: "partial.bin", Size: int64(len(payload)), Reader: bytes.NewReader(payload)}}, nil)
	}()
	waitOperator(t, a, func() bool { return strings.Contains(p.transfer.status, " / ") })
	term.CancelTransfer()
	waitOperator(t, a, func() bool { return !p.transfer.active && term.Available() })
	if p.transfer.status != "ZMODEM · 已取消" {
		t.Fatal(p.transfer.status)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("sender reported cancelled transfer as successful")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("sender did not stop")
	}
	entries, err := os.ReadDir(destination)
	if err != nil || len(entries) != 0 {
		t.Fatalf("partial file survived cancellation: %v %v", entries, err)
	}
}

func TestTerminalRealRzThenSzSameSession(t *testing.T) {
	for _, immediate := range []bool{false, true} {
		t.Run(fmt.Sprintf("immediate=%v", immediate), func(t *testing.T) { testTerminalRealRzThenSzSameSession(t, immediate) })
	}
}

func testTerminalRealRzThenSzSameSession(t *testing.T, immediate bool) {
	localDir, remoteDir := t.TempDir(), t.TempDir()
	name := "roundtrip.bin"
	payload := bytes.Repeat([]byte{0, 0x18, 0x11, 0xff, 'a'}, 14000)
	path := filepath.Join(localDir, name)
	if err := os.WriteFile(path, payload, 0600); err != nil {
		t.Fatal(err)
	}
	downloads := []string{t.TempDir(), t.TempDir(), t.TempDir()}
	script := `
for i in 1 2 3; do
  rz -b -y || exit 1
  printf '\r\nupload-ready-%s$ ' "$i"
  IFS= read -r next || exit 2
  sz -b -- roundtrip.bin || exit 3
  printf '\r\ndownload-ready-%s$ ' "$i"
  IFS= read -r next || exit 4
done`
	if immediate {
		script = strings.Replace(script, "  IFS= read -r next || exit 2\n", "", 1)
	}
	conn, wait := testutil.LrzszSessionPeer(t, remoteDir, script)
	a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
	p := &pane{}
	p.owner.Store(a)
	fragmented := &fragmentedZmodemConn{Conn: conn}
	opts := terminal.Options{Conn: fragmented, NoBlink: true}
	var downloadIndex atomic.Int32
	directions := make(chan bool, 10)
	a.configureTerminalTransferWithPicker(p, &opts, true, func(ctx context.Context, _ *mygo.Window, upload bool) ([]string, error) {
		directions <- upload
		if upload {
			return []string{path}, nil
		}
		i := int(downloadIndex.Add(1)) - 1
		if i >= len(downloads) {
			return nil, fmt.Errorf("unexpected extra download")
		}
		select {
		case <-time.After(50 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return []string{downloads[i]}, nil
	})
	done := make(chan error, 10)
	finished := opts.TransferFinished
	opts.TransferFinished = func(err error) { finished(err); done <- err }
	term, err := terminal.New(opts)
	if err != nil {
		t.Fatal(err)
	}
	p.term = term
	defer term.Close()
	defer func() {
		if t.Failed() {
			fragmented.mu.Lock()
			defer fragmented.mu.Unlock()
			t.Logf("status=%+v text=%q read=%x written=%x", p.transfer, term.Text(), fragmented.read, fragmented.written)
		}
	}()
	for round := 0; round < 3; round++ {
		for _, upload := range []bool{true, false} {
			completed := false
			waitOperator(t, a, func() bool {
				select {
				case err := <-done:
					if err != nil {
						t.Fatal(err)
					}
					completed = true
				default:
				}
				return completed
			})
			a.drain()
			select {
			case got := <-directions:
				if got != upload {
					t.Fatalf("direction %v want %v", got, upload)
				}
			default:
				t.Fatal("missing picker")
			}
			if !(immediate && upload) && (p.transfer.active || p.transfer.status != "ZMODEM · 传输完成") {
				t.Fatal(p.transfer)
			}
			prefix := "download"
			dir := downloads[round]
			if upload {
				prefix, dir = "upload", remoteDir
			}
			got, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil || !bytes.Equal(got, payload) {
				t.Fatalf("round %d %s data: %v", round, prefix, err)
			}
			prompt := fmt.Sprintf("%s-ready-%d$", prefix, round+1)
			if immediate && upload {
				continue
			}
			waitOperator(t, a, func() bool { return term.Available() && strings.Contains(term.Text(), prompt) })
			// This fixture uses pipes, so shell read needs LF instead of the
			// CR that interactive managed input sends to a remote PTY.
			term.Send([]byte("next\n"))
		}
	}
	wait()
}
