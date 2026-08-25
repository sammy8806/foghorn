package config

import (
	"strings"
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

func TestValidateVisibleEntriesEmptySource(t *testing.T) {
	entries := []VisibleEntry{
		{Source: "summary"},
		{Source: ""},
	}
	var diags Diagnostics
	surviving := validateVisibleEntries("display.visible_annotations", entries, &diags)
	if len(surviving) != 1 || surviving[0].Source != "summary" {
		t.Fatalf("surviving = %#v, want only the valid entry", surviving)
	}
	if len(diags) != 1 {
		t.Fatalf("diags = %#v, want exactly one", diags)
	}
	if !diags[0].Dropped {
		t.Errorf("Dropped = false, want true for a dropped entry")
	}
	if !strings.Contains(diags[0].Field, "display.visible_annotations[1]") {
		t.Errorf("field missing positional context: %v", diags[0].Field)
	}
	if !strings.Contains(diags[0].Message, "source is required") {
		t.Errorf("message missing 'source is required': %v", diags[0].Message)
	}
}

func TestValidateVisibleEntriesUnknownStyle(t *testing.T) {
	entries := []VisibleEntry{
		{Source: "summary"},
		{Source: "description", Style: []EntryStyle{StylePull, "ominous"}},
	}
	var diags Diagnostics
	surviving := validateVisibleEntries("display.visible_annotations", entries, &diags)
	if len(surviving) != 1 || surviving[0].Source != "summary" {
		t.Fatalf("surviving = %#v, want only the valid entry", surviving)
	}
	if len(diags) != 1 {
		t.Fatalf("diags = %#v, want exactly one", diags)
	}
	if !diags[0].Dropped {
		t.Errorf("Dropped = false, want true for a dropped entry")
	}
	if !strings.Contains(diags[0].Field, "display.visible_annotations[1]") {
		t.Errorf("field missing positional context: %v", diags[0].Field)
	}
	if !strings.Contains(diags[0].Message, `"ominous"`) {
		t.Errorf("message missing the bad token: %v", diags[0].Message)
	}
}

func TestValidateVisibleEntriesAllValid(t *testing.T) {
	entries := []VisibleEntry{
		{Source: "summary"},
		{Source: "description", Style: []EntryStyle{StyleMuted, StylePull, StyleDanger, StyleWarning, StyleInfo}},
	}
	var diags Diagnostics
	surviving := validateVisibleEntries("display.visible_annotations", entries, &diags)
	if len(surviving) != len(entries) {
		t.Fatalf("surviving = %#v, want all entries to survive", surviving)
	}
	if len(diags) != 0 {
		t.Errorf("diags = %#v, want none", diags)
	}
}

func TestSortVisibleEntries(t *testing.T) {
	entries := []VisibleEntry{
		{Source: "a", Order: -5},
		{Source: "b"},
		{Source: "c"},
		{Source: "d", Order: 5},
		{Source: "e"},
		{Source: "f", Order: -5},
	}
	sortVisibleEntries(entries)
	got := make([]string, len(entries))
	for i, e := range entries {
		got[i] = e.Source
	}
	want := []string{"a", "f", "b", "c", "e", "d"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sorted order = %v, want %v", got, want)
		}
	}
}
