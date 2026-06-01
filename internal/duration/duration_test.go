package duration

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"", 0, false},
		{"   ", 0, false},
		{"30m", 30 * time.Minute, false},
		{"2h", 2 * time.Hour, false},
		{"500ms", 500 * time.Millisecond, false},
		{"1d", 24 * time.Hour, false},
		{"3d", 3 * 24 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"1w2d3h", (7*24 + 2*24 + 3) * time.Hour, false},
		{"  2d  ", 2 * 24 * time.Hour, false},
		{"garbage", 0, true},
		{"3x", 0, true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := Parse(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = %v; want error", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("Parse(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
