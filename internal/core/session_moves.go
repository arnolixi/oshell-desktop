package core

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// MoveSessionItems applies a complete selection atomically. Drops into Links
// create references for ordinary sessions/directories, leaving their sources.
func (c *Config) MoveSessionItems(profiles, directories, links []string, destination string, createReferences ...bool) error {
	references := len(createReferences) == 0 || createReferences[0]
	if !references && (len(directories) > 0 || len(links) > 0) {
		return fmt.Errorf("移动实际会话只支持会话配置")
	}
	dest, err := NormalizeDirectory(destination)
	if err != nil {
		return err
	}
	existing := map[string]bool{"": true}
	for _, d := range c.AllDirectories() {
		existing[d] = true
	}
	if !existing[dest] {
		return fmt.Errorf("目标目录已不存在")
	}
	ids, linkIDs, dirs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, id := range profiles {
		ids[id] = true
	}
	for _, id := range links {
		linkIDs[id] = true
	}
	for _, d := range directories {
		if d == "" || d == "Links" || !existing[d] {
			return fmt.Errorf("目录已变化或不能移动根目录")
		}
		dirs[d] = true
	}
	for id := range ids {
		found := false
		for _, p := range c.Profiles {
			found = found || p.ID == id
		}
		if !found {
			return fmt.Errorf("会话已不存在")
		}
	}
	for id := range linkIDs {
		found := false
		for _, l := range c.Links {
			found = found || l.ID == id
		}
		if !found {
			return fmt.Errorf("快捷引用已不存在")
		}
	}
	roots := []string{}
	for d := range dirs {
		covered := false
		for parent := range dirs {
			if parent != d && Within(d, parent) {
				covered = true
				break
			}
		}
		if !covered {
			roots = append(roots, d)
		}
	}
	sort.Strings(roots)
	intoLinks := Within(dest, "Links")
	if !intoLinks && len(linkIDs) > 0 {
		return fmt.Errorf("快捷引用只能在 Links 内移动")
	}
	for _, d := range roots {
		if Within(dest, d) {
			return fmt.Errorf("不能移入自身或子目录")
		}
		if Within(d, "Links") && !intoLinks {
			return fmt.Errorf("快捷目录只能在 Links 内移动")
		}
	}
	out := c.Clone()
	covered := func(group string) bool {
		for _, root := range roots {
			if Within(group, root) {
				return true
			}
		}
		return false
	}
	addReference := func(profile Profile, target string) error {
		folder := strings.TrimPrefix(strings.TrimPrefix(target, "Links"), "/")
		for _, l := range out.Links {
			if l.ProfileID == profile.ID && l.Folder == folder {
				return nil
			}
		}
		return out.AddLink(profile, folder)
	}
	for _, from := range roots {
		to := path.Join(dest, path.Base(from))
		if to == from {
			continue
		}
		if references && intoLinks && !Within(from, "Links") {
			for _, d := range out.AllDirectories() {
				if d == to {
					return fmt.Errorf("目标已有同名目录")
				}
			}
			for _, d := range c.AllDirectories() {
				if Within(d, from) {
					if err := out.AddDirectory(to + strings.TrimPrefix(d, from)); err != nil {
						return err
					}
				}
			}
			for _, p := range c.Profiles {
				if Within(p.Group, from) {
					if err := addReference(p, to+strings.TrimPrefix(p.Group, from)); err != nil {
						return err
					}
				}
			}
		} else if err := out.MoveDirectory(from, to); err != nil {
			return err
		}
	}
	for _, p := range c.Profiles {
		if !ids[p.ID] || covered(p.Group) {
			continue
		}
		if references && intoLinks && !Within(p.Group, "Links") {
			if err := addReference(p, dest); err != nil {
				return err
			}
		} else {
			for i := range out.Profiles {
				if out.Profiles[i].ID == p.ID {
					out.Profiles[i].Group = dest
				}
			}
		}
	}
	for _, l := range c.Links {
		if !linkIDs[l.ID] || covered(path.Join("Links", l.Folder)) {
			continue
		}
		folder := strings.TrimPrefix(strings.TrimPrefix(dest, "Links"), "/")
		if err := out.UpdateLink(l.ID, l.Name, folder); err != nil {
			return err
		}
	}
	// Keep source parents visible even when their last session moves away.
	for _, d := range c.AllDirectories() {
		if !Within(d, "Links") && !covered(d) {
			out.Directories = append(out.Directories, d)
		}
	}
	paths := out.AllDirectories()
	out.Directories = nil
	for _, d := range paths {
		if d != "Links" {
			out.Directories = append(out.Directories, d)
		}
	}
	out.NormalizeLinkRootOrder()
	if err := out.Validate(); err != nil {
		return err
	}
	*c = out
	return nil
}
