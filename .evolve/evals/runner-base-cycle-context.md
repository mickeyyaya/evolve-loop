---
score_cap:
  - criterion: "runner.BaseCycleContext emits the byte-identical 4-field core Cycle Context block"
    max_if_missing: 6
    evidence: "cd go && go test -run TestBaseCycleContext ./internal/phases/runner/"
  - criterion: "Migrated callers compose prompts as BaseCycleContext + phase-specific extras (projection equivalence)"
    max_if_missing: 6
    evidence: "cd go && go test -run TestComposePromptParity ./internal/phases/tdd/"
  - criterion: "No phase outside runner/ (retro excluded by design) re-builds the core block"
    max_if_missing: 7
    evidence: "test \"$(grep -rn -- '- cycle: %d' go/internal/phases/ --include='*.go' | grep -v '/runner/' | grep -v '/retro/' | grep -v '_test.go' | wc -l | tr -d ' ')\" = \"0\""
---

# Eval: Extract shared BaseCycleContext helper (single-source core prompt block)

> Pins the cycle-249 DRY refactor: the "## Cycle Context" core block (cycle,
> goal_hash, project_root, workspace) lives ONLY in
> `runner.BaseCycleContext`; the 10 phase files that previously copy-pasted
> it delegate and append only their unique extras (worktree, goal, mode,
> carryover_summary). retro/ is deliberately excluded — its block carries
> `previous_verdict` instead of `goal_hash`, so it is a different
> projection, not a copy. Source incident: cycle 249 scout Finding 1
> (10-file duplication; any block change required 10 coordinated edits).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| byte-parity | Helper output identical to the pre-refactor block | 6/10 | `go test -run TestBaseCycleContext ./internal/phases/runner/` |
| projection-equivalence | Caller prompt == base + extras | 6/10 | `go test -run TestComposePromptParity ./internal/phases/tdd/` |
| zero-remaining-copies | `- cycle: %d` marker absent outside runner/ & retro/ | 7/10 | marker grep returns 0 |
