package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalTransferNamesRespectDestinationCaseRules(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"Report.txt", "REPORT.txt.1"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("keep"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	want := "report.txt"
	if _, err := os.Lstat(filepath.Join(dir, want)); err == nil {
		want = "report.txt.2" // default Windows and macOS filesystems
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	got, err := uniqueLocalTransferPath(dir, "report.txt")
	if err != nil || filepath.Base(got) != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
	if err := os.WriteFile(got, []byte("download"), 0600); err != nil {
		t.Fatal(err)
	}
	next, err := uniqueLocalTransferPath(dir, "report.txt")
	if err != nil || next == got {
		t.Fatalf("reused destination %q: %v", next, err)
	}
}

func TestTransferNamesMatchOriginalNumbering(t *testing.T) {
	names := map[string]bool{"report.txt": true, "report.txt.1": true, "目录": true}
	for _, tc := range []struct{ input, want string }{{"report.txt", "report.txt.2"}, {"report.txt", "report.txt.3"}, {"目录", "目录.1"}, {"new.txt", "new.txt"}, {"new.txt", "new.txt.1"}} {
		if got := uniqueTransferName(tc.input, names); got != tc.want {
			t.Fatalf("%s -> %s want %s", tc.input, got, tc.want)
		}
	}
}
