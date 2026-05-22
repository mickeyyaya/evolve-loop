# bridge — a uniform CLI for driving AI agents

> **Status**: v0.5.x — production-ready for `claude-p` and `claude-tmux`; `codex` and `agy` are stubs (v2).
> **Audience**: anyone who needs to launch an AI CLI from a script or pipeline with predictable arguments, structured output, and standardized failure modes.
> **License**: same as the host repo. Intended for extraction to its own repo per the v2 plan.

`bridge` is an **independent CLI tool**. It does not depend on any parent repository's runtime, scripts, or state. It can be vendored into another project, installed standalone on a workstation, or invoked from CI without pulling in extra dependencies beyond `bash 3.2+`, `jq`, `tmux`, and `openssl` plus whichever AI CLI (Claude Code, Codex, Antigravity) you actually want to drive.

The intended consumers are agent orchestrators (e.g., evolve-loop, custom pipelines), CI jobs that need a single LLM run, and operators driving Claude from scripts. Anything that needs to say *"run this prompt with this profile, write this artifact, tell me if it worked"* without per-CLI special-casing.

---

## Why

Each AI CLI ships with its own invocation surface, authentication model, output format, and operational quirks. Direct shell wrappers around them drift over time — diagnostics get inconsistent, failure modes go uncovered, observability is bolted on after the fact, and the cost model is whatever happens to leak through.

`bridge` is a uniform dispatch layer that solves four concrete problems:

