package core

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestMacFontCatalogIncludesUsableMonospace(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS Core Text catalog")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fonts, e := FontCatalog(ctx)
	if e != nil {
		t.Fatal(e)
	}
	mono, other := 0, 0
	for _, font := range fonts {
		if font.Monospaced {
			mono++
		} else {
			other++
		}
	}
	if mono == 0 || other == 0 {
		t.Fatal("font classification incomplete", mono, other)
	}
}
