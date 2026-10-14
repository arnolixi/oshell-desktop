//go:build darwin

package core

import (
	"bytes"
	"github.com/ebitengine/purego"
	"sync"
)

var localNetworkAPI struct {
	once            sync.Once
	copyValue       func(uintptr, uintptr) uintptr
	makeString      func(uintptr, string, uint32) uintptr
	dictionaryValue func(uintptr, uintptr) uintptr
	typeID          func(uintptr) uintptr
	dictionaryType  func() uintptr
	stringType      func() uintptr
	stringBytes     func(uintptr, *byte, int64, uint32) bool
	release         func(uintptr)
	ready           bool
}

func primaryLocalInterfaces() map[string]bool {
	a := &localNetworkAPI
	a.once.Do(func() {
		sc, err := purego.Dlopen("/System/Library/Frameworks/SystemConfiguration.framework/SystemConfiguration", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			return
		}
		cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			return
		}
		bindings := []struct {
			handle uintptr
			name   string
			target any
		}{
			{sc, "SCDynamicStoreCopyValue", &a.copyValue}, {cf, "CFStringCreateWithCString", &a.makeString},
			{cf, "CFDictionaryGetValue", &a.dictionaryValue}, {cf, "CFGetTypeID", &a.typeID},
			{cf, "CFDictionaryGetTypeID", &a.dictionaryType}, {cf, "CFStringGetTypeID", &a.stringType},
			{cf, "CFStringGetCString", &a.stringBytes}, {cf, "CFRelease", &a.release},
		}
		for _, b := range bindings {
			symbol, err := purego.Dlsym(b.handle, b.name)
			if err != nil {
				return
			}
			purego.RegisterFunc(b.target, symbol)
		}
		a.ready = true
	})
	result := map[string]bool{}
	if !a.ready {
		return result
	}
	field := a.makeString(0, "PrimaryInterface", 0x08000100)
	if field == 0 {
		return result
	}
	defer a.release(field)
	for _, family := range []string{"IPv4", "IPv6"} {
		key := a.makeString(0, "State:/Network/Global/"+family, 0x08000100)
		if key == 0 {
			continue
		}
		state := a.copyValue(0, key)
		a.release(key)
		if state == 0 {
			continue
		}
		if a.typeID(state) == a.dictionaryType() {
			value := a.dictionaryValue(state, field)
			if value != 0 && a.typeID(value) == a.stringType() {
				buffer := make([]byte, 256)
				if a.stringBytes(value, &buffer[0], int64(len(buffer)), 0x08000100) {
					result[string(bytes.TrimRight(buffer, "\x00"))] = true
				}
			}
		}
		a.release(state)
	}
	return result
}
