# Trust-Kernel Porting Ledger

Tracks the migration of the 240 cycle-pegged `TestC<N>_<id>_*` predicate tests
under `go/acs/cycle*/predicates_test.go` (Go ports of the bash EGPS predicates)
into behavior-named black-box tests in `go/test/trustkernel/`.

The `go/acs/` predicates are NOT deleted in this stage — they keep running in the
live EGPS suite. This ledger is the categorization that makes the remaining port
(and the eventual Stage 5 retirement of the cruft) a tracked follow-up, not a
guess.

## Triage rubric

- **KEEP-INVARIANT** — encodes a durable trust-kernel safety property (ship gate,
  audit-binding, routing floor, transition legality, profile/allowlist validity,
  single-writer/worktree isolation, schema enforcement). Worth a permanent
  behavior-named test in `trustkernel/` that exercises real Go code.
- **RETIRE-CRUFT** — cycle-incident bookkeeping: "roadmap entry exists", "doc on
  disk", "lessons merged", "binary version is vX.Y.Z", "PNewNN investigation
  complete", per-cycle cost snapshots, RCA-doc-exists. These assert that a
  one-time cycle artifact landed; they have no ongoing invariant value and should
  be dropped at Stage 5 rather than ported.

## Population summary (240 funcs)

| Bucket | Count (approx) | Disposition |
|---|---|---|
| KEEP-INVARIANT | ~44 | Port to `trustkernel/` (8 done this batch) |
| RETIRE-CRUFT (explicit markers) | ~45 | Drop at Stage 5 |
| NEITHER → mostly RETIRE | ~151 | Re-triage; the bulk are cycle-incident/feature-presence checks already subsumed by in-package `internal/*_test.go`; a small residue (e.g. routing-through-build-planner, retrospective-YAML-contract) are KEEP candidates pending per-func read |

Counts are heuristic (name-pattern partition); each func gets a definitive
KEEP/RETIRE stamp when its body is read during the remaining port.

## Ported this batch (8 — all GREEN against real Go code)

| trustkernel test | Invariant | Real code exercised | Legacy predicate(s) subsumed |
|---|---|---|---|
| `TestShipGate_ShipEligibleOnlyWhenRedCountZero` | all-green suite ⇒ ship-eligible | `acssuite.Run` | `C47_ShipRefused*`, EGPS red_count predicates |
| `TestShipGate_BlocksWhenRedCountNonZero` | any RED ⇒ verdict FAIL, not ship-eligible | `acssuite.Run` | `C102_003_IncidentDocShipRefused`, `C86_CarryoverShipRefusedDismissed` |
| `TestRoutingFloor_ShipRequiresBuildAndAudit` | reach(ship) ⇒ build ∧ audit ∧ tdd | `router.ClampPlanToFloor` | `C104_*` routing-floor predicates |
| `TestRoutingFloor_NoShipCycleIsUnconstrained` | no-ship cycle imposes no floor | `router.ClampPlanToFloor` | (floor antecedent) |
| `TestRoutingFloor_TrivialCycleExemptsTDDNotBuildAudit` | trivial ⇒ tdd exempt, build/audit not | `router.ClampPlanToFloor` | conditional-mandatory tdd predicates |
| `TestStateMachine_ShipFollowsAuditOnlyViaShippableVerdict` | scout/build → ship illegal; audit → ship legal | `core.StateMachine.CanTransition` | `C106_002_StateMachineTransitions` |
| `TestStateMachine_AuditVerdictRoutesShipOrRetro` | PASS/WARN → ship, FAIL → retro | `core.StateMachine.Next` | `C48_003_AuditorStopCriterionHardGate` |
| `TestProfile_AllPhaseProfilesValid` | every `.evolve/profiles/*.json` is valid JSON with name+cli | on-disk profiles | `C87_ProfilesJsonValidate`, `C103_002_BuildPlannerProfileValid` |

## KEEP-INVARIANT — remaining port queue (representative; not exhaustive)

| Legacy predicate | Invariant it pins | Likely real-code target |
|---|---|---|
| `C41_001_TesterAllowlist` | Tester profile allowlist is restricted | `subagent.ValidateProfile` / profile parse |
| `C41_002_WorktreeIsolationDefaultOn` | worktree isolation default-on | core/profile config |
| `C85_BuilderWorktreeIsolationHardError` | builder outside worktree is a HARD error | build phase preflight |
| `C54_010_TrustKernelCliIndependent` | trust kernel holds across any CLI | router / statemachine (CLI-agnostic) |
| `C49_002_FingerprintDeterminism` | task fingerprint is deterministic | research-cache fingerprint fn |
| `C47_ShipBacktickStripping` | ship strips backticks from commit msg | ship message normalization |
| `C47_SchemaFilterAdapterEnforcement` | schema-filter adapter is enforced | adapter/schema layer |
| `C48_003_AuditorStopCriterionHardGate` | auditor stop-criterion is a hard gate | audit phase (partially covered above) |
| `C55_020_PhaseRegistryExistsAndValidates` | phase registry validates | config/phase-registry loader |
| `C90_002_OrphanWorktreesPruned` | orphan worktrees are pruned | worktree GC |
| `C103_005_SubagentRunAllowlistIncludesBuildPlanner` | build-planner is in the dispatch allowlist | subagent-run allowlist |

## RETIRE-CRUFT — drop at Stage 5 (representative)

`*_PNew*` (roadmap items), `*_RoadmapEntries`, `*_RoadmapDone`, `*_DocExists`,
`*_DocMigrationNote`, `*_*OnDisk`, `*_Lessons*`, `*_InstinctSummary*`,
`*_IncidentDoc*` (presence-only), `*_*Snapshot` (per-cycle cost), `*_*RCA*`,
`*_KBUpdated`, `*_BinaryVersionIsV*`, `*_*Committed`, `*_Cycle*Cost*`,
`*_*InvestigationComplete`. These assert a one-time cycle artifact landed and
carry no ongoing invariant — they should be deleted with `go/acs/cycle*/` at
Stage 5, not ported.

## Pending e2e-tier ports (go/test/e2e/)

- **Native CLI-matrix full-cycle e2e** — `cmd/evolve/e2e_cycle_cli_matrix_test.go::TestE2ECycleCLIMatrix`
  is `t.Skip`'d as of Stage 5.1 (it fake-shipped via the removed `EVOLVE_NATIVE_SHIP=0` +
  `EVOLVE_SHIP_SCRIPT` hatch). Re-author in `go/test/e2e/` driving the **native** ship:
  seed a bare remote + a real auditor ledger binding (mirror `dispatch_test.go`'s
  `addRemote`/`seedAudit`) and resolve the worktree↔main `state.json` ff-merge divergence.
  Until then, native ship is covered by `phases/ship/native_test.go` + `dispatch_test.go`.
