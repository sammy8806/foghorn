package silenceaccess

import (
	"encoding/json"
	"fmt"
)

// ParseRules maps rule UID to folder UID from a Grafana
// /api/prometheus/grafana/api/v1/rules response. A rule's own folderUid wins
// over its group's.
func ParseRules(body []byte) (map[string]string, error) {
	var resp struct {
		Data struct {
			Groups []struct {
				FolderUID string `json:"folderUid"`
				Rules     []struct {
					UID       string `json:"uid"`
					FolderUID string `json:"folderUid"`
				} `json:"rules"`
			} `json:"groups"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding rules: %w", err)
	}
	out := make(map[string]string)
	for _, g := range resp.Data.Groups {
		for _, r := range g.Rules {
			if r.UID == "" {
				continue
			}
			folder := r.FolderUID
			if folder == "" {
				folder = g.FolderUID
			}
			if folder != "" {
				out[r.UID] = folder
			}
		}
	}
	return out, nil
}

// ParseFolders reads a Grafana /api/search?type=dash-folder response into
// parent and title maps. Top-level folders have parent "".
func ParseFolders(body []byte) (parents, titles map[string]string, err error) {
	var items []struct {
		UID       string `json:"uid"`
		FolderUID string `json:"folderUid"`
		Title     string `json:"title"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, nil, fmt.Errorf("decoding folders: %w", err)
	}
	parents = make(map[string]string, len(items))
	titles = make(map[string]string, len(items))
	for _, it := range items {
		if it.UID == "" {
			continue
		}
		parents[it.UID] = it.FolderUID
		titles[it.UID] = it.Title
	}
	return parents, titles, nil
}
