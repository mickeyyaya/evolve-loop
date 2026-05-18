# Per-Cycle Orchestrator Isolation (v10.x+)

> Canonical reference for the orchestrator's read/write scope and the
> resume-quarantine + per-cycle release protocol. Plan source:
> `~/.claude/plans/linked-meandering-lobster.md`.
>
> Companion docs: [checkpoint-resume.md](checkpoint-resume.md) for the
> --resume protocol, [sequential-write-discipline.md](sequential-write-discipline.md)
> for single-writer invariants, [egps-v10.md](egps-v10.md) for predicate
> gating.

## Why this exists

Cycle 79 (2026-05-17 session) burned **$78.25 of session cost over three
auto-resume attempts** without producing a deliverable. Post-mortem
identified the dominant cost-amplifier: the orchestrator was re-reading
its own killed attempts' workspace artifacts on each resume.

**The leak in three sentences:**

1. The orchestrator profile already denied **writes** to historical paths,
   but `deny_subpaths` is a write-deny mechanism — reads through `Read`,
   `Bash(cat:*)`, `Bash(head:*)`, `Bash(tail:*)` were unrestricted.
2. On `--resume`, attempt-K's prompt re-injected ledger entries from
   attempts 1..K-1 of the SAME cycle (verified in cycle 79 attempt 3's
   prompt: `[cycle:79 role:builder exit:0 sha:8a496b24]` from attempt 2).
3. The workspace dir `.evolve/runs/cycle-79/` retained killed-attempt
   artifacts (`builder-stdout.log`, `audit-report.md`, etc.) — the resumed
   orchestrator opened them via Read, processed them as if they were
   current state, and burned tokens regenerating output that was already
   on disk.

This ADR codifies the structural fix.

## The isolation contract

The orchestrator is a per-cycle subprocess. It MAY:

| Resource | Access |
|----------|--------|
| `.evolve/runs/cycle-<current>/` | full read+write (its workspace) |
| `.evolve/cycle-state.json` | read via `cycle-state.sh get`; writes via the four allowlisted ops (`advance`, `set-agent`, `checkpoint`, `clear-checkpoint`) |
| `.evolve/ledger.jsonl` | **write-only** (via the child `subagent-run.sh` appending phase-agent entries) — reads denied |
| `scripts/**`, `agents/**`, `skills/**` | reads allowed (the persona may need to read its own contract); writes denied |
| Prompt-injected digest fields (`recentLedgerEntries`, `recentFailures`, `instinctSummary`, `adaptiveFailureDecision`) | the **authoritative** source of cross-cycle history |

The orchestrator MAY NOT:

| Resource | Reason |
|----------|--------|
| `.evolve/ledger.jsonl` (Read or Bash cat/head/tail/grep) | use the pre-digested `recentLedgerEntries` injection instead |
| `.evolve/runs/cycle-<N>/.attempt-*/` (any access) | resume-quarantine paths — forensics-preserved on disk but invisible to the resumed orchestrator |
| `.evolve/state.json`, `.evolve/instincts/`, `.evolve/history/`, `.evolve/research/` | already denied via `deny_subpaths`; use `instinctSummary` |
| `git worktree add`/`remove`, `git commit`/`push`, `cycle-state.sh clear`/`init`/`set-worktree` | denied via `disallowed_tools` — these are kernel-owned operations |

Reaching for raw cross-cycle history (e.g., listing `.evolve/runs/`) is
explicitly anti-pattern — the digest is what survives across attempts;
raw artifacts are noise that costs money.

## Enforcement layers

Two layers, with complementary failure modes:

### Layer 1: Claude Code tool permissions (`allowed_tools` / `disallowed_tools`)

The orchestrator profile (`.evolve/profiles/orchestrator.json`) declares
specific Read and Bash deny patterns:

