# internal/resolvellm

## Purpose

`internal/resolvellm` resolves a phase role to its `{cli, model_tier}` dispatch info by reading the role's profile. It has no side effects.

## Design

- The profile is the only source (Step 9, [the llm-config removal](../step9-llm-config-removal.md)). Profiles, with policy pins applied upstream, own the CLI. The live model-catalog overlay on the manifest `ModelTierMap` (Step 10c) translates the abstract tier to a concrete model downstream. `Result` carries only the CLI and the tier, and `Source` is always `profile`.
- The tier is the canonical abstract tier: `fast`, `balanced` or `deep`. A profile with no `model_tier_default` gets `balanced`.
- The legacy values `haiku`, `sonnet` and `opus` stay accepted for one release. The fallback ladder of the realizer and the v1 shim of `parseManifest` accept them ([ADR-0022](../adr/0022-launch-intent-realizer.md)).
- `Resolve` searches the profile paths in order: plugin root (`EVOLVE_PLUGIN_ROOT`), project root (`EVOLVE_PROJECT_ROOT`), then the git root (`git rev-parse --show-toplevel`). The first existing profile wins. This mirrors `_find_profile()` of the shell script `resolve-llm.sh`. `Options.Env` stubs `os.Getenv`, and `ErrProfileNotFound` matches the exit code 1 of that script.
- `JSON` formats its output by hand rather than with `encoding/json`.
- The key order of the wire shape (`cli`, `model_tier`, `source`) is part of the contract that shell callers read.

## Invariants

- A profile that exists but cannot be read is an error, not `ErrProfileNotFound`.
- A profile's `llm_config.json` is ignored. Resolution comes solely from the profile. Before Step 9, the phase entry of `llm_config.json` won. Pinned by `TestResolve_IgnoresLLMConfig`.
- The order is plugin root, project root, git root. Pinned by `TestResolve_PluginRootFirst` and `TestResolve_EnvDrivenRoots`.
- A missing tier becomes `balanced`. Pinned by `TestResolve_ProfileDefaultsTierToBalanced`.
- A profile that exists but is a directory is a read error, not `ErrProfileNotFound`. Pinned by `TestResolve_ProfilePathIsDirectory_ReadError`.
- The key order of `JSON` is fixed (`cli`, `model_tier`, `source`). Pinned by the exact string in `TestResolve_ProfileFallback`.

## Findings

- None open.
