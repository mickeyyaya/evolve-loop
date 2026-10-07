# Pluggability — Every Phase Swappable, Every LLM Routable

> Evolve Loop has three independent axes of pluggability: personas (the "who"), skills (the "how"), and LLMs (the "model running the persona").
> The CLI router lets you assign different LLMs to different phases for each cycle. You do this declaratively, in one config file.
> This makes the framework portable across providers and budget profiles.
> Audience: a person who decides if they adopt evolve-loop, or who wants to mix models to optimize cost and quality. This document assumes that you know [overview.md](overview.md).

## Table of Contents

1. [Three Axes of Pluggability](#three-axes-of-pluggability)
2. [Persona Pluggability](#persona-pluggability)
3. [Skill Pluggability](#skill-pluggability)
4. [LLM Pluggability (the CLI Router)](#llm-pluggability-the-cli-router)
5. [Resolution Precedence](#resolution-precedence)
6. [Example Configurations](#example-configurations)
7. [Verifying What Actually Ran](#verifying-what-actually-ran)
8. [Adding a New CLI Adapter](#adding-a-new-cli-adapter)
9. [Caveats and Limitations](#caveats-and-limitations)
10. [References](#references)

---

## Three Axes of Pluggability

The framework keeps three things separate: *what work occurs*, *who does it*, and *what model runs the who*. These are independent dimensions:

| Axis | What is pluggable | File location | Example |
|---|---|---|---|
| **Persona** | The role definition of the agent (prompt, output format, perspective) | `agents/<role>.md` | Swap `evolve-scout.md` for a domain-specific scout |
| **Skill** | The workflow steps inside a persona | `skills/<name>/SKILL.md` | Replace the `evolve-tdd` skill with a property-based-test skill |
| **LLM** | The model + CLI that drives the persona | `.evolve/llm_config.json` | Route Scout to gemini-3.1-pro, Builder to claude-sonnet, Auditor to claude-opus |

You can change one axis and not the others. When you swap the LLM that drives Scout, you do NOT need a new persona file.
When you swap the Scout persona, you do NOT need a new skill.
The tri-layer separation ([tri-layer.md](../architecture/tri-layer.md)) makes this composable.

---

## Persona Pluggability

Each phase has a persona. A persona is a markdown file in `agents/` that declares:
- The perspective of the role (single-purpose: "scout finds work", "auditor adversarially verifies")
- The output format of the role (a specific artifact path, a structure, and a challenge token)
- The tools that the role can use (Read/Write/Bash subsets, through `.evolve/profiles/<role>.json`)

To swap a persona:

1. Write a replacement at `agents/<your-role>.md` (or fork a current persona).
2. Register it in `.claude-plugin/plugin.json:components.agents[]`.
3. Update the orchestrator persona OR the skill that calls it, so that it points at your role.

The kernel does not care which persona ran. It only verifies the profile permissions of the persona and the SHA of the artifact against the ledger.
A custom Scout can read from JIRA instead of `state.json:carryoverTodos[]`.
It works identically to the stock Scout, on the condition that it makes a valid `scout-report.md` with the challenge token.

**Constraint:** the role must still obey the trust kernel. Custom personas inherit the Tier 1 enforcement (phase-gate, role-gate, ship-gate).
You cannot write a "Scout that also commits to main": ship-gate denies the commit before the command runs.

---

## Skill Pluggability

The workflow of a persona is in `skills/<name>/SKILL.md`. The skill is the imperative recipe: steps, exit criteria and checklists.
More than one persona can share a skill. One persona can call more than one skill.

evolve-loop ships these skills:

| Skill | What it does | Used by |
|---|---|---|
| `evolve-scout` | Discovery workflow that reads the carryover and the instincts | scout persona |
| `evolve-triage` | Cycle-scope bouncer | triage persona |
| `evolve-tdd` | Predicate-first TDD per ADR-7 | builder persona (when EGPS) |
| `evolve-build` | Implementation workflow | builder persona |
| `evolve-audit` | Adversarial-mode audit | auditor persona |
| `evolve-ship` | Pre-flight + ship.sh invocation | orchestrator persona |
| `evolve-retro` | Retrospective lesson extraction | retrospective persona |
| `evolve-memo` | Carryover capture | memo persona |
| `evolve-loop` | The macro: orchestration of the full lifecycle | `/evo:loop` command |

To swap a skill:

1. Write a replacement at `skills/<your-skill>/SKILL.md`.
2. Register it in `plugin.json:components.skills[]`.
3. Update the persona/command that names the skill.

**Composition pattern:** you can change only the approach that the audit uses to collect evidence, and keep the rest of the lifecycle.
To do this, fork `evolve-audit` to `your-domain-audit`, edit the SKILL.md, and update `agents/evolve-auditor.md` to call your skill.
The framework stays intact.

---

## LLM Pluggability (the CLI Router)

This axis makes evolve-loop different from single-vendor agent frameworks. **Every phase declares which CLI + model runs it. Operators override this through `.evolve/llm_config.json`.**

### The router script

`legacy/scripts/dispatch/resolve-llm.sh <role> [config_path]` is a pure function:

```bash
$ bash legacy/scripts/dispatch/resolve-llm.sh scout
{"cli":"gemini","model":"gemini-3.1-pro-preview","source":"llm_config"}

$ bash legacy/scripts/dispatch/resolve-llm.sh builder
{"cli":"claude","model_tier":"sonnet","source":"llm_config_fallback"}

$ bash legacy/scripts/dispatch/resolve-llm.sh auditor
{"cli":"claude","model_tier":"sonnet","source":"profile"}
```

`subagent-run.sh` calls this script before every phase dispatch.
It uses the result to select the correct adapter (`legacy/scripts/cli_adapters/claude.sh`, `gemini.sh`, or `codex.sh`).

### The config file

`.evolve/llm_config.json` (gitignored — operator-local):

```json
{
  "schema_version": 1,
  "phases": {
    "scout":   {"provider": "google",    "cli": "gemini", "model": "gemini-3.1-pro-preview"},
    "builder": {"provider": "anthropic", "cli": "claude", "model_tier": "sonnet"},
    "auditor": {"provider": "anthropic", "cli": "claude", "model": "claude-opus-4-7"}
  },
  "_fallback": {
    "provider": "anthropic",
    "cli": "claude",
    "model_tier": "sonnet"
  }
}
```

- `phases.<role>` — the per-phase override. The resolver returns it verbatim.
- `_fallback` — the resolver uses it for each phase that is not listed. For example, triage, memo and retrospective inherit the fallback unless the file lists them explicitly.
- Absent file → every phase uses the default of its profile (backward-compat with v8.34-era cycles).

### Adapters

Three adapters ship today, each in `legacy/scripts/cli_adapters/`:

| Adapter | Mode | What it does |
|---|---|---|
| `claude.sh` | Native | Translates the profile JSON → `claude -p` argv with `--allowedTools` / `--disallowedTools` / `--max-budget-usd` / sandbox |
| `gemini.sh` | Native (v10.7+) | `gemini -p` + `-m <model>` + `--output-format json` + `--approval-mode yolo`. It translates the gemini stats → a claude-style usage envelope |
| `codex.sh` | Hybrid | Delegates to `claude.sh` when claude is on the PATH. Otherwise, it uses a same-session degraded mode |

Each adapter implements the same env-var contract (for example, `PROFILE_PATH`, `RESOLVED_MODEL`, `PROMPT_FILE`, `WORKSPACE_PATH`, `STDOUT_LOG`).
Each adapter writes the same translated usage envelope to STDOUT_LOG.
Thus, the upstream `subagent-run.sh` and the ledger do not need to know which CLI actually ran.

---

## Resolution Precedence

The router applies these rules in this order:

| Priority | Rule | Source string |
|---|---|---|
| 1 | `llm_config.phases.<role>` (exact match) | `llm_config` |
| 2 | `llm_config._fallback` | `llm_config_fallback` |
| 3 | Profile's `cli` + `model_tier_default` | `profile` |
| 4 | `llm_config.json` absent | `profile` (backward-compat) |

The `source` field appears in `cli_resolution.source` in every ledger entry. Thus, after the cycle, you can verify exactly which rule fired for which phase.
The cycle 61 incident showed this problem as B6 (the orchestrator narrative did not disclose the actual routing).
The CLI Resolution renderer of cycle 63 makes the routing visible, byte-stable, in `orchestrator-report.md`.
See the Tier 3 section of [trust-architecture.md](trust-architecture.md).

---

## Example Configurations

### Config A — Cost-optimized (Haiku everywhere)

```json
{
  "schema_version": 1,
  "_fallback": {"provider": "anthropic", "cli": "claude", "model_tier": "haiku"}
}
```

~$0.15-0.30 per cycle. The quality is acceptable for small refactors. The accuracy decreases on complex tasks.

### Config B — Quality-optimized (Opus for Scout/Audit, Sonnet for Builder)

```json
{
  "schema_version": 1,
  "phases": {
    "scout":    {"cli": "claude", "model": "claude-opus-4-7"},
    "builder":  {"cli": "claude", "model_tier": "sonnet"},
    "auditor":  {"cli": "claude", "model": "claude-opus-4-7"},
    "retrospective": {"cli": "claude", "model": "claude-opus-4-7"}
  },
  "_fallback": {"cli": "claude", "model_tier": "sonnet"}
}
```

~$2-5 per cycle. This gives the maximum reasoning quality on the discovery + audit phases. Builder stays on Sonnet (the best coding model).

### Config C — Cross-vendor for adversarial audit

```json
{
  "schema_version": 1,
  "phases": {
    "scout":    {"cli": "gemini", "model": "gemini-3.1-pro-preview"},
    "builder":  {"cli": "claude", "model_tier": "sonnet"},
    "auditor":  {"cli": "claude", "model": "claude-opus-4-7"}
  },
  "_fallback": {"cli": "claude", "model_tier": "sonnet"}
}
```

This is the adversarial-mode default: the Auditor is on a **different model family from Builder**, to stop the sycophancy of a same-model judge.
Scout is on Gemini, because Gemini has better long-context retrieval. ~$1.50-3.50 per cycle.

### Config D — Gemini-only (no Claude in the loop)

```json
{
  "schema_version": 1,
  "phases": {
    "scout":    {"cli": "gemini", "model": "gemini-3.1-pro-preview"},
    "builder":  {"cli": "gemini", "model": "gemini-3.1-pro-preview"},
    "auditor":  {"cli": "gemini", "model": "gemini-3.1-pro-preview"}
  },
  "_fallback": {"cli": "gemini", "model": "gemini-3.1-pro-preview"}
}
```

This configuration bypasses Anthropic entirely. It works if your subscription/API of choice is Google AI.
The cycle-61/64 experiments validated this configuration end-to-end.
There are caveats: the Gemini CLI has different workspace restrictions, and its tool-write behavior has some quirks. See [`../incidents/cycle-61.md`](../incidents/cycle-61.md).

### Config E — Mixed by cost-per-phase

```json
{
  "schema_version": 1,
  "phases": {
    "scout":         {"cli": "claude", "model_tier": "haiku"},
    "triage":        {"cli": "claude", "model_tier": "haiku"},
    "builder":       {"cli": "claude", "model_tier": "sonnet"},
    "auditor":       {"cli": "claude", "model": "claude-opus-4-7"},
    "memo":          {"cli": "claude", "model_tier": "haiku"},
    "retrospective": {"cli": "claude", "model_tier": "sonnet"}
  },
  "_fallback": {"cli": "claude", "model_tier": "haiku"}
}
```

Spend the budget where it is important: opus for audit (the hardest reasoning), sonnet for builder (the best coding), haiku for the rest (read-only roles). ~$0.50-1.50 per cycle.

---

## Verifying What Actually Ran

After a cycle, the auto-rendered `## CLI Resolution` section in `orchestrator-report.md` is the source of truth:

```
## CLI Resolution

_Auto-rendered from `.evolve/ledger.jsonl` by `legacy/scripts/observability/render-cli-resolution.sh 62`. Do NOT edit manually._

| Phase | Actual CLI | Actual Model | Source | Mode |
|-------|------------|--------------|--------|------|
| scout         | gemini | gemini-3.1-pro-preview | llm_config           | hybrid |
| triage        | claude | sonnet                  | llm_config_fallback  | full   |
| builder       | gemini | gemini-3.1-pro-preview | llm_config           | hybrid |
| auditor       | claude | sonnet                  | llm_config_fallback  | full   |
| memo          | claude | sonnet                  | llm_config_fallback  | full   |
| orchestrator  | claude | sonnet                  | llm_config_fallback  | full   |
```

The renderer makes this section from the `state.json:ledger` entries.
It does not use the operator config (which can drift) or the orchestrator narrative (which can hallucinate).
The ledger entries record `cli_resolution.source`, which shows the rule that fired (`llm_config`, `llm_config_fallback`, or `profile`).
Thus, you can confirm that the fallbacks occurred when you expected them.

To render the section manually after the cycle:

```bash
bash legacy/scripts/observability/render-cli-resolution.sh <cycle>
```

---

## Adding a New CLI Adapter

To support a new CLI (for example, a hypothetical `mistral-cli`):

1. Create `legacy/scripts/cli_adapters/mistral.sh`. It must match the current adapter contract (env vars in / translated usage envelope out).
2. Create `legacy/scripts/cli_adapters/mistral.capabilities.json`. It describes what the CLI supports:
   ```json
   {
     "adapter": "mistral",
     "version": 1,
     "supports": {
       "budget_cap_native": true,
       "permission_scoping": false,
       "sandbox_native": false,
       "non_interactive_prompt": true,
       "model_flag": true
     }
   }
   ```
3. Add `"mistral"` as a valid `cli` value in the allowlist of `legacy/scripts/dispatch/resolve-llm.sh`.
4. Add a test in `legacy/scripts/tests/mistral-adapter-test.sh`.
5. Write a predicate at `acs/cycle-N/NNN-mistral-native-mode.sh`. It must prove that the adapter calls the binary correctly.

The v8.32-v10.7 Gemini support followed exactly this path. The router does not care which CLIs are registered. It only enforces the contract.

---

## Caveats and Limitations

| Limitation | Why | Mitigation |
|---|---|---|
| Gemini's native mode has workspace restrictions | The `gemini` CLI refuses writes outside the allowed workspace directories | Use `--include-directories <worktree>` (the adapter does this automatically in v10.7+) |
| Cross-CLI cost reporting is approximate | The Gemini JSON does not include `cost_usd` (only tokens). The adapter calculates the cost with a hardcoded price table, which can drift | Update the prices in `gemini.sh` periodically. Use `gemini_translate_error: true` in the usage envelope as a drift signal |
| Codex adapter is hybrid-only | As of v10.7, the Codex CLI has no non-interactive prompt mode | If you must run codex natively, file an issue. The path is the same as the path that gemini followed |
| Per-phase model overrides do not compose with `EVOLVE_TASK_MODE` budget tiers | The tier resolution occurs after the CLI resolution. The budget tiers are per CLI profile | Set the budget caps with `EVOLVE_MAX_BUDGET_USD` instead |
| Auditor's "different model family" assumption | If you set Builder=Sonnet and Auditor=Sonnet in `llm_config.json`, the adversarial-audit framing becomes weaker | Use Config C above. Or, if you deliberately run a permissive sweep, set `ADVERSARIAL_AUDIT=0` |
| Memo profile shell-tool restrictions (cycle 62 B4 fix) | The fix removed `Bash(cat:*)`, `Bash(head:*)`, `Bash(tail:*)` from the memo allowlist. If a custom memo persona needs these tools, it must use Read instead | Use the Read tool to examine files |

---

## References

| Source | Relevance |
|---|---|
| [`../architecture/platform-compatibility.md`](../architecture/platform-compatibility.md) | Top-level CLI support matrix + adapter contract |
| [`../architecture/capability-schema.md`](../architecture/capability-schema.md) | Schema for `*.capabilities.json` files |
| [`../architecture/tri-layer.md`](../architecture/tri-layer.md) | Skill / Persona / Command separation |
| [trust-architecture.md](trust-architecture.md) Tier 3 §Adversarial Auditor | Why the default forbids same-model judges |
| [`../incidents/cycle-61.md`](../incidents/cycle-61.md) | Cycle 61/64 incidents: the gemini-3.1-pro-preview routing exposed real B0-B7 framework bugs |
| [`../incidents/gemini-forgery.md`](../incidents/gemini-forgery.md) | The v7.9.0+ structural defenses for non-claude CLIs (anti-forgery prompt, artifact content checks, .sh write protection) |
| ADR-1 (in `docs/adr/` or `docs/architecture/`) | The design rationale of the LLM router |
