# Antigravity CLI (agy) Runtime

> How `/evo:loop` reaches the dispatcher under Antigravity CLI (agy). Unlike Gemini, `agy` has its own bridge drivers, so no hybrid alias is required when `agy` is on PATH.
>
> **Note (script→Go migration, 2026-06):** the bash CLI adapters (`legacy/scripts/cli_adapters/*.sh`) were removed. The Go bridge dispatches agy natively through two drivers: `agy` (headless `agy -p`) and `agy-tmux` (the interactive REPL in tmux).

## Invocation chain

```
User: /evo:loop 5 polish improve dispatcher
  (typed into agy CLI)

  ↓ agy resolves the skill via ~/.antigravity/extensions/<install-path>
  ↓ activates SKILL.md content; reads reference/platform-detect.md
  ↓ detects platform = antigravity, reads reference/agy-runtime.md (this file)

Skill activates → STRICT MODE: execute exactly one shell command:
  "${EVOLVE_GO_BIN:-<plugin_root>/go/bin/evolve}" loop 5 polish "improve dispatcher"

  ↓ (agy calls run_shell_command)

evolve loop runs one cycle per iteration, in process:
  the Go orchestrator (go/internal/core) runs the phases in order

  ↓

For each phase, the orchestrator dispatches the phase agent through the
Go bridge (go/internal/bridge). The routing table (.evolve/policy.json
cli_routing) and the agent profile choose the CLI; the default chain is
agy-tmux → agy-claude-tmux → claude-tmux
(`evolve cli-routing explain <agent>` prints it).

  ↓

A profile cli = "antigravity" becomes "agy" (detectcli.Canonical).
The agy-tmux driver runs the agy REPL in tmux. The headless agy driver runs
`agy -p <prompt> --dangerously-skip-permissions`
(go/internal/bridge/driver_agy.go); the bridge wraps it in the OS sandbox
when the sandbox applies (go/internal/bridge/sandbox_wrap.go).
```

## Removed design: NATIVE, HYBRID and DEGRADED modes

The bash `agy.sh` adapter had three modes: NATIVE (`agy -p`), HYBRID (delegate to the claude adapter) and DEGRADED (same session). The Go drivers have no modes. The routing chain gives the fallback: if agy cannot run a phase, the next link of the chain runs it. The `agy.sh` zero-cost envelope (`cost_blind:true`) is also gone.

## Required environment

| Variable | Required | Purpose |
|---|---|---|
| `agy` binary on PATH | yes | The execution engine for the `agy`, `agy-tmux` and `agy-claude-tmux` drivers |
| `claude` binary on PATH | for the last link of the routing chain | The `claude-tmux` driver runs Claude Code |
| Auth credentials | yes | agy uses API key env vars or `~/.antigravity/` OAuth dir |

## Cross-name resolution

Detection names the CLI `antigravity` (`evolve detect-cli`), but the drivers, the capability manifest and the binary are named `agy`. `detectcli.Canonical` (`go/internal/detectcli/detectcli.go`) changes `antigravity` to `agy`. Every dispatch entry point calls it, for example `go/internal/subagent/validateprofile.go`.

## Verifying the agy path before running cycles

```bash
# 1. Confirm agy binary is present
evolve doctor probe agy
# Expected: [doctor] OK: agy found at <path>

# 2. List the bridge drivers and their tiers
evolve bridge probe
# Expected: entries for "agy", "agy-tmux" and "agy-claude-tmux"

# 3. Smoke-test detection
evolve detect-cli
# Expected: prints "antigravity" if agy is on PATH and no CLAUDE_CODE_*/GEMINI_*/CODEX_* env set

# 4. Show the routing chain of a phase agent
evolve cli-routing explain scout
```

## Interactive agy (agy-tmux): pane vocabulary and liveness

The loop's default agy path is the interactive `agy-tmux` driver, not `agy -p`. What the bridge reads from an agy pane is declared in `go/internal/bridge/manifests/agy-tmux.json`. The values below were verified against agy 1.2.17 on 2026-10-06; fixtures are in `go/internal/bridge/panestream/testdata/agy-1.2.17/`.

