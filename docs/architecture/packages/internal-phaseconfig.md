# internal/phaseconfig

## Purpose

`internal/phaseconfig` is the one unit that describes a pipeline phase. It holds the identity, I/O, classification and routing (from `phasespec.PhaseSpec`). It also holds the dispatch configuration (cli, model, tools, sandbox, persona) and the swarm worker count. The routing advisor emits it when it mints a phase.

## Design

- **Option B.** The unified `PhaseConfig` type is the emission and API shape. The on-disk `PhaseSpec`, `profiles.Profile` and agent prompt loaders stay as they are. `Spec`, `ToProfile` and `PromptBody` rebuild them from a `PhaseConfig`. The runner, `cli_chain`, `resolvellm` and the existing profiles do not change.
- **Layering.** The package imports `phasespec`, `profiles` and the standard library only. It must not import core, runner or llmroute. Those packages consume a `PhaseConfig`, never the reverse.
- **`Dispatch`.** It is the subset of `profiles.Profile` that the dispatch path reads, without the fields that `PhaseSpec` already carries. It reuses the `profiles` value types, so `ToProfile` is a direct field copy.
- **`ToProfile` is partial.** Profile fields that `Dispatch` does not model take their zero value (`MaxTurns`, `ParallelEligible`, `OutputArtifact`, `ResearchQuota`, `ModelTierOverrides`, `AddDir`). The zero values are safe for a minted phase: `ParallelEligible=false` keeps the single-writer invariant, and `specrunner` derives `OutputArtifact` from the spec outputs. The Registrar overlays the result onto a template profile when a phase needs another value. `ToProfile` alone is not a complete profile.
- **Profile name.** `ProfileName` is the agent name without the `evolve-` prefix, which is the runner derivation of `profileName`. `Role` mirrors it because `Dispatch` has no role field.
- **`Dispatch.SystemPrompt`.** The persona is inline. A minted phase has no `system_prompt_file` on disk.
- **`SwarmWorkers`.** Zero uses the global default (`EVOLVE_SWARM_CONCURRENCY`). A value above one fans the phase out to that many workers, and writers then need disjoint file ownership. It is the per-phase knob that the global flag did not give.
- **`Prompt`.** An empty value means the loader reads `agents/<agent>.md` by name. A non-empty value lets a minted phase ship its prompt without a file.
- **`Dispatch.ModelTierDefault`.** It is the tier when `PhaseSpec.Model` is auto or unset. Empty lets `resolvellm` derive the tier from the model hint.

## Invariants

- `Load` fails on a missing or empty name, because a phase with no identity cannot be routed. A parse error names the phase and the path. Pinned by the tests in `phaseconfig_test.go`.
- `Spec` returns the embedded value unchanged. Pinned by the `reflect.DeepEqual` test.
- The white-box tests of this package cannot import `go/test/fixtures`. The fixtures import `internal/core`, which imports `phaseconfig`, so the import makes a cycle. The local `writeCfg` helper is the replacement.

## Findings

- None open.