```jsonc
"disallowed_tools": [
  // ledger reads
  "Read(.evolve/ledger.jsonl)",
  "Bash(cat .evolve/ledger.jsonl*)",
  "Bash(head .evolve/ledger.jsonl*)",
  "Bash(tail .evolve/ledger.jsonl*)",
  "Bash(grep:* .evolve/ledger.jsonl*)",
  // quarantine reads
  "Read(.evolve/runs/cycle-*/.attempt-*)",
  "Read(.evolve/runs/cycle-*/.attempt-*/**)",
  "Bash(cat .evolve/runs/cycle-*/.attempt-*/**)",
  // ...etc for head/tail/ls
]
```

**Applies regardless of OS sandbox.** Works in both `sandbox-exec` (macOS),
`bwrap` (Linux), AND in nested-Claude environments where the OS sandbox
is disabled (`EVOLVE_INNER_SANDBOX=0`). This is the load-bearing layer.

### Layer 2: OS sandbox (`sandbox.deny_subpaths`)

`scripts/cli_adapters/claude.sh:494-506` emits `(deny file-write* ...)`
sandbox-exec rules for each `deny_subpaths` entry. **This layer denies
writes only — `file-read*` access remains unrestricted across the repo.**

Quarantine paths and write-deny additions:

```jsonc
"sandbox": {
  "deny_subpaths": [
    // ... existing entries ...
    ".evolve/runs/cycle-*/.attempt-*"
  ]
}
```

**Deferred work:** broad historical-cycle-dir read denial via OS sandbox
would require extending the adapter to emit `(deny file-read* ...)` rules.
Currently the tool-perm layer (Layer 1) is the only enforcement for reads.
For most production paths this is sufficient — adapter extension is a
follow-up if a sandbox-disabled environment proves problematic.

## Resume-quarantine protocol

When a cycle is auto-resumed, prior killed-attempt artifacts are moved
into `.evolve/runs/cycle-N/.attempt-K/` subdirs before the orchestrator
re-spawns. The orchestrator profile (Layer 1) denies reads of these
subdirs, so forensics are preserved but the resumed orchestrator sees a
clean workspace.

```
.evolve/runs/cycle-79/
├── intent.md                  ← current attempt's state, freshly produced
├── scout-report.md            ← (or moved into .attempt-1 if this is resume)
├── .attempt-1/                ← prior killed attempt (denied)
│   ├── builder-stdout.log
│   ├── audit-report.md
│   └── ...
└── .attempt-2/                ← second prior killed attempt (denied)
    └── ...
```

**K is derived from `checkpoint.autoResumeAttempts`** (incremented by
`resume-cycle.sh` step 2b), bumped further if the target `.attempt-K/`
dir already exists. The `.cycle-start-marker` file (used by the
`git-worktree-status` baseline) is preserved through quarantine.

**Defense-in-depth:** quarantine logic lives in BOTH
`scripts/dispatch/resume-cycle.sh` (Step 5) AND
`scripts/dispatch/run-cycle.sh` (Step 4, in the `EVOLVE_RESUME_MODE=1`
block). Whichever runs first moves the artifacts; the second finds an
already-clean workspace and noops. This covers the case where
`run-cycle.sh` is invoked directly with `EVOLVE_RESUME_MODE=1` (test
harnesses, manual recovery) bypassing `resume-cycle.sh`.

**Operator escape hatch:** `EVOLVE_QUARANTINE_PRIOR_ATTEMPT=0` disables
quarantine. Use sparingly — typically for white-box debugging of a
specific resume failure where you want the resumed orchestrator to see
the killed attempt's state directly.

## Same-cycle ledger entry filter

`scripts/dispatch/run-cycle.sh:build_context()` constructs the
`recentLedgerEntries` digest. On resume, attempts 1..K-1 of the SAME
cycle would otherwise pollute attempt-K's prompt with stale-attempt
entries. The filter:

```bash
ledger_tail=$(jq -c --argjson cur "$cycle" \
    'select(.cycle != $cur)' "$LEDGER" 2>/dev/null | tail -5)
```

