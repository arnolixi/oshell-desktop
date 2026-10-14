package core

import "strings"

// CompatibilityLauncher matches the native app's legacy direct-executable
// entrypoints. The selected launcher's parser remains responsible for validation.
func CompatibilityLauncher(args []string) string {
	if len(args) == 0 {
		return ""
	}
	first := args[0]
	lower := strings.ToLower(first)
	if strings.HasPrefix(first, "--site") || first == "-c" || strings.HasPrefix(lower, "ftp://") || strings.HasPrefix(lower, "sftp://") {
		return "OShell-FileZilla"
	}
	for _, arg := range args {
		lower := strings.ToLower(arg)
		if strings.HasPrefix(arg, "/") || strings.HasPrefix(lower, "-ssh") || strings.HasPrefix(lower, "-connect") || strings.HasPrefix(lower, "-dev") {
			return "OShell-ZOC"
		}
	}
	return ""
}
