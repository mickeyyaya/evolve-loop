# bridge — CLI reference

Full surface for v0.1.0.

```text
bridge <subcommand> [flags]
```

---

## Subcommands

| Subcommand | Purpose |
|---|---|
| `launch` | Run a subagent invocation through a chosen CLI (the main verb) |
| `probe` | Detect available CLIs and capability tiers (JSON output) |
| `validate` | Reserved — currently stubs to 0 |
| `report` | Reserved — currently stubs to 0 |
| `add-rule` | Append an `interactive_prompts` rule to a CLI manifest |
| `version` | Print bridge version |
| `help` | Print top-level help |

---

## `bridge launch`

```bash
bridge launch \
  --cli=NAME \
  --profile=PATH \
  --model=MODEL \
  --prompt-file=PATH \
  --workspace=DIR \
  --stdout-log=PATH \
  --stderr-log=PATH \
  --artifact=PATH \
  [--cycle=N] [--worktree=DIR] [--agent=NAME] \
  [--require-full] [--validate-only] [--allow-bypass] \
  [--permission-mode=MODE]
```

### Required flags

| Flag | Description |
|---|---|
| `--cli=NAME` | `claude-p` \| `claude-tmux` (v2: `codex` \| `agy`) |
| `--profile=PATH` | Absolute path to agent profile JSON |
| `--model=MODEL` | `haiku` \| `sonnet` \| `opus` \| `auto` (`auto` resolves to `profile.model`) |
| `--prompt-file=PATH` | Absolute path to prompt text |
| `--workspace=DIR` | Absolute path to per-invocation output dir |
| `--stdout-log=PATH` | Where to write LLM stdout |
| `--stderr-log=PATH` | Where to write diagnostics |
| `--artifact=PATH` | Expected artifact path the agent must write |

### Optional flags

| Flag | Default | Description |
|---|---|---|
| `--cycle=N` | `0` | Cycle number (logging only) |
| `--worktree=DIR` | `$PWD` | Working dir for the driver |
| `--agent=NAME` | `probe` | Agent role label |
| `--require-full` | off | Exit 99 if CLI doesn't reach `full` or `hybrid` tier |
| `--validate-only` | off | Dry-run: parse + print resolved config, exit 0 (no driver call) |
| `--allow-bypass` | off | Acknowledge `--dangerously-skip-permissions` use (claude-tmux requires this UNLESS `--permission-mode` is set) |
| `--permission-mode=MODE` | unset | Pass through to `claude --permission-mode MODE`. Valid: `plan` \| `default` \| `acceptEdits` \| `bypassPermissions` \| `auto` \| `dontAsk`. Precedence: CLI flag > `BRIDGE_PERMISSION_MODE` env > `profile.permission_mode`. In plan mode the claude-tmux driver replaces `--dangerously-skip-permissions` with `--permission-mode plan` (the bypass acknowledgment is NOT required because plan mode disables writes by design). |

### Prompt-file substitutions

The driver substitutes these placeholders before invoking the CLI:

| Placeholder | Replaced with |
|---|---|
| `$ARTIFACT_PATH` | the value of `--artifact` |
| `$CHALLENGE_TOKEN` | a fresh `openssl rand -hex 8` token; the token is also written to `workspace/challenge-token.txt` |

---

## `bridge probe`

```bash
bridge probe [--cli=NAME]
```

Output: JSON to stdout.

```json
{
  "os": "Darwin/25.4.0",
  "results": [
    {"cli": "claude-p", "tier": "full", "binary": "/usr/.../claude", "version": "2.1.146", "stub": false},
    {"cli": "claude-tmux", "tier": "hybrid", "binary": "/usr/.../claude", "version": "2.1.146", "stub": false},
    {"cli": "codex", "tier": "none", "binary": null, "version": null, "stub": true},
    {"cli": "agy",   "tier": "none", "binary": null, "version": null, "stub": true}
  ]
}
```

Tier semantics: see `docs/design.md` §5.

---

## `bridge add-rule`

```bash
# From an escalation report:
bridge add-rule --escalation=REPORT --regex=R --response=KEYS [--policy=auto_respond]

# Direct:
bridge add-rule --cli=NAME --regex=R --response=KEYS --policy=auto_respond \
                [--name=N] [--note=TEXT]
```

| Flag | Description |
|---|---|
| `--escalation=PATH` | Path to `workspace/escalation-report.json` from a failed run |
| `--cli=NAME` | Target CLI (auto-detected from `--escalation` if absent) |
| `--name=N` | Rule name (auto-generated from CLI + timestamp if absent) |
| `--regex=R` | Extended-regex pattern matched against the pane |
| `--response=KEYS` | Comma-separated tmux key names: `"y,Enter"`, `"Enter"`, `"3,Enter"` |
| `--policy=P` | `auto_respond` \| `escalate` (default: `escalate` when no `--response`) |
| `--note=TEXT` | Human-readable note saved with the rule |

Duplicates (by name) are refused with rc=2.

---

## Exit codes

See `docs/design.md` §7.

---

## Env vars

| Var | Effect |
|---|---|
| `BRIDGE_LIB_DIR` | Override `lib/` location (for tests / vendoring) |
| `BRIDGE_RUN_LIVE_LLM=1` | Enable live-LLM integration tests in `tests/integration/` |
| `BRIDGE_BILLING_TESTS=1` | Enable opt-in billing-snapshot tests |
| `BRIDGE_ALLOW_ANTHROPIC_BASE_URL=1` | Permit running when `ANTHROPIC_BASE_URL` is set (custom proxy) |
| `BRIDGE_INSTALL_DIR` | Override `install.sh` target dir (default: `$HOME/.local/bin`) |
| `BRIDGE_KEEP_WS=1` | Keep test workspaces around (debugging) |
