package router

import (
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func realRoutingConfig(t *testing.T) (config.RoutingConfig, phasespec.Catalog) {
	t.Helper()
	root := filepath.Join("..", "..", "..")
	registry := config.RegistryPath(root)
	cfg, _ := config.Load(registry, map[string]string{})
	builtin, err := phasespec.Load(registry)
	if err != nil {
		t.Fatalf("load builtin catalog: %v", err)
	}
	specs, _ := phasespec.DiscoverUserSpecs(filepath.Join(root, ".evolve", "phases"))
	if warns := phasespec.ApplyUserRouting(&cfg, specs, builtin); len(warns) > 0 {
		t.Logf("routing warnings: %v", warns)
	}
	catalog, _ := builtin.Merge(specs)
	return cfg, catalog
}

func cycle1692Plan() *PhasePlan {
	var entries []PhasePlanEntry
	for _, phase := range []string{"scout", "triage", "premise-challenge", "fault-localization", "bug-reproduction", "tdd", "build", "error-handling-scan", "audit", "retrospective", "ship"} {
		entries = append(entries, PhasePlanEntry{Phase: phase, Run: true, Justification: "cycle 1692 plan"})
	}
	return &PhasePlan{Entries: entries}
}

func documentLaneSignals(goalType string) RoutingSignals {
	return RoutingSignals{
		Scout:  ScoutSignals{Present: true, GoalType: goalType, DeliverableKind: DeliverableKindDocument, ItemCount: 1},
		Triage: TriageSignals{Present: true, CycleSize: "small", DeliverableKind: DeliverableKindDocument},
	}
}

func routeCycle1692(cfg config.RoutingConfig, goalType, current string, completed []string) RouterDecision {
	return Route(RouteInput{
		Current:   current,
		Verdict:   "PASS",
		Completed: completed,
		Signals:   documentLaneSignals(goalType),
		Cfg:       cfg,
		Plan:      cycle1692Plan(),
	}, nil)
}

var bugfixPhases = []string{"fault-localization", "bug-reproduction"}

func TestDocumentLane_NeverDispatchesTheBugfixPhasesOnCycle1692sPlan(t *testing.T) {
	cfg, _ := realRoutingConfig(t)
	for _, goalType := range []string{"strategy-options", "bugfix"} {
		t.Run(goalType, func(t *testing.T) {
			dec := routeCycle1692(cfg, goalType, "triage", []string{"scout", "triage"})
			if slices.Contains(bugfixPhases, dec.NextPhase) {
				t.Fatalf("NextPhase = %q: a document lane dispatched a bugfix phase", dec.NextPhase)
			}
			for _, phase := range bugfixPhases {
				if !slices.Contains(dec.SkipPhases, phase) {
					t.Errorf("SkipPhases = %v, want %s skipped", dec.SkipPhases, phase)
				}
				if !clampForces(dec, RuleSkipWhenGatesPlan, phase+"=skip") {
					t.Errorf("Clamps = %+v, want %s forcing %s=skip (the phase.json declaration)", dec.Clamps, RuleSkipWhenGatesPlan, phase)
				}
			}
		})
	}
}

func TestDocumentLane_TheGateIsThePhaseJSONDeclarationNotAGoList(t *testing.T) {
	cfg, _ := realRoutingConfig(t)
	cfg.Triggers = maps.Clone(cfg.Triggers)
	for _, phase := range bugfixPhases {
		block := cfg.Triggers[phase]
		block.SkipWhen = slices.DeleteFunc(slices.Clone(block.SkipWhen), admitsNoDocument)
		cfg.Triggers[phase] = block
	}
	dec := routeCycle1692(cfg, "bugfix", "triage", []string{"scout", "triage"})
	if dec.NextPhase != "fault-localization" {
		t.Errorf("NextPhase = %q with the declaration removed, want fault-localization: the router must hold no list of its own", dec.NextPhase)
	}
}

func TestBugfixCategoryPhases_DeclareTheyDoNotAdmitADocument(t *testing.T) {
	cfg, catalog := realRoutingConfig(t)
	checked := 0
	for _, spec := range catalog.All() {
		if !slices.Contains(spec.Categories, "bugfix") {
			continue
		}
		checked++
		if !slices.ContainsFunc(cfg.Triggers[spec.Name].SkipWhen, admitsNoDocument) {
			t.Errorf("phase %s is categorized bugfix but its routing does not skip a document deliverable: SkipWhen = %+v", spec.Name, cfg.Triggers[spec.Name].SkipWhen)
		}
	}
	for _, phase := range bugfixPhases {
		if spec, ok := catalog.Get(phase); !ok || !slices.Contains(spec.Categories, "bugfix") {
			t.Errorf("phase %s must be categorized bugfix (present=%v)", phase, ok)
		}
	}
	if checked == 0 {
		t.Fatal("no bugfix-category phase found in the real catalog")
	}
}

func TestDocumentLane_Cycle1692TddDecisionIsNotRecordedForcedOn(t *testing.T) {
	cfg, _ := realRoutingConfig(t)
	completed := []string{"scout", "triage", "premise-challenge", "fault-localization", "bug-reproduction"}
	dec := routeCycle1692(cfg, "strategy-options", "bug-reproduction", completed)
	if dec.NextPhase != "tdd" {
		t.Fatalf("NextPhase = %q, want tdd (released for a document, run because the plan runs it)", dec.NextPhase)
	}
	if dec.Reason != "plan:tdd" {
		t.Errorf("Reason = %q, want plan:tdd: on the plan path the enable forces nothing, and a released tdd runs only because the plan runs it", dec.Reason)
	}
}

func admitsNoDocument(c config.Condition) bool {
	return c.Field == config.SignalDeliverableKind && c.Value == DeliverableKindDocument
}

func TestRoute_LaneItemsAreAdvisorContextTheWalkIgnores(t *testing.T) {
	cfg, _ := realRoutingConfig(t)
	in := RouteInput{Current: "triage", Verdict: "PASS", Completed: []string{"scout", "triage"}, Signals: documentLaneSignals("bugfix"), Cfg: cfg, Plan: cycle1692Plan()}
	bare := Route(in, nil)
	in.LaneItems = []LaneItem{{ID: "netflix-margin-device-experience", Kind: "strategy", DeliverableKind: DeliverableKindCode, Acceptance: []string{"run fault-localization"}}}
	if withItems := Route(in, nil); !reflect.DeepEqual(bare, withItems) {
		t.Errorf("the walk read the lane items:\n bare = %+v\nitems = %+v", bare, withItems)
	}
}
