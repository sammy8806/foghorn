package config

import "testing"

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
