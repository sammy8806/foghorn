package config

import (
	"errors"
	"fmt"
	"testing"
)

func TestDiagnosticsDropAndSubstitute(t *testing.T) {
	var diags Diagnostics
	diags.Drop(`resolvers[0] "cluster-name"`, "templates in %s are no longer supported", "args[1]")
	diags.Substitute("ui.popup_position", "%q is not valid; using top_right", "middle")

	if len(diags) != 2 {
		t.Fatalf("len(diags) = %d, want 2", len(diags))
	}
	if !diags[0].Dropped {
		t.Errorf("Drop() produced Dropped = false, want true")
	}
	if diags[0].Message != "templates in args[1] are no longer supported" {
		t.Errorf("Message = %q, want the formatted message", diags[0].Message)
	}
	if diags[1].Dropped {
		t.Errorf("Substitute() produced Dropped = true, want false")
	}
}

func TestFingerprintIsEmptyForNoDiagnostics(t *testing.T) {
	var diags Diagnostics
	if got := diags.Fingerprint(); got != "" {
		t.Fatalf("Fingerprint() = %q, want empty string", got)
	}
}

func TestFingerprintIgnoresOrder(t *testing.T) {
	a := Diagnostics{{Field: "x", Message: "one", Dropped: true}, {Field: "y", Message: "two"}}
	b := Diagnostics{{Field: "y", Message: "two"}, {Field: "x", Message: "one", Dropped: true}}

	if a.Fingerprint() != b.Fingerprint() {
		t.Fatalf("Fingerprint() differs by order: %q vs %q", a.Fingerprint(), b.Fingerprint())
	}
}

func TestFingerprintIgnoresPositionalIndices(t *testing.T) {
	a := Diagnostics{{Field: `sources[1] "production"`, Message: "type is required", Dropped: true}}
	b := Diagnostics{{Field: `sources[8] "production"`, Message: "type is required", Dropped: true}}

	if a.Fingerprint() != b.Fingerprint() {
		t.Fatalf("Fingerprint() differs by display index: %q vs %q", a.Fingerprint(), b.Fingerprint())
	}
}

func TestFingerprintChangesWithContent(t *testing.T) {
	a := Diagnostics{{Field: "x", Message: "one"}}
	b := Diagnostics{{Field: "x", Message: "two"}}
	c := Diagnostics{{Field: "x", Message: "one", Dropped: true}}

	if a.Fingerprint() == b.Fingerprint() {
		t.Errorf("Fingerprint() ignored a message change")
	}
	if a.Fingerprint() == c.Fingerprint() {
		t.Errorf("Fingerprint() ignored a Dropped change")
	}
}

func TestDescribeLoadFailure(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantLocator string
		wantMessage string
	}{
		{
			name:        "yaml syntax error carries its line",
			err:         fmt.Errorf("parsing config: %w", errors.New("yaml: line 2: mapping values are not allowed in this context")),
			wantLocator: "line 2",
			wantMessage: "mapping values are not allowed in this context",
		},
		{
			name:        "unclosed flow sequence",
			err:         fmt.Errorf("parsing config: %w", errors.New("yaml: line 1: did not find expected ',' or ']'")),
			wantLocator: "line 1",
			wantMessage: "did not find expected ',' or ']'",
		},
		{
			name:        "a single type error loses the group header",
			err:         fmt.Errorf("parsing config: %w", errors.New("yaml: unmarshal errors:\n  line 2: cannot unmarshal !!str `notanint` into int")),
			wantLocator: "line 2",
			wantMessage: "cannot unmarshal !!str `notanint` into int",
		},
		{
			name:        "several type errors keep every line and point at no single one",
			err:         fmt.Errorf("parsing config: %w", errors.New("yaml: unmarshal errors:\n  line 1: cannot unmarshal !!int `3` into []string\n  line 3: cannot unmarshal !!str `nope` into int")),
			wantLocator: "",
			wantMessage: "line 1: cannot unmarshal !!int `3` into []string; line 3: cannot unmarshal !!str `nope` into int",
		},
		{
			name:        "an unreadable file has no position to point at",
			err:         fmt.Errorf("reading config: %w", errors.New("open /tmp/foghorn/config.yaml: no such file or directory")),
			wantLocator: "",
			wantMessage: "open /tmp/foghorn/config.yaml: no such file or directory",
		},
		{
			name:        "validation failures are unwrapped too",
			err:         fmt.Errorf("validating config: %w", errors.New("sources: at least one source is required")),
			wantLocator: "",
			wantMessage: "sources: at least one source is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			locator, message := DescribeLoadFailure(tc.err)
			if locator != tc.wantLocator || message != tc.wantMessage {
				t.Fatalf("got (%q, %q), want (%q, %q)", locator, message, tc.wantLocator, tc.wantMessage)
			}
		})
	}
}

func TestFingerprintSeparatesProblemsByLocator(t *testing.T) {
	// The line number moved out of Message and into Locator, so it has to stay
	// part of the dismissal identity: fixing line 3 only to break line 9 must
	// show the card again rather than stay dismissed.
	atLine3 := Diagnostics{{Field: "config", Locator: "line 3", Message: "mapping values are not allowed in this context"}}
	atLine9 := Diagnostics{{Field: "config", Locator: "line 9", Message: "mapping values are not allowed in this context"}}
	if atLine3.Fingerprint() == atLine9.Fingerprint() {
		t.Fatal("the same message at a different line must not share a fingerprint")
	}
}
