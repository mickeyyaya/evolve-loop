---
score_cap:
  - criterion: "Every production gh call site is listed in the gh call table with its verb and purpose"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2475_001' ./acs/cycle2475"
  - criterion: "docs/operations/cell-gh-calls.md lists each gh call with its site, command and purpose"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2475_002' ./acs/cycle2475"
  - criterion: "A gh command in a phase cell fails before gh starts, through sysexec, the library callers and the evolve status verb"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2475_003|TestC2475_004|TestC2475_005' ./acs/cycle2475"
  - criterion: "The host gh config dir is in the denied secret class and no read grant reaches it"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2475_006' ./acs/cycle2475"
  - criterion: "No GitHub token key reaches a cell on any env carrier, even from a passthrough list or an opt-in"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2475_007' ./acs/cycle2475"
---

# Eval: A phase cell does not reach GitHub credentials

> Pins the decision to remove gh from phase cells. The pipeline does GitHub work outside the cell. A cell gets no GitHub token and an empty gh config dir. Source: inbox item cells-read-gh-credentials, wave 145 (360 reads outside grants, mostly `~/.config/gh/hosts.yml`). Cycle 2367 built the guard and cycle 2475 closes the inventory and the proof.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| inventory-drift | gh call table equals the gh call sites of production code | 6/10 | `TestC2475_001` |
| inventory-doc | the call doc lists each table entry | 6/10 | `TestC2475_002` |
| cell-guard | gh in a cell returns `sysexec.ErrGhInCell` and never starts | 7/10 | `TestC2475_003`, `TestC2475_004`, `TestC2475_005` |
| secret-class | the gh config dir is denied and never granted | 7/10 | `TestC2475_006` |
| token-closure | the four GitHub token keys are unset on each carrier | 7/10 | `TestC2475_007` |
