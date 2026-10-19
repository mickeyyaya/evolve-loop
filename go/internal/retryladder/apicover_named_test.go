package retryladder_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

func TestExportsAreNamed(t *testing.T) {
	facts := retryladder.Facts{
		ExitCode:           85,
		QuotaBench:         true,
		TeardownError:      false,
		AuditClass:         "code-build-fail",
		ProvenBlocker:      false,
		LaneScope:          []string{"go/internal/foo"},
		Floors:             []retryladder.FloorFailure{{Path: "go/internal/foo/a.go", Protected: false}},
		DeliverableMissing: false,
		Reason:             "[missing_section] heading absent",
	}
	var verdict retryladder.Verdict = retryladder.Classify(facts)
	var class retryladder.Class = verdict.Class
	if class != retryladder.Quota {
		t.Fatalf("85 with a bench = %q, want %q", class, retryladder.Quota)
	}
	if verdict.Fingerprint != retryladder.Fingerprint(facts.Reason) {
		t.Fatalf("verdict fingerprint = %q, want %q", verdict.Fingerprint, retryladder.Fingerprint(facts.Reason))
	}
	want := map[retryladder.Class]string{
		retryladder.Infra:      "infra",
		retryladder.Quota:      "quota",
		retryladder.Halt:       "halt",
		retryladder.Spec:       "spec",
		retryladder.Capability: "capability",
		retryladder.Unknown:    "unknown",
	}
	for c, name := range want {
		if string(c) != name {
			t.Fatalf("class %q, want %q", c, name)
		}
	}
}

func TestDeciderExportsAreNamed(t *testing.T) {
	in := retryladder.Input{
		Rung:     retryladder.R0,
		Earlier:  []string{"codes:a"},
		Verdict:  retryladder.Verdict{Class: retryladder.Capability, Fingerprint: "codes:a"},
		Base:     convergence.TierEffort{Family: "claude", Tier: "balanced", Model: "sonnet", Effort: "medium"},
		Chain:    []string{"claude"},
		Headroom: convergence.HeadroomTable{"claude": {"deep": {Model: "opus", Effort: "high"}}},
		Bounds:   retryladder.Bounds{MaxTier: retryladder.DefaultMaxTier, MaxEffort: retryladder.DefaultMaxEffort},
		Critic:   retryladder.R1,
	}
	var got retryladder.Decision = retryladder.Next(in)
	var action retryladder.Action = got.Action
	var rung retryladder.Rung = got.Rung
	if action != retryladder.ActionRaise || rung != retryladder.R2 || got.CriticRung != retryladder.R1 {
		t.Fatalf("Next = %+v, want a raise to R2 recording critic R1", got)
	}
	stopped := in
	stopped.Rung = retryladder.R2
	if d := retryladder.Next(stopped); d.Action != retryladder.ActionStop || d.Class != retryladder.Spec || d.Signals[0] != retryladder.SignalStopSpec {
		t.Fatalf("R2 on a repeated fingerprint = %+v", d)
	}
	stopped.Verdict.Fingerprint, stopped.Headroom = "codes:z", convergence.HeadroomTable{}
	stopped.Rung = retryladder.R1
	if d := retryladder.Next(stopped); d.Signals[0] != retryladder.SignalNoHeadroom {
		t.Fatalf("R1 without headroom = %+v", d)
	}
	if retryladder.R0 >= retryladder.R1 || retryladder.ActionRetry == retryladder.ActionStop {
		t.Fatalf("rung and action constants collide")
	}
	if r := (retryladder.Bounds{}).Resolved(); r.MaxTier != "deep" || r.MaxEffort != "high" {
		t.Fatalf("Resolved = %+v", r)
	}
}
