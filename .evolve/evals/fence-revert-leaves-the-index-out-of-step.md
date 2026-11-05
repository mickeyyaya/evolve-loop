---
score_cap:
  - criterion: "A path staged outside the writable list leaves no AD or MM status after the fence revert, and the diagnostic names it"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC2168_001_|TestC2168_002_|TestC2168_003_)' ./acs/cycle2168"
  - criterion: "The reverted patch still holds each staged change"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2168_004_' ./acs/cycle2168"
  - criterion: "A path inside the writable list stays staged and on disk"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2168_005_' ./acs/cycle2168"
  - criterion: "The revert never stages an edit that was unstaged at dispatch"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2168_006_' ./acs/cycle2168"
  - criterion: "An index that cannot be restored is reported and never verified"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2168_007_' ./acs/cycle2168"
  - criterion: "A staged change that the seat undoes on disk (AD, MM, D plus untracked, add -A over a dispatch-time untracked file) leaves one state and the diagnostic names the path"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC2168_011_|TestC2168_012_|TestC2168_013_|TestC2168_014_|TestC2168_015_|TestC2168_016_|TestC2168_017_)' ./acs/cycle2168"
  - criterion: "An unmerged index at dispatch does not turn the fence off"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2168_018_' ./acs/cycle2168"
  - criterion: "A retracted test-writing phase leaves the index and the working copy in step and never stages a dispatch-time unstaged edit"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC2168_019_|TestC2168_020_|TestC2168_021_)' ./acs/cycle2168"
  - criterion: "Each changed function of the fence has 100 percent coverage and the package gains no comment"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC2168_009_|TestC2168_010_)' ./acs/cycle2168"
  - criterion: "A gate write that the fence reverted (unstaged, staged then deleted, tracked edit, staged) never lets the identity carry ship without a second audit; a gate that writes nothing still carries"
    max_if_missing: 2
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2168_022_' ./acs/cycle2168"
  - criterion: "A stage undone on disk gives no patch, and agent-run.md and internal-treefence.md say so"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2168_023_' ./acs/cycle2168"
  - criterion: "A SHA-256 repository leaves one state after the fence revert"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2168_024_' ./acs/cycle2168"
---

# Eval: A fence revert leaves the index and the working copy apart

> The worktree fence reverts a write that a seat makes outside its writable list.
> It restores the working copy but never the real index, because the snapshot is
> taken on an index copy. A staged path thus stays staged, and `git status`
> shows AD or MM. Source incident: 2026-11-04, trees wi-i3, wi-i6, wi-i10,
> wi-i13 and role-guard (inbox item `fence-revert-leaves-the-index-out-of-step`,
> cycle 2168). Audit round 1 of cycle 2168 added the undone-stage, unmerged and retract patterns.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| one-state | No AD or MM after the revert; the diagnostic names each path | 3/10 | `TestC2168_001_` to `TestC2168_003_` |
| patch | The patch holds the staged change | 4/10 | `TestC2168_004_` |
| writable | A writable path stays staged | 5/10 | `TestC2168_005_` |
| no-new-staging | A dispatch-time unstaged edit stays unstaged | 5/10 | `TestC2168_006_` |
| loud-failure | A failed index restore is never verified | 5/10 | `TestC2168_007_` |
| undone-stage | A stage undone on disk leaves one state; each path is named | 3/10 | `TestC2168_011_` to `TestC2168_017_` |
| unmerged | An unmerged index at dispatch keeps the fence on | 4/10 | `TestC2168_018_` |
| retract | A retraction leaves one state | 4/10 | `TestC2168_019_` to `TestC2168_021_` |
| hygiene | 100 percent coverage of changed functions; no comment | 6/10 | `TestC2168_009_`, `TestC2168_010_` |
| carry | A reverted gate write returns the identity carry to audit | 2/10 | `TestC2168_022_` |
| undone-stage-docs | An undone stage gives no patch and both documents say so | 5/10 | `TestC2168_023_` |
| sha256 | A SHA-256 repository leaves one state | 6/10 | `TestC2168_024_` |
