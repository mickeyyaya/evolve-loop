# Pluggability — Every Phase Swappable, Every LLM Routable

> Evolve Loop has three independent axes of pluggability: personas (the "who"), skills (the "how"), and LLMs (the "model running the persona"). The CLI router lets you assign different LLMs to different phases per cycle, declaratively, in one config file. This is what makes the framework portable across providers and budget profiles.
> Audience: anyone deciding whether to adopt evolve-loop, anyone wanting to mix models for cost/quality optimization. Assumes [overview.md](overview.md).

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

The framework separates *what work happens* from *who does it* from *what model runs the who*. These are independent dimensions:

| Axis | What is pluggable | File location | Example |
|---|---|---|---|
| **Persona** | The agent's role definition (prompt, output format, perspective) | `agents/<role>.md` | Swap `evolve-scout.md` for a domain-specific scout |
| **Skill** | The workflow steps inside a persona | `skills/<name>/SKILL.md` | Replace `evolve-tdd` skill with a property-based-test skill |
| **LLM** | The model + CLI driving the persona | `.evolve/llm_config.json` | Route Scout to gemini-3.1-pro, Builder to claude-sonnet, Auditor to claude-opus |

You can change one without changing the others. Swapping the LLM driving Scout does NOT require a new persona file. Swapping the Scout persona does NOT require a new skill. The tri-layer separation ([tri-layer.md](../architecture/tri-layer.md)) is what makes this composable.

---

## Persona Pluggability

Each phase has a persona — a markdown file in `agents/` declaring:
- The role's perspective (single-purpose: "scout finds work", "auditor adversarially verifies")
- The role's output format (a specific artifact path, structure, and challenge token)
- The role's tools allowed (Read/Write/Bash subsets via `.evolve/profiles/<role>.json`)

To swap a persona:

1. Author a replacement at `agents/<your-role>.md` (or fork an existing one)
2. Register it in `.claude-plugin/plugin.json:components.agents[]`
3. Update the orchestrator persona OR the skill that invokes it to point at your role

The kernel doesn't care which persona ran — it only verifies the persona's profile permissions and the artifact's SHA against the ledger. A custom Scout that reads from JIRA instead of `state.json:carryoverTodos[]` works identically to the stock Scout as long as it produces a valid `scout-report.md` with the challenge token.

**Constraint:** the role still must respect the trust kernel. Custom personas inherit Tier 1 enforcement (phase-gate, role-gate, ship-gate). You cannot author a "Scout that also commits to main" — ship-gate denies it before the command runs.

---

## Skill Pluggability

A persona's workflow lives in `skills/<name>/SKILL.md`. The skill is the imperative recipe: steps, exit criteria, checklists. Multiple personas can share a skill; one persona can invoke multiple skills.

evolve-loop ships these skills:

| Skill | What it does | Used by |
|---|---|---|
| `evolve-scout` | Discovery workflow with carryover + instinct reading | scout persona |
| `evolve-triage` | Cycle-scope bouncer | triage persona |
| `evolve-tdd` | Predicate-first TDD per ADR-7 | builder persona (when EGPS) |
| `evolve-build` | Implementation workflow | builder persona |
| `evolve-audit` | Adversarial-mode audit | auditor persona |
| `evolve-ship` | Pre-flight + ship.sh invocation | orchestrator persona |
| `evolve-retro` | Retrospective lesson extraction | retrospective persona |
| `evolve-memo` | Carryover capture | memo persona |
| `evolve-loop` | The macro: full lifecycle orchestration | `/evolve-loop` command |

To swap a skill:

1. Author a replacement at `skills/<your-skill>/SKILL.md`
2. Register in `plugin.json:components.skills[]`
3. Update the persona/command that names the skill

**Composition pattern:** if you only want to change the audit's evidence-gathering approach but keep the rest of the lifecycle, fork `evolve-audit` to `your-domain-audit`, edit the SKILL.md, and update `agents/evolve-auditor.md` to invoke yours. The framework remains intact.

---

## LLM Pluggability (the CLI Router)

This is the axis that distinguishes evolve-loop from single-vendor agent frameworks. **Every phase declares which CLI + model runs it. Operators override via `.evolve/llm_config.json`.**

### The router script

`scripts/dispatch/resolve-llm.sh <role> [config_path]` is a pure function:

