# evolve-loop Release Archive

> This file contains implementation details, incident post-mortems, and migration notes for v8.13–v8.37. It is **not** auto-loaded into agent context — it is reference documentation for operators investigating historical behavior or debugging regressions.
>
> For current behavioral rules, see [CLAUDE.md](../CLAUDE.md).

---

## v8.37.0 — Tamper-Evident Ledger Hash Chain

The ledger (`.evolve/ledger.jsonl`) records every subagent invocation with cycle binding (HEAD + tree_state_sha at audit time), challenge-token, and artifact_sha256. v8.37.0 hardens the **forensic integrity** of this record so future audits, retrospectives, and operator investigations can detect tampering — covering threats the runtime enforcement layers (phase-gate, role-gate, ship-gate) don't address:

| Threat | Detection in v8.37.0 |
|---|---|
| **Entry rewrite** (modify a historical line to flip a verdict) | Each new entry's `prev_hash` = SHA256 of the previous entry's full JSON line. Modifying any historical entry breaks every entry after it. |
| **Entry forgery / insertion** (splice a fake entry between two real ones) | The next legitimate entry's `prev_hash` references the original — chain breaks immediately. |
| **Truncation** (lop the last N entries to hide a failed cycle) | `.evolve/ledger.tip` records `<seq>:<sha256>` of the latest entry. Truncation makes tip-vs-actual-last-line mismatch. |
| **Concurrent fan-out race** (two writers compute the same prev_hash) | Verifier flags duplicate prev_hash as a chain anomaly. |

**Pipeline impact: zero.** The `prev_hash` and `entry_seq` fields are additive. Existing readers all use jq's `// empty` pattern — unknown fields are ignored.

**Verifier:** `bash scripts/verify-ledger-chain.sh` — exit 0 = chain intact, 1 = chain break, 2 = tip mismatch.

**Three writers updated:** `scripts/subagent-run.sh:write_ledger_entry()`, `scripts/subagent-run.sh:_write_fanout_ledger_entry()`, `scripts/merge-lesson-into-state.sh`.

**Pre-v8.37 entries** (no `prev_hash` field) are tolerated as a boundary; the first v8.37+ entry chains from the last pre-v8.37 entry's SHA.

---

## v8.36.0 — Worktree Provisioning Robustness

`run-cycle.sh` pre-flight cleanup now runs `git worktree prune` BEFORE attempting `git branch -D`. Closes a recurring failure mode in nested-Claude environments: when a prior cycle was hard-killed at a different `$TMPDIR`-based worktree path, `.git/worktrees/cycle-N/` retained a stale admin pointer. `git branch -D` silently no-ops on a branch that's still admin-checked-out; `git worktree add` then fails with `fatal: a branch named 'evolve/cycle-N' already exists`.

The fix: `git worktree prune` removes admin entries for worktrees whose directories no longer exist. Active worktrees (whose dirs still exist) are NOT touched, so concurrent cycles are safe.

Two locations got the fix: pre-flight cleanup (`scripts/run-cycle.sh:~382`, primary fix) and the `cleanup()` EXIT trap (`scripts/run-cycle.sh:~298`, defensive).

Test: `scripts/run-cycle-worktree-test.sh` (8 assertions).

---

## v8.35.0 — Orchestrator Fluency on WARN + Adaptive Auditor

**Fix 1 — Orchestrator ships on WARN by default** (matches v8.28.0 ship.sh policy).

| Verdict | Pre-v8.35.0 | v8.35.0 |
|---|---|---|
| PASS | ship.sh | ship.sh (unchanged) |
| **WARN** | record-failure-to-state.sh, skip ship | **record (low-severity awareness) THEN ship.sh** |
| FAIL | record-failure-to-state.sh, skip ship | (unchanged) |
| EVOLVE_STRICT_AUDIT=1 + WARN | skip ship | skip ship (legacy behavior preserved) |

**Fix 2 — New `code-audit-warn` classification:** severity=low, age_out=86400 (1d), retry=yes. `failure_normalize_legacy "WARN" → code-audit-warn` (was `code-audit-fail`).

**Fix 3 — Adaptive auditor model selection** via `scripts/diff-complexity.sh`:
- **trivial** (≤3 files AND ≤100 lines AND no security paths) → Sonnet
- **complex** (>10 files OR >500 lines OR security path regex) → Opus (default)
- **standard** → Opus (conservative)

