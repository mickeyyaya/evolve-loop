# Orchestrator Context Modes

> **Status:** Stable since v10.10.0 (cycle 97)
> **Audience:** Operators tuning per-phase context size; persona authors adding new agents
> **Source:** `scripts/lifecycle/role-context-builder.sh:200-250`, `.evolve/profiles/*.json:context_mode`

## TL;DR

Every phase agent receives a context block assembled by `role-context-builder.sh`. The block has two flavors:

| Mode | What the agent sees | Use when |
|---|---|---|
| **`full`** (default) | Whole `state.json` arrays — `carryoverTodos[]`, `instinctSummary[]`, `failedApproaches[]`, `abnormalEvents[]`, plus full `intent.md` + relevant phase artifacts | Phase needs raw evidence (Builder, Auditor, Retrospective) OR last cycle hit a FAIL/WARN |
| **`digest`** | Pre-summarized cycle digest (`.evolve/runs/cycle-N/digest.md`) + compact intent + carryover headers only | Phase needs cycle direction, not raw history (Orchestrator, Scout for stable cycles) |

Selection is **per-agent**, declared in `.evolve/profiles/<agent>.json:context_mode`. Env-var `EVOLVE_CONTEXT_DIGEST=0|1` overrides per invocation. Recent FAIL on `state.json:failedApproaches[].classification ∈ {code-audit-fail, code-build-fail}` auto-promotes to `full` even when profile says `digest` — so a recovering cycle never gets under-fed context.

## Why this exists

Before cycle 97 (v10.10.0), every phase agent received the same kitchen-sink context block: full `state.json`, full `scout-report`, full `build-report`, full `audit-report`, full `retrospective` if present. Measurements from `knowledge-base/research/cycle-21-cost-attribution.md` showed the Orchestrator phase alone was burning 8-15 K tokens per cycle on context the role didn't act on. The Builder doesn't reason about retrospective theory; the Auditor doesn't need Scout's raw research notes (it has the build-report).

Cycle 97 shipped two structural changes:

1. **`context_mode` field on agent profiles.** Declarative per-role. Default unchanged (`full`) so existing agents preserve behavior. Orchestrator was flipped to `digest` because its role is sequencing, not synthesis.
2. **FAIL-path promotion.** When the previous cycle failed, the orchestrator silently flips back to `full` for the recovering cycle so remediation has complete defect context. This eliminates a footgun where a digest-mode orchestrator could under-feed a Builder fix attempt.

## Implementation contract

### Profile declaration

`.evolve/profiles/orchestrator.json`:

```json
{
  "name": "orchestrator",
  "role": "orchestrator",
  "context_mode": "digest",
  "...": "..."
}
```

Profiles without `context_mode` field default to `full` (behavior preserved from pre-v10.10).

### Selection logic (`role-context-builder.sh:206-250`)

```text
function _load_profile_context_mode():
  if jq unavailable                       → return (no-op, fail-open to full)
  if profile.context_mode != "digest"     → return (no-op)
  if EVOLVE_CONTEXT_DIGEST already set    → return (env-var precedence wins)
  if state.failedApproaches[-1].classification in {code-audit-fail, code-build-fail}:
      EVOLVE_CONTEXT_DIGEST=0             → force full (FAIL recovery)
      return
  EVOLVE_CONTEXT_DIGEST=1                 → activate digest
  EVOLVE_CONTEXT_DIGEST_FROM_PROFILE=1    → mark provenance
```

The order matters:

1. **Tool availability check** — `jq` is required; without it, fail open to `full` so we never silently emit a degraded context.
2. **Profile read** — only `context_mode: "digest"` activates the path. Any other value (or missing field) is a no-op.
3. **Env-var precedence** — operators can force a mode via `EVOLVE_CONTEXT_DIGEST=0` or `=1` without editing profiles.
4. **FAIL guard** — only the **last** entry of `failedApproaches[]` matters. A subsequent PASS cycle re-enables digest mode. Two classifications trip the guard: `code-audit-fail` (Auditor verdict FAIL) and `code-build-fail` (Builder failed to compile/predicate).

The fail-open is deliberate: if the loader cannot determine intent, emit the larger context. A few wasted tokens beat an under-fed agent corrupting a recovery cycle.

### Digest payload

