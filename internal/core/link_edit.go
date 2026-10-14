package core

import (
	"fmt"
	"strings"
	"unicode"
)

func (c *Config) UpdateLink(id, name, folder string) error {
	index := -1
	for i := range c.Links {
		if c.Links[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("快捷引用已不存在")
	}
	entry := c.Links[index]
	var profile *Profile
	for i := range c.Profiles {
		if c.Profiles[i].ID == entry.ProfileID {
			profile = &c.Profiles[i]
			break
		}
	}
	if profile == nil {
		return fmt.Errorf("原会话已不存在")
	}
	folder, e := NormalizeDirectory(folder)
	if e != nil {
		return e
	}
	exists := folder == ""
	for _, directory := range c.AllDirectories() {
		exists = exists || directory == "Links/"+folder
	}
	if !exists {
		return fmt.Errorf("目标快捷目录已不存在")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = profile.Name
	}
	if len(name) > 1024 || strings.ContainsFunc(name, unicode.IsControl) {
		return fmt.Errorf("链接名称无效或超过 1 KB")
	}
	for _, other := range c.Links {
		if other.ID != id && other.ProfileID == entry.ProfileID && other.Folder == folder {
			return fmt.Errorf("目标目录已存在此会话的快捷引用")
		}
	}
	c.rememberLinkFolder(c.Links[index].Folder)
	c.Links[index].Name = name
	c.Links[index].Folder = folder
	c.rememberLinkFolder(folder)
	c.NormalizeLinkRootOrder()
	return nil
}
func (c *Config) RemoveLink(id string) bool {
	for i := range c.Links {
		if c.Links[i].ID == id {
			c.rememberLinkFolder(c.Links[i].Folder)
			c.Links = append(c.Links[:i], c.Links[i+1:]...)
			c.NormalizeLinkRootOrder()
			return true
		}
	}
	return false
}
func LinkFolderChild(parent, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || len(name) > 255 || strings.ContainsAny(name, "/\\") || strings.ContainsFunc(name, unicode.IsControl) {
		return "", fmt.Errorf("请填写单个文件夹名称，最多 255 字节，不能包含斜杠或控制字符")
	}
	if parent == "" {
		return name, nil
	}
	return parent + "/" + name, nil
}
func (c *Config) RemoveLinkFolder(folder string) error {
	if folder == "" {
		return fmt.Errorf("不能删除 Links 根目录")
	}
	for _, profile := range c.Profiles {
		if Within(profile.Group, "Links/"+folder) {
			return fmt.Errorf("该目录包含实际会话，请在会话管理中删除并确认范围")
		}
	}
	return c.DeleteDirectory("Links/" + folder)
}