1. **Subscription billing stays accessible.** After 2026-06-15 Anthropic split Claude's subscription quota from programmatic credits. `claude -p <prompt>` (headless) bills the API credit pool at full rates; the interactive `claude` REPL bills the subscription quota. For an orchestrator running tight inner loops, that's the difference between `$200/mo flat` and `$5+/cycle`. `bridge` keeps the subscription path open by driving the interactive REPL via tmux — invisible to the caller, who still says "run this prompt, write this artifact."
2. **One launch verb, many CLIs.** A single `bridge launch --cli=<name>` handles Claude (both flavors) today and Codex / Antigravity as drivers land. The caller never needs to know the underlying CLI's flag conventions, prompt-detection quirks, or exit-code idioms.
3. **Failure has a vocabulary.** A fixed exit-code table (see [Exit codes](#standardized-exit-codes)) replaces the per-CLI grab bag of `1` / `127` / `255` so callers can write deterministic retry and escalation logic.
4. **Drift goes into manifests, not code.** Each CLI's capabilities (binary, version floor, prompt marker, allowed flags, interactive-prompt rules) live in `lib/manifests/<cli>.json`. Extending the system means editing JSON, not patching scripts.

The non-goal is impersonation. `bridge` does not rewrite headers, fingerprint API calls, or evade the CLI's intended behavior. Whatever auth the operator has configured for their CLI is what gets used. Subscription routing works because the CLI itself does it when run interactively — `bridge` only delivers keystrokes.

For the full design rationale see `docs/design.md`.

---

## Features

### Core dispatch

- **`bridge launch`** — one verb, all CLIs. Required flags (`--cli`, `--profile`, `--model`, `--prompt-file`, `--workspace`, three log/artifact paths) are identical across drivers. Optional flags map driver-specific behavior in a predictable place.
- **`bridge probe`** — JSON report of available CLIs with capability tier (`full` / `hybrid` / `degraded` / `none`), binary path, and detected version. Lets callers decide pre-launch whether to proceed or fall back.
- **`bridge validate`** — dry-run mode: parse profile, resolve manifest, print everything, exit 0. No driver call. Used in CI and pre-flight checks.
- **`bridge report`** — re-print a structured summary for a past workspace from its artifacts.
- **`bridge add-rule`** — append an `interactive_prompts` rule to a CLI's manifest from an escalation report (see [Auto-respond learning loop](#auto-respond-learning-loop)).
- **`bridge version`** / **`bridge help`** — boring but documented.

### Profile-based configuration

Every launch reads a JSON profile that declares:

- `model` — `haiku` / `sonnet` / `opus` (resolved if `--model=auto`)
- `allowed_tools[]` — whitelist passed to `claude --allowedTools` for the headless driver
- `permission_mode` — default `--permission-mode` for claude (overridden by CLI flag or env)
- `stream_output` — boolean, enable realtime JSONL events (claude-p, v0.3+)
- `max_turns`, `agent_role`, optional metadata fields

Profiles live with the caller, not with `bridge`. The same `bridge` install drives many orchestrators, each with its own profile catalog.

### Two Claude drivers

| Driver | Underlying call | Billing path | When to use |
|---|---|---|---|
| **`claude-p`** | `claude -p "<prompt>" --model <m> --allowedTools <list>` | Programmatic credit pool (full API rate) | Operator has opted into API billing, or pre-2026-06-15 |
| **`claude-tmux`** | Interactive `claude` REPL inside a detached tmux session, driven by `tmux send-keys` and pane polling | Subscription quota | Default for cost-conscious workflows post-cutover |

Both drivers honor the same `bridge launch` surface. The choice is one CLI flag.

### Capability tiers via `bridge probe`

Each driver declares dependency sets in its manifest. `bridge probe` evaluates them and reports a tier:

| Tier | Meaning |
|---|---|
| `full` | All optional dependencies present; every feature available |
| `hybrid` | Core dependencies present; some optional features (e.g., stream output) unavailable |
| `degraded` | Bare minimum; only basic dispatch works |
| `none` | Binary missing or below version floor; driver cannot run |

`--require-full` rejects anything below `full` (or `hybrid` for tiers that explicitly allow it) with exit 99. Callers can probe first, then decide whether to proceed at the available tier.

### Stream output (claude-p, v0.3+)

`--stream-output` (or `BRIDGE_STREAM_OUTPUT=1`, or `profile.stream_output: true`) appends `--output-format=stream-json --include-partial-messages --verbose` to the underlying `claude -p` call. Phase observers see realtime JSONL events instead of waiting for the final response blob.

This solves a real failure mode: default text output is silent until the end. Long orchestrator sessions trip phase-observer stall detectors at the 600s threshold even though work is in progress. Stream-output keeps the wire warm.

Other drivers (`claude-tmux`, `codex`, `agy`) accept the flag but log a NOTE — they either already stream (tmux scrollback) or have no streaming flag equivalent.

### Permission-mode pass-through (v0.4+)

`--permission-mode=MODE` (or `BRIDGE_PERMISSION_MODE` env, or `profile.permission_mode`) passes through to `claude --permission-mode`. Valid values: `plan`, `default`, `acceptEdits`, `bypassPermissions`, `auto`, `dontAsk`.

Special handling: in `plan` mode the `claude-tmux` driver replaces `--dangerously-skip-permissions` with `--permission-mode plan` (the bypass acknowledgment is **not** required because plan mode disables writes by design).

Non-claude drivers (`codex`, `agy`) reject `--permission-mode` with exit 3 — that flag is claude-specific.

### Session lifecycle (claude-tmux, v0.5a–c)

- **v0.5a — session-name foundation**: each launch creates a uniquely-named tmux session derived from the workspace path. The session name is recorded in `workspace/session-name.txt`.
- **v0.5b — driver-level session-name + resume**: `--session-name=<name>` allows the caller to specify an explicit session for resume scenarios. The driver attaches to an existing session if present, otherwise creates a new one.
- **v0.5c — session-resume integration tests**: regression coverage for the resume path under realistic conditions (orphan cleanup, multi-launch reuse, name collision).

A v0.4 feature, **orphan tmux-session sweep on launch**, kills stale sessions that match the bridge naming pattern but have no live workspace — a defensive cleanup that prevents accumulation across crashes.

### Auto-respond — fallback policy engine

`--dangerously-skip-permissions` suppresses *permission* prompts. It does not suppress auth re-checks, rate-limit warnings, model-deprecation continuations, terminal-resize redraws, or anything else interactive that's not permission-gated.

`lib/auto-respond.sh` is a policy engine that:

1. Reads the per-CLI manifest's `interactive_prompts[]` array
2. Polls the tmux pane between artifact checks
3. Matches the pane against each pattern's `regex`
4. Acts based on `policy`:
   - `auto_respond` → send the configured keys
   - `escalate` → bail with exit 85 and write `workspace/escalation-report.json`
5. **Loop guard**: same pattern matching > 5× in one run → bail with exit 86 (regex is probably too greedy)

The function splits into a pure `auto_respond_decide` (testable without tmux) and an effectful `auto_respond_tick` that wraps it with tmux I/O. Unit tests cover the pure layer; integration tests cover the tick.

#### Auto-respond learning loop

When `bridge` bails with exit 85, the escalation report contains:

```json
{
  "schema_version": 1,
  "captured_at": "2026-05-22T03:16:33Z",
  "cli": "claude-tmux",
  "pattern_name": "unknown_prompt_xxx",
  "reason": "escalate",
  "pane_tail": "...last 30 lines...",
  "suggested_rule_template": { ... },
  "next_steps": ["Run: bridge add-rule --escalation=..."]
}
```

The operator runs `bridge add-rule --escalation=<report> --regex=R --response=KEYS` and the new rule is appended to the manifest. Re-running the workflow auto-responds correctly. `bridge` accumulates known prompts over time without code changes.

### Credential isolation

Drivers refuse to run if conflicting authentication paths are configured. For Claude, that means rejecting (or requiring an explicit opt-in for) the combination of subscription credentials and `ANTHROPIC_API_KEY` / `ANTHROPIC_BASE_URL` env vars. The check fires before the driver shells out, exit code 3.

### Human-input behavioral plausibility (H1–H5)

When `claude-tmux` types prompts into the REPL, the H1–H5 modes shape keystroke timing to look plausibly human — important for Anthropic safety models that flag burst-of-text input. Off by default; opt-in per profile.

### Empty-prompt guard + workspace-reuse warning

`bridge launch` refuses an empty prompt file with exit 10 (F5). When `--workspace` points to a directory that already has artifacts from a prior run, `bridge` emits a WARN to stderr and proceeds (F6) — destructive overwrite is the operator's call, but they should know it's happening.

### Standardized exit codes

A fixed vocabulary, identical across all drivers:

| Code | Meaning |
|---|---|
| `0` | Success |
| `2` | Safety gate (e.g., credential conflict not acknowledged) |
| `3` | Credential isolation violated |
| `10` | Bad flags / missing required input |
| `80` | REPL boot timeout |
| `81` | Artifact-write timeout |
| `85` | Auto-respond escalation (unknown interactive prompt) |
| `86` | Auto-respond loop guard (same pattern matched > 5×) |
| `99` | `--require-full` not satisfied |
| `127` | Missing binary |

Callers can `case` on these without per-CLI special-casing.

---

## Install

```bash
bash install.sh                        # symlinks bin/bridge → $HOME/.local/bin/bridge
bash install.sh --check                # verify install (symlink + PATH + schema_version=1)
which bridge && bridge --json version  # smoke
```

Make sure `$HOME/.local/bin` is on your `$PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

To uninstall:

```bash
bash install.sh --uninstall
```

`install.sh --copy` (B-W1, v0.4) copies the source tree instead of symlinking. Use this when the install must survive deletion of the source checkout (e.g., system-wide installs, or when callers can't tolerate the symlink dependency).

**Dependencies**

| Tool | Version | Required for |
|---|---|---|
| `bash` | 3.2+ | All drivers |
| `jq` | any recent | Profile/manifest parsing |
| `tmux` | any recent | `claude-tmux` driver |
| `openssl` | any recent | Challenge-token generation |
| `claude` | ≥ 2.1.x | claude drivers |
| `codex` | (v2) | codex drivers |
| `agy` | (v2) | antigravity drivers |

On macOS the billing-verification helper also uses the system `security` keychain probe — no extra install needed.

---

## CLI surface

```
bridge launch     Run one subagent invocation (the main verb)
bridge probe      Detect available CLIs and capability tiers (JSON output)
bridge validate   Dry-run: parse profile, print resolved config, exit 0
bridge report     Re-print structured summary for a past workspace
bridge add-rule   Append an interactive_prompts rule from an escalation report
bridge version    Print bridge version
bridge help       Print help
```

Full reference: `docs/cli-reference.md`.

### Quick example

```bash
bridge launch \
  --cli=claude-tmux \
  --model=haiku \
  --allow-bypass \
  --profile=./profile.json \
  --prompt-file=./prompt.txt \
  --workspace=/tmp/bridge-run-$$ \
  --stdout-log=/tmp/bridge-run-$$/stdout.log \
  --stderr-log=/tmp/bridge-run-$$/stderr.log \
  --artifact=/tmp/bridge-run-$$/artifact.md
```

The driver substitutes `$ARTIFACT_PATH` and `$CHALLENGE_TOKEN` placeholders in the prompt file before invoking the CLI. The challenge token is also written to `workspace/challenge-token.txt` so the agent's artifact can be verified end-to-end.

---

## Tests

```bash
bash tests/run-tests.sh --suite=unit          # fast, no network (~5s)
bash tests/run-tests.sh --suite=integration   # requires the relevant CLI(s) (~2m)
```

Integration suites cover:

- `cross-cli-contract.bats` — JSON output schema parity across drivers
- `permission-mode-drivers.bats` — `--permission-mode` semantics per driver
- `phase-patterns.bats` — broader-usage patterns matching real orchestrator load
- `session-lifecycle.bats` — tmux session create/attach/cleanup
- `session-resume.bats` — explicit session-name + resume scenarios (v0.5c)
- `stream-output-drivers.bats` — `--stream-output` behavior across drivers
- `skill-flow-e2e.bats` — end-to-end skill invocation

Unit suites cover: `human-input.bats`, `json-output-contract.bats`, `permission-mode.bats`, `stream-output.bats` plus the pure-helper modules in `lib/`.

---

## Layout

```
tools/agent-bridge/
├── bin/bridge                # entrypoint (dispatcher)
├── lib/                      # shared helpers (no I/O if possible)
│   ├── profile.sh            # parse agent profile JSON
│   ├── manifest-loader.sh    # parse per-CLI manifest JSON
│   ├── manifest-patcher.sh   # append rules (bridge add-rule)
│   ├── probe.sh              # capability tier resolution
│   ├── auto-respond.sh       # fallback prompt-detection policy engine
│   └── manifests/            # per-CLI capability manifests (JSON)
├── drivers/                  # per-CLI driver scripts
│   ├── claude-p.sh           # headless claude -p
│   ├── claude-tmux.sh        # interactive claude via tmux
│   ├── codex.sh              # v2 stub (rc=99)
│   ├── codex-tmux.sh         # v2 stub (rc=99)
│   ├── agy.sh                # v2 stub (rc=99)
│   └── agy-tmux.sh           # v2 stub (rc=99)
├── docs/
│   ├── design.md             # architecture + design rationale
│   ├── cli-reference.md      # complete CLI surface
│   ├── adding-a-driver.md    # contract for new drivers
│   └── skill-integration.md  # how consumers invoke bridge
├── tests/                    # bats-core suites
│   ├── unit/
│   ├── integration/
│   └── contract/
└── install.sh                # symlink-or-copy install with --check / --uninstall
```

### Independence invariant

Nothing under `tools/agent-bridge/` `source`s files outside the sub-project. Verified by:

```bash
grep -rn '\.\./' tools/agent-bridge/ | grep -v '^tools/agent-bridge/' | grep -vE '(README|design|\.bats)'
```

Must return empty. This makes `tools/agent-bridge/` `git filter-branch`-extractable into its own repo.

---

## Adding a CLI

See `docs/adding-a-driver.md`. The contract is small: a `drivers/<cli>.sh` script that defines `drv_launch_<cli>()`, accepts the standard env-var inputs (`BRIDGE_CLI`, `BRIDGE_PROFILE`, `BRIDGE_MODEL`, `BRIDGE_PROMPT_FILE`, `BRIDGE_WORKSPACE`, `BRIDGE_ARTIFACT`, etc.), and exits with the standardized code table. Add a `lib/manifests/<cli>.json` declaring the binary, version floor, and tier dependencies.

---

## Using bridge from another project

`bridge` is designed to be called from any consumer. Two installation patterns:

**Symlink install (default)** — single source of truth, source checkout updates flow through immediately. Best for active development.

```bash
cd /path/to/checkout/tools/agent-bridge
bash install.sh
# bridge is now at $HOME/.local/bin/bridge → /path/to/checkout/tools/agent-bridge/bin/bridge
```

**Copy install (`--copy`)** — installs a frozen snapshot. Best for production / CI / system-wide use.

```bash
bash install.sh --copy --prefix=/usr/local
```

The consumer then calls `bridge launch ...` like any other system command. Detection script:

```bash
if command -v bridge >/dev/null 2>&1 && bridge --json version | jq -e '.schema_version == 1' >/dev/null; then
    bridge launch --cli=claude-tmux ...
else
    # fallback to direct CLI invocation
fi
```

The parent `evolve-loop` project's `claude-tmux` adapter follows exactly this pattern and delegates to `bridge` by default when available. See `scripts/cli_adapters/claude-tmux.sh` in evolve-loop for the integration reference. To force-disable the delegation (for CI bit-for-bit reproducibility, or to debug the native prototype path):

```bash
export EVOLVE_USE_BRIDGE=0
```

---

## Non-goals

- **No vendor-API impersonation.** No header rewriting, fingerprint manipulation, or evasion of the underlying CLI's intended behavior. `bridge` passes through to the CLI as installed.
- **No multi-turn dialog driving in v1.** Each `bridge launch` is one round-trip. Multi-turn is the orchestrator's responsibility.
- **No persistent-session optimization in v1.** Every launch spawns a fresh tmux session (the v0.5a-c work added session-name + resume primitives; performance optimization is v2).
- **Codex / Antigravity drivers are stubs in v1.** They return exit 99 when called. Manifests are in place so callers can probe and degrade gracefully; the actual driver implementations land with v2.

---

## References

- Design rationale: `docs/design.md`
- Full CLI surface: `docs/cli-reference.md`
- Driver contract: `docs/adding-a-driver.md`
- Skill consumer pattern: `docs/skill-integration.md`
