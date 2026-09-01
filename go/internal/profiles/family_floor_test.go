package profiles

import (
	"strings"
	"testing"
)

// family_floor_test.go — the claude-family FLOOR (2026-09-02 operator
// directive: rebalance claude/codex usage; claude is the quota-constrained
// family). claudeFamilyFloor is the ONE home of "which phases stay claude and
// why" — the deep-tier arrangement guard projects its exceptions from this
// map rather than restating it (2026-09-02 architecture review: the belief
// briefly had three homes, one self-contradicting).
//
// THE HONEST PREDICATE (stated because the tree falsifies the tempting one):
// this floor is NOT "everything that feeds a blocking verdict". The residual
// claude set is: the two ADVERSARIAL graders whose judgment of build CONTENT
// must be cross-family (anti-gaming core), the test author (anti-cooperative
// -bias family split from the builder), and the two spec verifiers
// (audit-side verification of build output). Verdict-DECLARING mechanical
// gates — merge-to-main-gate, coverage-gate — are deliberately on codex:
// their verdicts are deterministic measurements the host re-verifies, not
// adversarial judgment. The classify.require_sections("Verdict") discriminator
// exists in phase.json but deliberately does NOT map onto this floor.
//
// Every floor entry keeps cli_fallback [] ON PURPOSE: a claude-quota halt on
// a floored phase must fail loudly, because the "obvious" operator remedy —
// adding a codex fallback to the auditor — silently puts codex in judgment
// of codex on every fallback dispatch. The guard binds cli, cli_fallback,
// AND allowed_clis (the policy-pin validator's enforcement surface, mirrored
// per the tdd-engineer precedent) so no plane can breach the floor quietly.
//
// Tier and effort facts live with their own guards
// (deep_tier_family_arrangement_test.go, effort_defaults_test.go), never
// restated here. Runtime-minted audit-side stubs
// (pre-audit-evidence-check, production-path-wiring-proof,
// defect-disposition-*, inherited-defect-reconcile, ship-stage-hygiene-check)
// are UNTRACKED runtime-plane state — RealTreeProfiles filters them by design
// (the 2026-08-09 zero-ship class); their family is the minting registrar's
// contract, not this guard's.
var claudeFamilyFloor = map[string]string{
	"auditor":            "adversarial grading of build content — cross-family anti-gaming core",
	"adversarial-review": "adversarial grading of build content — cross-family anti-gaming core",
	"tdd-engineer":       "test author — anti-cooperative-bias family split from the builder",
	"spec-verifier":      "audit-side verification of build output",
	"spec-verify":        "audit-side verification of build output",
}

// family reduces a driver name to its CLI family (claude-tmux and a future
// claude-p are the same family for floor purposes). Local mirror of
// policy.BaseCLI semantics — importing internal/policy here would cycle.
func family(cli string) string {
	if i := strings.IndexByte(cli, '-'); i > 0 {
		return cli[:i]
	}
	return cli
}

// TestClaudeFamilyFloor holds both directions, value-free against the live
// builder so a future builder flip cannot make the floor self-contradictory:
//
//	forward: a claude-family profile outside the floor is unjustified spend on
//	         the quota-constrained family (2026-09-02: 24 such moved to codex,
//	         triage included — the every-cycle spine win);
//	reverse: every floor entry must resolve, must NOT share the builder's
//	         family (on cli, on every cli_fallback entry, and on every
//	         allowed_clis entry — the policy-pin plane), and must keep the
//	         loud-failure empty fallback.
func TestClaudeFamilyFloor(t *testing.T) {
	loader, names := RealTreeProfiles(t)
	builder, err := loader.Get("builder")
	if err != nil {
		t.Fatalf("Get(builder): %v", err)
	}
	builderFam := family(builder.CLI)

	claudeCount := 0
	for _, name := range names {
		p, err := loader.Get(name)
		if err != nil {
			t.Errorf("profile %s: Get failed (%v) — it cannot be checked, so it cannot be trusted", name, err)
			continue
		}
		if family(p.CLI) != "claude" {
			continue
		}
		claudeCount++
		if _, ok := claudeFamilyFloor[name]; !ok {
			t.Errorf("profile %s: claude family without a floor justification — claude is the quota-constrained family (2026-09-02 directive); either add it to claudeFamilyFloor WITH its load-bearing reason, or route it to codex", name)
		}
	}
	if claudeCount == 0 {
		t.Fatal("matched NO claude-family profiles — the selector is broken and this guard is vacuous")
	}

	for name, why := range claudeFamilyFloor {
		p, err := loader.Get(name)
		if err != nil {
			t.Errorf("floor entry %s (%s) does not resolve in the TRACKED tree: %v — a floor pinned on a missing/untracked profile is vacuous", name, why, err)
			continue
		}
		if family(p.CLI) == builderFam {
			t.Errorf("floor entry %s runs on the builder's own family %q — the floor (%s) is broken", name, p.CLI, why)
		}
		if len(p.CLIFallback) != 0 {
			t.Errorf("floor entry %s: cli_fallback=%v, want [] — a floored phase fails LOUDLY on quota rather than silently handing %s to the builder's family (see the floor doc)", name, p.CLIFallback, why)
		}
		for _, allowed := range p.AllowedCLIs {
			if allowed == "all" || family(allowed) == builderFam {
				t.Errorf("floor entry %s: allowed_clis=%v admits the builder's family — a policy pin could breach the floor at dispatch while every profile guard stays green (mirror the tdd-engineer allowed_clis precedent)", name, p.AllowedCLIs)
				break
			}
		}
	}
}
