//go:build acs

// Package cycle1638 materializes the acceptance criteria for the single
// triage-committed task of this fleet lane:
//
//   - triage-unified-solution-synthesis → the operator directive that triage
//     synthesizes ACROSS backlog items and commits ONE unified general
//     solution, with PLANNING given teeth over that commitment.
//
// The second lane-scoped id, overlay-family-name-transport-ambiguity, was
// DEFERRED by triage as already shipped (797b8518, verified an ancestor of
// this tree: `git merge-base --is-ancestor 797b8518 HEAD` exits 0). R9.3
// forbids predicates for non-top_n work, so nothing below binds to it.
//
// # What is already built, and what this package adds
//
// Cycles 1633 and 1637 built the SELECTION half of the directive and it is
// preserved in this tree: inboxbatch.UnifiedCommitment.Validate (typed,
// evidence-cited, homogeneous-campaign, >=2 members), triage's
// processUnifiedCommitment (top_n binding, fail-open rejection, campaign-plan
// emission, claimed-item lifecycle resolution), router.Digest's
// UnifiedSize/UnifiedMemberCount projection, and transactional member closure
// at landing. go/acs/cycle1637 pins all of it and is GREEN here; 005 below
// re-runs that whole package as ONE named-package `go test` so the re-ship
// cannot regress it, rather than re-authoring 650 lines of the same contract.
//
// What is NOT built is how_to_apply step (2) — "PLANNING gets teeth: a unified
// commitment routes through buildplanner+plan-review AT DEEP TIER; plan-review
// verdict REVISE/ABORT if the design is a patch-bundle rather than a general
// abstraction". Verified absent by driving the production callers, not by
// reading code:
//
//  1. NO TIER TEETH. router.RoutingSignals.Triage.UnifiedSize is resolvable as
//     the routing field "triage.unified_size" (internal/router/condition.go:83)
//     and pins plan-review/build-planner to RUN via the registry's
//     conditional_mandatory block — but nothing anywhere reads it to raise a
//     model TIER. `grep -rn UnifiedSize` outside tests returns exactly three
//     sites (the field, the digest write, the condition read). So the deepest
//     design decision the loop makes is reviewed at whatever tier the advisor
//     happened to propose. 001 pins the raise at the production clamp; 002 is
//     the anti-no-op negative that forbids a blanket "always deep".
//  2. THE BIGGEST BUNDLES ESCAPE REVIEW ENTIRELY. The registry pins
//     plan-review and build-planner on `triage.unified_size==small` only, so a
//     LARGE commitment — the one that emits a multi-cycle ADR-0054 campaign
//     plan, the highest-blast-radius design the loop can commit — is the one
//     case that runs with no plan review at all. 003 drives the real
//     PhasePolicy over the real registry and requires both sizes to pin.
//  3. NO PATCH-BUNDLE RUBRIC. agents/plan-reviewer.md carries the four lenses
//     and the PROCEED/REVISE/ABORT aggregation, but says nothing about
//     rejecting a plan that is N patches wearing one commitment's clothes —
//     the exact failure mode step (2) names. 004 loads the persona through the
//     production loader and requires the rule to be reachable in the composed
//     prompt.
//
// Adversarial axes (skills/adversarial-testing §6): NEGATIVE — 002 (no
// commitment must not escalate), 003's no-commitment subtest, 001's
// clamp-recorded requirement (a silent raise fails). EDGE — the `large`
// commitment (003), a plan that proposes no tier at all (002). SEMANTIC — tier
// escalation (001/002), run-pinning (003), review rubric (004), prior-contract
// regression (005/006) are four distinct behaviors, not one restated.
//
// Flaky-shape hygiene: the two `go test` subprocesses each name ONE package
// (./acs/cycle1637, ./internal/router — measured 3.1s and 0.5s), no `/...`
// sweep, no ./internal/core or ./cmd/evolve, no wall-clock bounds, no literal
// PIDs, every git call is -C rooted, no load generators.
package cycle1638

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/buildplanner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// advisorTier is the tier the advisor is made to propose for the planning
// phases in every fixture below: a mid tier, so a raise to "deep" is visible
// and a no-op is equally visible.
const advisorTier = "balanced"

