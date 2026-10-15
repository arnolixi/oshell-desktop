package sftp

import "testing"

func TestOwnerHintsRequireMatchingAttributes(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode uint32
		size uint64
		want int
	}{{"file", 0100644, 42, 2}, {"other", 0100644, 42, 0}, {"file", 0100600, 42, 0}, {"file", 0100644, 43, 0}} {
		s := &FileStat{Mode: tc.mode, Size: tc.size}
		retainOwnerNames(s, tc.name, "-rw-r--r-- 1 alice staff 42 Jan 01 2026 file")
		if len(s.Extended) != tc.want {
			t.Fatalf("%+v %+v", tc, s)
		}
	}
}