| What | How agy 1.2.17 shows it | Where the bridge declares it |
|---|---|---|
| REPL ready | footer `? for shortcuts` | `prompt_marker` and the driver's `promptMarker` |
| Turn running | footer `esc to cancel`, plus a spinner line: one of `⣾⣽⣻⢿⡿⣟⣯⣷`, two spaces, a verb (`Generating...`, `Working...`, `Loading...`, `Editing files...`, or a running thought summary) | `busy_line_regex`; group 1 is the frame, which is cut before progress is judged, so a spinner tick is not progress |
| Input line | the box between two separators: exactly `>` when empty; `> <text>` or `> [Pasted text #N +M lines]` when a prompt is parked | the driver's `inputLineMarker` (`>`), so submit-verify detects a parked prompt and re-sends Enter |
| Thinking tokens | `▸ Thought for 14s, 1.5k tokens` after each thinking block | `token_line_regex` (named groups `count`, `scale`): token telemetry records the peak as `scrollback_peak` instead of "uncovered" |
| Model in use | footer, bottom right: `Gemini 3.8 Flash · low`. When agy ignores `--model`, this shows the model it fell back to | `model_label_regex` (named groups `footer`, the `esc to cancel` or `? for shortcuts` prefix, and `model`; the pane watch reads the last line, and agy-claude's launch-time model check reads only the label on the footer-prefix line) |
| Quota wall | `quota_exhausted` rule (`quota.*exceed`, daily, monthly or free-tier limit) | escalates (exit 85); `clihealth` benches the agy family on it |

### `/usage`: one screen, two quota groups (agy 1.3.0)

`/usage` is agy's usage command (`controls.usage.send`). agy pools quota per model group, and the screen prints one block per group. Each block carries a weekly and a five-hour window:

```
GEMINI MODELS
  Models within this group: Gemini Flash, Gemini Pro
  Weekly Limit Remaining
    [██████████████████████████████████████████████░░░░] 91.70%
    Refreshes in 140h 56m
  Five Hour Limit Remaining
    [██████████████████████████████████████████████████] 100.00%
    Quota available

CLAUDE AND GPT MODELS
  Models within this group: Claude Opus, Claude Sonnet, GPT-OSS
  ...
```

| Screen element | Meaning | Where the bridge declares it (`controls.usage.windows` in `agy-tmux.json`) |
|---|---|---|
| `GEMINI MODELS`, `CLAUDE AND GPT MODELS` | a quota group's header; its windows take it as their scope | `section_regex` (named group `scope`); `scopes` maps it to the routing family the bench is keyed by: `agy` and `agy-claude` |
| `Models within this group: …` | the models the group's quota covers | `models_regex` (`models`) |
| `Weekly Limit Remaining`, `Five Hour Limit Remaining` | a window's label | `labels`, each with its `kind` (`week`, `5h`) |
| the bar line's `91.70%` | the window's *remaining* quota | `value_regex` (`pct`) with `value_direction: remaining`, normalized to 8.3% used; a window at or above `exhausted_at_used_pct` (99.5) drains its group |
| `Refreshes in 40m`, `Refreshes in 140h 56m` | when the window refills; `Quota available` when it is not drained | `reset_regex` (`reset`); the bench lasts until the latest reset of the group's drained windows, capped at 24 h |

The pre-wave usage probe reads this screen once (through agy-tmux) and benches each drained group's own family, so a drained Claude group benches `agy-claude` and leaves the Gemini models routable. agy 1.2.x printed an extra `N% remaining · Refreshes in …` line under each bar; the same spec reads both layouts. The same reader serves every CLI: Claude Code's `/usage` (`N% used`, `Resets …`) is declared in `claude-tmux.json`. The older whole-family `exhausted_regex` stays as the fallback for a screen with no windows; it cannot see `0.00%` on its own line. To see the windows now, run `evolve clihealth usage agy` (read-only). When agy fails, start from the [CLI-failure triage runbook](../../../docs/operations/cli-failure-triage.md): it verifies or rules out quota before anything else. Fixtures: `go/internal/quotastate/testdata/agy_usage_*.txt`.

### Cold start

agy's first launch after an idle period (a wave boundary) can take longer than the bridge's 60 s REPL boot budget, while the next launch boots in seconds. The probes that run first (the boundary updater's `doctor live`, preflight's bridge-boot, the cli-health canary) retry one bare boot timeout for agy (`probe_boot_retries: 1`) and log `cold start: boot attempt 1 of 2 …`. See [the incident note](../../../docs/incidents/agy-cold-start-boot-timeout-2026-10-06.md).

To see whether agy is still working on a phase, and what it did last, run `evolve bridge sessions`. It is read-only and lists every live pane with its busy state, progress age, drawn age and last token line. The per-phase observer reads the same snapshot and signals `LIVENESS_PHASE_STALLED` when the transcript stops changing for the stall threshold. Design record: [agy-liveness-monitoring-2026-10.md](../../../docs/research/agy-liveness-monitoring-2026-10.md).

Two agy facts that matter for liveness:

- **`#{window_activity}` is not progress.** agy repaints about every 2 s even when idle.
- **A spinning `Generating...` line is not progress.** In one 1.2.17 session agy spun it for about two minutes and then printed `failed to construct executor: plan model not specified`; `agy models` listed the model it had rejected.

## See also

- [reference/agy-tools.md](agy-tools.md) — tool name translation map and agy-specific flags
- [reference/platform-detect.md](platform-detect.md) — how the skill identifies its platform
- [reference/gemini-runtime.md](gemini-runtime.md) — Gemini hybrid driver (comparison reference)
- [reference/claude-runtime.md](claude-runtime.md) — the last link of the routing chain
- [docs/architecture/platform-compatibility.md](../../../docs/architecture/platform-compatibility.md) — CLI support matrix

## Last verified

- **agy `/usage` groups:** 2026-10-06, agy 1.3.0, read through `evolve bridge control agy usage`; the older layout from a 2026-08-10 usage-probe escalation report.
- **agy-tmux pane vocabulary:** 2026-10-06, agy 1.2.17 (`agy --help` lists `-i/--prompt-interactive`, `--log-file`, `--effort`), captured on a private tmux socket.
- **Date:** 2026-05-21 (cycle-101 infrastructure ship)
- **agy binary:** `~/.local/bin/agy` (140MB, installed 2026-05-20), flags `--print`/`-p`, `--dangerously-skip-permissions`, `--add-dir`
- **Re-verify cadence:** quarterly or when agy releases new flags. If agy adds `--max-budget-usd` or native JSON output, update `agy.capabilities.json:supports.budget_cap_native` accordingly.
- **Quick re-verify command:** `agy --help 2>&1 | grep -E 'print|budget|json|dir'`
