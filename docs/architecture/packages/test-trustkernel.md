# test/trustkernel

> The invariant-to-test map is `go/docs/testing.md`, section G3. The port of the old per-cycle predicates is in `go/test/trustkernel/PORTING-LEDGER.md`. This page keeps the shape of the package.

## Purpose

`go/test/trustkernel` holds black-box tests that pin the trust-kernel invariants: the small set of safety properties that the whole evolve-loop relies on. They are the ship gate, the audit binding, the routing integrity floor, the legality of phase transitions and the validity of profiles. Each test calls the real exported Go code in `go/internal/...`. No test is a grep over source. Each test is named for the behavior that it pins, never for a cycle.

## Design

- **One group per invariant**, each with its pinning tests:

  | Invariant | Pinning tests |
  |---|---|
  | A cycle is ship-eligible only when the EGPS predicate suite has zero RED predicates. It is the same `red_count` that the ship phase refuses to ship on. | `TestShipGate_ShipEligibleOnlyWhenRedCountZero`, `TestShipGate_BlocksWhenRedCountNonZero` |
  | `reach(ship)` implies build and audit, and tdd unless the cycle is trivial. `router.ClampPlanToFloor` is the floor. It cannot be configured away, and no routing strategy or mandatory-phase set can weaken it. | `TestRoutingFloor_ShipRequiresBuildAndAudit`, `TestRoutingFloor_NoShipCycleIsUnconstrained`, `TestRoutingFloor_TrivialCycleExemptsTDDNotBuildAudit` |
  | Ship is reachable only after audit. Ship hands off only to the end phase or to a recovery phase, never back up the spine. `core.StateMachine` holds the transition table. | `TestStateMachine_ShipFollowsAuditOnlyViaShippableVerdict`, `TestStateMachine_AuditVerdictRoutesShipOrRetro` |
  | Every phase profile on disk is well-formed JSON that declares its routing fields (`name` and `cli`). The dispatcher reads them before it launches a subagent. | `TestProfile_AllPhaseProfilesValid` |

- **The EGPS fixture runs real `go test -tags acs`.** `writeGoPredFixture` writes a minimal Go ACS predicate module, passing or RED, under a temp root. `acssuite.Run` then discovers it through its Go lane. That is the integration that the ship gate depends on.
- **The routing tests use an adversarial plan.** One plan jumps straight to ship and skips the chain. A scout-only investigation cycle has a false antecedent, so it imposes no floor.
- **The transition tests include the illegal edges.** `audit` to `ship` is legal. `scout` to `ship` is not, and neither is `build` to `ship`, because audit sits between them.
- **`repoRootOrSkip` finds the git checkout** so that the on-disk fixtures (`.evolve/profiles`) are found from any working directory. Outside a git checkout it skips the test and does not fail it, which keeps the tier portable.
- **Non-profile JSON is passed over.** `.evolve/profiles` also holds policy files (for example `tool-policy.json`) with no `name` field. The profile test validates agent profiles, not policy config, so it skips a file with no name.

## Invariants

- **Each group above has a test that fails when the rule breaks.** The knowledge documents for them are `knowledge/architecture/trust-kernel-and-egps.md`, `routing-and-advisor.md`, `phase-pipeline.md` and `cli-matrix-and-drivers.md`.
- **Pending ports.** The audit-binding tree SHA, the single-writer and worktree isolation, and schema-filter enforcement are tracked in the porting ledger. They are not pinned here yet.

## Findings

- None open.
