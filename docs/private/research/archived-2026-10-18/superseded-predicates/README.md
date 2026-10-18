# Retired ACS predicates: tests that never existed or that changed their contract (2026-10-18)

> Archived by the console lane for the inbox item `acs-run-names-vacuous` ([the spec section](../../../../architecture/acs-predicate-quality-gate.md#predicate-test-names-resolve-acsnames-2026-10-18), component A1 of [logic-first-delivery-design.md](../../../../architecture/logic-first-delivery-design.md)). The new check `go/acs/regression/acsnames` fails on a predicate that names a test that no test declares. The predicates below named such tests, and no test in the module can satisfy them. They are archived here under the A2 retention rule (never deleted, never silently edited; precedent: [the 2026-10-07 archive](../../archived-2026-10-07/superseded-predicates/README.md)). The rest of each package stays live in `go/acs/`.

## What was archived and why

| Predicate | It asserted | Why it is retired | What pins the contract now |
|---|---|---|---|
| `TestC766_005_goal_hash_mismatch_aborts_before_triage` | A scout report whose `goal_hash` differs from the lane pin aborts the cycle before triage. | Commit c37dc324a made the kernel stamp the pinned value, and the cycle proceeds. No abort remains. | `internal/core` `TestLaneScopePin_ScoutGoalHashMismatchNormalizesAndProceeds` |
| `TestC776_009_goal_hash_mismatch_gate_regression` | The same test, run by name as a regression. | The same change. | The same test. |
| `TestC566_002_EffortDefaultMatrix` | The profiles pin a per-phase effort matrix: scout and triage low, tdd, audit, adversarial and builder medium. | Commit 0936a73c2 made every effort start at medium. The tier and agent effort live in the `cli_routing` policy. | `internal/profiles` `TestEveryTrackedProfileLoadsWithoutAnEffortField` and `TestLoader_RefusesTheRetiredEffortFields` |
| `TestC786_001_trivial_rebase_carries_audit_forward` | A clean trivial rebase with an unchanged patch id and green composed-tree gates carries the audit forward with no new audit. | Commit 09d32a190 (#535) restored the audit authority. A composed tree that differs from the audited tree now requires a predicate re-audit. | `internal/phases/ship` `TestTrivialRebase_ChangedTreeRequiresPredicateReaudit`, and the live `TestC786_002` and `TestC786_003` |
| `TestC1580_008_ScoutReportChallengeTokenIsEnforcedNatively` | The scout report must echo the challenge token at the pre-audit contract gate. Five tests in `internal/deliverable` pin it. | None of the five tests ever existed. Commit e6961d0ab salvaged the predicate without them, so it passed without a check. Only the build phase requires the token. | `internal/deliverable` `TestContract_RequireChallengeToken_BuildOnlyAtV1` pins that the scout is not token-required. No test pins a scout-side echo. |

## Two packages left the module whole

| Package | It asserted | Why it is retired | What pins the contract now |
|---|---|---|---|
| `go/acs/cycle1222` (four predicates) | Two guard tests exist in `internal/acssuite` and pass: `TestCurrentCycleScopeReachable` and `TestGoLanePatterns_CycleNumberDrift`. | Neither test was ever added. `git log -S` finds no commit that defines either name. Predicate 003 was a sentinel that proved the harness of the other three. | Nothing pins the two behaviors. `TestGoLane_CurrentCycleScope` and `TestGoLanePatterns_CycleAndRegressionPresent` pin the selection of a present cycle package only. |
| `go/acs/cycle1241` (five predicates) | The same two tests, and that the predicates of cycle 1222 pass unmodified. | The same, and the cycle 1222 predicates are archived. | The same. |

The package of cycle 1241 held a check on the Git state of the cycle 1222 predicate file. That check goes with the packages.

## Notes

- The archived files keep each predicate's original package clause and body, so they read as they ran. They live outside the Go module and never build.
- `go/acs/cycle776` lost its `corePkg` constant and `go/acs/cycle1580` lost its `deliverablePkg` constant. Each constant had no other user. Every removed line is in an archived file, so the move is pure.
- `go/acs/regression/acsnames` reads `go/acs` only. It does not read this archive, which holds the stale names.
- `TestC786_004_eval_files_pass_quality_check` stays live. It fails in a dev worktree because `.evolve/evals/merge-rung0-*.md` is absent. That failure predates this change.