Overrides: `MODEL_TIER_HINT=opus`, `EVOLVE_AUDITOR_TIER_OVERRIDE=opus`, `EVOLVE_DIFF_COMPLEXITY_DISABLE=1`.

---

## v8.34.0 — Pipeline Continuation + Diff Transparency

**Fix 1 — `ship.sh` advances `state.json:lastCycleNumber` on successful cycle ship.** Pre-v8.34, only failure paths wrote `lastCycleNumber`. Successful ships left it unchanged → the dispatcher's next iteration computed `ran_cycle = last_before + 1 = the SAME cycle just shipped` → 5-repeat circuit-breaker fired prematurely. Only for `--class cycle`.

**Fix 2 — `ship.sh` appends an `## Actual diff` footer to commit messages.** Allows reviewers and future audits to spot message-vs-diff divergence in `git log`. Format:

```
<original commit message>

---
## Actual diff (v8.34.0+)

Files modified (N):
- M path/to/file1.ts
- A path/to/newfile.tsx

 N files changed, NN insertions(+), NN deletions(-)
```

Skipped for `--class release`. Intentionally non-blocking — divergence is recorded, not rejected.

---

## v8.33.0 — Token Optimization

Three surgical fixes targeting per-cycle input-token spend:

1. **Cache-friendly prompt order:** `run-cycle.sh:build_context()` emits static agent prompt first (cache hit after first cycle in a 5-minute batch window).
2. **Conditional context blocks:** Pre-v8.33 emitted empty headers unconditionally (~500-1000 token padding for cycle 1). Now emits each block only when non-empty.
3. **Per-cycle cost summary in dispatcher:** `show-cycle-cost.sh --json` → `[dispatch] cycle N cost: $X.XX (scout=..., builder=..., auditor=...) cache_hit=XX%`.

**Side fix:** `show-cycle-cost.sh` now honors `EVOLVE_PROJECT_ROOT` (writable side of dual-root).

---

## v8.23.0 — Swarm Optimization

Sprint 1's Pattern-3 fan-out gained three optimizations:

| Flag | Default | Effect |
|---|---|---|
| `EVOLVE_FANOUT_CANCEL_ON_CONSENSUS` | `0` | Cancel remaining workers when K agree on FAIL |
| `EVOLVE_FANOUT_CONSENSUS_K` | `2` | Number of agreeing workers required |
| `EVOLVE_FANOUT_CONSENSUS_POLL_S` | `1` | Polling interval (seconds) |
| `EVOLVE_FANOUT_CACHE_PREFIX` | `1` | Write deterministic cache-prefix.md shared across siblings (~47% input-token reduction) |
| `EVOLVE_FANOUT_TRACK_WORKERS` | `1` | Write per-worker status into `cycle-state.json:parallel_workers.workers[]` |

**`parallel_workers.workers[]` schema:**
```json
"parallel_workers": {
  "agent": "scout", "count": 3, "started_at": "...",
  "workers": [
    {"name": "scout-codebase", "status": "done", "exit_code": 0},
    {"name": "scout-research", "status": "running"},
    {"name": "scout-evals", "status": "pending"}
  ]
}
```
Operator helpers: `cycle-state.sh init-workers <agent> <name>...`, `cycle-state.sh set-worker-status <name> <status> [<exit_code>]`.

---

## v8.22.0 — Failure Adaptation Kernel

Promotes failure adaptation from a prompt rule to a deterministic kernel function:

```bash
bash scripts/failure-adapter.sh decide --state .evolve/state.json
# emits JSON: {action, reason, remediation, set_env, skip_phases, verdict_for_block, evidence}
```

**Structured taxonomy** (7 classifications in `scripts/failure-classifications.sh`):

| Classification | Age-out | Severity |
|---|---|---|
| `infrastructure-transient` | 1 day | low |
| `infrastructure-systemic` | 7 days | high |
| `intent-malformed` | 1 day | low |
| `intent-rejected` | never | terminal |
| `code-build-fail` | 30 days | high |
| `code-audit-fail` | 30 days | high |
| `human-abort` | 1 hour | low |

**Decision rules** (priority order):
1. `intent-rejected` → `BLOCK-CODE` / `SCOPE-REJECTED`
2. `infrastructure-systemic` → `BLOCK-OPERATOR-ACTION` / `BLOCKED-SYSTEMIC`
3. 2+ `code-audit-fail` → `BLOCK-CODE` / `BLOCKED-RECURRING-AUDIT-FAIL`
4. 2+ `code-build-fail` → `BLOCK-CODE` / `BLOCKED-RECURRING-BUILD-FAIL`
5. 3+ consecutive `infrastructure-transient` → `BLOCK-OPERATOR-ACTION` / `BLOCKED-SYSTEMIC`
6. 1+ `infrastructure-transient` → `RETRY-WITH-FALLBACK`
7. otherwise → `PROCEED`

