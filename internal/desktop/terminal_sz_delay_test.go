package desktop

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"oshell-desktop/internal/core"
	"oshell-desktop/internal/terminal"
	"oshell-desktop/internal/zmodem/testutil"
)

func TestTerminalRealSzDelayedDirectorySelection(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		pickerDelay, latency time.Duration
		flow                 bool
	}{
		{name: "immediate"},
		{name: "slow-picker", pickerDelay: 12 * time.Second},
		{name: "network-delay", latency: 10 * time.Millisecond},
		{name: "flow-control", latency: time.Millisecond, flow: true},
	} {
		delay := tc.pickerDelay
		t.Run(tc.name, func(t *testing.T) {
			source, destination := t.TempDir(), t.TempDir()
			payload := bytes.Repeat([]byte{0, 0x18, 0x11, 0xff, 'a'}, 200000)
			if err := os.WriteFile(filepath.Join(source, "download.bin"), payload, 0600); err != nil {
				t.Fatal(err)
			}
			conn, wait := testutil.LrzszSessionPeer(t, source, "sz -- download.bin 2>&1; printf '\r\nshell-ready$ '; read -r next")
			a := newApp(&core.Store{Dir: t.TempDir()}, core.DefaultConfig())
			p := &pane{}
			p.owner.Store(a)
			if tc.latency > 0 || tc.flow {
				conn = &delayedSzConn{Conn: conn, latency: tc.latency, flow: tc.flow}
			}
			opts := terminal.Options{Conn: conn, NoBlink: true, LocalToolsOnExit: true}
			a.configureTerminalTransferWithPicker(p, &opts, true, func(ctx context.Context, _ *mygo.Window, upload bool) ([]string, error) {
				if upload {
					return nil, fmt.Errorf("unexpected upload")
				}
				select {
				case <-time.After(delay):
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
			deadline := time.Now().Add(delay + 20*time.Second)
			for time.Now().Before(deadline) {
				a.drain()
				if p.transfer.visible && !p.transfer.active {
					break
				}
				time.Sleep(time.Millisecond)
			}
			if p.transfer.active || p.transfer.status != "ZMODEM · 传输完成" {
				t.Fatalf("download state: %+v", p.transfer)
			}
			term.Send([]byte("next\n"))
			wait()
			got, err := os.ReadFile(filepath.Join(destination, "download.bin"))
			if err != nil || !bytes.Equal(got, payload) {
				t.Fatal("download corrupt/missing", err)
			}
			waitOperator(t, a, func() bool { return term.Available() && strings.Contains(term.Text(), "shell-ready$") })
		})
	}
}

type delayedSzConn struct {
	net.Conn
	latency time.Duration
	flow    bool
	started atomic.Bool
}

func (c *delayedSzConn) Read(p []byte) (int, error) {
	if c.flow && c.started.Load() && len(p) >= 2 {
		buffer := make([]byte, len(p)/2)
		n, err := c.Conn.Read(buffer)
		for i, b := range buffer[:n] {
			p[2*i] = []byte{0x11, 0x13}[i%2]
			p[2*i+1] = b
		}
		time.Sleep(c.latency)
		return 2 * n, err
	}
	n, err := c.Conn.Read(p)
	time.Sleep(c.latency)
	return n, err
}
func (c *delayedSzConn) Write(p []byte) (int, error) {
	c.started.Store(true)
	time.Sleep(c.latency)
	return c.Conn.Write(p)
}
