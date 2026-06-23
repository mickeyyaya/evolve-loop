// shadow_observer_test.go — R8.2: shadow-stage promoted rules OBSERVE the
// pane and record a would-fire signal; they never send keys and never alter
// the tick's control flow. One signal per rule per launch (the dedup that
// keeps a lingering pane from inflating the soak evidence).
package bridge

import (
	"context"
	"regexp"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/interaction"
)

func TestShadowRule_RecordsWouldFireOnceSendsNothing(t *testing.T) {
	t.Parallel()
	// Three captures, all matching the shadow rule — the signal must be ONE.
	pane := "Update available! choose: 1. Update now 2. Skip"
	ar, rec := autoRespondHarness(t, []string{pane, pane, pane}, nil)
	ar.shadowRules = []shadowObserver{{id: "rule-abc123", re: regexp.MustCompile(`Update available!.*Skip`)}}

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, rc := ar.tick(ctx, "s"); rc != 0 {
			t.Fatalf("tick %d: shadow observation must not change control flow (rc=%d)", i, rc)
		}
	}
	if got := len(ar.deps.Tmux.(*fakeTmux).sentKeys); got != 0 {
		t.Fatalf("shadow rule sent %d key sequences — must send NOTHING", got)
	}
	outs := rec.Outcomes()
	if len(outs) != 1 {
		t.Fatalf("outcomes = %d, want exactly 1 would-fire (per-launch dedup)", len(outs))
	}
	o := outs[0]
	if o.Kind != "rule_shadow_fire" || o.RuleID != "rule-abc123" || o.Result != "would_fire" {
		t.Errorf("signal shape wrong: %+v", o)
	}
}

func TestShadowRule_NoMatchNoSignal(t *testing.T) {
	t.Parallel()
	ar, rec := autoRespondHarness(t, []string{"● working on the task…"}, nil)
	ar.shadowRules = []shadowObserver{{id: "rule-abc123", re: regexp.MustCompile(`Update available!.*Skip`)}}
	if _, rc := ar.tick(context.Background(), "s"); rc != 0 {
		t.Fatalf("rc=%d", rc)
	}
	if outs := rec.Outcomes(); len(outs) != 0 {
		t.Fatalf("no match must record nothing: %+v", outs)
	}
}

func TestLoadShadowObservers_SplitsRegistryByStage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := interactionRulesDir(root)
	id, err := interaction.PromoteRule(dir, "Do you want to overwrite the existing\\?", "n,Enter", "t", healthyCorpus)
	if err != nil {
		t.Fatalf("PromoteRule: %v", err)
	}
	if got := loadShadowObservers(root); len(got) != 1 || got[0].id != id {
		t.Fatalf("shadow set = %+v, want the freshly-promoted rule", got)
	}
	if got := loadPromotedPrompts(root); len(got) != 0 {
		t.Fatalf("enforce set must be empty pre-flip: %+v", got)
	}
	if err := EnforceMeasuredRule(root, id); err != nil {
		t.Fatalf("EnforceMeasuredRule: %v", err)
	}
	if got := loadShadowObservers(root); len(got) != 0 {
		t.Fatalf("flipped rule must leave the shadow set: %+v", got)
	}
	if got := loadPromotedPrompts(root); len(got) != 1 {
		t.Fatalf("flipped rule must enter the enforce set: %+v", got)
	}
}
