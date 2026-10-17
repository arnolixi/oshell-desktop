package zmodem

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
)

type SendFile struct {
	Name   string
	Size   int64
	Reader io.ReaderAt
}
type FileProgress struct {
	Name         string
	Bytes, Total int64
	Index        int
}

func SendFiles(conn net.Conn, files []SendFile, progress func(FileProgress)) error {
	if len(files) == 0 {
		return fmt.Errorf("未选择上传文件")
	}
	s := newSender(conn, nil, "")
	defer s.reader.close()
	if _, e := conn.Write(BuildZhexHeader(FrameZRQINIT, 0)); e != nil {
		return e
	}
	if e := s.waitForZrinit(); e != nil {
		return e
	}
	for i, file := range files {
		if file.Size < 0 || file.Size > int64(^uint32(0)) || file.Name == "" || strings.ContainsAny(file.Name, "\x00\r\n/\\") {
			return fmt.Errorf("ZMODEM 文件名或大小无效")
		}
		s.filename = file.Name
		s.source = file.Reader
		s.size = file.Size
		s.onProgress = func(n, total int64) {
			if progress != nil {
				progress(FileProgress{file.Name, n, total, i})
			}
		}
		if e := s.sendCurrentFile(false); e != nil && !errors.Is(e, ErrSkipped) {
			return e
		}
	}
	if _, e := conn.Write(BuildZhexHeader(FrameZFIN, 0)); e != nil {
		return e
	}
	if e := s.waitForZfin(); e != nil {
		return e
	}
	_, e := conn.Write([]byte("OO"))
	return e
}

type ReceiveOptions struct{ ConfirmPrompt func([]byte) bool }
type ReceiveFile struct {
	Name string
	Size int64
}

// ReceiveFiles streams each file into the writer returned by open. A rejected
// file sends ZSKIP; no server-provided path is interpreted by this package.
func ReceiveFiles(conn net.Conn, open func(ReceiveFile) (io.WriteCloser, error), progress func(FileProgress), options ...ReceiveOptions) error {
	r := &receiver{conn: conn, reader: newReader(conn)}
	defer r.reader.close()
	if e := r.sendZRINIT(); e != nil {
		return e
	}
	index := 0
	for {
		frame, _, e := r.readHeader()
		if e != nil {
			return e
		}
		switch frame {
		case FrameZRQINIT:
			if e = r.sendZRINIT(); e != nil {
				return e
			}
		case FrameZSINIT:
			if e = r.receiveSenderInit(); e != nil {
				return e
			}
		case FrameZFILE:
			r.filename = ""
			r.totalSize = 0
			r.received = 0
			r.out = nil
			if e = r.readZFileSubpacket(); e != nil {
				return e
			}
			writer, e := open(ReceiveFile{r.filename, int64(r.totalSize)})
			if e != nil {
				if _, e = conn.Write(r.buildHeader(FrameZSKIP, 0)); e != nil {
					return e
				}
				continue
			}
			r.writer = writer
			name, total, current := r.filename, int64(r.totalSize), index
			r.cfg.OnStatus = func(s ReceiveStatus) {
				if progress != nil {
					progress(FileProgress{name, int64(s.Bytes), total, current})
				}
			}
			if _, e = conn.Write(r.buildHeader(FrameZRPOS, 0)); e != nil {
				if abort, ok := writer.(interface{ Abort(error) }); ok {
					abort.Abort(e)
				}
				writer.Close()
				return e
			}
			e = r.receiveData()
			if e == nil && r.totalSize > 0 && r.received != r.totalSize {
				e = io.ErrUnexpectedEOF
			}
			if e != nil {
				if abort, ok := writer.(interface{ Abort(error) }); ok {
					abort.Abort(e)
				}
			}
			closeErr := writer.Close()
			r.writer = nil
			if e != nil {
				return e
			}
			if closeErr != nil {
				return closeErr
			}
			index++
			if e = r.sendZRINIT(); e != nil {
				return e
			}
		case FrameZFIN:
			if _, e = conn.Write(BuildZhexHeader(FrameZFIN, 0)); e != nil {
				return e
			}
			var confirm func([]byte) bool
			if len(options) > 0 {
				confirm = options[0].ConfirmPrompt
			}
			return r.waitFinalOO(confirm)
		case FrameZABORT, FrameZFERR:
			return ErrCancelled
		default:
			if e = r.sendZRINIT(); e != nil {
				return e
			}
		}
	}
}
func Cancel(conn io.Writer) { conn.Write(cancelEcho) }

func (r *receiver) waitFinalOO(confirm func([]byte) bool) error {
	var pending []byte
	previousO := false
	for {
		b, e := r.reader.readByte(receiveTimeout)
		if e != nil {
			return e
		}
		pending = append(pending, b)
		if len(pending) > 8192 {
			pending = append([]byte(nil), pending[len(pending)-8192:]...)
		}
		if previousO && b == 'O' {
			return nil
		}
		previousO = b == 'O'
		if confirm != nil && confirm(pending) {
			start := 0
			for i, c := range pending {
				if c == '\n' || c == '\r' || c == 0x8a {
					start = i + 1
				}
			}
			r.reader.unread(pending[start:])
			return nil
		}
	}
}
