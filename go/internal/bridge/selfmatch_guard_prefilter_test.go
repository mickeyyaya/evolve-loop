package bridge

import (
	"reflect"
	"strings"
	"testing"
)

const plantedFoldCaseRule = `(?i)planted needle\s*\z`

func selfMatchFixtureRules(t *testing.T) []ManifestPrompt {
	t.Helper()
	_, shaped, catchAll := codexUpdateMenuRules(t)
	claude, err := LoadManifest("claude-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	codex, err := LoadManifest("codex-tmux")
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	rules := append(append([]ManifestPrompt(nil), shaped...), catchAll)
	for _, p := range append(claude.InteractivePrompts, codex.InteractivePrompts...) {
		if strings.HasPrefix(p.Name, "trust_prompt") {
			rules = append(rules, p)
		}
	}
	return append(rules,
		ManifestPrompt{Name: "planted_fold_case", Regex: plantedFoldCaseRule, TailLines: 4, Policy: "hold"},
		ManifestPrompt{Name: "invalid_regex", Regex: `(`, TailLines: 4, Policy: "hold"},
		ManifestPrompt{Name: "empty_regex", TailLines: 4, Policy: "hold"},
	)
}

func selfMatchFixtureBody() string {
	prose := "a CHANGELOG line that names Update now, Press enter to continue and Update available! in prose"
	quiet := strings.Repeat("an unrelated line\n", selfMatchPaneDepth+1)
	planted := []string{
		codexUpdateMenuFrame(codexMenuSkip),
		codexUpdateMenuFrame(codexMenuUpdateNow),
		codexUpdateMenuFrameWith(codexVariantMenuFooter, codexMenuUpdateNow),
		codexTrustDialogFrame,
		"Quick safety check: Is this a project you created or one you trust?\n ❯ 1. Yes, I trust this folder\n   2. No, exit\n Enter to confirm · Esc to cancel",
		"   No, exit\n ❯ Yes, I trust this folder\n Enter to confirm · Esc to cancel",
		"❯ No, exit\n  Yes, I trust this folder\n Enter to confirm · Esc to cancel",
		"A PLANTED NEEDLE",
	}
	return strings.Join(planted, "\n"+prose+"\n") + "\n" + quiet + prose + "\n" + quiet
}

func TestSelfMatchGuard_ThePrefilterGivesTheExhaustiveVerdictForEveryRule(t *testing.T) {
	t.Parallel()
	rules := selfMatchFixtureRules(t)
	body := selfMatchFixtureBody()
	neverFires := map[string]bool{"invalid_regex": true, "empty_regex": true}

	for _, p := range rules {
		fast := firesAtPaneBottoms(body, []ManifestPrompt{p})
		slow := firesAtPaneBottomsExhaustively(body, []ManifestPrompt{p})

		if !reflect.DeepEqual(fast, slow) {
			t.Errorf("rule %s: the prefiltered guard gives %v, the exhaustive guard gives %v", p.Name, fast, slow)
		}
		if neverFires[p.Name] != (len(slow) == 0) {
			t.Errorf("rule %s fires at %v on the fixture; the fixture must plant a pane that each valid rule matches, or this test proves nothing", p.Name, slow)
		}
	}
	if fast, slow := firesAtPaneBottoms(body, rules), firesAtPaneBottomsExhaustively(body, rules); !reflect.DeepEqual(fast, slow) {
		t.Errorf("all rules together: the prefiltered guard gives %v, the exhaustive guard gives %v", fast, slow)
	}
}

func TestSelfMatchGuard_RequiredLiteralsAreOnlyWhatEveryMatchContains(t *testing.T) {
	t.Parallel()
	cases := []struct {
		expr string
		want []string
	}{
		{`(?m)^[ \t]*Press enter to continue\s*\z`, []string{"Press enter to continue"}},
		{`x(?:abc|de)y`, []string{"abc", "de"}},
		{`(?:abc)?z`, []string{"z"}},
		{`(?:abc){2,3}`, []string{"abc"}},
		{`(?:abc){0,3}`, nil},
		{`(abcd)+e`, []string{"abcd"}},
		{`abc|[0-9]+`, nil},
		{`(?i)abc`, nil},
		{"abc\ndef", nil},
		{`(`, nil},
		{``, nil},
	}
	for _, tc := range cases {
		if got := requiredLiterals(tc.expr); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("requiredLiterals(%q) = %q, want %q", tc.expr, got, tc.want)
		}
	}
}
