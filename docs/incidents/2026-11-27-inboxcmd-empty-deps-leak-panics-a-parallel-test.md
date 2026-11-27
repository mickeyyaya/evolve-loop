---
schema: incident-record/1
id: 2026-11-27-inboxcmd-empty-deps-leak-panics-a-parallel-test
title: "An empty Deps that a test left in the inboxcmd package host panicked a parallel test, and a re-run hid it"
level: record
status: fixed
selfheal: "inc-2026-10-27-6f89f94c"
fingerprint: ""
signals: []
prs: [1849]
items: [inboxcmd-tests-never-leak-an-empty-deps-into-the-package-host, repair-inc-2026-10-27-6f89f94c]
actions: []
effectiveness:
  metric: {code: "", filters: [], unit: "", since: ""}
  baseline: {value: 0, units: 0}
  target: 0
  checks: 0
  results: []
verified_at: ""
---

# An empty Deps that a test left in the inboxcmd package host panicked a parallel test, and a re-run hid it

## Summary

<!-- GENERATED:status BEGIN -->
- **Status:** fixed
<!-- GENERATED:status END -->

## Trigger and level

- **Trigger:** on 2026-11-27 the boundary batch PR #1849 (PRs 1845 to 1848) was red on try 1 of run 45942606193, in the job `go / build + test (Go 1.27, ubuntu, every package, no gates)`. The batch verb re-ran the failed job, and try 2 of the same commit was green. The batch landed.
- **Impact:** no wrong code landed. The batch took one extra CI run. The self-heal monitor opened the flaky-test incident `inc-2026-10-27-6f89f94c` with the generic subject `rerun-green` and no test name.
- **Level:** record. The red reached CI and the batch, and the cause was a defect in the tests, not an outside fault.

## Timeline

| Time (2026-11-27, UTC) | Event | Evidence |
|---|---|---|
| 05:00 | Try 1 of run 45942606193 panics in `TestRunInboxLint_TextNamesTheCitedProtectedBasenamesAndFailsOnEachLoadOrWriteError`. | the failed log of try 1 |
| about 05:10 | The batch verb re-runs the failed job. Try 2 is green, and PRs 1845 to 1848 land through #1849 (main `c2b5eb608`). | `evolve integration batch` output: "the first run was red and the rerun was green (a flake)" |
| 05:35 | The console reads the failed log, finds the one real panic among nested fixture output, and traces it to the package host. | `go/internal/cli/inboxcmd/cmd_inbox.go:137` at `c2b5eb608` |
| 05:50 | The console files the lane item `inboxcmd-tests-never-leak-an-empty-deps-into-the-package-host` (rank 2 of 73) and adds it to wave 192. | `evolve inbox add` output |
| 05:52 | The self-heal step of the boundary routes `repair-inc-2026-10-27-6f89f94c` to the console: rerun-green events share one fingerprint with no test name. | `INBOX_ITEM_FILED_CONSOLE` |
| wave 192 | Cycle 2662 ships the fix as commit `ae3a35b30`. | `git show --stat ae3a35b30` |

## Key logs

```text
--- FAIL: TestRunInboxLint_TextNamesTheCitedProtectedBasenamesAndFailsOnEachLoadOrWriteError (0.01s)
panic: runtime error: invalid memory address or nil pointer dereference [recovered, repanicked]
github.com/mickeyyaya/evolve-loop/go/internal/cli/inboxcmd.loadPendingInbox(...)
	go/internal/cli/inboxcmd/cmd_inbox.go:137 +0x452
github.com/mickeyyaya/evolve-loop/go/internal/cli/inboxcmd.runInboxLint(...)
	go/internal/cli/inboxcmd/cmd_inbox.go:257 +0x2b3
FAIL	github.com/mickeyyaya/evolve-loop/go/internal/cli/inboxcmd	5.153s
```

Try 1 has other `--- FAIL` lines, for example `TestRatchet_NoNewRawGitFixtures`, `acs/cycle10` and `acs/cycle1950`. Tests that run the build floor against fixtures write these lines on purpose. They are not failures of this run.

## Root cause and contributing factors

This section describes the code at `c2b5eb608`, before the fix.

- **Root cause:** `Run` stored its `Deps` in the package global `current` and did not restore it (`current.Store(&deps)` at `go/internal/cli/inboxcmd/cmd_inbox.go:59`). The sequential test `TestInboxCmdRun` called `Run` with `Deps{}` through the helper `runInbox` (`go/internal/cli/inboxcmd/inboxcmd_run_test.go:17`). So it left an empty `Deps` in the global, and `host()` returned a `Deps` whose `LaneForbidden` was nil.
- **Trigger of the panic:** the parallel tests start after the sequential tests. When the lint test ran before a parallel test that stored `testDeps()`, it called the nil `LaneForbidden` at `cmd_inbox.go:137`. The order of the parallel tests decided the result, so a re-run was green.
- **Contributing factor:** the self-heal monitor gives every rerun-green event one fingerprint with no test name. So this red and any other flaky test share one incident.

## Analysis

The panic was a defect in the tests, not a flake of the runner. A package global that each entry point overwrites is shared state across tests. One test that passes a partial value changes the result of every later parallel test that reads the global.

## Approaches taken

| Approach | Result |
|---|---|
| Classify the red with `evolve ci classify 45942606193` | The verb read the final conclusion (success) and gave no class. The log of try 1 was read by hand. |
| Count the `--- FAIL` lines | Misleading: most of them are fixture output of build-floor tests. Only the package-level panic with a stack trace is real. |
| Read the panic frame and its callers | Found `host().LaneForbidden` and the one test that stored `Deps{}`. |

## Actions

<!-- GENERATED:actions BEGIN -->
_No actions._
<!-- GENERATED:actions END -->

- Fixed by commit `ae3a35b30` (cycle 2662, item `inboxcmd-tests-never-leak-an-empty-deps-into-the-package-host`). `Run` and `RunMover` call `defer pushHost(deps)()`, and `pushHost` restores the previous host on return (`go/internal/cli/inboxcmd/deps.go`). `runInbox` passes `testDeps()`.
- Regression tests in `go/internal/cli/inboxcmd/deps_isolation_test.go`: `TestRun_AnEmptyDepsDoesNotLeakIntoALaterParallelLint`, `TestRun_RestoresThePreviousHostOnReturn` and `TestRunMover_RestoresThePreviousHostOnReturn`.
- Open: item `repair-inc-2026-10-27-6f89f94c` (console). The self-heal fingerprint of a rerun-green event must carry the job and the test name.

## Effectiveness

<!-- GENERATED:effectiveness BEGIN -->
- **Metric:** `-` (unit: `-`, since: `-`, filters: none)
- **Baseline:** 0 over 0 -(s) | **Target:** 0 | **Checks:** 0
- **Results:** none
<!-- GENERATED:effectiveness END -->

- The regression tests are on main. The fingerprint repair is open.

## Lessons

- A red that a re-run turns green is a defect until its log says otherwise. Read the failed try before you call it a flake.
- In a log of tests that run other tests, count only the package-level failures with a stack trace.
- A package global that each entry point sets is shared state between tests. Restore it on return, or pass the dependencies as a value.
- Check main for a fix before you write the record. A lane fixed this defect inside the same wave, before the record was written.
