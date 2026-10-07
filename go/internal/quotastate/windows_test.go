package quotastate

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var agyWindowSpec = WindowSpec{
	SectionRegex: `^\s*(?P<scope>[A-Z][A-Z0-9 &/-]*MODELS)\s*$`,
	ModelsRegex:  `Models within this group:\s*(?P<models>.+?)\s*$`,
	Labels: []WindowLabel{
		{Regex: `^\s*Weekly Limit Remaining\s*$`, Kind: KindWeek},
		{Regex: `^\s*Five Hour Limit Remaining\s*$`, Kind: KindFiveHour},
	},
	ValueRegex:         `(?P<pct>\d{1,3}(?:\.\d+)?)%`,
	ValueDirection:     DirectionRemaining,
	ResetRegex:         `(?i)refreshes in\s+(?P<reset>\d+\s*[hms](?:\s*\d+\s*[hms])*)`,
	ExhaustedAtUsedPct: 99.5,
	Scopes: map[string]ScopeTarget{
		"GEMINI MODELS":         {Family: "agy"},
		"CLAUDE AND GPT MODELS": {Family: "agy-claude"},
	},
}

var claudeWindowSpec = WindowSpec{
	Labels: []WindowLabel{
		{Regex: `^\s*Current session\s*$`, Kind: KindSession, Scope: "session"},
		{Regex: `^\s*Current week \((?P<scope>[^)]+)\)\s*$`, Kind: KindWeek},
	},
	ValueRegex:     `(?P<pct>\d{1,3}(?:\.\d+)?)%\s*used`,
	ValueDirection: DirectionUsed,
	ResetRegex:     `^\s*Resets\s+(?P<reset>.+?)\s*$`,
	Scopes: map[string]ScopeTarget{
		"session":    {Family: "claude"},
		"all models": {Family: "claude"},
		AnyScope:     {Family: "claude", PerModel: true},
	},
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func taipei(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		t.Skipf("no tz database: %v", err)
	}
	return loc
}

func at(t time.Time) *time.Time { return &t }

