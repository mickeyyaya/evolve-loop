# bridge — skill integration guide

> **Audience**: anyone driving `bridge` from a Claude Code skill, a shell script, CI, or another tool.
> **Goal**: a stable, machine-readable CLI contract you can write a skill against without reading bridge's source.

---

## TL;DR — the five things a skill calls

```bash
bridge --json probe                                                 # 1. discovery
bridge --json doctor [--cli=NAME] [--deep]                          # 2. pre-flight
bridge --json launch --cli=NAME --profile=... --prompt-file=... ... # 3. execute
bridge --json report --workspace=DIR                                # 4. post-hoc
bridge --json selftest --suite=unit                                 # 5. self-verify
```

Every command emits structured output to stdout on success. Stable exit codes signal success/failure categories.

---

## 1. Discovery — `bridge probe`

```bash
bridge --json probe
```

Returns: JSON object with `{os, results: [...]}`. Each result is `{cli, tier, binary, version, stub}`.

A skill should iterate `.results[]`, filter by `tier != "none"`, and pick a CLI that matches its needs.

```bash
# Pick the first hybrid-or-better claude-family CLI:
bridge --json probe | jq -r '.results[]
  | select(.cli | startswith("claude") and (.tier != "none"))
  | .cli' | head -1
```

---

## 2. Pre-flight — `bridge doctor`

```bash
bridge --json doctor [--cli=NAME] [--deep]
```

**Shallow (default)**: zero-cost auth + binary checks. Returns instantly.
**Deep (`--deep`)**: live noop call per headless CLI. Costs ~$0.01 per CLI on cheapest models.

Each result is `{cli, binary, auth, env_warnings, deep_probe, verdict}` with `verdict ∈ {ready, warning, blocked}`.

```bash
# Block launch until a specific CLI is ready:
if bridge --json doctor --cli=claude-tmux | jq -e '.results[0].verdict == "ready"' >/dev/null; then
  echo "claude-tmux is ready"
else
  echo "claude-tmux is not ready; see:"
  bridge doctor --cli=claude-tmux   # human-readable explanation
  exit 1
fi
```

**Exit codes**: `0` all ready, `1` ≥1 warning, `2` ≥1 blocked.

---

## 3. Execute — `bridge launch`

```bash
bridge --json launch \
  --cli=NAME \
  --profile=PATH \
  --model=MODEL \
  --prompt-file=PATH \
  --workspace=DIR \
  --stdout-log=PATH --stderr-log=PATH --artifact=PATH \
  [--allow-bypass] [--require-full] [--dry-run] [--validate-only]
```

Or via env vars (drop-in for the common cli_adapter contract):

```bash
export PROFILE_PATH=... RESOLVED_MODEL=... PROMPT_FILE=... WORKSPACE_PATH=...
export STDOUT_LOG=... STDERR_LOG=... ARTIFACT_PATH=...
export BRIDGE_CLI=claude-tmux BRIDGE_ALLOW_BYPASS=1
bridge --json launch
```

Flags always win when both flag and env var are set.

**Always run dry-run first when integrating** to ensure the artifact path + workspace + log paths are valid before spending LLM credits:

```bash
bridge launch --dry-run ...   # writes synthetic outputs; rc=0 if plumbing is right
```

**Prompt-file substitutions** the driver performs:

| Placeholder | Replaced with |
|---|---|
| `$ARTIFACT_PATH` | the value of `--artifact` |
| `$CHALLENGE_TOKEN` | a fresh hex token; also written to `workspace/challenge-token.txt` |

