---
schema: incident-record/1
id: 2026-11-26-cmd-evolve-darwin-suite-at-the-30m-limit
title: "The cmd/evolve darwin CI suite reached its 30-minute limit twice in one day"
level: record
status: open
selfheal: ""
fingerprint: ""
signals: []
prs: [1841]
items: [cmd-evolve-darwin-tests-stay-clear-of-the-go-test-timeout, ci-classify-names-a-test-timeout-and-a-rerun-red]
actions: []
effectiveness:
  metric: {code: "", filters: [], unit: "", since: ""}
  baseline: {value: 0, units: 0}
  target: 0
  checks: 0
  results: []
verified_at: ""
---

# The cmd/evolve darwin CI suite reached its 30-minute limit twice in one day

## Summary

<!-- GENERATED:status BEGIN -->
- **Status:** open
<!-- GENERATED:status END -->

## Trigger and level

- **Trigger:** on 2026-11-26 the darwin job of required CI failed on main `a3b1f16d5` and then on PR #1841. Both failures were a `go test` timeout panic in the `cmd/evolve` package.
- **Earlier occurrences:** on 2026-11-24 the macOS job of PRs 1789 and 1790 failed with the same panic in the same test. The item `cmd-evolve-darwin-tests-stay-clear-of-the-go-test-timeout` has tracked the cause since 2026-11-25, and no cycle has built it.
- **Impact:** main was red for one re-run, and the boundary batch of #1841 waited for one re-run. No wrong code landed.
- **Level:** record. The failure occurred four times in three days, and it reached main and CI.

## Timeline

| Time (2026-11-26, UTC) | Event | Evidence |
|---|---|---|
| about 11:00 | Required CI on main `a3b1f16d5` failed: the `cmd/evolve` darwin tests timed out after 30m. The batch run of the same SHA was green. | `gh run list --branch main` |
| about 11:05 | `gh run rerun --failed` on the main run. The re-run was green. | the run page |
| 13:25 | The console shipped PR #1841 (`2f8d41392`). | the ship output |
| about 14:10 | Required CI on #1841 failed in the darwin set with the same timeout. | the failed darwin step of the #1841 run (operator console log, not kept) |
| about 14:15 | `evolve ci classify` returned class `unknown` and retry-safe `no`. | the classify output |
| about 14:20 | One re-run of the failed jobs was green: 14 checks passed, 2 skipped. | the checks of PR #1841 |

## Key logs

Source: the failed darwin step of PR #1841, trimmed.

```text
panic: test timed out after 30m0s
	running tests:
		TestCLIContract_HarnessKillsEachMutantOfDossierShow/write-cwd (1m39s)
		TestCLIContract_HarnessKillsEachMutantOfDossierShow/write-in-home (1m37s)
		TestCLIContract_HarnessKillsEachMutantOfDossierShow/write-in-tmpdir (1m39s)
FAIL	github.com/mickeyyaya/evolve-loop/go/cmd/evolve	1805.645s
```

## Root cause and contributing factors

- The `cmd/evolve` package test run on the darwin runner takes close to its 30-minute timeout. A slow runner pushes it past the limit.
- `TestCLIContract_HarnessKillsEachMutantOfDossierShow` in `go/cmd/evolve/cli_contract_mutants_integration_test.go` runs one subtest per mutant, and each subtest takes about 1m39s. The subtests ran at the panic both times.
- Contributing factor: `evolve ci classify` has no class for a test timeout panic, so it returned `unknown`. Its `recurredOn` check reads the main runs that `MainRunsBefore` returns. A run that a re-run turned green has no failure keys (`failureKeys` returns none for a success). So the classifier did not see the recurrence.

## Analysis

- Two reds with the same running test and the same panic are one cause, not two flakes. A re-run hides the cause and costs one CI round each time.
- The classifier is the gate that must catch a recurrence. It missed this one because it reads the final state of a run, not each run try.

## Approaches taken

| Approach | Result | Kept |
|---|---|---|
| Re-run the failed jobs | Green both times; it hides the cause | Only as the immediate recovery |
| Raise the `go test` timeout | It moves the limit and hides the growth. The operator owns the value. | No |
| Make the mutant harness cheaper (build once, run in parallel, or a separate job) | Removes the cause | Yes: item `cmd-evolve-darwin-tests-stay-clear-of-the-go-test-timeout` |
| Classify a timeout panic and count a red run try that went green | The next recurrence is named, not `unknown` | Yes: item `ci-classify-names-a-test-timeout-and-a-rerun-red` |

## Actions

<!-- GENERATED:actions BEGIN -->
_No actions._
<!-- GENERATED:actions END -->

- Item `cmd-evolve-darwin-tests-stay-clear-of-the-go-test-timeout`: the margin report shows at least 10 minutes of margin for `cmd/evolve` on the macOS job, and no timeout value changes. The 2026-11-26 evidence is added to the item, and its weight goes from 0.7 to 0.85.
- Item `ci-classify-names-a-test-timeout-and-a-rerun-red`: a timeout class, and `recurredOn` reads each run try.

## Effectiveness

<!-- GENERATED:effectiveness BEGIN -->
- **Metric:** `-` (unit: `-`, since: `-`, filters: none)
- **Baseline:** 0 over 0 -(s) | **Target:** 0 | **Checks:** 0
- **Results:** none
<!-- GENERATED:effectiveness END -->

- Open. The two items carry the red-first tests.

## Lessons

- When a re-run turns a red green, record the running test of the panic. A second red with the same test is a recurrence, not a flake.
- A classifier that reads only the final state of a run cannot see a recurrence that a re-run hid.
- Before you file an item, search the open items for the failing test name. The console filed a duplicate of the 2026-11-25 item, and the inbox review found it.
