package worker

import (
	"testing"
)

func TestNormalizeSubdomain(t *testing.T) {
	tests := []struct {
		name       string
		subdomain  string
		rootDomain string
		expected   string
	}{
		{
			name:       "Remove www prefix - reject if equals root",
			subdomain:  "www.example.com",
			rootDomain: "example.com",
			expected:   "", // بعد از حذف www برابر rootDomain می‌شود، پس رد می‌شود
		},
		{
			name:       "Keep subdomain",
			subdomain:  "api.example.com",
			rootDomain: "example.com",
			expected:   "api.example.com",
		},
		{
			name:       "Remove www from subdomain",
			subdomain:  "www.api.example.com",
			rootDomain: "example.com",
			expected:   "api.example.com",
		},
		{
			name:       "Reject root domain",
			subdomain:  "example.com",
			rootDomain: "example.com",
			expected:   "",
		},
		{
			name:       "Case insensitive - normalize to lowercase",
			subdomain:  "WWW.API.EXAMPLE.COM",
			rootDomain: "example.com",
			expected:   "api.example.com",
		},
		{
			name:       "Invalid domain",
			subdomain:  "other.com",
			rootDomain: "example.com",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeSubdomain(tt.subdomain, tt.rootDomain)
			if result != tt.expected {
				t.Errorf("normalizeSubdomain(%q, %q) = %q, want %q", tt.subdomain, tt.rootDomain, result, tt.expected)
			}
		})
	}
}

func TestParseCrtshJSON(t *testing.T) {
	tests := []struct {
		name        string
		jsonData    string
		regexPattern string
		rootDomain  string
		expected    []string
	}{
		{
			name: "Parse common_name and name_value",
			jsonData: `[
				{"common_name": "*.example.com", "name_value": "example.com\nwww.example.com"},
				{"common_name": "api.example.com", "name_value": "api.example.com"}
			]`,
			regexPattern: "example\\.com",
			rootDomain:   "example.com",
			expected:     []string{"example.com", "www.example.com", "api.example.com"},
		},
		{
			name: "Filter by root domain",
			jsonData: `[
				{"common_name": "example.com"},
				{"common_name": "other.com"}
			]`,
			regexPattern: "example\\.com",
			rootDomain:   "example.com",
			expected:     []string{"example.com"},
		},
		{
			name: "Remove wildcard prefix",
			jsonData: `[
				{"common_name": "*.sub.example.com"},
				{"common_name": "\\*.api.example.com"}
			]`,
			regexPattern: "example\\.com",
			rootDomain:   "example.com",
			expected:     []string{"sub.example.com", "api.example.com"},
		},
		{
			name:        "Empty JSON",
			jsonData:    `[]`,
			regexPattern: "example\\.com",
			rootDomain:   "example.com",
			expected:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCrtshJSON([]byte(tt.jsonData), tt.regexPattern, tt.rootDomain)
			
			// Convert to map for easier comparison
			resultMap := make(map[string]bool)
			for _, r := range result {
				resultMap[r] = true
			}
			expectedMap := make(map[string]bool)
			for _, e := range tt.expected {
				expectedMap[e] = true
			}

			if len(resultMap) != len(expectedMap) {
				t.Errorf("parseCrtshJSON() returned %d results, want %d", len(resultMap), len(expectedMap))
				return
			}

			for _, e := range tt.expected {
				if !resultMap[e] {
					t.Errorf("parseCrtshJSON() missing expected result: %q", e)
				}
			}
		})
	}
}

