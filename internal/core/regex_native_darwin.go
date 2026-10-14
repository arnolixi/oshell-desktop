//go:build darwin

package core

import (
	"fmt"
	"github.com/ebitengine/purego"
	"runtime"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"
)

var icu struct {
	once                  sync.Once
	err                   error
	open                  func(*uint16, int32, uint32, uintptr, *int32) uintptr
	close                 func(uintptr)
	setText               func(uintptr, *uint16, int32, *int32)
	findNext              func(uintptr, *int32) int8
	start, end            func(uintptr, int32, *int32) int32
	timeLimit, stackLimit func(uintptr, int32, *int32)
}
var emptyRegexText [1]uint16

func loadICU() {
	icu.once.Do(func() {
		handle, e := purego.Dlopen("/usr/lib/libicucore.A.dylib", purego.RTLD_NOW|purego.RTLD_LOCAL)
		if e != nil {
			icu.err = e
			return
		}
		register := func(target any, name string) {
			address, e := purego.Dlsym(handle, name)
			if e != nil {
				icu.err = e
				return
			}
			purego.RegisterFunc(target, address)
		}
		register(&icu.open, "uregex_open")
		register(&icu.close, "uregex_close")
		register(&icu.setText, "uregex_setText")
		register(&icu.findNext, "uregex_findNext")
		register(&icu.start, "uregex_start")
		register(&icu.end, "uregex_end")
		register(&icu.timeLimit, "uregex_setTimeLimit")
		register(&icu.stackLimit, "uregex_setStackLimit")
	})
}

type nativePattern struct {
	mu     sync.Mutex
	handle uintptr
}

func compileNativePattern(source string, budget time.Duration) (nativeMatcher, bool, error) {
	loadICU()
	if icu.err != nil {
		return nil, true, fmt.Errorf("无法加载系统 ICU 正则引擎：%w", icu.err)
	}
	words := utf16.Encode([]rune(source))
	if len(words) == 0 {
		words = []uint16{0}
	}
	var status int32
	handle := icu.open(&words[0], int32(len(utf16.Encode([]rune(source)))), 0, 0, &status)
	runtime.KeepAlive(words)
	if status > 0 || handle == 0 {
		return nil, true, fmt.Errorf("正则表达式无效（ICU %d）", status)
	}
	if budget <= 0 {
		budget = 2 * time.Millisecond
	}
	icu.timeLimit(handle, int32(max(1, budget.Milliseconds())), &status)
	icu.stackLimit(handle, 4<<20, &status)
	if status > 0 {
		icu.close(handle)
		return nil, true, fmt.Errorf("无法设置正则资源限制")
	}
	p := &nativePattern{handle: handle}
	runtime.SetFinalizer(p, (*nativePattern).Close)
	return p, true, nil
}
func (p *nativePattern) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle != 0 {
		icu.close(p.handle)
		p.handle = 0
	}
	runtime.SetFinalizer(p, nil)
}
func (p *nativePattern) FindAllStringIndex(text string, limit int) ([][]int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == 0 {
		return nil, fmt.Errorf("正则表达式已释放")
	}
	words := utf16.Encode([]rune(text))
	if len(words) == 0 {
		words = []uint16{0}
	}
	var pin runtime.Pinner
	pin.Pin(unsafe.Pointer(&words[0]))
	defer pin.Unpin()
	var status int32
	icu.setText(p.handle, &words[0], int32(len(utf16.Encode([]rune(text)))), &status)
	defer func() { var e int32; icu.setText(p.handle, &emptyRegexText[0], 0, &e); runtime.KeepAlive(words) }()
	offsets := make([]int, 0, len(words)+1)
	for at, r := range text {
		offsets = append(offsets, at)
		if r > 0xffff {
			offsets = append(offsets, at)
		}
	}
	offsets = append(offsets, len(text))
	var out [][]int
	for (limit < 0 || len(out) < limit) && status <= 0 && icu.findNext(p.handle, &status) != 0 {
		start, end := int(icu.start(p.handle, 0, &status)), int(icu.end(p.handle, 0, &status))
		if status > 0 {
			break
		}
		if start >= 0 && end >= start && end < len(offsets) {
			out = append(out, []int{offsets[start], offsets[end]})
		}
	}
	if status > 0 {
		return nil, fmt.Errorf("正则匹配超时或超过资源限制（ICU %d）", status)
	}
	return out, nil
}
