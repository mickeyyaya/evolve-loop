package router

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func psmasBase(cur string, size string, skip []string) RouteInput {
	in := base(cur)
	in.PSMASEnabled = true
	in.Signals.Triage = TriageSignals{CycleSize: size, PhaseSkip: skip, Present: true}
	return in
}

func TestPSMAS_SkipsTriageRecommendedOptionalPhase(t *testing.T) {
	in := psmasBase("build", "small", []string{"tester"})
	in.Completed = []string{"scout", "tdd", "build"}
	in.Signals.Build = BuildSignals{ACSRed: 2, Present: true}

	d := Route(in, nil)
	if d.NextPhase != "audit" {
		t.Errorf("build(red>0, psmas skip tester) → %q, want audit", d.NextPhase)
	}
	if !containsPhase(d.SkipPhases, "tester") {
		t.Errorf("SkipPhases = %v, want tester recorded (skip must be auditable, not silent)", d.SkipPhases)
	}
}

func TestPSMAS_GateOffIgnoresPhaseSkip(t *testing.T) {
	in := psmasBase("build", "small", []string{"tester"})
	in.PSMASEnabled = false
	in.Completed = []string{"scout", "tdd", "build"}
	in.Signals.Build = BuildSignals{ACSRed: 2, Present: true}

	d := Route(in, nil)
	if d.NextPhase != "tester" {
		t.Errorf("gate off → %q, want tester (PhaseSkip must be ignored)", d.NextPhase)
	}
}

func TestPSMAS_GateOnWithoutTriagePresenceDoesNotSkip(t *testing.T) {
	in := psmasBase("build", "small", []string{"tester"})
	in.Signals.Triage.Present = false
	in.Completed = []string{"scout", "tdd", "build"}
	in.Signals.Build = BuildSignals{ACSRed: 2, Present: true}

	d := Route(in, nil)
	if d.NextPhase != "tester" {
		t.Errorf("gate on but no triage presence -> %q, want tester (stale PhaseSkip ignored)", d.NextPhase)
	}
	if containsPhase(d.SkipPhases, "tester") {
		t.Errorf("SkipPhases = %v, want no tester skip recorded without triage presence", d.SkipPhases)
	}
}

func TestPSMAS_IgnoresUnknownPhaseNamesButAppliesKnownOnes(t *testing.T) {
	in := psmasBase("build", "small", []string{"not-a-phase", "tester"})
	in.Completed = []string{"scout", "tdd", "build"}
	in.Signals.Build = BuildSignals{ACSRed: 2, Present: true}

	d := Route(in, nil)
	if d.NextPhase != "audit" {
		t.Errorf("unknown + tester skip -> %q, want audit (known optional skip still applies)", d.NextPhase)
	}
	if !containsPhase(d.SkipPhases, "tester") {
		t.Errorf("SkipPhases = %v, want tester recorded", d.SkipPhases)
	}
	if containsPhase(d.SkipPhases, "not-a-phase") {
		t.Errorf("SkipPhases = %v, want unknown phase name ignored", d.SkipPhases)
	}
}

func TestPSMAS_CannotSkipMandatoryPhase(t *testing.T) {
	in := psmasBase("scout", "trivial", []string{"build", "audit", "ship"})
	in.Completed = []string{"scout"}

	d := Route(in, nil)
	if d.NextPhase != "build" {
		t.Errorf("psmas skip [build,audit,ship] → %q, want build (mandatory-never-skipped)", d.NextPhase)
	}
}

func TestPSMAS_CannotUnpinTDDOnNonTrivial(t *testing.T) {
	in := psmasBase("scout", "medium", []string{"tdd-engineer"})
	in.Completed = []string{"scout"}

	d := Route(in, nil)
	if d.NextPhase != "tdd" {
		t.Errorf("psmas skip tdd-engineer on medium → %q, want tdd (conditional pin wins)", d.NextPhase)
	}
}

func TestPSMAS_TriageVocabularyNormalized(t *testing.T) {
	in := psmasBase("scout", "trivial", []string{"tdd-engineer", "retrospective"})
	in.Completed = []string{"scout"}
	// EnableOn makes legacy routing run tdd even on a trivial cycle, so only the PSMAS skip can drop it.
	in.Cfg.PhaseEnable["tdd"] = config.EnableOn

	d := Route(in, nil)
	if d.NextPhase != "build" {
		t.Errorf("trivial + psmas skip tdd-engineer → %q, want build", d.NextPhase)
	}
	if !containsPhase(d.SkipPhases, "tdd") {
		t.Errorf("SkipPhases = %v, want canonical \"tdd\" recorded from triage's \"tdd-engineer\"", d.SkipPhases)
	}
}

func containsPhase(list []string, want string) bool {
	for _, p := range list {
		if p == want {
			return true
		}
	}
	return false
}