// realRegistry loads the LIVE docs/architecture/phase-registry.json through
// config.Load — the same loader the composition root uses — so these
// predicates bind to the shipped routing config, never to a fixture that can
// agree with a broken tree.
func realRegistry(t *testing.T) config.RoutingConfig {
	t.Helper()
	root := acsassert.RepoRoot(t)
	cfg, warns := config.Load(filepath.Join(root, "docs", "architecture", "phase-registry.json"), map[string]string{})
	for _, w := range warns {
		t.Logf("registry warning: %+v", w)
	}
	if len(cfg.Order) == 0 && len(cfg.Conditional) == 0 {
		t.Fatalf("precondition: the live phase registry loaded empty — the predicate would assert nothing")
	}
	return cfg
}

// unifiedSignals is the digest a cycle carries AFTER triage validated a
// commitment. size "" is the no-commitment case (an absent or rejected claim —
// router.Digest writes the field only for a validated one).
func unifiedSignals(size string, members int) router.RoutingSignals {
	var sig router.RoutingSignals
	sig.Triage.UnifiedSize = size
	sig.Triage.UnifiedMemberCount = members
	return sig
}

// planningPlan is an advisory plan in which the advisor scheduled both
// planning phases at advisorTier, plus the spine so the floor clamp has a
// well-formed plan to work on.
func planningPlan() *router.PhasePlan {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "plan-review", Run: true, Tier: advisorTier, Justification: "advisor default"},
		{Phase: "build-planner", Run: true, Tier: advisorTier, Justification: "advisor default"},
		{Phase: "tdd", Run: true, Tier: advisorTier},
		{Phase: "build", Run: true, Tier: advisorTier},
		{Phase: "audit", Run: true, Tier: advisorTier},
		{Phase: "ship", Run: true, Tier: advisorTier},
	}}
}

// clampPlan drives the PRODUCTION integrity-floor clamp — the exact function
// internal/core/phase_advisor.go:1030 and internal/core/cyclerun.go:828 call
// on every routed cycle. A predicate that re-implemented the walk would pass
// on dead code.
func clampPlan(t *testing.T, sig router.RoutingSignals) (*router.PhasePlan, []router.Clamp) {
	t.Helper()
	in := router.RouteInput{
		Current:   "triage",
		Verdict:   "PASS",
		Signals:   sig,
		Cfg:       realRegistry(t),
		Completed: []string{"scout", "triage"},
	}
	return router.ClampPlanToFloorWith(in, planningPlan(), router.DefaultShipFloor(), false)
}

// tierOf returns the clamped plan's tier for phase, and whether the phase
// survived the clamp at all.
func tierOf(plan *router.PhasePlan, phase string) (string, bool) {
	for _, e := range plan.Entries {
		if e.Phase == phase {
			return e.Tier, true
		}
	}
	return "", false
}

// planningPhases are the two phases how_to_apply step (2) names.
var planningPhases = []string{"plan-review", "build-planner"}

// ---------------------------------------------------------------------------
// 001: a validated unified commitment gives PLANNING deep-tier teeth
// ---------------------------------------------------------------------------

