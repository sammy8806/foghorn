package config

import (
	"fmt"
	"strings"
)

const (
	defaultCriticalColor = "#ef4444"
	defaultWarningColor  = "#f59e0b"
	defaultInfoColor     = "#3b82f6"
	defaultUnknownColor  = "#6b7280"
)

type SeverityConfig struct {
	Default string          `yaml:"default" json:"default"`
	Levels  []SeverityLevel `yaml:"levels" json:"levels"`
}

type SeverityLevel struct {
	Name    string   `yaml:"name" json:"name"`
	Color   string   `yaml:"color" json:"color"`
	Aliases []string `yaml:"aliases" json:"aliases"`
}

type NormalizedSeverityConfig struct {
	Default string                    `json:"default"`
	Levels  []NormalizedSeverityLevel `json:"levels"`
}

type NormalizedSeverityLevel struct {
	Name    string   `json:"name"`
	Color   string   `json:"color"`
	Aliases []string `json:"aliases"`
	Rank    int      `json:"rank"`
}

type SeverityScheme struct {
	Default          string
	Levels           []NormalizedSeverityLevel
	aliasToCanonical map[string]string
	rankByName       map[string]int
	colorByName      map[string]string
}

func DefaultSeverityConfig() SeverityConfig {
	return SeverityConfig{
		Default: "unknown",
		Levels: []SeverityLevel{
			{Name: "critical", Color: defaultCriticalColor, Aliases: []string{"critical"}},
			{Name: "warning", Color: defaultWarningColor, Aliases: []string{"warning"}},
			{Name: "info", Color: defaultInfoColor, Aliases: []string{"info"}},
			{Name: "unknown", Color: defaultUnknownColor, Aliases: []string{"unknown"}},
		},
	}
}

func NormalizeSeverityConfig(raw SeverityConfig) (NormalizedSeverityConfig, error) {
	if len(raw.Levels) == 0 {
		raw = DefaultSeverityConfig()
	}

	levels := make([]NormalizedSeverityLevel, 0, len(raw.Levels))
	seenNames := make(map[string]struct{}, len(raw.Levels))
	seenAliases := make(map[string]string)

	for i, level := range raw.Levels {
		name := normalizeSeverityKey(level.Name)
		if name == "" {
			return NormalizedSeverityConfig{}, fmt.Errorf("severities.levels[%d]: name is required", i)
		}
		if _, exists := seenNames[name]; exists {
			return NormalizedSeverityConfig{}, fmt.Errorf("severities.levels[%d]: duplicate name %q", i, name)
		}
		seenNames[name] = struct{}{}

		aliases := make([]string, 0, len(level.Aliases)+1)
		aliasSeenForLevel := map[string]struct{}{name: {}}
		aliases = append(aliases, name)
		for _, alias := range level.Aliases {
			normalized := normalizeSeverityKey(alias)
			if normalized == "" {
				continue
			}
			if _, exists := aliasSeenForLevel[normalized]; exists {
				continue
			}
			aliasSeenForLevel[normalized] = struct{}{}
			aliases = append(aliases, normalized)
		}

		for _, alias := range aliases {
			if existing, exists := seenAliases[alias]; exists && existing != name {
				return NormalizedSeverityConfig{}, fmt.Errorf("severities.levels[%d]: alias %q already assigned to %q", i, alias, existing)
			}
			seenAliases[alias] = name
		}

		levels = append(levels, NormalizedSeverityLevel{
			Name:    name,
			Color:   normalizeSeverityColor(name, level.Color),
			Aliases: aliases,
			Rank:    i,
		})
	}

	defaultName := normalizeSeverityKey(raw.Default)
	if defaultName == "" {
		defaultName = "unknown"
	}
	if _, exists := seenNames[defaultName]; !exists {
		if _, hasUnknown := seenNames["unknown"]; hasUnknown {
			defaultName = "unknown"
		} else {
			defaultName = levels[len(levels)-1].Name
		}
	}

	return NormalizedSeverityConfig{
		Default: defaultName,
		Levels:  levels,
	}, nil
}

func (n NormalizedSeverityConfig) Scheme() SeverityScheme {
	aliasToCanonical := make(map[string]string)
	rankByName := make(map[string]int, len(n.Levels))
	colorByName := make(map[string]string, len(n.Levels))

	for _, level := range n.Levels {
		rankByName[level.Name] = level.Rank
		colorByName[level.Name] = level.Color
		for _, alias := range level.Aliases {
			aliasToCanonical[alias] = level.Name
		}
	}

	return SeverityScheme{
		Default:          n.Default,
		Levels:           n.Levels,
		aliasToCanonical: aliasToCanonical,
		rankByName:       rankByName,
		colorByName:      colorByName,
	}
}

func (s SeverityScheme) Canonicalize(value string) string {
	key := normalizeSeverityKey(value)
	if canonical, ok := s.aliasToCanonical[key]; ok {
		return canonical
	}
	if s.Default != "" {
		return s.Default
	}
	if len(s.Levels) > 0 {
		return s.Levels[len(s.Levels)-1].Name
	}
	return key
}

func (s SeverityScheme) Rank(value string) int {
	canonical := s.Canonicalize(value)
	if rank, ok := s.rankByName[canonical]; ok {
		return rank
	}
	return len(s.Levels)
}

func (s SeverityScheme) Color(value string) string {
	canonical := s.Canonicalize(value)
	if color, ok := s.colorByName[canonical]; ok {
		return color
	}
	return defaultUnknownColor
}

func (s SeverityScheme) EmptyCounts() map[string]int {
	counts := make(map[string]int, len(s.Levels))
	for _, level := range s.Levels {
		counts[level.Name] = 0
	}
	return counts
}

func normalizeSeverityKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeSeverityColor(name, color string) string {
	if value := strings.TrimSpace(color); value != "" {
		return value
	}
	switch name {
	case "critical":
		return defaultCriticalColor
	case "warning":
		return defaultWarningColor
	case "info":
		return defaultInfoColor
	case "unknown":
		return defaultUnknownColor
	default:
		return defaultUnknownColor
	}
}
