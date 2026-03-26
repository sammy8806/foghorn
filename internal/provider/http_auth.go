package provider

import (
	"net/http"
	"strings"

	"foghorn/internal/config"
)

func applyAuth(req *http.Request, auth config.AuthConfig) {
	switch strings.ToLower(strings.TrimSpace(auth.Type)) {
	case "basic":
		if auth.Username != "" {
			req.SetBasicAuth(auth.Username, auth.Password)
		}
	case "bearer":
		if auth.Token != "" {
			req.Header.Set("Authorization", "Bearer "+auth.Token)
		}
	}
}
