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

func TestVisibleEntryUnmarshalMapping(t *testing.T) {
	yamlData := `
- source: description
  order: 5
  label: Description
  style: [pull, danger]
- source: field:hiddenBy
  order: -5
  label: Hidden By
  style: [muted]
`
	var entries []VisibleEntry
	if err := yaml.Unmarshal([]byte(yamlData), &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	want := VisibleEntry{
		Source: "description",
		Order:  5,
		Label:  "Description",
		Style:  []EntryStyle{StylePull, StyleDanger},
	}
	if entries[0].Source != want.Source || entries[0].Order != want.Order || entries[0].Label != want.Label {
		t.Errorf("entry[0] scalars mismatch: got %#v, want %#v", entries[0], want)
	}
	if len(entries[0].Style) != 2 || entries[0].Style[0] != StylePull || entries[0].Style[1] != StyleDanger {
		t.Errorf("entry[0].Style = %#v, want %#v", entries[0].Style, want.Style)
	}
	if entries[1].Order != -5 || len(entries[1].Style) != 1 || entries[1].Style[0] != StyleMuted {
		t.Errorf("entry[1] = %#v", entries[1])
	}
}

func TestVisibleEntryStyleScalar(t *testing.T) {
	yamlData := `
- source: description
  style: "pull, danger"
- source: link
  style: info
`
	var entries []VisibleEntry
	if err := yaml.Unmarshal([]byte(yamlData), &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries[0].Style) != 2 || entries[0].Style[0] != StylePull || entries[0].Style[1] != StyleDanger {
		t.Errorf("entry[0].Style = %#v, want [pull danger]", entries[0].Style)
	}
	if len(entries[1].Style) != 1 || entries[1].Style[0] != StyleInfo {
		t.Errorf("entry[1].Style = %#v, want [info]", entries[1].Style)
	}
}