**Operator utilities:** `bash scripts/state-prune.sh --classification <name>`, `--age 7d`, `--cycle <N>`, `--all --yes`.

---

## v8.21.0 — Worktree Provisioning Contract

Per-cycle git worktrees are provisioned by `scripts/run-cycle.sh` at `$EVOLVE_PROJECT_ROOT/.evolve/worktrees/cycle-N` on branch `evolve/cycle-N`. The path is recorded in `cycle-state.json:active_worktree`, exported as `WORKTREE_PATH`, and torn down via the EXIT trap.

**Trust-boundary invariant:** the orchestrator and all phase agents may NOT call `git worktree add` or `git worktree remove`. Both are denied in `orchestrator.json` and in every phase profile that has a deny list.

This closes the architectural gap that made v8.13.x–v8.20.2 require `EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1`: previously no component provisioned the build worktree, leaving `cycle-state.active_worktree` null, causing all source writes to EPERM.

---

## v8.24.0 — Dispatcher Safety Nets

Three structural fixes preventing the "cycle-N runs M× and burns budget" deadlock:

1. **Pre-flight state.json writability check.** Dispatcher tests writability before any cycle. Catches OS-sandbox-blocks-state.json at $0 cost.
2. **Fail-loud on `record_failed_approach` write EPERM.** Both `mv -f` calls error-checked; FATAL log + return 1. Propagates as DISPATCH_RC=1 + break.
3. **Same-cycle circuit-breaker.** Tracks `prev_ran_cycle`; aborts batch when N consecutive iterations report the same cycle number (default `EVOLVE_DISPATCH_REPEAT_THRESHOLD=3`).

---

## v8.32.0 — Version-Aware TOFU

Pre-v8.32.0, `ship.sh` pinned its own SHA in `state.json:expected_ship_sha` on first run. Plugin updates changed the SHA → INTEGRITY-FAIL on next cycle.

v8.32.0 pins BOTH SHA AND plugin version:

| State | What v8.32.0 does |
|---|---|
| First run (no pin) | Pin both SHA + version |
| Same version + same SHA | Pass (clean) |
| Same version + different SHA | **REAL TAMPERING** — fail |
| Different version (plugin updated) | **Re-pin** SHA + version, continue |
| Legacy state.json (SHA pinned, no version) | Auto-migrate to version-aware schema |

---

## v8.31.0 — Builder Write-Leak Fix

The Builder profile previously had `Edit(scripts/**)` in `disallowed_tools` AND bare `Bash` in `allowed_tools`, forcing Builder to use Bash for script edits. Bare Bash has no path gating. With v8.25.1 disabling inner `sandbox-exec` in nested-Claude, nothing prevented Builder from writing to main repo.

**Cycle-25 evidence:** 5 files modified in main repo's working tree despite `active_worktree=/var/folders/.../cycle-25`.

**Fix:** Remove redundant `Edit(scripts/**)` denials from Builder's `disallowed_tools` (path gating is enforced by `role-gate.sh`); add interpreter-execution Bash denials: `perl`, `ruby`, `python3 -c`, `node -e`, `sh -c`, `bash -c`, `zsh -c`, `env`, `exec`, `eval`, `awk`, `unlink`, `ln`.

---

## v8.30.0 — Operational Polish

1. **`run-cycle.sh` exit 1 → recoverable when orchestrator-report exists.** Only abort when no report exists (true breach) or classification is `integrity-breach`. Others record and continue (rc=3).
2. **`ship.sh` rejects dual-verdict reports.** When audit-report.md contains both `Verdict: PASS` AND `Verdict: FAIL`, ship-gate refuses with "auditor produced inconsistent artifact".
3. **`cycle-state.sh advance` clears `parallel_workers`.** Prevents stale per-worker state from one phase contaminating the next.
4. **SKILL.md goal-quoting docs.** When `<args>` contains an apostrophe, wrap the goal in double quotes.

---

## v8.28.0 — Fluent-by-Default

Operating principle: reduce friction is top priority; preventing structural fakery is equal priority. v8.28.0 inverts defaults:

