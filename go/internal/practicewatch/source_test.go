package practicewatch

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func boolRef(b bool) *bool { return &b }

func TestDefaults_CoverTheSubscribedCLIsAndKeepTheOptionalOnesOff(t *testing.T) {
	on := map[string]string{}
	off := map[string]string{}
	for _, s := range Defaults() {
		if s.Enabled {
			on[s.ID] = s.Family
		} else {
			off[s.ID] = s.Family
		}
	}

	for id, family := range map[string]string{"claude-code-changelog": "claude", "agy-cli-changelog-command": "agy"} {
		if on[id] != family {
			t.Errorf("default source %s: family %q on, want %q", id, on[id], family)
		}
	}
	for _, family := range on {
		if family != "claude" && family != "agy" {
			t.Errorf("a default source is on for %q, which the operator does not subscribe to", family)
		}
	}
	if off["codex-releases"] != "codex" || off["gemini-cli-releases"] != "gemini" {
		t.Errorf("optional sources off by default = %v, want codex and gemini", off)
	}
}

func TestDefaults_EachSourceIsValid(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range Defaults() {
		if err := s.validate(); err != nil {
			t.Errorf("default %s: %v", s.ID, err)
		}
		if seen[s.ID] {
			t.Errorf("duplicate default id %s", s.ID)
		}
		seen[s.ID] = true
	}
}

func TestResolve_APolicyEntryWithADefaultIDOverridesOnlyTheFieldsItSets(t *testing.T) {
	defaults := []Source{{ID: "a", Family: "claude", Kind: KindPage, Locator: "https://x.test/a", Enabled: false}}

	got, err := Resolve(defaults, []policy.PracticeWatchSource{{ID: "a", Enabled: boolRef(true), Locator: "https://y.test/a"}})

	if err != nil {
		t.Fatal(err)
	}
	want := Source{ID: "a", Family: "claude", Kind: KindPage, Locator: "https://y.test/a", Enabled: true}
	if len(got) != 1 || got[0] != want {
		t.Errorf("Resolve = %+v, want [%+v]", got, want)
	}
}

func TestResolve_ANewPolicyIDIsAppendedAndEnabledByDefault(t *testing.T) {
	got, err := Resolve(nil, []policy.PracticeWatchSource{{ID: "mine", Family: "claude", Kind: "page", Locator: "https://x.test/"}})

	if err != nil || len(got) != 1 || !got[0].Enabled || got[0].Kind != KindPage {
		t.Errorf("Resolve = %+v, %v", got, err)
	}
}

func TestResolve_RefusesASourceThatCannotRun(t *testing.T) {
	for name, tc := range map[string]struct {
		entry policy.PracticeWatchSource
		want  string
	}{
		"unknown kind":    {policy.PracticeWatchSource{ID: "n", Family: "claude", Kind: "rss", Locator: "https://x.test"}, `kind "rss"`},
		"missing locator": {policy.PracticeWatchSource{ID: "n", Family: "claude", Kind: "page"}, "locator"},
		"missing family":  {policy.PracticeWatchSource{ID: "n", Kind: "page", Locator: "https://x.test"}, "family"},
		"unsafe id":       {policy.PracticeWatchSource{ID: "../x", Family: "claude", Kind: "page", Locator: "https://x.test"}, "kebab-case"},
		"file scheme":     {policy.PracticeWatchSource{ID: "n", Family: "claude", Kind: "page", Locator: "file:///etc/passwd"}, "http"},
		"blank command":   {policy.PracticeWatchSource{ID: "n", Family: "claude", Kind: "command", Locator: "  "}, "locator"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Resolve(nil, []policy.PracticeWatchSource{tc.entry})

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
