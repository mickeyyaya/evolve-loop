# Self-evolution: durable lessons and bounded recall

The loop records failures and carries useful evidence into later planning and implementation. This is retrieval and adaptation, not a guarantee that mistakes never recur.

## Current data flow

1. Core failure learning and Retro write lesson YAML to `.evolve/instincts/lessons/`.
2. The file-backed knowledge base ranks lessons using deterministic keyword and failure-context matching, with the policy's recall bound.
3. The advisor receives matching lessons from the current goal and latest failure. Scout, TDD, Build, and Audit receive task/goal-relevant digests from the same corpus through both fresh and resumed dispatch.
4. Phase prompts quote recalled text as untrusted data, retain source paths, and bound unusually long digests. A lesson is evidence to assess, never permission to weaken a gate.
5. Continuations and carryover preserve unfinished work and unresolved findings. Terminal records describe the actual outcome.

There is no persisted `state.json:instinctSummary` database in the Go runtime. The host projects recall at dispatch; personas may also inspect the corpus on demand. Missing corpus or no relevant match yields no lessons. Retrieval failures that reach core are exposed as unavailable memory; malformed individual YAML files may be skipped by the best-effort knowledge base.

## What counts as improvement

Prove that a persisted lesson reaches the intended next prompt, then measure whether comparable tasks recur less often. Lesson-file counts, prompt inclusion, and a green helper test do not establish improved task delivery. Record actual landings, recurrence, repair rounds, and missing outcome evidence separately.

Implementation: `go/internal/core/failure_learning.go`, `task_recall.go`, `routing_dispatch.go`, `go/internal/research/filekb.go`, and `go/internal/phases/runner/runner.go`. Regression: `TestDispatch_TaskRecallWithoutFailureHistory`, `TestAdvisorPlanInput_RecallsGoalWithoutHistory`, and `TestBaseCycleContext_RecalledLessonsAreQuotedData`.

See [the current runtime contract](../architecture/current-runtime-contract.md) and the [archived shell-era description](../private/research/archived-2026-09-09/concepts-self-evolution.md).
