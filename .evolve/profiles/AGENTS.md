# .evolve/profiles/ — Agent Permission Profile Schema

> **Directory purpose**: Per-role permission profiles for evolve-loop subagents.
> Each `*.json` file grants a role least-privilege tool access, budget caps,
> sandbox write paths, and scheduling hints. Loaded exclusively by
> `scripts/dispatch/subagent-run.sh` before spawning a `claude -p` subprocess.

## Schema Reference

Each profile is a JSON object. All fields listed below.

### Identity

| Field | Type | Description |
|---|---|---|
| `name` | string | Role name (matches filename without `.json`) |
| `role` | string | Pipeline role: `scout`, `builder`, `auditor`, `tdd-engineer`, `orchestrator`, … |
| `cli` | string | Target CLI binary (`claude`). Builder locked to `claude` until adapter parity. |

### Model selection

| Field | Type | Description |
|---|---|---|
| `model_tier_default` | string | Default capability tier from `modelcatalog.CanonicalTiers`: `fast`, `balanced`, `deep`. Builder defaults to `balanced`; Auditor to `deep`. Claude-specific tier labels are forbidden because they miss for non-Claude CLIs. |
| `model_tier_overrides` | object | Named condition → tier overrides. Keys: `ultrathink_strategy`, `m_complex_5plus_files`, `audit_retry_2plus`, `s_complex_with_cache`. |
| `model_tier_envelope` | object | `{ min, default, max }` — runtime may auto-escalate/downgrade within this envelope. |
| `cross_family_with` | string | Role name whose model family MUST differ (breaks same-model-judge sycophancy). |

### Tool access

| Field | Type | Description |
|---|---|---|
| `allowed_tools` | array | Explicit allowlist. Format: tool names (`Read`, `Write`, `Bash`) and `Skill(<name>)` entries. |
| `disallowed_tools` | array | Deny-override list. Path-scoped entries use `Edit(<path>)` / `Write(<path>)` / `Bash(<cmd>:*)`. Evaluated after `allowed_tools`. |

**disallowed_tools path patterns (examples):**

```
Edit(.evolve/state.json)          # deny edit to specific file
Write(acs/regression-suite/**)    # deny writes under a subtree
Bash(git push:*)                  # deny git push in any form
Bash(perl:*)                      # deny all perl invocations
```

### Scheduling and budget

| Field | Type | Description |
|---|---|---|
| `parallel_eligible` | bool | `false` for single-writer roles (Builder, Orchestrator, Intent, TDD). `true` for read-only roles (Scout sub-tasks, Evaluator). Enforced by `sequential-write-discipline.md`. |
| `max_budget_usd` | number | Per-invocation USD ceiling. `scripts/dispatch/subagent-run.sh` passes as `--budget-usd`. |
| `max_turns` | int | Hard turn ceiling; `claude -p` exits at this count. |
| `turn_budget_hint` | int | Soft guidance written into the system prompt checkpoint note. |
| `turn_budget_guidance` | object | `{ target_turns, checkpoint_at_turn, hard_exit_at_turn, checkpoint_note }` |
| `effort_level` | string | `low`, `medium`, `high` — informs orchestrator scheduling. |

### Sandbox

| Field | Type | Description |
|---|---|---|
| `sandbox.enabled` | bool | When `true`, wraps the subprocess in `sandbox-exec` (macOS) or `bwrap` (Linux). |
| `sandbox.write_subpaths` | array | Absolute or `{worktree_path}`-templated paths the subprocess may write. |
| `sandbox.read_only_repo` | bool | When `true`, repo root is mounted read-only. Used for Auditor/Evaluator roles. |
| `sandbox.deny_subpaths` | array | Explicit deny mounts (e.g., `.evolve/state.json`, `.evolve/ledger.jsonl`). |
| `sandbox.allow_network` | bool | When `false`, outbound network is blocked at the OS level. |

### Invocation extras

| Field | Type | Description |
|---|---|---|
| `add_dir` | array | Additional working directory paths mounted into the subprocess. |
| `extra_flags` | array | Additional `claude` CLI flags appended verbatim (e.g., `--no-session-persistence`). |
| `permission_mode` | string | `bypassPermissions` for autonomous operation. |
| `output_artifact` | string | Template path for the role's required output file. |
| `challenge_token_required` | bool | When `true`, `subagent-run.sh` generates a challenge token and injects it; ship-gate validates it on the artifact's first line. |
| `isolation` | string | `worktree` — per-cycle git worktree provisioned by `run-cycle.sh`. |
| `context_clear_trigger_tokens` | int | Auto-clear context when prompt exceeds this token count. |
| `research_quota` | object | `{ web_search, web_fetch, kb_search }` per-phase call caps. |

## Authoring a new profile

1. Copy `builder.json` as a starting template.
2. Set `name` + `role` + `model_tier_default`.
3. Restrict `allowed_tools` to only what the role needs.
4. Set `parallel_eligible: false` for any role that writes files (single-writer invariant).
5. Set `sandbox.read_only_repo: true` for read-only or audit-only roles.
6. Register the profile path in `scripts/dispatch/subagent-run.sh` role→profile map.

## Relationship to pipeline enforcement

`scripts/guards/role-gate.sh` reads the active role's profile at runtime and denies
any tool call that matches a `disallowed_tools` pattern. Violations are logged to
`.evolve/ledger.jsonl` and terminate the subagent invocation with `ROLE_GATE_VIOLATION`.
