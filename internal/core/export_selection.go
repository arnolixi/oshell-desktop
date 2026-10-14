package core

import "fmt"

// ExportSelection keeps the requested subtree/reference scope and only adds the
// source profiles required by selected references. Proxy closure is handled by
// PortableArchive, using the retained catalog.
func (c Config) ExportSelection(profileIDs, directories, linkIDs []string) (Config, error) {
	profiles, links := map[string]bool{}, map[string]bool{}
	for _, id := range profileIDs {
		profiles[id] = true
	}
	for _, id := range linkIDs {
		links[id] = true
	}
	allDirectories := c.AllDirectories()
	exists := map[string]bool{"": true}
	for _, dir := range allDirectories {
		exists[dir] = true
	}
	for _, dir := range directories {
		if !exists[dir] {
			return Config{}, fmt.Errorf("所选目录已不存在")
		}
	}
	within := func(group string) bool {
		for _, dir := range directories {
			if dir == "" || Within(group, dir) {
				return true
			}
		}
		return false
	}
	foundProfiles, foundLinks := map[string]bool{}, map[string]bool{}
	out := c.Clone()
	out.Profiles = nil
	out.Links = nil
	out.Directories = nil
	out.LinkRootOrder = nil
	for _, link := range c.Links {
		if links[link.ID] || within("Links"+func() string {
			if link.Folder == "" {
				return ""
			}
			return "/" + link.Folder
		}()) {
			out.Links = append(out.Links, link)
			profiles[link.ProfileID] = true
			foundLinks[link.ID] = true
		}
	}
	for _, profile := range c.Profiles {
		if profiles[profile.ID] || within(profile.Group) {
			out.Profiles = append(out.Profiles, profile)
			foundProfiles[profile.ID] = true
		}
	}
	for id := range profiles {
		if !foundProfiles[id] {
			return Config{}, fmt.Errorf("所选会话或引用源会话已不存在")
		}
	}
	for id := range links {
		if !foundLinks[id] {
			return Config{}, fmt.Errorf("所选快捷引用已不存在")
		}
	}
	for _, dir := range allDirectories {
		if within(dir) {
			out.Directories = append(out.Directories, dir)
		}
	}
	out.NormalizeLinkRootOrder()
	return out, nil
}