**Exit codes**: see `bridge help`. `0` = artifact written; `81` = artifact timeout (the agent didn't write the file in 300s); `85` = unknown interactive prompt (auto-respond escalation); etc.

---

## 4. Post-hoc — `bridge report`

```bash
bridge --json report --workspace=DIR [--artifact-name=NAME]
```

Returns a JSON object with `{workspace, scanned_at, verdict, artifact, challenge_token, logs, escalation_report, billing_snapshots}`.

A skill should check `.verdict`:

| Verdict | Meaning |
|---|---|
| `complete` | artifact present + (challenge token matches if used) |
| `escalated` | auto-respond bailed; `escalation_report.json` is in the workspace |
| `incomplete` | no artifact, no escalation report |
| `incomplete-token-mismatch` | artifact present but the token doesn't match (suspicious) |
| `unknown` | workspace empty or unreadable |

---

## 5. Self-verify — `bridge selftest`

```bash
bridge --json selftest [--suite=unit|integration|billing|all] [--filter=PATTERN] [--live]
```

A skill running in CI should gate on `.totals.failed == 0`:

```bash
if bridge --json selftest --suite=unit | jq -e '.totals.failed == 0' >/dev/null; then
  echo "bridge healthy"
else
  echo "bridge regression detected"; exit 1
fi
```

**Exit codes**: `0` all pass, `1` failures, `127` bats not installed.

---

## Stable contract

### Subcommand list (v0.1.0)

| Subcommand | Output format (default) | `--json` mode |
|---|---|---|
| `probe` | JSON | (no-op; already JSON) |
| `doctor` | human table | JSON object |
| `launch` | workspace files + stderr trace | + one-line JSON summary on stdout |
| `report` | JSON | (no-op; already JSON) |
| `selftest` | bats pretty output | JSON summary |
| `add-rule` | stderr trace + manifest write | + JSON status on stdout |
| `version` | semver string | `{version, schema_version}` |
| `help` | human text | (no-op) |

### Top-level flags

| Flag | Effect |
|---|---|
| `--json` | Switch every subcommand to machine-readable JSON output where applicable |

Place `--json` **before** the subcommand: `bridge --json doctor` (not `bridge doctor --json`).

### Exit-code matrix

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | warning (e.g. `doctor` found a warning) OR test failure (`selftest`) |
| 2 | hard-blocked (e.g. `doctor` found a blocked CLI, claude-tmux without `--allow-bypass`) |
| 3 | cost-leak guard tripped (env-var leak detected) |
| 10 | bad flags or missing required arg |
| 80 | REPL boot timeout (tmux drivers) |
| 81 | artifact never appeared |
| 85 | unknown interactive prompt → escalation report written |
| 86 | auto-respond loop guard tripped |
| 99 | `--require-full` set, CLI doesn't reach `full`/`hybrid` tier |
| 127 | required external binary missing |

### JSON schema version

Every JSON-producing subcommand includes (or should be assumed to include) a `schema_version` field at the top level or per-result. Currently `1` across the board. Bumped only on backward-incompatible changes; new optional fields appear without a bump.

**Feature-detect**, don't version-check:

```bash
# GOOD: tolerate missing fields gracefully
verdict=$(bridge --json doctor | jq -r '.results[0].verdict // "unknown"')

# BAD: hardcode schema_version=1 expectations
[[ $(bridge --json version | jq .schema_version) == 1 ]] || error
```

---

## Anti-patterns

A skill should NOT:

1. **Set `ANTHROPIC_API_KEY` and expect bridge to honor it.** Bridge's cost-leak guards refuse to launch claude-p / claude-tmux when this is set — it would invalidate subscription billing. If you genuinely need API-key billing, use a separate tool, not bridge.

2. **Read driver stdout/stderr logs as the primary contract.** Use `bridge report` instead. The log file format is not a stability commitment.

3. **Hard-code paths to internal lib/ files.** All consumer interaction goes through `bin/bridge`. The `lib/` directory is implementation, not API.

4. **Skip `bridge doctor` before `bridge launch` in CI.** A missing auth file or expired token is a common, recoverable error class — catching it pre-launch keeps logs clean and saves the cost of a failed run.

5. **Mutate manifest files directly.** Use `bridge add-rule` to append `interactive_prompts[]` entries; that subcommand validates the schema. Hand-editing risks invalid JSON or dropped fields.

---

## Quickstart: a minimal skill recipe

```bash
#!/usr/bin/env bash
# minimal-skill.sh — run one bridge invocation with all guards in place
set -uo pipefail

CLI="${1:-claude-tmux}"
PROMPT_FILE="${2:?prompt file required}"
WS="$(mktemp -d /tmp/skill-XXXXXX)"

# 1. Discovery (cached at the start of a skill session)
TIER=$(bridge --json probe | jq -r ".results[] | select(.cli==\"$CLI\") | .tier")
if [[ "$TIER" == "none" ]]; then
  echo "[skill] $CLI not available (tier=none)" >&2
  exit 1
fi

# 2. Pre-flight (shallow; cheap)
if ! bridge --json doctor --cli="$CLI" | jq -e '.results[0].verdict == "ready"' >/dev/null; then
  echo "[skill] $CLI not ready:" >&2
  bridge doctor --cli="$CLI" >&2
  exit 1
fi

# 3. Dry-run plumbing check (no cost)
bridge launch --dry-run \
  --cli="$CLI" --profile=./profile.json --model=haiku \
  --prompt-file="$PROMPT_FILE" --workspace="$WS" \
  --stdout-log="$WS/stdout.log" --stderr-log="$WS/stderr.log" \
  --artifact="$WS/artifact.md" \
  ${BRIDGE_ALLOW_BYPASS:+--allow-bypass} >/dev/null

# 4. Real launch
bridge --json launch \
  --cli="$CLI" --profile=./profile.json --model=haiku \
  --prompt-file="$PROMPT_FILE" --workspace="$WS" \
  --stdout-log="$WS/stdout.log" --stderr-log="$WS/stderr.log" \
  --artifact="$WS/artifact.md" \
  ${BRIDGE_ALLOW_BYPASS:+--allow-bypass}

# 5. Post-hoc check
VERDICT=$(bridge --json report --workspace="$WS" | jq -r .verdict)
echo "[skill] result: $VERDICT (workspace: $WS)"
[[ "$VERDICT" == "complete" ]]
```

This pattern works across all 6 backends (`claude-p`, `claude-tmux`, `codex`, `codex-tmux`, `agy`, `agy-tmux`) without modification — that's the bridge's promise to skill authors.

---

## References

- `docs/design.md` — architecture and rationale
- `docs/cli-reference.md` — exhaustive flag tables per subcommand
- `docs/adding-a-driver.md` — extending bridge with a new CLI
