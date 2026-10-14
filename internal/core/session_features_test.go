package core

import "testing"

func TestDirectoryAndLinksLifecycle(t *testing.T) {
	c := DefaultConfig()
	p := NewProfile()
	p.Group = "生产/数据库"
	p.Host = "example.invalid"
	p.Username = "dev"
	c.Profiles = []Profile{p}
	if e := c.AddDirectory("空目录"); e != nil {
		t.Fatal(e)
	}
	if e := c.AddLink(p, "常用"); e != nil {
		t.Fatal(e)
	}
	if e := c.AddDirectory("归档"); e != nil {
		t.Fatal(e)
	}
	if e := c.MoveDirectory("生产", "归档/生产"); e != nil {
		t.Fatal(e)
	}
	if c.Profiles[0].Group != "归档/生产/数据库" {
		t.Fatal(c.Profiles[0])
	}
	if e := c.MoveDirectory("归档", "归档/子目录"); e == nil {
		t.Fatal("moved into descendant")
	}
	if e := c.DeleteDirectory("Links/常用"); e != nil {
		t.Fatal(e)
	}
	if len(c.Profiles) != 1 || len(c.Links) != 0 {
		t.Fatal("deleting link deleted source")
	}
	c.AddLink(c.Profiles[0], "")
	if e := c.DeleteDirectory("归档"); e != nil {
		t.Fatal(e)
	}
	if len(c.Profiles) != 0 || len(c.Links) != 0 {
		t.Fatal("dangling profiles or references")
	}
}
func TestSearchUnicodeRegexAndLimit(t *testing.T) {
	m, limited, e := Search("中文错误 ERROR\n无错误\nerror", "错误|error", true, false)
	if e != nil || limited || len(m) != 4 {
		t.Fatal(m, limited, e)
	}
	if m[0].Start != 2 || m[0].End != 4 {
		t.Fatal(m[0])
	}
	if _, _, e = Search("x", "[", true, false); e == nil {
		t.Fatal("invalid regex accepted")
	}
	m, _, e = Search("a+b\naab", "a+b", false, true)
	if e != nil || len(m) != 1 {
		t.Fatal(m, e)
	}
}
