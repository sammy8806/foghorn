package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// EntryStyle is one of a closed set of visual treatment tokens applied to a
// visible annotation or label entry.
type EntryStyle string

const (
	StyleMuted   EntryStyle = "muted"
	StylePull    EntryStyle = "pull"
	StyleDanger  EntryStyle = "danger"
	StyleWarning EntryStyle = "warning"
	StyleInfo    EntryStyle = "info"
)

// VisibleEntry is one row of display.visible_annotations or
// display.visible_labels. It accepts either a bare YAML scalar (just a field
// ref) or a mapping with source/order/label/style.
type VisibleEntry struct {
	Source string       `json:"source"`
	Order  int          `json:"order"`
	Label  string       `json:"label,omitempty"`
	Style  []EntryStyle `json:"style,omitempty"`
}

func (e *VisibleEntry) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		e.Source = node.Value
		return nil
	case yaml.MappingNode:
		var aux struct {
			Source string      `yaml:"source"`
			Order  int         `yaml:"order"`
			Label  string      `yaml:"label"`
			Style  styleTokens `yaml:"style"`
		}
		if err := node.Decode(&aux); err != nil {
			return fmt.Errorf("visible entry: %w", err)
		}
		e.Source = aux.Source
		e.Order = aux.Order
		e.Label = aux.Label
		e.Style = []EntryStyle(aux.Style)
		return nil
	default:
		return fmt.Errorf("visible entry: expected scalar or mapping, got node kind %d", node.Kind)
	}
}

var validStyles = map[EntryStyle]struct{}{
	StyleMuted:   {},
	StylePull:    {},
	StyleDanger:  {},
	StyleWarning: {},
	StyleInfo:    {},
}

// validateVisibleEntries enforces that every entry has a non-empty source and
// only references known style tokens. The field argument is used to produce
// positional error messages like "display.visible_annotations[2]: ...".
func validateVisibleEntries(field string, entries []VisibleEntry) error {
	for i, e := range entries {
		if strings.TrimSpace(e.Source) == "" {
			return fmt.Errorf("%s[%d]: source is required", field, i)
		}
		for _, s := range e.Style {
			if _, ok := validStyles[s]; !ok {
				return fmt.Errorf("%s[%d]: unknown style %q", field, i, s)
			}
		}
	}
	return nil
}

// styleTokens is a custom YAML adapter that accepts either a comma-separated
// scalar ("pull, danger") or a sequence ([pull, danger]).
type styleTokens []EntryStyle

func (s *styleTokens) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		for _, part := range strings.Split(node.Value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			*s = append(*s, EntryStyle(part))
		}
		return nil
	case yaml.SequenceNode:
		var raw []string
		if err := node.Decode(&raw); err != nil {
			return err
		}
		for _, v := range raw {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			*s = append(*s, EntryStyle(v))
		}
		return nil
	default:
		return fmt.Errorf("style: expected scalar or sequence, got node kind %d", node.Kind)
	}
}
