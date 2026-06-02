# Step 9 — Remove `llm_config.json` (migration plan)

> **Status:** **9a + 9b DONE (2026-06-02)** — `llm_config.json` is fully removed.
> 9a removed it from the dispatch resolution path; 9b removed the residual
> operator surfaces AND repointed `/setup` onboarding at `.evolve/policy.json`
> `pins` (the durable per-phase override layer) with `evolve setup detect`
> overlaying + validating pins via `policy.ValidatePin`. The standalone
> `evolve setup validate` subcommand is gone. Split because 9a was the
> load-bearing behavior change and was coherent + tested on its own.
>
> **9b shipped:** removed `setup.Validate` + the `evolve setup validate`
> subcommand + `--config`, `paths.LLMConfigFile` + `EVOLVE_LLM_CONFIG_PATH`, the
> `subagent` `LLMConfigPath` plumbing + the `ResolveLLM(agent, configPath)` seam
> param, `examples/llm_config.example.json`, the stale `llm_config` comments in
> `llmroute.go`/`runner.go`, `setup.PhaseStatus.CurrentModel` + the dead
> `DetectOptions.ConfigPath`, and the `acs/cycle-52`+`acs/cycle-53` llm_config
> predicates. New: `setup.Detect` is policy-pin-aware (`source=policy-pin`,
> `pin_violation`, `policy_error`); `/setup` SKILL + setup-onboarding.md rewritten.
>
> **9a (done):** `resolvellm.Resolve` collapsed to profile-only — the
> `llm_config.phases`/`_fallback` branches + the `ConfigPath` option + the
> `llmConfig`/`phaseEntry` types are gone. Profiles (+ policy pins upstream) own
> the CLI; the Step-10c catalog overlay owns tier→model. Consumers updated:
> `setup.Detect`, `cmd resolve-llm` (dropped the `[config_path]` arg),
> `subagent.defaultResolveLLM` (ignores the now-dead configPath). New
> `TestResolve_IgnoresLLMConfig` proves a present llm_config.json is ignored.
>
> **9b (remaining, follow-up):** the residual non-dispatch surfaces — remove
> `setup.Validate`'s llm_config clamp + the `evolve setup validate` subcommand's
> `--config`/default, `paths.LLMConfigFile`, the dead `subagent` `LLMConfigPath`
> plumbing (`run.go`/`validateprofile.go` field + `ResolveLLM(agent, configPath)`
> seam), `examples/llm_config.example.json`, the stale `llm_config` comments in
> `llmroute.go`/`runner.go`, the now-always-empty `setup.PhaseStatus.CurrentModel`
> report field, and the stale `acs/cycle52`+`acs/cycle53` predicates (which still
> assert `source=llm_config`; acs/ is excluded from CI so they don't fail the
> gate, but they test removed behavior). None is load-bearing for dispatch.

## Goal

Eliminate `.evolve/llm_config.json` as a dispatch-config surface. After the
refactor, **profiles + `.evolve/policy.json` own the CLI**, and the **live
model-catalog (Step 10) owns `tier → model`**. This removes a redundant 5th
dispatch surface (env > policy-pin > profile > llm_config > default) and makes
the config story: *profile/policy decide CLI + tier; catalog resolves tier to a
concrete model.*

This reverses the *original* Step 9 ("make `llm_config.cli` authoritative") —
that pointed the wrong way; the target is removal, not promotion.

## Current state (the oracle)

- **`resolvellm` precedence ladder** (`go/internal/resolvellm/resolvellm.go`):
  1. `llm_config.phases.<role>` → `source="llm_config"`
  2. `llm_config._fallback` → `source="llm_config_fallback"`
  3. profile `cli` + `model_tier_default` → `source="profile"`
  4. nothing usable → `ErrProfileNotFound`
  - **Oracle:** `resolvellm_test.go` already pins every branch (19 tests). These
    are the characterization tests the migration must keep green (or migrate
    deliberately, branch by branch). No new golden tests needed — they exist.
- **`llm_config.cli` is already INERT for dispatch.** The runner's CLI chain is
  `EVOLVE_<AGENT>_CLI > EVOLVE_CLI > profile.cli > default` (`llmroute.go`);
  `resolvellm`'s CLI result is consulted only for the legacy `cmd_resolve_llm`
  surface and `setup.Validate`. So removing the CLI role is low-risk.
