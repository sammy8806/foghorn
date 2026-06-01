// Package duration provides foghorn's project-wide duration parser. It
// extends Go's time.ParseDuration with "d" (day) and "w" (week) units so
// the same grammar works in the silence dialog, hide rule min_age, and any
// future configuration field that takes a duration string.
package duration

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// leadingWeekDay matches a leading week or day component (e.g. "1w", "3d"),
// which the standard library's time.ParseDuration does not understand.
var leadingWeekDay = regexp.MustCompile(`^\s*(\d+)([wd])`)

// Parse extends time.ParseDuration with week ("w") and day ("d") units. The
// extended units must appear at the start of the string; any remaining
// portion is passed to time.ParseDuration unchanged, preserving its support
// for h/m/s/ms and composite values. Examples: "3d", "1w2d3h", "30m".
//
// Empty/whitespace-only input is treated as zero (not an error) so callers
// can distinguish "absent" from "explicitly zero" without parsing.
func Parse(s string) (time.Duration, error) {
	rest := strings.TrimSpace(s)
	if rest == "" {
		return 0, nil
	}
	var extra time.Duration
	for {
		m := leadingWeekDay.FindStringSubmatch(rest)
		if m == nil {
			break
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, err
		}
		unit := 24 * time.Hour
		if m[2] == "w" {
			unit = 7 * 24 * time.Hour
		}
		extra += time.Duration(n) * unit
		rest = strings.TrimSpace(rest[len(m[0]):])
	}
	if rest == "" {
		if extra == 0 {
			// No week/day components and nothing left: mirror stdlib's error.
			return time.ParseDuration(strings.TrimSpace(s))
		}
		return extra, nil
	}
	d, err := time.ParseDuration(rest)
	if err != nil {
		return 0, err
	}
	return extra + d, nil
}
