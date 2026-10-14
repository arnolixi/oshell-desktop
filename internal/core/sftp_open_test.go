package core

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestSFTPInitializationCancellationPreservesSSHTransport(t *testing.T) {
	for _, stage := range []string{"channel-open", "subsystem", "version"} {
		t.Run(stage, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			unblock := make(chan struct{})
			defer close(unblock)
			reached := make(chan struct{})
			cfg := &ssh.ServerConfig{NoClientAuth: true}
			cfg.AddHostKey(key(t))
			go func() {
				socket, err := listener.Accept()
				if err != nil {
					return
				}
				defer socket.Close()
				server, channels, requests, err := ssh.NewServerConn(socket, cfg)
				if err != nil {
					return
				}
				defer server.Close()
				go func() {
					for req := range requests {
						req.Reply(true, nil)
					}
				}()
				for incoming := range channels {
					if stage == "channel-open" {
						close(reached)
						<-unblock
						incoming.Reject(ssh.Prohibited, "fixture complete")
						return
					}
					channel, requests, err := incoming.Accept()
					if err != nil {
						return
					}
					for request := range requests {
						if request.Type != "subsystem" {
							request.Reply(false, nil)
							continue
						}
						if stage == "subsystem" {
							close(reached)
							<-unblock
							request.Reply(false, nil)
							channel.Close()
							return
						}
						request.Reply(true, nil)
						close(reached)
						io.Copy(io.Discard, channel)
						<-unblock
						channel.Close()
						return
					}
				}
			}()
			client, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{User: "fixture", HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			transport := &transport{client: client, refs: 1}
			connection := &Connection{Client: client, transport: transport}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result := make(chan error, 1)
			go func() {
				files, release, err := connection.OpenFiles(ctx)
				if files != nil {
					files.Close()
					release()
				}
				result <- err
			}()
			select {
			case <-reached:
			case <-time.After(2 * time.Second):
				t.Fatal("fixture did not reach", stage)
			}
			cancel()
			select {
			case err := <-result:
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancel blocked", stage)
			}
			if stage == "channel-open" {
				if _, _, err := connection.OpenFiles(context.Background()); err == nil {
					t.Fatal("unbounded pending channel opens")
				}
			}
			transport.mu.Lock()
			refs := transport.refs
			transport.mu.Unlock()
			if refs != 1 {
				t.Fatal("cancel retained lease", refs)
			}
			// A cancelled file browser must not disconnect the shared SSH terminal.
			if ok, _, err := client.SendRequest("still-alive", true, nil); err != nil || !ok {
				t.Fatal("shared transport closed", err)
			}
		})
	}
}
