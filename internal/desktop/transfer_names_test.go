package desktop

import "testing"

func TestTransferNamesMatchOriginalNumbering(t *testing.T) {
	names := map[string]bool{"report.txt": true, "report.txt.1": true, "目录": true}
	for _, tc := range []struct{ input, want string }{{"report.txt", "report.txt.2"}, {"report.txt", "report.txt.3"}, {"目录", "目录.1"}, {"new.txt", "new.txt"}, {"new.txt", "new.txt.1"}} {
		if got := uniqueTransferName(tc.input, names); got != tc.want {
			t.Fatalf("%s -> %s want %s", tc.input, got, tc.want)
		}
	}
}
