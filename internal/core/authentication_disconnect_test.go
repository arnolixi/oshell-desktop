package core

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestSSHDisconnectedAuthenticationCancelsPrompt(t *testing.T) {
	for _, method := range []string{"password", "keyboard-interactive"} {
		t.Run(method, func(t *testing.T) {
			t.Setenv("OSHELL_TEST_NO_AGENT", "1")
			t.Setenv("OSHELL_TEST_EXPLICIT_KEYS_ONLY", "1")
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			serverConfig := &ssh.ServerConfig{}
			serverConfig.AddHostKey(key(t))
			if method == "password" {
				serverConfig.PasswordCallback = func(ssh.ConnMetadata, []byte) (*ssh.Permissions, error) { return nil, nil }
			} else {
				serverConfig.KeyboardInteractiveCallback = func(_ ssh.ConnMetadata, ask ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
					_, err := ask("MFA", "", []string{"Code:"}, []bool{false})
					return nil, err
				}
			}
			peer := make(chan net.Conn, 1)
			serverDone := make(chan struct{})
			go func() {
				defer close(serverDone)
				c, err := listener.Accept()
				if err != nil {
					return
				}
				defer c.Close()
				peer <- c
				connection, _, _, err := ssh.NewServerConn(c, serverConfig)
				if err == nil {
					connection.Close()
				}
			}()
			conn, err := net.Dial("tcp", listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			prompt := make(chan struct{}, 1)
			challenge := func(ctx context.Context, _ AuthenticationPrompt) ([]string, error) {
				prompt <- struct{}{}
				<-ctx.Done()
				return nil, ctx.Err()
			}
			cred := Credentials{Challenge: challenge}
			if method == "password" {
				cred.Method = "password"
			}
			result := make(chan error, 1)
			trust := testTrust(t)
			go func() {
				connection, err := authenticate(ctx, conn, testProfile(listener.Addr().String()), cred, trust, acceptHost, &atomic.Bool{})
				if connection != nil {
					connection.Close()
				}
				result <- err
			}()
			select {
			case <-prompt:
			case <-ctx.Done():
				t.Fatal("prompt never opened")
			}
			(<-peer).Close()
			select {
			case err := <-result:
				if err == nil {
					t.Fatal("disconnected client authenticated")
				}
			case <-time.After(time.Second):
				t.Fatal("closed peer left an abandoned prompt waiting for operator input")
			}
			<-serverDone
		})
	}
}
