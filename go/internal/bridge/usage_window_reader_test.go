package bridge

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func usagePane(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "quotastate", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var usageNow = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

func exhaustedTargets(family, pane string) map[string]bool {
	out := map[string]bool{}
	for _, w := range UsageWindows(family, pane, usageNow) {
		key := w.Family
		if w.Model != "" {
			key += ":" + w.Model
		}
		out[key] = out[key] || w.Exhausted
	}
	return out
}

func TestUsageWindows_EachCLIReadsItsOwnScreenThroughItsManifest(t *testing.T) {
	for _, tc := range []struct {
		family, fixture string
		want            map[string]bool
	}{
		{"agy", "agy_usage_groups.txt", map[string]bool{"agy": false, "agy-claude": false}},
		{"agy", "agy_usage_claude_drained.txt", map[string]bool{"agy": false, "agy-claude": true}},
		{"agy", "agy_usage_gemini_drained.txt", map[string]bool{"agy": true, "agy-claude": false}},
		{"agy", "agy_usage_legacy_layout.txt", map[string]bool{"agy": true, "agy-claude": false}},
		{"claude", "claude_usage_2.1.291.txt", map[string]bool{"claude": false, "claude:Fable": false}},
		{"claude", "claude_usage_session_exhausted.txt", map[string]bool{"claude": true, "claude:Fable": false}},
		{"claude", "claude_usage_week_exhausted.txt", map[string]bool{"claude": true, "claude:Fable": false}},
		{"claude", "claude_usage_fable_exhausted.txt", map[string]bool{"claude": false, "claude:Fable": true}},
		{"claude", "claude_usage.txt", map[string]bool{"claude": false, "claude:Fable": true}},
	} {
		if got := exhaustedTargets(tc.family, usagePane(t, tc.fixture)); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s %s: target → exhausted = %v, want %v", tc.family, tc.fixture, got, tc.want)
		}
	}
}

func TestUsageWindows_TheWholeFamilyRegexIsBlindToTheseDrainedScreens(t *testing.T) {
	for family, fixtures := range map[string][]string{
		"agy":    {"agy_usage_claude_drained.txt", "agy_usage_gemini_drained.txt"},
		"claude": {"claude_usage_session_exhausted.txt", "claude_usage_week_exhausted.txt"},
	} {
		for _, fixture := range fixtures {
			if ClassifyExhausted(family, usagePane(t, fixture)) {
				t.Errorf("%s %s: exhausted_regex matched; the window reader would not be the only signal", family, fixture)
			}
		}
	}
}

func TestUsageWindows_AFamilyWhoseManifestDeclaresNoWindowsHasNone(t *testing.T) {
	for _, family := range []string{"codex", "ollama", "no-such"} {
		if got := UsageWindows(family, usagePane(t, "claude_usage_week_exhausted.txt"), usageNow); len(got) != 0 {
			t.Errorf("%s: windows = %+v; only a manifest's usage.windows reads windows", family, got)
		}
	}
}

func TestUsageWindows_AgyClaudeInheritsTheWindowSpecThroughItsBase(t *testing.T) {
	base, err := LoadManifest("agy-tmux")
	if err != nil {
		t.Fatal(err)
	}
	derived, err := LoadManifest("agy-claude-tmux")
	if err != nil {
		t.Fatal(err)
	}
	b, d := base.Controls["usage"].Windows, derived.Controls["usage"].Windows
	if b == nil || !reflect.DeepEqual(b, d) {
		t.Fatalf("agy-claude-tmux's usage.windows = %+v, want agy-tmux's %+v", d, b)
	}
}

func TestParseManifest_RefusesABrokenUsageWindowSpec(t *testing.T) {
	valid := `"labels":[{"regex":"^Current session$","kind":"session","scope":"session"}],"value_regex":"(?P<pct>\\d+)% used","value_direction":"used","scopes":{"session":{"family":"x"}}`
	for field, windows := range map[string]string{
		"value_regex":     strings.Replace(valid, `(?P<pct>\\d+)`, `(\\d+)`, 1),
		"value_direction": strings.Replace(valid, `"used","scopes"`, `"spent","scopes"`, 1),
		"labels":          strings.Replace(valid, `"kind":"session"`, `"kind":"fortnight"`, 1),
		"section_regex":   valid + `,"section_regex":"^[A-Z]+$"`,
		"reset_regex":     valid + `,"reset_regex":"Resets (.+)"`,
		"scopes":          strings.Replace(valid, `{"family":"x"}`, `{"family":""}`, 1),
	} {
		body := `{"cli":"x-tmux","binary":"x","controls":{"usage":{"send":"/usage","windows":{` + windows + `}}}}`
		_, err := parseManifestWithStderr("x-tmux", []byte(body), &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "controls.usage.windows."+field) {
			t.Errorf("a broken %s parsed (err=%v); want a refusal naming the field", field, err)
		}
	}
	body := `{"cli":"x-tmux","binary":"x","controls":{"usage":{"send":"/usage","windows":{` + valid + `}}}}`
	if _, err := parseManifestWithStderr("x-tmux", []byte(body), &bytes.Buffer{}); err != nil {
		t.Errorf("a well-formed window spec was refused: %v", err)
	}
}
