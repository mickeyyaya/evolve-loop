---
score_cap:
  - criterion: "consensus-dispatch reports a malformed policy.json instead of swallowing it"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestConsensusDispatch_'"
  - criterion: "cycle-health absolutizes a relative or empty EVOLVE_PROJECT_ROOT"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestCycleHealth_RelativeProjectRootIsAbsolutized$'"
  - criterion: "the signal-center count pin counts call expressions, not comment text"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^(TestCountCallExprs_|TestNilSignalCenterRootsPin_UsesTheASTCount$)'"
---

# Eval: cmd-evolve-silent-policy-and-count-pins

> Pins the silent policy default in consensus-dispatch, the relative project root in cycle-health (the cycle-119 class), the substring count pins, and the composition root's policy WARN going to os.Stderr instead of its console. Source: comment-reduction batch 44.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| swallowed-error | policy load error reported | 6/10 | `go test -run TestConsensusDispatch_` |
| relative-root | root absolutized | 6/10 | `go test -run TestCycleHealth_Relative...` |
| comment-hides-call | AST count | 6/10 | `go test -run TestCountCallExprs_` |
