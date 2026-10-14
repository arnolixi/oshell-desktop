//go:build darwin

package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"golang.org/x/sys/unix"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var ErrLaunchBusy = errors.New("此配置已有启动服务")

func launchDirectory(configuration string) (string, error) {
	absolute, e := filepath.Abs(configuration)
	if e != nil {
		return "", e
	}
	digest := sha256.Sum256([]byte(filepath.Clean(absolute)))
	dir := filepath.Join("/tmp", fmt.Sprintf("oshell-desktop-launch-%d-%s", os.Geteuid(), hex.EncodeToString(digest[:10])))
	if e = os.Mkdir(dir, 0700); e != nil && !os.IsExist(e) {
		return "", e
	}
	var stat unix.Stat_t
	if e = unix.Lstat(dir, &stat); e != nil || stat.Mode&unix.S_IFMT != unix.S_IFDIR || stat.Uid != uint32(os.Geteuid()) || stat.Mode&0077 != 0 {
		return "", fmt.Errorf("启动通道目录权限无效")
	}
	return dir, nil
}
func launchLock(dir, name string) (*os.File, error) {
	fd, e := unix.Open(filepath.Join(dir, name), unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if e != nil {
		return nil, e
	}
	var stat unix.Stat_t
	if e = unix.Fstat(fd, &stat); e != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Uid != uint32(os.Geteuid()) || stat.Mode&0077 != 0 {
		unix.Close(fd)
		return nil, fmt.Errorf("启动通道锁属性无效")
	}
	if e = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); e != nil {
		unix.Close(fd)
		if errors.Is(e, unix.EWOULDBLOCK) {
			return nil, ErrLaunchBusy
		}
		return nil, e
	}
	return os.NewFile(uintptr(fd), name), nil
}
func verifyLaunchPeer(conn *net.UnixConn) error {
	raw, e := conn.SyscallConn()
	if e != nil {
		return e
	}
	var peerErr error
	e = raw.Control(func(fd uintptr) {
		cred, err := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if err != nil {
			peerErr = err
		} else if cred.Uid != uint32(os.Geteuid()) {
			peerErr = fmt.Errorf("启动通道用户不匹配")
		}
	})
	if e != nil {
		return e
	}
	return peerErr
}

type launchRecent struct {
	done     chan struct{}
	response LaunchResponse
}
type LaunchServer struct {
	listener *net.UnixListener
	lock     *os.File
	handler  func(context.Context, ExternalLaunchRequest) error
	ctx      context.Context
	cancel   context.CancelFunc
	slots    chan struct{}
	mu       sync.Mutex
	recent   map[string]*launchRecent
	order    []string
	once     sync.Once
}

func StartLaunchServer(configuration string, handler func(context.Context, ExternalLaunchRequest) error) (*LaunchServer, error) {
	dir, e := launchDirectory(configuration)
	if e != nil {
		return nil, e
	}
	lock, e := launchLock(dir, "server.lock")
	if e != nil {
		return nil, e
	}
	path := filepath.Join(dir, "s")
	var stat unix.Stat_t
	if e = unix.Lstat(path, &stat); e == nil {
		if stat.Mode&unix.S_IFMT != unix.S_IFSOCK || stat.Uid != uint32(os.Geteuid()) {
			lock.Close()
			return nil, fmt.Errorf("启动通道被其他文件占用")
		}
		if e = os.Remove(path); e != nil {
			lock.Close()
			return nil, e
		}
	} else if !errors.Is(e, unix.ENOENT) {
		lock.Close()
		return nil, e
	}
	listener, e := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if e != nil {
		lock.Close()
		return nil, e
	}
	if e = os.Chmod(path, 0600); e != nil {
		listener.Close()
		lock.Close()
		return nil, e
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := &LaunchServer{listener: listener, lock: lock, handler: handler, ctx: ctx, cancel: cancel, slots: make(chan struct{}, 8), recent: map[string]*launchRecent{}}
	go server.accept()
	return server, nil
}
func (s *LaunchServer) Close() error {
	var e error
	s.once.Do(func() { s.cancel(); e = s.listener.Close(); s.lock.Close() })
	return e
}
func (s *LaunchServer) accept() {
	for {
		conn, e := s.listener.AcceptUnix()
		if e != nil {
			return
		}
		select {
		case s.slots <- struct{}{}:
			go s.serve(conn)
		default:
			conn.Close()
		}
	}
}
func (s *LaunchServer) serve(conn *net.UnixConn) {
	defer conn.Close()
	defer func() { <-s.slots }()
	stop := context.AfterFunc(s.ctx, func() { conn.Close() })
	defer stop()
	if verifyLaunchPeer(conn) != nil {
		return
	}
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	var request ExternalLaunchRequest
	if readLaunchMessage(conn, &request) != nil || request.Validate() != nil {
		_ = writeLaunchMessage(conn, LaunchResponse{Message: "启动参数无效或通道读取失败"})
		return
	}
	conn.SetDeadline(time.Now().Add(300 * time.Second))
	key := request.ID()
	s.mu.Lock()
	entry, duplicate := s.recent[key]
	if !duplicate {
		entry = &launchRecent{done: make(chan struct{})}
		s.recent[key] = entry
	}
	s.mu.Unlock()
	if duplicate {
		select {
		case <-entry.done:
		case <-s.ctx.Done():
			return
		}
	} else {
		ctx, cancel := context.WithTimeout(s.ctx, 300*time.Second)
		e := s.handler(ctx, request)
		cancel()
		entry.response = LaunchResponse{Accepted: e == nil}
		if e != nil {
			entry.response.Message = "OShell 尚未解锁、正在关闭或启动请求未被接受"
		}
		s.mu.Lock()
		if e != nil {
			delete(s.recent, key)
		} else {
			s.order = append(s.order, key)
			if len(s.order) > 256 {
				delete(s.recent, s.order[0])
				s.order = s.order[1:]
			}
		}
		close(entry.done)
		s.mu.Unlock()
	}
	_ = writeLaunchMessage(conn, entry.response)
}
func SendExternalLaunch(ctx context.Context, configuration string, request ExternalLaunchRequest) error {
	if e := request.Validate(); e != nil {
		return e
	}
	dir, e := launchDirectory(configuration)
	if e != nil {
		return e
	}
	dialer := net.Dialer{}
	raw, e := dialer.DialContext(ctx, "unix", filepath.Join(dir, "s"))
	if e != nil {
		return fmt.Errorf("OShell 启动服务尚未就绪")
	}
	conn := raw.(*net.UnixConn)
	defer conn.Close()
	if e = verifyLaunchPeer(conn); e != nil {
		return e
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline := time.Now().Add(300 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	conn.SetDeadline(deadline)
	if e = writeLaunchMessage(conn, request); e != nil {
		return e
	}
	var response LaunchResponse
	if e = readLaunchMessage(conn, &response); e != nil {
		return e
	}
	if !response.Accepted {
		return fmt.Errorf("%s", response.Message)
	}
	return nil
}
