package policy_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestHealingLiveReaction_DefaultsToShadowAndReadsEachStage(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		`{}`:                               policy.HealingStageShadow,
		`{"healing":{"live_reaction":{}}}`: policy.HealingStageShadow,
		`{"healing":{"live_reaction":{"stage":"off"}}}`:     policy.HealingStageOff,
		`{"healing":{"live_reaction":{"stage":"enforce"}}}`: policy.HealingStageEnforce,
	}
	for body, want := range cases {
		p, err := policy.Parse([]byte(body))
		if err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		if got := p.HealingLiveReaction(); got != (policy.HealingLiveReactionPolicy{Stage: want}) {
			t.Errorf("%s: %+v, want stage %q", body, got, want)
		}
	}
}

func TestHealingLiveReactionPolicy_RefusesAStageOutsideTheRange(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		`{"healing":{"live_reaction":{"stage":"loud"}}}`:   `live_reaction.stage "loud"`,
		`{"healing":{"live_reaction":{"stgae":"shadow"}}}`: "healing.live_reaction.stgae",
	}
	for body, want := range cases {
		if _, err := policy.Parse([]byte(body)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want it to name %s", body, err, want)
		}
	}
}
