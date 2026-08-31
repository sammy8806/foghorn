package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}
	return path
}

func TestExcerptForSurroundsTheLocatedLine(t *testing.T) {
	path := writeConfigFile(t, "one\ntwo\nthree\nfour\nfive\n")
	diags := Diagnostics{{Field: "config", Locator: "line 3", Message: "boom"}}

	got := ExcerptFor(path, diags)

	want := []ExcerptLine{
		{Number: 2, Text: "two"},
		{Number: 3, Text: "three", Marked: true},
		{Number: 4, Text: "four"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("excerpt = %#v, want %#v", got, want)
	}
}

func TestExcerptForClampsAtTheFileEdges(t *testing.T) {
	path := writeConfigFile(t, "only line\n")
	diags := Diagnostics{{Field: "config", Locator: "line 1", Message: "boom"}}

	got := ExcerptFor(path, diags)

	want := []ExcerptLine{{Number: 1, Text: "only line", Marked: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("excerpt = %#v, want %#v", got, want)
	}
}

func TestExcerptForSkipsTheFirstDiagnosticWithoutAPosition(t *testing.T) {
	path := writeConfigFile(t, "one\ntwo\nthree\n")
	diags := Diagnostics{
		{Field: "ui.popup_position", Message: "unknown position"},
		{Field: "config", Locator: "line 2", Message: "boom"},
	}

	got := ExcerptFor(path, diags)

	if len(got) != 3 || !got[1].Marked || got[1].Number != 2 {
		t.Fatalf("excerpt = %#v, want line 2 marked with a line either side", got)
	}
}

func TestExcerptForReturnsNothingWithoutSomethingToShow(t *testing.T) {
	path := writeConfigFile(t, "one\ntwo\n")

	cases := map[string]struct {
		path  string
		diags Diagnostics
	}{
		"no positioned diagnostic": {path, Diagnostics{{Field: "ui.scale", Message: "out of range"}}},
		"no diagnostics at all":    {path, nil},
		"no path yet":              {"", Diagnostics{{Field: "config", Locator: "line 1"}}},
		"unreadable file":          {filepath.Join(t.TempDir(), "missing.yaml"), Diagnostics{{Field: "config", Locator: "line 1"}}},
		"line past the end":        {path, Diagnostics{{Field: "config", Locator: "line 99"}}},
		"locator is not a line":    {path, Diagnostics{{Field: "config", Locator: "somewhere"}}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := ExcerptFor(tc.path, tc.diags); got != nil {
				t.Fatalf("excerpt = %#v, want nil", got)
			}
		})
	}
}

func TestExcerptForNormalisesLineContent(t *testing.T) {
	long := "key: " + string(make([]byte, excerptLineLimit))
	path := writeConfigFile(t, "\tindented: yes\r\nsecond\r\n"+long+"\r\n")
	diags := Diagnostics{{Field: "config", Locator: "line 2", Message: "boom"}}

	got := ExcerptFor(path, diags)

	if got[0].Text != "  indented: yes" {
		t.Errorf("tabs = %q, want them widened to spaces", got[0].Text)
	}
	if len([]rune(got[2].Text)) != excerptLineLimit+1 || !hasSuffixRune(got[2].Text, '…') {
		t.Errorf("long line = %d runes, want it clipped to %d plus an ellipsis", len([]rune(got[2].Text)), excerptLineLimit)
	}
}

func hasSuffixRune(text string, want rune) bool {
	runes := []rune(text)
	return len(runes) > 0 && runes[len(runes)-1] == want
}
