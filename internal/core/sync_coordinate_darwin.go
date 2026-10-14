//go:build darwin

package core

import (
	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"runtime"
	"sync"
)

var foundationLoader struct {
	sync.Once
	err error
}

func foundationReady() error {
	foundationLoader.Do(func() {
		_, foundationLoader.err = purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_NOW|purego.RTLD_LOCAL)
	})
	return foundationLoader.err
}
func foundationURL(path string) objc.ID {
	data := append([]byte(path), 0)
	text := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), &data[0])
	runtime.KeepAlive(data)
	return objc.ID(objc.GetClass("NSURL")).Send(objc.RegisterName("fileURLWithPath:"), text)
}
func platformSyncConflictCheck(path string) error {
	if foundationReady() != nil {
		return safeSyncFailure("directory")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	versions := objc.ID(objc.GetClass("NSFileVersion")).Send(objc.RegisterName("unresolvedConflictVersionsOfItemAtURL:"), foundationURL(path))
	if versions != 0 && objc.Send[uint64](versions, objc.RegisterName("count")) > 0 {
		return safeSyncFailure("file-conflict")
	}
	return nil
}
func coordinateSyncWrite(path string, write func() error) error {
	if foundationReady() != nil {
		return safeSyncFailure("directory")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	coordinator := objc.ID(objc.GetClass("NSFileCoordinator")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithFilePresenter:"), objc.ID(0))
	defer coordinator.Send(objc.RegisterName("release"))
	url := foundationURL(path)
	var failure objc.ID
	var result error
	called := false
	block := objc.NewBlock(func(_ objc.Block, actual objc.ID) {
		called = true
		if !objc.Send[bool](url, objc.RegisterName("isEqual:"), actual) {
			result = safeSyncFailure("directory")
			return
		}
		result = write()
	})
	defer block.Release()
	coordinator.Send(objc.RegisterName("coordinateWritingItemAtURL:options:error:byAccessor:"), url, uint64(8), &failure, block)
	if failure != 0 || !called {
		return safeSyncFailure("directory")
	}
	return result
}
