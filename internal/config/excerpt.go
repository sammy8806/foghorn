package config

import (
	"os"
	"regexp"
	"strconv"
	"strings"
)

var locatorLinePattern = regexp.MustCompile(`^line (\d+)$`)

// excerptRadius is how many lines of context to show either side of the one the
// parser stopped on. One is enough to recognise the place in a config you wrote
// yourself; more turns the card into a file viewer.
const excerptRadius = 1

// excerptLineLimit caps a single line, so a config with a pasted-in token or a
// one-line JSON blob cannot push a wall of text through the event payload.
const excerptLineLimit = 200

// ExcerptLine is one line of the config file, shown as evidence beside the
// problem that names it. Marked is true for the line the parser stopped on.
type ExcerptLine struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
	Marked bool   `json:"marked"`
}

// ExcerptFor reads the lines around the first diagnostic that points at one, so
// the UI can show where a config stopped parsing rather than only saying so.
//
// It returns nil whenever there is nothing worth showing — no positioned
// diagnostic, an unreadable file, a line past the end — because the excerpt is
// a bonus on top of the message, never the thing carrying it.
func ExcerptFor(path string, diags Diagnostics) []ExcerptLine {
	line := firstLocatedLine(diags)
	if path == "" || line == 0 {
		return nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	// A file ends in a newline, so the split leaves a phantom empty line after
	// it; keeping it would put a blank row under the last line of the file.
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if line > len(lines) {
		return nil
	}

	first := max(line-excerptRadius, 1)
	last := min(line+excerptRadius, len(lines))

	excerpt := make([]ExcerptLine, 0, last-first+1)
	for number := first; number <= last; number++ {
		excerpt = append(excerpt, ExcerptLine{
			Number: number,
			Text:   clipLine(lines[number-1]),
			Marked: number == line,
		})
	}
	return excerpt
}

func firstLocatedLine(diags Diagnostics) int {
	for _, diag := range diags {
		if match := locatorLinePattern.FindStringSubmatch(diag.Locator); match != nil {
			if number, err := strconv.Atoi(match[1]); err == nil && number > 0 {
				return number
			}
		}
	}
	return 0
}

// Tabs are widened here rather than in CSS: the excerpt renders the line
// numbers in their own column, so a tab stop measured from the start of the row
// would not line up with the text anyway.
func clipLine(text string) string {
	text = strings.ReplaceAll(strings.TrimRight(text, "\r"), "\t", "  ")
	if len(text) > excerptLineLimit {
		return text[:excerptLineLimit] + "…"
	}
	return text
}
