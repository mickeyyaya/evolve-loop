# CLI capability matrix

> **Authoritative per-CLI capability + driver reference.** Which LLM CLIs can run
> evolve-loop, which interactive-control / pipeline guarantees each driver
> provides, the NATIVE/HYBRID/DEGRADED tier model, the ADR-0029 fallback chain,
> and the any-CLI × any-phase × any-model invariant.
>
> Machine-readable sources of truth:
> - **Launch manifests** — `go/internal/bridge/manifests/<cli>.json` (binary, tier, model-tier map, interactive prompts), surfaced via the bridge realizer.
> - **Capability catalogs** — `go/internal/bridge/capabilities/catalogs/<cli>.json` (slash commands, key bindings, extension mechanism, headless entrypoint), surfaced by `evolve bridge capabilities --cli=<cli>` and drift-checked against live `/help` by `evolve bridge introspect --cli=<cli>`.
> - **Adapter capability manifests** — `<cli>.capabilities.json` `supports.*` block (budget/permission/sandbox booleans, ADR-0002).
>
> Related: [bridge & adapters architecture](../architecture/bridge-and-adapters.md) ·
> [glossary](../00-overview/glossary.md) ·
> [env-vars reference](./env-vars.md)

---

## Registered drivers

Eight drivers are registered in `go/internal/bridge/` (`func init() { Register(...) }`):

| Driver | Binary | Headless entrypoint | Session model | Prompt marker | Notes |
|---|---|---|---|---|---|
| `claude-p` | `claude` (≥ 2.1.0) | `claude -p` / `claude plugin …` | one-shot subprocess | — (`null`) | Reference runtime. Native budget cap, profile permissions, sandbox. Default model `haiku`. |
| `claude-tmux` | `claude` | interactive REPL in tmux | persistent REPL | `❯` | Live injection + recipe-driven slash sequences. Honors `--permission-mode`. |
| `codex` | `codex` (≥ 0.13.0) | `codex exec` | one-shot subprocess | — (`null`) | OpenAI. `default_model` empty (codex picks per active auth). |
| `codex-tmux` | `codex` | interactive REPL in tmux | persistent REPL | `›` (alt-screen) | Plugin install is menu-driven (`/plugins` TUI), not a one-liner. |
| `agy` | `agy` (≥ 1.0.0) | `agy -p` | one-shot subprocess | — (`null`) | Antigravity, **Gemini-backed (NOT Anthropic)**. Accepts `-m/--model` flag. |
| `agy-tmux` | `agy` | interactive REPL in tmux | persistent REPL | `? for shortcuts` | Skills + MCP file-config extension model (no marketplace). |
| `ollama-tmux` | `ollama` (≥ 0.1.0) | `ollama run <m> "…"` | persistent REPL | `>>> ` | Local-or-cloud. **Reasoning/review phases only** — no agentic tool-use; source-writing assignment rejected at Launch. |

> `agy` = Antigravity CLI. `agy-tmux` historically also surfaced as "gemini" in the
> tier table — both route to the Gemini backend.

## Per-driver model-tier maps

The `model_tier_map` in each manifest translates abstract `fast`/`balanced`/`deep`
tiers into a CLI-native model string. `EVOLVE_<AGENT>_MODEL` / `--model agent=model`
override per-phase.

| Driver | fast | balanced | deep | default_model |
|---|---|---|---|---|
| `claude-p` / `claude-tmux` | — | — | — | `haiku` (claude-p) |
| `codex` / `codex-tmux` | `gpt-5.4-mini` | `gpt-5.4` | `gpt-5.5` | empty (auth-determined; `gpt-5.5` is the ChatGPT-account default) |
| `agy` / `agy-tmux` | `gemini-3.5-flash` | `gemini-3.5-flash` | `gemini-3.5-flash` | `gemini-3.5-flash` (deep-tier string pending live `-m` validation) |
| `ollama-tmux` | `qwen3:7b` | `qwen3:30b` | `qwen3-coder:30b` | `llama3.1:8b` |

> **ollama cloud routing is by MODEL TAG**, not env: `gpt-oss:120b-cloud` hits
> ollama.com (needs `OLLAMA_API_KEY` / `ollama signin`); bare tags run on local
> hardware. `OLLAMA_HOST` selects which `ollama serve` instance, not cloud
> routing. Cloud cannot do `format: {schema}` structured output — pin local tags
> for schema-bound phases.

## Interactive-control surface (tmux drivers)

The `keystroke` envelope (`evolve bridge send --kind=keystroke --body=<keys>`)
reaches a live REPL verbatim via raw `tmux send-keys`. `keyspec` warns on mistyped
key names but never blocks.

