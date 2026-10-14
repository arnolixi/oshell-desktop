//go:build darwin

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func resolveLocalTool(requested, canonical string) (string, error) {
	if filepath.IsAbs(requested) {
		return findToolInRoots(requested, nil)
	}
	roots := []string{"/usr/bin", "/bin", "/usr/sbin", "/sbin", "/opt/homebrew/bin", "/opt/homebrew/sbin", "/usr/local/bin", "/usr/local/sbin"}
	roots = append(roots, strings.Split(os.Getenv("PATH"), ":")...)
	return findToolInRoots(canonical, roots)
}
func findToolInRoots(name string, roots []string) (string, error) {
	var candidates []string
	if filepath.IsAbs(name) {
		candidates = []string{name}
	} else {
		seen := map[string]bool{}
		for _, root := range roots {
			if filepath.IsAbs(root) && !seen[root] {
				seen[root] = true
				candidates = append(candidates, filepath.Join(root, name))
			}
		}
	}
	for _, candidate := range candidates {
		executable, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}
		info, err := os.Stat(executable)
		if err == nil && info.Mode().IsRegular() {
			return executable, nil
		}
	}
	return "", fmt.Errorf("本机未找到可执行的 %s", filepath.Base(name))
}
