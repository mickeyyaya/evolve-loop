# Comment history: `acs/cycle1623`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1623/predicates_test.go:3` — above `package cycle1623`

```text
// Package cycle1623 materializes the acceptance criteria for cycle 1623's
// AUDIT-REPAIR round. The cycle's first round selected the inbox item
// `agy-tier-map-single-source`, but triage committed an EMPTY `top_n` (the
// atomic inbox claim failed), and the audit REJECTED the result. The audit's
// findings — not the unclaimed inbox item — are this round's scope.
//
// Why the agy predicates that used to live here are gone: triage-decision.json
// lists `agy-tier-map-single-source` under `deferred`, never `top_n`. R9.3
// binds predicates to triage-COMMITTED work only, and the host's floor-binding
// gate rejects a predicate that gates deferred work (cycle-280: predicates
// gating a deferred item starved the committed task). Those predicates were
// also the audit's own M1 finding — untracked RED residue that any later lane
// inheriting this worktree would inherit as a false regression. They are
// removed here, not weakened: the work they encoded is deferred intact and its
// contract is re-authored by the cycle that actually claims the item.
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC-R1 (audit H1) sandbox denies the inbox-claim path  → manual+checklist (NOT a predicate; see below)
//	AC-R2 (audit H2) empty top_n must terminate the cycle → TestC1623_001_EmptyTopNTerminatesInsteadOfDispatchingSpine
//	AC-R3 (audit H2) a committed top_n must still advance → TestC1623_002_CommittedTopNStillAdvancesTheSpine
//	AC-R4 (audit M2) claim failure must strand no item    → TestC1623_003_ClaimFailureLeavesInboxItemInPlace
//	AC-R5 (audit M1) this cycle's ACS package is tracked  → TestC1623_004_CycleACSPackageIsGitTracked
//
// AC-R1 carries NO predicate on purpose. The audit located H1's fix at
// go/internal/bridge/sandbox_wrap.go:209 (sandboxWritePaths omits the
// project-root inbox from the write allowlist). Both that file and
// go/internal/adapters/sandbox/ are entries in
// guards.ProtectedSurfaceManifest — verified live, not inferred:
// guards.IsProtectedSurface("go/internal/bridge/sandbox_wrap.go") == true,
// which internal/guards/role.go:62 denies at write time and
// internal/phases/ship/integrity.go:30 blocks at ship. A lane CANNOT land that
// edit. Freezing a doNotModifyTests predicate against it would be exactly the
// cycle-644 shape: an acceptance criterion that is unsatisfiable by
// construction, burning the whole cycle. It is dispositioned
// manual+checklist and routed to the console owner instead.
//
// Adversarial axes (skills/adversarial-testing §6):
//   - NEGATIVE — TestC1623_002 is the anti-no-op: a "fix" that simply always
//     terminates after triage passes 001 and FAILS 002. The gate must key on
//     the committed count, not on the phase.
//   - EDGE — TestC1623_003 drives the permission-denied branch (the exact
//     failure mode that blanked this cycle), not the happy path.
//   - SEMANTIC — 001 pins the routing DECISION, 003 pins inbox ownership
//     ATOMICITY, 004 pins ship-tree TRACKING: three distinct behaviors.
//
// No grep-only predicates (the cycle-85 ban): 001/002 drive the real exported
// router.Digest → router.Route composition over a real on-disk workspace (the
// composed dispatch path, per lesson inst-L1563a — never a hand-built signal
// literal); 003 calls the real exported inboxmover.Claim and asserts on the
// filesystem side effect; 004 asserts on `git ls-files` exit status.
```

### `go/acs/cycle1623/predicates_test.go:78` — above `func spineConfig() config.RoutingConfig {`

```text
// spineConfig is a production-shaped RoutingConfig whose Mandatory spine
// INCLUDES build. That is deliberate: cycle 1623 routed `spine:build` off an
// empty top_n (routing-decision-4.json), so the contract under test is that
// the empty-commitment gate outranks the mandatory spine. Mirrors the
// defaults internal/routingtest.buildConfig fills in.
```

### `go/acs/cycle1623/predicates_test.go:121` — above `func routeAfterTriage(sig router.RoutingSignals) router.RouterDecision {`

```text
// routeAfterTriage runs the real pure kernel for the triage→next-phase edge —
// the exact transition that produced routing-decision-3/4.json in cycle 1623.
```

### `go/acs/cycle1623/predicates_test.go:181` — above `func TestC1623_003_ClaimFailureLeavesInboxItemInPlace(t *testing.T) {`

