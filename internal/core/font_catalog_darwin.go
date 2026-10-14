//go:build darwin

package core

import (
	"context"
	"fmt"
	"github.com/ebitengine/purego"
	"sync"
)

var fontAPI struct {
	once    sync.Once
	err     error
	names   func() uintptr
	count   func(uintptr) int64
	at      func(uintptr, int64) uintptr
	length  func(uintptr) int64
	cstring func(uintptr, *byte, int64, uint32) bool
	create  func(uintptr, float64, uintptr) uintptr
	traits  func(uintptr) uint32
	glyphs  func(uintptr, *uint16, *uint16, int64) bool
	release func(uintptr)
}

func platformFonts(ctx context.Context) ([]FontEntry, error) {
	fontAPI.once.Do(func() {
		ct, e := purego.Dlopen("/System/Library/Frameworks/CoreText.framework/CoreText", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if e != nil {
			fontAPI.err = e
			return
		}
		cf, e := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if e != nil {
			fontAPI.err = e
			return
		}
		reg := func(dst any, lib uintptr, name string) {
			ptr, e := purego.Dlsym(lib, name)
			if e != nil {
				fontAPI.err = e
				return
			}
			purego.RegisterFunc(dst, ptr)
		}
		reg(&fontAPI.names, ct, "CTFontManagerCopyAvailableFontFamilyNames")
		reg(&fontAPI.create, ct, "CTFontCreateWithName")
		reg(&fontAPI.traits, ct, "CTFontGetSymbolicTraits")
		reg(&fontAPI.glyphs, ct, "CTFontGetGlyphsForCharacters")
		reg(&fontAPI.count, cf, "CFArrayGetCount")
		reg(&fontAPI.at, cf, "CFArrayGetValueAtIndex")
		reg(&fontAPI.length, cf, "CFStringGetLength")
		reg(&fontAPI.cstring, cf, "CFStringGetCString")
		reg(&fontAPI.release, cf, "CFRelease")
	})
	if fontAPI.err != nil {
		return nil, fontAPI.err
	}
	list := fontAPI.names()
	if list == 0 {
		return nil, fmt.Errorf("无法读取系统字体")
	}
	defer fontAPI.release(list)
	var chars []uint16
	for _, r := range "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789[]{}|~" {
		chars = append(chars, uint16(r))
	}
	glyphs := make([]uint16, len(chars))
	var out []FontEntry
	for i, n := int64(0), fontAPI.count(list); i < n && i < 5000; i++ {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		name := fontAPI.at(list, i)
		length := fontAPI.length(name)
		if length <= 0 || length > 512 {
			continue
		}
		buffer := make([]byte, length*4+1)
		if !fontAPI.cstring(name, &buffer[0], int64(len(buffer)), 0x08000100) {
			continue
		}
		end := 0
		for end < len(buffer) && buffer[end] != 0 {
			end++
		}
		font := fontAPI.create(name, 13, 0)
		if font == 0 {
			continue
		}
		supports := fontAPI.glyphs(font, &chars[0], &glyphs[0], int64(len(chars)))
		mono := fontAPI.traits(font)&(1<<10) != 0
		fontAPI.release(font)
		if supports {
			out = append(out, FontEntry{string(buffer[:end]), mono})
		}
	}
	return out, nil
}
