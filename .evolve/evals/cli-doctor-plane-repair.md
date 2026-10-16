---
score_cap:
  - criterion: "`evolve doctor plane --repair` restores both link directions after a fixture hub moves and prints `repaired: <path>` for each worktree"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1905_001 ./acs/cycle1905"
  - criterion: "Without `--repair`, `evolve doctor plane` prints `broken: <path>` for each broken worktree, exits 1 and rewrites no link"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1905_002 ./acs/cycle1905"
  - criterion: "Negative cases: a failing git worktree exits 2 with no repaired claim, an unknown flag exits 10, a missing path errors"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1905_003 ./acs/cycle1905"
  - criterion: "Edge case: a healthy hub exits 0 under inspection and under --repair (flag before or after the root) and claims no repairs"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1905_004 ./acs/cycle1905"
  - criterion: "docs/operations/workspace-layout.md names `evolve doctor plane --repair` instead of raw `git --git-dir=.repo.git worktree repair`"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1905_005 ./acs/cycle1905"
  - criterion: "A worktree under a dot-directory (runtime/.evolve/worktrees/<lane>) is reported broken after a hub move and repaired by --repair"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1905_006 ./acs/cycle1905"
  - criterion: "Linked worktrees and separate-git-dir checkouts of other repositories inside the hub are never reported broken nor passed to git worktree repair"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1905_007 ./acs/cycle1905"
  - criterion: "Worktree enumeration comes from the store's records: a registered worktree outside the hub tree is inspected and repaired"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1905_008 ./acs/cycle1905"
---

# Eval: Doctor plane worktree repair (cycle 1905)

> Pins `evolve doctor plane --repair` (inbox item `cli-doctor-plane-repair`, source `docs/research/cli-coverage-inventory-2026-09-30.md:64,227`).
> When the hub directory moves, each worktree `.git` file and each store record `.repo.git/worktrees/<name>/gitdir` points at the old place.
> The command reports these broken links (exit 1) and, with `--repair`, runs `git worktree repair` with the new paths (exit 0). A git failure exits 2.
> Source incident: cycle 1905 scout finding — `docs/operations/workspace-layout.md:214` told the operator to run raw git.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| repair-restores | moved hub repaired, each path printed | 6/10 | `go test -tags acs -run TestC1905_001 ./acs/cycle1905` |
| inspect-reports | broken links reported, exit 1, nothing rewritten | 6/10 | `go test -tags acs -run TestC1905_002 ./acs/cycle1905` |
| negative-cases | git failure exit 2, bad flag exit 10, missing path errors | 7/10 | `go test -tags acs -run TestC1905_003 ./acs/cycle1905` |
| edge-cases | healthy hub exit 0 under both modes | 7/10 | `go test -tags acs -run TestC1905_004 ./acs/cycle1905` |
| doc-updated | layout doc names the new command | 7/10 | `go test -tags acs -run TestC1905_005 ./acs/cycle1905` |
| dot-dir-lane | nested lane under .evolve/worktrees inspected + repaired (audit H1) | 5/10 | `go test -tags acs -run TestC1905_006 ./acs/cycle1905` |
| foreign-git | other repositories' .git files left alone (audit M2) | 6/10 | `go test -tags acs -run TestC1905_007 ./acs/cycle1905` |
| store-enumeration | outside-hub registered worktree inspected (audit M1 proxy) | 6/10 | `go test -tags acs -run TestC1905_008 ./acs/cycle1905` |
