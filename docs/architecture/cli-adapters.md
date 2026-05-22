# CLI adapters — native + optional bridge delegation

> **Audience**: operators considering enabling bridge for evolve-loop, contributors editing `legacy/scripts/cli_adapters/*.sh`, anyone debugging why a subagent didn't behave as expected.

evolve-loop's `legacy/scripts/dispatch/subagent-run.sh` dispatches each cycle phase to a per-CLI adapter under `legacy/scripts/cli_adapters/`. The contract is well-defined: 8 mandatory env vars + a standard exit-code matrix. As of v10.18.0, **claude-tmux** has an **optional** delegation path to an external CLI called `bridge` — installed separately by the operator. All other adapters (`claude.sh`, `codex.sh`, `agy.sh`, `gemini.sh`) are unchanged.

---

## The adapter contract (unchanged)

Every `legacy/scripts/cli_adapters/${CLI}.sh` accepts the following env vars from `subagent-run.sh`:

| Env var | Meaning |
|---|---|
| `PROFILE_PATH` | absolute path to agent profile JSON |
| `RESOLVED_MODEL` | model alias (haiku/sonnet/opus) |
| `PROMPT_FILE` | absolute path to prompt text |
| `CYCLE` | integer cycle number (or 0 for probes) |
| `WORKSPACE_PATH` | absolute path to per-cycle output dir |
| `STDOUT_LOG` | where adapter writes LLM stdout |
| `STDERR_LOG` | where adapter writes diagnostics |
| `ARTIFACT_PATH` | path the agent must write |
| `WORKTREE_PATH` (opt) | git worktree dir |
| `AGENT` (opt) | role label |

Exit codes: see each adapter file's header.

---

## Optional `bridge` integration (4 adapters: claude / claude-tmux / codex / agy)

### What is bridge?

`bridge` is a multi-CLI tmux-driven AI agent bridge — a separate, user-installable CLI that wraps Claude / Codex / Antigravity CLIs behind a uniform interface. Its **value-add** is a battle-tested implementation: better REPL prompt detection for tmux variants, auto-respond fallback for permission prompts, billing-snapshot verification, stable JSON-over-stdout contract for skill consumers, and a single CLI surface across 3 different vendor CLIs.

Bridge source is **not** distributed in this repository. It is the operator's responsibility to install.

### Per-adapter mapping

| evolve-loop adapter | Bridge CLI | Default-on? |
|---|---|---|
| `legacy/scripts/cli_adapters/claude.sh` | `claude-p` (headless `claude -p`) | ✓ |
| `legacy/scripts/cli_adapters/claude-tmux.sh` | `claude-tmux` (interactive via tmux) | ✓ |
| `legacy/scripts/cli_adapters/codex.sh` | `codex` (`codex exec`) | ✓ |
| `legacy/scripts/cli_adapters/agy.sh` | `agy` (`agy -p`) | ✓ |
| `legacy/scripts/cli_adapters/gemini.sh` | *no bridge support* | n/a (always native) |

When the gate fires in any of the 4 supported adapters, control transfers to bridge via `exec`. Bridge picks up the standard env vars (PROFILE_PATH, RESOLVED_MODEL, PROMPT_FILE, WORKSPACE_PATH, STDOUT_LOG, STDERR_LOG, ARTIFACT_PATH, CYCLE, WORKTREE_PATH, AGENT) from its env-var fallback contract.

**Behavioral note for `codex.sh` and `agy.sh`**: the existing native paths in these two adapters use a HYBRID-delegate-to-claude pattern (i.e., they run claude when claude is installed, not the actual codex/agy binary). With bridge default-on, these adapters now run the *real* codex / agy CLIs. To restore the old HYBRID-to-claude behavior, set `EVOLVE_USE_BRIDGE=0`.

### Activation (default-on with per-CLI auto-fallback)

The integration is **default-on** when bridge is installed AND supports the specific CLI being dispatched. The gate at the top of each adapter delegates to bridge when ALL of these hold:

1. `EVOLVE_USE_BRIDGE` is unset OR set to anything other than `"0"` (default).
2. `bridge` is on PATH.
3. `bridge --json version` reports `schema_version=1`.
4. `bridge --json probe --cli=<NAME>` reports `tier != "none"` (i.e., bridge has a manifest entry for this CLI AND the underlying binary is on PATH).

If ANY of those is false, the gate falls back to the **native adapter** quietly — no error to the operator. This means:
- An evolve-loop installation without bridge → runs native adapters (unchanged behavior)
- An evolve-loop installation with bridge but missing the underlying CLI binary (e.g., `claude` not installed) → falls back to native, which surfaces a clearer error
- An evolve-loop installation with bridge for some CLIs but not others → uses bridge for the supported ones, native for the rest

To **force-disable bridge across the board** (e.g., to debug the native path, or for bit-for-bit reproducibility in CI):

```bash
export EVOLVE_USE_BRIDGE=0
```

### Why default-on?

