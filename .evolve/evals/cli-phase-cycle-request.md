---
score_cap:
  - criterion: "`evolve phase scout --cycle N --project-root P` runs scout once with the cycle, workspace, worktree, worktree base SHA, run id, goal hash and explanation version read from runs/cycle-N/cycle-state.json"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1830_001_' ./acs/cycle1830"
  - criterion: "`evolve phase <name> --cycle N` exits 1 naming the reason, and dispatches nothing, when the cycle has no cycle-state.json or run directory, an unparseable state, a missing or unrecorded worktree, or a live run lease; a stale lease does not refuse"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1830_002_' ./acs/cycle1830"
  - criterion: "`evolve phase <name> --cycle N` exits 10 naming stdin when a request also arrives on stdin (checked before the state is read) and on a non-integer --cycle; whitespace-only stdin is no request"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1830_003_' ./acs/cycle1830"
  - criterion: "`evolve phase <name>` without --cycle still dispatches the stdin request as given and still exits 11 on malformed stdin JSON"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1830_004_' ./acs/cycle1830"
  - criterion: "the evolve binary's `compose --phases scout,triage --cycle N --dry-run` plans scout -> triage for a valid cycle and exits 1 or 10 with the reason for every refusal the phase command makes"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1830_005_' ./acs/cycle1830"
  - criterion: "`evolve compose --phases scout,triage --cycle N` runs both phases in order with one identical request derived from cycle-state.json, and a refused derivation runs no phase"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestCompose_CycleFlag_' ./cmd/evolve"
---

# Eval: `evolve phase <name>` and `evolve compose` rerun a built-in phase from a cycle's own state

> Pins the `--cycle N` (with `--project-root P`) request derivation added to
> `evolve phase <name>` and `evolve compose`. Before it, rerunning a phase for a
> real cycle meant hand-copying cycle, workspace, worktree, base SHA, run id and
> goal hash out of `runs/cycle-N/cycle-state.json` into stdin JSON, and a typo ran
> the phase against the wrong workspace. The derivation reads that file itself and
> refuses (exit 1) a cycle with no state, a gone worktree or a live run lease;
> `--cycle` beside a stdin request is exit 10; stdin stays the input without
> `--cycle`. Source: inbox item `cli-phase-cycle-request` (console core-function
> CLI inventory, 2026-09-30), materialized in cycle 1830.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| phase-derivation | scout dispatched with every cycle-state.json field | 7/10 | `go test -tags acs -run '^TestC1830_001_' ./acs/cycle1830` |
| phase-refusal-fences | exit 1 with reason, no dispatch; stale lease proceeds | 7/10 | `go test -tags acs -run '^TestC1830_002_' ./acs/cycle1830` |
| phase-stdin-conflict | exit 10 on stdin request or malformed --cycle | 6/10 | `go test -tags acs -run '^TestC1830_003_' ./acs/cycle1830` |
| phase-stdin-compat | stdin request path unchanged without --cycle | 5/10 | `go test -tags acs -run '^TestC1830_004_' ./acs/cycle1830` |
| compose-binary-wiring | real binary honors --cycle and every refusal | 6/10 | `go test -tags acs -run '^TestC1830_005_' ./acs/cycle1830` |
| compose-one-request | both phases get the one derived request | 7/10 | `go test -run '^TestCompose_CycleFlag_' ./cmd/evolve` |
