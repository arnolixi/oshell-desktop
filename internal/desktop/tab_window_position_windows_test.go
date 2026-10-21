package desktop

import (
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

func TestDetachedWindowPositionUsesExactScreenOrigin(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// Exercise real Win32 positioning on our own hidden window. This includes
	// valid negative coordinates and the origin, which must not mean center.
	class, _ := syscall.UTF16PtrFromString("STATIC")
	handle, _, err := tabPositionUser32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(class)), 0, 0x00CF0000, 40, 40, 320, 240, 0, 0, 0, 0)
	if handle == 0 {
		t.Fatal(err)
	}
	defer tabPositionUser32.NewProc("DestroyWindow").Call(handle)
	getRect := tabPositionUser32.NewProc("GetWindowRect")
	for _, position := range []tabWindowPosition{{480, 320}, {0, 0}, {-1200, 200}, {500, -600}} {
		if err := position.applyHandle(handle); err != nil {
			t.Fatal(err)
		}
		var rect struct{ left, top, right, bottom int32 }
		if ok, _, err := getRect.Call(handle, uintptr(unsafe.Pointer(&rect))); ok == 0 {
			t.Fatal(err)
		}
		if int(rect.left) != position.x || int(rect.top) != position.y {
			t.Fatalf("window origin = (%d,%d), want %+v", rect.left, rect.top, position)
		}
		if rect.right-rect.left != 320 || rect.bottom-rect.top != 240 {
			t.Fatal("positioning resized the window")
		}
	}
}
