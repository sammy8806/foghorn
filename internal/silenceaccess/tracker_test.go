package silenceaccess

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"foghorn/internal/config"
	"foghorn/internal/model"
)

var t0 = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

func cfg(gran string, onStart bool, interval time.Duration, onDenied bool) config.SilenceCheckConfig {
	return config.SilenceCheckConfig{Granularity: gran, OnStart: &onStart, Interval: &interval, OnDenied: &onDenied}
}

func defaultGrafana() *Tracker {
	return NewTracker("grafana", cfg("folder", true, 15*time.Minute, true))
}

func eq(name, value string) model.Matcher {
	return model.Matcher{Name: name, Value: value, IsEqual: true}
}

func TestApplyLookupFolderScopes(t *testing.T) {
	tr := defaultGrafana()
	tr.ApplyLookup(Lookup{Permissions: map[string][]string{actionCreate: {"folders:uid:a", "folders:uid:a", "folders:uid:b"}}}, t0)
	s := tr.Snapshot()
	if !slices.Equal(s.CreateFolders, []string{"a", "b"}) || s.CanCreateAll || s.Check != "ok" || !s.CheckedAt.Equal(t0) {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestApplyLookupWildcard(t *testing.T) {
	tr := defaultGrafana()
	tr.ApplyLookup(Lookup{Permissions: map[string][]string{actionCreate: {"folders:*"}, actionWrite: {"folders:*"}}}, t0)
	if s := tr.Snapshot(); !s.CanCreateAll || !s.CanWriteAll {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestApplyLookupNoSilencePermissions(t *testing.T) {
	tr := defaultGrafana()
	tr.ApplyLookup(Lookup{Permissions: map[string][]string{actionRead: {"folders:*"}}}, t0)
	s := tr.Snapshot()
	if !s.CanRead || s.CanCreateAll || len(s.CreateFolders) != 0 {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestApplyLookupWithoutRules(t *testing.T) {
	tr := defaultGrafana()
	tr.ApplyLookup(Lookup{Permissions: map[string][]string{}, RuleFolders: nil}, t0)
	s := tr.Snapshot()
	if s.Check != "ok" || s.RuleFolders == nil || len(s.RuleFolders) != 0 {
		t.Fatalf("snapshot %+v", s)
	}
}

func TestLookupErrorKeepsDecisionsOpen(t *testing.T) {
	tr := defaultGrafana()
	tr.ApplyLookup(Lookup{Permissions: map[string][]string{actionCreate: {"folders:*"}}}, t0)
	tr.ApplyLookupError(errors.New("boom"), t0)
	s := tr.Snapshot()
	if s.Check != "error" || s.CheckError != "boom" || s.CanCreateAll {
		t.Fatalf("snapshot %+v", s)
	}
	if !tr.CanReadSilences() {
		t.Fatal("error must not stop reads")
	}
}

func TestLookupUnsupported(t *testing.T) {
	tr := defaultGrafana()
	tr.ApplyLookupError(fmt.Errorf("x: %w", ErrUnsupported), t0)
	if s := tr.Snapshot(); s.Check != "unsupported" {
		t.Fatalf("check %q", s.Check)
	}
	if tr.CheckDue(t0.Add(24 * time.Hour)) {
		t.Fatal("unsupported must never be due")
	}
}

func TestCheckDue(t *testing.T) {
	tr := defaultGrafana()
	if !tr.CheckDue(t0) {
		t.Fatal("on_start: due first")
	}
	tr.ApplyLookup(Lookup{}, t0)
	if tr.CheckDue(t0.Add(14 * time.Minute)) {
		t.Fatal("due before interval")
	}
	if !tr.CheckDue(t0.Add(15 * time.Minute)) {
		t.Fatal("not due after interval")
	}

	once := NewTracker("grafana", cfg("folder", true, 0, true))
	once.CheckDue(t0)
	once.ApplyLookup(Lookup{}, t0)
	if once.CheckDue(t0.Add(48 * time.Hour)) {
		t.Fatal("interval 0 must not repeat")
	}

	late := NewTracker("grafana", cfg("folder", false, 15*time.Minute, true))
	if late.CheckDue(t0) || late.CheckDue(t0.Add(14*time.Minute)) {
		t.Fatal("on_start false must wait")
	}
	if !late.CheckDue(t0.Add(15 * time.Minute)) {
		t.Fatal("on_start false: due after interval")
	}

	if NewTracker("grafana", cfg("off", true, time.Minute, true)).CheckDue(t0) {
		t.Fatal("off must never be due")
	}
	if NewTracker("alertmanager", config.SilenceCheckConfig{}).CheckDue(t0) {
		t.Fatal("alertmanager must never be due")
	}
}

func TestRecordDeniedNarrowing(t *testing.T) {
	newTr := func() *Tracker {
		tr := defaultGrafana()
		tr.ApplyLookup(Lookup{RuleFolders: map[string]string{"r1": "f1"}}, t0)
		return tr
	}
	tr := newTr()
	tr.RecordDenied(OpCreate, []model.Matcher{eq("alertname", "A"), eq(RuleUIDLabel, "r1")}, 403, "nope", t0)
	d := tr.Snapshot().Denied
	if !slices.Equal(d.CreateFolders, []string{"f1"}) || d.CreateGeneral || d.CreateSource {
		t.Fatalf("folder denial %+v", d)
	}
	if d.Status != 403 || d.Message != "nope" || !d.At.Equal(t0) {
		t.Fatalf("details %+v", d)
	}

	tr = newTr()
	tr.RecordDenied(OpCreate, []model.Matcher{eq("alertname", "A")}, 403, "", t0)
	if d := tr.Snapshot().Denied; !d.CreateGeneral || d.CreateSource {
		t.Fatalf("general denial %+v", d)
	}

	tr = newTr()
	tr.RecordDenied(OpCreate, []model.Matcher{eq(RuleUIDLabel, "unknown")}, 403, "", t0)
	if d := tr.Snapshot().Denied; !d.CreateSource {
		t.Fatalf("unknown folder denial %+v", d)
	}

	tr = newTr()
	tr.RecordDenied(OpWrite, nil, 403, "", t0)
	if d := tr.Snapshot().Denied; !d.WriteSource || d.WriteGeneral {
		t.Fatalf("unknown silence denial %+v", d)
	}

	tr = newTr()
	tr.RecordDenied(OpCreate, []model.Matcher{{Name: RuleUIDLabel, Value: "r1", IsEqual: true, IsRegex: true}}, 403, "", t0)
	if d := tr.Snapshot().Denied; !d.CreateGeneral || len(d.CreateFolders) != 0 {
		t.Fatalf("regex rule matcher denial %+v", d)
	}

	tr = newTr()
	tr.RecordDenied(OpWrite, []model.Matcher{eq(RuleUIDLabel, "r1")}, 401, "", t0)
	tr.RecordDenied(OpWrite, []model.Matcher{eq("a", "b")}, 401, "", t0)
	if d := tr.Snapshot().Denied; !slices.Equal(d.WriteFolders, []string{"f1"}) || !d.WriteGeneral || d.CreateGeneral || len(d.CreateFolders) != 0 {
		t.Fatalf("write denial %+v", d)
	}
}

func TestRecordDeniedAlertmanager(t *testing.T) {
	tr := NewTracker("alertmanager", config.SilenceCheckConfig{})
	tr.RecordDenied(OpCreate, []model.Matcher{eq(RuleUIDLabel, "r1")}, 403, "", t0)
	tr.RecordDenied(OpWrite, nil, 403, "", t0)
	if d := tr.Snapshot().Denied; !d.CreateSource || !d.WriteSource {
		t.Fatalf("denial %+v", d)
	}
}

func TestRecordDeniedDisabled(t *testing.T) {
	tr := NewTracker("grafana", cfg("folder", true, time.Minute, false))
	tr.RecordDenied(OpCreate, []model.Matcher{eq("a", "b")}, 403, "", t0)
	if d := tr.Snapshot().Denied; d.CreateGeneral || d.CreateSource || d.Status != 0 {
		t.Fatalf("denial %+v", d)
	}
}

func TestDeniedMakesCheckDue(t *testing.T) {
	tr := defaultGrafana()
	tr.CheckDue(t0)
	tr.ApplyLookup(Lookup{}, t0)
	tr.RecordDenied(OpCreate, []model.Matcher{eq("a", "b")}, 403, "", t0)
	if !tr.CheckDue(t0.Add(time.Second)) {
		t.Fatal("denial must make a lookup due")
	}
	none := NewTracker("grafana", cfg("folder", false, 0, true))
	none.RecordDenied(OpCreate, nil, 403, "", t0)
	if none.CheckDue(t0) {
		t.Fatal("no lookups configured: never due")
	}
}

func TestDenialPrecedence(t *testing.T) {
	p1 := Lookup{Permissions: map[string][]string{actionCreate: {"folders:uid:a"}, "dashboards:read": {"*"}}}
	p1other := Lookup{Permissions: map[string][]string{actionCreate: {"folders:uid:a"}, "dashboards:read": {"x"}}}
	p2 := Lookup{Permissions: map[string][]string{actionCreate: {"folders:*"}}}
	denied := func(tr *Tracker) bool { return tr.Snapshot().Denied.CreateGeneral }

	tr := defaultGrafana()
	tr.ApplyLookup(p1, t0)
	tr.RecordDenied(OpCreate, []model.Matcher{eq("a", "b")}, 403, "", t0)
	tr.ApplyLookup(p1, t0)
	if !denied(tr) {
		t.Fatal("same permissions must keep the denial")
	}
	tr.ApplyLookup(p1other, t0)
	if !denied(tr) {
		t.Fatal("non-silence permission changes must keep the denial")
	}
	tr.ApplyLookup(p2, t0)
	if denied(tr) {
		t.Fatal("changed permissions must clear the denial")
	}

	tr = defaultGrafana()
	tr.RecordDenied(OpCreate, []model.Matcher{eq("a", "b")}, 403, "", t0)
	tr.ApplyLookup(p1, t0)
	if !denied(tr) {
		t.Fatal("first lookup after an unchecked denial is the baseline")
	}
	tr.ApplyLookup(p2, t0)
	if denied(tr) {
		t.Fatal("change after baseline must clear")
	}
}

func TestCanReadSilences(t *testing.T) {
	tr := defaultGrafana()
	if !tr.CanReadSilences() {
		t.Fatal("unchecked: read")
	}
	tr.ApplyLookup(Lookup{Permissions: map[string][]string{actionCreate: {"folders:*"}}}, t0)
	if tr.CanReadSilences() {
		t.Fatal("ok without read permission: skip")
	}
}

func TestSnapshotIsDeepCopy(t *testing.T) {
	tr := defaultGrafana()
	tr.ApplyLookup(Lookup{Permissions: map[string][]string{actionCreate: {"folders:uid:a"}}, RuleFolders: map[string]string{"r": "a"}}, t0)
	s := tr.Snapshot()
	s.CreateFolders[0] = "x"
	s.RuleFolders["r"] = "x"
	if s2 := tr.Snapshot(); s2.CreateFolders[0] != "a" || s2.RuleFolders["r"] != "a" {
		t.Fatal("snapshot shares memory")
	}
}

func TestDenialDuringErrorRebaselines(t *testing.T) {
	p1 := Lookup{Permissions: map[string][]string{actionCreate: {"folders:uid:a"}}}
	p2 := Lookup{Permissions: map[string][]string{actionCreate: {"folders:uid:b"}}}
	tr := defaultGrafana()
	tr.ApplyLookup(p1, t0)
	tr.RecordDenied(OpCreate, []model.Matcher{eq("a", "b")}, 403, "", t0)
	tr.ApplyLookupError(errors.New("down"), t0)
	tr.RecordDenied(OpWrite, []model.Matcher{eq("a", "b")}, 403, "", t0)
	tr.ApplyLookup(p2, t0)
	if !tr.Snapshot().Denied.WriteGeneral {
		t.Fatal("a denial recorded while the check was failing must use the next lookup as its baseline")
	}
}

func TestRecordDeniedDescribesScope(t *testing.T) {
	tr := defaultGrafana()
	tr.ApplyLookup(Lookup{RuleFolders: map[string]string{"r1": "f1"}}, t0)
	cases := []struct {
		op       Op
		matchers []model.Matcher
		want     string
	}{
		{OpCreate, []model.Matcher{eq(RuleUIDLabel, "r1")}, `create in folder "f1"`},
		{OpWrite, []model.Matcher{eq("a", "b")}, "write of general silences"},
		{OpCreate, nil, "create on the whole source"},
	}
	for _, c := range cases {
		if got := tr.RecordDenied(c.op, c.matchers, 403, "", t0); got != c.want {
			t.Errorf("got %q want %q", got, c.want)
		}
	}
	off := NewTracker("grafana", cfg("folder", true, time.Minute, false))
	if got := off.RecordDenied(OpCreate, nil, 403, "", t0); got != "" {
		t.Errorf("on_denied off: got %q", got)
	}
}

func TestDescribeLookup(t *testing.T) {
	tr := defaultGrafana()
	tr.ApplyLookup(Lookup{
		Permissions:   map[string][]string{actionCreate: {"folders:uid:a"}, actionRead: {"folders:*"}},
		RuleFolders:   map[string]string{"r": "a"},
		FolderParents: map[string]string{"a": ""},
	}, t0)
	want := "granularity=folder check=ok create=folders[a] write=none read=yes rules=1 folders=1"
	if got := tr.Describe(); got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	tr.ApplyLookupError(errors.New("HTTP 500"), t0)
	if got := tr.Describe(); got != `granularity=folder check=error error="HTTP 500"` {
		t.Fatalf("got %q", got)
	}
}

func TestFolderStructureSurvivesFailedLookups(t *testing.T) {
	tr := defaultGrafana()
	tr.ApplyLookup(Lookup{RuleFolders: map[string]string{"r1": "f1"}, FolderParents: map[string]string{"f1": ""}}, t0)
	tr.RecordDenied(OpCreate, []model.Matcher{eq(RuleUIDLabel, "r1")}, 403, "", t0)
	tr.ApplyLookupError(errors.New("down"), t0)
	if s := tr.Snapshot(); s.RuleFolders["r1"] != "f1" {
		t.Fatalf("rule folders lost on error: %+v", s.RuleFolders)
	}
	tr.RecordDenied(OpCreate, []model.Matcher{eq(RuleUIDLabel, "r1")}, 403, "", t0)
	if d := tr.Snapshot().Denied; d.CreateSource {
		t.Fatal("refusal during a failed lookup widened to the whole source")
	}
	tr.ApplyLookup(Lookup{RuleFolders: nil, FolderParents: nil}, t0)
	if s := tr.Snapshot(); s.RuleFolders["r1"] != "f1" || s.FolderParents == nil {
		t.Fatalf("unreadable rules/folders must keep the last known structure: %+v", s)
	}
}

func TestRequestCheckMakesCheckDue(t *testing.T) {
	tr := defaultGrafana()
	tr.CheckDue(t0)
	tr.ApplyLookup(Lookup{}, t0)
	if tr.CheckDue(t0.Add(time.Minute)) {
		t.Fatal("due before interval")
	}
	tr.RequestCheck()
	if !tr.CheckDue(t0.Add(time.Minute)) {
		t.Fatal("a requested check must be due at once")
	}
	tr.ApplyLookup(Lookup{}, t0.Add(time.Minute))
	if tr.CheckDue(t0.Add(2 * time.Minute)) {
		t.Fatal("a lookup must satisfy the request")
	}

	for name, off := range map[string]*Tracker{
		"granularity off":     NewTracker("grafana", cfg("off", true, time.Minute, true)),
		"no lookups":          NewTracker("grafana", cfg("folder", false, 0, true)),
		"alertmanager source": NewTracker("alertmanager", config.SilenceCheckConfig{}),
	} {
		off.RequestCheck()
		if off.CheckDue(t0) {
			t.Fatalf("%s: a request must not enable lookups", name)
		}
	}
}

func TestClearDenials(t *testing.T) {
	tr := defaultGrafana()
	tr.CheckDue(t0)
	tr.ApplyLookup(Lookup{RuleFolders: map[string]string{"r1": "f1"}}, t0)
	tr.RecordDenied(OpWrite, []model.Matcher{eq("__alert_rule_uid__", "r1")}, 403, "nope", t0)
	tr.ApplyLookup(Lookup{RuleFolders: map[string]string{"r1": "f1"}}, t0.Add(time.Second))
	if got := tr.Snapshot().Denied.WriteFolders; !slices.Equal(got, []string{"f1"}) {
		t.Fatalf("setup: denial not held, got %v", got)
	}

	tr.ClearDenials()
	if d := tr.Snapshot().Denied; d.Status != 0 || len(d.WriteFolders) != 0 {
		t.Fatalf("denial survived ClearDenials: %+v", d)
	}
	if !tr.CheckDue(t0.Add(2 * time.Second)) {
		t.Fatal("clearing denials must re-check permissions")
	}
	// Same permissions as before: the cleared denial must not come back.
	tr.ApplyLookup(Lookup{RuleFolders: map[string]string{"r1": "f1"}}, t0.Add(3*time.Second))
	if d := tr.Snapshot().Denied; d.Status != 0 {
		t.Fatalf("denial came back after the next lookup: %+v", d)
	}
}