```bash
$ bash scripts/dispatch/resolve-llm.sh scout
{"cli":"gemini","model":"gemini-3.1-pro-preview","source":"llm_config"}

$ bash scripts/dispatch/resolve-llm.sh builder
{"cli":"claude","model_tier":"sonnet","source":"llm_config_fallback"}

$ bash scripts/dispatch/resolve-llm.sh auditor
{"cli":"claude","model_tier":"sonnet","source":"profile"}
```

`subagent-run.sh` calls this before every phase dispatch and uses the result to pick the right adapter (`scripts/cli_adapters/claude.sh`, `gemini.sh`, or `codex.sh`).

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

- `phases.<role>` — per-phase override; the resolver returns this verbatim
- `_fallback` — used for any phase not listed (e.g., triage, memo, retrospective inherit fallback unless explicitly listed)
- Absent file → every phase uses the profile's default (backward-compat with v8.34-era cycles)

### Adapters

Three adapters ship today, each in `scripts/cli_adapters/`:

| Adapter | Mode | What it does |
|---|---|---|
| `claude.sh` | Native | Translates profile JSON → `claude -p` argv with `--allowedTools` / `--disallowedTools` / `--max-budget-usd` / sandbox |
| `gemini.sh` | Native (v10.7+) | `gemini -p` + `-m <model>` + `--output-format json` + `--approval-mode yolo`; translates gemini stats → claude-style usage envelope |
| `codex.sh` | Hybrid | Delegates to `claude.sh` when claude on PATH; same-session degraded mode otherwise |

Each adapter implements the same env-var contract (`PROFILE_PATH`, `RESOLVED_MODEL`, `PROMPT_FILE`, `WORKSPACE_PATH`, `STDOUT_LOG`, etc.) and emits the same translated usage envelope to STDOUT_LOG, so the upstream `subagent-run.sh` and the ledger don't need to know which CLI actually ran.

---

## Resolution Precedence

The router applies these rules in order:

| Priority | Rule | Source string |
|---|---|---|
| 1 | `llm_config.phases.<role>` (exact match) | `llm_config` |
| 2 | `llm_config._fallback` | `llm_config_fallback` |
| 3 | Profile's `cli` + `model_tier_default` | `profile` |
| 4 | `llm_config.json` absent | `profile` (backward-compat) |

