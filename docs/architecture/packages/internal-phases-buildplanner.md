# internal/phases/buildplanner

## Purpose

`internal/phases/buildplanner` is the optional build-planner phase (Opt C). It moves the Builder's step 3, the design step, into a separate Opus session. The Builder then spends its 25-turn budget on the implementation and not on planning.

## Design

- **Rollout in three cycles.** Shadow (shipped): `EVOLVE_BUILD_PLANNER=0` by default, the phase is wired but always skipped, and the Go constant, state-machine edges and profile exist. Advisory: the default flips to 1, `build-plan.md` is produced, and the Builder reads it in its step 2.8. Enforce: the Builder's internal step 3 is removed and the Auditor checks adherence to `build-plan.md`.
- **Skip.** `ShouldSkip` delegates to the central `PhasePolicy`; `config.Load` is the only reader of `EVOLVE_BUILD_PLANNER`. The phase is opt-in: skipped unless the flag enables it. `Config.Bridge` and `Config.Prompts` can be nil in shadow mode because `ShouldSkip` returns before either is used.
- **Optional.** The phase is advisory. If its artifact never appears (`ExitArtifactTimeout`, for example during a quota exhaustion), the phase degrades to WARN. The cycle advances to build and does not abort.
- **Default model.** Opus, an independent LLM session from the Builder's Sonnet, to keep the anti-cooperative-bias separation.
- **`Classify`.** It checks that the required section headings are present in `build-plan.md`. During the advisory stage a missing heading is WARN, not FAIL. Enforcement is the third stage.

## Invariants

- **Shadow mode skips.** Pinned by `TestShouldSkip_ShadowMode`.
- **The concrete `*Phase` satisfies `runner.Hooks` and `runner.Skipper`, and `BaseRunner` reports the `build-planner` name.** Pinned by `TestPhase_BaseRunnerNamedRunner`.
- **The contract verifier is one verifier for the gate and the verdict engine** (`Config.ContractVerifier`).
