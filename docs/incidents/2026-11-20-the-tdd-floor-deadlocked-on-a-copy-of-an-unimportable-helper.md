# 2026-11-20: the TDD floor deadlocked on a copy of an unimportable helper

Related documents: the floor rules are in `docs/architecture/test-system.md` (§6.4 and §6.5). The procedure is the [issue-record](../../skills/issue-record/SKILL.md) skill.

## Summary

- **Impact:** cycle 2567 failed in the TDD phase. Its 4 predicates were red by assertion and correct, but the handoff floor refused the deliverable twice. The failure class `tdd|gate-block` has now recurred 5 times, in cycles 2349, 2365, 2408, 2532 and 2567.
- **Root cause:** the TDD agent wrote a helper (`finishedPID`) in `go/acs/cycle2567/predicates_test.go`. Its body equals a helper in a `_test.go` file of another package. The floor named that file and said "inline it". A test helper of another package cannot be imported. The agent renamed the helper, and a renamed helper is still a copy.
- **Fix:** PR #1601 moves the exited-process helper to `acsassert.ExitedPID` and makes the floor name the kit helper for a copy. It then fixed the CI of the PR (see [the CI record](2026-11-20-four-console-prs-failed-pr-ci-after-green-local-runs.md)).
- **Lesson:** a floor line that names a source the agent cannot use makes a deadlock. A floor line names a helper that the agent can call, or says to write no helper.

## Terms used throughout

- **Handoff floor:** the deterministic checks that a phase runs before its handoff. A refusal gets a correction round.
- **Predicate kit:** `go/pkg/acsassert/predicate_kit.go`. It holds helpers that an ACS predicate can import.
- **Helper copy:** a function whose normalized body equals the body of another function.

## Timeline

| Time (UTC, 2026-11-20) | Event | Evidence |
|---|---|---|
| 11:53 | The failure class is recorded with recurrence 5. | `class_recorded_at` in `.evolve/runs/cycle-2567/failure-digest.json` |
| 11:55 | The console files the item `tdd-floor-helper-copy-deadlock`. | the item file name |
| about 15:21 | Two seats pass the review of the fix for PR #1601. | commit body of `c1fc68cad` |

The floor text after the correction round (`audit-fail-reason.json` of cycle 2567):

```text
acs/cycle2567/predicates_test.go:finishedPID = internal/ledgerartifacts/store_test.go:exitedPID
 — a test helper in another package cannot be imported: inline it
```

The first refusal named the `exitedPID` of `internal/cli/guardcmd`. The agent swapped one copy for another.

## Root cause

**F1.1** The helper-copy floor ran only in the host reviewer, and its "inline it" remedy was ambiguous. The disposition of cycle 2567 states this (`.evolve/runs/cycle-2567/disposition.json`).
*Implication:* the agent had no safe next step. The two corrections ended with a copy.

**F1.2** The exited-process helper already had 4 baselined copies in the tree (the item `tdd-floor-helper-copy-deadlock`). PR #1601 replaced the copies in `acs/cycle1826`, `gittest`, `ledgerartifacts`, `guardcmd` and `observer` with a call to the kit.
*Implication:* each new predicate that needed it copied a body that the floor already knew.

**F1.3** The disposition of cycle 2567 counts the class as the 8th recurrence of the helper-copy class. The failure digest counts 5 for the class fingerprint. This record uses the digest. The two counts are not reconciled.

## What changed

| Change | Pull request | Regression test |
|---|---|---|
| `pkg/acsassert.ExitedPID` is the one home of the helper. The floor names the kit helper for a copy of its body, or says to define no helper. | #1601 (commit `7eb7fa0f5`) | `core/tdd_predicate_floor_test.go::TestTddPredicateFloor_ACopyOfAnUnimportableHelperNamesTheKitHelperWithTheSameBody` and `::TestTddPredicateFloor_ACopyOfAnUnimportableHelperWithNoKitTwinSaysToDefineNoHelper` |
| `ExitedPID` starts its process through `sysexec.Command` | #1601 (commit `c1fc68cad`) | `acsassert::TestExitedPID_ReturnsTheIDOfAProcessThatHasExited` |
| The floor runs inside the phase verify, with an unambiguous remedy | inbox item `tdd-floor-in-phase-verify-and-unambiguous-inline-remedy` | proposed in the disposition; not verified as filed |

## Limits

- PR #1601 landed through batch PR #1604 at main 30915d28c.
- The fix covers helpers whose body the kit holds. Another helper gets the "define no helper" text.

## What we learned

- A refusal text must name a next step that the agent can do.
- Count a recurrence with one source. Two counts for one class make the severity unclear.
