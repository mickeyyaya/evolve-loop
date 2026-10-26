package retryladder_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

func TestClassify_ContractRejectionIsCapability(t *testing.T) {
	if got := retryladder.Classify(retryladder.Facts{ContractRejected: true}).Class; got != retryladder.Capability {
		t.Fatalf("a rejected deliverable = %q, want %q", got, retryladder.Capability)
	}
	if got := retryladder.Classify(retryladder.Facts{ContractRejected: true, ProvenBlocker: true}).Class; got != retryladder.Spec {
		t.Fatalf("a proven blocker with a rejection = %q, want %q", got, retryladder.Spec)
	}
}

func TestClimbs_OnlyDeliverableSeats(t *testing.T) {
	for _, phase := range []string{"build", "tdd", "scout", "triage", "fault-localization", "bug-reproduction"} {
		if !retryladder.Climbs(phase) {
			t.Errorf("Climbs(%q) = false, want true: the seat produces a deliverable", phase)
		}
	}
	for _, phase := range []string{"audit", "adversarial-review", "code-review", "spec-verify", "coverage-gate", "retro", ""} {
		if retryladder.Climbs(phase) {
			t.Errorf("Climbs(%q) = true, want false: a judge or control seat never climbs", phase)
		}
	}
}

func TestTryMark_NilKeepsTheBaseRecord(t *testing.T) {
	var try *retryladder.Try
	var rec llmcalls.Record
	try.Mark(&rec)
	if rec.Rung != "R0" || rec.Context != "new" || rec.LadderStage != "shadow" || rec.LadderWould != "" || rec.FailureClass != "" {
		t.Fatalf("nil try marked %+v, want R0, new, shadow and no decision", rec)
	}
}

func TestTryMark_CarriesTheDecision(t *testing.T) {
	try := &retryladder.Try{
		Rung: retryladder.R2, FailureClass: retryladder.Capability, Fingerprint: "codes:floor",
		Context: retryladder.ContextFresh, Stage: retryladder.StageEnforce, Would: retryladder.R2, Tier: "deep",
	}
	var rec llmcalls.Record
	try.Mark(&rec)
	if rec.Rung != "R2" || rec.Context != "fresh" || rec.LadderStage != "enforce" || rec.LadderWould != "R2" ||
		rec.FailureClass != "capability" || rec.Fingerprint != "codes:floor" {
		t.Fatalf("marked %+v, want the decision of the try", rec)
	}
	if try.LadderTier() != "deep" || (*retryladder.Try)(nil).LadderTier() != "" {
		t.Fatalf("LadderTier = %q, want deep, and empty for no try", try.LadderTier())
	}
}

func TestTryMark_ShadowKeepsTheRungAtR0(t *testing.T) {
	try := &retryladder.Try{FailureClass: retryladder.Capability, Fingerprint: "codes:x", Would: retryladder.R1}
	var rec llmcalls.Record
	try.Mark(&rec)
	if rec.Rung != "R0" || rec.Context != "new" || rec.LadderStage != "shadow" || rec.LadderWould != "R1" {
		t.Fatalf("shadow try marked %+v, want R0, new, shadow, would R1", rec)
	}
}

func TestDigestRender_CarriesEachPartVerbatim(t *testing.T) {
	d := retryladder.Digest{
		Rung:         retryladder.R1,
		Class:        retryladder.Capability,
		Failures:     []string{"[cover_changed] 11 of 26 functions below 100%: core.askLadder 80.0%"},
		Findings:     "H1: the decider is not asked on the resume path",
		ChangedFiles: []string{" M go/internal/core/retry_ladder.go"},
		History:      []string{"R0 capability codes:cover_changed"},
	}
	out := d.Render(500)
	for _, want := range []string{
		"[cover_changed] 11 of 26 functions below 100%: core.askLadder 80.0%",
		"H1: the decider is not asked on the resume path",
		"go/internal/core/retry_ladder.go",
		"R0 capability codes:cover_changed",
		"R1", "capability", "new session",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("digest lacks %q:\n%s", want, out)
		}
	}
	if strings.Index(out, d.Failures[0]) > strings.Index(out, d.Findings) {
		t.Errorf("the failed check must come before the findings:\n%s", out)
	}
}

func TestDigestRender_OmitsEmptyParts(t *testing.T) {
	out := retryladder.Digest{Rung: retryladder.R1, Class: retryladder.Capability, Failures: []string{"floor red"}}.Render(500)
	for _, absent := range []string{"Audit findings", "Changed files", "Tries before"} {
		if strings.Contains(out, absent) {
			t.Errorf("digest has an empty %q part:\n%s", absent, out)
		}
	}
}

func TestDigestRender_StaysInsideTheBudget(t *testing.T) {
	for _, lead := range []string{"first check failed: ", "first check failed:  "} {
		d := retryladder.Digest{
			Rung: retryladder.R2, Class: retryladder.Capability,
			Failures: []string{lead + strings.Repeat("é", 5000)},
			Findings: strings.Repeat("finding ", 4000),
		}
		const budget = 100
		out := d.Render(budget)
		if len(out) > budget*retryladder.DigestBytesPerToken {
			t.Fatalf("digest is %d bytes, over the budget of %d", len(out), budget*retryladder.DigestBytesPerToken)
		}
		if !utf8.ValidString(out) {
			t.Fatalf("the cut split a rune after lead %q", lead)
		}
		if !strings.Contains(out, "first check failed") || !strings.Contains(out, "digest cut") {
			t.Fatalf("a cut digest keeps the first failed check and says it was cut:\n%s", out)
		}
	}
}

func TestDigestRender_NoFailedCheckWritesNoFailurePart(t *testing.T) {
	out := retryladder.Digest{Rung: retryladder.R1, Class: retryladder.Unknown, Findings: "H1: a finding"}.Render(500)
	if strings.Contains(out, "Failed checks") || !strings.Contains(out, "H1: a finding") {
		t.Fatalf("digest without a failed check:\n%s", out)
	}
}

func TestDigestRender_ZeroBudgetUsesTheDefault(t *testing.T) {
	d := retryladder.Digest{Rung: retryladder.R1, Class: retryladder.Capability, Failures: []string{strings.Repeat("x", 100000)}}
	if got, max := len(d.Render(0)), retryladder.DefaultDigestTokens*retryladder.DigestBytesPerToken; got > max || got < max/2 {
		t.Fatalf("zero budget gave %d bytes, want the default cap of %d", got, max)
	}
}

func TestDigestRender_TheMinimumBudgetKeepsTheFirstCheckInsideTheCap(t *testing.T) {
	d := retryladder.Digest{Rung: retryladder.R1, Class: retryladder.Capability, Failures: []string{"[x] " + strings.Repeat("y", 4000)}}
	out := d.Render(policy.MinRetryLadderDigestTokens)
	if len(out) > policy.MinRetryLadderDigestTokens*retryladder.DigestBytesPerToken || !strings.Contains(out, "[x] y") {
		t.Fatalf("minimum-budget digest is %d bytes:\n%s", len(out), out)
	}
}

func TestLadderVocabularyIsNamed(t *testing.T) {
	if retryladder.SignalDecision != "LADDER_DECISION" || retryladder.ContextFresh != "fresh" || retryladder.StageEnforce != "enforce" {
		t.Fatalf("vocabulary drifted from spec sections 6 and 8")
	}
}
