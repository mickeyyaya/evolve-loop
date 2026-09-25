---
score_cap:
  - criterion: "statemap exports ResolveWriteTarget with the bounded, dangling-tolerant chain semantics of the former unexported helper, including a dangling-symlink case"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -v -run '^TestResolveWriteTarget$' ./internal/adapters/statemap | grep -q -- '--- PASS: TestResolveWriteTarget'"
  - criterion: "core.SealCycle's flock.WithPathLock takes the RESOLVED canonical sidecar when <evolveDir>/state.json is a symlink"
    max_if_missing: 8
    evidence: "cd go && env -u EVOLVE_CYCLE_STATE_FILE go test -count=1 -v -run '^TestSealCycle_SymlinkedStateLocksCanonicalTarget$' ./internal/core | grep -q -- '--- PASS: TestSealCycle_SymlinkedStateLocksCanonicalTarget'"
  - criterion: "ResolveWriteTarget is a documented export of the apicover-enrolled statemap package and a statemap test names it"
    max_if_missing: 6
    evidence: "cd go && go doc ./internal/adapters/statemap ResolveWriteTarget | grep -q 'func ResolveWriteTarget(path string) string' && grep -lq 'ResolveWriteTarget' internal/adapters/statemap/*_test.go"
  - criterion: "statemap and core's seal/reset family carry no regression"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./internal/adapters/statemap && env -u EVOLVE_CYCLE_STATE_FILE go test -count=1 -run '^Test(SealCycle|AutosealStaleMarker|MarkerShouldAutoseal)' ./internal/core"
---

# Eval: export statemap.ResolveWriteTarget and lock SealCycle on the resolved path

> Scout task 1 of inbox item `statejson-latent-unresolved-writers` (cycle 1690).
> `statemap.resolveWriteTarget` already gave `WriteStateMap`/`UpdateStateMap`
> symlink write-through and cross-tree lock unification, but it was unexported,
> so `core.SealCycle` (`reset.go`, `flock.WithPathLock(statePath, …)`) locked the
> raw path. Through a worktree link that is a sidecar no canonical-path writer
> ever takes. Exporting the helper and locking
> `statemap.ResolveWriteTarget(statePath)` restores the single-lock contract.
> Source incidents: cycle-999/1000 (link sever / stranded writes); the
> 2026-07-21 go-reviewer MEDIUM that filed the inbox item. The authority eval for
> the whole item is `statejson-latent-unresolved-writers.md`.
>
> Re-authored by the cycle-1690 TDD phase: the scout draft's `[code]` lines were
> not graders the eval-quality parser recognises (quality-check L1 "zero parsed
> commands", WARN), so the draft verified nothing.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| export-semantics | final target through relative/absolute/chained links; dangling tail returned | 7/10 | `go test -run '^TestResolveWriteTarget$' ./internal/adapters/statemap` |
| seal-lock-unification | SealCycle locks `<canonical>.lock` | 8/10 | `go test -run '^TestSealCycle_SymlinkedStateLocksCanonicalTarget$' ./internal/core` |
| apicover | documented, test-named export | 6/10 | `go doc` + test-name grep |
| no-regression | statemap + core seal/reset family green | 5/10 | package suites |
