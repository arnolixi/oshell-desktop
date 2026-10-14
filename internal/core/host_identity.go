// SPDX-License-Identifier: GPL-3.0-only
package core

import (
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type HostIdentity struct{ Hostname, Address string }

func ValidHostnameHint(host string) bool {
	return host != "" && len(host) <= 253 && host != "." && host != ".." && validHost(host)
}
func ParseHostIdentity(payload string) (HostIdentity, bool) {
	var result HostIdentity
	if len(payload) > 4096 || !utf8.ValidString(payload) {
		return result, false
	}
	fields := strings.Split(payload, "|")
	if len(fields) != 3 || !ValidHostnameHint(fields[0]) {
		return result, false
	}
	address := func(s string) string {
		ip := strings.SplitN(s, "/", 2)[0]
		if net.ParseIP(ip) != nil {
			return ip
		}
		return ""
	}
	result.Hostname = fields[0]
	result.Address = address(fields[1])
	if result.Address == "0.0.0.0" || result.Address == "::" {
		result.Address = ""
	}
	var candidates []string
	for _, field := range strings.Fields(fields[2]) {
		ip := address(field)
		if ip == "" || ip == "0.0.0.0" || ip == "::" || ip == "::1" || strings.HasPrefix(ip, "127.") || strings.HasPrefix(strings.ToLower(ip), "fe80:") {
			continue
		}
		candidates = append(candidates, ip)
	}
	if result.Address == "" {
		for _, ip := range candidates {
			if !strings.Contains(ip, ":") {
				result.Address = ip
				break
			}
		}
		if result.Address == "" && len(candidates) > 0 {
			result.Address = candidates[0]
		}
	}
	return result, true
}
func ParseShellIntegration(payload string) (HostIdentity, bool) {
	const prefix = "OShellHost=1;"
	if !strings.HasPrefix(payload, prefix) {
		return HostIdentity{}, false
	}
	return ParseHostIdentity(strings.TrimPrefix(payload, prefix))
}
func SameHostHint(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if a == b {
		return true
	}
	if net.ParseIP(a) != nil || net.ParseIP(b) != nil {
		return false
	}
	return (!strings.Contains(a, ".") || !strings.Contains(b, ".")) && strings.SplitN(a, ".", 2)[0] == strings.SplitN(b, ".", 2)[0]
}

var promptHostPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^\s*\[[^\s@\[\]:]{1,64}@([A-Za-z0-9][A-Za-z0-9_.-]{0,252})\s+[^\r\n\]]*\]\s*[#$%>]\s*$`),
	regexp.MustCompile(`^\s*[^\s@\[\]:]{1,64}@([A-Za-z0-9][A-Za-z0-9_.-]{0,252}):[^\r\n]*[#$%>]\s*$`),
	regexp.MustCompile(`^\s*[^\s@\[\]:]{1,64}@([A-Za-z0-9][A-Za-z0-9_.-]{0,252})\s+[^\r\n]*[#$%>]\s*$`),
}

func HostnameFromPrompt(text string) string {
	if len(text) > 2048 || strings.ContainsFunc(text, unicode.IsControl) {
		return ""
	}
	for _, pattern := range promptHostPatterns {
		if match := pattern.FindStringSubmatch(text); len(match) > 1 && ValidHostnameHint(match[1]) {
			return match[1]
		}
	}
	return ""
}
func HostnameFromDirectory(text string) string {
	if len(text) > 8192 {
		return ""
	}
	u, e := url.Parse(text)
	if e != nil || u.Scheme != "file" || strings.EqualFold(u.Hostname(), "localhost") || !ValidHostnameHint(u.Hostname()) {
		return ""
	}
	return u.Hostname()
}

// HostObservation is display-only and must never feed authentication or dialing.
type HostObservation struct {
	Mode                          string
	Identity                      HostIdentity
	HasIntegration, PromptPending bool
	aliases                       []string
}

func (h *HostObservation) Integration(identity HostIdentity) bool {
	if h.Mode == "passive" {
		return false
	}
	h.Identity = identity
	h.HasIntegration = true
	h.PromptPending = true
	h.aliases = []string{identity.Hostname}
	return true
}
func (h *HostObservation) Hint(host string) bool {
	if host == "" {
		return false
	}
	if h.PromptPending {
		h.aliases = append(h.aliases, host)
		h.PromptPending = false
		return false
	}
	for _, alias := range h.aliases {
		if SameHostHint(alias, host) {
			return false
		}
	}
	h.Identity = HostIdentity{Hostname: host}
	h.aliases = []string{host}
	return true
}
func (h *HostObservation) AllowsActiveProbe() bool {
	return h.Mode == "activeProbe" && !h.HasIntegration
}

var hostCommandPrompt = regexp.MustCompile(`^(?:\s*\[[^\]\r\n]+@[^\]\r\n]+\]\s*[#$%>]\s*|\s*[^\s@]+@[^\s:]+(?:[:\s])[^\r\n]*?[$#%>]\s*)(.*)$`)

func ChangesHost(input string, containsPrompt bool) bool {
	if len(input) > 2048 {
		return false
	}
	command := strings.TrimSpace(input)
	if containsPrompt {
		match := hostCommandPrompt.FindStringSubmatch(command)
		if len(match) < 2 {
			return false
		}
		command = match[1]
	}
	words := strings.Fields(command)
	if len(words) == 0 {
		return false
	}
	first := words[0]
	if index := strings.LastIndexByte(first, '/'); index >= 0 {
		first = first[index+1:]
	}
	switch first {
	case "ssh", "telnet", "rlogin", "exit", "logout", "su":
		return true
	}
	return false
}
func (h *HostObservation) HostChange() {
	h.Identity.Address = ""
	h.PromptPending = false
	if !h.AllowsActiveProbe() {
		h.Identity.Hostname = ""
		h.aliases = nil
	}
}

func (h *HostObservation) Probe(identity HostIdentity) {
	h.Identity = identity
	h.aliases = append(h.aliases, identity.Hostname)
}
