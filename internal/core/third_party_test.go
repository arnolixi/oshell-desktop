package core

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXshellOriginalPasswordVectors(t *testing.T) {
	for _, v := range []struct{ cipher, master, plain string }{{"Rrm3P3AL0iDV7nBbS2bHvh7ZAvuN1NSJl8ZFL11+UJ+82+KAixa89O3OTAfRTg==", "123123", "This is a test"}, {"TaNuRIYAqRCMnEiseXgqJgwobNCbit204blttGz1BxMTz0hzI+Ok44+AT+NsLndE", "test-master-密码", "测试口令🔐"}} {
		p := XshellPassword{encoded: v.cipher}
		got, e := p.Decrypt(v.master)
		if e != nil || got != v.plain {
			t.Fatal(got, e)
		}
		if _, e = p.Decrypt("wrong"); e == nil {
			t.Fatal("wrong master accepted")
		}
		if strings.Contains(fmt.Sprintf("%v %#v", p, p), v.cipher) {
			t.Fatal("ciphertext leaked to diagnostics")
		}
	}
}
func TestThirdPartyINIAndSecureXML(t *testing.T) {
	dir := t.TempDir()
	xsh := filepath.Join(dir, "生产.xsh")
	os.WriteFile(xsh, []byte("[CONNECTION]\nProtocol=SSH\nHost=host.example\nPort=2222\n[CONNECTION:AUTHENTICATION]\nUserName=ops\nPassword=Rrm3P3AL0iDV7nBbS2bHvh7ZAvuN1NSJl8ZFL11+UJ+82+KAixa89O3OTAfRTg==\n"), 0600)
	report, e := ReadThirdParty(context.Background(), []string{xsh}, "Xshell")
	if e != nil || len(report.Profiles) != 1 || len(report.Passwords) != 1 {
		t.Fatal(report, e)
	}
	v, _ := OpenVault(t.TempDir())
	cfg, updates, _, e := report.Merge(context.Background(), DefaultConfig(), v, "Imported", true, false, "123123")
	if e != nil || len(updates) != 1 || updates[0].Password != "This is a test" || cfg.Profiles[0].Group != "Imported/Xshell" {
		t.Fatal(e)
	}
	file := filepath.Join(dir, "secure.xml")
	os.WriteFile(file, []byte(`<VanDyke><key name="Sessions"><key name="folder"><key name="host"><dword name="Is Session">1</dword><string name="Protocol Name">SSH2</string><string name="Hostname">example.test</string><string name="Username">ops</string><dword name="[SSH2] Port">22</dword></key></key></key></VanDyke>`), 0600)
	secure, e := ReadThirdParty(context.Background(), []string{file}, "SecureCRT")
	if e != nil || len(secure.Profiles) != 1 || secure.Profiles[0].Group != "SecureCRT/folder" {
		t.Fatal(secure, e)
	}
	os.WriteFile(file, []byte(`<!DOCTYPE VanDyke [<!ENTITY a SYSTEM "file:///etc/passwd">]><VanDyke/>`), 0600)
	if _, e = ReadThirdParty(context.Background(), []string{file}, "SecureCRT"); e == nil {
		t.Fatal("DTD accepted")
	}
	ini := filepath.Join(dir, "one.ini")
	os.WriteFile(ini, []byte("S:\"Protocol Name\"=SSH2\nS:\"Hostname\"=example.test\nS:\"Username\"=ops\nD:\"[SSH2] Port\"=00000016\n"), 0600)
	secure, e = ReadThirdParty(context.Background(), []string{ini}, "SecureCRT")
	if e != nil || len(secure.Profiles) != 1 || secure.Profiles[0].Port != 22 {
		t.Fatal(secure, e)
	}
}
func TestVendorZipRejectsTraversal(t *testing.T) {
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	f, _ := w.Create("../bad.xsh")
	f.Write([]byte("[CONNECTION]\nHost=host\nProtocol=SSH\n"))
	w.Close()
	file := filepath.Join(t.TempDir(), "bad.zip")
	os.WriteFile(file, b.Bytes(), 0600)
	if _, e := ReadThirdParty(context.Background(), []string{file}, "Xshell"); e == nil {
		t.Fatal("zip traversal accepted")
	}
}
func TestAdvancedRegexUnicodeAndTimeout(t *testing.T) {
	p, e := CompilePattern(`(?<=用户:)\p{L}+(?=!)`, 0)
	if e != nil {
		t.Fatal(e)
	}
	matches, e := p.FindAllStringIndex("用户:测试!", 10)
	if e != nil || len(matches) != 1 || "用户:测试!"[matches[0][0]:matches[0][1]] != "测试" {
		t.Fatal(matches, e)
	}
	p, e = CompilePattern(`(ab)\1`, 0)
	if e != nil {
		t.Fatal(e)
	}
	matches, e = p.FindAllStringIndex("abab", 10)
	if e != nil || len(matches) != 1 {
		t.Fatal(matches, e)
	}
	p, e = CompilePattern(`(?=(a+)+$)(a+)+$`, 0)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = p.FindAllStringIndex(strings.Repeat("a", 100)+"!", 1); e == nil {
		t.Fatal("backtracking was not bounded")
	}
}
