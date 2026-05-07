# Project Instructions

> **Cross-CLI canonical instructions are in [AGENTS.md](AGENTS.md).** This file (CLAUDE.md) is the Claude Code-specific overlay — runtime hooks, plugin manifest details, version-specific notes, and incident history. Read AGENTS.md first for the universal pipeline contract; come here for Claude Code runtime specifics.


> This file is read by AI coding assistants. Platform equivalents: `CLAUDE.md` (Claude Code), `GEMINI.md` (Gemini CLI), `AGENTS.md` (generic). Content is platform-agnostic.

## Autonomous Execution

If the user is in autonomous mode (bypass permissions / yolo mode / auto-approve), YOU MUST:

1. **Continue all cycles without pausing** — complete every requested cycle end-to-end without stopping to ask for user approval. Never output "should I continue?" or wait for confirmation between cycles.
2. **Run the FULL pipeline every cycle** — launch Scout, Builder, and Auditor agents for every cycle. No shortcuts, no inline edits that skip agents, no batch-claiming multiple cycles. Each cycle must produce workspace artifacts (scout-report.md, build-report.md, audit-report.md).
3. **Run phase-gate.sh at every transition** — the deterministic phase gate script (`scripts/phase-gate.sh`) must execute at every phase boundary. This is non-negotiable even in bypass mode. Bypass permissions means "don't ask the user" — it does NOT mean "skip integrity checks."
4. **Never fabricate cycles** — claiming cycle numbers without doing work is a CRITICAL violation. Every cycle number must correspond to real Scout → Build → Audit → Ship → Learn execution.
5. **Phase agents MUST be invoked via `scripts/subagent-run.sh`** — the in-process `Agent` tool is forbidden in production cycles. v8.21.0 made this **structurally enforced**: `Agent` is denied in `orchestrator.json:disallowed_tools` AND blocked at the kernel layer by `phase-gate-precondition.sh` whenever `cycle-state.json` exists. There is no bypass. The runner enforces per-agent CLI permission profiles in `.evolve/profiles/` (least-privilege isolation), generates a per-invocation challenge token, and writes a tamper-evident ledger entry that `phase-gate.sh check_subagent_ledger_match` verifies against the on-disk artifact's SHA256.
6. **OS-level sandboxing wraps every claude subprocess.** When `EVOLVE_SANDBOX=1` (or profile `sandbox.enabled: true`), the runner wraps `claude -p` in `sandbox-exec` (macOS) or `bwrap` (Linux). Anthropic's Secure Deployment Guide states `--allowedTools` is "a permission gate, not a sandbox" — so kernel-layer enforcement is non-negotiable in production. Auditor and Evaluator profiles run with `read_only_repo: true` (read-only mount of the repo). **`EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1` is REQUIRED for nested-claude scenarios** (the primary use case for `/evolve-loop`, where the slash command runs from inside Claude Code's own sandbox). On Darwin 25.4+, `sandbox_apply()` returns EPERM when the parent process is itself sandboxed — the flag instructs `claude.sh` to retry the inner subagent unsandboxed, while kernel hooks (role-gate, ship-gate, phase-gate-precondition) remain active and enforce trust boundaries structurally. **`scripts/evolve-loop-dispatch.sh` auto-enables the flag** via `scripts/detect-nested-claude.sh` whenever `CLAUDECODE` / `CLAUDE_CODE_*` env vars are present (defense-in-depth alongside `SKILL.md`). The v8.21 deprecation of this flag was reverted in v8.22.0 after evidence showed the worktree-provisioning fix and the EPERM fallback target *orthogonal* layers (builder writes vs sub-claude startup).
7. **Mutation testing pre-flight on every eval.** `phase-gate.sh gate_discover_to_build` runs `scripts/mutate-eval.sh` against each new eval definition. If kill rate < 0.8, eval is flagged as tautological. Rollout phase 1: WARN-only logging. Rollout phase 2 (after one verification cycle): fail the gate.
8. **Adversarial Auditor mode is default-on.** The runner prepends an "ADVERSARIAL AUDIT MODE" framing to the auditor's prompt that requires positive evidence for PASS verdicts. Auditor model defaults to Opus (different family from Builder's Sonnet) to break same-model-judge sycophancy. Disable only with `ADVERSARIAL_AUDIT=0` for deliberately permissive sweeps.

**The rule is: maximum velocity, zero shortcuts.** Go fast by being efficient, not by skipping steps.

### Tamper-Evident Ledger Hash Chain (v8.37.0+)

The ledger (`.evolve/ledger.jsonl`) records every subagent invocation with cycle binding (HEAD + tree_state_sha at audit time), challenge-token, and artifact_sha256. v8.37.0 hardens the **forensic integrity** of this record so future audits, retrospectives, and operator investigations can detect tampering — covering threats the runtime enforcement layers (phase-gate, role-gate, ship-gate) don't address:

| Threat | Detection in v8.37.0 |
|---|---|
| **Entry rewrite** (modify a historical line to flip a verdict, change cycle attribution, alter timestamps) | Each new entry's `prev_hash` = SHA256 of the previous entry's full JSON line. Modifying any historical entry breaks every entry after it. |
| **Entry forgery / insertion** (splice a fake entry between two real ones) | The next legitimate entry's `prev_hash` references the original previous-entry SHA — chain breaks immediately. |
| **Truncation** (lop the last N entries to hide a failed cycle) | `.evolve/ledger.tip` records `<seq>:<sha256>` of the latest entry. Truncation makes tip-vs-actual-last-line mismatch. |
| **Concurrent fan-out race** (two writers compute the same prev_hash) | Verifier flags duplicate prev_hash as a chain anomaly (surfaces real concurrency bug rather than masking it). |

**Pipeline impact: zero.** The `prev_hash` and `entry_seq` fields are additive. Existing readers (`failure-adapter.sh`, `phase-gate.sh`, `phase-gate-precondition.sh`, `ship.sh`, `release/preflight.sh`) all use jq's `// empty` pattern — unknown fields are ignored. No reader was modified. No agent persona was modified. No profile was modified. No phase ordering, no Tier-1 hook semantics changed.

**Verifier:** `bash scripts/verify-ledger-chain.sh` walks the ledger end-to-end, recomputes each entry's prev_hash, and reports any break. Exit codes:
- `0` — chain intact, tip matches
- `1` — chain break (rewrite or forgery)
- `2` — tip mismatch (truncation or untracked write)
- `--json` for machine-readable output, `--quiet` for CI gating, `--ledger PATH` for alternate ledger paths

**Migration:** soft-start. Pre-v8.37 entries (no `prev_hash` field) are tolerated as a boundary; the first v8.37+ entry chains from the last pre-v8.37 entry's SHA. Pre-v8.37 entries themselves are not retro-validated (they predate the field). Existing 363-entry ledgers verify cleanly out of the box.

**Three writers updated** (the ledger has exactly three append paths):
- `scripts/subagent-run.sh:write_ledger_entry()` — primary `agent_subprocess` writer
- `scripts/subagent-run.sh:_write_fanout_ledger_entry()` — Pattern-3 fan-out writer (`agent_fanout` kind)
- `scripts/merge-lesson-into-state.sh` — `SYSTEMIC_FAILURE` retrospective writer

Each writer now: (1) reads the last line of `.evolve/ledger.jsonl`, (2) computes `prev_hash` and `entry_seq`, (3) emits the new entry with both fields, (4) updates `.evolve/ledger.tip` atomically with the new entry's SHA.

