// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"context"
	"sort"
	"strings"
)

type FontEntry struct {
	Name       string
	Monospaced bool
}

func FontCatalog(ctx context.Context) ([]FontEntry, error) {
	fonts, e := platformFonts(ctx)
	if e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	var out []FontEntry
	for _, font := range fonts {
		name := strings.TrimSpace(font.Name)
		key := strings.ToLower(name)
		if name == "" || len(name) > 512 || strings.HasPrefix(name, ".") || seen[key] || strings.HasPrefix(key, "dejavu sans mono") || key == "lastresort" || strings.Contains(key, "symbols nerd") {
			continue
		}
		font.Name = name
		seen[key] = true
		out = append(out, font)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Monospaced != out[j].Monospaced {
			return out[i].Monospaced
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}
