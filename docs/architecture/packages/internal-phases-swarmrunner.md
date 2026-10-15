# internal/phases/swarmrunner

## Purpose

`internal/phases/swarmrunner` is the orchestrator seam of the swarm harness ([ADR-0032](../adr/0032-swarm-harness.md)). It is a decorator. It wraps any phase runner. When the swarm stage is active and the planner produced a partitionable plan, it runs the phase across N parallel workers. Otherwise it delegates to the wrapped runner. The composition root wraps the build (writer) and scout (reader) runners.

## Design

- **Stages.** `Config.Stage` comes from the policy. `off` and `shadow` delegate to the inner runner byte for byte. `advisory` runs the swarm live, but the inner runner stays authoritative, and the swarm outcome goes to the signals. `enforce` makes the swarm result the phase output.
- **Advisory runs the inner first.** The inner runner runs on the caller's context, so a long side dispatch cannot starve it of time. The swarm dispatch then runs on a detached context, and only its observations are annotated onto the response.
- **Safe fallback.** A missing, invalid or non-partitionable plan collapses to the inner runner, with N=1. A writer plan whose files are not provably disjoint does the same. An unknown stage word means off.
- **Writer refusal.** Once a valid plan exists, a writer swarm is refused under the advisory and enforce stages. Shadow maps to off, so it delegates. The decorator returns FAIL before any side effect. The error points at `swarm.stage=shadow` or at fleet lanes. The integration branch has no authoritative tree transfer or guard wiring yet.
- **Reader synthesis.** Each worker report folds into one synthesized report at the phase's canonical path. The order follows the plan, because the results are indexed by worker, so the order is deterministic and must not be sorted. A worker that exited 0 without a report appears as a gap in the synthesis, and it is never dropped silently.
- **Capabilities pass through.** The decorator forwards the persona, signals, host-effects and verifier wiring proofs to the inner runner. A decorator that hid them will make a wrapped runner look unwired or unselectable.
- **Env copy.** The worker environment is a fresh map on every call. The shared phase environment belongs to the orchestrator, and the bridge may store or mutate what it is given.
- **Kill guard.** `groupKiller` refuses process group 1 and below. Group 0 is the caller's own group, and group 1 is init. The tmux teardown lives in the swarm package, behind a seam. Unit tests never execute real tmux, because one earlier suite killed the session of the caller that ran it.
- **Open follow-up.** The guard-dependency symlinker is not wired into writer worktrees. It matters only for the opt-in live writer path, never for the default shadow delegate.

## Invariants

- Off and shadow are byte-identical to the inner runner. Pinned by `TestDecorator_ShadowDelegates` in `swarmrunner_test.go`.
- The decorator never narrows what it wraps. Pinned by `TestDecorator_ForwardsPersonaProber`, `TestSwarmDecorator_ForwardsSignalsWired`, `TestSwarmDecorator_ForwardsHostEffectsWired` and `TestSwarmDecorator_ForwardsContractVerifierWired`.
- Enforce makes the swarm authoritative. Pinned by `TestDecorator_EnforceReaderDrivesResult`, and the inner runner does not run in that path.
- A failed worker in enforce fails the phase. Pinned by `TestDecorator_EnforceWriter_WorkerFailureReturnsFail` and `TestDecorator_EnforceReader_WorkerFailureReturnsFail`.
- A worker that produced no report appears as a gap. Pinned by `TestDecorator_EnforceReaderMarksMissingArtifact`.
- Worker tokens sum into the merged result. Pinned by `TestSwarmMerge_SumsWorkerTokens` and `TestSwarmMerge_LauncherCarriesTokens`.
- `mergeEnv` never aliases its inputs. Pinned by its test in `swarmrunner_test.go`.