**What v8.37.0 does NOT add:** no Sigstore signing (cross-org attestation is v8.39+ candidate), no Merkle tree (linear chain is sufficient at our scale), no new enforcement hooks (the verifier is read-only diagnostic).

**Threat model context:** runtime cheating is structurally prevented by the existing pipeline. v8.37.0 is the **forensics layer** — when an AI agent attempts to cheat in unexpected ways, the ledger remains the authoritative reconstruction surface. Without hash-chaining, a sufficiently-clever agent could rewrite history to hide misbehavior; with hash-chaining, any post-hoc modification is structurally detectable.

### Worktree Provisioning Robustness (v8.36.0+)

`run-cycle.sh` pre-flight cleanup now runs `git worktree prune` BEFORE attempting `git branch -D`. Closes a recurring failure mode in nested-Claude environments: when a prior cycle was hard-killed at a different `$TMPDIR`-based worktree path (e.g., session 1 used `/var/folders/.../HASH_A/cycle-N/`, session 2 uses `/var/folders/.../HASH_B/cycle-N/`), `.git/worktrees/cycle-N/` retained a stale admin pointer to the old path. `git branch -D` silently no-ops on a branch that's still admin-checked-out; `git worktree add` then fails with `fatal: a branch named 'evolve/cycle-N' already exists`.

The fix: `git worktree prune` removes admin entries for worktrees whose directories no longer exist. Active worktrees (whose dirs still exist) are NOT touched, so concurrent cycles are safe. The prune is idempotent (no-op on clean repos) so it's safe to run unconditionally.

Two locations got the fix:
1. **Pre-flight cleanup** (`scripts/run-cycle.sh:~382`, before stale-branch deletion) — primary fix.
2. **`cleanup()` EXIT trap** (`scripts/run-cycle.sh:~298`, after worktree-remove) — defensive belt-and-suspenders, ensures next cycle starts clean even if the trap's worktree-remove silently failed.

Test: `scripts/run-cycle-worktree-test.sh` (8 assertions) reproduces the bug, verifies the fix, and checks safety properties (idempotent on clean repo, doesn't touch active worktrees).

### Orchestrator Fluency on WARN + Adaptive Auditor (v8.35.0+)

Two structural fixes addressing downstream-user pain ($25/5-cycle batch with 0 ships):

**Fix 1 — Orchestrator ships on WARN by default** (matches v8.28.0 ship.sh policy).
Pre-v8.35.0 the orchestrator persona pre-decided to skip ship on any non-PASS verdict, recording WARN as `code-audit-fail`. ship.sh's v8.28.0 fluent-by-default policy (WARN ships unless `EVOLVE_STRICT_AUDIT=1`) never got a chance to apply because ship.sh was never invoked. v8.35.0 updates `agents/evolve-orchestrator.md` so:

| Verdict | Pre-v8.35.0 | v8.35.0 |
|---|---|---|
| PASS | ship.sh | ship.sh (unchanged) |
| **WARN** | record-failure-to-state.sh, skip ship | **record (low-severity awareness) THEN ship.sh** — ship.sh accepts WARN per v8.28.0 |
| FAIL | record-failure-to-state.sh, skip ship | (unchanged) |
| EVOLVE_STRICT_AUDIT=1 + WARN | skip ship | skip ship (legacy behavior preserved) |

The orchestrator's verdict in its report becomes `SHIPPED-WITH-WARNINGS` for the WARN-shipped path.

**Fix 2 — New `code-audit-warn` classification + adaptive auditor model**.
Pre-v8.35.0, `failure_normalize_legacy "WARN" → code-audit-fail` (HIGH severity, 30-day age-out). This polluted the failure-adapter's lookback. v8.35.0 introduces:

- `code-audit-warn` classification: severity=low, age_out=86400 (1d), retry=yes
- `failure_normalize_legacy "WARN" → code-audit-warn` (was `code-audit-fail`)
- `failure_normalize_legacy "FAIL" → code-audit-fail` (unchanged)
- The failure-adapter's existing `code-audit-fail`-block rule is FAIL-only; WARN entries can't trigger it

**Fix 3 — Adaptive auditor model selection** via `scripts/diff-complexity.sh`.
The auditor profile defaulted to Opus on every cycle regardless of diff size. A 2-file 50-line diff doesn't need Opus — Sonnet catches the same findings at ~$0.50 instead of $2.39. v8.35.0 adds tier-based auto-selection:

- **trivial** (≤3 files AND ≤100 lines AND no security paths) → Sonnet (~$0.40)
- **complex** (>10 files OR >500 lines OR matches `(auth|crypto|payment|secret|\.env|password|token)` regex) → Opus (~$2.39, current default)
- **standard** (everything else) → Opus (conservative; can downgrade later if quality holds)

Operators can override:
- `MODEL_TIER_HINT=opus` — force Opus regardless of diff
- `EVOLVE_AUDITOR_TIER_OVERRIDE=opus` — auditor-only override
- `EVOLVE_DIFF_COMPLEXITY_DISABLE=1` — kill switch (revert to profile default everywhere)

Builder model unchanged. Same-family-judge concern (Builder=Sonnet + Auditor=Sonnet for trivial cycles) mitigated by: adversarial-audit framing (prompt-level), small surface area (≤100 lines), three independent inputs (scout-report + build-report + git diff), and security-path force-Opus.

**Cost projection for the downstream "$25 with 0 ships" scenario**:

| Cycle | Pre-v8.35.0 | Post-v8.35.0 |
|---|---|---|
| 1 | $4.27 (no ship) | $3.30 (ships if PASS/WARN) |
| 2 | $4.89 (WARN-skipped) | $3.30 (WARN ships) |
| 3 | $5.00 (retry) | $3.30 (NEW work, no retry needed) |
| 4 | $5.00 (retry) | $3.30 |
| 5 | $5.00 (retry) | $3.30 |
| **Total** | **~$24, 0 ships** | **~$16.50, 4-5 ships** |

40% cost reduction + 4-5× more delivered work.

**Anti-gaming preserved**. Tier-1 hooks (phase-gate, role-gate, ship-gate canonical entry, cycle binding, ledger SHA, audit-report verdict text) all unchanged. v8.34.0's actual-diff commit footer still records ground truth in `git log`.

### Pipeline Continuation + Diff Transparency (v8.34.0+)

Two structural fixes addressing real-world friction reported from a downstream user:

**Fix 1 — `ship.sh` advances `state.json:lastCycleNumber` on successful cycle ship.**
Pre-v8.34, only failure paths (`record_failed_approach` in the dispatcher) wrote `lastCycleNumber`. Successful ships left it unchanged → the dispatcher's next iteration computed `ran_cycle = last_before + 1 = the SAME cycle just shipped` → 5-repeat circuit-breaker fired prematurely on legitimate runs (downstream user saw cycle 1 ran 5 times before abort). v8.34.0 has `ship.sh` read `cycle_id` from `cycle-state.json` and atomically write it to `state.json:lastCycleNumber` after a successful push. **Only for `--class cycle`** (manual + release classes don't have cycle semantics).

**Fix 2 — `ship.sh` appends an `## Actual diff` footer to commit messages.**
Cycles 102+ on a downstream project shipped commits whose messages claimed major refactors (e.g., "moved StudyReminder + Recommendations to 4 above-fold blocks") but `git diff` showed unrelated 2-line moves. The auditor scored the build-report's narrative without verifying it against the actual diff. Per the user's "record, don't block" principle: v8.34.0 records the actual diff in the commit message itself so reviewers (and future audits) can spot message-vs-diff divergence in `git log`. Format:

