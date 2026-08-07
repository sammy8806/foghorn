package provider

import "testing"

func TestSanitizeRemoteURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"https", "https://example.com/graph?a=b", "https://example.com/graph?a=b"},
		{"http", "http://example.com/graph", "http://example.com/graph"},
		{"uppercase scheme", "HTTPS://example.com/x", "HTTPS://example.com/x"},
		{"javascript", "javascript:alert(1)", ""},
		{"javascript mixed case", "JavaScript:alert(1)", ""},
		{"data", "data:text/html,<script>alert(1)</script>", ""},
		{"file", "file:///etc/passwd", ""},
		{"custom scheme", "smb://attacker.example/share", ""},
		{"relative", "/relative/path", ""},
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
		{"no host", "https://", ""},
		{"leading whitespace is trimmed", "  https://example.com/x  ", "https://example.com/x"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeRemoteURL(tc.in); got != tc.want {
				t.Fatalf("sanitizeRemoteURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSameOrigin(t *testing.T) {
	tests := []struct {
		name string
		base string
		raw  string
		want bool
	}{
		{"identical", "https://api.example.com", "https://api.example.com/page/2", true},
		{"case-insensitive host", "https://API.example.com", "https://api.example.com/x", true},
		{"trailing slash on base", "https://api.example.com/", "https://api.example.com/x", true},
		{"different host", "https://api.example.com", "https://attacker.example/x", false},
		{"different scheme", "https://api.example.com", "http://api.example.com/x", false},
		{"different port", "https://api.example.com", "https://api.example.com:8443/x", false},
		{"relative next", "https://api.example.com", "/page/2", false},
		{"empty next", "https://api.example.com", "", false},
		{"userinfo smuggling", "https://api.example.com", "https://api.example.com@attacker.example/x", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameOrigin(tc.base, tc.raw); got != tc.want {
				t.Fatalf("sameOrigin(%q, %q) = %v, want %v", tc.base, tc.raw, got, tc.want)
			}
		})
	}
}
