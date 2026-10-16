package core

import (
	"fmt"
	"path/filepath"
	"strings"
)

func validateStorageSeparation(source, target string) error {
	// Paths on different Windows volumes cannot contain each other. Rel
	// returns an error for them, not a relative path to inspect.
	if !strings.EqualFold(filepath.VolumeName(source), filepath.VolumeName(target)) {
		return nil
	}
	outside := func(p string) bool { return p == ".." || strings.HasPrefix(p, ".."+string(filepath.Separator)) }
	for _, pair := range [][2]string{{source, target}, {target, source}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err != nil {
			return fmt.Errorf("无法比较数据目录：%w", err)
		}
		if !outside(rel) {
			return fmt.Errorf("新目录不能是当前目录的父目录或子目录")
		}
	}
	return nil
}
