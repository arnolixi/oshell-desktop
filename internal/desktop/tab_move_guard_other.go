//go:build !darwin

package desktop

import "github.com/egoist/mygo"

func nativeTabMoveDialogOpen(*mygo.Window) bool { return false }
