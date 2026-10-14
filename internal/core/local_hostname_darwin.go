//go:build darwin

package core

import (
	"bytes"
	"github.com/ebitengine/purego"
	"sync"
)

var hostNameAPI struct {
	once        sync.Once
	copy        func(uintptr) uintptr
	release     func(uintptr)
	stringBytes func(uintptr, *byte, int64, uint32) bool
}

func configuredLocalHostname() string {
	a := &hostNameAPI
	a.once.Do(func() {
		sc, e := purego.Dlopen("/System/Library/Frameworks/SystemConfiguration.framework/SystemConfiguration", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if e != nil {
			return
		}
		cf, e := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if e != nil {
			return
		}
		copy, e := purego.Dlsym(sc, "SCDynamicStoreCopyLocalHostName")
		if e != nil {
			return
		}
		release, e := purego.Dlsym(cf, "CFRelease")
		if e != nil {
			return
		}
		text, e := purego.Dlsym(cf, "CFStringGetCString")
		if e != nil {
			return
		}
		purego.RegisterFunc(&a.copy, copy)
		purego.RegisterFunc(&a.release, release)
		purego.RegisterFunc(&a.stringBytes, text)
	})
	if a.copy == nil {
		return ""
	}
	value := a.copy(0)
	if value == 0 {
		return ""
	}
	defer a.release(value)
	buffer := make([]byte, 1024)
	if !a.stringBytes(value, &buffer[0], int64(len(buffer)), 0x08000100) {
		return ""
	}
	return string(bytes.TrimRight(buffer, "\x00"))
}