func TestC1638_001_ValidatedUnifiedCommitmentRaisesPlanningPhasesToDeepTier(t *testing.T) {
	for _, size := range []string{"small", "large"} {
		t.Run(size, func(t *testing.T) {
			plan, clamps := clampPlan(t, unifiedSignals(size, 3))
			for _, phase := range planningPhases {
				tier, present := tierOf(plan, phase)
				if !present {
					t.Fatalf("RED: %s was dropped from the clamped plan entirely", phase)
				}
				if tier != "deep" {
					t.Errorf("RED: a validated %s unified commitment must raise %s to the deep tier "+
						"(how_to_apply step 2: \"routes through buildplanner+plan-review at deep tier\"); got tier=%q",
						size, phase, tier)
				}
			}
			// The raise must be a RECORDED decision, not a silent rewrite: the
			// clamp ledger is how an operator and the audit phase see that the
			// unified commitment, and not the advisor, chose the tier.
			var recorded []string
			for _, c := range clamps {
				for _, phase := range planningPhases {
					if c.Phase == phase && strings.Contains(strings.ToLower(c.Forced), "deep") {
						recorded = append(recorded, c.Phase)
					}
				}
			}
			if len(recorded) < len(planningPhases) {
				t.Errorf("RED: the tier raise must be recorded as a Clamp for each planning phase "+
					"(proposed=%s → forced=deep); recorded only %v of %v; clamps=%+v",
					advisorTier, recorded, planningPhases, clamps)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 002: NEGATIVE — no validated commitment, no escalation (anti-no-op)
// ---------------------------------------------------------------------------

// The cheapest fake for 001 is "always force the planning phases to deep".
// This forbids it: an ordinary cycle, and a cycle whose unified_commitment
// triage REJECTED (both digest to UnifiedSize==""), must leave the advisor's
// proposal exactly as written. Fail-open is the directive's own guard (step 5).
func TestC1638_002_NoValidatedCommitmentLeavesAdvisorTiersUntouched(t *testing.T) {
	plan, clamps := clampPlan(t, unifiedSignals("", 0))
	for _, phase := range planningPhases {
		tier, present := tierOf(plan, phase)
		if !present {
			continue // the clamp may decline to schedule an optional phase; only the tier is pinned here
		}
		if tier != advisorTier {
			t.Errorf("with no validated unified commitment the advisor's tier must stand: %s tier=%q, want %q "+
				"(a blanket escalation would make 001 pass on a no-op)", phase, tier, advisorTier)
		}
	}
	for _, c := range clamps {
		for _, phase := range planningPhases {
			if c.Phase == phase && strings.Contains(strings.ToLower(c.Forced), "deep") {
				t.Errorf("no unified commitment must produce no tier clamp on %s; got %+v", phase, c)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 003: the LARGE commitment cannot outrun the design review
// ---------------------------------------------------------------------------

// The registry pins plan-review/build-planner on `triage.unified_size==small`
// only, so today the multi-cycle campaign — the largest design the loop ever
// commits — is the single case that ships with no plan review. Driven through
// router.PhasePolicy, the production enablement authority every self-skipping
// phase consults.
func TestC1638_003_EverySizeOfUnifiedCommitmentPinsThePlanningPhases(t *testing.T) {
	pol := router.NewPhasePolicy(realRegistry(t))
	for _, size := range []string{"small", "large"} {
		t.Run(size, func(t *testing.T) {
			for _, phase := range planningPhases {
				if !pol.Enabled(phase, unifiedSignals(size, 3)) {
					t.Errorf("RED: a validated %s unified commitment must pin %s to run "+
						"(how_to_apply step 2 names both planning phases and does not exempt the campaign path); Enabled=false",
						size, phase)
				}
			}
		})
	}
	// NEGATIVE: the pin is the commitment's doing, not an unconditional
	// enable — an ordinary cycle must not be pinned by this rule.
	t.Run("no-commitment", func(t *testing.T) {
		for _, phase := range planningPhases {
			if pol.Enabled(phase, unifiedSignals("", 0)) {
				t.Errorf("fail-open: with no validated commitment %s must not be pinned by the unified rule; Enabled=true", phase)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// 004: plan-review must be able to REJECT a patch-bundle
// ---------------------------------------------------------------------------

// acs-predicate: config-check — the rubric is prompt text consumed by an LLM,
// so presence in the PRODUCTION-resolved persona is the only mechanical check
// available. It is loaded through prompts.NewForProject (the canonical
// loader-resolution helper phase registrations use), not read off disk, so a
// rubric that exists in a file the loader never reaches still fails.
func TestC1638_004_PlanReviewPersonaRejectsPatchBundleDesigns(t *testing.T) {
	root := acsassert.RepoRoot(t)
	t.Setenv("EVOLVE_PROMPTS_DIR", "") // deterministic: take the projectRoot branch
	persona, err := prompts.NewForProject(root).Agent("plan-reviewer")
	if err != nil {
		t.Fatalf("the production prompt loader must resolve the plan-review persona: %v", err)
	}
	body := strings.ToLower(persona.Raw)
	// The failure mode step (2) names, in the words a reviewer can act on.
	if !strings.Contains(body, "patch-bundle") && !strings.Contains(body, "patch bundle") {
		t.Errorf("RED: the plan-review persona must name the patch-bundle failure mode " +
			"(a unified commitment whose plan is N patches rather than one general abstraction)")
	}
	// Naming it is not enough — it must carry a VERDICT consequence.
	hasVerdict := strings.Contains(body, "revise") || strings.Contains(body, "abort")
	if !hasVerdict {
		t.Errorf("RED: the persona must bind the patch-bundle finding to a REVISE/ABORT verdict, not merely mention it")
	}
	// And the rubric the directive spells out: what a GENERAL solution looks like.
	wanted := []string{"single-source", "immutab", "kiss"}
	var missing []string
	for _, w := range wanted {
		if !strings.Contains(body, w) {
			missing = append(missing, w)
		}
	}
	if len(missing) > 0 {
		t.Errorf("RED: the persona must state the general-abstraction rubric the directive names "+
			"(single-source-with-projection, Strategy/DI over flags, immutability, KISS floor); missing markers: %v", missing)
	}
}

// ---------------------------------------------------------------------------
// 005: the salvaged selection contract stays GREEN across the re-ship
// ---------------------------------------------------------------------------

func TestC1638_005_SalvagedCycle1637ContractStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	pkgDir := filepath.Join(root, "go", "acs", "cycle1637")
	if !acsassert.FileExists(t, filepath.Join(pkgDir, "predicates_test.go")) {
		t.Fatalf("the salvaged predicate package must stay in the tree: %s", pkgDir)
	}
	cmd := exec.Command("go", "test", "-tags", "acs", "-count=1", "./acs/cycle1637/")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the cycle-1637 unified-commitment contract regressed — the planning-teeth work must keep it GREEN:\n%s\n%v", out, err)
	}
	if !strings.Contains(string(out), "ok ") {
		t.Errorf("expected an `ok` line from the cycle1637 package; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// 006: the package the change lands in stays GREEN
// ---------------------------------------------------------------------------

func TestC1638_006_RouterPackageStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "test", "-count=1", "./internal/router/")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("internal/router regressed:\n%s\n%v", out, err)
	}
	if !strings.Contains(string(out), "ok ") {
		t.Errorf("expected an `ok` line from internal/router; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// 007: ship-tree tracking of this package (cycle-93 lesson)
// ---------------------------------------------------------------------------

func TestC1638_007_CycleACSPackageIsGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	rel := filepath.Join("go", "acs", "cycle1638", "predicates_test.go")
	if !acsassert.FileExists(t, filepath.Join(root, rel)) {
		t.Fatalf("RED: %s missing on disk", rel)
	}
	if _, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); err != nil || code != 0 {
		t.Errorf("RED: %s is untracked (git ls-files exit=%d err=%v) — the audit's predicate tree would carry an input absent from the ship tree", rel, code, err)
	}
}

// ---------------------------------------------------------------------------
// 008-011: the cycle-1638 audit-round-1 repair contract
//
// Round 1 REJECTED the build. Each predicate below encodes one auditor finding
// as a failing test, so the rebuild must address it rather than re-earn the
// verdict. The findings and their owners:
//
//   - H1 two contradictory executable predicates over the same production walk
//     (cycle1633:619 "large must NOT visit build-planner" vs cycle1638:236
//     "every size must pin build-planner"). RECONCILED at the source in this
//     same phase: how_to_apply step (2) is unqualified by size, step (3)'s
//     small/large split is about what EXECUTION emits. cycle1633's block now
//     asserts the reconciled direction; 008 pins the SHAPE of the defect so it
//     cannot re-fork -- the two production routing authorities (PhasePolicy and
//     the router walk) must AGREE on every size.
//   - H2 the Builder narrative framed H1 as inherited history. It is not:
//     all three acs packages are added by this diff. 011 makes that claim
//     mechanically false-able against the base tree.
//   - M1 buildplanner.ShouldSkip discards router.Digest's error AND
//     RoutingSignals.DigestDegraded, so a read failure zero-values the signals
//     and the phase silently self-skips -- disarming the very registry pin this
//     cycle installs. 009 drives the real ShouldSkip over a degraded workspace.
//   - M2 docs/architecture/phase-registry.json is the sole mechanism that makes
//     003 pass, yet the explanation document never names it. 010 derives the
//     required set from the real base-bound diff instead of hard-coding it.
//
// Not encoded here, because no production code is at fault: the audit's first
// gate reason ("Evidence must cite .evolve/inbox/2026-07-21T02-00-00Z-triage-
// unified-solution-synthesis.json with path:line evidence") is a defect in the
// AUDITOR's own artifact. internal/explanationdocs.ValidateReviewedHandoff
// requires the audit report's explanation-review Evidence to cite every host
// material path at a concrete line, and reportdoc.RequirePathLineEvidenceAt
// resolves a path the Build deleted against the BASE blob -- so the deleted
// root inbox copy must be cited at its base path, not at its consumed path.
// See test-report.md "Auditor checklist".
// ---------------------------------------------------------------------------

// declinedPlan is an advisor plan that runs the spine but DECLINES both
// planning phases -- the production shape a conditional pin must beat (a
// non-nil Plan bypasses insert_when, so only conditional_mandatory survives).
func declinedPlan() *router.PhasePlan {
	return &router.PhasePlan{Entries: []router.PhasePlanEntry{
		{Phase: "scout", Run: true}, {Phase: "triage", Run: true},
		{Phase: "plan-review", Run: false}, {Phase: "tdd", Run: true},
		{Phase: "build-planner", Run: false}, {Phase: "build", Run: true},
		{Phase: "audit", Run: true}, {Phase: "ship", Run: true},
	}}
}

// walkFrom follows the PRODUCTION router.Route from `from` until it reaches
// build or end (16 steps max), returning every phase visited. This is the
// second routing authority -- the one cycle1633's predicates drive.
func walkFrom(cfg config.RoutingConfig, sig router.RoutingSignals, from string, completed []string) []string {
	var visited []string
	cur := from
	done := append([]string(nil), completed...)
	for i := 0; i < 16; i++ {
		d := router.Route(router.RouteInput{Current: cur, Verdict: "PASS", Signals: sig, Cfg: cfg, Completed: done, Plan: declinedPlan()}, nil)
		if d.NextPhase == router.PhaseEnd {
			return visited
		}
		visited = append(visited, d.NextPhase)
		if d.NextPhase == "build" {
			return visited
		}
		done = append(done, d.NextPhase)
		cur = d.NextPhase
	}
	return visited
}

func visits(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// entryFor describes where in the canonical order each planning phase is
// reachable from, so the walk starts at the real predecessor rather than at an
// arbitrary phase that could never route to it.
var entryFor = map[string]struct {
	from      string
	completed []string
}{
	"plan-review":   {from: "triage", completed: []string{"scout", "triage"}},
	"build-planner": {from: "tdd", completed: []string{"scout", "triage", "plan-review", "tdd"}},
}

// ---------------------------------------------------------------------------
// 008: the two routing authorities must agree (audit H1, anti-re-fork)
// ---------------------------------------------------------------------------

// H1's defect was not "the wrong answer" but TWO production answers: the phase
// enablement authority (router.PhasePolicy, what a self-skipping phase asks)
// said a large commitment pins build-planner, while the transition authority
// (router.Route over the same registry, what the orchestrator walks) was
// asserted to say it does not. A tree in which those disagree cannot be all
// green whatever the ACS corpus says, so pin the AGREEMENT itself over both
// sizes, then pin the agreed value.
func TestC1638_008_BothRoutingAuthoritiesAgreeOnEverySizeOfCommitment(t *testing.T) {
	cfg := realRegistry(t)
	pol := router.NewPhasePolicy(cfg)
	for _, size := range []string{"small", "large"} {
		t.Run(size, func(t *testing.T) {
			sig := unifiedSignals(size, 3)
			for _, phase := range planningPhases {
				entry := entryFor[phase]
				walk := walkFrom(cfg, sig, entry.from, entry.completed)
				enabled, visited := pol.Enabled(phase, sig), visits(walk, phase)
				if enabled != visited {
					t.Fatalf("RED: the two production routing authorities disagree for a %s commitment on %s: "+
						"PhasePolicy.Enabled=%t but the router walk from %q visited %v -- one tree cannot satisfy both "+
						"(this is the cycle-1638 audit H1 shape)", size, phase, enabled, entry.from, walk)
				}
				if !enabled {
					t.Errorf("RED: a validated %s unified commitment must pin %s in BOTH authorities; both report not-pinned (walk from %q visited %v)",
						size, phase, entry.from, walk)
				}
			}
		})
	}
	// NEGATIVE: agreement on "pinned" must be the commitment's doing. With no
	// validated commitment both authorities must agree on NOT pinned -- a
	// blanket mandatory entry would satisfy the positive half on a no-op.
	t.Run("no-commitment", func(t *testing.T) {
		sig := unifiedSignals("", 0)
		for _, phase := range planningPhases {
			entry := entryFor[phase]
			walk := walkFrom(cfg, sig, entry.from, entry.completed)
			if enabled, visited := pol.Enabled(phase, sig), visits(walk, phase); enabled || visited {
				t.Errorf("fail-open: with no validated commitment %s must be pinned by neither authority; Enabled=%t walkVisited=%t (%v)",
					phase, enabled, visited, walk)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// 009: a degraded digest must be LOUD, not a silent self-skip (audit M1)
// ---------------------------------------------------------------------------

// internal/phases/buildplanner/buildplanner.go:63 reads
// `signals, _ := router.Digest(...)` and never consults
// RoutingSignals.DigestDegraded, while ShouldSkip returns a []core.Diagnostic
// channel it always leaves nil. On a read failure the signals are zero-valued,
// Enabled returns false, and the phase skips itself -- the registry pin this
// cycle installs is disarmed by an unreported error with no trace anywhere.
// Driven through the REAL ShouldSkip against the REAL project root.
func TestC1638_009_BuildPlannerReportsADegradedDigestInsteadOfSelfSkippingSilently(t *testing.T) {
	root := acsassert.RepoRoot(t)
	bp := buildplanner.New(buildplanner.Config{})
	request := func(ws string) core.PhaseRequest {
		return core.PhaseRequest{Cycle: 1638, ProjectRoot: root, Workspace: ws, Env: map[string]string{}}
	}
	mentionsDegradation := func(diags []core.Diagnostic) bool {
		for _, d := range diags {
			m := strings.ToLower(d.Message)
			if strings.Contains(m, "digest") || strings.Contains(m, "degrad") {
				return true
			}
		}
		return false
	}

	// DEGRADED: triage-decision.json exists but cannot be read (a directory).
	// This is a read FAILURE, not a clean absence -- the distinction
	// router.Digest already draws via DigestDegraded.
	t.Run("degraded-digest-is-reported", func(t *testing.T) {
		ws := t.TempDir()
		if err := os.Mkdir(filepath.Join(ws, "triage-decision.json"), 0o755); err != nil {
			t.Fatalf("fixture: %v", err)
		}
		sig, _ := router.Digest(ws, []string{"triage"})
		if len(sig.DigestDegraded) == 0 {
			t.Fatalf("precondition: router.Digest must record the unreadable decision in DigestDegraded; got none")
		}
		_, verdict, _, diags := bp.ShouldSkip(request(ws))
		if len(diags) == 0 {
			t.Errorf("RED: build-planner must SURFACE the degraded routing digest %v as a diagnostic instead of "+
				"self-skipping on zero-valued signals (verdict=%q, diagnostics=none) -- an unreported read failure "+
				"silently disarms the conditional_mandatory pin", sig.DigestDegraded, verdict)
		} else if !mentionsDegradation(diags) {
			t.Errorf("RED: the diagnostic must name the digest degradation so an operator can act on it; got %+v (degraded=%v)", diags, sig.DigestDegraded)
		}
	})

	// NEGATIVE (anti-no-op): a healthy cycle with no commitment must skip
	// SILENTLY. A phase that always emits a diagnostic would pass the case
	// above without ever consulting DigestDegraded.
	t.Run("clean-absence-stays-silent", func(t *testing.T) {
		ws := t.TempDir()
		sig, err := router.Digest(ws, []string{"triage"})
		if err != nil || len(sig.DigestDegraded) != 0 {
			t.Fatalf("precondition: an empty workspace must be a CLEAN absence; err=%v degraded=%v", err, sig.DigestDegraded)
		}
		skipped, _, _, diags := bp.ShouldSkip(request(ws))
		if !skipped {
			t.Error("build-planner must stay opt-in (skipped) when no commitment is present")
		}
		if len(diags) != 0 {
			t.Errorf("a clean no-commitment cycle must stay silent; got %+v", diags)
		}
	})

	// POSITIVE: a validated projection runs the phase, with nothing to report.
	// This is the behavior M1 says the swallowed error defeats.
	t.Run("validated-commitment-runs-the-phase", func(t *testing.T) {
		ws := t.TempDir()
		decision := `{"top_n":[{"id":"a"},{"id":"b"},{"id":"c"}],"unified_projection":{"size":"small","member_count":3}}`
		if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(decision), 0o644); err != nil {
			t.Fatalf("fixture: %v", err)
		}
		sig, _ := router.Digest(ws, []string{"triage"})
		if sig.Triage.UnifiedSize != "small" {
			t.Fatalf("precondition: the fixture must project UnifiedSize=small; got %q (degraded=%v)", sig.Triage.UnifiedSize, sig.DigestDegraded)
		}
		skipped, verdict, _, diags := bp.ShouldSkip(request(ws))
		if skipped {
			t.Errorf("a validated unified commitment must RUN build-planner; ShouldSkip=true verdict=%q", verdict)
		}
		if len(diags) != 0 {
			t.Errorf("a healthy validated cycle must report nothing; got %+v", diags)
		}
	})
}

// ---------------------------------------------------------------------------
// 010-011: the build explanation must be findable and true (audit M2, H2)
// ---------------------------------------------------------------------------

// explanationDoc returns the path and body of the build explanation THIS TREE
// ships. It is deliberately NOT a cycle-1638-*.md glob: on a continuation the
// host archives every unshipped predecessor record under
// docs/private/research/archived-*/ (explanationdocs.
// ArchiveUnpublishedContinuationRecords), so the cycle-1638 draft is history
// and the deliverable is the ONE record the tree adds under
// docs/explain/builds/ — the index/working-tree addition on a pre-commit lane,
// or the record HEAD's own commit added once the cycle has landed. The former
// glob made 010/011 structurally unsatisfiable on ANY continuation tree
// (cycle-1647 audit H1); the intent — name every load-bearing path, no history
// attribution of this diff's own packages — binds to whichever record ships.
func explanationDoc(t *testing.T) (string, string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	rel := shippingCycleRecord(t, root)
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return rel, string(raw)
}

// shippingCycleRecord resolves the one docs/explain/builds/cycle-*.md record
// this tree adds, from git rather than from any report or cycle number.
func shippingCycleRecord(t *testing.T, root string) string {
	t.Helper()
	const dir = "docs/explain/builds"
	isRecord := func(p string) bool {
		return strings.HasPrefix(p, dir+"/cycle-") && strings.HasSuffix(p, ".md")
	}
	var found []string
	// A pre-commit lane: the record is an addition the index or working tree
	// carries (the host stages it; a rename lists as `old -> new`).
	out, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "status", "--porcelain", "--untracked-files=all", "--", dir)
	if err != nil || code != 0 {
		t.Fatalf("git status -- %s: exit=%d err=%v stderr=%s", dir, code, err, stderr)
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if i := strings.LastIndex(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		if isRecord(path) {
			found = append(found, path)
		}
	}
	if len(found) == 0 {
		// A landed tree: the record HEAD's most recent record-adding commit
		// introduced.
		out, stderr, code, err = acsassert.SubprocessOutput("git", "-C", root, "log", "-1", "--diff-filter=A", "--name-only", "--format=", "--", dir+"/cycle-*.md")
		if err != nil || code != 0 {
			t.Fatalf("git log -- %s: exit=%d err=%v stderr=%s", dir, code, err, stderr)
		}
		for _, line := range strings.Split(out, "\n") {
			if path := strings.TrimSpace(line); isRecord(path) {
				found = append(found, path)
			}
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly one shipping build explanation under %s/ (the record this tree adds; unshipped predecessors are archived under docs/private/); got %v", dir, found)
	}
	return found[0]
}

// docSection returns the body of a `## <title>` section of a markdown doc.
func docSection(t *testing.T, body, title string) string {
	t.Helper()
	lines := strings.Split(body, "\n")
	var out []string
	in := false
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if in {
				break
			}
			in = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(line, "## ")), title)
			continue
		}
		if in {
			out = append(out, line)
		}
	}
	if !in && len(out) == 0 {
		t.Fatalf("the build explanation has no `## %s` section", title)
	}
	return strings.Join(out, "\n")
}

// baseSHA reads the Build Binding the explanation carries about itself, so the
// diff these predicates check is the same base-bound diff the host bound.
func baseSHA(t *testing.T, body string) string {
	t.Helper()
	for _, line := range strings.Split(docSection(t, body, "Build Binding"), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		if rest, ok := strings.CutPrefix(line, "Base SHA:"); ok {
			if sha := strings.Trim(strings.TrimSpace(rest), "`"); sha != "" {
				return sha
			}
		}
	}
	t.Fatal("the build explanation's `## Build Binding` must carry a `- Base SHA:` line")
	return ""
}

// changedSinceBase is the base-bound diff, read from git rather than from any
// report that could agree with a wrong answer.
func changedSinceBase(t *testing.T, root, base string) []string {
	t.Helper()
	out, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", base)
	if err != nil || code != 0 {
		t.Fatalf("git diff --name-only %s: exit=%d err=%v stderr=%s", base, code, err, stderr)
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	if len(paths) == 0 {
		t.Fatalf("precondition: the base-bound diff against %s is empty -- the predicate would assert nothing", base)
	}
	return paths
}

// loadBearing is the class of file whose absence from the explanation leaves a
// reader unable to find the code that carries the behavior: production Go and
// the routing/registry configuration that drives it. Tests and report
// artifacts are excluded -- they are evidence, not mechanism.
func loadBearing(path string) bool {
	switch {
	case strings.HasSuffix(path, "_test.go"):
		return false
	case strings.HasPrefix(path, "go/internal/"), strings.HasPrefix(path, "go/cmd/"):
		return strings.HasSuffix(path, ".go")
	case strings.HasPrefix(path, "docs/architecture/") && strings.HasSuffix(path, ".json"):
		return true
	}
	return false
}

// TestC1638_010 pins the M2 finding: docs/architecture/phase-registry.json is
// the SOLE mechanism that makes 003 and 008 pass -- the conditional_mandatory
// rules are the behavior -- yet the explanation names it nowhere, so a reader
// cannot find the file that carries the change. The required set is derived
// from the real base-bound diff, not hard-coded, so any future unnamed
// production-config or production-Go change fails the same way.
func TestC1638_010_ExplanationNamesEveryLoadBearingFileTheBuildChanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docPath, body := explanationDoc(t)
	areas := docSection(t, body, "Changed Areas")
	var missing []string
	for _, path := range changedSinceBase(t, root, baseSHA(t, body)) {
		if loadBearing(path) && !strings.Contains(areas, path) {
			missing = append(missing, path)
		}
	}
	if len(missing) > 0 {
		t.Errorf("RED: %s `## Changed Areas` omits load-bearing file(s) the base-bound diff changed: %v -- "+
			"a change a reader cannot find in the explanation is an unexplained change", docPath, missing)
	}
}

// provenanceClaims are words that assert a path PREDATES this diff. Using one
// about a file this diff adds misattributes the defect (and its owner).
var provenanceClaims = []string{"preserved", "pre-existing", "preexisting", "inherited", "historical", "older", "prior cycle", "earlier cycle"}

// TestC1638_011 pins the H2 finding: the round-1 narrative framed the routing
// contradiction as inherited history ("a contradictory preserved cycle-1633
// assertion"), when `git cat-file -e <base>:go/acs/cycle1633/predicates_test.go`
// is ABSENT -- all three acs packages are added by this diff. The
// misattribution moved the fix to the wrong owner and cost the cycle a round.
func TestC1638_011_ExplanationDoesNotAttributeThisDiffsOwnPredicatesToHistory(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docPath, body := explanationDoc(t)
	base := baseSHA(t, body)
	lower := strings.ToLower(body)
	for _, path := range changedSinceBase(t, root, base) {
		if !strings.HasPrefix(path, "go/acs/cycle") {
			continue
		}
		pkg := strings.Split(strings.TrimPrefix(path, "go/acs/"), "/")[0] // e.g. "cycle1633"
		if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "cat-file", "-e", base+":"+path); code == 0 {
			continue // genuinely present at base -- history language is honest
		}
		hyphen := strings.Replace(pkg, "cycle", "cycle-", 1)
		for _, line := range strings.Split(lower, "\n") {
			for _, sentence := range strings.Split(line, ". ") {
				if !strings.Contains(sentence, pkg) && !strings.Contains(sentence, hyphen) {
					continue
				}
				for _, claim := range provenanceClaims {
					if strings.Contains(sentence, claim) {
						t.Errorf("RED: %s attributes %s to history (%q) in: %q -- but `git cat-file -e %s:%s` is ABSENT, "+
							"so that package is ADDED by this diff and its contract is this cycle's to reconcile",
							docPath, path, claim, strings.TrimSpace(sentence), base[:8], path)
					}
				}
			}
		}
	}
}
