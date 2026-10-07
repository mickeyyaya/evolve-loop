# Your First Cycle — A Hands-On Walkthrough

> Run evolve-loop end-to-end on your own machine. Follow this document from top to bottom: ~15 minutes wall clock, ~$0.50–1.50 budget.
> At the end, you will have one shipped commit, and you will know the output of every phase. You will also have read your first audit verdict.
> Audience: people who read [overview.md](../concepts/overview.md) and want to actually try it.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Step 1 — Install the Plugin](#step-1--install)
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
| Claude Code CLI | v2.1.139+ | The `claude -p` non-interactive mode + Agent View. Older versions work, but they do not have the `/goal` reference points |
| macOS or Linux | macOS 12+ / glibc 2.31+ | `sandbox-exec` (macOS) or `bwrap` (Linux); WSL2 works |
| bash | 3.2+ (default macOS) | Many scripts target the default bash of macOS; no bash-4-isms required |
| git | 2.5+ | Per-cycle worktrees need `git worktree add` |
| `jq` | 1.6+ | Every state.json + ledger operation |
| Anthropic auth | Subscription through `~/.claude.json` OR `ANTHROPIC_API_KEY` | Subscription auth is first-class for `/evo:loop`. The API key is also supported |
| (optional) Gemini CLI | v0.42+ | Only if you want Gemini-routed phases |
| (optional) Codex CLI | any | Only if you want Codex-routed phases (hybrid mode) |
| Free disk | ~200 MB | Per-cycle worktrees + workspace artifacts |

Verify each one:

```bash
claude --version       # Expect 2.1.139 or newer
bash --version         # Expect 3.2+
git --version          # Expect 2.5+
jq --version           # Expect 1.6+
echo $ANTHROPIC_API_KEY  # Either set, or use subscription auth via ~/.claude.json
```

---

## Step 1 — Install

The fastest path is one line. It detects your platform and gets the `evolve` binary
(prebuilt, or, as a fallback, built from source). It automatically installs the deps that are not there.
Then it installs evolve for the CLI(s) that you have:

```bash
curl -fsSL https://mickeyyaya.github.io/evolve-loop/install.sh | sh
```

Or, inside Claude Code, add the plugin directly:

```
/plugin marketplace add mickeyyaya/evolve-loop
/plugin install evo
```

**On Windows:** the loop runtime is Unix-based (tmux, bash), so run it under
[WSL2](https://learn.microsoft.com/windows/wsl/install). Install WSL, open your
WSL shell (for example, Ubuntu), then run the one-liner above there. Inside WSL, it
installs exactly as on Linux. The `/evo:*` skills install natively in Claude
Code on Windows, with the `/plugin` commands above. Only the loop runtime needs WSL.

Or, for a project-local install (recommended when you want to try it out):

```bash
cd /your/project
git clone https://github.com/mickeyyaya/evolve-loop.git .evolve/plugin
```

Then, in Claude Code: `/plugin reload`.

Verify the install:

```bash
ls .evolve/plugin/.claude-plugin/plugin.json
# Expect: file exists with version 10.7.0+
jq '.version' .evolve/plugin/.claude-plugin/plugin.json
```

---

## Step 2 — Verify Your Setup

Before you run a cycle, do a sanity check that the kernel hooks are connected:

```bash
ls .claude/settings.json   # Expect: hooks block referencing legacy/scripts/guards/*.sh
bash legacy/scripts/utility/release.sh
# Expect: PASSED: All version references are consistent.

bash legacy/scripts/dispatch/detect-cli.sh
# Expect: claude (or gemini/codex if those are your default)

ls legacy/scripts/guards/
# Expect: phase-gate-precondition.sh, role-gate.sh, ship-gate.sh, ...
```

If one of these checks fails, see [Common First-Time Issues](#common-first-time-issues).

---

## Step 3 — (Optional) Choose Your LLM Routing

By default, evolve-loop runs each phase on the model that its **profile** declares. The simplest way to change that is the one-question setup flow:

```
/evo:setup
```

It automatically detects which CLIs/subscriptions you have, and it explains the pipeline.
Then it offers **three ready-made presets**. Each preset is a complete per-phase model plan that the binary calculates from your profiles:

| Preset | What it does |
|---|---|
| **Recommended** | Profile defaults, subscription-aware. When you have two model families, Builder and Auditor run on different families (adversarial integrity). |
| **Economy** | One tier cheaper per phase, where the envelope allows it. It uses less quota and costs less. |
| **Max-quality** | The top of the envelope of each phase. Best quality, highest cost. |

You select **one**. The binary writes per-phase pins to `.evolve/policy.json` (only where they are different from the profile default). Skip this step on your first cycle, because the defaults work.

Do you prefer the command line, or do you want to script it?

```bash
evolve setup recommend                 # show the three presets for your machine
evolve setup apply --preset economy    # write the chosen preset's pins to .evolve/policy.json
evolve setup apply --preset recommended --dry-run   # preview the merged policy, write nothing
```

The presets themselves are public config (`go/internal/setup/presets.json`). You can override them per repo through `.evolve/setup-presets.json`.
For the full mechanism, see [setup-onboarding.md](../architecture/setup-onboarding.md). For more routing configs, see [pluggability.md](../concepts/pluggability.md).

---

## Step 4 — Run One Cycle

Select a small, contained goal for your first cycle. Do not use broad refactors. Good examples:

| Good first goals | Why |
|---|---|
| "Add a `--dry-run` flag to `legacy/scripts/foo.sh`" | Single file, clear acceptance |
| "Document the `bar()` function in `lib/baz.py`" | Doc-only; no test infra needed |
| "Fix the typo in README.md line 42" | Trivial; verifies that the pipeline runs |
| "Add unit tests for the `parseConfig()` function" | Small but real |

| Bad first goals | Why |
|---|---|
| "Refactor the database layer" | Huge scope; the cycle will get stuck in Triage |
| "Make the app faster" | Vague; the intent phase will reject it |
| "Update all dependencies" | Touches many files; high risk |
| "Improve security" | No measurable acceptance |

Run:

```bash
bash archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh --cycles 1 --budget-usd 3 \
  "Add a --dry-run flag to legacy/scripts/foo.sh that prints the planned operation without executing it."
```

Or, if you are inside Claude Code:

```
/evo:loop --cycles 1 --budget-usd 3 "Add a --dry-run flag to legacy/scripts/foo.sh..."
```

The dispatcher starts the orchestrator subprocess. You will see streaming output for ~10-20 minutes.

---

## Step 5 — Watch It Work (Without Polling)

The dispatcher logs to stdout. While it runs, watch these lines:

| What to watch | Why |
|---|---|
| `[phase-watchdog] phase advance: 'X' → 'Y'` | Tracks the progress of the pipeline |
| `[claude-adapter]` or `[gemini-adapter]` lines | Which CLI dispatches this phase |
| `[subagent-run] cli_resolution: ...` | The router decision |
| Watchdog stalls | If a phase is idle for >180s, you will see a WARN |

**Do not poll**: do not run commands again and again in a tight loop. Each poll burns prompt tokens. Do one of these:
- Wait passively (the dispatcher prints natural progress).
- Open a second terminal and run `tail -f .evolve/runs/cycle-N/*.log`.
- Open the Claude Code Agent View (UI) for visual monitoring.

The cycle artifacts appear in `.evolve/runs/cycle-N/` as each phase completes:

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

When the dispatcher exits, check three things in this order:

### A — Exit code

```bash
echo $?
```

| Exit code | Meaning |
|---|---|
| `0` | All cycles shipped successfully |
| `2` | INTEGRITY-BREACH — examine the cause before you run again |
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

`verdict: PASS` with `red_count: 0` triggers ship-gate to allow the commit. Any RED predicate fails the cycle deterministically.

### D — Git log

```bash
git log --oneline -3
# Latest commit: feat: cycle N — <task summary> --- ## Actual diff ...
```

---

## Step 7 — Inspect the Phase Artifacts

Here, you learn what each agent actually did. Read the artifacts in pipeline order:

### `scout-report.md`

Look for:
- `## Discovery Summary` — what scout saw in your repo
- `## Key Findings` — facts grounded in `git status` / `git diff` (the post-cycle-62 grounding check)
- `## Selected Tasks` — what scout proposed
- `## Carryover Decisions` — the items deferred to the next cycle

### `triage-decision.md`

Look for:
- `## top_n` — what triage let this cycle try
- `## deferred` — items pushed to the next cycle
- `## dropped` — items rejected entirely

### `build-report.md`

Look for:
- `## Files Changed/Staged` — the diff that Builder produced
- `## AC Claims` — the claim of Builder that each acceptance criterion is met
- `## Self-Verification` — the pre-audit check of Builder

### `audit-report.md`

Look for:
- `## Verdict` — PASS / WARN / FAIL
- `## Evidence Summary` — per-AC verification with `path:line` citations
- `## Defects Found` — the RED findings
- `## Observations` — non-blocking notes

### `orchestrator-report.md`

Look for:
- `## Phase Outcomes` — the per-phase table
- `## CLI Resolution` — auto-rendered from the ledger. It shows which CLI/model actually ran each phase
- `## Verdict` — the narrative verdict of the orchestrator (SHIPPED / WARN / FAILED-AND-LEARNED)

### `acs/cycle-N/*.sh`

These are the **predicates**: the actual verdicts, based on exit codes. Open one:

```bash
cat acs/cycle-N/001-*.sh
```

Each predicate has a metadata header, an explicit acceptance criterion, and a bash test. The test returns 0 (GREEN) or non-zero (RED).
After a successful ship, the pipeline promotes these predicates to `acs/regression-suite/cycle-N/`. They then run in the audit of every future cycle.

---

## Step 8 — Verify the Ledger

The ledger is the tamper-evident audit trail. Every phase writes one entry:

```bash
grep -F '"cycle":N' .evolve/ledger.jsonl | jq -c '{role, kind, model, exit_code, artifact_sha256}'
```

The `prev_hash` of each entry chains to the previous entry. Verify that the chain is intact:

```bash
bash legacy/scripts/observability/verify-ledger-chain.sh
# Expect: ledger chain verified, N entries
```

This makes evolve-loop "tamper-evident": a change to any past entry invalidates every later `prev_hash`.

---

## What Happens Next: Cycle 2

Run another cycle:

```bash
bash archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh --cycles 1 --budget-usd 3
```

Note: there is no goal argument. The orchestrator selects from `state.json:carryoverTodos[]` (if cycle 1 left any) and from `state.json:instinctSummary[]` (the lessons learned until now).

This is the self-evolving property in action: the Scout of cycle 2 reads the lessons of cycle 1. If the cycle 1 audit FAIL'd, cycle 2 has a `retrospective-report.md` lesson YAML to consult.

For more about cross-cycle learning, see [self-evolution.md](../concepts/self-evolution.md).

---

## Common First-Time Issues

| Symptom | Likely cause | Fix |
|---|---|---|
| `ship-gate DENY` on a manual git command | The hook enforces its rule: a direct git commit is forbidden | Use `bash legacy/scripts/lifecycle/ship.sh --class manual "<msg>"` |
| `claude binary not found` | The Claude Code CLI is not in the PATH | Run `which claude` to verify. Install it as claude.com/code tells |
| `sandbox-exec: Operation not permitted` | Nested-Claude environment (you run `/evo:loop` from inside Claude Code) | Auto-detected. Check that `.evolve/environment.json:auto_config.inner_sandbox=false` is set |
| `INTEGRITY-FAIL: expected_ship_sha mismatch` | Out-of-date pin after a ship.sh update | Delete `.evolve/state.json:expected_ship_sha` and run again. v8.32+ auto-rotates it |
| Cycle stuck in `calibrate` for >2 min | The orchestrator subprocess is slow to start | Run `pgrep -fl claude` to confirm that the subprocess runs. Wait, or kill it and try again |
| `state.json:lastCycleNumber` does not advance | Worktree-state-not-syncing (B7) | Fixed in v10.7.0+. On an older version, run `jq '.lastCycleNumber += 1 \| .' state.json > tmp && mv tmp state.json` |
| Audit FAIL, but you think that the code is correct | EGPS predicates are stricter than prose verdicts. Read `audit-report.md` for the cited `path:line` evidence | Trust the predicates. Adjust the code, or refine the predicate definition |
| Memo phase API 529 | Anthropic rate limit during memo | Classified as `infrastructure` (recoverable). The next run tries again |
| `role-gate DENY: phase=retrospective ...` | A stuck cycle-state from an earlier failed run | `bash legacy/scripts/lifecycle/cycle-state.sh clear` |
| `BATCH-BUDGET CRITICAL: cumulative ... >= 95%` | The cost cap is almost exhausted | Increase `--budget-usd`, OR let the next cycle checkpoint with the v9.1.0 mechanism |

---

## Reading the Failure Mode (If Cycle FAIL'd)

If your first cycle returned an audit FAIL:

1. **It is normal.** The cycle 61 incident (kept in `docs/incidents/cycle-61.md`) is a worked example: the audit of the framework caught 7 bugs.
2. **Read the lesson YAML.** `.evolve/instincts/lessons/cycle-N-*.yaml` shows what the retrospective learned. The Scout of the next cycle will see these lessons.
3. **Run again.** Cycle N+1 will read the lesson. It will (likely) succeed at the same task with a different approach.
4. **If it FAILs the same way two times**, that is a real signal. Read [error-recovery.md](../concepts/error-recovery.md). Then decide if the goal needs decomposition.

The value proposition of the framework is **not "every cycle succeeds"**. It is **"failures produce durable lessons that improve future cycles."**

Welcome to the loop.

---

## What to Read Next

- [Why evolve-loop is self-evolving](../concepts/self-evolution.md) — the mechanism of cross-cycle learning
- [How LLMs are prevented from gaming the verdict](../concepts/trust-architecture.md) — the 3-tier enforcement
- [Per-phase mechanics deep-dive](../architecture/phase-architecture.md) — what each phase does, in detail
- [Comparison with /goal and other long-running skills](../comparisons/long-running-claude-skills.md) — when to use what
