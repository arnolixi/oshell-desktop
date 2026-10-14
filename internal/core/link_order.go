package core

import (
	"fmt"
	"strings"
)

type LinkRootItem struct{ Kind, ID, Folder, Name, ProfileID string }

func linkIdentity(id string) string {
	if uuidText.MatchString(id) {
		return strings.ToUpper(id)
	}
	return id
}
func (item LinkRootItem) Key() string {
	if item.Kind == "folder" {
		return "folder:" + item.Folder
	}
	return item.Kind + ":" + linkIdentity(item.ID)
}
func canonicalLinkRootKey(key string) string {
	kind, id, ok := strings.Cut(key, ":")
	if ok && (kind == "link" || kind == "session") {
		return kind + ":" + linkIdentity(id)
	}
	return key
}
func (c Config) OrderedLinkRootItems() []LinkRootItem {
	defaults := []LinkRootItem{}
	for _, directory := range c.AllDirectories() {
		if strings.HasPrefix(directory, "Links/") && strings.Count(directory, "/") == 1 {
			folder := strings.TrimPrefix(directory, "Links/")
			if folder == "" {
				continue
			}
			defaults = append(defaults, LinkRootItem{Kind: "folder", Folder: folder, Name: folder})
		}
	}
	valid := map[string]bool{}
	for _, p := range c.Profiles {
		valid[linkIdentity(p.ID)] = true
	}
	for _, link := range c.Links {
		if link.Folder == "" && valid[linkIdentity(link.ProfileID)] {
			defaults = append(defaults, LinkRootItem{Kind: "link", ID: link.ID, Name: link.Name, ProfileID: link.ProfileID})
		}
	}
	for _, profile := range c.Profiles {
		if profile.Group == "Links" {
			defaults = append(defaults, LinkRootItem{Kind: "session", ID: profile.ID, Name: profile.Name, ProfileID: profile.ID})
		}
	}
	available := map[string]LinkRootItem{}
	for _, item := range defaults {
		available[item.Key()] = item
	}
	result := make([]LinkRootItem, 0, len(defaults))
	seen := map[string]bool{}
	add := func(key string) {
		key = canonicalLinkRootKey(key)
		if item, ok := available[key]; ok && !seen[key] {
			seen[key] = true
			result = append(result, item)
		}
	}
	for _, key := range c.LinkRootOrder {
		add(key)
	}
	for _, item := range defaults {
		add(item.Key())
	}
	return result
}
func (c *Config) NormalizeLinkRootOrder() {
	if len(c.LinkRootOrder) == 0 {
		return
	}
	items := c.OrderedLinkRootItems()
	c.LinkRootOrder = make([]string, 0, len(items))
	for _, item := range items {
		c.LinkRootOrder = append(c.LinkRootOrder, item.Key())
	}
}
func (c *Config) ReorderLinkRoot(source, before string) (bool, error) {
	source, before = canonicalLinkRootKey(source), canonicalLinkRootKey(before)
	current := c.OrderedLinkRootItems()
	sourceFound, targetFound := false, before == ""
	for _, item := range current {
		sourceFound = sourceFound || item.Key() == source
		targetFound = targetFound || item.Key() == before
	}
	if !sourceFound || !targetFound {
		return false, fmt.Errorf("链接栏项目已变化，请重新拖动")
	}
	if source == before {
		return false, nil
	}
	ordered := make([]string, 0, len(current))
	for _, item := range current {
		key := item.Key()
		if key == source {
			continue
		}
		if key == before {
			ordered = append(ordered, source)
		}
		ordered = append(ordered, key)
	}
	if before == "" {
		ordered = append(ordered, source)
	}
	same := len(ordered) == len(current)
	for i := range ordered {
		if ordered[i] != current[i].Key() {
			same = false
			break
		}
	}
	if same {
		return false, nil
	}
	c.LinkRootOrder = ordered
	return true, nil
}
func (c *Config) rememberLinkFolder(folder string) {
	if folder == "" {
		return
	}
	known := map[string]bool{}
	for _, d := range c.Directories {
		known[d] = true
	}
	parent := "Links"
	for _, part := range strings.Split(folder, "/") {
		parent += "/" + part
		if !known[parent] {
			c.Directories = append(c.Directories, parent)
			known[parent] = true
		}
	}
}

func (c *Config) MoveLinkRootIntoFolder(source, folder string) (bool, error) {
	source = canonicalLinkRootKey(source)
	var selected *LinkRootItem
	for _, item := range c.OrderedLinkRootItems() {
		if item.Key() == source {
			copy := item
			selected = &copy
			break
		}
	}
	if selected == nil || selected.Kind == "folder" {
		return false, fmt.Errorf("拖动来源已变化或不是会话项目")
	}
	valid := false
	for _, directory := range c.AllDirectories() {
		valid = valid || directory == "Links/"+folder
	}
	if !valid || folder == "" {
		return false, fmt.Errorf("快捷目录已不存在")
	}
	switch selected.Kind {
	case "link":
		for _, entry := range c.Links {
			if entry.ProfileID == selected.ProfileID && entry.Folder == folder {
				return false, fmt.Errorf("目标目录已存在此会话的快捷引用")
			}
		}
		for i := range c.Links {
			if c.Links[i].ID == selected.ID {
				c.Links[i].Folder = folder
				break
			}
		}
	case "session":
		for i := range c.Profiles {
			if c.Profiles[i].ID == selected.ID {
				c.Profiles[i].Group = "Links/" + folder
				break
			}
		}
	default:
		return false, fmt.Errorf("拖动来源无效")
	}
	c.rememberLinkFolder(folder)
	c.NormalizeLinkRootOrder()
	return true, nil
}
