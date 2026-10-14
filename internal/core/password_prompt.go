package core

import "strings"

// loginPasswordPrompt is scoped to the resolved SSH endpoint. Account-qualified
// prompts must match that endpoint exactly; arbitrary password-like text is not
// sufficient to disclose or remember a login password.
func loginPasswordPrompt(identity SSHIdentity, prompt string) bool {
	clean := strings.TrimSpace(prompt)
	switch strings.ToLower(clean) {
	case "password:", "password", "密码：", "密码:":
		return true // This prompt comes directly from the authenticated SSH peer.
	}
	expected := identity.User + "@" + strings.Trim(identity.Host, "[]")
	return clean == expected+"'s password:" || clean == "("+expected+") Password:" || clean == "("+expected+") password:"
}

// loginPasswordQuestion requires exactly one unambiguous, hidden login field.
func loginPasswordQuestion(identity SSHIdentity, questions []string, echo []bool) int {
	if len(questions) != len(echo) {
		return -1
	}
	index := -1
	for i, question := range questions {
		if !echo[i] && loginPasswordPrompt(identity, question) {
			if index >= 0 {
				return -1
			}
			index = i
		}
	}
	return index
}