```text
// TestC1623_003_ClaimFailureLeavesInboxItemInPlace encodes audit finding M2.
// inboxmover.Claim mkdirs `<inbox>/processing/cycle-N` and only then renames
// the item in. When the mkdir fails — the exact sandbox denial that blanked
// cycle 1623's top_n — the item must remain in the inbox root, claimable by a
// later cycle. Today that holds only by statement ordering
// (inboxmover.go:209-215) and nothing asserts it, so a future reordering could
// strand the item silently.
```

### `go/acs/cycle1623/predicates_test.go:221` — above `func TestC1623_004_CycleACSPackageIsGitTracked(t *testing.T) {`

```text
// TestC1623_004_CycleACSPackageIsGitTracked encodes audit finding M1 and the
// audit gate's own stated reason ("predicate execution tree includes
// undeclared inputs absent from the ship tree"). Cycle 1623's predicates were
// left UNTRACKED, so the audit's predicate tree and the ship tree disagreed.
// Disk presence alone is not enough — the cycle-93 lesson: an untracked file
// is silently dropped at ship.
```

### `go/acs/cycle1623/predicates_test.go:238` — above `type dispatchLog struct {`

```text
// ---------------------------------------------------------------------------
// AUDIT-REPAIR ROUND 3 — audit round 2 finding H1 (CRITICAL).
//
// Round 2's gate was written and graded at router.Route. Route's decision is
// only a PROPOSAL: go/internal/core/cyclerun_select.go:103 routes it through
// Orchestrator.enforceNext, whose PhaseEnd branch
// (go/internal/core/routing_dispatch.go:59-63) asks
// StateMachine.CanTerminateEarly(current, shipPlanned) — and that returns false
// unconditionally when shipPlanned is true
// (go/internal/core/statemachine.go:230-233). Cycle 1623's own clamped plan
// schedules ship, so the proposal was dropped and the orchestrator dispatched
// tdd anyway. TestC1623_001 stayed GREEN through the whole defect because it
// asserts one layer ABOVE the authority that decides the next phase.
//
// The three predicates below re-grade that criterion where the decision is
// CONSUMED, not where it is produced: they drive a real core.Orchestrator
// through a real RunCycle with routing at the live advisory stage and no
// clamped plan (planRunsShip(nil) == true — the exact ship-planned
// configuration that made the round-2 gate inert) and assert on the phases the
// orchestrator ACTUALLY DISPATCHED. A fix that only changes router.Route
// leaves 005 RED.
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1623/predicates_test.go:318` — above `func triageDecisionJSON(committedIDs ...string) string {`

```text
// triageDecisionJSON renders a triage-decision.json committing exactly the
// given task ids in top_n. No ids ⇒ the explicit empty commitment that blanked
// cycle 1623.
```

### `go/acs/cycle1623/predicates_test.go:354` — above `func composedRoutingConfig() config.RoutingConfig {`

```text
// composedRoutingConfig is the LIVE dispatch configuration: routing at the
// advisory stage (the production default since 2026-06-06), the full ordered
// spine, and triage force-enabled the way the phase registry enables it in
// production. Derived from spineConfig so the pure-kernel predicates (001/002)
// and the composed ones cannot drift apart on the mandatory set.
```

### `go/acs/cycle1623/predicates_test.go:394` — above `func runComposedCycle(t *testing.T, triageHook func(core.PhaseRequest) error) (*dispatchLog, error) {`

```text
// runComposedCycle drives the REAL orchestrator — core.NewOrchestrator +
// RunCycle — over the real routing kernel (router.StaticPreset, whose Decide is
// the same pure Route the production loop calls). No planner is wired, so
// clampedPlan is nil and planRunsShip reports SHIP PLANNED: the configuration
// under which the round-2 gate was proven inert.
//
// It returns the dispatch log AND RunCycle's error rather than failing on the
// error, deliberately. The contract under test is the triage→next DISPATCH
// DECISION, which is settled before the epilogue: in the control cases the
// no-LLM build runner cannot satisfy the real explanation-documentation floor,
// so RunCycle legitimately ends in a build-floor rejection AFTER the phases
// under test have already been dispatched. Every assertion below quotes both
// the dispatch log and this error, and each test guards against a vacuous pass
// by requiring the cycle to have reached triage at all — so an early abort can
// never be mistaken for a satisfied contract.
```

### `go/acs/cycle1623/predicates_test.go:458` — above `func TestC1623_005_EmptyCommitmentTerminatesOnTheComposedDispatchPath(t *testing.T) {`

```text
// TestC1623_005_EmptyCommitmentTerminatesOnTheComposedDispatchPath re-grades
// audit finding H2 at the layer that DECIDES the next phase. Round 2's fix
// stops at router.Route; enforceNext discards its PhaseEnd whenever ship is
// planned, so the orchestrator still dispatches tdd. This drives the composed
// path end to end and asserts on what was actually dispatched.
```
