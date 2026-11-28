---
schema: incident-record/1
id: 2026-11-28-production-embed-from-testdata-turned-main-red
title: "A production file embedded its bench set from testdata, so each tree copy failed to build and main was red for ten hours"
level: record
status: fixed
selfheal: ""
fingerprint: ""
signals: [GATE_REPO_MAIN_RED_PARKED]
prs: [1880, 1881]
items: [fix-main-red-cb3a9abe]
actions: []
effectiveness:
  metric: {code: "", filters: [], unit: "", since: ""}
  baseline: {value: 0, units: 0}
  target: 0
  checks: 0
  results: []
verified_at: ""
---

# A production file embedded its bench set from testdata, so each tree copy failed to build and main was red for ten hours

## Summary

<!-- GENERATED:status BEGIN -->
- **Status:** fixed
<!-- GENERATED:status END -->

## Trigger and level

- **Trigger:** on 2026-11-28 the `required CI` run 46102235437 of main `0cddbcee0` was red in the two ubuntu jobs. Nine `TestSkillsDriftGate_*` tests in `go/internal/phases/audit` failed. The commit `0cddbcee0` is the ship of cycle 2670 (the item `code-index-second-independent-truth-set`). It did not change the audit package.
- **Impact:** main stayed red from 03:18Z to 13:56Z, through nine red runs of `required CI`. The ship gate parked six cycles of wave 193 (2668, 2674, 2675, 2676, 2677 and 2683) with `GATE_REPO_MAIN_RED_PARKED`. Other cycles shipped. Each console PR that the batch tested on main got the same red, for example PR #1879.
- **Level:** record. The red reached main, the lanes and CI.

## Timeline

| Time (2026-11-28, UTC) | Event | Evidence |
|---|---|---|
| 02:15 | The build floor of cycle 2670 passes. | `.evolve/runs/cycle-2670/signals.ndjson`, `HOST_VERIFY_LOCK_WAIT` source `build-floor` |
| 03:18 | Cycle 2670 ships `0cddbcee0` to main. `required CI` run 46102235437 starts. | `git log origin/main`; `gh run list --branch main` |
| 03:38 | The two ubuntu jobs report the nine drift tests red. | the failed log of run 46102235437 |
| 04:29 | The main-red stop records main red, with the fix item `fix-main-red-cb3a9abe`. | the field `since` of `GATE_REPO_MAIN_RED_PARKED` |
| 04:42 | The ship gate parks cycle 2668. Five more cycles park by 12:21. | `evolve signals tail --cycle 2668 -code GATE_REPO_MAIN_RED_PARKED` |
| about 08:30 | The console runs `evolve ci classify`. It names the drift tests and the embed error. | `evolve ci classify` output |
| 09:12 | PR #1880 ships the fix and a guard, with three review seats (PASS). | `evolve ship` output |
| 12:24 | `evolve integration batch --main-red-fix 1880` tests the fix alone in batch PR #1881. | the batch output |
| 13:23 | The first run of #1881 is red on the focus pool flake (`TestFocus_FiveLanePoolLaunchesFocusIDsFirstThenOthersInRankOrder`, fixed by PR #1873). The rerun is green, and the batch lands #1880 as `b62e11c23`. | run 46144532853; the batch output |
| 13:56 | `required CI` run 46149355625 of `b62e11c23` is green. | `evolve integration watch --tip b62e11c23` |

## Key logs

```text
internal/codeindex/independent.go:10:12: pattern testdata/independent/go.mod.txt: no matching files found
--- FAIL: TestSkillsDriftGate_Cycle1840ReplayPasses (1.80s)
audit_skillsdrift_worktree_integration_test.go:205: a generator change with identical output = ([the worktree generator skills check exited 1 without a drift report: internal/codeindex/independent.go:10:12: pattern testdata/independent/go.mod.txt: no matching files found], <nil>), want (nil, nil)
```

The same log also has `--- FAIL` lines for `TestC1946_001`, `TestC1950_001`, `TestRatchet_NoNewRawGitFixtures`, `TestA` and `TestF`. Tests that run other tests against fixtures write these lines. They are not failures of the run.

## Root cause and contributing factors

This section describes the code at `0cddbcee0`, before the fix.

