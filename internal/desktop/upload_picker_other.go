//go:build !darwin

package desktop

import "github.com/egoist/mygo"

const supportsMixedUploadSelection = false

func chooseUploadItems(parent *mygo.Window, directoryOnly bool) ([]string, error) {
	return mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: parent, Title: "选择上传内容", Directory: directoryOnly, Multiple: true})
}
