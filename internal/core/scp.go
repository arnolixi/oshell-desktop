// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'" }
func scpLine(reader *bufio.Reader) (string, error) {
	line := make([]byte, 0, 128)
	for len(line) < 8192 {
		b, e := reader.ReadByte()
		if e != nil {
			return "", e
		}
		if b == '\n' {
			return string(line), nil
		}
		line = append(line, b)
	}
	return "", fmt.Errorf("SCP 协议行过长")
}
func scpACK(reader *bufio.Reader) error {
	b, e := reader.ReadByte()
	if e != nil {
		return e
	}
	if b == 0 {
		return nil
	}
	if b == 1 || b == 2 {
		message, e := scpLine(reader)
		if e != nil {
			return e
		}
		return fmt.Errorf("SCP 服务端错误：%s", message)
	}
	return fmt.Errorf("SCP 响应无效")
}
func (c *Connection) scp(ctx context.Context, command string, transfer func(*bufio.Reader, io.WriteCloser) error) error {
	release, e := c.Lease()
	if e != nil {
		return e
	}
	defer release()
	session, e := c.Client.NewSession()
	if e != nil {
		return e
	}
	defer session.Close()
	stop := context.AfterFunc(ctx, func() { session.Close() })
	defer stop()
	input, e := session.StdinPipe()
	if e != nil {
		return e
	}
	output, e := session.StdoutPipe()
	if e != nil {
		return e
	}
	session.Stderr = io.Discard
	if e = session.Start(command); e != nil {
		return e
	}
	if e = transfer(bufio.NewReader(output), input); e != nil {
		return e
	}
	input.Close()
	e = session.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return e
}
func (c *Connection) SCPUpload(ctx context.Context, local, remote string, report func(TransferProgress)) error {
	ctx = snapshotTransferLimit(ctx)
	name := path.Base(remote)
	if !safeEntry(name) || strings.ContainsAny(remote, "\x00\r\n") {
		return fmt.Errorf("SCP 目标路径无效")
	}
	return c.scp(ctx, "scp -rt -- "+shellQuote(path.Dir(remote)), func(reader *bufio.Reader, writer io.WriteCloser) error {
		if e := scpACK(reader); e != nil {
			return e
		}
		state := &transferState{ctx: ctx, report: report}
		var send func(string, string, int) error
		send = func(local, name string, depth int) error {
			if depth > 32 || !safeEntry(name) {
				return fmt.Errorf("SCP 目录层级或名称无效")
			}
			if e := ctx.Err(); e != nil {
				return e
			}
			info, e := os.Lstat(local)
			if e != nil {
				return e
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("不跟随符号链接")
			}
			if info.IsDir() {
				if _, e = fmt.Fprintf(writer, "D0755 0 %s\n", name); e != nil {
					return e
				}
				if e = scpACK(reader); e != nil {
					return e
				}
				entries, e := os.ReadDir(local)
				if e != nil {
					return e
				}
				for _, entry := range entries {
					if e = send(filepath.Join(local, entry.Name()), entry.Name(), depth+1); e != nil {
						return e
					}
				}
				if _, e = io.WriteString(writer, "E\n"); e != nil {
					return e
				}
				return scpACK(reader)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("不是普通文件")
			}
			file, e := os.Open(local)
			if e != nil {
				return e
			}
			defer file.Close()
			if _, e = fmt.Fprintf(writer, "C%04o %d %s\n", info.Mode().Perm(), info.Size(), name); e != nil {
				return e
			}
			if e = scpACK(reader); e != nil {
				return e
			}
			limited := &io.LimitedReader{R: file, N: info.Size()}
			if e = state.copy(writer, limited, remote); e != nil {
				return e
			}
			if limited.N != 0 {
				return io.ErrUnexpectedEOF
			}
			if e = writeAll(writer, []byte{0}); e != nil {
				return e
			}
			return scpACK(reader)
		}
		return send(local, name, 0)
	})
}
func (c *Connection) SCPDownload(ctx context.Context, remote, local string, replace bool, report func(TransferProgress)) error {
	ctx = snapshotTransferLimit(ctx)
	if strings.ContainsAny(remote, "\x00\r\n") || !safeEntry(path.Base(remote)) {
		return fmt.Errorf("SCP 远程路径无效")
	}
	temp, e := os.MkdirTemp(filepath.Dir(local), ".oshell-scp-*")
	if e != nil {
		return e
	}
	defer os.RemoveAll(temp)
	dest := filepath.Join(temp, "content")
	received := false
	directory := false
	e = c.scp(ctx, "scp -rf -- "+shellQuote(remote), func(reader *bufio.Reader, writer io.WriteCloser) error {
		if e := writeAll(writer, []byte{0}); e != nil {
			return e
		}
		state := &transferState{ctx: ctx, report: report}
		var receive func(string, int) error
		receive = func(parent string, depth int) error {
			if depth > 32 {
				return fmt.Errorf("SCP 目录层级过深")
			}
			for {
				if e := ctx.Err(); e != nil {
					return e
				}
				tag, e := reader.ReadByte()
				if e == io.EOF && depth == 0 {
					return nil
				}
				if e != nil {
					return e
				}
				line, e := scpLine(reader)
				if e != nil {
					return e
				}
				if tag == 1 || tag == 2 {
					return fmt.Errorf("SCP 服务端错误：%s", line)
				}
				if tag == 'T' {
					if e = writeAll(writer, []byte{0}); e != nil {
						return e
					}
					continue
				}
				if tag == 'E' {
					if depth == 0 {
						return fmt.Errorf("SCP 目录层级无效")
					}
					return writeAll(writer, []byte{0})
				}
				if tag != 'C' && tag != 'D' {
					return fmt.Errorf("未知 SCP 消息")
				}
				parts := strings.SplitN(line, " ", 3)
				if len(parts) != 3 || !safeEntry(parts[2]) {
					return fmt.Errorf("SCP 文件名无效")
				}
				size, e := strconv.ParseInt(parts[1], 10, 64)
				if e != nil || size < 0 {
					return fmt.Errorf("SCP 文件大小无效")
				}
				target := filepath.Join(parent, parts[2])
				if depth == 0 {
					if received || parts[2] != path.Base(remote) {
						return fmt.Errorf("SCP 返回了未请求的项目")
					}
					received = true
					directory = tag == 'D'
					target = dest
				}
				if tag == 'D' {
					if e = os.Mkdir(target, 0700); e != nil {
						return e
					}
					if e = writeAll(writer, []byte{0}); e != nil {
						return e
					}
					if e = receive(target, depth+1); e != nil {
						return e
					}
					continue
				}
				file, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if e != nil {
					return e
				}
				if e = writeAll(writer, []byte{0}); e != nil {
					file.Close()
					return e
				}
				limited := &io.LimitedReader{R: reader, N: size}
				e = state.copy(file, limited, remote)
				ce := file.Close()
				if e != nil {
					return e
				}
				if ce != nil {
					return ce
				}
				if limited.N != 0 {
					return io.ErrUnexpectedEOF
				}
				if e = scpACK(reader); e != nil {
					return e
				}
				if e = writeAll(writer, []byte{0}); e != nil {
					return e
				}
			}
		}
		return receive(temp, 0)
	})
	if e != nil {
		return e
	}
	if !received {
		return fmt.Errorf("SCP 没有返回文件")
	}
	if _, e = os.Lstat(local); e == nil && (directory || !replace) {
		return fmt.Errorf("本地目标已存在")
	}
	return publishDownload(dest, local, directory, replace)
}

type SCPFileSystem struct {
	RemoteFileSystem
	Connection *Connection
}

func (f *SCPFileSystem) Upload(ctx context.Context, local, remote string, p func(TransferProgress)) error {
	if _, e := f.Lstat(remote); !os.IsNotExist(e) {
		if e != nil {
			return e
		}
		return fmt.Errorf("远程目标已存在")
	}
	return f.Connection.SCPUpload(ctx, local, remote, p)
}
func (f *SCPFileSystem) Download(ctx context.Context, remote, local string, replace bool, p func(TransferProgress)) error {
	return f.Connection.SCPDownload(ctx, remote, local, replace, p)
}
