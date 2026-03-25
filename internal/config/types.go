package config

import "time"

type Config struct {
	Sources       []SourceConfig      `yaml:"sources"`
	Display       DisplayConfig       `yaml:"display"`
	Sounds        SoundsConfig        `yaml:"sounds"`
	Notifications NotificationsConfig `yaml:"notifications"`
	Actions       []ActionConfig      `yaml:"actions"`
	UI            UIConfig            `yaml:"ui"`
}

type SourceConfig struct {
	Name         string        `yaml:"name"`
	Type         string        `yaml:"type"`
	URL          string        `yaml:"url"`
	Auth         AuthConfig    `yaml:"auth"`
	PollInterval time.Duration `yaml:"poll_interval"`
	Filters      []string      `yaml:"filters"`
}

type AuthConfig struct {
	Type     string `yaml:"type"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type DisplayConfig struct {
	VisibleLabels      []string `yaml:"visible_labels" json:"visible_labels"`
	VisibleAnnotations []string `yaml:"visible_annotations" json:"visible_annotations"`
	GroupBy            []string `yaml:"group_by" json:"group_by"`
	SortBy             string   `yaml:"sort_by" json:"sort_by"`
}

type SoundsConfig struct {
	Enabled  bool                      `yaml:"enabled"`
	Critical *SoundEntry               `yaml:"critical"`
	Warning  *SoundEntry               `yaml:"warning"`
	Info     *SoundEntry               `yaml:"info"`
	Sources  map[string]SoundOverrides `yaml:"sources"`
}

type SoundEntry struct {
	File     string        `yaml:"file"`
	Repeat   int           `yaml:"repeat"`
	Interval time.Duration `yaml:"interval"`
}

type SoundOverrides struct {
	Critical *SoundEntry `yaml:"critical"`
	Warning  *SoundEntry `yaml:"warning"`
	Info     *SoundEntry `yaml:"info"`
}

type NotificationsConfig struct {
	Enabled        bool `yaml:"enabled"`
	OnNew          bool `yaml:"on_new"`
	OnResolved     bool `yaml:"on_resolved"`
	BatchThreshold int  `yaml:"batch_threshold"`
}

type ActionConfig struct {
	Name   string            `yaml:"name"`
	Match  map[string]string `yaml:"match"`
	Action ActionDef         `yaml:"action"`
	Icon   string            `yaml:"icon"`
}

type ActionDef struct {
	Type     string `yaml:"type"`
	Template string `yaml:"template"`
	Command  string `yaml:"command"`
	Terminal bool   `yaml:"terminal"`
}

type UIConfig struct {
	Theme        string `yaml:"theme" json:"theme"`
	PopupWidth   int    `yaml:"popup_width" json:"popup_width"`
	PopupHeight  int    `yaml:"popup_height" json:"popup_height"`
	ShowResolved bool   `yaml:"show_resolved" json:"show_resolved"`
	ShowSilenced bool   `yaml:"show_silenced" json:"show_silenced"`
}
