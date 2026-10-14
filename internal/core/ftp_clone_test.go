package core

import "testing"

func TestFTPSessionCloneOwnsIndependentCredentials(t *testing.T) {
	source := &FTPSession{profile: NewProfile(), password: []byte("fixture-password")}
	clone, err := source.Clone()
	if err != nil {
		t.Fatal(err)
	}
	source.Close()
	if string(clone.password) != "fixture-password" || clone.closed {
		t.Fatal("source close cleared clone")
	}
	if _, err = source.Clone(); err == nil {
		t.Fatal("closed session cloned")
	}
	clone.Close()
	if len(clone.password) != 0 {
		t.Fatal("clone retained password")
	}
}
