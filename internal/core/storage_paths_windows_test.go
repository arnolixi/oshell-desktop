package core

import "testing"

func TestStorageSeparationAcrossWindowsVolumes(t *testing.T) {
	for _, tc := range []struct {
		from, to string
		valid    bool
	}{
		{`C:\data`, `D:\data`, true},
		{`C:\data`, `c:\other`, true},
		{`C:\data`, `c:\DATA\child`, false},
		{`C:\data\child`, `C:\data`, false},
		{`C:\data`, `c:\DATA`, false},
		{`\\server\first\data`, `\\server\second\data`, true},
	} {
		if err := validateStorageSeparation(tc.from, tc.to); (err == nil) != tc.valid {
			t.Errorf("%s -> %s: %v", tc.from, tc.to, err)
		}
	}
}
