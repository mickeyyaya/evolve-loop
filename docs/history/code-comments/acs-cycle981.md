# Comment history: `acs/cycle981`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle981/predicates_test.go:3` — above `package cycle981`

```text
// Package cycle981 materializes the cycle-981 acceptance criteria for the sole
// inbox item this fleet lane is pinned to: prefix-speculation-landing-queue
// (.evolve/inbox/2026-07-13T14-21-00Z-prefix-speculation-landing-queue.json,
// weight 0.93, campaign merge-efficiency-2026-07). Per R9.3 no predicate here
// binds to any other lane's items — fleet_scope pins this lane to exactly this id.
//
// Scout split the item into two dependent, independently-verifiable tasks:
//
//	Task 1  salvage-prefix-queue-composer-core   — promote cycle-975's audited
//	        go/internal/fleet/prefixqueue.go (PASS 0.90, untracked in
//	        .evolve/worktrees/cycle-21f9f7ae-975) into the main lineage verbatim.
//	Task 2  prefix-queue-ship-wiring-and-policy   — the deferred AC4/AC5 wiring:
//	        add FleetPolicy.Landing (per-lane|prefix-queue, shadow-first closed
//	        vocab mirroring FleetPolicy.Scheduling) + the ship-phase seam that
//	        ROUTES landing through fleet.PrefixQueue when prefix-queue is selected.
//
// SUT SURFACE the Builder must add WITHOUT modifying this file (the RED contract —
// these symbols/fields do not exist yet in the main lineage, so this predicate
// package FAILS TO COMPILE now, which is the correct greenfield RED per
// go/acs/README.md "a predicate package that fails to compile is a HARD suite
// error"):
//
//	Task 1 (promote verbatim from the cycle-975 worktree, package go/internal/fleet):
//	  type PrefixQueue, LaneCandidate, RiskTier + NewPrefixQueue/Enqueue/Window/
//	  OnGreen/OnRed/ComposePrefixes/ResolveCulprit + LandingMode/ParseLandingMode.
//
//	Task 2 (new):
//	  // go/internal/policy — mirror the Scheduling closed-vocab resolver:
//	  FleetPolicy.Landing string   // json:"landing,omitempty"
//	  FleetConfig.Landing string   // resolved: "per-lane" (default) | "prefix-queue"
//	                               // unknown => "per-lane" + a surfaced Warnings entry
//
//	  // go/internal/phases/ship — the WIRING SEAM (the single function the
//	  // main-push path consults; it must NOT be a parallel getter — the postship
//	  // landing decision routes through it):
//	  func PlanLanding(cfg policy.FleetConfig, lanes []fleet.LaneCandidate) [][]string
//	  //   per-lane    => each lane lands independently (legacy, byte-identical):
//	  //                  one singleton group per lane, never a multi-lane group.
//	  //   prefix-queue=> routes the lanes through fleet.PrefixQueue.ComposePrefixes
//	  //                  so the single-writer composer owns the main-push decision.
//
// PREDICATE STYLE (cycle-85 anti-gaming rule): every predicate CALLS the SUT and
// asserts on its return value — no source-grep predicate exists here. go/internal
// is importable from go/acs (cycle-962 imports internal/core precedent).
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	POSITIVE → C981_001 (good lanes land around a poisoned middle; AIMD grows on
//	           green), C981_002 (prefix-queue mode composes two lanes into one
//	           prefix group), C981_003 (canonical modes resolve).
//	NEGATIVE → C981_001 (the poisoned lane must NOT land, and NNFI resolves in a
//	           LINEAR verify budget — no bisection sweep), C981_002 (per-lane and
//	           prefix-queue plans MUST DIFFER — a wiring that ignores config and
//	           always returns per-lane is INERT and fails this; the exact cycle-975
//	           "composer stays inert" risk the Auditor flagged), C981_003 (a bogus
//	           landing value must fail SAFE to per-lane WITH a warning — never
//	           silently enter composer mode).
//	EDGE     → C981_001 (window floors at 1 under repeated reds), C981_002
//	           (empty lane set => empty plan, both modes).
//	SEMANTIC → salvaged-composer-behavior / ship-wiring-routes-through-composer /
//	           policy-vocabulary-resolution are three DISTINCT behaviors.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	AC1 salvaged fleet.PrefixQueue present & behaves (ejection + AIMD)  → C981_001 (predicate)
//	AC2 cycle-975 predicates pass in promoted loc + fleet race green    → manual+checklist (Auditor)
//	AC3 full repo build green (go build ./...)                          → manual+checklist (Auditor)
//	AC4 ship PlanLanding routes through the composer iff prefix-queue,
//	    legacy per-lane otherwise (default byte-identical; gate-wiring)  → C981_002 (predicate)
//	AC5 policy resolves fleet.landing as closed vocab mirroring
//	    scheduling: default per-lane, prefix-queue ok, unknown fail-safe → C981_003 (predicate)
```

### `go/acs/cycle981/predicates_test.go:105` — above `func TestC981_001_SalvagedComposerBehaves(t *testing.T) {`

```text
// C981_001 — AC1: the salvaged composer is present in the main lineage and
// behaves per its contract. Task 1 is a verbatim promotion, so this re-exercises
// the two load-bearing behaviors (positional NNFI culprit ejection + AIMD window)
// to prove the package actually landed and works outside the cycle-975 worktree.
```

### `go/acs/cycle981/predicates_test.go:161` — above `func TestC981_002_ShipWiringRoutesThroughComposer(t *testing.T) {`

```text
// C981_002 — AC4: the ship-phase landing seam ROUTES through the composer iff
// policy selects prefix-queue, and falls back to the legacy independent per-lane
// plan otherwise. This is the gate-WIRING proof: the two modes must produce
// OBSERVABLY DIFFERENT landing plans, so an inert wiring (config ignored, always
// per-lane — the exact cycle-975 failure the Auditor flagged) cannot pass.
```
