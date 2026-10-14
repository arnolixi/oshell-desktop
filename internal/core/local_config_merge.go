package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// LocalConfigMerge handles desktop configuration units. Credential-source
// selection and committing the result are separate from this pure planning step.
type LocalConfigMerge struct {
	Conflicts             []SharedMergeConflict
	merged, local, remote map[string]json.RawMessage
	order                 []string
	origins               map[string]SharedMergeChoice
}

func localConfigUnits(cfg Config) (map[string]json.RawMessage, []string, error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, nil, err
	}
	units := map[string]json.RawMessage{}
	if err = json.Unmarshal(data, &units); err != nil {
		return nil, nil, err
	}
	delete(units, "profiles")
	delete(units, "proxies")
	var order []string
	for _, p := range cfg.Profiles {
		key := "profile:" + strings.ToUpper(p.ID)
		if _, exists := units[key]; exists {
			return nil, nil, fmt.Errorf("会话 ID 重复")
		}
		units[key], err = json.Marshal(p)
		if err != nil {
			return nil, nil, err
		}
		order = append(order, key)
	}
	for _, p := range cfg.Proxies {
		key := "proxy:" + strings.ToUpper(p.ID())
		if _, exists := units[key]; exists {
			return nil, nil, fmt.Errorf("代理 ID 重复")
		}
		units[key], err = json.Marshal(p)
		if err != nil {
			return nil, nil, err
		}
		order = append(order, key)
	}
	return units, order, nil
}
func NewLocalConfigMerge(base, local, remote Config) (*LocalConfigMerge, error) {
	b, _, err := localConfigUnits(base)
	if err != nil {
		return nil, err
	}
	l, order, err := localConfigUnits(local)
	if err != nil {
		return nil, err
	}
	r, extra, err := localConfigUnits(remote)
	if err != nil {
		return nil, err
	}
	return mergeLocalUnits(b, l, r, order, extra), nil
}
func mergeLocalUnits(b, l, r map[string]json.RawMessage, order, extra []string) *LocalConfigMerge {
	plan := &LocalConfigMerge{merged: map[string]json.RawMessage{}, local: l, remote: r, origins: map[string]SharedMergeChoice{}}
	seen := map[string]bool{}
	for _, key := range append(order, extra...) {
		if !seen[key] {
			plan.order = append(plan.order, key)
			seen[key] = true
		}
	}
	keys := map[string]bool{}
	for key := range b {
		keys[key] = true
	}
	for key := range l {
		keys[key] = true
	}
	for key := range r {
		keys[key] = true
	}
	sorted := make([]string, 0, len(keys))
	for key := range keys {
		sorted = append(sorted, key)
	}
	sort.Strings(sorted)
	for _, key := range sorted {
		switch {
		case equalSharedJSON(l[key], r[key]):
			plan.merged[key] = l[key]
			plan.origins[key] = SharedKeepLocal
		case equalSharedJSON(l[key], b[key]):
			plan.merged[key] = r[key]
			plan.origins[key] = SharedKeepRemote
		case equalSharedJSON(r[key], b[key]):
			plan.merged[key] = l[key]
			plan.origins[key] = SharedKeepLocal
		default:
			title := key
			if strings.HasPrefix(key, "profile:") {
				var p Profile
				_ = json.Unmarshal(l[key], &p)
				if p.Name == "" {
					_ = json.Unmarshal(r[key], &p)
				}
				title = "会话：" + p.Name
			}
			if strings.HasPrefix(key, "proxy:") {
				var p ProxyProfile
				_ = json.Unmarshal(l[key], &p)
				if p.Name == "" {
					_ = json.Unmarshal(r[key], &p)
				}
				title = "代理：" + p.Name
			}
			var localUnit, remoteUnit map[string]json.RawMessage
			_ = json.Unmarshal(l[key], &localUnit)
			_ = json.Unmarshal(r[key], &remoteUnit)
			if !equalSharedJSON(localUnit["_credentialVersion"], remoteUnit["_credentialVersion"]) {
				title += "（含凭据版本差异）"
			}
			plan.Conflicts = append(plan.Conflicts, SharedMergeConflict{Key: key, Title: title, LocalDescription: describeSharedJSON(stripCredentialMarker(l[key])), RemoteDescription: describeSharedJSON(stripCredentialMarker(r[key]))})
		}
	}
	return plan
}
func (m *LocalConfigMerge) Resolve(choices map[string]SharedMergeChoice) (Config, error) {
	values := map[string]json.RawMessage{}
	for key, value := range m.merged {
		values[key] = value
	}
	for _, conflict := range m.Conflicts {
		switch choices[conflict.Key] {
		case SharedKeepLocal:
			values[conflict.Key] = m.local[conflict.Key]
		case SharedKeepRemote:
			values[conflict.Key] = m.remote[conflict.Key]
		default:
			return Config{}, fmt.Errorf("请选择如何处理 %s", conflict.Title)
		}
	}
	var profiles, proxies []json.RawMessage
	for _, key := range m.order {
		value := values[key]
		if len(value) == 0 {
			continue
		}
		if strings.HasPrefix(key, "profile:") {
			profiles = append(profiles, stripCredentialMarker(value))
		} else {
			proxies = append(proxies, stripCredentialMarker(value))
		}
	}
	for key, value := range values {
		if strings.HasPrefix(key, "profile:") || strings.HasPrefix(key, "proxy:") || len(value) == 0 {
			delete(values, key)
		}
	}
	values["profiles"], _ = json.Marshal(profiles)
	values["proxies"], _ = json.Marshal(proxies)
	data, err := json.Marshal(values)
	if err != nil {
		return Config{}, err
	}
	cfg := DefaultConfig()
	if err = json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	cfg.NormalizeLinkRootOrder()
	return cfg, cfg.Validate()
}

func stripCredentialMarker(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var value map[string]json.RawMessage
	if json.Unmarshal(raw, &value) != nil {
		return raw
	}
	delete(value, "_credentialVersion")
	data, err := json.Marshal(value)
	if err != nil {
		return raw
	}
	return data
}
func (m *LocalConfigMerge) unitSource(key string, choices map[string]SharedMergeChoice) SharedMergeChoice {
	if source, ok := m.origins[key]; ok {
		return source
	}
	return choices[key]
}
