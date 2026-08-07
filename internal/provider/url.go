package provider

import (
	"net/url"
	"strings"
)

// sanitizeRemoteURL returns the URL unchanged if it is an absolute http(s) URL,
// and "" otherwise.
//
// URLs in alert payloads (generatorURL, Better Stack incident links, annotation
// values) are attacker-controlled if the source is hostile or MITM'd. They end
// up in the UI as links and in action templates, so anything that is not plain
// http/https — `javascript:`, `file:`, custom scheme handlers — is dropped here,
// at the boundary, rather than trusted downstream.
func sanitizeRemoteURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return ""
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return ""
	}
	if parsed.Host == "" {
		return ""
	}
	return trimmed
}

// sameOrigin reports whether raw has the same scheme and host as base. Used to
// keep server-supplied follow-up URLs (pagination links) pointed at the
// configured source, so credentials are never sent somewhere else.
func sameOrigin(base, raw string) bool {
	b, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return false
	}
	r, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return strings.EqualFold(b.Scheme, r.Scheme) && strings.EqualFold(b.Host, r.Host)
}