```
<original commit message>

---
## Actual diff (v8.34.0+)

Files modified (N):
- M path/to/file1.ts
- A path/to/newfile.tsx
- D path/to/removed.tsx

 N files changed, NN insertions(+), NN deletions(-)
```

Skipped for `--class release` (version-bump commits have well-defined scope; the footer adds bulk without value). Applied to both `--class cycle` and `--class manual`.

**This is intentionally non-blocking.** No ship is refused for claim-vs-diff divergence — the divergence is recorded in `git log` and surfaces during review. Per the user's directive: *"if the design cannot consider too many layer and use cases, we should keep the action history and record for further improvement but not directly blocking (unless it's fundamental rules)."*

### Token Optimization (v8.33.0+)

Three surgical fixes targeting per-cycle input-token spend without touching agent behavior:

1. **Cache-friendly prompt order**: `run-cycle.sh:build_context()` already emits the static agent prompt first (read verbatim from `agents/<role>.md`), then dynamic context last. This pattern hits Anthropic's automatic prompt cache (Sonnet ≥1024 tokens, 5-minute TTL, 0.1× cost on reads). Three back-to-back cycles in one batch hit the cache after the first.

2. **Conditional context blocks**: Pre-v8.33, every cycle injected `recentLedgerEntries:`, `recentFailures:`, `instinctSummary:` headers unconditionally — even with empty bodies. For cycle 1 with empty state, this padded ~500–1000 tokens of useless boilerplate. v8.33.0 emits each block only when its data source is non-empty.

3. **Per-cycle cost summary in dispatcher**: After each cycle's ledger verification, the dispatcher invokes `show-cycle-cost.sh --json` and emits one log line:
   ```
   [dispatch] cycle 25 cost: $1.5702 (scout=$0.5234, builder=$0.5234, auditor=$0.5234) cache_hit=66%
   ```
   Surfaces the optimization (cache hit %) AND the cost-driver phases without operators grepping sidecar JSON. Best-effort — silently no-ops if the cycle didn't produce stdout.log files.

**Side fix in v8.33.0**: `show-cycle-cost.sh` now honors `EVOLVE_PROJECT_ROOT` (writable side of dual-root) instead of script-relative `REPO_ROOT`. Pre-v8.33 it was broken in plugin-install scenarios because cycle data lives under the user's project, not under the plugin install. Tests can also override via `RUNS_DIR_OVERRIDE`.

Quality impact: zero. The three fixes are pure cost-side; same agents, same prompts, same artifacts, same verdicts.

### Swarm Optimization (v8.23.0+)

Sprint 1's Pattern-3 fan-out (Scout/Auditor/Retrospective parallel sub-personas) gained three optimizations in v8.23.0, all gated by env flags so existing behavior is preserved when off.

| Flag | Default | Effect |
|---|---|---|
| `EVOLVE_FANOUT_CANCEL_ON_CONSENSUS` | `0` | When `1`, audit fan-out cancels remaining workers when K agree on FAIL. Saves 3-8 min wall-time + $0.50-$2.00 in token budget on catastrophic audit failures. |
| `EVOLVE_FANOUT_CONSENSUS_K` | `2` | Number of agreeing workers required to trigger cancel. |
| `EVOLVE_FANOUT_CONSENSUS_POLL_S` | `1` | Polling interval (seconds) for consensus check while workers run. |
| `EVOLVE_FANOUT_CACHE_PREFIX` | `1` | When `1`, `dispatch-parallel` writes a deterministic `workers/cache-prefix.md` shared across siblings. Anthropic prompt-cache (≥1024 token, 5-min TTL) is hit by every sibling after the first. ~47% input-token reduction on 3-worker fan-out. Default-on because it's pure savings. |
| `EVOLVE_FANOUT_TRACK_WORKERS` | `1` | When `1`, `fanout-dispatch.sh` writes per-worker status (`pending` → `running` → `done` / `failed`) into `cycle-state.json:parallel_workers.workers[]`. Default-on for observability. |

Disabling all three reverts to v8.22.0 behavior bit-for-bit. `cancel_on_consensus` is opt-in because it's invasive (SIGTERMs running workers); the other two are pure additive wins.

**`parallel_workers.workers[]` schema** (Task D):

```json
"parallel_workers": {
  "agent": "scout",
  "count": 3,
  "started_at": "2026-05-05T03:47:33Z",
  "workers": [
    {"name": "scout-codebase", "status": "done", "started_at": "...", "ended_at": "...", "exit_code": 0},
    {"name": "scout-research", "status": "running", "started_at": "..."},
    {"name": "scout-evals",    "status": "pending"}
  ]
}
```

Operator helpers: `cycle-state.sh init-workers <agent> <name>...` (initialize all `pending`); `cycle-state.sh set-worker-status <name> <status> [<exit_code>]` (transition a single worker).

### Failure Adaptation Kernel (v8.22.0+)

`/evolve-loop` cycles read prior failures from `state.json:failedApproaches[]` to decide whether to proceed, retry with fallback, or block. Pre-v8.22.0 this was a markdown rule the orchestrator interpreted; v8.22.0 promotes it to a **deterministic kernel function** computed by `scripts/failure-adapter.sh`:

```bash
bash scripts/failure-adapter.sh decide --state .evolve/state.json
# emits JSON: {action, reason, remediation, set_env, skip_phases, verdict_for_block, evidence}
```

**Structured taxonomy** (7 classifications, each with severity tier + age-out window + retry policy — defined in `scripts/failure-classifications.sh`):

