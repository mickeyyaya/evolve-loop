---
score_cap:
  - criterion: "cyclestate shared reader pins absent and malformed state.json"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestReadLastCycleNumber|TestReadState)$' ./internal/cyclestate"
  - criterion: "wave facts list squash-merged PRs (feat: x (#123) yields 123)"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestParseMergedPRs' ./cmd/evolve"
  - criterion: "wave and redteamcheck behavior unchanged after reader unification"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/wave ./internal/redteamcheck"
---

# Eval: shared state.json reader and squash-merged PRs in wave facts

> Pins cycle 1870: one read-only state decoder in cyclestate used by wave and redteamcheck, and wave facts counting GitHub squash merges `(#N)` beside `Merge pull request #N`. Source: inbox 2026-10-08T13-39-29Z-wave-shared-state-reader-and-squash-prs.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| shared-reader | absent and malformed cases pinned | 6/10 | `go test -run TestReadLastCycleNumber ./internal/cyclestate` |
| squash-prs | `(#123)` subject listed | 7/10 | `go test -run TestParseMergedPRs ./cmd/evolve` |
| no-regression | wave and redteam suites green | 7/10 | `go test ./internal/wave ./internal/redteamcheck` |
