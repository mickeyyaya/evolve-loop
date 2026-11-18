# 2026-11-18: a proven protected-path blocker still got two repair rounds

## Summary

- **Impact:** in wave 180, cycle 2494 used 46.4 LLM minutes and shipped nothing. 33.8 of these minutes came after its fail point at 12.7 minutes (`evolve wave status`: waste 33.8 min, fail-fast 0.27).
- **Duration:** 05:31 to 06:32 UTC. The cycle ran 3 build rounds and 3 audit rounds.
- **Root cause:** two causes together. A test-cleanup flake in a repo-wide floor test stopped the host from routing a blocker that it had proved. After that, the fixability stop saw that no repair round can fix the cause. The stop runs in shadow, so the cycle got two more repair rounds.
- **Fix:** two items are open. One judges the shadow record and then sets the fixability stop to enforce. One finds and removes the cleanup flake.
- **Lesson:** a stop that is correct in shadow saves nothing. Each would_stop that is correct is evidence for the flip, so collect that evidence on purpose.

## Terms used throughout

- **Protected-path blocker:** a build verdict that says that the only fix changes a protected file, which a lane cannot change.
- **Proven blocker:** a blocker that the host checks and accepts. A proven blocker ends the cycle and routes the item to the console.
- **Fixability stop:** the check of [ADR-0158](../architecture/adr/0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md) that runs before a repair round. At enforce, it seals the cycle as handed-off when no repair round can fix the finding.

## Timeline

| Time (UTC, 2026-11-18) | Event | Evidence |
|---|---|---|
| 05:31 | Cycle 2494 starts on the item `latency-wave-breach-from-wave-158`. | signal seq 1 |
| 05:32 to 05:41 | Scout routes the item as lane work. TDD measures the cause: the serial scope loop at `go/internal/acssuite/golane.go:43-45`. | signal seq 64, 135; `scout-report.md:37` |
| 05:46 | Build round 1 declares a protected-path blocker. The host proves it, but refuses to route it, because one floor failure is not explained: `guards.TestDocDelete_EverythingElseUnderTheDocRootsStaysProtected`, `TempDir RemoveAll cleanup: unlinkat`. | signal seq 169 `ORCHESTRATOR_BLOCKER_REFUSED`; `build-interactions.ndjson`, kind salvage |
| 05:50 to 05:51 | Audit round 1 fails with H1: the acceptance is not delivered, and the cause is in a protected file. | `audit-report.round1.md` |
| 05:51 | The fixability stop says would_stop. The stage is shadow, so the policy grants a repair round. | signal seq 233 `REPAIR_HANDOFF`, seq 235 `ORCHESTRATOR_AUDIT_REPAIR_GRANTED` |
| 05:52 to 06:22 | Repair round 1: TDD, build, audit. Audit fails again with the same H1. | signal seq 238, 297, 358; `audit-report.round2.md` |
| 06:22 | The fixability stop says would_stop again. The policy grants a second repair round. | signal seq 385, 387 |
| 06:22 to 06:30 | Repair round 2: TDD, build, audit. Audit fails with the same H1. | signal seq 390, 450, 507; `audit-report.md` |
| 06:30 | The retry budget is spent. The cycle spend limit says would_stop (shadow). | signal seq 535 `ORCHESTRATOR_AUDIT_REPAIR_DECLINED`, seq 536 `CYCLE_SPEND_LIMIT` |
| 06:32 | The cycle seals FAIL. The retrospective splits the item into 2 console items. | signal seq 546; `latency-wave-breach-from-wave-158-ac-1-console`, `-ac-2-console` |

## Root cause

**F1.1** Build round 1 declared a protected-path blocker on `go/internal/acssuite/golane.go:43`. The host proved it (`cycleRun.provenBlocker`, `go/internal/core/build_blocker_exit.go:120`).
*Implication:* the host had the correct answer after 15 minutes.

**F1.2** A proven blocker ends the cycle only when it explains every floor failure. The refusal of signal seq 169 says: "the blocker on go/internal/acssuite/golane.go is proven, and these failures are not explained". The one failure was a cleanup flake in an unrelated test: `TempDir RemoveAll cleanup: unlinkat`. The same test passes on main (`go test -count=1`, 0.4 s).
*Implication:* a flake in a repo-wide floor test can keep a correct route from happening.

**F1.3** After each audit FAIL, the fixability stop (`go/internal/core/repair_handoff.go:52`) found the hand-off finding H1 and answered would_stop. The key `workflow.repair.fixability.stage` is not in `.evolve/policy.json`, so the stage is shadow, and `decideAfterAuditFail` granted the repair round.
*Implication:* the stop saw the right answer twice and changed nothing.

**F1.4** Since cycle 2400, the shadow record holds would_stop answers from 1 cycle only: cycle 2494, 2 answers, both correct.
*Implication:* the record is too small to flip the stage now. It must grow first.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| Judge the shadow record of the fixability stop, then set it to enforce. The test replays the failure blocks of cycle 2494 and expects a hand-off after audit round 1. | open: `repair-fixability-stop-flip-to-enforce-after-cycle-2494` | open, in the item |
| Find the writer that outlives the docdelete guard test, and fix the fixture. | open: `guards-docdelete-test-tempdir-cleanup-flake` | open: a stress run of `TestDocDelete_` |
| The lane recorded the cause and the regression tests as console follow-ups. | none (retrospective split) | `.evolve/runs/cycle-2494/console-followup/` |

## What we learned

| Lesson | Rule or guard it became |
|---|---|
| A correct stop in shadow saves no minutes. When a shadow stop is correct, record it as flip evidence at once. (F1.3, F1.4) | The item `repair-fixability-stop-flip-to-enforce-after-cycle-2494`: a recomputable verdict over the shadow record, then the flip with the cycle 2494 replay test. |
| A flake in a repo-wide floor test costs more than one red run. It can also block a correct route. (F1.2) | The item `guards-docdelete-test-tempdir-cleanup-flake`: a stress run of `TestDocDelete_` that is red before the fix. |
| A diagnosis item can find its cause on a protected file only in the build. Scout cannot know this before the build. (F1.1, F1.3) | The fixability stop at enforce is the guard for this case. It is the same item as the first lesson. |
