# Comment history: `acs/cycle1057`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1057/predicates_test.go:3` — above `package cycle1057`

```text
// Package cycle1057 encodes the cycle-1057 acceptance criteria for
// `retro-artifact-budget-perphase` (retry of the cycle-1054 audit-FAIL):
// a per-phase bridge artifact-wait budget (`BridgePolicy.PhaseArtifactTimeoutS`,
// compiled default {"retrospective": 900, "retro": 900}) threaded
// policy → adapters/bridge.productionEngineDeps → bridge.Deps → Engine.Launch
// (arg vector `--artifact-timeout-s=N`) → parseLaunchArgs → Config.ArtifactTimeoutS
// → the existing tmux artifact-wait loop, while every other phase keeps the
// 300s builtin.
//
// Source incident: cycle-1048's retro was ctx-canceled at ~608s because the
// global 300s artifact deadline is too small for the grown retro contract
// (report + preventive_actions + disposition.json).
//
// Key correction over cycle-1054 (architecture-design.md Axis B): the live
// retro launch passes Agent: "retrospective" (internal/phases/retro/retro.go),
// NOT "retro" — a map keyed only on "retro" would be unit-green and live-dead.
// TestC1057_008 is the behavioral drift guard binding the compiled key to the
// label the real retro phase actually dispatches with.
//
// Every predicate here exercises the system under test (policy resolution, the
// real Engine.Launch dispatch path, the real production adapter root, the real
// retro phase). No source-grep assertions.
//
// PARTIALLY SUPERSEDED (inbox item deep-phase-artifact-budget-too-small): the
// "every other phase keeps the 300s builtin" clause above was deliberately
// NARROWED, not abandoned. build/audit/tdd/adversarial-review now carry compiled
// 1200s budgets because ~650s (300s base × 6 extends) killed six deep-tier
// phases in one day with no artifact at all. The invariant this suite pins —
// an UNLISTED phase resolves 0, the "use the builtin" sentinel, so global hang
// detection is not weakened across the board — is unchanged and is now probed
// through phases that are still unlisted (scout/intent/triage/ship).
```

### `go/acs/cycle1057/predicates_test.go:398` — above `type captureBridge struct {`

```text
// captureBridge records the BridgeRequest the retro phase dispatches, so the
// agent label under test is the one the REAL phase produces — not a constant
// copied from a report. This is the anti-recurrence proof for the cycle-1054
// defect class (compiled key "retro" vs live label "retrospective"): a rename
// on either side turns this predicate RED instead of silently restoring the
// 300s timeout in production.
```
