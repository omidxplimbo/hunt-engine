package skills

import (
	"strings"
)

// SelectSkillsForTech returns skill names relevant to a detected tech stack.
// Used by the agent loop to dynamically load guidance before testing.
func SelectSkillsForTech(loader *SkillLoader, techSignals []string) []*Skill {
	var relevant []*Skill
	seen := make(map[string]bool)

	for _, signal := range techSignals {
		sig := strings.ToLower(strings.TrimSpace(signal))
		if sig == "" {
			continue
		}
		for _, s := range loader.GetAll() {
			if seen[s.Name] {
				continue
			}
			if skillMatchesSignal(s, sig) {
				relevant = append(relevant, s)
				seen[s.Name] = true
			}
		}
	}

	return relevant
}

// skillMatchesSignal checks if a skill matches a technology signal
func skillMatchesSignal(s *Skill, signal string) bool {
	// Check triggers
	for _, t := range s.Triggers {
		if strings.Contains(signal, strings.ToLower(t)) || strings.Contains(strings.ToLower(t), signal) {
			return true
		}
	}

	// Check category/bug class mapping
	mappings := map[string][]string{
		"php":         {"xss", "sqli", "file_upload", "command_injection"},
		"nginx":       {"ssrf", "lfi", "path_traversal"},
		"apache":      {"xss", "path_traversal"},
		"node":        {"prototype_pollution", "xss", "ssti"},
		"express":     {"prototype_pollution", "xss"},
		"react":       {"xss"},
		"vue":         {"xss"},
		"angular":     {"xss"},
		"django":      {"ssti", "sqli", "xss"},
		"flask":       {"ssti", "sqli"},
		"jinja":       {"ssti"},
		"spring":      {"ssti", "sqli", "rce"},
		"java":        {"ssti", "deserialization", "xxe"},
		"tomcat":      {"ssti", "rce"},
		"api":         {"idor", "bola", "mass_assignment"},
		"json":        {"idor"},
		"rest":        {"idor"},
		"upload":      {"file_upload"},
		"file":        {"file_upload"},
		"redirect":    {"open_redirect"},
		"login":       {"auth_bypass", "bruteforce"},
		"jwt":         {"jwt_attacks"},
		"token":       {"jwt_attacks"},
		"search":      {"xss", "sqli"},
		"query":       {"sqli"},
		"id":          {"idor"},
	}

	for keyword, classes := range mappings {
		if strings.Contains(signal, keyword) {
			for _, c := range classes {
				if s.BugClass == c || strings.EqualFold(s.Category, c) {
					return true
				}
				for _, t := range s.Triggers {
					if strings.EqualFold(t, c) {
						return true
					}
				}
			}
		}
	}

	return false
}
