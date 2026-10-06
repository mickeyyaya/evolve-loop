# Antigravity CLI (agy) Runtime

> How `/evo:loop` reaches the dispatcher under Antigravity CLI (agy). Unlike Gemini, `agy` supports native non-interactive prompt mode (`agy -p`), so NATIVE execution is the preferred path — no hybrid shim required when `agy` is on PATH.
>
> **Note (script→Go migration, 2026-06):** the bash CLI adapters (`legacy/scripts/cli_adapters/*.sh`) referenced below were removed; dispatch is now native Go via the bridge drivers (`agy` / `agy-tmux`). The invocation-chain and example-command sections below describe the former bash path and are retained as historical context.

## Invocation chain

```
User: /evo:loop 5 polish improve dispatcher
  (typed into agy CLI)

  ↓ agy resolves the skill via ~/.antigravity/extensions/<install-path>
  ↓ activates SKILL.md content; reads reference/platform-detect.md
  ↓ detects platform = antigravity, reads reference/agy-runtime.md (this file)

Skill activates → STRICT MODE: execute exactly one shell command:
  bash archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh 5 polish "improve dispatcher"

  ↓ (agy calls run_shell_command)

Dispatcher loops once per cycle:
  bash archive/legacy/scripts/dispatch/run-cycle.sh "improve dispatcher"

  ↓

run-cycle.sh spawns the orchestrator subagent via:
  bash legacy/scripts/dispatch/subagent-run.sh orchestrator $CYCLE $WORKSPACE

  ↓

subagent-run.sh reads .evolve/profiles/orchestrator.json
  → cli = "antigravity"  (set by agy CLI users via env or profile override)
  → cross-name resolver maps antigravity → agy
  → dispatches to legacy/scripts/cli_adapters/agy.sh

  ↓

agy.sh (three-mode adapter):
  NATIVE  (agy on PATH): invokes `agy -p <prompt> --dangerously-skip-permissions [--add-dir ...]`
           Captures plain-text stdout; appends zero-cost envelope as last STDOUT_LOG line.
  HYBRID  (claude on PATH, no agy): delegates to legacy/scripts/cli_adapters/claude.sh
           Full subprocess isolation + profile permissions + budget cap.
  DEGRADED (neither): emits stub JSON to STDOUT_LOG; same-session execution.
```

## Mode semantics

| Mode | Trigger | Subprocess isolation | Budget cap | Cost |
|---|---|---|---|---|
| NATIVE | agy binary on PATH | agy subprocess (no profile permissions) | none (cost_blind) | zero-attributed (see below) |
| HYBRID | claude on PATH, agy missing | claude subprocess (full profile permissions, sandbox) | native via `--max-budget-usd` | attributed |
| DEGRADED | neither binary | none (same-session) | none | zero |

### HYBRID vs DEGRADED asymmetry for agy

Unlike Gemini's HYBRID/DEGRADED distinction (which primarily differs on JSON output quality), agy's modes differ structurally:

- **HYBRID** (claude present): executes in an isolated claude subprocess with profile-scoped permissions, sandbox, and budget cap. Full pipeline guarantees.
- **DEGRADED** (no agy, no claude): executes in the calling session. No subprocess isolation.
- **NATIVE** (agy present): executes in an agy subprocess. Has process-level isolation but lacks profile permissions, sandbox, and budget cap. Plain text output only.

HYBRID is the gold standard. NATIVE is usable but degrades capability tier. DEGRADED is last-resort.

### cost_blind:true — why and what it means

In NATIVE mode, agy does not emit billing data (no token counts, no cost). The adapter appends a hardcoded zero-cost envelope to satisfy `subagent-run.sh`'s usage-parsing contract. `cost_blind:true` in the envelope signals this is an intentional stub, not a real zero.

**Deferred work:** A rollout cycle must add one of:
1. An external billing tap (API usage query after the call)
2. A turn-cap fallback (`--print-timeout` based heuristic)

Until then, `state.json:currentBatch.cycleAccruedCostUSD` will under-report costs from NATIVE agy cycles.

## Required environment

| Variable | Required | Purpose |
|---|---|---|
| `agy` binary on PATH | yes (for NATIVE) | The execution engine |
| `claude` binary on PATH | yes (for HYBRID, if agy missing) | Full-caps fallback |
| Auth credentials | yes | agy uses API key env vars or `~/.antigravity/` OAuth dir |

## Cross-name resolution

`subagent-run.sh` resolves adapter file names by `${cli}.sh`. Since profile `cli` field is `"antigravity"` but the adapter file is `agy.sh`, a cross-name resolver block is inserted before the adapter path construction in both `validate_profile` and `run_agent`:

```bash
[ "${cli:-}" = "antigravity" ] && cli="agy"
```

