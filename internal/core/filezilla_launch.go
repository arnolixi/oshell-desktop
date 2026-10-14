package core

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

func FileZillaSitePath(path string) ([]string, error) {
	fail := func() ([]string, error) { return nil, fmt.Errorf("仅支持有效用户站点路径 0/目录/站点") }
	if !strings.HasPrefix(path, "0/") || len(path) > 4096 {
		return fail()
	}
	parts := []string{}
	var current strings.Builder
	escaped := false
	for _, r := range path[2:] {
		if escaped {
			if r != '/' && r != '\\' {
				return fail()
			}
			current.WriteRune(r)
			escaped = false
		} else if r == '\\' {
			escaped = true
		} else if r == '/' {
			parts = append(parts, current.String())
			current.Reset()
		} else {
			current.WriteRune(r)
		}
	}
	parts = append(parts, current.String())
	if escaped || len(parts) > 32 {
		return fail()
	}
	for _, part := range parts {
		if part == "" || strings.ContainsFunc(part, unicode.IsControl) {
			return fail()
		}
	}
	return parts, nil
}

type launchXMLNode struct {
	name, text string
	attrs      []xml.Attr
	children   []*launchXMLNode
}

func (n *launchXMLNode) named(name string) []*launchXMLNode {
	var out []*launchXMLNode
	for _, child := range n.children {
		if child.name == name {
			out = append(out, child)
		}
	}
	return out
}
func launchXML(data []byte) (*launchXMLNode, error) {
	if len(data) > 8<<20 || !utf8.Valid(data) {
		return nil, fmt.Errorf("站点配置过大或编码无效")
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	root := &launchXMLNode{}
	stack := []*launchXMLNode{root}
	count := 0
	for {
		token, e := decoder.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, fmt.Errorf("站点 XML 无效")
		}
		switch token := token.(type) {
		case xml.Directive:
			return nil, fmt.Errorf("不支持站点 XML 声明")
		case xml.StartElement:
			count++
			if count > 100000 || len(stack) > 128 || token.Name.Space != "" {
				return nil, fmt.Errorf("站点 XML 结构无效")
			}
			node := &launchXMLNode{name: token.Name.Local, attrs: token.Attr}
			parent := stack[len(stack)-1]
			parent.children = append(parent.children, node)
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) <= 1 {
				return nil, fmt.Errorf("站点 XML 无效")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			stack[len(stack)-1].text += string(token)
		}
	}
	if len(root.children) != 1 || root.children[0].name != "FileZilla3" {
		return nil, fmt.Errorf("缺少 FileZilla 站点列表")
	}
	return root.children[0], nil
}
func ParseFileZillaSite(path string, data []byte) (FileLaunchRequest, error) {
	r := FileLaunchRequest{Version: 1, ID: NewID()}
	parts, e := FileZillaSitePath(path)
	if e != nil {
		return r, e
	}
	root, e := launchXML(data)
	if e != nil {
		return r, e
	}
	roots := root.named("Servers")
	if len(roots) != 1 {
		return r, fmt.Errorf("缺少或重复站点列表")
	}
	folders := roots
	for _, part := range parts[:len(parts)-1] {
		var next []*launchXMLNode
		for _, folder := range folders {
			for _, child := range folder.named("Folder") {
				if strings.TrimSpace(child.text) == part {
					next = append(next, child)
				}
			}
		}
		folders = next
	}
	var entries []*launchXMLNode
	for _, folder := range folders {
		for _, entry := range folder.named("Server") {
			name := strings.TrimSpace(entry.text)
			names := entry.named("Name")
			if len(names) > 0 {
				name = names[0].text
			}
			if name == parts[len(parts)-1] {
				entries = append(entries, entry)
			}
		}
	}
	if len(entries) != 1 {
		return r, fmt.Errorf("站点不存在或名称重复")
	}
	entry := entries[0]
	for _, key := range []string{"Host", "Port", "Protocol", "Logontype", "User", "Pass", "Keyfile", "Name"} {
		if len(entry.named(key)) > 1 {
			return r, fmt.Errorf("站点包含重复字段")
		}
	}
	value := func(key string) string {
		nodes := entry.named(key)
		if len(nodes) == 0 {
			return ""
		}
		if len(nodes[0].children) > 0 {
			return ""
		}
		return nodes[0].text
	}
	protocol, login := value("Protocol"), value("Logontype")
	if protocol != "0" && protocol != "1" {
		return r, fmt.Errorf("只支持普通 FTP 或 SFTP")
	}
	switch login {
	case "0", "1", "2", "3", "5":
	default:
		return r, fmt.Errorf("不支持该登录类型")
	}
	if protocol == "0" && login == "5" {
		return r, fmt.Errorf("密钥登录仅适用于 SFTP")
	}
	p := NewProfile()
	p.Name = "文件连接"
	p.Group = ""
	p.Kind = "ftp"
	p.Port = 21
	if protocol == "1" {
		p.Kind = "sftp"
		p.Port = 22
	}
	p.Host = strings.Trim(value("Host"), "[]")
	p.Username = value("User")
	p.IdentityFile = value("Keyfile")
	p.QuickConnect = false
	p.InitialDirectory = "."
	if port := value("Port"); port != "" {
		p.Port, e = strconv.Atoi(port)
		if e != nil {
			return r, fmt.Errorf("站点端口无效")
		}
	}
	if login == "0" {
		p.Username = "anonymous"
		password := "anonymous@"
		r.Password = &password
	} else if passes := entry.named("Pass"); len(passes) == 1 && login != "2" && login != "3" {
		encoding := ""
		found := false
		for _, attr := range passes[0].attrs {
			if attr.Name.Local == "encoding" {
				if found {
					return r, fmt.Errorf("密码编码字段重复")
				}
				found = true
				encoding = attr.Value
			}
		}
		password := passes[0].text
		switch encoding {
		case "":
		case "base64":
			raw, e := base64.StdEncoding.DecodeString(strings.Map(func(r rune) rune {
				if unicode.IsSpace(r) {
					return -1
				}
				return r
			}, password))
			if e != nil || !utf8.Valid(raw) {
				return r, fmt.Errorf("站点密码编码无效")
			}
			password = string(raw)
		default:
			return r, fmt.Errorf("不支持 FileZilla 专有密码加密")
		}
		r.Password = &password
	}
	r.Profile = p
	return r, r.Validate()
}
