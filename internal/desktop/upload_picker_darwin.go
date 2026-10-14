//go:build darwin

package desktop

import (
	"fmt"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
	"unsafe"
)

type uploadPickerResult struct {
	paths []string
	err   error
}

// The pinned public MyGo dialog options select files OR directories. AppKit
// supports both, as required by the original OShell upload picker.
const supportsMixedUploadSelection = true

func chooseUploadItems(parent *mygo.Window, directoryOnly bool) ([]string, error) {
	result := make(chan uploadPickerResult, 1)
	mygo.RunOnMain(func() {
		if parent == nil || parent.NativeHandle() == 0 {
			result <- uploadPickerResult{err: fmt.Errorf("上传窗口已关闭")}
			return
		}
		pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("init"))
		defer pool.Send(objc.RegisterName("drain"))
		panel := objc.ID(objc.GetClass("NSOpenPanel")).Send(objc.RegisterName("openPanel"))
		if panel == 0 {
			result <- uploadPickerResult{err: fmt.Errorf("无法打开上传选择器")}
			return
		}
		panel.Send(objc.RegisterName("retain"))
		panel.Send(objc.RegisterName("setTitle:"), appearanceString("选择上传文件或目录"))
		panel.Send(objc.RegisterName("setCanChooseFiles:"), !directoryOnly)
		panel.Send(objc.RegisterName("setCanChooseDirectories:"), true)
		panel.Send(objc.RegisterName("setAllowsMultipleSelection:"), true)
		panel.Send(objc.RegisterName("setTreatsFilePackagesAsDirectories:"), false)
		completion := objc.NewBlock(func(_ objc.Block, response int) {
			defer panel.Send(objc.RegisterName("release"))
			var paths []string
			if response == 1 {
				urls := panel.Send(objc.RegisterName("URLs"))
				count := objc.Send[uint64](urls, objc.RegisterName("count"))
				for i := uint64(0); i < count; i++ {
					value := urls.Send(objc.RegisterName("objectAtIndex:"), i).Send(objc.RegisterName("path"))
					bytes := objc.Send[*byte](value, objc.RegisterName("UTF8String"))
					length := objc.Send[uint64](value, objc.RegisterName("lengthOfBytesUsingEncoding:"), uint64(4))
					if bytes != nil {
						paths = append(paths, string(unsafe.Slice(bytes, int(length))))
					}
				}
			}
			result <- uploadPickerResult{paths: paths}
		})
		panel.Send(objc.RegisterName("beginSheetModalForWindow:completionHandler:"), objc.ID(parent.NativeHandle()), completion)
		completion.Release() // AppKit retains the copied completion block until dismissal.
	})
	selected := <-result
	return selected.paths, selected.err
}
