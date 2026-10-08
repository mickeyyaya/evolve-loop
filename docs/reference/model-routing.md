# Model Routing — Tier Definitions, Provider Mappings & Dynamic Routing

This page describes the routing of v22.27.0. The bash-era tier abstraction (`tier-1`, `tier-2`, `tier-3` and `.evolve/models.json`) is removed, and no code reads `.evolve/models.json`. The full reference is the "CLI routing table" row of [runtime-reference.md](../operations/runtime-reference.md).

## Tier Definitions

Each profile in `.evolve/profiles/` names a default tier and an envelope (`model_tier_default`, `model_tier_envelope`). The tier ladder has four steps: `fast`, `balanced`, `deep` and `top`. The manifest of each CLI family maps a tier to a model (`model_tier_map` in `go/internal/bridge/manifests/<driver>.json`).

## Models for each tier

| Tier | `claude-tmux` | `agy-tmux` | `agy-claude-tmux` | `codex-tmux` |
|---|---|---|---|---|
| `fast` | haiku | Gemini 3.8 Flash (Low) | Claude Sonnet 5.5 (Low) | gpt-5.6-luna |
| `balanced` | sonnet | Gemini 3.8 Flash (High) | Claude Sonnet 5.5 (High) | gpt-5.6-terra |
| `deep` | opus | Gemini 3.1 Pro (High) | Claude Opus 5.5 (High) | gpt-5.6-sol |
| `top` | opus | Gemini 3.1 Pro (High) | Claude Opus 5.5 (High) | gpt-5.6-sol |

In Claude Code 2.1.293, the `haiku` alias of the `fast` tier resolves to Haiku 5.5 (checked on 2026-10-08). agy has no Haiku model, so the `fast` tier of `agy-claude-tmux` is Claude Sonnet 5.5 (Low).

`evolve models refresh` refreshes the live model catalog. The resolution order is: a policy pin, then the live catalog, then the manifest baseline.

## The CLI routing table

Since v22.27.0, one table in `.evolve/policy.json` (`cli_routing`) decides which CLI runs each dispatch ([ADR-0119](../architecture/adr/0119-one-routing-table-one-resolver.md)). The table of this repository has these rules:

- A phase below the deep tier runs agy first, at the `balanced` tier (see the table above). Then it runs agy-owned Claude (`agy-claude`), then Claude Code. `evolve cli-routing explain <agent>` prints the chain and the model at the tier of the agent.
- A phase at the deep or the top tier runs agy-owned Claude first, then Claude Code.
- The Claude-family floor agents (the auditor, the adversarial review, the tdd engineer, the code reviewer and the spec verifiers) run Claude Code only.
- Codex runs only under `--bypass-policy`.

A project without a table uses the chain of each profile (`cli`, then `cli_fallback`). Then the loop probes each CLI, skips a benched CLI and falls back to an installed CLI.

## Commands

| Command | What it does |
|---|---|
| `evolve cli-routing init --clis a,b,c` | Writes `clis` and `default`, and moves the legacy keys into the table |
| `evolve cli-routing set <key> <a,b>` | Sets one key: `clis`, `default`, `after_chain`, `tiers.<tier>`, `work.<role>` or `agents.<agent>` |
| `evolve cli-routing unset <key>` | Removes one key |
| `evolve cli-routing migrate [--dry-run]` | Moves `pins`, `workflow.universal_fallback` and `router.cli/model` into the table. It refuses a non-empty `workflow.universal_fallback_exclude` |
| `evolve cli-routing show`, `check`, `explain <agent>` | Prints the table, checks it, or prints the chain of one agent |

These verbs are the only writers of the table. Each write compiles the merged file first. A write is refused inside a phase and while a cycle holds a live lease. `evolve setup recommend` and `evolve setup apply` refuse when the policy file has a `cli_routing` key.

## Dynamic Model Routing

The advisor chooses the tier of each phase inside the envelope of its profile. This is the default (`EVOLVE_DYNAMIC_ROUTING=advisory`). `EVOLVE_DYNAMIC_ROUTING=off` keeps the static profile tiers. The advisor uses the chain of the table, from the first CLI that is not benched.
