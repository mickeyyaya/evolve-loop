# Inbox triage record, 2026-10-20

- **Status:** Done (2026-10-20). One triage pass over the pending inbox at `origin/main` `f35448242`.
- **Prior art:** [queue audit 2026-08-27](queue-audit-2026-08-27.md).
- **Items:** the 8 closed items are in `.evolve/inbox/consumed/`. Each item has a `consumed.resolution` stamp that holds its evidence.

## The request

The operator asked for a pass that "shrinks the console-only backlog (about 235 items) by closing what is stale, done or duplicated, with evidence for each, and fixes malformed items, without closing any live work". The rule for a doubtful item was "when unsure, KEEP".

## Result

| Measure | Before | After |
|---|---|---|
| Pending items (`.evolve/inbox/*.json`) | 244 | 238 |
| Status `console` (`evolve inbox list --status console`) | 234 | 229 |
| Status `ready` | 7 | 6 |
| Status `waiting` | 3 | 3 |
| Loader warnings (`evolve inbox lint`) | 36 | 0 |
| Warnings for an invalid `priority_class` (`evolve inbox rank`) | 2 | 0 |

The pass closed 8 items and filed 2 new items. Thus the pending count went down by 6.

| Class | Count | Action |
|---|---|---|
| DONE | 7 | `evolve inbox consume` with the evidence as the resolution |
| SUPERSEDED | 1 | `evolve inbox consume` with the cited decision |
| DUPLICATE | 0 | none |
| MALFORMED-FIXABLE | 38 | `evolve inbox edit` (the item stays pending) |
| KEEP | 236 | none (38 of them also had a field fix) |

## Method

1. A deterministic pass read all 244 pending items. It checked these facts:
   - each `files` path against `git ls-tree origin/main`;
   - duplicate ids and titles, and ids already in `consumed/`;
   - the item ids in the git log;
   - the warnings of `evolve inbox lint` and `evolve inbox rank`.
2. Four read-only agents classified 61 items each. Each agent read the item and the code at `origin/main`. A close needed a `path:line`, a test name or a commit.
3. The console re-read each close. An acceptance line that is a live measure (for example "the median realized width is at least 4") counts as not proven, so the item stays.
4. Two independent read-only agents checked the remaining 18 closes against the code. The pass did not close an item that a check disputed or found a gap in.

The deterministic pass found no proof to close an item. It found no duplicate id, no duplicate title, and no pending id in `consumed/`. The 12 `-console` and `-lane` pairs with the same title are intentional splits (`split_from`), not duplicates. A missing `files` path was not proof. 15 items name a path that is not on `origin/main`. Some of these paths are files that the item creates, and others are run evidence or files that moved.

## Closed items

