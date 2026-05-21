# CLI adapters — native + optional bridge delegation

> **Audience**: operators considering enabling bridge for evolve-loop, contributors editing `scripts/cli_adapters/*.sh`, anyone debugging why a subagent didn't behave as expected.

evolve-loop's `scripts/dispatch/subagent-run.sh` dispatches each cycle phase to a per-CLI adapter under `scripts/cli_adapters/`. The contract is well-defined: 8 mandatory env vars + a standard exit-code matrix. As of v10.18.0, **claude-tmux** has an **optional** delegation path to an external CLI called `bridge` — installed separately by the operator. All other adapters (`claude.sh`, `codex.sh`, `agy.sh`, `gemini.sh`) are unchanged.

---

## The adapter contract (unchanged)

Every `scripts/cli_adapters/${CLI}.sh` accepts the following env vars from `subagent-run.sh`:

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

## Optional `bridge` integration (claude-tmux only)

### What is bridge?

`bridge` is a multi-CLI tmux-driven AI agent bridge — a separate, user-installable CLI that wraps Claude / Codex / Antigravity CLIs behind a uniform interface. Its **value-add for claude-tmux specifically** is a more battle-tested implementation of the same prototype: better REPL prompt detection, auto-respond fallback for permission prompts, billing-snapshot verification, and a stable JSON-over-stdout contract for skill consumers.

Bridge source is **not** distributed in this repository. It is the operator's responsibility to install.

### Activation gates (both required)

The gate at the top of `scripts/cli_adapters/claude-tmux.sh` fires ONLY when **both** are true:

1. `EVOLVE_USE_BRIDGE=1` is set in the environment.
2. `bridge` is on PATH AND `bridge --json version` reports `schema_version=1`.

When either condition is false, the existing prototype adapter runs unchanged. There is no regression for users who don't install bridge or don't opt in.

### Why opt-in (and not auto-on)?

Safer rollout. The native prototype adapter is well-understood and stable; bridge is an additional surface that we want operators to consciously activate. To upgrade gradually, you enable `EVOLVE_USE_BRIDGE=1` in your shell or CI environment for a single cycle, verify it behaves as expected, then keep or disable.

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

### Enabling for a session

```bash
export EVOLVE_USE_BRIDGE=1
# now `claude-tmux` adapter invocations delegate to:
# bridge launch --cli=claude-tmux --allow-bypass
# (bridge picks up PROFILE_PATH, RESOLVED_MODEL, PROMPT_FILE, etc. from env)
```

### Force-disable (CI guardrail)

```bash
export EVOLVE_USE_BRIDGE=0   # or simply unset; default is "off"
```

This is recommended for CI configurations that need bit-for-bit reproducibility — even if bridge happens to be installed on the runner, the native adapter is used.

### Schema-version compatibility

The gate refuses to delegate if `bridge --json version | jq -r .schema_version` returns anything other than `"1"`. On mismatch, it emits a WARN to stderr and falls through to the native adapter. As bridge evolves, version mismatches will be flagged loudly here.

### Failure modes

| Scenario | What happens |
|---|---|
| `bridge` not on PATH, `EVOLVE_USE_BRIDGE=1` | Gate silently falls through to native adapter. |
| `bridge --json version` exits non-zero | Gate WARNs and falls through. |
| Bridge schema mismatch | Gate WARNs (`schema_version='X' (expected 1)`) and falls through. |
| Bridge runs but returns non-zero | The `exec` propagates bridge's exit code. **Cycle fails the same way as a native adapter failure would.** |
| Operator forgot to install `claude` (the underlying CLI) | Both native and bridge paths fail. Bridge surfaces a clearer error via `bridge --json doctor`. |

### Why claude-tmux only?

Of evolve-loop's 5 adapter scripts (`claude`, `claude-tmux`, `codex`, `agy`, `gemini`), only `claude-tmux` has both the highest value-add from bridge (subscription billing path) AND the most complex underlying behavior that bridge already implements (tmux driving + auto-respond). The other adapters are simpler and the native path is fine.

Future work may extend delegation to `codex` and `agy` — those are bridge-supported but lower-priority. `claude.sh` (headless) and `gemini.sh` are out of scope: claude.sh's evolve-loop-specific budgeting features aren't yet in bridge; gemini isn't a bridge backend.

---

## Operator verification

To verify the integration on your host:

```bash
# 1. Install bridge (separate concern; see Installing section).
bash tools/agent-bridge/install.sh --check && echo "bridge OK"

# 2. Run a probe cycle WITHOUT bridge enabled — should use native adapter.
EVOLVE_USE_BRIDGE=0 bash scripts/cli_adapters/claude-tmux.sh   # expect: prototype path
# (will fail without env vars; check the error mentions PROFILE_PATH, not bridge)

# 3. Same call WITH bridge enabled — should delegate.
EVOLVE_USE_BRIDGE=1 bash scripts/cli_adapters/claude-tmux.sh   # expect: "[claude-tmux] delegating to bridge ..."
```

Both invocations fail rc-nonzero in this contrived example (env vars missing); the point of the smoke is the **stderr log line** which tells you which path ran.

For a real cycle, set the 8 mandatory env vars and run via `subagent-run.sh` normally with `EVOLVE_USE_BRIDGE=1`.

---

## References

- `scripts/cli_adapters/claude-tmux.sh` — the prototype adapter + the new gate (top ~30 lines)
- `scripts/dispatch/subagent-run.sh` — the upstream caller (unchanged)
- Bridge source + design — separate; not in this repository