This is additive only — existing claude/gemini/codex paths are unaffected.

## Verifying the agy path before running cycles

```bash
# 1. Confirm agy binary is present
command -v agy

# 2. Confirm agy.sh resolves correctly (validate-only mode)
VALIDATE_ONLY=1 bash legacy/scripts/cli_adapters/agy.sh
# Expected: exit 0, log line "[agy-adapter] VALIDATE_ONLY=1 — not executing"

# 3. Probe mode (check tier)
bash legacy/scripts/cli_adapters/agy.sh --probe
# Expected: "[agy-adapter] PROBE OK: agy binary present; resolved tier=..."

# 4. Smoke-test detection
bash legacy/scripts/dispatch/detect-cli.sh
# Expected: prints "antigravity" if agy is on PATH and no CLAUDE_CODE_*/GEMINI_*/CODEX_* env set

# 5. Capability check
bash legacy/scripts/cli_adapters/_capability-check.sh antigravity --human
```

## Interactive agy (agy-tmux): pane vocabulary and liveness

The loop's default agy path is the interactive `agy-tmux` driver, not `agy -p`. What the bridge reads from an agy pane is declared in `go/internal/bridge/manifests/agy-tmux.json`. The values below were verified against agy 1.2.17 on 2026-10-06; fixtures are in `go/internal/bridge/panestream/testdata/agy-1.2.17/`.

| What | How agy 1.2.17 shows it | Where the bridge declares it |
|---|---|---|
| REPL ready | footer `? for shortcuts` | `prompt_marker` and the driver's `promptMarker` |
| Turn running | footer `esc to cancel`, plus a spinner line: one of `⣾⣽⣻⢿⡿⣟⣯⣷`, two spaces, a verb (`Generating...`, `Working...`, `Loading...`, `Editing files...`, or a running thought summary) | `busy_line_regex`; group 1 is the frame, which is cut before progress is judged, so a spinner tick is not progress |
| Input line | the box between two separators: exactly `>` when empty; `> <text>` or `> [Pasted text #N +M lines]` when a prompt is parked | the driver's `inputLineMarker` (`>`), so submit-verify detects a parked prompt and re-sends Enter |
| Thinking tokens | `▸ Thought for 14s, 1.5k tokens` after each thinking block | `token_line_regex` (named groups `count`, `scale`): token telemetry records the peak as `scrollback_peak` instead of "uncovered" |
| Model in use | footer, bottom right: `Gemini 3.8 Flash · low`. When agy ignores `--model`, this shows the model it fell back to | `model_label_regex` (named group `model`, read from the last line) |
| Quota wall | `quota_exhausted` rule (`quota.*exceed`, daily, monthly or free-tier limit) | escalates (exit 85); `clihealth` benches the agy family on it |

To see whether agy is still working on a phase, and what it did last, run `evolve bridge sessions`. It is read-only and lists every live pane with its busy state, progress age, drawn age and last token line. The per-phase observer reads the same snapshot and signals `LIVENESS_PHASE_STALLED` when the transcript stops changing for the stall threshold. Design record: [agy-liveness-monitoring-2026-10.md](../../../docs/research/agy-liveness-monitoring-2026-10.md).

Two agy facts that matter for liveness:

- **`#{window_activity}` is not progress.** agy repaints about every 2 s even when idle.
- **A spinning `Generating...` line is not progress.** In one 1.2.17 session agy spun it for about two minutes and then printed `failed to construct executor: plan model not specified`; `agy models` listed the model it had rejected.

## See also

- [reference/agy-tools.md](agy-tools.md) — tool name translation map and agy-specific flags
- [reference/platform-detect.md](platform-detect.md) — how the skill identifies its platform
- [reference/gemini-runtime.md](gemini-runtime.md) — Gemini hybrid driver (comparison reference)
- [reference/claude-runtime.md](claude-runtime.md) — what HYBRID mode delegates to
- [docs/architecture/platform-compatibility.md](../../../docs/architecture/platform-compatibility.md) — CLI support matrix

## Last verified

- **agy-tmux pane vocabulary:** 2026-10-06, agy 1.2.17 (`agy --help` lists `-i/--prompt-interactive`, `--log-file`, `--effort`), captured on a private tmux socket.
- **Date:** 2026-05-21 (cycle-101 infrastructure ship)
- **agy binary:** `~/.local/bin/agy` (140MB, installed 2026-05-20), flags `--print`/`-p`, `--dangerously-skip-permissions`, `--add-dir`
- **Re-verify cadence:** quarterly or when agy releases new flags. If agy adds `--max-budget-usd` or native JSON output, update `agy.capabilities.json:supports.budget_cap_native` and the cost_blind envelope accordingly.
- **Quick re-verify command:** `agy --help 2>&1 | grep -E 'print|budget|json|dir'`
