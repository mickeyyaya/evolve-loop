---
score_cap:
  - criterion: "A removal claim for a path that is absent from the worktree but STILL IN that worktree's Git index is reported as a false claim (a fresh checkout restores it)"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -v -run '^TestRemovalClaimFailures_TrackedButAbsentFromDisk$' ./internal/core | grep -q -- '--- PASS: TestRemovalClaimFailures_TrackedButAbsentFromDisk'"
  - criterion: "NEGATIVE — an untracked absent path stays an honest removal, and a worktree that is not a Git repository still fails open (the build floor never false-blocks over its own plumbing)"
    max_if_missing: 9
    evidence: "cd go && go test -count=1 -v -run '^TestRemovalClaimFailures_UntrackedAbsent_StaysHonest$' ./internal/core | grep -q -- '--- PASS: TestRemovalClaimFailures_UntrackedAbsent_StaysHonest'"
  - criterion: "EDGE — a path that is false on both axes (still on disk AND still tracked) yields exactly one failure, not two, and keeps the cycle-660 on-disk message"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -v -run '^TestRemovalClaimFailures_TrackedAndPresent_ReportsExactlyOnce$' ./internal/core | grep -q -- '--- PASS: TestRemovalClaimFailures_TrackedAndPresent_ReportsExactlyOnce'"
  - criterion: "The stale live inbox record is retired as a TRACKED deletion — gone from the Git index, not merely from the working tree"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -v -tags acs -run '^TestC1591_003_stale_inbox_record_retired_as_tracked_deletion$' ./acs/cycle1591 | grep -q -- '--- PASS: TestC1591_003_stale_inbox_record_retired_as_tracked_deletion'"
  - criterion: "The RED contract file is git-tracked, so the acceptance tests survive the ship commit instead of being dropped as an untracked working-tree file"
    max_if_missing: 7
    evidence: "git ls-files --error-unmatch go/internal/core/build_removal_check_index_test.go"
---

# Eval: retire the stale retro-prompt-delivery-stall record as a tracked deletion

> The live inbox record `.evolve/inbox/2026-08-18T02-30-00Z-retro-prompt-delivery-stall.json`
> has been "retired" more than once and has come back every time. Each retirement was a
> filesystem-only removal — a plain delete, or a move into the `.gitignore`d
> `.evolve/inbox/processed/` destination (cycle-1584's staging failure) — with no matching
> `git rm`, so the path stayed in the Git index and the next fresh checkout restored it.
> The item then reopened as live and reopened an empty lane, while its own bridge fix had
> already shipped (submit-verify bounding, `submit_wedged` → retryable `ErrArtifactTimeout`,
> the `auto` sentinel omission).
>
> The gate that exists to catch exactly this false claim, `core.RemovalClaimFailures`
> (`go/internal/core/build_removal_check.go`), asks only the worktree filesystem via
> `os.Stat` (lines 60-62): a path gone from disk reads to it as an honest removal, whatever
> the index says. The claim a build report makes is about the state of the REPOSITORY, so
> the index is the second half of the truth check. This eval pins that half generically —
> for ANY claimed path, never the retro filename — and pins its blast radius in both
> directions: honest untracked removals stay silent, a non-repo worktree still fails open,
> and the cycle-660 on-disk case still reports exactly once. Source incidents: cycle 660
> (the original false removal claim), cycle 1584 (the ignored-destination staging failure),
> cycle 1591 (this retirement).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| index-truth | Tracked-but-absent claim is reported as false | 9/10 | `-run TestRemovalClaimFailures_TrackedButAbsentFromDisk` |
| negative-no-overreach | Untracked-absent honest; non-repo fails open | 9/10 | `-run TestRemovalClaimFailures_UntrackedAbsent_StaysHonest` |
| no-double-count | False on both axes reports exactly once | 6/10 | `-run TestRemovalClaimFailures_TrackedAndPresent_ReportsExactlyOnce` |
| record-retired | Stale record gone from index AND disk | 8/10 | `-tags acs -run TestC1591_003_...` |
| contract-tracked | The RED contract file is committed, not untracked | 7/10 | `git ls-files --error-unmatch` |

## Acceptance Criteria (code-graded)

### AC1: a tracked-but-absent removal claim is rejected [code]
```bash
cd go && go test -count=1 -v -run '^TestRemovalClaimFailures_TrackedButAbsentFromDisk$' ./internal/core | grep -q -- '--- PASS: TestRemovalClaimFailures_TrackedButAbsentFromDisk'
```
Expected: exit 0

### AC2 (negative): honest untracked removals and non-repo worktrees are untouched [code]
```bash
cd go && go test -count=1 -v -run '^TestRemovalClaimFailures_UntrackedAbsent_StaysHonest$' ./internal/core | grep -q -- '--- PASS: TestRemovalClaimFailures_UntrackedAbsent_StaysHonest'
```
Expected: exit 0

### AC3 (edge): a claim false on both axes is reported exactly once [code]
```bash
cd go && go test -count=1 -v -run '^TestRemovalClaimFailures_TrackedAndPresent_ReportsExactlyOnce$' ./internal/core | grep -q -- '--- PASS: TestRemovalClaimFailures_TrackedAndPresent_ReportsExactlyOnce'
```
Expected: exit 0

### AC4: the stale record is gone from the Git index and from disk [code]
```bash
cd go && go test -count=1 -v -tags acs -run '^TestC1591_003_stale_inbox_record_retired_as_tracked_deletion$' ./acs/cycle1591 | grep -q -- '--- PASS: TestC1591_003_stale_inbox_record_retired_as_tracked_deletion'
```
Expected: exit 0

### AC5: the RED contract file is git-tracked (not dropped at ship) [code]
```bash
git ls-files --error-unmatch go/internal/core/build_removal_check_index_test.go
```
Expected: exit 0
