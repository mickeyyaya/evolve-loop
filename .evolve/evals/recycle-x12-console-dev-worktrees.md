---
score_cap:
  - criterion: "evolve gc removes a clean tree whose branch is merged or closed and keeps each branch"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2640_001_' ./acs/cycle2640"
  - criterion: "a dirty dev tree is kept and the report lists its changed paths"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2640_002_' ./acs/cycle2640"
  - criterion: "a tree that matches gc.dev_worktrees.protected stays with the reason naming the glob, and no code list holds code-index"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2640_00[34]_' ./acs/cycle2640"
  - criterion: "a tree with a process inside or an open pull request stays"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2640_00[56]_' ./acs/cycle2640"
  - criterion: "the dry run on the 98-tree fixture lists the keep reason of each tree, and the real verb removes what the dry run planned"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2640_00[78]_' ./acs/cycle2640"
  - criterion: "each changed production function is covered at 100 percent"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2640_009_' ./acs/cycle2640"
---

# Eval: the console dev worktree sweep (X12)

> Pins the X12 contract of the static gc flow: `evolve gc` removes a clean console dev tree whose branch is merged or closed, keeps each branch, keeps a dirty tree and lists its changed paths, and honors `gc.dev_worktrees.protected` as data. Source: plan `docs/plans/cycle-recycle-and-gc-check-2026-11.md` section 13 (D24 to D26), cycle 2640. On 2026-11-22 the console removed 35 clean landed trees by hand because the sweep kept all of them with the same reason.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| closed-state | A clean closed or merged tree is removed and its branch stays | 6/10 | `TestC2640_001_` |
| dirty-report | A dirty tree is kept with its changed paths | 7/10 | `TestC2640_002_` |
| protected-data | A glob in policy protects, with no code list | 7/10 | `TestC2640_003_`, `TestC2640_004_` |
| occupied | A process or an open pull request keeps the tree | 7/10 | `TestC2640_005_`, `TestC2640_006_` |
| keep-reasons | Each of 98 trees has a reason, and the real run equals the dry run | 6/10 | `TestC2640_007_`, `TestC2640_008_` |
| coverage | Each changed function is at 100 percent | 5/10 | `TestC2640_009_` |
