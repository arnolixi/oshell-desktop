// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"context"
	"errors"
	"fmt"

	"net"
	"os"

	"sync"

	"golang.org/x/crypto/ssh"

	"golang.org/x/crypto/ssh/knownhosts"
)

type AuthenticationPrompt struct {
	Title             string
	Identity          SSHIdentity
	Name, Instruction string
	Questions         []string
	Echo              []bool
}

type Credentials struct {
	ExternalLaunch     bool // Ephemeral external launch; never persisted.
	OpenTerminal       bool // Desktop launch mode; does not change the saved credential identity.
	PasswordProtection string
	// MixedPasswordChallenge identifies the sole target-password field in a multi-question challenge.
	MixedPasswordChallenge func(context.Context, AuthenticationPrompt, int) ([]string, error)
	// PasswordAuthenticated reports the accepted login password after the full handshake.
	PasswordAuthenticated func(string)
	// PasswordChallenge is only used for recognized target login-password prompts.
	PasswordChallenge                                          func(context.Context, AuthenticationPrompt) (string, error)
	Method                                                     string
	ProxyCredentials                                           func(context.Context, ProxySettings, SSHIdentity) (Credentials, error)
	Challenge                                                  func(context.Context, AuthenticationPrompt) ([]string, error)
	ExpectedIdentity                                           *SSHIdentity
	Password, KeyPassphrase, ProxyPassword, ProxyKeyPassphrase string
	SavePassword                                               bool
}
type HostPrompt struct{ Address, Algorithm, Fingerprint string }
type TrustStore struct {
	Path string
	mu   sync.Mutex
}

func (t *TrustStore) Callback(ctx context.Context, approve func(HostPrompt) bool) ssh.HostKeyCallback {
	return func(host string, remote net.Addr, key ssh.PublicKey) error {
		t.mu.Lock()
		defer t.mu.Unlock()
		f, err := os.OpenFile(t.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		if err = f.Close(); err != nil {
			return err
		}
		cb, err := knownhosts.New(t.Path)
		if err != nil {
			return err
		}
		err = cb(host, remote, key)
		if err == nil {
			return nil
		}
		var ke *knownhosts.KeyError
		if !errors.As(err, &ke) || len(ke.Want) > 0 {
			return fmt.Errorf("主机密钥不匹配，连接已阻止。请核实服务器身份后检查 %s: %w", t.Path, err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if approve == nil || !approve(HostPrompt{host, key.Type(), ssh.FingerprintSHA256(key)}) {
			return fmt.Errorf("未信任服务器指纹")
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		f, err = os.OpenFile(t.Path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(f, knownhosts.Line([]string{knownhosts.Normalize(host)}, key))
		closeErr := f.Close()
		if err != nil {
			return err
		}
		return closeErr
	}
}

func authenticationTitle(challenge func(context.Context, AuthenticationPrompt) ([]string, error), title string) func(context.Context, AuthenticationPrompt) ([]string, error) {
	if challenge == nil {
		return nil
	}
	return func(ctx context.Context, request AuthenticationPrompt) ([]string, error) {
		request.Title = title
		return challenge(ctx, request)
	}
}
