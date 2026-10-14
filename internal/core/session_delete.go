package core

import (
	"fmt"
	"sort"
)

type SessionDeletion struct {
	Profiles, Directories, Links int
	RemovedIDs                   []string
}

func (c Config) PlanSessionDeletion(profiles, dirs, links []string) (Config, SessionDeletion, error) {
	summary := SessionDeletion{}
	ids, paths, refs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	knownIDs, knownDirs, knownLinks := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, p := range c.Profiles {
		knownIDs[p.ID] = true
	}
	for _, d := range c.AllDirectories() {
		knownDirs[d] = true
	}
	for _, l := range c.Links {
		knownLinks[l.ID] = true
	}
	for _, id := range profiles {
		if !knownIDs[id] {
			return c, summary, fmt.Errorf("所选会话已不存在")
		}
		ids[id] = true
	}
	for _, d := range dirs {
		if d == "" || d == "Links" || !knownDirs[d] {
			return c, summary, fmt.Errorf("所选目录已不存在或不可删除")
		}
		paths[d] = true
	}
	for _, id := range links {
		if !knownLinks[id] {
			return c, summary, fmt.Errorf("所选快捷引用已不存在")
		}
		refs[id] = true
	}
	var roots []string
	for d := range paths {
		covered := false
		for parent := range paths {
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
	out := c.Clone()
	for _, d := range roots {
		if err := out.DeleteDirectory(d); err != nil {
			return c, summary, err
		}
	}
	retained := out.AllDirectories()
	out.RemoveProfiles(ids)
	remaining := out.Links[:0]
	for _, link := range out.Links {
		if !refs[link.ID] {
			remaining = append(remaining, link)
		}
	}
	out.Links = remaining
	out.Directories = nil
	for _, d := range retained {
		if d != "Links" {
			out.Directories = append(out.Directories, d)
		}
	}
	out.NormalizeLinkRootOrder()
	remainingIDs := map[string]bool{}
	for _, p := range out.Profiles {
		remainingIDs[p.ID] = true
	}
	for _, p := range c.Profiles {
		if !remainingIDs[p.ID] {
			summary.RemovedIDs = append(summary.RemovedIDs, p.ID)
		}
	}
	summary.Profiles = len(summary.RemovedIDs)
	summary.Links = len(c.Links) - len(out.Links)
	summary.Directories = len(c.AllDirectories()) - len(out.AllDirectories())
	if err := out.Validate(); err != nil {
		return c, SessionDeletion{}, err
	}
	return out, summary, nil
}
