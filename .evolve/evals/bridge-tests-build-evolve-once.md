---
score_cap:
  - criterion: "The bridge test binary builds ./cmd/evolve at most once for all its contract hosts"
    max_if_missing: 6
    evidence: "cd go && go test -tags integration -count=1 -run '^TestEvolveBuildRunsOnce$' ./internal/bridge"
  - criterion: "The contract doc names the single build and its guard"
    max_if_missing: 4
    evidence: "grep -q TestEvolveBuildRunsOnce docs/architecture/agent-sandbox-contract.md"
---

# Eval: Build the evolve binary once per bridge test binary

> The bridge sandbox contract tests ran `go build ./cmd/evolve` once per test, three builds for each test binary on macOS. `TestEvolveBuildRunsOnce` pins the build count at one or fewer. Source: docs/research/ci-wall-variance-2026-11.md section 6.1 (R5), cycle 2572.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| build-count | at most one build of ./cmd/evolve for three hosts | 6/10 | `go test -tags integration -run TestEvolveBuildRunsOnce ./internal/bridge` |
| docs | the contract doc names the guard | 4/10 | `grep -q TestEvolveBuildRunsOnce docs/architecture/agent-sandbox-contract.md` |