- **Only the model/tier auto-expansion is load-bearing** — `resolvellm` expands
  the `"auto"` model sentinel via `llm_config.phases.<role>` (model or tier).
- **No real `.evolve/llm_config.json` is shipped** — only
  `examples/llm_config.example.json`. In the default repo, `resolvellm` always
  falls through to profile (step 3). The `llm_config` branches are dormant
  unless an operator authors the file. This makes the removal *mostly*
  dormant-path cleanup — but operators who DID author one will see a behavior
  change, so it is not a pure refactor.

## Target

- CLI: `EVOLVE_<AGENT>_CLI > EVOLVE_CLI > policy-pin > profile.cli > default`
  (already the live chain minus the dead llm_config consult).
- Model: `EVOLVE_<AGENT>_MODEL` override > catalog `DispatchModel(cli, tier)`
  (live-source-gated) > profile `model_tier_default` > manifest default.
  Catalog API already exists: `modelcatalog.Catalog.DispatchModel(cli, tier)
  (model, ok)` — `ok` only when the entry is live-sourced (the 10c safety gate).

## Blast radius (9 files)

| File | Role |
|---|---|
| `internal/resolvellm/resolvellm.go` | the reader — collapses to profile-only (or is deleted) |
| `internal/resolvellm/resolvellm_test.go` | the oracle — migrate branch tests deliberately |
| `internal/llmroute/llmroute.go` | AutoModel seam — point auto-expansion at the catalog |
| `internal/phases/runner/runner.go` | consumes resolvellm via the AutoModel seam |
| `internal/setup/setup.go` | `Validate` reads `llm_config.json` — drop or repoint |
| `internal/paths/paths.go` | `LLMConfigFile` constant — remove |
| `internal/subagent/run.go`, `validateprofile.go` | resolvellm consumers |
| `cmd/evolve/cmd_resolve_llm.go`, `cmd_setup.go` | CLI surfaces referencing llm_config |
| `examples/llm_config.example.json` | delete; update docs |

## Migration order (behavior-preserving where possible)

1. **Repoint auto-expansion to the catalog** behind the existing `AutoModel`
   seam in `llmroute`/`runner`: when model is `"auto"`, resolve via
   `catalog.DispatchModel` first, fall back to the *current* resolvellm path.
   Additive — no removal yet. Soak this (the catalog must actually cover the
   ready CLIs' tiers — see prerequisite).
2. **Drop the `llm_config.cli` consult** from `resolvellm` (it is already inert
   for the runner; only `cmd_resolve_llm`/`setup.Validate` observe it). Keep
   profile fallback. Migrate the affected oracle tests.
3. **Collapse `resolvellm`** to profile-only resolution (or delete it and inline
   the profile read), once nothing reads `llm_config`.
4. **Remove** `paths.LLMConfigFile`, `setup.Validate`'s llm_config read,
   `examples/llm_config.example.json`, and doc references.
5. Update `cmd_resolve_llm` / `cmd_setup` surfaces.

## Behavior-preservation invariants (must NOT change)

- `EVOLVE_<AGENT>_MODEL` / `EVOLVE_<AGENT>_CLI` overrides always win.
- Profile `model_tier_default` fallback and the `balanced` default tier.
- A phase with no resolvable model still gets a deterministic default (never
  empty → never "launch model named ''").

## Prerequisite: catalog coverage (RUNTIME check, not a unit test)

The catalog is **live-populated** (`evolve models refresh --source live`); in
CI it is empty, so coverage can't be asserted by a unit test. Before flipping
step 1 to authoritative, verify at runtime that `DispatchModel` returns `ok`
for every ready CLI × canonical tier (`evolve models list`). Until then, the
fall-back-to-resolvellm path in step 1 keeps dispatch safe.

## Rollout / rollback

- Step 1 is additive + fall-back-safe; soak ≥1 cycle.
- Steps 2–5 are the removal; gate behind a 1-cycle soak each, each its own PR.
- Rollback: revert the PR; `examples/llm_config.example.json` + resolvellm carry
  no state, so revert is clean.

## Risk

The autonomous cycles actively merge into `runner`/`config`/dispatch. Rebase
onto latest `main` immediately before each step and re-run the resolvellm +
runner suites; a dispatch regression affects every phase.
