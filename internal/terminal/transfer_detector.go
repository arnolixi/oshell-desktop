// SPDX-License-Identifier: GPL-3.0-only
package terminal

import (
	"bytes"
	"oshell-desktop/internal/zmodem"
)

// Matches OShell 0.2.64's streaming detector. Printable '*' characters are
// displayed immediately, but remain eligible for matching across read calls.
// A timer must never discard a partial protocol header on a slow connection.
type transferDetector struct {
	pending   []byte
	displayed int
}

var transferPrefix = []byte{'*', '*', 0x18, 'B'}

func (d *transferDetector) consume(input []byte) (text []byte, upload bool, protocol []byte) {
	d.pending = append(d.pending, input...)
	for {
		i := bytes.Index(d.pending, transferPrefix)
		if i < 0 {
			break
		}
		text = append(text, d.pending[min(d.displayed, i):i]...)
		d.pending = d.pending[i:]
		d.displayed = max(0, d.displayed-i)
		if len(d.pending) < 18 {
			return
		}
		kind, _, valid := zmodem.DecodeHexHeader(d.pending, 4)
		if valid && (kind == zmodem.FrameZRINIT || kind == zmodem.FrameZRQINIT) {
			protocol = d.pending
			upload = kind == zmodem.FrameZRINIT
			d.pending = nil
			d.displayed = 0
			return
		}
		if d.displayed > 0 {
			d.displayed--
		} else {
			text = append(text, d.pending[0])
		}
		d.pending = d.pending[1:]
	}
	retained := 0
	for n := 1; n <= min(3, len(d.pending)); n++ {
		if bytes.Equal(d.pending[len(d.pending)-n:], transferPrefix[:n]) {
			retained = n
		}
	}
	flushed := len(d.pending) - retained
	text = append(text, d.pending[min(d.displayed, flushed):flushed]...)
	d.displayed = max(0, d.displayed-flushed)
	d.pending = append([]byte(nil), d.pending[flushed:]...)
	printable := min(retained, 2)
	if printable > d.displayed {
		text = append(text, d.pending[d.displayed:printable]...)
		d.displayed = printable
	}
	return
}

func (d *transferDetector) flush() []byte {
	text := append([]byte(nil), d.pending[d.displayed:]...)
	d.pending = nil
	d.displayed = 0
	return text
}
