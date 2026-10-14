package core

import (
	"context"
	"fmt"
	"io"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// OpenFiles returns promptly on cancellation without closing other SSH channels.
func (c *Connection) OpenFiles(ctx context.Context) (RemoteFileSystem, func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	release, err := c.Lease()
	if err != nil {
		return nil, nil, err
	}
	// SSH channel-open has no cancellation primitive. Bound pending opens to one
	// per transport until the peer replies or that transport disconnects.
	if !c.transport.fileOpening.CompareAndSwap(false, true) {
		release()
		return nil, nil, fmt.Errorf("文件连接仍在初始化，请稍后重试或重新连接")
	}
	type result struct {
		client *sftp.Client
		err    error
	}
	ready := make(chan result)
	go func() {
		defer c.transport.fileOpening.Store(false)
		client, err := openSFTPChannel(ctx, c.Client)
		select {
		case ready <- result{client, err}:
		case <-ctx.Done():
			if client != nil {
				client.Close()
			}
			release()
		}
	}()
	select {
	case <-ctx.Done():
		release()
		return nil, nil, ctx.Err()
	case result := <-ready:
		if err := ctx.Err(); err != nil {
			if result.client != nil {
				result.client.Close()
			}
			release()
			return nil, nil, err
		}
		if result.err != nil {
			release()
			return nil, nil, result.err
		}
		return &SFTPFileSystem{result.client}, release, nil
	}
}

func openSFTPChannel(ctx context.Context, client *ssh.Client) (*sftp.Client, error) {
	channel, requests, err := client.OpenChannel("session", nil)
	if err != nil {
		return nil, err
	}
	go ssh.DiscardRequests(requests)
	go io.Copy(io.Discard, channel.Stderr())
	stop := context.AfterFunc(ctx, func() { channel.Close() })
	defer stop()
	if err = ctx.Err(); err != nil {
		channel.Close()
		return nil, err
	}
	ok, err := channel.SendRequest("subsystem", true, ssh.Marshal(struct{ Name string }{"sftp"}))
	if err != nil || !ok {
		channel.Close()
		if err == nil {
			err = fmt.Errorf("服务器不支持 SFTP 子系统")
		}
		return nil, err
	}
	files, err := sftp.NewClientPipe(channel, channel)
	if err != nil {
		channel.Close()
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		files.Close()
		return nil, err
	}
	return files, nil
}
