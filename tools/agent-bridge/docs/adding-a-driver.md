# Adding a new CLI driver

To support a new CLI (e.g., `cursor-agent`, a future MCP-only CLI), add three things.

## 1. Create the manifest

Path: `lib/manifests/<cli>.json`

Use an existing manifest as a template (`claude-tmux.json` is the most complete). Required fields: `schema_version`, `cli`, `binary`. See `docs/design.md` §5 for the schema.

For a CLI you haven't fully integrated yet, mark `"stub": true` and `"default_tier": "none"`; probe will report it but bridge won't try to launch.

## 2. Create the driver script

Path: `drivers/<cli>.sh`

Contract:

- Defines a function `drv_launch_<cli-with-_>` (e.g., `drv_launch_cursor_agent`)
- The function takes no arguments; instead it reads local vars from the caller's scope (`cmd_launch` in `bin/bridge`):
  - `$cli` `$profile` `$effective_model` `$prompt_file` `$workspace`
  - `$stdout_log` `$stderr_log` `$artifact`
  - `$cycle` `$worktree` `$agent`
  - `$require_full` `$allow_bypass`
- It also reads `bridge_profile_*` (from `profile_load`) and may call `manifest_load <cli>` to populate `bridge_manifest_*`
- It returns one of the documented exit codes (see `docs/design.md` §7)
- It must NOT mutate the caller's local vars

### Minimal driver skeleton

```bash
#!/usr/bin/env bash
# drivers/cursor-agent.sh — driver for cursor-agent (TODO)

drv_launch_cursor_agent() {
  # Preflight
  command -v cursor-agent >/dev/null 2>&1 || { echo "[cursor-agent] binary missing" >&2; return $EC_MISSING_BINARY; }

  # Cost-leak guards
  if [[ -n "${ANTHROPIC_API_KEY:-}" ]]; then
    echo "[cursor-agent] ANTHROPIC_API_KEY set — would bill API" >&2
    return $EC_COST_LEAK
  fi

  mkdir -p "$workspace"

  # Substitute placeholders
  local prompt_content
  prompt_content="$(cat "$prompt_file")"
  prompt_content="${prompt_content//\$ARTIFACT_PATH/$artifact}"

  # ... your driver-specific logic ...

  # Write logs
  echo "[cursor-agent] done" >&2
  return 0
}
```

## 3. Add a probe + integration test

### Probe test

Add a `@test` to `tests/integration/probe-tier.bats` (or write a new bats file under `integration/`) that asserts `bridge probe` reports the new CLI with the expected tier.

### Launch test

Pattern after `tests/integration/launch-claude-p.bats`:

- Use `setup_file` to make ONE live invocation
- `setup()` skips on `BRIDGE_RUN_LIVE_LLM!=1`
- Multiple `@test` cases assert on the shared workspace
- `teardown_file` cleans up

---

## Auto-respond integration

If your CLI is interactive (has a REPL or shows prompts), populate `interactive_prompts[]` in the manifest. For each prompt:

```json
{
  "name": "snake_case_descriptive_name",
  "regex": "an EREXTENDED regex matched against the pane",
  "response_keys": "y,Enter",       // or null for policy=escalate
  "policy": "auto_respond",         // or "escalate"
  "note": "why this rule is needed"
}
```

Then call `auto_respond_tick "$session"` periodically in your driver's wait-loop (see `drivers/claude-tmux.sh` for the pattern).

---

## Promoting a stub to a full driver

1. Replace `drv_launch_<cli>` to do real work
2. Flip `"stub": false` in the manifest
3. Set the right `default_tier` and `tier_dependencies`
4. Add the launch test

---

## Capability schema migrations

Currently `schema_version=1`. If you change the manifest schema (rename a field, add a new required one), bump `schema_version` and update:

- `lib/manifest-loader.sh` to handle both versions during the migration window
- `docs/design.md` §5 to document the new field
- All manifests (`lib/manifests/*.json`) to bump and conform

A `schema_version` mismatch causes `manifest_load` to fail with rc=1 — drivers won't run against an unmigrated manifest.
