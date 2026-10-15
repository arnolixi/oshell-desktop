package ftp

import (
	"testing"
	"time"
)

func TestUNIXMetadata(t *testing.T) {
	e, err := parseRFC3659ListLine("type=file;unix.mode=4750;unix.uid=10;unix.ownername=alice;unix.gid=20;unix.groupname=staff; sample", time.Now(), time.UTC)
	if err != nil || !e.UNIXModeKnown || e.UNIXMode != 04750 || e.UNIXOwner != "alice" || e.UNIXGroup != "staff" {
		t.Fatalf("%+v %v", e, err)
	}
	e, err = parseLsListLine("-rwsr-x--- 1 alice staff 42 Jan 01 2026 file.txt", time.Now(), time.UTC)
	if err != nil || e.UNIXMode != 04750 || e.UNIXOwner != "alice" || e.UNIXGroup != "staff" {
		t.Fatalf("%+v %v", e, err)
	}
	e, err = parseRFC3659ListLine("type=file;size=3; sample", time.Now(), time.UTC)
	if err != nil || e.UNIXModeKnown || e.UNIXOwner != "" {
		t.Fatalf("invented metadata: %+v %v", e, err)
	}
}
