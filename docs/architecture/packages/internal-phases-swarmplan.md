# internal/phases/swarmplan

## Purpose

`internal/phases/swarmplan` is the optional swarm-plan phase: an LLM planner. It splits the task of one phase into N independent worker assignments for the swarm harness ([ADR-0032](../adr/0032-swarm-harness.md)). It writes `swarm-plan.md`, a fenced JSON block that `internal/swarm.ParsePlan` reads.

## Design

- **Rollout ladder** (mirrors `buildplanner`): shadow, then advisory, then enforce.
  - Advisory: the dispatcher fans out workers, and the integration is compared but not authoritative.
  - Enforce: the merge-train integration result is the phase output.
  - Shadow (shipped): the phase is wired and writes the plan for inspection when enabled. The orchestrator does not yet dispatch a swarm, so build and scout still run with N=1.
- **Skip.** `ShouldSkip` delegates to the central `PhasePolicy` (`config.Load` reads `EVOLVE_SWARM_STAGE`). In shadow or off the phase is skipped and the next phase runs single-writer. `Config.Bridge` and `Config.Prompts` can be nil because `ShouldSkip` returns before either is used.
- **Optional phase.** A missing artifact degrades to the single-writer fallback and does not abort the cycle.
- **`Classify` is non-blocking.** An empty artifact is FAIL (the planner produced nothing). Any non-empty plan passes. The real gate is `internal/swarm.Validate` at dispatch time, and a plan that cannot be partitioned is a legitimate "fall back to N=1" outcome, not a phase failure.
- **Default model.** Opus: partition design is high-value reasoning, and the session is independent of the workers' models.

## Invariants

- **Shadow skips by default.** Pinned by `TestPhase_ShouldSkip_DefaultShadow`.
- **The concrete `*Phase` satisfies `runner.Hooks` and `runner.Skipper`.** Pinned by `TestPhase_NamedSatisfiesSkipperContract`.