When `EVOLVE_CONTEXT_DIGEST=1`, instead of dumping raw state arrays, the builder emits a single block sourced from `.evolve/runs/cycle-N/digest.md` (lazy-built via `scripts/lifecycle/build-cycle-digest.sh` if absent). The digest is a pre-compressed ~500-800 token summary of:

- Cycle direction (intent goal + AwN classification)
- Top 3 carryover items (not full backlog)
- Top 3 active instincts (not full `instinctSummary[]`)
- Failed-approach summary line (not full array)
- Cycle phase + last advance timestamp

Full-mode emission still happens for `scout-report.md`, `build-report.md`, etc. — only the **state-array preamble** flips between digest and full.

## Measured impact

From `knowledge-base/research/v10-17-0-release-debrief.md` per-cycle breakdown:

| Cycle | Orchestrator phase tokens (estimated) | Mode |
|---|---|---|
| 93 (pre-flip) | ~9 K | full |
| 97 (default) | ~3 K | digest (default-on via profile) |
| 98 (recovery test) | ~9 K | digest profile, promoted to full by FAIL guard |

Per-cycle savings: ~6 K tokens on the orchestrator-phase preamble. At cycle volumes typical of v10.17 batches (5-10 cycles per session), that's 30-60 K tokens saved per session — roughly $0.10-0.20 in Opus pricing or $0.03-0.06 in Haiku.

## Compatibility

- **v10.10.0+** has both the profile field and FAIL-guard logic.
- **v8.62.0–v10.9.x** had `EVOLVE_CONTEXT_DIGEST` env-var only (no profile field). Operators set it manually per invocation.
- **Pre-v8.62** had no digest path. All agents always saw full state.

A profile written with `context_mode: "digest"` is **forward-compatible**: it activates on v10.10+ and is silently ignored on older versions (no error, just full mode).

## Operator controls

| Need | How |
|---|---|
| Disable digest for one invocation | `EVOLVE_CONTEXT_DIGEST=0 /evolve-loop ...` |
| Enable digest for an agent without profile field | `EVOLVE_CONTEXT_DIGEST=1 /evolve-loop ...` |
| Permanently flip an agent | Edit `.evolve/profiles/<agent>.json`, add `"context_mode": "digest"` |
| Check what mode was emitted | Look for `EVOLVE_CONTEXT_DIGEST_FROM_PROFILE=1` in role-context-builder stderr / ledger |
| Force full mode after a non-recorded failure | `EVOLVE_CONTEXT_DIGEST=0` until the next PASS |

## Failure modes and guards

| Failure | Detection | Behavior |
|---|---|---|
| `jq` missing | `command -v jq` check | Skip profile read; emit full (fail-open) |
| Profile file missing | `[ -f "$_lpcm_profile" ]` check | Skip; emit full |
| `state.json` unreadable | jq parse error caught with `|| echo ""` | Skip FAIL guard; honor profile mode |
| `failedApproaches[]` empty/missing | jq returns empty | Treat as no-fail; honor profile mode |
| Digest file missing | `build-cycle-digest.sh` lazy-builds | Re-runs digest construction; emits result |

All paths are designed to **degrade to full mode** rather than fail. Under-feeding an agent is worse than over-feeding it.

## See also

- `docs/architecture/context-window-control.md` — `EVOLVE_PROMPT_MAX_TOKENS` soft cap and autotrim behavior
- `docs/architecture/sequential-write-discipline.md` — companion: `parallel_eligible` field on profiles
- `docs/architecture/token-economics-2026.md` — full token-cost roadmap (P1–P8)
- `knowledge-base/research/cycle-21-cost-attribution.md` — original measurement that motivated digest mode
- `knowledge-base/research/v10-17-0-release-debrief.md` §2 — cycle 97 ship and post-ship measurement
- ACS predicates that lock this contract:
  - `acs/regression-suite/cycle-97/001-orchestrator-profile-has-context-mode-digest.sh`
  - `acs/regression-suite/cycle-97/002-role-context-builder-honors-profile-context-mode.sh`
  - `acs/regression-suite/cycle-97/003-role-context-builder-env-var-wins.sh`
  - `acs/regression-suite/cycle-97/004-role-context-builder-promotes-to-full-on-fail.sh`
