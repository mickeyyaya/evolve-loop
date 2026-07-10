//go:build acs

// Package cycle672 materialises the acceptance criteria for the single
// triage-committed top_n task of cycle 672, echo-veto-wiring-completion
// (weight 0.92). Cycle 654 landed the leaf helpers
// (Classifier.SetInjectedPrompt, stripPromptEchoLines) with green helper-level
// tests; cycle 656's wiring attempt was quota-killed. Grep of this tree
// confirms both helpers still have ZERO production call sites, so the live
// paths misfire exactly as the cycle-656 retro (D3) caught: a 100%-echoed
// pane classified rate_limit. This cycle wires consumption:
//
//   - AC1 (C672_001) Produce mechanism: ProduceConfig.InjectedPrompt threads
//     the phase prompt into the Classifier — echoed stderr line suppressed,
//     genuine 429 frame still emits.
//   - AC1 (C672_002) runner caller: BaseRunner's REAL default events producer
//     passes the composed prompt through to phasestream.Produce — asserted on
//     the emitted <phase>-events.ndjson via Run() end-to-end.
//   - AC2 (C672_003) auto-responder: tick() strips echoed prompt lines ahead
//     of the exhaustion/escalation scan (behavioral, rc-85 assertion) and both
//     production construction sites populate the responder's injected prompt.
//   - AC3 (C672_004) negative axis: a genuine quota banner / genuine runtime
//     infra signal still escalates/classifies — the veto is not a blanket
//     disable. Includes the cycle-654 regression arms (TestC654_002/003/004),
//     which must STAY green after the wiring.
//
// Predicate strategy: behavioural-via-subprocess (cycle-549…654 precedent) —
// every predicate shells `go test -run` over RED wiring tests that EXERCISE
// the SUT (Produce over an on-disk workspace; BaseRunner.Run; tick() over a
// scripted pane); none is source-grep-only. RED now: phasestream and bridge
// fail to compile (ProduceConfig.InjectedPrompt / autoResponder.injectedPrompt
// absent); the runner case fails behaviorally (echoed line emits
// infra_failure). GREEN once Builder lands the wiring. The
// Acceptance-Criteria-Summary line "go test -race PASS; apicover clean" is
// dispositioned manual+checklist in test-report.md (repo-wide toolchain gates
// the cycle audit already runs), not predicated here.
package cycle672

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	phasestreamPkg = "github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
	runnerPkg      = "github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	bridgePkg      = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	classifyPkg    = "github.com/mickeyyaya/evolve-loop/go/internal/cycleclassify"
)

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the
// test cache so the predicate always exercises current source. code<0 is a
// genuine launch failure (binary missing / killed by signal), never a test
// verdict — that fails loudly rather than being misread as a RED result.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC672_001_ProduceEchoVetoWired — AC1 mechanism: Produce threads the
// injected prompt into the Classifier; echo suppressed, genuine frame emits.
func TestC672_001_ProduceEchoVetoWired(t *testing.T) {
	ok, out := runGoTest(t, phasestreamPkg, "TestC672_001_ProduceInjectedPromptEchoVeto")
	if !ok {
		t.Errorf("Produce does not thread the injected prompt into the live Classifier (ProduceConfig.InjectedPrompt wiring missing):\n%s", out)
	}
}

// TestC672_002_RunnerThreadsPrompt — AC1 caller: the runner's default events
// producer carries the composed phase prompt through to Produce.
func TestC672_002_RunnerThreadsPrompt(t *testing.T) {
	ok, out := runGoTest(t, runnerPkg, "TestC672_002_RunnerThreadsComposedPromptToEventsProducer")
	if !ok {
		t.Errorf("BaseRunner's events producer drops the composed prompt — echoed prompt text still classifies infra_failure in <phase>-events.ndjson:\n%s", out)
	}
}

// TestC672_003_TickEchoVetoWired — AC2: tick() strips echoed prompt lines
// before the exhaustion scan, and both construction sites populate the
// responder's injected prompt.
func TestC672_003_TickEchoVetoWired(t *testing.T) {
	ok, out := runGoTest(t, bridgePkg,
		"TestC672_003_TickEchoedExhaustionDoesNotEscalate|TestC672_005_ResponderConstructionSitesCarryInjectedPrompt")
	if !ok {
		t.Errorf("autoResponder.tick still scans the RAW pane for exhaustion (stripPromptEchoLines unwired) or a construction site drops the prompt:\n%s", out)
	}
}

// TestC672_004_GenuineSignalsSurvive — AC3 negative axis: genuine exhaustion
// banners and genuine runtime infra signals still escalate/classify after the
// wiring; includes the cycle-654 regression arms which must stay green.
func TestC672_004_GenuineSignalsSurvive(t *testing.T) {
	if ok, out := runGoTest(t, bridgePkg,
		"TestC672_004_TickGenuineExhaustionStillEscalates|TestC654_004_EchoedExhaustionStrippedGenuineSurvives"); !ok {
		t.Errorf("genuine CLI exhaustion no longer escalates — the echo-veto wiring over-corrected (bridge):\n%s", out)
	}
	if ok, out := runGoTest(t, phasestreamPkg, "TestC654_003_PromptEchoNotEmittedGenuineEmitted"); !ok {
		t.Errorf("cycle-654 normalizer regression arm broke (phasestream):\n%s", out)
	}
	if ok, out := runGoTest(t, classifyPkg, "TestC654_002_GenuineInfraStillVetoes"); !ok {
		t.Errorf("genuine runtime infra no longer classifies infrastructure (cycleclassify regression):\n%s", out)
	}
}
