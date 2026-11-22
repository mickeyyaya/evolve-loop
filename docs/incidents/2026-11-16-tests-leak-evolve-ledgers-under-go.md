# 2026-11-16: tests left .evolve ledgers under go/, and the composed-tree gate of each lane failed

The record of the first use of the [issue-record](../../skills/issue-record/SKILL.md) skill. The cause came from [issue-analyze](../../skills/issue-analyze/SKILL.md). The plan: [issue-record-skill-2026-11.md](../plans/issue-record-skill-2026-11.md). The package note of the fix: [internal-inboxmover.md](../architecture/packages/internal-inboxmover.md).

## Summary

- **Impact:** the composed-tree `make test` of the lanes failed with `LEAK` paths under `go/`. Cycle 2419 looped between ship and re-audit for about 3 hours (23:56Z on 2026-11-15 to 03:00Z on 2026-11-16). CI on main stayed green.
- **Root cause:** `inboxmover` built its ledger path from `ProjectRoot`. Tests that named only `InboxDir` left `ProjectRoot` empty, so the ledger went to `.evolve/` under the package directory of the test.
- **Escape:** CI runs `make test-integration`, which has no treewalk guard. Only `make test` has it, and only the lanes run `make test` on the composed tree.
- **Fix:** PR #1490 derives `ProjectRoot` from an `InboxDir` of the form `<root>/.evolve/inbox`, and gives the three tests a root.
- **Open:** the inbox item `ci-test-integration-treewalk-parity` adds the treewalk guard to the CI target. The plan [every-inbox-mover-names-its-project-root](../plans/every-inbox-mover-names-its-project-root-2026-11.md) fixes the mover half, and that item keeps the CI wrap.

## Terms used throughout

- **Treewalk guard:** `evolve treewalk snapshot` and `evolve treewalk check` around the tests of `make test` (`go/Makefile:107-111`). It fails the run on each path that the tests leave under `go/`.
- **Composed-tree gate:** the check of the orchestrator that runs the make gates on the tree of a lane composed with main (`Orchestrator.identityCarryForward`).
- **The plane ledger:** `<root>/.evolve/ledger.jsonl`, the hash-chained ledger of the project.

## Timeline

| Time (UTC) | Event | Evidence |
|---|---|---|
| 2026-11-15 23:56:30 | The first ship of cycle 2419 stops: main moved. | `SHIP_GIT_FLEET_REBASE_NEEDED`, `.evolve/runs/cycle-2419/signals.ndjson` |
| 2026-11-16 00:50:27 | The composed-tree gate declines: `make test` fails with 4 `LEAK` paths. The ship aborts and goes back to audit (try 1 of 9). | `ORCHESTRATOR_COMPOSED_GATE_DECLINED` (WARN), `ORCHESTRATOR_PHASE_ABORTED` |
| 01:39:47 | The second ship stops: main moved again. | `SHIP_GIT_FLEET_REBASE_NEEDED` |
| 01:51:21 | The console files the class fix as an inbox item. | `.evolve/inbox/2026-11-16T01-51-21Z-ci-test-integration-treewalk-parity.json` |
| 02:37:28 | The ship aborts again (try 2 of 9). | `ORCHESTRATOR_PHASE_ABORTED` |
| 03:00:38 | Cycle 2419 delivers its record to the integration train. | `SHIP_INTEGRATION_DELIVERED` |
| 03:03:47 | PR #1490 lands on main. | commit `7a3537655` |

The key log, from the field `tail_test` of the 00:50:27 signal (trimmed):

```text
ok   github.com/mickeyyaya/evolve-loop/go/test/trustkernel 1.579s coverage: [no statements]
LEAK internal/phases/triage/.evolve
LEAK internal/phases/triage/.evolve/ledger.jsonl
LEAK internal/phases/triage/.evolve/ledger.lock
LEAK internal/phases/triage/.evolve/ledger.tip
evolve treewalk check: 4 path(s) appeared under . while the tests ran
exit status 1
make: *** [test] Error 1
```

## Root cause

**Symptom.** The treewalk guard of `make test` found four new paths under `go/internal/phases/triage/.evolve` (signal of 00:50:27).

**Chain.**

1. `evolve treewalk check` found new files under `go/`.
2. A test wrote `.evolve/ledger.jsonl` in its package directory. `go test` runs each test in its package directory.
3. `Options.resolveOpts` built `LedgerPath` as `filepath.Join(o.ProjectRoot, ".evolve", "ledger.jsonl")`. With an empty `ProjectRoot`, that path is relative (`go/internal/inboxmover/inboxmover.go`, before commit `7a3537655`).
4. `resolveOpts` filled `InboxDir` from `ProjectRoot`, but it never filled `ProjectRoot` from `InboxDir`. A caller that named only the inbox thus had a correct inbox and a wrong ledger.

**Contributing causes.**

- Three test files named only the inbox or passed a nil ledger: `go/internal/phases/triage/host_chain_test.go`, `go/cmd/evolve/cmd_cycle_host_effects_test.go` and `go/cmd/evolve/cmd_loop_pool_paused_hold_test.go`. Two tests in `go/internal/core` had no `ProjectRoot` for the same reason. PR #1490 changes all five.
- Each lane composes its change with main and runs `make test` on the result. So each lane met the leak of main, whatever its own change.

**Escape.** CI runs `make test-integration` (`.github/workflows/go.yml:51`). That target has no treewalk guard (`go/Makefile:127`). So main stayed green, and no signal on main showed the leak. The lanes were the first control that saw it, and a lane cannot fix main.

This record does not name the commit that first made the claim write a ledger row. The trace stopped at the resolution rule, which is the cause that the fix removes.

## What changed

| Change | Regression test (red without the change) |
|---|---|
| PR #1490: `planeRootOf` in `go/internal/inboxmover/inboxmover.go` derives `ProjectRoot` from `<root>/.evolve/inbox`; another `InboxDir` keeps the empty root | `go/internal/inboxmover/resolve_root_test.go`: `TestResolveOpts_AnInboxUnderAPlaneNamesItsProjectRoot`, `TestPlaneRootOf_OnlyThePlaneInboxLayoutNamesARoot`, `TestResolveOpts_AnInboxOutsideAPlaneKeepsTheGivenRoot`, `TestClaimPending_AnInboxOnlyCallerWritesTheClaimRowToThePlaneLedger` |
| PR #1490: the five tests get a root, and two assert that the claim row lands in the root ledger | `TestNewHostEffects_AProtectedFilesItemIsRoutedAndAClaimableItemIsClaimed`, `TestHostChain_AnUnroutedPinnedItemTheHostClaimsPassesTheGate` |
| Open: the treewalk guard in `make test-integration`, with an allow list of the outputs of the target | inbox item `ci-test-integration-treewalk-parity`; its acceptance asks for a test that plants a leaked `.evolve` directory and proves that the CI target fails |

## What we learned

- A path that a caller can leave empty must not decide where a write goes. Derive it from a path that the caller did give, or refuse. The rule is now in the package note of `inboxmover`.
- Two targets that run the same tests must run the same guards. When the lanes have a guard that CI does not have, main stays green while each lane fails. The class fix is the inbox item `ci-test-integration-treewalk-parity`.
- A WARN signal can cost hours. The rule [issue-gets-a-record](../../guidance/rules/issue-gets-a-record.md) and the skill `issue-record` now start on WARN as well as on INCIDENT.
