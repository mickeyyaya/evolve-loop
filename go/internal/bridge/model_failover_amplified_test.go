package bridge

import (
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestLaunch_AmplifiedAuditorStepsOpusFableSonnetWhenOpusLeadsTheChain(t *testing.T) {
	useLiveClaudeDeepChain(t, "opus", []string{"fable", "opus", "sonnet"})
	s, resp, err, _ := launchAuditorAtDeep(t, map[string]core.BridgeResponse{"opus": wall, "fable": wall, "sonnet": ok})
	if err != nil || resp.ExitCode != ExitOK || !slices.Equal(s.models, []string{"opus", "fable", "sonnet"}) {
		t.Fatalf("exit %d err %v models %v; want opus, fable, then success on sonnet", resp.ExitCode, err, s.models)
	}
}

func TestLaunch_AmplifiedAuditorReturnsTheQuotaExitWhenAllThreeWallWithOpusFirst(t *testing.T) {
	useLiveClaudeDeepChain(t, "opus", []string{"fable", "opus", "sonnet"})
	s, resp, err, _ := launchAuditorAtDeep(t, map[string]core.BridgeResponse{"opus": wall, "fable": wall, "sonnet": wall})
	if err == nil || resp.ExitCode != ExitUnknownPrompt || !slices.Equal(s.models, []string{"opus", "fable", "sonnet"}) {
		t.Fatalf("exit %d err %v models %v; want exit 85 after each of the three models walls once", resp.ExitCode, err, s.models)
	}
}

func TestLaunch_AmplifiedAuditorTriesNoModelTwiceWhenTheFallbackListRepeatsThePrimary(t *testing.T) {
	useLiveClaudeDeepChain(t, "fable", []string{"fable", "fable", "opus", "opus"})
	s, _, _, _ := launchAuditorAtDeep(t, map[string]core.BridgeResponse{"fable": wall, "opus": wall, "sonnet": wall})
	seen := map[string]int{}
	for _, m := range s.models {
		seen[m]++
		if seen[m] > 1 {
			t.Fatalf("model %q launched twice in %v", m, s.models)
		}
	}
}

func TestLaunch_AmplifiedAuditorKeepsOnlyTheFableSuccessWhenFableLeads(t *testing.T) {
	useLiveClaudeDeepChain(t, "fable", []string{"fable", "opus", "sonnet"})
	s, resp, err, _ := launchAuditorAtDeep(t, map[string]core.BridgeResponse{"fable": ok})
	if err != nil || resp.ExitCode != ExitOK || !slices.Equal(s.models, []string{"fable"}) {
		t.Fatalf("exit %d err %v models %v; want a single launch on fable", resp.ExitCode, err, s.models)
	}
}

func TestIsFamilyModel_AmplifiedRejectsEmptyAndUnknownFamilies(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ family, model string }{{"claude", ""}, {"", "fable"}, {"nosuchfamily", "fable"}, {"claude", "gemini-3-pro"}} {
		if isFamilyModel(tc.family, tc.model) {
			t.Errorf("isFamilyModel(%q, %q) = true, want false", tc.family, tc.model)
		}
	}
}
