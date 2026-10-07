package router

import (
	"maps"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func reviewCycleSignals(kind string, filesTouched int) RoutingSignals {
	return RoutingSignals{
		Scout:  ScoutSignals{Present: true, DeliverableKind: kind, ItemCount: 1},
		Triage: TriageSignals{Present: true, CycleSize: "small", DeliverableKind: kind},
		Build:  BuildSignals{Present: true, Verdict: "PASS", FilesTouched: filesTouched},
	}
}

func planWithoutCodeReview() *PhasePlan {
	var entries []PhasePlanEntry
	for _, phase := range []string{"scout", "triage", "tdd", "build", "adversarial-review", "audit", "ship"} {
		entries = append(entries, PhasePlanEntry{Phase: phase, Run: true, Justification: "the advisor never plans code-review"})
	}
	return &PhasePlan{Entries: entries}
}

func routeAfterBuild(cfg config.RoutingConfig, signals RoutingSignals) RouterDecision {
	return Route(RouteInput{
		Current:   "build",
		Verdict:   "PASS",
		Completed: []string{"scout", "triage", "tdd", "build"},
		Signals:   signals,
		Cfg:       cfg,
		Plan:      planWithoutCodeReview(),
	}, nil)
}

func TestCodeReviewPin_RunsNextOnEveryCodeCycleThatTouchedFilesEvenWhenTheAdvisorOmitsIt(t *testing.T) {
	cfg, _ := realRoutingConfig(t)
	dec := routeAfterBuild(cfg, reviewCycleSignals(DeliverableKindCode, 3))
	if dec.NextPhase != "code-review" {
		t.Fatalf("NextPhase = %q, want code-review: a code cycle whose build touched 3 files must be reviewed", dec.NextPhase)
	}
	if dec.Reason != "conditional-pin:code-review" {
		t.Errorf("Reason = %q, want conditional-pin:code-review (the registry pin, not the advisor's plan)", dec.Reason)
	}
	if slices.Contains(dec.InsertPhases, "code-review") {
		t.Errorf("InsertPhases = %v: a pinned phase is not an optional insertion and must not spend the insertion cap", dec.InsertPhases)
	}
}

func TestCodeReviewPin_SkipsADocumentCycleAndAZeroFileBuild(t *testing.T) {
	cfg, _ := realRoutingConfig(t)
	for name, signals := range map[string]RoutingSignals{
		"document cycle":   reviewCycleSignals(DeliverableKindDocument, 3),
		"zero-file build":  reviewCycleSignals(DeliverableKindCode, 0),
		"no build signals": {Triage: TriageSignals{Present: true, CycleSize: "small", DeliverableKind: DeliverableKindCode}},
	} {
		t.Run(name, func(t *testing.T) {
			if dec := routeAfterBuild(cfg, signals); dec.NextPhase == "code-review" {
				t.Errorf("NextPhase = code-review (reason %q): the pin must not fire here", dec.Reason)
			}
		})
	}
}

func TestCodeReviewPin_IsTheRegistryRuleNotAGoList(t *testing.T) {
	cfg, _ := realRoutingConfig(t)
	cfg.Conditional = maps.Clone(cfg.Conditional)
	delete(cfg.Conditional, "code-review")
	dec := routeAfterBuild(cfg, reviewCycleSignals(DeliverableKindCode, 3))
	if dec.NextPhase == "code-review" {
		t.Errorf("NextPhase = code-review with conditional_mandatory[code-review] removed: the router must hold no list of its own")
	}
}
