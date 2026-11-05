# 2026-11-05: Wave 155 gave false signals, a logout read as a quota wall, and seven fixes were blocked

> Design set: [research](../research/pipeline-blockers-2026-11.md) · [plan](../plans/pipeline-blockers-2026-11.md) · [ADR-0161](../architecture/adr/0161-a-pipeline-blocker-fix-meets-six-test-rows-and-a-signal-states-its-cause-and-next-step.md) · [spec](../architecture/signal-accuracy.md).

## Summary

- **Impact:** Wave 155 landed two ships (`c9a7ef419`, `09bb331d4`), and `evolve wave status` printed "0 of 11 cycles shipped". Near 01:00 the CLI logged out, and the loop stopped with `quota-pause` at 56 percent usage. The link to each of cycles 2165 to 2171 is not proven. Seven fixes were blocked by review, four of them twice.
- **Duration:** from 2026-11-04 22:23 to 2026-11-05 07:10 (UTC+8), about nine hours, of which about six hours had no loop work.
- **Root cause:** the pipeline states causes and counts that it derives from text or from a partial view. Examples: a class from words, a count from one of two readers, and no class for a logout.
- **Fix:** none has landed except B1 (`bb69b6092`, PR #1283). This set holds the design, the test plan and the rules for the rest.
- **Safety:** the commit gate refused each empty review, so nothing landed unreviewed (inbox item `logout-is-reported-as-a-quota-wall`).

## Timeline

All times are UTC+8. The ledger and the signals use UTC, and I converted them. The two items marked "brief" come from the report of the console and not from a file.

| Time | Event | Evidence |
|---|---|---|
| 2026-11-04 22:23:41 | The census classes the item of cycle 2161 as `console_route` | `waves/supply.json`, cycle 2161 retrospective |
| 22:23:42 | `LOOP_ORCH_LANE_LAUNCHED` starts that item through a paused scope | cycle 2161 retrospective, D1 |
| 22:23:42 | Wave 155 starts from run `20261104T141300Z` | `.evolve/waves/wave-155.json` |
| 23:27:21 | Cycle 2159 ships as `c9a7ef419` | `.evolve/ledger.jsonl` |
| 23:27:29 | The digest of cycle 2157 records the class `infra-error` | `failure-digest.json`, `class_recorded_at` |
| 2026-11-05 00:47:30 | Cycle 2158 lands as `09bb331d4` | `git show 09bb331d4` |
| about 01:00 | The claude CLI is logged out. Lanes report `quota-pause` at 56 percent weekly usage (brief) | inbox item `logout-is-reported-as-a-quota-wall` |
| 01:16 | Last write to the loop log of wave 155 (1,511 `ISOLATION_I2_VIOLATION` lines) | `.evolve/logs/20261104T141300Z/loop.log` |
| about 07:10 | The operator runs /login (brief) | inbox item, created 07:12:03 |
| 08:17:40 | The inbox item for the train attestation is filed | `train-record-pr-has-no-reachable-attestation` |

## Root cause

The wave had eleven separate faults, B1 to B11. They share three causes in the chain.

1. **A fact was derived from a partial view.**
   - `pausedPlan.order` added a paused scope with no route check (B1). The triage gate read the lane pin and not the route (B2).
   - The resume path read `review.ProvenBlocker` and did not run the prover (B3). The ship check read the main checkout (B4).
   - The halt counted cycles above the batch floor (B5).
2. **A class or a cause was derived from text or guessed.**
   - A test name with `quota` gave `infra-error` to a gate block (B8).
   - A routine filter wrote a WARN with `action=refused` (B9).
   - The code has no class for a logout. If this made it read as quota is not traced (B10, plan Q4).
3. **A signal was not tested as a whole.**
   - Tests pinned fragments, so a missing cause or a failing next step passed review (research F1.15, F1.16).
   - The fixes were built before a design review, so the architecture review found design faults after the build (research F1.14).

The false count has two readers. Cycle 2159 resumed, so its number was below the cycle floor of 2160. `wave status` reads only cycles above that floor (`cmd_wave_status.go:102`), so it printed zero for two ships. `zeroShipHalt` starts at the batch start in the same way (`cmd_loop_zeroship.go:19`), and a probe showed a false halt.

The observed order for the logout: the CLI was logged out near 01:00. The loop then stopped with `quota-pause` at 56 percent usage. The link between the two is not traced, and plan Q4 holds the question.

## What changed

| Fix | Pull request | Regression test | State |
|---|---|---|---|
| B1: a paused scope of a console-routed item does not resume | #1283 (`bb69b6092`) | `TestProductionPoolPlanFn_APausedScopeOfAConsoleRoutedItemLeadsNoLane` | Committed on the PR |
| B2, B3, B4, B5, B6, B8, B9 | Not opened | Staged in worktrees; blocked by the architecture review | Waits for the design check |
| B7: `sync-main` and a quarantine move | Cycle 2153 | Not checked | Waits for the cycle |
| B10: a logout is reported as a logout | None | None | Inbox item filed |
| B11: the train accepts a reviewed PR | None | None | Inbox item filed; design first |
| The design set of this incident | Not opened | `evolve docs ste-lint --strict` | This set |

## What we learned

| Lesson | Rule or guard |
|---|---|
| A fix that is built before its design review costs a second build. | Decision 1 of ADR-0161: a design card before any test. |
| A signal that names a wrong cause sends the operator to the wrong place. | Decisions 4 and 5: six parts, and "unknown" when the cause is not known. |
| A class from words in text breaks on a test name. | Decision 6: the writer sets the class. |
| Two readers of one fact disagree. | Decision 7: one reader for each fact. |
| A test through one entry leaves the other entries open. | Decision 3: one wiring test for each entry. |
| A task with more than three blocks needs a new approach. | Decisions 9 and 10: a redesign at the top tier. |
| A logout and a quota wall need different steps. | Decision 11: a logout is its own cause. |
| A scratch script has no test. | Decisions 12 and 13: tested read-only verbs. |

The rules are in [operating-policy §1](../operations/operating-policy.md#1-pipeline-integrity-policy-highest-severity).
