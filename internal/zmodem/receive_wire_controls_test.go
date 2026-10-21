package zmodem

import (
	"bytes"
	"fmt"
	"testing"
)

func TestReceiveHeaderPadding(t *testing.T) {
	for _, format := range []string{"hex", "binary16", "binary32"} {
		for _, pads := range []int{1, 2, 3, 4, 5} {
			t.Run(fmt.Sprintf("%s/%d", format, pads), func(t *testing.T) {
				header := BuildZhexHeader(FrameZDATA, 123)
				if format == "binary16" {
					header = BuildZbinHeader(FrameZDATA, 123)
				}
				if format == "binary32" {
					header = BuildZbin32Header(FrameZDATA, 123)
				}
				header = append(bytes.Repeat([]byte{ZPAD}, pads), bytes.TrimLeft(header, "*")...)
				c := &wireFixture{Reader: bytes.NewReader(header)}
				r := &receiver{conn: c, reader: newReader(c)}
				defer r.reader.close()
				frame, offset, err := r.readHeader()
				if err != nil || frame != FrameZDATA || offset != 123 {
					t.Fatalf("header lost: %d %d %v", frame, offset, err)
				}
			})
		}
	}
}

func TestReceiveIgnoresWireFlowControl(t *testing.T) {
	// Flow control may occur even inside an escape or CRC. Escaped values
	// in the file itself must still survive unchanged.
	payload := []byte{0, 0x11, 0x13, 0x91, 0x93, ZDLE, 0xff, 'a'}
	for _, crc32 := range []bool{false, true} {
		t.Run(fmt.Sprint(crc32), func(t *testing.T) {
			header := BuildZbinHeader(FrameZDATA, 0)
			if crc32 {
				header = BuildZbin32Header(FrameZDATA, 0)
			}
			wire := append(header, fixtureSubpacket(payload, crc32)...)
			var noisy []byte
			for i, b := range wire {
				noisy = append(noisy, []byte{0x11, 0x13, 0x91, 0x93}[i%4], b)
			}
			c := &wireFixture{Reader: bytes.NewReader(noisy)}
			r := &receiver{conn: c, reader: newReader(c)}
			defer r.reader.close()
			if _, _, err := r.readHeader(); err != nil {
				t.Fatal(err)
			}
			got, _, err := r.readSubpacket()
			if err != nil || !bytes.Equal(got, payload) {
				t.Fatalf("payload %x: %v", got, err)
			}
		})
	}
}

func TestReceiveFinalOOWithFlowControl(t *testing.T) {
	c := &wireFixture{Reader: bytes.NewReader([]byte{'\r', 0x8a, 'O', 0x11, 0x93, 'O'})}
	r := &receiver{conn: c, reader: newReader(c)}
	defer r.reader.close()
	if err := r.waitFinalOO(nil); err != nil {
		t.Fatalf("completed transfer rejected: %v", err)
	}
}
