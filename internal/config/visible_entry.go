package config

import (
	"fmt"

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
	if node.Kind == yaml.ScalarNode {
		e.Source = node.Value
		return nil
	}
	return fmt.Errorf("visible entry: expected scalar or mapping, got node kind %d", node.Kind)
}
