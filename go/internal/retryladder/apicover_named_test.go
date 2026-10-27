package retryladder_test

import (
	"errors"
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

func TestTryRecordVocabularyIsNamed(t *testing.T) {
	var tryContext retryladder.Context = retryladder.ContextNew
	var stage retryladder.Stage = retryladder.StageShadow
	if tryContext != "new" || stage != "shadow" {
		t.Fatalf("context %q stage %q, want new and shadow as spec section 6 spells them", tryContext, stage)
	}
}

func TestReportExportsAreNamed(t *testing.T) {
	var rows []retryladder.ReportRow = retryladder.BuildReport(nil)
	if len(rows) != 0 {
		t.Fatalf("no runs gave rows %+v", rows)
	}
	if _, err := retryladder.LoadRuns(t.TempDir()); err != nil {
		t.Fatalf("LoadRuns on an empty dir: %v", err)
	}
}

func TestFixSpecExportsAreNamed(t *testing.T) {
	var spec retryladder.FixSpec
	var err error
	spec, err = retryladder.ParseFixSpec("fix-spec: 1\nexpected: ok\nedit: a.go:1-2\n> x\n")
	if err != nil {
		t.Fatalf("ParseFixSpec: %v", err)
	}
	var edit retryladder.FixEdit = spec.Edits[0]
	if edit.File != "a.go" || edit.FromLine != 1 || edit.ToLine != 2 || len(edit.Replacement) != 1 {
		t.Fatalf("edit = %+v", edit)
	}
	_, err = retryladder.ParseFixSpec("")
	if !errors.Is(err, retryladder.ErrMalformedFixSpec) {
		t.Fatalf("empty spec error = %v", err)
	}
	var violations []retryladder.ScopeViolation
	violations, err = retryladder.FixScopeViolations(spec, "diff --git a/b.go b/b.go\n@@ -1 +1 @@\n")
	var code retryladder.ScopeCode = retryladder.ScopeFileOutside
	if err != nil || len(violations) != 1 || violations[0].Code != code || violations[0].Code == retryladder.ScopeLinesOutside {
		t.Fatalf("violations = %+v, %v", violations, err)
	}
	if _, err = retryladder.FixScopeViolations(spec, "@@ -1 +1 @@\n"); !errors.Is(err, retryladder.ErrMalformedFixDiff) {
		t.Fatalf("headless hunk error = %v", err)
	}
}