- **Root cause:** `go/internal/codeindex/independent.go:10-17` embedded three files from `testdata/independent/` with `//go:embed`. The skills drift gate copies the tree to a temporary worktree, and the copy skips each `testdata` directory, as the Go tools do (`audit_skillsdrift_worktree_integration_test.go:77`). So the copy had no files for the embed patterns, `go build` failed, and the gate reported no drift report.
- **Escape:** the drift tests have the build tag `integration` and live in a package that the cycle did not change. The build floor of the cycle ran the changed packages, `codeindex` and `cmd/evolve`, and these passed. The tests fail on each run of that tree, and the floor passed, so the floor did not run them. Only the CI jobs that run every package with every gate ran them.
- **Contributing factor:** no guard stopped a production file from embedding from `testdata`. Go permits the embed, and a normal build in the source tree finds the files.
- **Contributing factor:** the shadow train composed cycle 2670 onto `integration/p2665-b2`, and `evolve integration status` still gives the pool the verdict `clean`. In shadow, "clean" means that the wave record is complete and chained. It does not mean that CI passed on each batch tip.

## Analysis

The defect was in production code, but it showed only when a tool copied the tree. Two conventions met: Go keeps `testdata` out of the package, and the tree copiers of the gates skip it for the same reason. A file under `testdata` is test input by contract. Code that ships must not read it.

The cycle gates ran the packages that the diff changed. A build defect that shows only in another package's integration test is outside that set. The main-red stop worked as designed: it parked only the cycles whose ship scan met the red tests, and it did not blame their lanes.

## Approaches taken

| Approach | Result |
|---|---|
| Read the failed log of the main run | The log has more than 370 000 lines, and many `FAIL` lines are fixture output. The real failures were not clear. |
| Run `evolve ci classify` on the run | It named the nine drift tests and the embed error. |
| Run the audit package without `-tags integration` | It passed, because the drift tests need the tag. With the tag, the red reproduced on the host. |
| Move the files to `benchdata/independent/` (PR #1880) | The drift tests, `codeindex`, and the code-bench tests in `cmd/evolve` pass. The bytes of the three files did not change. |
| Write the guard with a hand parser of `//go:embed` lines | Round 1 of the architecture review found that it missed `all:`, a tab after the directive, and quoted patterns. The console rewrote it on `go/build`. |

## Actions

<!-- GENERATED:actions BEGIN -->
_No actions._
<!-- GENERATED:actions END -->

- Fixed by PR #1880 (branch `cl/codeindex-embed-outside-testdata`). The three files move to `go/internal/codeindex/benchdata/independent/`, and `independent.go` embeds them from there.
- Regression test: `go/internal/guards/embed_outside_testdata_test.go` (`TestNoProductionFileEmbedsFromATestdataDirectory`). It reads the embed patterns of each source directory with `go/build` (`UseAllFiles`), which skips test files and accepts each pattern form that Go accepts. On main `bea93ef80`, the red tip of main when the console wrote the guard, it fails and names the three embeds. With the fix it passes. Its own tests killed six of six mutants of the reader.

## Effectiveness

<!-- GENERATED:effectiveness BEGIN -->
- **Metric:** `-` (unit: `-`, since: `-`, filters: none)
- **Baseline:** 0 over 0 -(s) | **Target:** 0 | **Checks:** 0
- **Results:** none
<!-- GENERATED:effectiveness END -->

- The guard is in the `guards` package. The CI jobs that run each package run it, so a new embed from `testdata` makes the PR red before it reaches main.

## Lessons

- Production code must not embed from `testdata`. The tools that copy a tree skip that directory, so the build breaks there and not in the source tree.
- A cycle floor that runs only the changed packages does not see a break that shows in the integration tests of another package. The guard closes this one form. The general escape stays open.
- In shadow, a clean pool verdict is evidence about the record, not about CI. Read the batch tip CI before you use a shadow wave as evidence of safe merges.
- The main-red stop has two release paths. After a landing, `evolve integration batch` releases the keys that its tip proves green (`stopWiring.landed` in `cmd/evolve/cmd_integration_stop.go`). But the key run of the batch goes through `core.CIKeysStayRed`, which always gives the job-level keys (`origin-main-ci`) as red. Only the main CI poll of a running loop releases these keys (`mainCIPoll.closeOnGreen` in `cmd/evolve/main_ci_poll.go`).
- So the batch of 6 PRs on green main got `GATE_REPO_MAIN_RED_STOP_WOULD_REFUSE` (`not-a-fix`). The stop is in shadow, so nothing was refused. Under enforce, `StopHold.decide` returns `ErrMainRedHold` for that admission, so enforce refuses this batch until a loop runs the poll. This last step comes from the code. It was not run.
- Run `evolve ci classify` before you read a long CI log by hand. The log of tests that run other tests has many `FAIL` lines that are not failures.