The integration was opt-in for the first iteration to validate it works without breaking native users. After end-to-end verification (bridge installed + EVOLVE_USE_BRIDGE=1 + Haiku probe → subscription-billed cycle ran cleanly), the default flipped to ON: any user who has bothered to install bridge wants the integration to take effect. Operators who specifically prefer the native prototype set `EVOLVE_USE_BRIDGE=0`.

### Installing bridge

Bridge is maintained separately. As of 2026-05-21 the source lives on a private branch (`bridge-local`) of this repository; the maintainer can share install instructions on request. The basic shape:

```bash
# In the bridge-local checkout:
bash tools/agent-bridge/install.sh           # symlinks bin/bridge → $HOME/.local/bin/bridge
bash tools/agent-bridge/install.sh --check   # verify install (symlink + PATH + schema_version=1)
bridge --json doctor                          # check per-CLI auth + binary state
```

If `$HOME/.local/bin` is not on your PATH, add it:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

To uninstall:

```bash
bash tools/agent-bridge/install.sh --uninstall
```

### Default-on activation

Once bridge is installed, the integration is active by default. No env var setting needed:

```bash
# Just run as normal:
bash legacy/scripts/dispatch/subagent-run.sh ...
# claude-tmux adapter delegates to bridge automatically
```

You'll see this log line on stderr when the gate fires:

```
[claude-tmux] delegating to bridge launch --cli=claude-tmux (default; set EVOLVE_USE_BRIDGE=0 to disable)
```

### Force-disable (CI guardrail or debugging native)

```bash
export EVOLVE_USE_BRIDGE=0
```

This is recommended for CI configurations that need bit-for-bit reproducibility — even if bridge happens to be installed on the runner, the native adapter is used. Also useful when debugging the prototype adapter's behavior in isolation.

### Schema-version compatibility

The gate refuses to delegate if `bridge --json version | jq -r .schema_version` returns anything other than `"1"`. On mismatch, it emits a WARN to stderr and falls through to the native adapter. As bridge evolves, version mismatches will be flagged loudly here.

### Failure modes

| Scenario | What happens |
|---|---|
| `bridge` not on PATH (regardless of env) | Gate silently falls through to native adapter. |
| `bridge --json version` exits non-zero | Gate WARNs and falls through. |
| Bridge schema mismatch (≠ `"1"`) | Gate WARNs (`schema_version='X' (expected 1)`) and falls through. |
| Bridge can't handle this CLI (manifest missing) | Gate WARNs (`tier=none (manifest missing or binary not on PATH)`) and falls through. |
| Underlying CLI binary missing (e.g., `claude` not on PATH) | Gate WARNs (`tier=none`) and falls through. Native adapter would also fail, but its error message is more familiar. |
| Bridge runs but returns non-zero | The `exec` propagates bridge's exit code. Cycle fails the same way as a native adapter failure would. |
| `EVOLVE_USE_BRIDGE=0` set | Gate doesn't fire. Native adapter runs as before. |

### Why these 4 adapters?

All 4 are CLIs that bridge has a verified driver for (claude-p, claude-tmux, codex, agy). Each native adapter today has either:
  - Significant native complexity that bridge re-implements with better guarantees (`claude-tmux`), OR
  - A HYBRID-degrade pattern (the codex/agy adapters historically delegate to claude when their underlying binary isn't recognized — bridge gives them real CLI drivers).

`gemini.sh` is excluded because bridge does not yet support gemini as a backend. The native gemini adapter runs unchanged regardless of `EVOLVE_USE_BRIDGE`.

Operators who want bit-for-bit reproducibility of the historical (native) behavior should set `EVOLVE_USE_BRIDGE=0` once per shell or CI run.

---

## Operator verification

To verify the integration on your host:

```bash
# 1. Install bridge (separate concern; see Installing section).
bash tools/agent-bridge/install.sh --check && echo "bridge OK"

# 2. Run a probe cycle WITHOUT bridge enabled — should use native adapter.
EVOLVE_USE_BRIDGE=0 bash legacy/scripts/cli_adapters/claude-tmux.sh   # expect: prototype path
# (will fail without env vars; check the error mentions PROFILE_PATH, not bridge)

# 3. Same call WITH bridge enabled — should delegate.
EVOLVE_USE_BRIDGE=1 bash legacy/scripts/cli_adapters/claude-tmux.sh   # expect: "[claude-tmux] delegating to bridge ..."
```

Both invocations fail rc-nonzero in this contrived example (env vars missing); the point of the smoke is the **stderr log line** which tells you which path ran.

For a real cycle, set the 8 mandatory env vars and run via `subagent-run.sh` normally with `EVOLVE_USE_BRIDGE=1`.

---

## References

- `legacy/scripts/cli_adapters/claude-tmux.sh` — the prototype adapter + the new gate (top ~30 lines)
- `legacy/scripts/dispatch/subagent-run.sh` — the upstream caller (unchanged)
- Bridge source + design — separate; not in this repository