The `source` field appears in `cli_resolution.source` in every ledger entry — so post-hoc you can verify exactly which rule fired for which phase. The cycle 61 incident exposed this as B6 (orchestrator narrative didn't disclose the actual routing); cycle 63's CLI Resolution renderer makes it byte-stable visible in `orchestrator-report.md`. See [trust-architecture.md](trust-architecture.md) Tier 3 section.

---

## Example Configurations

### Config A — Cost-optimized (Haiku everywhere)

```json
{
  "schema_version": 1,
  "_fallback": {"provider": "anthropic", "cli": "claude", "model_tier": "haiku"}
}
```

~$0.15-0.30 per cycle. Acceptable quality for small refactors; loses accuracy on complex tasks.

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

~$2-5 per cycle. Maximum reasoning quality on the discovery + audit phases. Builder stays on Sonnet (best coding model).

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

The adversarial-mode default — Auditor on a **different model family from Builder** to break same-model-judge sycophancy. Scout on Gemini for its better long-context retrieval. ~$1.50-3.50 per cycle.

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

Bypasses Anthropic entirely. Works if your subscription/API of choice is Google AI. The cycle-61/64 experiments validated this end-to-end (with caveats: Gemini's CLI has different workspace restrictions; some quirks in tool-write behavior). See [`../incidents/cycle-61.md`](../incidents/cycle-61.md).

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

Spend the budget where it matters: opus for audit (hardest reasoning), sonnet for builder (best coding), haiku for the rest (read-only roles). ~$0.50-1.50 per cycle.

---

## Verifying What Actually Ran

After a cycle, the auto-rendered `## CLI Resolution` section in `orchestrator-report.md` is the source of truth:

```
## CLI Resolution

_Auto-rendered from `.evolve/ledger.jsonl` by `scripts/observability/render-cli-resolution.sh 62`. Do NOT edit manually._

| Phase | Actual CLI | Actual Model | Source | Mode |
|-------|------------|--------------|--------|------|
| scout         | gemini | gemini-3.1-pro-preview | llm_config           | hybrid |
| triage        | claude | sonnet                  | llm_config_fallback  | full   |
| builder       | gemini | gemini-3.1-pro-preview | llm_config           | hybrid |
| auditor       | claude | sonnet                  | llm_config_fallback  | full   |
| memo          | claude | sonnet                  | llm_config_fallback  | full   |
| orchestrator  | claude | sonnet                  | llm_config_fallback  | full   |
```

This is generated from `state.json:ledger` entries — not from operator config (which could drift) or orchestrator narrative (which could hallucinate). The ledger entries record `cli_resolution.source` showing which rule fired (`llm_config`, `llm_config_fallback`, or `profile`), so you can confirm fallbacks happened when you expected.

Render manually post-hoc:

```bash
bash scripts/observability/render-cli-resolution.sh <cycle>
```

---

## Adding a New CLI Adapter

To support a new CLI (e.g., a hypothetical `mistral-cli`):

1. Create `scripts/cli_adapters/mistral.sh` matching the existing adapter contract (env vars in / translated usage envelope out)
2. Create `scripts/cli_adapters/mistral.capabilities.json` describing what the CLI supports:
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
3. Add `"mistral"` as a valid `cli` value in `scripts/dispatch/resolve-llm.sh` allowlist
4. Add a test in `scripts/tests/mistral-adapter-test.sh`
5. Write a predicate at `acs/cycle-N/NNN-mistral-native-mode.sh` proving the adapter invokes the binary correctly

This is exactly the path the v8.32-v10.7 Gemini support followed. The router doesn't care which CLIs are registered — it only enforces the contract.

---

## Caveats and Limitations

| Limitation | Why | Mitigation |
|---|---|---|
| Gemini's native mode has workspace restrictions | `gemini` CLI refuses writes outside the allowed workspace directories | Use `--include-directories <worktree>` (the adapter does this automatically v10.7+) |
| Cross-CLI cost reporting is approximate | Gemini's JSON doesn't include `cost_usd` (only tokens); adapter computes via a hardcoded price table that may drift | Periodically update pricing in `gemini.sh`; check `gemini_translate_error: true` in usage envelope as drift signal |
| Codex adapter is hybrid-only | Codex CLI lacks non-interactive prompt mode as of v10.7 | If you must run codex natively, file an issue — same path as gemini followed |
| Per-phase model overrides don't compose with `EVOLVE_TASK_MODE` budget tiers | Tier resolution happens after CLI resolution; budget tiers are per-CLI-profile | Set budget caps via `EVOLVE_MAX_BUDGET_USD` instead |
| Auditor's "different model family" assumption | If you set Builder=Sonnet and Auditor=Sonnet in `llm_config.json`, the adversarial-audit framing is weakened | Use Config C above; or set `ADVERSARIAL_AUDIT=0` if you're deliberately running a permissive sweep |
| Memo profile shell-tool restrictions (cycle 62 B4 fix) | Removed `Bash(cat:*)`, `Bash(head:*)`, `Bash(tail:*)` from memo allowlist; if a custom memo persona needs these, it must use Read instead | Use Read tool for file inspection |

---

## References

| Source | Relevance |
|---|---|
| [`../architecture/platform-compatibility.md`](../architecture/platform-compatibility.md) | Top-level CLI support matrix + adapter contract |
| [`../architecture/capability-schema.md`](../architecture/capability-schema.md) | Schema for `*.capabilities.json` files |
| [`../architecture/tri-layer.md`](../architecture/tri-layer.md) | Skill / Persona / Command separation |
| [trust-architecture.md](trust-architecture.md) Tier 3 §Adversarial Auditor | Why same-model judges are forbidden by default |
| [`../incidents/cycle-61.md`](../incidents/cycle-61.md) | Cycle 61/64 incidents: gemini-3.1-pro-preview routing exposed real B0-B7 framework bugs |
| [`../incidents/gemini-forgery.md`](../incidents/gemini-forgery.md) | The v7.9.0+ structural defenses for non-claude CLIs (anti-forgery prompt, artifact content checks, .sh write protection) |
| ADR-1 (in `docs/adr/` or `docs/architecture/`) | LLM router design rationale |