Cross-cycle entries (the compound-improvement digest) survive intact.
Fallback to plain `tail -5` if jq is unavailable or `$cycle` non-numeric.

## Per-cycle release (`cycle-release.sh`)

`scripts/lifecycle/cycle-release.sh` is the canonical per-cycle release
script. Called from `run-cycle.sh`'s `cleanup()` trap on non-checkpointed
terminal exits; also callable standalone by an operator for manual
release after an emergency abort.

Lifecycle (in order):

1. **Skip if checkpointed.** A checkpoint means the cycle is paused, not
   done. Defense-in-depth — `run-cycle.sh` already has this check.
2. **Remove worktree.** From `cycle-state.active_worktree`, unless
   `EVOLVE_KEEP_WORKTREE=1` or the dir is gone. Tries
   `git worktree remove --force` first, `rm -rf` fallback for orphans.
3. **Keep workspace dir.** `.evolve/runs/cycle-N/` is preserved for
   forensics — no auto-archive (per plan's "Moderate scope" decision).
4. **Append `role:release` ledger entry.** Maintains v8.37.0 SHA hash-chain
   (`prev_hash` + `entry_seq` + `.evolve/ledger.tip` atomic update).
   This is the auditable terminal marker — operators reading the ledger
   can now distinguish "cycle 79 completed cleanly" from "cycle 79's last
   entry was auditor, orchestrator hung, SIGKILLed."
5. **Clear `cycle-state.json`.** Idempotent — if `run-cycle.sh`'s cleanup
   already cleared it, this is a no-op.

The script is invoked with `bash cycle-release.sh <cycle> <run_exit_code>`
so the ledger entry records what triggered the release.

## Verification

The contract is enforced by
`scripts/tests/orchestrator-isolation-test.sh` — a profile-static check
suite covering all 8 assertions:

| # | Test | Enforcement layer |
|---|------|-------------------|
| 1 | Profile + jq prerequisites | meta |
| 2 | `.evolve/ledger.jsonl` reads denied | Layer 1 (`disallowed_tools` Read + Bash) |
| 3 | `.attempt-*` reads denied | Layer 1 (`disallowed_tools` Read + Bash) |
| 4 | `.attempt-*` writes denied | Layer 2 (`sandbox.deny_subpaths`) |
| 5 | Same-cycle ledger filter | `run-cycle.sh:build_context()` |
| 6 | `resume-cycle.sh` quarantine | `resume-cycle.sh` Step 2c |
| 7a | `cycle-release.sh` exists and executable | `scripts/lifecycle/` |
| 7b | `run-cycle.sh` invokes `cycle-release.sh` | `cleanup()` trap wiring |

Run `bash scripts/tests/orchestrator-isolation-test.sh` after any change
to the orchestrator profile, the resume path, or the ledger digest
pipeline.

## What this is NOT

- **NOT a budget cap.** The cycle-79 incident burned $78 not because the
  orchestrator was unbounded but because it was re-processing dead state.
  Budget caps are orthogonal (see `EVOLVE_MAX_BUDGET_USD` in `CLAUDE.md`).
- **NOT a fix for the orchestrator hanging in the first place.** Cycle 79
  attempt 1's stall-inactivity was a different bug (watchdog interaction).
  This ADR prevents the *amplification* of cost on each resume.
- **NOT a compound-improvement break.** The pre-digested `recentFailures`,
  `instinctSummary`, and filtered `recentLedgerEntries` injections
  preserve cross-cycle learning. We sever raw-history reads, not the
  digest.

## References

- Plan: `~/.claude/plans/linked-meandering-lobster.md`
- Profile: `.evolve/profiles/orchestrator.json`
- Verification: `scripts/tests/orchestrator-isolation-test.sh`
- Related ADRs: [checkpoint-resume.md](checkpoint-resume.md), [auto-resume.md](auto-resume.md), [sequential-write-discipline.md](sequential-write-discipline.md)