| Classification | Age-out | Severity |
|---|---|---|
| `infrastructure-transient` (sandbox-eperm, network blip, rate-limit) | 1 day | low |
| `infrastructure-systemic` (host broken, tooling missing, claude-cli down) | 7 days | high |
| `intent-malformed` (intent persona output invalid) | 1 day | low |
| `intent-rejected` (IBTC out-of-scope) | never | terminal |
| `code-build-fail` (builder couldn't compile/test) | 30 days | high |
| `code-audit-fail` (auditor returned FAIL) | 30 days | high |
| `human-abort` (operator killed run) | 1 hour | low |

**Retention policy**: each entry has an `expiresAt` timestamp. Both write paths (`record-failure-to-state.sh` and the dispatcher's `record_failed_approach`) compute `expiresAt = recordedAt + age-out-window` and apply a FIFO cap (max 50 entries). The adapter and read-side filter exclude expired entries automatically.

**Decision rules** (priority order):
1. `intent-rejected` (any non-expired) → `BLOCK-CODE` / `SCOPE-REJECTED` (operator must refine goal)
2. `infrastructure-systemic` (any non-expired) → `BLOCK-OPERATOR-ACTION` / `BLOCKED-SYSTEMIC`
3. 2+ `code-audit-fail` → `BLOCK-CODE` / `BLOCKED-RECURRING-AUDIT-FAIL`
4. 2+ `code-build-fail` → `BLOCK-CODE` / `BLOCKED-RECURRING-BUILD-FAIL`
5. 3+ consecutive `infrastructure-transient` (tail streak) → `BLOCK-OPERATOR-ACTION` / `BLOCKED-SYSTEMIC`
6. 1+ `infrastructure-transient` (anywhere) → `RETRY-WITH-FALLBACK` (auto-set `EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1`)
7. otherwise → `PROCEED`

Code and infrastructure failures are **scored separately** — 17 infra failures + 0 code failures still yields `PROCEED` (with fallback) for code work. No more "any-kind" conflation.

**Operator utilities**:
- `bash scripts/state-prune.sh --classification <name>` — drop entries by classification
- `bash scripts/state-prune.sh --age 7d` — drop entries older than 7 days
- `bash scripts/state-prune.sh --cycle <N>` — drop a specific cycle's entry
- `bash scripts/state-prune.sh --all --yes` — wipe entirely (with confirmation)
- `bash scripts/cycle-state.sh prune-expired-failures` — programmatic auto-aging-out

The orchestrator agent (`agents/evolve-orchestrator.md`) reads the adapter's JSON verbatim and follows the action — it no longer interprets adaptation rules from prose. When the action is `BLOCK-*`, the orchestrator-report.md must contain a structured "## Operator Action Required" block with the remediation field embedded so humans know exactly what to do.

### Worktree Provisioning Contract (v8.21.0+)

Per-cycle git worktrees are provisioned by `scripts/run-cycle.sh` (privileged shell context, structurally outside any LLM agent's reach) at `$EVOLVE_PROJECT_ROOT/.evolve/worktrees/cycle-N` on branch `evolve/cycle-N`. The path is recorded in `cycle-state.json:active_worktree`, exported as `WORKTREE_PATH`, and torn down via the EXIT trap on every exit path (success, failure, signal).

The trust-boundary invariant: **the orchestrator and all phase agents may NOT call `git worktree add` or `git worktree remove`.** Both are denied in `orchestrator.json` and in every phase profile that has a deny list. `cycle-state.sh set-worktree` is also privileged-shell-only (denied in orchestrator profile).

This closes the architectural gap that made v8.13.x — v8.20.2 require `EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1` to function on macOS Darwin 25.4: previously no component provisioned the build worktree, leaving `cycle-state.active_worktree` null, the builder's sandbox profile expanding `{worktree_path}` to empty, and all source writes EPERM.

If you ever need the worktree path from agent context, read it via `cycle-state.sh get active_worktree`. Never compute it yourself — the dispatcher is the canonical source.

## Three-Tier Strictness Model (v8.24.0+, refined v8.25.0)

evolve-loop's strictness is layered. The user-facing pain in pre-v8.24.0 came from conflating layers; v8.24.0 made the layers explicit; v8.25.0 replaced "skip the worktree" with "relocate the worktree" so isolation is preserved even in nested-Claude.

| Tier | Mechanism | Default | Auto-adapt? | What it catches |
|---|---|---|---|---|
| **1 — Structural integrity** | phase-gate, ledger SHA, role-gate, ship-gate (`scripts/guards/`) | Always on | NEVER | Reward hacking, phase-skipping, integrity breach (cycle 102–111, cycle 132–141 incidents) |
| **2 — OS isolation** | `sandbox-exec`/`bwrap`, per-cycle worktree | On (always) | Worktree path auto-selected per environment; sandbox falls back when nested | Compromised builder writing outside its sandbox; one cycle's edits leaking into another's workspace |
| **3 — Workflow defaults** | intent capture, fan-out, mutation testing, adversarial audit | Opt-in via env flags | N/A — already opt-in | Vague goals, sycophantic audits, tautological evals |

**The governing rule:** Tier 1 is non-negotiable and runs in privileged shell context (no sandbox dependency). Tier 2 *adapts to* the environment instead of *degrading* — per-cycle worktrees always exist; only the path moves. Tier 3 is operator-controlled per-run.

### Capability detection (v8.25.0+, refined v8.25.1)

`scripts/preflight-environment.sh` runs at dispatcher start, probes the host, and emits a JSON profile to `.evolve/environment.json` (schema v3 since v8.25.1). The dispatcher reads `auto_config` and applies it. **One observable file replaces the 6+ env flags that accumulated as escape hatches.**

`auto_config` decides three things:

| Field | Values | Decision rule |
|---|---|---|
| `EVOLVE_SANDBOX_FALLBACK_ON_EPERM` | `"0"` / `"1"` | `1` when nested-Claude is detected (Darwin sandbox-exec startup will EPERM otherwise) |
| `worktree_base` | absolute path | priority: operator-set `EVOLVE_WORKTREE_BASE` > in-project `.evolve/worktrees/` (standalone) > `$TMPDIR/evolve-loop/<hash>` (nested-Claude default) > user cache dir |
| `inner_sandbox` | `true` / `false` | `false` when nested-Claude OR sandbox not expected to work; `true` when standalone with working sandbox-exec/bwrap |

The dispatcher exports `EVOLVE_WORKTREE_BASE`; `run-cycle.sh` provisions worktrees there via `git worktree add`. Worktrees can live anywhere — `git`'s `.git/worktrees/cycle-N` pointer file handles cross-directory references — so cleanup, isolation, and audit binding work regardless of physical path.

`scripts/cli_adapters/claude.sh` reads `inner_sandbox` and **skips the inner sandbox-exec wrapper** when false. This is the v8.25.1 fix: in nested-Claude, the OUTER Claude Code OS sandbox already provides process isolation; the INNER nested sandbox-exec adds no protection (nested sandboxes can only intersect, never expand) AND introduces execution-time EPERM that no startup-fallback can catch. Skipping it lets builds actually land while Tier-1 hooks (phase-gate, role-gate, ledger-SHA) keep enforcing structural integrity.

**Operator overrides** are observable:
- Edit `.evolve/environment.json` directly (schema is documented in `scripts/preflight-environment.sh`)
- Set `EVOLVE_WORKTREE_BASE=/path` before invoking the dispatcher (dispatcher logs `operator set EVOLVE_WORKTREE_BASE=... (override profile)`)
- Set `EVOLVE_FORCE_INNER_SANDBOX=1` to force-enable inner sandbox-exec even in nested-Claude (paranoid mode)
- Set `EVOLVE_INNER_SANDBOX=0` to force-disable inner sandbox-exec everywhere (debugging)

The claude-adapter logs the source of each sandbox decision: `[claude-adapter] sandbox=0 (source: environment.json:auto_config.inner_sandbox=false)`.

### EVOLVE_SKIP_WORKTREE is a true emergency hatch (v8.25.0+)

Pre-v8.25.0, `EVOLVE_SKIP_WORKTREE=1` was auto-enabled in nested-Claude. **This was wrong** — it abandoned per-cycle isolation, breaking fast-rollback and audit binding. v8.25.0 removes the auto-enable. The flag still exists as an operator-only emergency override, with a loud WARN. **Do not auto-enable this flag.** If the v8.25.0 worktree relocation can't find a writable base, the dispatcher fails loud at startup with a remediation message — preferable to silently disabling isolation.

### Ship commit classifiers (v8.25.0+)

`scripts/ship.sh` now requires an explicit class via `--class`:

| Class | Use case | Verification |
|---|---|---|
| `cycle` (default) | `/evolve-loop` cycle commits | Full audit-binding (must have recent PASS, SHA matches, HEAD/tree bound) |
| `manual` | Operator-driven manual commits | Skips audit; requires interactive y/N confirmation after `git diff --cached --stat` preview. Refuses when stdin is not a tty. CI mode via `EVOLVE_SHIP_AUTO_CONFIRM=1`. |
| `release` | Internal — `scripts/release-pipeline.sh` only | Skips audit (version-bump.sh mutates files post-audit); logs RELEASE class loudly |

**Migration path:** scripts/cron jobs that previously used `EVOLVE_BYPASS_SHIP_VERIFY=1` continue to work but emit a deprecation warning pointing to `--class manual` (interactive) or `EVOLVE_SHIP_AUTO_CONFIRM=1 ... --class manual` (CI). The legacy env var will not be removed in v8.x — it's a permanent compatibility bridge.

**Anti-gaming preserved because:**
- Interactive y/N for `--class manual` is a boundary an LLM agent cannot cross (no tty access)
- Class is logged via `provenance:` line so commit lifecycle is observable from `git log` + ship.sh stderr
- Ship-gate kernel hook stays as the structural enforcer (denies bare `git push`)
- `release` class can only be invoked from the release-pipeline (gated by ship-gate's allowlist)

### Dispatcher safety nets (v8.24.0+)

Three structural fixes prevent the pre-v8.24.0 "cycle-N runs M× and burns budget" deadlock:

1. **Pre-flight state.json writability check.** Dispatcher tests writability before any cycle. If unwritable, abort with copy-paste remediation. Catches the OS-sandbox-blocks-state.json case at $0 cost.
2. **Fail-loud on `record_failed_approach` write EPERM.** Both `mv -f` calls now error-checked; FATAL log + return 1 if write fails. Caller propagates as DISPATCH_RC=1 + break (no silent loop).
3. **Same-cycle circuit-breaker.** Tracks `prev_ran_cycle`; aborts batch when N consecutive iterations report the same cycle number (default `EVOLVE_DISPATCH_REPEAT_THRESHOLD=3`). Catches systemic failures the other guards miss.

If you ever see "ABORT: same cycle number" in the dispatcher log, the remediation block tells you exactly which env var to set. The cost of the failed batch is bounded to ~3 cycles, not the full requested N.

## Release & Publish Workflow (v8.13.2+)

**"publish" ≠ "push".** See [docs/release-protocol.md](docs/release-protocol.md) for the canonical vocabulary (push, tag, release, propagate, publish, ship). When the user asks to "publish vX.Y.Z", use the self-healing pipeline:

```bash
bash scripts/release-pipeline.sh X.Y.Z              # full publish
bash scripts/release-pipeline.sh X.Y.Z --dry-run    # simulate, no mutations
bash scripts/release-pipeline.sh X.Y.Z --skip-tests # hot-fix path (CI-pre-verified)
```

The pipeline owns the entire lifecycle: pre-flight gate → version bump → auto-changelog from conventional commits → consistency check → atomic ship via `scripts/ship.sh` → marketplace propagation polling (5 min) → cache refresh → auto-rollback on any post-push failure.

**Bare `git push origin main` is denied by ship-gate** (since v8.13.0). Direct commits and pushes go through `scripts/ship.sh`. The release pipeline calls ship.sh internally; it does not bypass the gate.

### Required version markers (all auto-bumped by `version-bump.sh`)

1. `.claude-plugin/plugin.json` — canonical version (source of truth)
2. `.claude-plugin/marketplace.json` — `.plugins[0].version`
3. `skills/evolve-loop/SKILL.md` — heading `# Evolve Loop vX.Y` (only if major.minor changed)
4. `README.md` — "Current (vX.Y)" + version history row (only if major.minor changed)
5. `CHANGELOG.md` — `## [X.Y.Z]` block (auto-generated by `changelog-gen.sh`)

`scripts/release.sh` is now a **consistency verifier** invoked by the pipeline. Run standalone for diagnostics: `bash scripts/release.sh <version>`.

### Conventional commits

Auto-changelog buckets commits by type prefix:
- `feat:` / `feat(scope):` → `### Added`
- `fix:` → `### Fixed`
- `refactor:` / `perf:` / `stability:` → `### Changed`
- `docs:` → `### Documentation`
- `chore:` / `ci:` / `test:` / `build:` / `revert:` / `release:` → skipped
- no prefix → `### Other` (audit found ~40% of historical commits)

## Version-Aware TOFU (v8.32.0+) — close the plugin-update SHA trap

Pre-v8.32.0, `ship.sh` pinned its own SHA in `state.json:expected_ship_sha` on first run (TOFU = trust-on-first-use). Subsequent runs verified the SHA matched. **The dominant cause of SHA changes is plugin updates** — every `/reload-plugins` after a v8.X release ships a new ship.sh with different content. The TOFU pin then refused to ship until the operator manually deleted `expected_ship_sha` from state.json. Real-world report: every plugin upgrade caused INTEGRITY-FAIL on the next cycle's ship phase.

**v8.32.0 fix**: pin BOTH SHA AND plugin version. Verification logic:

| State | What v8.32.0 does |
|---|---|
| First run (no pin) | Pin both SHA + version |
| Same version + same SHA | Pass (clean) |
| Same version + different SHA | **REAL TAMPERING** — fail integrity check |
| Different version (plugin updated) | **Re-pin** SHA + version, log loudly, continue |
| Legacy state.json (SHA pinned, no version) | Auto-migrate to version-aware schema |

The within-version-different-SHA path still fails — that's the threat model TOFU was designed for (someone editing ship.sh in place to bypass checks while plugin version stays the same). Cross-version SHA changes auto-recover, eliminating the false-positive on plugin updates. This is the fluent-philosophy applied to TOFU: detect update vs tampering, only block on tampering.

**Anti-gaming impact**: minimal. The within-version threat (an attacker with write access to `~/.claude/plugins/.../scripts/ship.sh` who modifies it without bumping plugin.json:version) still trips. The cross-version threat is weaker because the attacker would need to forge `plugin.json:version` too — and at that point they have full write access and can also tamper with role-gate.sh, ship-gate.sh, profiles, etc. (all of which never had TOFU). The Tier-1 hooks (phase-gate, role-gate, ledger SHA, ship-gate atomicity) remain the real defense.

**Operator impact**: plugin updates no longer trigger SHA-pin traps. The cycle-pipeline-completion guarantee is preserved across releases. Existing pinned states auto-migrate on next ship.sh run.

## Builder Write-Leak Fix (v8.31.0+)

The Builder profile previously had `Edit(scripts/**)` + `Edit(skills/**)` (and Write counterparts) in `disallowed_tools` AND bare `Bash` in `allowed_tools`. This combination forced Builder to use Bash for any script-edit work (Edit was denied → fallback to `cat > scripts/foo` or `python3 -c "open(...)"`). Bare Bash has no path gating: `--add-dir` only restricts the Edit/Write tools, not Bash redirects. With v8.25.1 disabling inner `sandbox-exec` in nested-Claude, nothing prevented Builder from writing to `/Users/.../evolve-loop/scripts/foo` directly.

**Cycle-25 evidence**: 5 files modified in main repo's working tree post-cycle, despite `active_worktree=/var/folders/.../cycle-25`. The build-report claimed "directly in worktree via Python3+Bash" but the changes ended up in main repo.

**Fix**: Builder's `disallowed_tools` now removes the redundant `Edit(scripts/**)` etc. denials (path gating is already enforced by `role-gate.sh` per-phase allowlists) and adds the same interpreter-execution Bash denials that orchestrator already has: `perl`, `ruby`, `python3 -c`, `node -e`, `sh -c`, `bash -c`, `zsh -c`, `env`, `exec`, `eval`, `awk`, `unlink`, `ln`. Net result: Builder uses Edit tool (gated to worktree by role-gate) instead of falling back to Bash; arbitrary-code-execution Bash patterns are blocked. `cat > FILE` still works for legitimate worktree writes since `cat` itself isn't in the denial list — but `python3 -c "open('/Users/.../scripts/foo', 'w').write(...)"` is.

## Operational Polish (v8.30.0+)

Four small fixes that close v8.30 candidate tickets without philosophical changes:

1. **`run-cycle.sh` exit 1 → recoverable when orchestrator-report exists.** Pre-v8.30, ANY non-zero exit from `run-cycle.sh` aborted the dispatcher batch. Now: if `orchestrator-report.md` exists for the attempted cycle and classifies as `infrastructure`/`audit-fail`/`build-fail`/`ship-gate-config`, the dispatcher records the failure and continues (rc=3). Only abort when no report exists (true breach) or classification is `integrity-breach`. Aligns with v8.28.0 fluent philosophy: don't kill the batch on a single transient cycle failure.

2. **`ship.sh` rejects dual-verdict reports.** When audit-report.md contains both `Verdict: PASS` AND `Verdict: FAIL`, ship-gate now refuses with a clear "auditor produced inconsistent artifact" message instead of just blocking on FAIL. Catches the cycle-25 inconsistency (FAIL header + PASS per-eval evidence) explicitly.

3. **`cycle-state.sh advance` clears `parallel_workers`.** Prevents stale per-worker state from one phase contaminating the next phase's display. Cosmetic but real: previously `phase=audit` could show `parallel_workers.agent=scout` from the prior fan-out.

4. **SKILL.md goal-quoting docs.** When `<args>` contains an apostrophe, the shell tokenizer breaks. Documented the fix: wrap the goal in double quotes.

## Fluent-by-Default (v8.28.0+) — proceed unless faking work

Operating principle declared: **reduce friction is the top priority; preventing the LLM from gaming the system is equal priority but only structural fakery counts.** Five Tier-1 hooks prove fakery (phase-gate, ledger SHA, role-gate, ship-gate canonical entry, cycle binding). Everything else is operational policy that has accumulated friction without proportional protection.

v8.28.0 inverts the default. Strict-mode behaviors are now opt-in.

| Behavior | Pre-v8.28.0 default | v8.28.0 default | Restore via |
|---|---|---|---|
| Audit `Verdict: WARN` | Block ship | **Ships, logs WARN** | `EVOLVE_STRICT_AUDIT=1` |
| Audit `Verdict: FAIL` | Block ship | Block ship (unchanged) | — |
| 1+ infrastructure-systemic | BLOCK-OPERATOR-ACTION | **PROCEED with awareness** | `EVOLVE_STRICT_FAILURES=1` |
| 2+ code-audit-fail | BLOCK-CODE | **PROCEED with awareness** | `EVOLVE_STRICT_FAILURES=1` |
| 2+ code-build-fail | BLOCK-CODE | **PROCEED with awareness** | `EVOLVE_STRICT_FAILURES=1` |
| 3+ consecutive infra-transient | BLOCK-OPERATOR-ACTION | **PROCEED with awareness + EPERM fallback set_env** | `EVOLVE_STRICT_FAILURES=1` |
| Same-cycle circuit-breaker | 3 iterations | **5 iterations** | `EVOLVE_DISPATCH_REPEAT_THRESHOLD=3` |
| Expired failedApproaches entries | Linger on disk | **Auto-pruned at dispatcher start** | `EVOLVE_AUTO_PRUNE=0` |

**Awareness instead of blocks**: when a would-have-blocked rule fires in fluent mode, the failure-adapter accumulates a `would-have-blocked: <action> — <reason>` line in the PROCEED reason field. The orchestrator reads this and adapts approach in the next cycle (smaller scope, alternative path) without the loop halting.

**Anti-gaming preserved.** The cycle 102–111, 132–141, and Gemini-forgery incidents — the canonical reward-hacking threats — were all caught by Tier-1 (SHA verification + verdict text + phase-gate ordering). None of those structural defenses change. v8.28.0 only relaxes operational policy that was over-engineered.

## Loop Continuation (v8.27.0+) — exit-code semantics + --reset

Three fixes in v8.27.0 close a remaining deadlock where audit-PASS cycles couldn't ship.

**Fix 1 — `ship.sh` accepts auditor exit_code 0 OR 1.** The auditor CLI follows Unix convention: `exit 0` = no findings (cleanest), `exit 1` = findings present (normal — every realistic audit has findings), `exit 2+` = true error. Pre-v8.27.0 ship-gate rejected ANY non-zero exit, treating "findings present" as integrity failure. The artifact-content checks (SHA + Verdict text + cycle binding + freshness) are the actual source of truth and run regardless of exit_code. Layer 6 was vestigial.

**Fix 2 — `ship-gate-config` classification.** Cycles where audit declared PASS but ship-gate refused (e.g., the v8.27.0 exit-code mismatch) are now classified as `ship-gate-config` (1d age-out, low severity, retry-yes), NOT `infrastructure-systemic` (7d, BLOCK-OPERATOR-ACTION). A config/logic mismatch in the plugin is not a "host broken" condition — the failure-adapter shouldn't conflate them. `dispatcher.classify_cycle_failure()` detects `SHIP_GATE_DENIED` markers and routes to the new classification.

**Fix 3 — `--reset` flag.** `bash scripts/evolve-loop-dispatch.sh --reset [N] [strategy] [goal]` runs `state-prune.sh` against `infrastructure-{systemic,transient}` + `ship-gate-config` entries before the cycle loop. Operator-driven recovery from BLOCKED-SYSTEMIC deadlock without bypassing any kernel hook. Logged loudly so the choice is auditable.

**Anti-gaming preserved.** Cycle 102–111, 132–141, and Gemini-forgery were all caught by SHA + verdict-text + phase-gate, none of which change. The exit_code check (the relaxation) was never the structural defense — it was an over-restrictive gate that conflicted with the auditor's actual exit semantics.

## Budget Cap Default-Unlimited (v8.26.0+)

Pre-v8.26.0, `--max-budget-usd` was sized per-phase from the profile (~$0.18 Scout default, $0.50 Intent, $1.00 Orchestrator). Complex meta-goals routinely exceeded these caps mid-thought, exiting subagents with `BUDGET_EXCEEDED` (rc=1) and aborting cycles with no useful output. The caps prevented cost runaway but did not prevent reward hacking — that's enforced entirely by Tier-1 hooks (phase-gate, role-gate, ledger SHA).

v8.26.0 sets `--max-budget-usd` to `999999` (effectively unlimited) by default. The flag is still passed because the claude binary expects it, but the value never triggers `BUDGET_EXCEEDED` in any realistic cycle. The profile's `max_budget_usd` is preserved in adapter logs for traceability.

| Env var | Effect |
|---|---|
| (default) | `--max-budget-usd 999999` — effectively unlimited |
| `EVOLVE_BUDGET_CAP=<value>` | Operator pin: hard cap at `<value>`, wins over both default and ENFORCE |
| `EVOLVE_BUDGET_ENFORCE=1` | Legacy strict mode: use the profile/env-resolved `MAX_BUDGET` |
| `EVOLVE_MAX_BUDGET_USD=<value>` | Per-invocation override of resolved budget (still works, only applies under `ENFORCE=1`) |
| `EVOLVE_TASK_MODE=<tier>` | Tier resolution from `budget_tiers` (still works, only applies under `ENFORCE=1`) |

**Anti-gaming preserved:** budget caps don't prevent reward hacking. The Tier-1 hooks enforce structural integrity regardless of how much an agent is allowed to spend. A buggy or hostile agent attempting to game the system will be caught by ledger SHA verification, phase-gate ordering, role-gate path allowlists, and ship-gate atomicity — not by `BUDGET_EXCEEDED`.

**Cost discipline:** operators who need hard cost ceilings should set `EVOLVE_BUDGET_CAP=<value>` (single hard ceiling for the run) or `EVOLVE_BUDGET_ENFORCE=1` (use resolved per-phase profile values). Both options remain operator-accessible; the change is only the default.

## Subagent Budget Controls (v8.13.4 / v8.13.5)

evolve-loop subagents have **three** budget-control mechanisms, evaluated in priority order:

### Precedence (highest priority first)

1. **`EVOLVE_MAX_BUDGET_USD`** (v8.13.4) — operator-controlled per-invocation override. Overrides all else.
2. **`EVOLVE_TASK_MODE` + `budget_tiers`** (v8.13.5) — declarative profile tier select. Used when the profile has a `budget_tiers` map AND the env var matches a key.
3. **`max_budget_usd`** (v8.12.x baseline) — static profile default. Always present.

### v8.13.4: per-invocation override

When a subagent task is unusually research-heavy or long-running and the static `max_budget_usd` in `.evolve/profiles/<agent>.json` is too tight, override per-invocation:

```bash
EVOLVE_MAX_BUDGET_USD=1.50 bash scripts/subagent-run.sh scout <cycle> <workspace>
```

The adapter logs the override loudly (`[claude-adapter] override max-budget-usd: ... (was ... from profile)`). Empty/malformed values → WARN + profile fallback. Negative values → rejected.

**When to use**: one-offs where the structured tier doesn't fit. Routine bypassing = CLAUDE.md violation; if your agent consistently needs more budget, declare a tier instead.

### v8.13.5: declarative task-mode tiers

For agents with structurally-different workloads (e.g., Scout doing codebase-scan vs Scout doing research-heavy web search), declare named tiers in the profile:

```json
{
  "max_budget_usd": 0.50,
  "budget_tiers": {
    "default": 0.50,
    "research": 1.50,
    "deep": 2.50
  }
}
```

Then select via `EVOLVE_TASK_MODE`:

```bash
EVOLVE_TASK_MODE=research bash scripts/subagent-run.sh scout <cycle> <workspace>
```

Adapter logs: `[claude-adapter] task-mode tier: research → $1.50 (was 0.50 from profile scout.json)`. Mode key absent from `budget_tiers` → WARN + profile fallback. No `budget_tiers` in profile + `EVOLVE_TASK_MODE` set → WARN.

**When to use**: agents whose workloads naturally cluster into 2-3 budget classes. The Scout profile (`.evolve/profiles/scout.json`) ships with `default` / `research` / `deep` tiers as the canonical example.

**Combining both**: `EVOLVE_TASK_MODE=research EVOLVE_MAX_BUDGET_USD=3.00` runs Scout with $3.00 cap; the explicit override wins, but the tier-resolution log line still appears for observability.

### Forward compatibility

These mechanisms complement (don't replace) Anthropic's `task_budget` (model-self-pacing). Once Claude Code adds `task_budget` support (currently API-only — see [Anthropic docs](https://platform.claude.com/docs/en/build-with-claude/task-budgets)), evolve-loop will integrate it as a fourth tier in the precedence chain. Hard $$ caps and declarative tiers remain useful even with model-self-pacing.

## Verification Before Claiming Done (v8.13.3+)

Three patterns the /insights audit identified as recurring friction. Apply ALL of them before reporting a task complete:

1. **Probe before declaring a CLI unavailable.** Never say "no `<tool>` command" without first running:
   ```bash
   bash scripts/probe-tool.sh <tool>      # canonical helper, checks PATH + common install dirs
   # or directly:
   command -v <tool> 2>/dev/null || which <tool> 2>/dev/null || ls /usr/local/bin/<tool> ~/.local/bin/<tool> 2>/dev/null
   ```
   The audit caught one instance where Claude said "no gws command" when `gws` was installed at `~/.local/bin/`. List what you checked in your response.

2. **Read actual exports before implementing against a module.** When working in a worktree or generating code that imports from `module X`, run `Read` on `X` first and list its real exports. Do not invent function names from context. The audit caught Builder agents shipping code against imagined APIs that didn't match `enhancedAdaptive.ts`'s actual exports — requiring full rewrites.

3. **Run tests after multi-file refactors and report pass/fail counts.** A claim of "tests pass" without explicit numbers is unverified. Format: `bash scripts/<suite>.sh — N/N PASS, no regression`.

If any of the three doesn't apply (e.g., no test infra exists), say so explicitly rather than skipping.

## Shell & Environment Conventions (v8.13.3+)

This project's shell scripts target **bash 3.2** (macOS default) for portability. Multiple regressions in /insights traced to bash-4-only features.

### Banned in shell scripts:

- `declare -A` (associative arrays — bash 4+)
- `mapfile` / `readarray` (bash 4+)
- `${var^^}` / `${var,,}` case modifications (bash 4+)
- GNU-only sed flags: `sed -i ''` is BSD-incompatible — write to a `.tmp.$$` file and `mv` instead
- GNU-only `date -d` — use `date -u -j -f "%Y-%m-%dT%H:%M:%SZ" "$ts" +%s` on macOS, fallback chain `gdate || date -d || date -j -f` for portability

### Required for shell scripts:

- `set -uo pipefail` (NOT `set -e` for orchestrator scripts that need to capture sub-script exit codes — `set -e` interacts badly with `if !cmd; then; rc=$?` patterns where `rc` ends up 0)
- Atomic writes via mv-of-temp: `printf ... > "${file}.tmp.$$" && mv -f "${file}.tmp.$$" "$file"`
- `git diff HEAD` to capture tree-state SHAs (untracked files don't count — match the audit-binding model)

### SSE / streaming endpoints (when you encounter them):

- Disable middleware buffering explicitly
- Add explicit timeouts
- Provide a cancel-UI button — don't rely on browser-side timeout alone

The audit caught the wiki Add Topics feature where SSE buffering blocked article generation; resolution required all three.

## Confirm Direction Before Multi-Cycle Work (v8.13.3+)

For ambiguous requests like "research X and integrate into Y" or "design a system for Z", produce a **3-bullet plan with success criteria** before invoking any tools beyond Read/Grep. Wait for user confirmation before proceeding.

Multi-cycle evolve-loop runs (≥5 cycles) MUST do this. The audit identified the "giant useless circle" force-graph as a case where Claude built for 25 cycles in the wrong direction; a 3-bullet pre-check would have caught it in <5 minutes.

Format:

```
Direction:
- <approach in one sentence>
- <key decision and tradeoff>
- <what success looks like, measurably>

Proceed? (yes/redirect)
```

This rule does NOT apply to:
- Single-cycle bug fixes
- Tasks where the user has already specified file paths and behavior
- Tasks invoked from a previously-approved plan (the plan is the confirmation)

## Cross-Platform Behavior (v8.15.0+)

evolve-loop ships as a Claude Code plugin but is invocable from Gemini CLI via a hybrid driver. The skill content is platform-neutral; the runtime requires the `claude` binary regardless of which CLI initiated the conversation.

### How dispatch works per CLI

| Caller | Skill activation | `cli` field in profile | Adapter | Engine |
|---|---|---|---|---|
| Claude Code | `Skill` tool | `claude` | `scripts/cli_adapters/claude.sh` | `claude -p` (sandboxed) |
| Gemini CLI | `activate_skill` | `gemini` | `scripts/cli_adapters/gemini.sh` (hybrid shim) | delegates to `claude.sh` → `claude -p` |
| Codex CLI | (stub) | `codex` | `scripts/cli_adapters/codex.sh` | exits 99 — unsupported |

The hybrid pattern exists because Gemini CLI lacks non-interactive prompt mode (`gemini -p`), `--max-budget-usd`, and subagent dispatch. Without these, the trust boundary (`role-gate`, `ship-gate`, `phase-gate-precondition`) can't structurally enforce phase isolation. By delegating to `claude.sh`, Gemini-driven cycles inherit the full Claude Code kernel-hook protection. See [docs/incidents/gemini-forgery.md](docs/incidents/gemini-forgery.md) for why this matters.

### Where the platform-specific knowledge lives

- `docs/platform-compatibility.md` — top-level support matrix and adapter contract
- `skills/evolve-loop/reference/platform-detect.md` — env-var probe table for runtime detection
- `skills/evolve-loop/reference/<platform>-tools.md` — tool name translation (`Read` → `read_file`, etc.)
- `skills/evolve-loop/reference/<platform>-runtime.md` — invocation patterns per CLI
- `scripts/detect-cli.sh` — shell helper that returns one of `claude | gemini | codex | unknown`
- `scripts/cli_adapters/<cli>.sh` — runtime adapter; receives env-var contract from `subagent-run.sh`

### When implementing for a new CLI

Mirror the hybrid pattern (delegate to `claude.sh`) before attempting a native adapter. The native-adapter path requires verifying the new CLI has: non-interactive prompt mode, profile-scoped permissions, and either a budget cap flag or external cost tracking. Until those are confirmed, the hybrid path keeps the trust boundary intact.

## Swarm Architecture (v8.16+)

evolve-loop is structured around three composable layers (see [docs/architecture/tri-layer.md](docs/architecture/tri-layer.md) for the formal spec):

| Layer | Files | Purpose |
|---|---|---|
| **Skill** | `skills/<name>/SKILL.md` | Workflow + steps + exit criteria — the *how* |
| **Persona** | `agents/<role>.md` | One role, one perspective, one output format — the *who* |
| **Command** | `.claude-plugin/commands/<name>.md` | User-facing entry point — the *when* (orchestration) |

**The governing rule:** the user (or a slash command) is the orchestrator. **Personas do not invoke other personas.** Claude Code enforces this at runtime: subagents cannot spawn subagents.

### Sprint 1 — Pattern 3 fan-out (parallel sub-personas + aggregator)

Scout, Auditor, and Retrospective each fan out into specialized parallel sub-personas, then `scripts/aggregator.sh` merges them into the canonical phase artifact:

```bash
EVOLVE_FANOUT_ENABLED=1 EVOLVE_FANOUT_SCOUT=1 \
  bash scripts/subagent-run.sh dispatch-parallel scout <cycle> <workspace>
```

| Phase | Sub-personas | Default-off env flag |
|---|---|---|
| Scout | scout-codebase, scout-research, scout-evals | `EVOLVE_FANOUT_SCOUT` |
| Auditor | audit-eval-replay, audit-lint, audit-regression, audit-build-quality | `EVOLVE_FANOUT_AUDITOR` |
| Retrospective | retro-instinct, retro-gene, retro-failure | `EVOLVE_FANOUT_RETROSPECTIVE` |

Master switch: `EVOLVE_FANOUT_ENABLED=1`. Concurrency cap: `EVOLVE_FANOUT_CONCURRENCY` (default 4). Per-worker timeout: `EVOLVE_FANOUT_TIMEOUT` (default 600s). Builder is **excluded** from fan-out — single-writer invariant on the worktree.

### Sprint 2 — Multi-lens plan review (gstack `/autoplan` inspired)

A new `plan-review` phase between `discover` and `tdd` runs four lens reviewers (CEO/Eng/Design/Security) in parallel against `scout-report.md`. Aggregator computes verdict:

| Verdict | Trigger | Orchestrator action |
|---|---|---|
| PROCEED | Avg ≥ 7 AND no lens < 5 | Advance to TDD |
| REVISE | Avg ≥ 5 AND any lens < 5 | Re-run Scout (max 2 retries) |
| ABORT | Any explicit ABORT, OR avg < 5 | End cycle |

Default-off via `EVOLVE_PLAN_REVIEW=0`. Phase gate `gate_plan_review_to_tdd` enforces verdict at the kernel layer.

### Sprint 3 — Tri-layer composable skill catalog

7 new composable skills (`skills/evolve-{spec,plan-review,tdd,build,audit,ship,retro}/`) compose with the existing macro:

| Skill | Maps to slash command | Pattern |
|---|---|---|
| `evolve-spec` | `/scout` (codebase sub-scout) | Pattern 3 fan-out |
| `evolve-plan-review` | `/plan-review` | Pattern 3 fan-out |
| `evolve-tdd` | `/tdd` | Pattern 1/2 (single persona) |
| `evolve-build` | `/build` | Pattern 1/2 (single, single-writer) |
| `evolve-audit` | `/audit` | Pattern 3 fan-out |
| `evolve-ship` | `/ship` | Pattern 1/2 (atomic) |
| `evolve-retro` | `/retro` | Pattern 3 fan-out |
| `evolve-loop` (existing macro) | `/loop` | **Pattern 5** auto-orchestrated under trust kernel |

**Pattern 5 is specific to evolve-loop** because the trust kernel (sandbox + ledger SHA + phase-gate) substitutes for the human checkpoints addyosmani's framework relies on — see [docs/architecture/tri-layer.md](docs/architecture/tri-layer.md) for justification.

### Worker name pattern (`<role>-worker-<subtask>`)

Fan-out workers invoke `subagent-run.sh <role>-worker-<name>`. Examples:
- `scout-worker-codebase`, `scout-worker-research`, `scout-worker-evals`
- `auditor-worker-eval-replay`, `auditor-worker-lint`, `auditor-worker-regression`, `auditor-worker-build-quality`

`cmd_run` strips the `-worker-<name>` suffix to find the parent profile (`scout.json`), but writes to `<workspace>/workers/<full-agent>.md`. `phase-gate-precondition.sh` derives the parent role for sequence checking — workers inherit phase eligibility from their parent role's expected-agent set.

### Verifying the swarm architecture

Run `bash scripts/swarm-architecture-test.sh` to verify all three layers wire correctly (40 assertions covering plugin.json registrations, skill files, slash commands, persona files, profile parallel_subtasks, state machine, phase gate, aggregator merge modes, dispatch-parallel command, and end-to-end smoke test).

## Evolve Loop Task Priority

When selecting tasks for `/evolve-loop` cycles, follow this priority order:

1. **New features** — Building new functionality is the top priority
2. **Bug fixes** — Fixing potential bugs is second priority
3. **Security issues** — Fixing security vulnerabilities is last priority
