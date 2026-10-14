package core

import "testing"

func TestMainExecutableCompatibilityRouting(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, ""}, {[]string{"--external-launch-service"}, ""}, {[]string{"--mygo-generate"}, ""},
		{[]string{"/SSH:fixture@example.invalid"}, "OShell-ZOC"},
		{[]string{"-SSH=fixture@example.invalid"}, "OShell-ZOC"},
		{[]string{"/CONNECT:SSH!fixture@example.invalid"}, "OShell-ZOC"},
		{[]string{"sftp://fixture@example.invalid/"}, "OShell-FileZilla"},
		{[]string{"FTP://fixture@example.invalid/"}, "OShell-FileZilla"},
		{[]string{"--site=0/folder/site"}, "OShell-FileZilla"},
		{[]string{"-c", "0/site"}, "OShell-FileZilla"},
	} {
		if got := CompatibilityLauncher(tc.args); got != tc.want {
			t.Fatalf("route %q != %q", got, tc.want)
		}
	}
}
