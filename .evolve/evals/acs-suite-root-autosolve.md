---
score_cap:
  - criterion: "evolve acs suite resolves its root from cycle-state.json active_worktree when --root is not set"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run TestACSSuiteRootAutosolve ./cmd/evolve/"
  - criterion: "Resolver degrades to empty string (flag-default fallback) on absent/empty/malformed cycle state"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run TestACSSuiteRootFallback ./cmd/evolve/"
---

# Eval: Kernel-owned ACS suite root resolution

> Pins mode 5 of user-phase-persona-resolution: the ACS suite root comes from
> the kernel-owned `cycle-state.json` `active_worktree`, not the invoking
> LLM's cwd. Source incident: cycles 226–227 false-FAILed because the auditor
> ran the suite against the main tree instead of the active worktree; commit
> 48f8ff7 added a C0 prompt block as a stopgap — this eval pins the kernel
> fix that makes the prompt block defense-in-depth only.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| autosolve-happy | active_worktree honored as suite root | 4/10 | `go test -run TestACSSuiteRootAutosolve ./cmd/evolve/` |
| graceful-fallback | absent/empty/malformed state → flag default | 5/10 | `go test -run TestACSSuiteRootFallback ./cmd/evolve/` |
