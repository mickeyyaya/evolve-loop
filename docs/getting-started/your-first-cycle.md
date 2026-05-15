# Your First Cycle — A Hands-On Walkthrough

> Run evolve-loop end-to-end on your own machine. Follow this top-to-bottom: ~15 minutes wall clock, ~$0.50–1.50 budget. By the end you'll have shipped one commit, inspected every phase's output, and read your first audit verdict.
> Audience: people who've read [overview.md](../concepts/overview.md) and want to actually try it.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Step 1 — Install the Plugin](#step-1--install-the-plugin)
3. [Step 2 — Verify Your Setup](#step-2--verify-your-setup)
4. [Step 3 — (Optional) Choose Your LLM Routing](#step-3--optional-choose-your-llm-routing)
5. [Step 4 — Run One Cycle](#step-4--run-one-cycle)
6. [Step 5 — Watch It Work (Without Polling)](#step-5--watch-it-work-without-polling)
7. [Step 6 — Read the Verdict](#step-6--read-the-verdict)
8. [Step 7 — Inspect the Phase Artifacts](#step-7--inspect-the-phase-artifacts)
9. [Step 8 — Verify the Ledger](#step-8--verify-the-ledger)
10. [What Happens Next: Cycle 2](#what-happens-next-cycle-2)
11. [Common First-Time Issues](#common-first-time-issues)

---

## Prerequisites

| Requirement | Minimum | Why |
|---|---|---|
| Claude Code CLI | v2.1.139+ | The `claude -p` non-interactive mode + Agent View; older versions work but lack `/goal` reference points |
| macOS or Linux | macOS 12+ / glibc 2.31+ | `sandbox-exec` (macOS) or `bwrap` (Linux); WSL2 works |
| bash | 3.2+ (default macOS) | Many scripts target the macOS-default bash; no bash-4-isms required |
| git | 2.5+ | Per-cycle worktrees need `git worktree add` |
| `jq` | 1.6+ | Every state.json + ledger operation |
| Anthropic auth | Subscription via `~/.claude.json` OR `ANTHROPIC_API_KEY` | Subscription auth is first-class for `/evolve-loop`; API key is also supported |
| (optional) Gemini CLI | v0.42+ | Only if you want Gemini-routed phases |
| (optional) Codex CLI | any | Only if you want Codex-routed phases (hybrid mode) |
| Free disk | ~200 MB | Per-cycle worktrees + workspace artifacts |

Verify each:

```bash
claude --version       # Expect 2.1.139 or newer
bash --version         # Expect 3.2+
git --version          # Expect 2.5+
jq --version           # Expect 1.6+
echo $ANTHROPIC_API_KEY  # Either set, or use subscription auth via ~/.claude.json
```

---

## Step 1 — Install the Plugin

In your Claude Code session:

```
/plugin marketplace add danleemh/evolve-loop
/plugin install evolve-loop
```

Or for a project-local install (recommended for trying it out):

```bash
cd /your/project
git clone https://github.com/danleemh/evolve-loop.git .evolve/plugin
```

Then in Claude Code: `/plugin reload`.

Verify install:

```bash
ls .evolve/plugin/.claude-plugin/plugin.json
# Expect: file exists with version 10.7.0+
jq '.version' .evolve/plugin/.claude-plugin/plugin.json
```

---

## Step 2 — Verify Your Setup

Before running a cycle, sanity-check that the kernel hooks are wired:

```bash
ls .claude/settings.json   # Expect: hooks block referencing scripts/guards/*.sh
bash scripts/utility/release.sh
# Expect: PASSED: All version references are consistent.

bash scripts/dispatch/detect-cli.sh
# Expect: claude (or gemini/codex if those are your default)

ls scripts/guards/
# Expect: phase-gate-precondition.sh, role-gate.sh, ship-gate.sh, ...
```

If any of those fail, see [Common First-Time Issues](#common-first-time-issues).

---

## Step 3 — (Optional) Choose Your LLM Routing

By default, evolve-loop runs every phase on Claude Sonnet. You can override per-phase via `.evolve/llm_config.json` (gitignored — operator-local).

Skip this step on your first cycle. Try the default first.

If you want to try a mixed setup later:

```bash
mkdir -p .evolve
cat > .evolve/llm_config.json << 'EOF'
{
  "schema_version": 1,
  "phases": {
    "scout":   {"provider": "google",    "cli": "gemini", "model": "gemini-3.1-pro-preview"},
    "builder": {"provider": "anthropic", "cli": "claude", "model_tier": "sonnet"},
    "auditor": {"provider": "anthropic", "cli": "claude", "model": "claude-opus-4-7"}
  },
  "_fallback": {"cli": "claude", "model_tier": "sonnet"}
}
EOF
```

This routes Scout to Gemini 3.1 Pro Preview, Builder to Claude Sonnet, Auditor to Claude Opus (different family from Builder — adversarial-mode default). See [pluggability.md](../concepts/pluggability.md) for more configs.

---

## Step 4 — Run One Cycle

Pick a small, contained goal for your first cycle. Avoid sweeping refactors. Good examples:

| Good first goals | Why |
|---|---|
| "Add a `--dry-run` flag to `scripts/foo.sh`" | Single file, clear acceptance |
| "Document the `bar()` function in `lib/baz.py`" | Doc-only; no test infra needed |
| "Fix the typo in README.md line 42" | Trivial; verifies the pipeline runs |
| "Add unit tests for the `parseConfig()` function" | Small but real |

| Bad first goals | Why |
|---|---|
| "Refactor the database layer" | Huge scope; cycle will get stuck in Triage |
| "Make the app faster" | Vague; intent phase will reject |
| "Update all dependencies" | Touches many files; high risk |
| "Improve security" | No measurable acceptance |

Run:

```bash
bash scripts/dispatch/evolve-loop-dispatch.sh --cycles 1 --budget-usd 3 \
  "Add a --dry-run flag to scripts/foo.sh that prints the planned operation without executing it."
```

Or, if you're inside Claude Code:

```
/evolve-loop --cycles 1 --budget-usd 3 "Add a --dry-run flag to scripts/foo.sh..."
```

The dispatcher launches the orchestrator subprocess. You'll see streaming output for ~10-20 minutes.

---

## Step 5 — Watch It Work (Without Polling)

The dispatcher logs to stdout. While it runs:

| What to watch | Why |
|---|---|
| `[phase-watchdog] phase advance: 'X' → 'Y'` | Tracks pipeline progression |
| `[claude-adapter]` or `[gemini-adapter]` lines | Which CLI is dispatching this phase |
| `[subagent-run] cli_resolution: ...` | The router decision |
| Watchdog stalls | If a phase idle >180s, you'll see WARN |

**Do not poll** by re-running commands in a tight loop. Each poll burns prompt tokens. Either:
- Wait passively (the dispatcher prints natural progress)
- Open a second terminal and `tail -f .evolve/runs/cycle-N/*.log` 
- Open the Claude Code Agent View (UI) for visual monitoring

Cycle artifacts appear in `.evolve/runs/cycle-N/` as each phase completes:

```bash
ls -lt .evolve/runs/cycle-N/
# scout-report.md      (after Scout)
# triage-decision.md   (after Triage)
# build-report.md      (after Build)
# audit-report.md      (after Audit)
# acs-verdict.json     (after Audit)
# orchestrator-report.md  (at cycle end)
# carryover-todos.json (PASS) OR retrospective-report.md (FAIL/WARN)
```

---

## Step 6 — Read the Verdict

When the dispatcher exits, check three things in order:

### A — Exit code

```bash
echo $?
```

| Exit code | Meaning |
|---|---|
| `0` | All cycles shipped successfully |
| `2` | INTEGRITY-BREACH — investigate before re-running |
| `3` | DONE-WITH-RECOVERABLE-FAILURES — review failedApproaches |
| `4` | BATCH-BUDGET-EXHAUSTED |

### B — Audit verdict

```bash
cat .evolve/runs/cycle-N/audit-report.md | head -20
# Look for: ## Verdict\n**PASS**  or  **FAIL**
```

### C — ACS predicate verdict (the authoritative one per EGPS v10)

```bash
jq '{verdict, green_count, red_count, total_predicates}' .evolve/runs/cycle-N/acs-verdict.json
```

```json
{
  "verdict": "PASS",
  "green_count": 47,
  "red_count": 0,
  "total_predicates": 47
}
```

`verdict: PASS` with `red_count: 0` is what triggers ship-gate to allow the commit. Any RED predicate fails the cycle deterministically.

### D — Git log

```bash
git log --oneline -3
# Latest commit: feat: cycle N — <task summary> --- ## Actual diff ...
```

---

## Step 7 — Inspect the Phase Artifacts

This is where you learn what each agent actually did. Read in pipeline order:

### `scout-report.md`

Look for:
- `## Discovery Summary` — what scout saw in your repo
- `## Key Findings` — facts grounded in `git status` / `git diff` (post-cycle-62 grounding check)
- `## Selected Tasks` — what scout proposed
- `## Carryover Decisions` — any items deferred to the next cycle

### `triage-decision.md`

Look for:
- `## top_n` — what triage allowed this cycle to attempt
- `## deferred` — items pushed to next cycle
- `## dropped` — items rejected entirely

### `build-report.md`

Look for:
- `## Files Changed/Staged` — the diff Builder produced
- `## AC Claims` — Builder's claim that each acceptance criterion is met
- `## Self-Verification` — Builder's pre-audit check

### `audit-report.md`

Look for:
- `## Verdict` — PASS / WARN / FAIL
- `## Evidence Summary` — per-AC verification with `path:line` citations
- `## Defects Found` — any RED findings
- `## Observations` — non-blocking notes

### `orchestrator-report.md`

Look for:
- `## Phase Outcomes` — the per-phase table
- `## CLI Resolution` — auto-rendered from ledger; shows which CLI/model actually ran each phase
- `## Verdict` — the orchestrator's narrative verdict (SHIPPED / WARN / FAILED-AND-LEARNED)

### `acs/cycle-N/*.sh`

These are the **predicates** — the actual exit-code-based verdicts. Open one:

```bash
cat acs/cycle-N/001-*.sh
```

Each predicate has a metadata header, an explicit acceptance criterion, and a bash test that returns 0 (GREEN) or non-zero (RED). After successful ship, these get promoted to `acs/regression-suite/cycle-N/` and run in every future cycle's audit.

---

## Step 8 — Verify the Ledger

The ledger is the tamper-evident audit trail. Every phase writes one entry:

```bash
grep -F '"cycle":N' .evolve/ledger.jsonl | jq -c '{role, kind, model, exit_code, artifact_sha256}'
```

Each entry's `prev_hash` chains to the previous one. Verify the chain is intact:

```bash
bash scripts/observability/verify-ledger-chain.sh
# Expect: ledger chain verified, N entries
```

This is what makes evolve-loop "tamper-evident" — modifying any past entry invalidates every subsequent `prev_hash`.

---

## What Happens Next: Cycle 2

Run another cycle:

```bash
bash scripts/dispatch/evolve-loop-dispatch.sh --cycles 1 --budget-usd 3
```

Note: no goal argument. The orchestrator picks from `state.json:carryoverTodos[]` (if cycle 1 left any) and from `state.json:instinctSummary[]` (lessons learned so far).

This is the self-evolving property in action: cycle 2's Scout reads cycle 1's lessons. If cycle 1 audit FAIL'd, cycle 2 has a `retrospective-report.md` lesson YAML to consult.

For deeper understanding of cross-cycle learning, see [self-evolution.md](../concepts/self-evolution.md).

---

## Common First-Time Issues

| Symptom | Likely cause | Fix |
|---|---|---|
| `ship-gate DENY` on a manual git command | Hook is enforcing — direct git commit forbidden | Use `bash scripts/lifecycle/ship.sh --class manual "<msg>"` |
| `claude binary not found` | Claude Code CLI not in PATH | `which claude` to verify; install per claude.com/code |
| `sandbox-exec: Operation not permitted` | Nested-Claude environment (running `/evolve-loop` from inside Claude Code) | Auto-detected; check `.evolve/environment.json:auto_config.inner_sandbox=false` is set |
| `INTEGRITY-FAIL: expected_ship_sha mismatch` | Out-of-date pin after a ship.sh update | Delete `.evolve/state.json:expected_ship_sha` and re-run; v8.32+ auto-rotates |
| Cycle stuck in `calibrate` for >2 min | Orchestrator subprocess slow to start | Check `pgrep -fl claude` to confirm subprocess running; wait or kill + retry |
| `state.json:lastCycleNumber` not advancing | Worktree-state-not-syncing (B7) | Fixed in v10.7.0+; if older, run `jq '.lastCycleNumber += 1 \| .' state.json > tmp && mv tmp state.json` |
| Audit FAIL but you think the code is correct | EGPS predicates are stricter than prose verdicts; read `audit-report.md` for cited `path:line` evidence | Trust the predicates; adjust the code or refine the predicate definition |
| Memo phase API 529 | Anthropic rate-limit during memo | Classified as `infrastructure` (recoverable); next run retries |
| `role-gate DENY: phase=retrospective ...` | Stuck cycle-state from prior failed run | `bash scripts/lifecycle/cycle-state.sh clear` |
| `BATCH-BUDGET CRITICAL: cumulative ... >= 95%` | About to exhaust cost cap | Either: increase `--budget-usd`, OR let next cycle checkpoint via v9.1.0 mechanism |

---

## Reading the Failure Mode (If Cycle FAIL'd)

If your first cycle returned audit FAIL:

1. **It's normal.** The cycle 61 incident (preserved in `docs/incidents/cycle-61.md`) is a worked example — 7 bugs caught by the framework's own audit.
2. **Read the lesson YAML.** `.evolve/instincts/lessons/cycle-N-*.yaml` shows what the retrospective learned. The next cycle's Scout will see these.
3. **Re-run.** Cycle N+1 will read the lesson and (likely) succeed at the same task with a different approach.
4. **If it FAILs the same way twice**, that's a real signal — read [error-recovery.md](../concepts/error-recovery.md) and consider whether the goal needs decomposition.

The framework's value proposition is **not "every cycle succeeds"** — it's **"failures produce durable lessons that improve future cycles."**

Welcome to the loop.

---

## What to Read Next

- [Why evolve-loop is self-evolving](../concepts/self-evolution.md) — the mechanism of cross-cycle learning
- [How LLMs are prevented from gaming the verdict](../concepts/trust-architecture.md) — the 3-tier enforcement
- [Per-phase mechanics deep-dive](../architecture/phase-architecture.md) — what each phase does in detail
- [Comparison with /goal and other long-running skills](../comparisons/long-running-claude-skills.md) — when to use what
