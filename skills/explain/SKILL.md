---
name: explain
description: Use when the user asks to explain how a feature/subsystem works and how the loops built it, or what the concurrent loops/lanes/cycles did and why. Default output = per-FEATURE HTML explainer pages (two movements — how it works today, then how it got built, with verified file:line claims and drift callouts) under docs/explain/; per-batch/per-lane story pages are the fallback framing. Derived from geoffreylitt/explain-diff-html, customized for evolve-loop.
argument-hint: "[cycle range | batch date | lane task-ids]"
---

# /evo:explain — per-feature HTML explainer

Produce a rich, interactive HTML explanation of **how a feature/subsystem works and how the loops collectively built it**. The default unit of explanation is the **feature** (campaign/subsystem — e.g. token-telemetry, fleet-width, queue-integrity), aggregating ALL its contributing cycles across batches: one feature = many lanes/cycles = the complete idea. Per-lane or per-batch framing is the fallback when the user asks for a specific cycle range or batch story.

Each feature page has two movements: first **how it works today** (architecture, data flow end to end, with diagrams and real example data), then **how it got built** (the contributing cycles in order, what each added, which incident or root cause motivated it, including instructive failures). Output goes to `docs/explain/<feature-slug>.html` plus a shared `docs/explain/index.html` (feature table + contributing-cycle map + the deep pipeline background, written once). Repo writes land ONLY at batch/wave-safe boundaries via the gated commit path — stage in the session scratchpad until the landing window.

## Step 1 — Gather per-lane data (this repo's sources)

- **Lane ↔ cycle mapping**: the newest loop log (`.evolve/loop-resume-*.log`) prefixes every line with `[<task-id>]`; worktree strings `cycle-<lane-hash>-N` inside those lines give cycle numbers. One task may map to several cycles.
- **Verdicts**: `"FinalVerdict"` lines in the loop log — dedup by distinct cycle, NOT by line count (verdict JSON re-flushes on lane teardown). Wave summaries: `[loop] wave N: K/M lanes ok`.
- **What shipped**: `git log --since=<batch start> --pretty='%h|%ad|%s'`; cycle ships carry the generic message `evolve-cycle: goal=<hash>`, so substance lives in `git show --stat <sha>` — map commits to lanes by timestamp + touched files.
- **Why the task existed**: the inbox item JSON (`.evolve/inbox/*.json`, or its consumed copy via git history) — `summary`, `acceptance`, `connects_to`; plus cycle dossiers `knowledge-base/cycles/cycle-N.json` (`final_verdict`, `defects`, `phases`).
- **Failures**: lane-tagged log context around `cycle level failure`, `ship error`, `exhausted after`; classify each as task defect, infrastructure, contention, or verdict-poisoning (e.g. a post-ship retro timeout FAILing an otherwise-successful cycle).

## Step 2 — Page structure

All pages are self-contained HTML (inline CSS only, no external resources, responsive, light+dark via `prefers-color-scheme`).

**Default — feature pages (`docs/explain/<feature-slug>.html`), two movements:**

1. **How it works today** — architecture and data flow end to end, every claim carrying file:line evidence; one flow/board diagram with real example data; callouts for key concepts; `drift` callouts naming any queued docs-code-alignment inbox item found while verifying.
2. **How it got built** — contributing cycles in order (table or prose), what each added, which incident or root cause motivated it; instructive failures included with what they taught.

The shared deep background (spine, trust kernel, fleet, queue) lives ONCE in `index.html`, never repeated per page. The index also carries the feature table with one-line summaries and a reading order.

**Fallback — per-batch/per-lane story page (single scratchpad HTML, only when the user asks for a specific batch or cycle-range story):**

1. **Background (deep, skippable)** — what a cycle is: the fixed spine `intent → scout → triage → tdd → build-planner → build → audit → ship` (the literal `spine_order` of `docs/architecture/phase-registry.json`, whose trailing `end` entry is a graph sentinel, not a phase; `retrospective` and `memo` are OPTIONAL post-ship phases, not spine; the single-word phase vocabulary is CLOSED per `docs/architecture/micro-phase-catalog.md`), plus goal-type-conditional insertions around tdd/build (e.g. `test-amplification`, `secret-leak-scan`, `flake-rerun-scan`, `error-handling-scan`, `coverage-gate`, `adversarial-review` — from the registry's `goal_recipes`; NOT universal sequential phases). Then the trust kernel (audit verdict bound to tree SHA; only `evolve ship` commits), the fleet (`fleet.count`/`min_lanes`, waves, per-lane worktrees), the queue (weighted inbox + carryover todos, triage).
2. **Background (narrow)** — the specific batch timeline: width changes, incidents, wave ok-rates, rendered as a swimlane board.
3. **Intuition** — the shared-lab-notebook model: lanes rebase-and-re-audit on main contention (`Not possible to fast-forward` is queueing, not failure); control-plane files must be committed before wave dispatch. Include one flow diagram tracing a real task end-to-end: inbox item → triage → lane worktree → ship commit (real SHA) → consumed.
4. **Per-lane walkthrough (the core)** — group lanes by THEME (incident-driven fixes, queue integrity, campaign chains, hardening). Each lane card: task-id, PASS/FAIL chips, cycles + commit SHAs + files touched, **What** (one paragraph), **Why** (the root cause, incident, or campaign that motivated it). Include instructive FAILURES as first-class cards — a repeated FAIL often exposes a lost dependency or a recurring blocker; say what it taught.
5. **Quiz** — five medium-difficulty interactive multiple-choice questions (click → correct/incorrect + explanation) testing the batch's mechanics, not trivia.

## Step 3 — Format rules

- Reused diagram families: pipeline strip · swimlane wave board · node-arrow flow diagram. HTML/CSS only — never ASCII art. Put example data in diagrams.
- Code blocks in `<pre>`; any custom-styled block MUST carry `white-space: pre` or `pre-wrap`. Verify each before saving.
- Callouts for key concepts (trust kernel, why width isn't free, the self-repair meta-pattern).
- Clear, flowing prose (Kleppmann register); smooth transitions between sections.
- Output location — ONE rule, two stages. Stage 1 (always): DRAFT in the session scratchpad as `YYYY-MM-DD-explanation-<slug>.html` and deliver via the platform's file-send mechanism (inline render) for immediate review. Stage 2 (only for the maintained set): copy `docs/explain/<feature-slug>.html` + `index.html` into the repo and land via the gated commit path (reviewers → commit-gate → `evolve ship --class manual`) at a wave/batch boundary. NEVER have dirty tracked paths in the main tree while lanes are running — not writes, not staged files, not pre-existing dirt: the tree-diff guard treats ANY non-allowlisted tracked-path dirt as a worktree leak and fails lanes (cycle 758 died on a mid-wave write; wave 7 went 0/3 on dirt that pre-dated the wave). Only `.evolve/inbox/` churn is allowlisted. The complete landing (write + add + gate + ship) must fit inside one inter-wave gap; if it cannot, `git stash push -- <paths>` until the next gap.

## Cautions

- Never read whole run dirs (`.evolve/runs/cycle-N/`) into context — they contain full LLM transcripts; take targeted greps only.
- A lane appearing across many waves may be: a flake retry (same work), a follow-up slice (new work), or a re-pick bug — distinguish via inbox consumption state before claiming waste.
- Keep the explainer honest: report FAIL/abort counts and their causes with the same prominence as PASSes.
