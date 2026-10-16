package terminal

import (
	"strings"
	"testing"
)

func TestWindowsEnvironmentOverrideIgnoresCase(t *testing.T) {
	t.Setenv("OSHELL_CASE_TEST", "old")
	env := environment([]string{"oshell_case_test=new", "Oshell_Case_Test=latest"})
	var values []string
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "OSHELL_CASE_TEST") {
			values = append(values, value)
		}
	}
	if len(values) != 1 || values[0] != "latest" {
		t.Fatalf("duplicate or stale environment: %q", values)
	}
}
