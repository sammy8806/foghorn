package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestVisibleEntryUnmarshalScalar(t *testing.T) {
	var entries []VisibleEntry
	if err := yaml.Unmarshal([]byte("- summary\n- description\n"), &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Source != "summary" || entries[0].Order != 0 || entries[0].Label != "" || len(entries[0].Style) != 0 {
		t.Errorf("entry[0] = %#v, want {Source:summary}", entries[0])
	}
	if entries[1].Source != "description" {
		t.Errorf("entry[1].Source = %q, want %q", entries[1].Source, "description")
	}
}
