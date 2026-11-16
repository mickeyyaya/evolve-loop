# 2026-11-14: a shadow feature was flipped to enforce, and its enforce path did not exist

The operator asked: "Scan the shadow features and review and enable them if it has been properly tested." Later the operator asked: "Turn the shadow features on if it passes the review and test in this wave".

Related documents: the package note [internal-integration.md](../architecture/packages/internal-integration.md), the design [integration-driver.md](../architecture/integration-driver.md) and the card [integration-driver-enforce.md](../architecture/integration-driver-enforce.md).

## Summary

- **Impact:** none reached main. The pull request was amended before the merge. Without the amendment, each lane ship of the next wave delivers a record that nothing lands.
- **Duration:** the flip was open on 2026-11-14, from the pull request to the amendment.
- **Root cause:** the review of the flip used test evidence from shadow. It did not read the code that runs at enforce. That code has no actor, and the package note said so.
- **Fix:** PR #1459 was amended (commit `94c7024a2`) to keep fleet integration at shadow. Only the curator went to enforce.
- **Guard today:** the driver refuses to start at enforce without an actor. `evolve integration status` prints "enforce: not ready (no enforce actor)".

## Terms used throughout

- **Shadow:** the stage where a feature runs and records, and does not change the result.
- **Enforce:** the stage where the feature decides the result.
- **The driver:** `integration.Driver`, which lands the delivered records.
- **An actor:** the object that does the work of the driver at one stage.
- **A deliverable record:** the file that ship writes at enforce instead of a push to main.

## Timeline

| Time (UTC) | Event | Evidence |
|---|---|---|
| 2026-11-13 and 2026-11-14 | The operator asks to enable the shadow features that passed review and test. | the two requests above |
| 2026-11-14 | PR #1459 sets `fleet.integration.stage` and `inbox.curator.stage` to `enforce`. Evidence: fleet integration composed 31 cycles with 0 ejects since cycle 2298 in shadow. | PR #1459 |
| 2026-11-14 | While the console checks ship latency, it reads the enforce path and finds no actor. | F1.1 to F1.4 |
| 2026-11-14 | A relaunch chain was about to merge #1459 at the boundary of wave 172. The console stops the chain. | console record |
| 2026-11-14 | The console amends #1459. Integration stays at shadow. The pull request lands with the curator only, in the batch at the wave 172 boundary. | commit `94c7024a2` |
| 2026-11-14 | The review of the enforce card finds 2 CRITICAL and 5 HIGH findings. The card needs a revision first. | card review of 2026-11-14 |

The fact sheet gives no clock time for these events.

## Root cause

**F1.1** `StageActors` registers an actor for shadow only (`go/internal/integration/driver.go:521`).
*Implication:* at enforce, the map of actors has no entry.

**F1.2** `Driver.actor` looks up the actor by stage. A missing entry gives the error "the stage ... has no actor, so the driver does not start" (`driver.go:130-140`). `Driver.Check` returns that error.
*Implication:* at enforce, the driver does not start.

**F1.3** At enforce, `deliver` writes the deliverable record and returns. The log line says "ship stops before" the main branch (`go/internal/phases/ship/deliverable.go:57-75`).
*Implication:* the record is the only result of a lane ship. Nothing else lands it.

**F1.4** The package note said it: "Enforce has no actor and is PR (c)" (`docs/architecture/packages/internal-integration.md:118`). The card for PR (c) is Proposed and has no code.
*Implication:* the information was in the repository. The review did not read it.

**F1.5** The inbox item `wave-integration-i14-driver-pr-c` is in `.evolve/inbox/consumed/`. A document cycle consumed it. That cycle wrote the card and no code.
*Implication:* the item looked done in the inbox, but the code was never built.

**F1.6** The evidence for the flip came from shadow: 31 composed cycles and 0 ejects.
*Implication:* shadow measures the compose engine. It does not run the code path that enforce adds.

Chain: F1.6 made the flip look safe. F1.5 made PR (c) look done. F1.4 was not read. So the flip reached a pull request with an enforce stage that F1.1 to F1.3 make unable to land work.

## What changed

| Change | Regression test or guard |
|---|---|
| PR #1459 amended: integration stays at shadow (commit `94c7024a2`) | the policy file, `.evolve/policy.json` |
| The driver refuses to start at enforce without an actor (`driver.go:130-140`) | the existing guard. The code `SHIP_INTEGRATION_DRIVER_FAILED` is pinned in `go/internal/integration/run_test.go:118`. |
| `evolve integration status` prints "enforce: not ready (no enforce actor)" | `go/cmd/evolve/cmd_integration_status_test.go:114` |
| A new console inbox item builds the enforce actor after its card passes review. The fact sheet names it `wave-integration-i14-driver-pr-c-build`. | not in the worktree inbox at this commit. The card review comes first. |

No new test was added for the flip itself. The fact sheet names none.

## What we learned

- Before a flip from shadow to enforce, read the code that runs at enforce and the "not built" lines of the package note. Test evidence from shadow does not cover the enforce path. This rule is recorded by the console. The fact sheet gives no file for it.
- An inbox item that a document cycle consumed is not proof of code. Check the code, not the inbox state.
- The guard that stopped the damage was the driver check at start. It shows the fault only after the merge. The check moved earlier by hand, through the console review of the pull request.