func TestReadWindows_ClaudeCode2_1_291ReadsEveryWindowAsPercentUsedWithItsReset(t *testing.T) {
	loc := taipei(t)
	now := time.Date(2026, 10, 6, 20, 0, 0, 0, loc)

	got := ReadWindows(claudeWindowSpec, fixture(t, "claude_usage_2.1.291.txt"), now)

	want := []UsageWindow{
		{Scope: "session", Kind: KindSession, PercentUsed: 7, ResetsText: "12:40am (Asia/Taipei)", ResetsAt: at(time.Date(2026, 10, 7, 0, 40, 0, 0, loc).UTC()), Family: "claude"},
		{Scope: "all models", Kind: KindWeek, PercentUsed: 64, ResetsText: "Oct 11 at 9pm (Asia/Taipei)", ResetsAt: at(time.Date(2026, 10, 11, 21, 0, 0, 0, loc).UTC()), Family: "claude"},
		{Scope: "Fable", Kind: KindWeek, PercentUsed: 0, ResetsText: "Oct 11 at 9pm (Asia/Taipei)", ResetsAt: at(time.Date(2026, 10, 11, 21, 0, 0, 0, loc).UTC()), Family: "claude", Model: "Fable"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("windows =\n%+v\nwant\n%+v", got, want)
	}
}

func TestReadWindows_EachClaudeWindowAtOneHundredPercentUsedIsExhausted(t *testing.T) {
	now := time.Date(2026, 10, 6, 20, 0, 0, 0, taipei(t))
	for fixtureName, wantScope := range map[string]string{
		"claude_usage_session_exhausted.txt": "session",
		"claude_usage_week_exhausted.txt":    "all models",
		"claude_usage_fable_exhausted.txt":   "Fable",
	} {
		var exhausted []string
		for _, w := range ReadWindows(claudeWindowSpec, fixture(t, fixtureName), now) {
			if w.Exhausted {
				exhausted = append(exhausted, w.Scope)
			}
		}
		if !reflect.DeepEqual(exhausted, []string{wantScope}) {
			t.Errorf("%s: exhausted scopes = %q, want [%s]", fixtureName, exhausted, wantScope)
		}
	}
	for _, w := range ReadWindows(claudeWindowSpec, fixture(t, "claude_usage_2.1.291.txt"), now) {
		if w.Exhausted {
			t.Errorf("the live screen has no drained window, yet %+v is exhausted", w)
		}
	}
}

func TestReadWindows_AgyGroupsNormalizeRemainingToUsedAndCarryTheirModels(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

	got := ReadWindows(agyWindowSpec, fixture(t, "agy_usage_claude_drained.txt"), now)

	gemini, claude := []string{"Gemini Flash", "Gemini Pro"}, []string{"Claude Opus", "Claude Sonnet", "GPT-OSS"}
	want := []UsageWindow{
		{Scope: "GEMINI MODELS", Kind: KindWeek, PercentUsed: 8.3, ResetsText: "140h 56m", ResetsAt: at(now.Add(140*time.Hour + 56*time.Minute)), Models: gemini, Family: "agy"},
		{Scope: "GEMINI MODELS", Kind: KindFiveHour, PercentUsed: 0, Models: gemini, Family: "agy"},
		{Scope: "CLAUDE AND GPT MODELS", Kind: KindWeek, PercentUsed: 27.32, ResetsText: "163h 40m", ResetsAt: at(now.Add(163*time.Hour + 40*time.Minute)), Models: claude, Family: "agy-claude"},
		{Scope: "CLAUDE AND GPT MODELS", Kind: KindFiveHour, PercentUsed: 100, ResetsText: "2h 15m", ResetsAt: at(now.Add(2*time.Hour + 15*time.Minute)), Models: claude, Family: "agy-claude", Exhausted: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("windows =\n%+v\nwant\n%+v", got, want)
	}
}

func TestReadWindows_AgyFixturesDrainOnlyTheirOwnGroup(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	for name, want := range map[string]map[string]bool{
		"agy_usage_groups.txt":         {"agy": false, "agy-claude": false},
		"agy_usage_claude_drained.txt": {"agy": false, "agy-claude": true},
		"agy_usage_gemini_drained.txt": {"agy": true, "agy-claude": false},
		"agy_usage_legacy_layout.txt":  {"agy": true, "agy-claude": false},
	} {
		got := map[string]bool{}
		for _, w := range ReadWindows(agyWindowSpec, fixture(t, name), now) {
			got[w.Family] = got[w.Family] || w.Exhausted
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: family → drained = %v, want %v", name, got, want)
		}
	}
}

func TestReadWindows_TheOlderAgyLayoutReadsTheBarValueNotItsRoundedEcho(t *testing.T) {
	w := ReadWindows(agyWindowSpec, fixture(t, "agy_usage_legacy_layout.txt"), time.Now())[0]

	if w.PercentUsed != 99.91 || w.ResetsText != "45h 12m" || !w.Exhausted {
		t.Fatalf("weekly window = %+v; want 99.91%% used from the 0.09%% bar value, refresh 45h 12m, exhausted at the 99.5 threshold", w)
	}
}

func TestReadWindows_TheThresholdIsTheSpecsOrOneHundred(t *testing.T) {
	pane := "Current session\n  99% used\nResets 4:10pm\n"
	for threshold, want := range map[float64]bool{0: false, 100: false, 99: true} {
		spec := claudeWindowSpec
		spec.ExhaustedAtUsedPct = threshold
		if got := ReadWindows(spec, pane, time.Now())[0].Exhausted; got != want {
			t.Errorf("threshold %v: a 99%%-used window exhausted = %v, want %v", threshold, got, want)
		}
	}
}

func TestReadWindows_AValueAndResetOnTheLabelLineAreRead(t *testing.T) {
	spec := WindowSpec{
		Labels:         []WindowLabel{{Regex: `^\s*5h limit:`, Kind: KindFiveHour, Scope: "account"}},
		ValueRegex:     `(?P<pct>\d+)% left`,
		ValueDirection: DirectionRemaining,
		ResetRegex:     `\(resets (?P<reset>[^)]+)\)`,
		Scopes:         map[string]ScopeTarget{"account": {Family: "codex"}},
	}

	got := ReadWindows(spec, "5h limit: [░░░░] 0% left (resets 14:39)\n", time.Now())

	if len(got) != 1 || got[0].PercentUsed != 100 || got[0].ResetsText != "14:39" || !got[0].Exhausted || got[0].Family != "codex" || got[0].ResetsAt != nil {
		t.Fatalf("windows = %+v; want one exhausted codex window whose unparsed reset leaves ResetsAt nil", got)
	}
}

func TestReadWindows_AnUnmappedScopeHasNoFamilyAndNoWindowsComeFromAnotherLayoutOrABrokenSpec(t *testing.T) {
	orphan := ReadWindows(agyWindowSpec, "OTHER MODELS\n  Weekly Limit Remaining\n    [░░] 0.00%\n", time.Now())
	if len(orphan) != 1 || orphan[0].Family != "" || !orphan[0].Exhausted {
		t.Errorf("windows = %+v; want one drained window no scope maps", orphan)
	}
	broken := claudeWindowSpec
	broken.ValueRegex = "("
	noDirection := claudeWindowSpec
	noDirection.ValueDirection = ""
	for name, tc := range map[string]struct {
		spec WindowSpec
		pane string
	}{
		"agy's screen under claude's spec": {claudeWindowSpec, fixture(t, "agy_usage_groups.txt")},
		"claude's screen under agy's spec": {agyWindowSpec, fixture(t, "claude_usage_2.1.291.txt")},
		"no spec":                          {WindowSpec{}, fixture(t, "claude_usage_2.1.291.txt")},
		"a bad regex":                      {broken, fixture(t, "claude_usage_2.1.291.txt")},
		"no value direction":               {noDirection, fixture(t, "claude_usage_2.1.291.txt")},
	} {
		if got := ReadWindows(tc.spec, tc.pane, time.Now()); len(got) != 0 {
			t.Errorf("%s: windows = %+v, want none", name, got)
		}
	}
}

func TestStatesOf_GroupsWindowsByFamilyIntoTheBucketsTheBudgetReads(t *testing.T) {
	loc := taipei(t)
	now := time.Date(2026, 10, 6, 20, 0, 0, 0, loc)
	windows := append(ReadWindows(claudeWindowSpec, fixture(t, "claude_usage_fable_exhausted.txt"), now),
		ReadWindows(agyWindowSpec, fixture(t, "agy_usage_groups.txt"), now)...)

	states := StatesOf(windows, now)

	byFamily := map[string]QuotaState{}
	for _, q := range states {
		byFamily[q.Family] = q
	}
	claude := byFamily["claude"]
	names := []string{}
	for _, b := range claude.Buckets {
		names = append(names, b.Name)
	}
	if !reflect.DeepEqual(names, []string{"session", "week"}) || claude.Exhausted || claude.Source != SourceProbed {
		t.Fatalf("claude state = %+v; want session and week only, not exhausted: a drained Fable week is no family quota", claude)
	}
	if got, ok := claude.TightestRemaining("session", "week"); !ok || !approx(got, 0.36) {
		t.Errorf("TightestRemaining(session, week) = %v, %v; want the weekly 0.36", got, ok)
	}
	if want := time.Date(2026, 10, 11, 21, 0, 0, 0, loc); !claude.Buckets[1].ResetAt.Equal(want) {
		t.Errorf("week ResetAt = %v, want %v", claude.Buckets[1].ResetAt, want)
	}
	if len(states) != 3 || byFamily["agy"].Exhausted || byFamily["agy-claude"].Exhausted {
		t.Errorf("states = %+v; want claude, agy and agy-claude, the agy ones healthy", states)
	}
}

func TestUsageWindow_EvidenceNamesTheScopeKindUseAndReset(t *testing.T) {
	for w, want := range map[*UsageWindow]string{
		{Scope: "Fable", Kind: KindWeek, PercentUsed: 100, ResetsText: "Oct 11 at 9pm (Asia/Taipei)"}: "Fable week window 100% used, resets Oct 11 at 9pm (Asia/Taipei)",
		{Scope: "CLAUDE AND GPT MODELS", Kind: KindFiveHour, PercentUsed: 99.91}:                      "CLAUDE AND GPT MODELS 5h window 99.91% used, resets at a time the screen does not state",
	} {
		if got := w.Evidence(); got != want {
			t.Errorf("Evidence() = %q, want %q", got, want)
		}
	}
}

func TestUsageWindow_ExhaustsFamilyOnlyForAnExhaustedFamilyScopedWindow(t *testing.T) {
	for _, tc := range []struct {
		w    UsageWindow
		want bool
	}{
		{UsageWindow{Scope: "all models", Family: "claude", Exhausted: true}, true},
		{UsageWindow{Scope: "Fable", Family: "claude", Model: "Fable", Exhausted: true}, false},
		{UsageWindow{Scope: "session", Family: "claude"}, false},
		{UsageWindow{Scope: "Fable", Family: "claude", Model: "Fable"}, false},
	} {
		if got := tc.w.ExhaustsFamily(); got != tc.want {
			t.Errorf("%+v.ExhaustsFamily() = %v, want %v", tc.w, got, tc.want)
		}
	}
}

func TestWindowSpec_ValidateNamesTheBrokenFieldAndReadWindowsReadsNothingWithIt(t *testing.T) {
	broken := func(edit func(*WindowSpec)) WindowSpec {
		spec := claudeWindowSpec
		spec.Labels = append([]WindowLabel(nil), claudeWindowSpec.Labels...)
		spec.Scopes = map[string]ScopeTarget{"session": {Family: "claude"}}
		edit(&spec)
		return spec
	}
	for field, spec := range map[string]WindowSpec{
		"value_regex":     broken(func(s *WindowSpec) { s.ValueRegex = `(\d+)% used` }),
		"section_regex":   broken(func(s *WindowSpec) { s.SectionRegex = `^[A-Z]+$` }),
		"models_regex":    broken(func(s *WindowSpec) { s.ModelsRegex = `(` }),
		"reset_regex":     broken(func(s *WindowSpec) { s.ResetRegex = `Resets (.+)` }),
		"value_direction": broken(func(s *WindowSpec) { s.ValueDirection = "spent" }),
		"labels[1].kind":  broken(func(s *WindowSpec) { s.Labels[1].Kind = "fortnight" }),
		"labels[0].regex": broken(func(s *WindowSpec) { s.Labels[0].Regex = "(" }),
		"scopes":          broken(func(s *WindowSpec) { s.Scopes["session"] = ScopeTarget{} }),
	} {
		err := spec.Validate()
		if err == nil || !strings.HasPrefix(err.Error(), field) {
			t.Errorf("a broken %s validated as %v; want an error starting with the field", field, err)
		}
		if got := ReadWindows(spec, fixture(t, "claude_usage_2.1.291.txt"), time.Now()); len(got) != 0 {
			t.Errorf("%s: ReadWindows read %d windows through a spec Validate refuses", field, len(got))
		}
	}
	if err := claudeWindowSpec.Validate(); err != nil {
		t.Errorf("the claude spec does not validate: %v", err)
	}
}