| Intent | claude | codex | agy | ollama |
|---|---|---|---|---|
| Interrupt turn | `Esc` | `Esc Esc` | `Esc Esc` | `Ctrl+C` |
| Exit | `Ctrl+D` / `/exit` | `Ctrl+C` / `/quit` | `(Ctrl+C)` | `Ctrl+D` / `/bye` |
| Confirm modal | `Enter` | `Enter` | `Enter` | — |
| Cancel / dismiss | `Esc` | `Esc` | `Esc` | `Ctrl+C` |
| Cycle permission mode | `Shift+Tab` | — | — | — |

## Extension / plugin-install model

| Driver | Extension kind | Install flow |
|---|---|---|
| `claude-tmux` | `plugin_marketplace` | `/plugin marketplace add <url>` → `/plugin install <name>@<mkt>` → `/reload-plugins` (reload **mandatory** — installs don't auto-activate mid-session). Drivable via `evolve bridge recipe run plugin-install --cli=claude-tmux …`. |
| `codex-tmux` | `plugin_marketplace` (TUI) | `/plugins` opens a TUI; install is arrow-key navigation, not a one-liner. The `plugin-install` recipe's codex arm opens the browser; finishing is `keystroke`-kind menu navigation. |
| `agy-tmux` | `skills_mcp` (file-config) | No marketplace. Place a skill at `~/.gemini/skills/<name>/SKILL.md` (or project `.agents/skills/`), configure MCP in `~/.gemini/config/mcp_config.json`, browse via `/skills` / `/mcp`. |
| `ollama-tmux` | `none` | No extension system; customization is Modelfiles + `/set`. |

## Pipeline-capability tiers (ADR-0002 `supports.*` booleans)

Each adapter ships a `supports.*` block declaring which kernel guarantees it
natively provides. Absent block (or absent file) → all booleans default `true`
(backward-compat). Missing booleans emit a parseable WARN
(`[adapter-cap] WARN cli=<n> missing=<cap> substitute=<sub>`) and set
`CAP_BUDGET_NATIVE=false` etc., so the adapter omits the unsupported flag rather
than failing silently.

| Field | claude | gemini/agy | codex | Substitute when false |
|---|---|---|---|---|
| `budget_cap_native` | `true` | `false` | `false` | `wall_clock_timeout` |
| `permission_scoping` | `true` | `false` | `false` | `kernel_role_gate_only` |
| `sandbox_native` | `true` | `false` | `false` | — |
| `non_interactive_prompt` | `true` | `true` | `true` | — (gates NATIVE mode) |
| `structured_logs` | `true` | `true` | `true` | — |
| `model_flag` | `true` | `true` | `true` | — |

## NATIVE / HYBRID / DEGRADED tier model (ADR-0003)

Each non-Claude adapter resolves one of three execution modes. **Priority: NATIVE > HYBRID > DEGRADED.**

| Mode | When | Behavior |
|---|---|---|
| **NATIVE** | CLI binary on PATH **and** `supports.non_interactive_prompt: true` | Invoke the adapter's own binary directly (`exec $BIN < $PROMPT_FILE`). Takes priority over HYBRID — if an operator installed `gemini`/`codex`/`agy`, they expect it to run the phase, not silently delegate to Claude. |
| **HYBRID** | `claude` binary on PATH | Delegate to `claude.sh` for full subprocess isolation, profile permissions, native budget cap. For operators who have Claude but not the target CLI. |
| **DEGRADED** | Neither binary available | Same-session execution. Reduced isolation; pipeline kernel hooks still provide structural safety. |

**Resolved tiers by CLI** (from platform-compatibility.md, terminology `full`/`hybrid`/`degraded`):

| CLI | Default tier | With claude on PATH | Without claude |
|---|---|---|---|
| Claude Code | `full` | `full` | n/a (native reference runtime) |
| Gemini CLI / `agy` | depends on env | `hybrid` (full caps via delegation) | `degraded` (same-session); `agy -p` NATIVE since v10.19.0 when on PATH |
| Codex CLI | depends on env | `hybrid` | `degraded` (v8.51.0+; pre-v8.51 was a tier-3 stub) |
| ollama | `hybrid` (manifest default) | hybrid | degraded (no tmux) |

**Critical safety invariant:** missing capabilities **never block the pipeline**.
They only lower quality (more warnings, less subprocess isolation, weaker forgery
defenses) and surface as `quality_tier` annotations in ledger entries. The trust
kernel (`role-gate`, `ship-gate`, `phase-gate-precondition`, ledger SHA chain)
fires on **bash commands**, not adapter dispatch — a degraded adapter cannot
bypass structural safety, only operate with reduced isolation.

> Probe your environment's resolved tier with `./bin/check-caps <adapter>`
> (or `./bin/check-caps` to auto-detect), or `evolve setup detect`.

## ADR-0029 CLI fallback chain (any-CLI × any-phase × any-model)

**User goal (cycle-121):** "Allow any combination of LLM CLIs + any model to be
adapted for any phase (even new customized user/LLM-constructed phases) — always
be executed in the pipeline."

Pre-ADR-0029 each phase pinned exactly one CLI — a single point of failure
(cycle-121: auditor pinned `codex-tmux`, codex 0.134 hit `ExitREPLBootTimeout (80)`,
the whole cycle died though three other CLIs were registered). The fix: a
**fallback chain** with per-agent overrides, launch flags, and a startup probe —
all defaulting to byte-identical pre-G behavior (single-element chains).

**Primary CLI resolution precedence** (`resolveCLIChain` in `cli_chain.go`):
1. `EVOLVE_<AGENT>_CLI` (per-agent env)
2. `EVOLVE_CLI` (global env)
3. `profile.CLI` (on-disk per-agent config)
4. `"claude-tmux"` (final default)

**Chain construction:** candidates = `[primary] + dedup(profile.cli_fallback − primary)`.
Triggers = `profile.cli_fallback_on_exit` or the default trigger list.

**Default fallback trigger codes:** `[80, 81, 124, 127]` (live default per cycle-122
remediation; ADR-0029 originally `[80, 127]`). Enforced by
`go/internal/phases/runner/cli_chain.go:defaultFallbackOnExit`:
- `80` = `ExitREPLBootTimeout`
- `81` = `ExitArtifactTimeout` (added cycle-122)
- `124` = wall-clock timeout (added cycle-122)
- `127` = `ExitMissingBinary`

**Dispatch loop** (`runner.go`): each attempt is logged + ledger-recorded. On a
**trigger** exit → advance to the next candidate. On a **non-trigger** exit (a
legitimate model FAIL verdict, `ExitSafetyGate`) → surface as-is.
**A legitimate model FAIL never silently retries on a different CLI** — the chain
only catches CLI-level integration bugs, never masks a real verdict.

**Launch flags (G2, sugar over G1 env):** repeatable
`--cli scout=agy-tmux --cli auditor=claude-tmux --model auditor=opus --model builder=llama3.1:8b`.
Each pair → `EVOLVE_<AGENT>_CLI` / `EVOLVE_<AGENT>_MODEL` (dash→underscore upcase).
Flags beat inherited shell env; malformed pairs reject with exit 10.

**Startup capability probe (G3):** `probeAvailableCLIChain` runs
`exec.LookPath(<binary>)` for each candidate BEFORE the dispatch loop.
Missing-binary CLIs are **DEMOTED to the end** (not deleted) — if ALL are missing,
the original primary still attempts so the bridge surfaces a real
`ExitMissingBinary 127`. Cuts a missing-CLI's failure time from ~60s to milliseconds.

**Adding a new CLI** (forward-compatibility) needs only:
1. A driver registered via `Register()` in `internal/bridge/driver.go`.
2. A manifest at `internal/bridge/manifests/<cli>.json`.
3. An entry in the `cliBinaryFor` map in `cli_chain.go` (so the probe can resolve the binary).

After that, any profile can name it in `cli` or `cli_fallback`, and
`--cli phase=<cli>` Just Works. User-defined phases inherit the same routing
automatically (they have profile JSONs and use the same runner code path).

> **Orthogonal axes:** ADR-0024 PhaseAdvisor chooses WHICH phases run; ADR-0029
> chooses WHICH CLI runs a chosen phase. The `allowed_clis` profile field
> (ADR-0027 setup-onboarding) is the envelope constraint gating which CLIs may
> appear in a chain.

## Capability catalog & drift detection (ADR-0031)

Each `catalogs/<cli>.json` records the CLI's control surface: `slash_commands[]`,
`key_bindings[]`, `extension{kind,summary,install_flow}`, `headless{entrypoint}`,
`sources[]` (cited from official docs + research dossier).

- `evolve bridge capabilities --cli=X [--json]` — print the static catalog.
- `evolve bridge introspect --cli=X [--pane-file=P]` — run `/help` live (or read a captured pane), parse the slash surface, and `Diff` it against the catalog. Exit `0` clean, `3` drift, `10` usage error. Reports `in_catalog_not_live` (documented but absent) and `in_live_not_catalog` (live but undocumented).
- `evolve bridge recipe run|list|show` — drive a scripted multi-step slash sequence (e.g. `plugin-install`) with inter-step verification, no orchestrator.

**Known-pending validations:** agy `-m` deep-tier model string (catalog pins
`gemini-3.5-flash`; `gemini-3.1-pro` for deep needs a live check); codex
`--no-alt-screen` for clean scrollback capture under tmux.
