# Comment history: `internal/phases/swarmplan`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d13

### `go/internal/phases/swarmplan/swarmplan.go:1` — above `package swarmplan`

```text
// Package swarmplan implements the optional swarm-plan phase: an LLM planner
// that partitions one phase's task into N independent worker assignments for the
// swarm harness (ADR-0032). It emits swarm-plan.md (a fenced JSON block parsed
// by internal/swarm.ParsePlan).
//
// Rollout (mirrors buildplanner's shadow→advisory→enforce ladder):
//   - v1 (this): SHADOW — EVOLVE_SWARM_STAGE default "shadow"/"off"; the phase
//     is wired and (when enabled) produces swarm-plan.md for inspection, but the
//     orchestrator does NOT yet dispatch a swarm — build/scout still run N=1.
//   - v3: ADVISORY — the dispatcher fans out workers; integration is computed
//     but compared, not authoritative.
//   - v4: ENFORCE — the merge-train integration result is the phase output.
//
// Shadow-mode invariant: ShouldSkip returns SKIPPED whenever the swarm stage is
// off, so Bridge/Prompts may be nil in Config.
```