| Id | Class | Evidence |
|---|---|---|
| `acs-run-names-vacuous` | DONE | PR #970 (`832f35791`). `acsassert.GoTests` refuses a pattern name that no test declares (`go/pkg/acsassert/go_tests.go:30-61`). `TestPredicateNamesResolveToTests` runs in CI (`.github/workflows/ci.yml:66`). |
| `scopedelta-wire-or-delete` | DONE | PR #977 (`935f822e4`). `core.ScopeDeltaFloorChecks` is in `productionBuildFloorChecks` (`go/cmd/evolve/cmd_cycle_config.go:44`). `TestScopeDeltaFloorChecks_ReportsEveryOutOfScopePathWithOneCodedSignal` flags a sibling lane's path. |
| `sandbox-dev-read-other-tty` | DONE | Commit `d95bedd19`. `devReadGrantWithoutTerminals` (`go/internal/adapters/sandbox/sandbox.go:161`). `TestAgentSandboxContract` denies a peer tty (`go/internal/bridge/sandbox_contract_integration_test.go:101-104`). |
| `triage-commits-disjoint-tasks-for-every-lane` | DONE | ADR-0139 O2c and O6 (#959, #963, #966, #975). The three named tests exist and pass: `TestSupply_RecordsTheCauseOfEachRefusal`, `TestSupply_ProposesASplitOfAMixedItemWhenTheSupplyIsLow` and `TestInboxSplit_TheConsolePartKeepsEveryProtectedFile`. |
| `cli-loop-zero-ship-breaker-console` | DONE | Commit `6aa8ea99c`. `go/cmd/evolve/cmd_loop_zeroship.go` runs each iteration (`cmd_loop_window.go:46`). The threshold comes from the policy, and the docs are at `operating-policy.md:209` and `runtime-reference.md:199`. `TestRunLoop_ZeroShip_TwoConsecutiveZeroShipWavesHaltBeforeTheThird` passes. |
| `tdd-comment-floor-then-enforce-console` | DONE | Commit `7b47cc927`. `core.TddCommentFloorFailures` (`go/internal/core/comment_floor.go:56`) is called at `go/internal/cli/phasecmd/phase_verify.go:126-147`. |
| `comments-strip-truncates-multiline-marker-lane` | DONE | PR #1012 (`168912a7d`). `TestStripComments_KeepsEveryLineOfAMultiLineAcsPredicateMarker` (`go/internal/commentaudit/strip_test.go:470`). The target test file has only its 3-line marker. |
| `landing-s1-readonly-redesign-console` | SUPERSEDED | Acceptance line 3 accepts a recorded drop. `docs/plans/fleet-landing-at-scale-2026-10.md:315` records "S1 stays dropped" with its two reasons (cycle 1960, `39f4c93e5`). |

## Items kept after a first-pass close

The four agents proposed 23 closes. The console kept 5 of them, because a live measure or a named line was not proven (the first 5 rows). The two checks then disputed 8 of the other 18 and found a gap in the fix of 2 more (the last 10 rows). The table records each gap, so that a later pass does not close the item for the same wrong reason.

| Id | Why it stays |
|---|---|
| `wave-runs-four-parallel-lanes` | Goal G1 (median realized width at least 4) is a live measure. Wave 130 ran 1 lane of 4 (PR #1013). |
| `rolling-pool-backfills-lanes` | Goal G3 (utilization at least 0.85) is a live measure, and the named lease test has a new name. |
| `wire-partitiongraph-for-package-level-conflicts` | The 3-run shadow cost measure is not recorded. |
| `lane-test-floors-oversubscribe-the-host` | The live load-average line is not measured. |
| `bridge-test-driver-lost-after-registry-reset` | Some registry resets use `defer`, but the acceptance asks for `t.Cleanup`. Nobody ran the 5-seed shuffle line. |
| `wave-orchestrator-oversees-every-lane` | `deadline_stops` is not in `wavemeasure.Measures`, and `Oversight.Run` has no production caller. |
| `cli-floor-run-console` | `cmd_floor.go:47-56` has its own commands and does not use the `ciparitygate` runners, as the fix asks. |
| `cli-cycle-reaudit-console` | The recovery chain does not call `core.ReauditCycle`. `runtime-reference.md` has no entry for the verb. |
| `cli-routing-plan-console` | The `runtime-reference.md` entry that the fix asks for is missing. |
| `cli-ship-pr-console` | `runtime-reference.md` does not document `--pr`. |
| `cli-rebuild-console` | The chain-boundary refresh still uses its own `defaultChainRebuild` (`cmd_loop_chain.go:60`). |
| `go-test-env-projection-pinned-console` | `phase_bindings_selfcheck.go:90,101` and `build_floor_reviewer.go:323` still use `ipcenv.Scrub`. The AST pin does not see test argvs that a helper builds. |
| `history-index-merge-conflict-console` | Only `merge=union` landed (`.gitattributes:27`). A union merge can keep two rows with different counts. |
| `sizeratchet-allowances-not-a-shared-registry` | The boundary does not run the tighten step (`docs/operations/runtime-reference.md:42`). |
| `resume-does-not-refresh-a-stale-worktree-base` | `GIT_FF_MERGE_DIVERGED` still shows when the recovery budget is spent. The scout-checkpoint step was not built. |

Each of these items is near done. A console session can narrow its acceptance to the gap above, or close the gap.

## Field fixes

All fixes used `evolve inbox edit`, which accepts `title`, `acceptance` and `priority_class` as curation fields. No item needed a hand edit. A check compared each file with `origin/main` with `jq -S`. Only the fields in this table changed.

| Fix | Items | Rule |
|---|---|---|
| A title shortened to 160 bytes or fewer | 33 | `maxFieldLen = 160` (`go/internal/inboxbatch/item.go:217`). 3 titles had 160 characters or fewer but more than 160 bytes. |
| The filed title put at the start of `summary` | `state-growth-caps`, `auditor-tier-cascade`, `capability-aware-prompt-scaffold` | The short title of these 3 items lost details that the summary did not have. |
| An acceptance entry split at a sentence boundary | 5 | `maxAcceptanceLen = 600` (`go/internal/inboxbatch/item.go:219`). The words did not change. A "; (b)" joint became ". (b)". |
| `priority_class` `pipeline` changed to `stability` | `codex-live-probe-submit-wedged-and-no-usage-windows` | The class must be one of the 8 classes that `evolve inbox rank` names. |
| `priority_class` `capacity` changed to `performance` | `deep-tier-shares-argon-by-budget` | Same rule. |

`evolve inbox edit` writes the item as one JSON line. Thus the diff of an edited item is large, but its content is the same. The pending item `inbox-item-writer-keeps-the-filed-format-console` tracks this writer behavior.

## Filed items

| Id | Source | Route |
|---|---|---|
| `wave-deadline-stop-seals-a-labelled-outcome` | The out-of-scope note of PR #1013. The loop must seal "stopped: lane deadline" for a cycle that `LOOP_ORCH_LANE_DEADLINE` stopped. `wave watch` and `wave next` must apply the stop. | `console-manual` |
| `reintent-fix-type-routes-the-item` | PR #1015. A cycle that a defer-or-quarantine decision ends must route its item by `fix_type`. Today `fix_type` goes only into the routing evidence (`go/internal/core/reintent_decision.go:43`). | `console-manual` |

## Limits

- The pass did not close an item that waits for ADR-0147. That ADR has the status Proposed (2026-10-20), so it does not yet supersede the per-lane landing items.
- The five 2026-08-27 sweep items each hold many prescriptions. The pass did not check each prescription, so the sweeps stay.
- A live wave held claims on `flags-and-predicate-tests-hermetic-env-lane` and `retro-minting-files-through-inbox-add-lane` (`evolve inbox claims`). The pass did not change these items.
