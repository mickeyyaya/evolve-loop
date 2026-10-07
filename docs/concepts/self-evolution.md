# Self-evolution: durable lessons and bounded recall

The loop records failures and carries useful evidence into later plans and implementation. This is retrieval and adaptation. It is not a guarantee that mistakes never recur.

## Current data flow

1. Core failure learning and Retro write lesson YAML to `.evolve/instincts/lessons/`.
2. The file-backed knowledge base ranks lessons with deterministic matches on keywords and on failure context.
   It uses the recall bound of the policy.
3. The advisor receives the lessons that match the current goal and the latest failure. Scout, TDD, Build, and Audit
   receive task/goal-relevant digests from the same corpus through both fresh and resumed dispatch.
4. Phase prompts quote recalled text as untrusted data, retain source paths, and bound unusually long digests.
   A lesson is evidence to assess, never permission to weaken a gate.
5. Continuations and carryover preserve unfinished work and unresolved findings. Terminal records describe the actual outcome.

There is no persisted `state.json:instinctSummary` database in the Go runtime. The host projects recall at dispatch. Personas can also inspect the corpus on demand. If the corpus is not there, or if no lesson is relevant, the result is no lessons. Core exposes the retrieval failures that reach it as unavailable memory. The best-effort knowledge base can skip malformed individual YAML files.

## What counts as improvement

Prove that a persisted lesson reaches the intended next prompt. Then measure if comparable tasks recur less often. Lesson-file counts, prompt inclusion and a green helper test do not establish improved task delivery. Record separately the actual landings, the recurrence, the repair rounds and the outcome evidence that is not available.

Implementation: `go/internal/core/failure_learning.go`, `task_recall.go`, `routing_dispatch.go`, `go/internal/research/filekb.go`, and `go/internal/phases/runner/runner.go`. Regression: `TestDispatch_TaskRecallWithoutFailureHistory`, `TestAdvisorPlanInput_RecallsGoalWithoutHistory`, and `TestBaseCycleContext_RecalledLessonsAreQuotedData`.

See [the current runtime contract](../architecture/current-runtime-contract.md) and the [archived shell-era description](../private/research/archived-2026-09-09/concepts-self-evolution.md).