| Behavior | Pre-v8.28.0 default | v8.28.0 default | Restore via |
|---|---|---|---|
| Audit `Verdict: WARN` | Block ship | **Ships, logs WARN** | `EVOLVE_STRICT_AUDIT=1` |
| 1+ infrastructure-systemic | BLOCK-OPERATOR-ACTION | **PROCEED with awareness** | `EVOLVE_STRICT_FAILURES=1` |
| 2+ code-audit-fail | BLOCK-CODE | **PROCEED with awareness** | `EVOLVE_STRICT_FAILURES=1` |
| 2+ code-build-fail | BLOCK-CODE | **PROCEED with awareness** | `EVOLVE_STRICT_FAILURES=1` |
| Same-cycle circuit-breaker | 3 iterations | **5 iterations** | `EVOLVE_DISPATCH_REPEAT_THRESHOLD=3` |
| Expired failedApproaches entries | Linger on disk | **Auto-pruned at dispatcher start** | `EVOLVE_AUTO_PRUNE=0` |

**Anti-gaming preserved.** The cycle 102–111, 132–141, and Gemini-forgery incidents were all caught by Tier-1 (SHA + verdict-text + phase-gate ordering). None of those structural defenses change.

---

## v8.27.0 — Loop Continuation

**Fix 1 — `ship.sh` accepts auditor exit_code 0 OR 1.** Unix convention: `exit 0` = no findings, `exit 1` = findings present (normal), `exit 2+` = true error. Pre-v8.27.0 ship-gate rejected ANY non-zero exit. The artifact-content checks (SHA + Verdict + cycle binding + freshness) are the actual source of truth.

**Fix 2 — `ship-gate-config` classification.** Cycles where audit declared PASS but ship-gate refused are now classified as `ship-gate-config` (1d age-out, low severity, retry-yes), NOT `infrastructure-systemic`.

**Fix 3 — `--reset` flag.** `bash scripts/evolve-loop-dispatch.sh --reset [N] [strategy] [goal]` runs `state-prune.sh` against `infrastructure-{systemic,transient}` + `ship-gate-config` entries before the cycle loop.

---

## v8.26.0 — Budget Cap Default-Unlimited

Pre-v8.26.0, `--max-budget-usd` was sized per-phase from the profile (~$0.18 Scout default, $0.50 Intent, $1.00 Orchestrator). Complex meta-goals routinely exceeded these caps mid-thought, exiting subagents with `BUDGET_EXCEEDED`.

v8.26.0 sets `--max-budget-usd` to `999999` (effectively unlimited) by default. The flag is still passed because the claude binary expects it. Budget caps don't prevent reward hacking — Tier-1 hooks enforce that.

| Env var | Effect |
|---|---|
| (default) | `--max-budget-usd 999999` |
| `EVOLVE_BUDGET_CAP=<value>` | Hard cap (wins over all) |
| `EVOLVE_BUDGET_ENFORCE=1` | Legacy strict mode: use profile-resolved `MAX_BUDGET` |
| `EVOLVE_MAX_BUDGET_USD=<value>` | Per-invocation override (only applies under `ENFORCE=1`) |
| `EVOLVE_TASK_MODE=<tier>` | Tier resolution from `budget_tiers` (only under `ENFORCE=1`) |

---

## v8.13.4 / v8.13.5 — Subagent Budget Controls (detailed examples)

### v8.13.4: per-invocation override

```bash
EVOLVE_MAX_BUDGET_USD=1.50 bash scripts/subagent-run.sh scout <cycle> <workspace>
```

The adapter logs the override loudly. Empty/malformed values → WARN + profile fallback. Negative values → rejected.

**When to use:** one-offs where the structured tier doesn't fit. Routine bypassing = CLAUDE.md violation; if your agent consistently needs more budget, declare a tier instead.

### v8.13.5: declarative task-mode tiers

Declare named tiers in the profile:

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

Adapter logs: `[claude-adapter] task-mode tier: research → $1.50 (was 0.50 from profile scout.json)`. The Scout profile (`.evolve/profiles/scout.json`) ships with `default` / `research` / `deep` tiers as the canonical example.

**Combining both:** `EVOLVE_TASK_MODE=research EVOLVE_MAX_BUDGET_USD=3.00` — explicit override wins.

### Forward compatibility

Once Claude Code adds `task_budget` support (currently API-only), evolve-loop will integrate it as a fourth tier in the precedence chain. Hard $$ caps and declarative tiers remain useful even with model-self-pacing.
