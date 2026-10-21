package zmodem

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

type finishFixture struct {
	wireFixture
	returned []byte
}

func (c *finishFixture) ReturnBuffered(b []byte) { c.returned = append(c.returned, b...) }

func TestReceiveFinishRetriesAndReturnsShell(t *testing.T) {
	for _, tail := range []string{"OO\r\nshell-ready$ ", "\r\nshell-ready$ ", ""} {
		t.Run(tail, func(t *testing.T) {
			var wire []byte
			for _, pads := range []int{1, 2, 5} {
				header := append(bytes.Repeat([]byte{ZPAD}, pads), bytes.TrimLeft(BuildZhexHeader(FrameZFIN, 0), "*")...)
				for _, b := range header {
					wire = append(wire, XON, b, 0x93)
				}
			}
			wire = append(wire, tail...)
			c := &finishFixture{wireFixture: wireFixture{Reader: bytes.NewReader(wire)}}
			r := &receiver{conn: c, reader: newReader(c)}
			err := r.waitFinalOO(nil)
			r.reader.close()
			if err != nil {
				t.Fatalf("completed batch rejected: %v", err)
			}
			if got := bytes.Count(c.written.Bytes(), BuildZhexHeader(FrameZFIN, 0)); got != 3 {
				t.Fatalf("acknowledged %d repeated ZFIN frames, want 3", got)
			}
			if !bytes.Equal(bytes.TrimLeft(c.returned, "\r\n\x8a\x11\x93"), bytes.TrimLeft(bytes.TrimPrefix([]byte(tail), []byte("OO")), "\r\n")) {
				t.Fatalf("shell tail lost: %q", c.returned)
			}
		})
	}
}

// Byte count reaching 100% is insufficient: both ZEOF and the batch ZFIN
// must arrive, and publishing the destination must succeed.
func TestReceiveFinishRequiresVerifiedFileAndBatch(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		eof, fin             bool
		closeErr             error
		corrupt, wrongOffset bool
		wantOK               bool
	}{
		{name: "data-only"},
		{name: "file-only", eof: true},
		{name: "finished-without-OO", eof: true, fin: true, wantOK: true},
		{name: "publish-failure", eof: true, fin: true, closeErr: errors.New("disk full")},
		{name: "corrupt-data", eof: true, fin: true, corrupt: true},
		{name: "wrong-EOF-offset", eof: true, fin: true, wrongOffset: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire := append(BuildZbinHeader(FrameZFILE, 0), fixtureSubpacket([]byte("file.bin\x003\x00"), false)...)
			wire = append(wire, BuildZbinHeader(FrameZDATA, 0)...)
			packet := fixtureSubpacket([]byte("abc"), false)
			if tc.corrupt {
				packet[0] ^= 1
			}
			wire = append(wire, packet...)
			if tc.eof {
				offset := uint32(3)
				if tc.wrongOffset {
					offset = 2
				}
				wire = append(wire, BuildZhexHeader(FrameZEOF, offset)...)
			}
			if tc.fin {
				wire = append(wire, BuildZhexHeader(FrameZFIN, 0)...)
			}
			c := &wireFixture{Reader: bytes.NewReader(wire)}
			writer := &finishWriter{err: tc.closeErr}
			err := ReceiveFiles(c, func(ReceiveFile) (io.WriteCloser, error) { return writer, nil }, nil)
			if (err == nil) != tc.wantOK {
				t.Fatalf("completion=%v, want success=%v", err, tc.wantOK)
			}
			if (!tc.corrupt && writer.String() != "abc") || !writer.closed {
				t.Fatal("file not written/closed")
			}
		})
	}
}

func TestReceiveFinishCancellationAndInvalidRetry(t *testing.T) {
	for _, frame := range []byte{FrameZABORT, FrameZFERR} {
		c := &wireFixture{Reader: bytes.NewReader(BuildZhexHeader(frame, 0))}
		r := &receiver{conn: c, reader: newReader(c)}
		err := r.waitFinalOO(nil)
		r.reader.close()
		if !errors.Is(err, ErrCancelled) {
			t.Fatalf("abort frame ignored: %v", err)
		}
	}
	c := &finishReadError{wireFixture: wireFixture{Reader: bytes.NewReader(nil)}}
	r := &receiver{conn: c, reader: newReader(c)}
	err := r.waitFinalOO(nil)
	r.reader.close()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("local cancel ignored: %v", err)
	}

	header := BuildZhexHeader(FrameZFIN, 0)
	header[6] = '1' // Leave the original CRC: this is not a valid retry.
	invalid := &wireFixture{Reader: bytes.NewReader(header)}
	r = &receiver{conn: invalid, reader: newReader(invalid)}
	err = r.waitFinalOO(nil)
	r.reader.close()
	if err != nil || invalid.written.Len() != 0 {
		t.Fatalf("invalid CRC acknowledged: %v", err)
	}
}

type finishReadError struct{ wireFixture }

func (*finishReadError) Read([]byte) (int, error) { return 0, context.Canceled }

type finishWriter struct {
	bytes.Buffer
	err    error
	closed bool
}

func (w *finishWriter) Close() error { w.closed = true; return w.err }
