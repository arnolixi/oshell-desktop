package terminal

import (
	"bytes"
	"oshell-desktop/internal/zmodem"
	"testing"
)

func TestTransferDetectorEveryBoundary(t *testing.T) {
	for _, kind := range []byte{zmodem.FrameZRINIT, zmodem.FrameZRQINIT} {
		header := zmodem.BuildZhexHeader(kind, 0)
		for split := 0; split <= len(header); split++ {
			var d transferDetector
			text, upload, protocol := d.consume(append([]byte("hello\r\n"), header[:split]...))
			if protocol == nil {
				more, u, p := d.consume(append(append([]byte(nil), header[split:]...), []byte("payload")...))
				text = append(text, more...)
				upload, protocol = u, p
			}
			if protocol == nil || !bytes.HasPrefix(protocol, header[:18]) || upload != (kind == zmodem.FrameZRINIT) {
				t.Fatalf("kind %d split %d: %q", kind, split, protocol)
			}
			if !bytes.HasPrefix(text, []byte("hello\r\n")) || bytes.Contains(text, []byte{0x18}) {
				t.Fatalf("leaked header: %q", text)
			}
		}
	}
}

func TestTransferDetectorLiteralOutputExactlyOnce(t *testing.T) {
	for _, input := range []string{"*", "**", "*** ordinary ** stars", "a*\x18Bhello", "a**\x18B000000000000ffz", "abc\x1b[31mred\x1b[0m**"} {
		for chunk := 1; chunk <= len(input); chunk++ {
			var d transferDetector
			var got []byte
			for i := 0; i < len(input); i += chunk {
				text, _, p := d.consume([]byte(input[i:min(i+chunk, len(input))]))
				if p != nil {
					t.Fatal("false transfer")
				}
				got = append(got, text...)
			}
			got = append(got, d.flush()...)
			if string(got) != input {
				t.Fatalf("chunk %d: %q != %q", chunk, got, input)
			}
		}
	}
}
