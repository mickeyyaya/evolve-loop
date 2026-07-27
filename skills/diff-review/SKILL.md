---
name: diff-review
description: Budget-capped adversarial review + simplification pass over a STAGED DIFF only — one agent, two lenses, findings-first. Use for commit-gate reviews instead of dispatching separate whole-repo reviewer and simplifier agents; the caller declares what already ran (tests, lint, gates) and the reviewer verifies claims against the diff's one-hop neighborhood, never the whole tree.
---

# Diff Review — precise, budget-capped, two lenses in one pass

> Why this skill exists: two generic review agents (go-reviewer + code-simplifier)
> spent 197k + 136k tokens and 97 + 58 tool calls reviewing ONE staged diff
> (2026-07-27). Measured waste: re-deriving repo context the caller already had,
> re-running test suites the caller had already run, unbounded whole-tree
> consumer sweeps, and loading the same diff into two separate contexts. This
> skill keeps the review QUALITY disciplines (evidence-first, adversarial,
> file:line findings) and deletes the waste structurally.

## Scope contract (RIGID)

The unit of review is the STAGED DIFF: `git diff --cached`. Nothing else is in
scope unless a hunk drags it in.

The caller's prompt MUST declare, and you MUST trust without re-running:
- **Already verified**: which test suites/lints/gates ran and their results.
  Re-running any of them is budget theft — CI re-proves them anyway.
- **Intent**: what the change is supposed to do, in 1-3 sentences.
- **Known constraints**: pinned contracts, protected surfaces, prior review
  findings already applied.

If the caller declared nothing, spend ONE tool call asking the diff itself
(`git diff --cached --stat`) and proceed — never block on missing context.

## Budgets (RIGID)

- **≤ 20 tool calls total.** Count them. At 20, stop and report with what you have.
- **≤ 15 file reads**, and reads are TARGETED: `Read` with offset/limit around
  the hunk, never whole large files; the diff already shows the change.
- **One hop of coupling, found by grep.** For each changed/added exported
  symbol and each changed behavior (e.g. a return-shape change): ONE `grep -rn`
  for its call sites. Read only the call sites the grep surfaces. NEVER sweep
  docs/, .evolve/runs/, or knowledge-base/ unless a hunk edits them.
- **Run nothing the caller declared green.** You may run at most ONE cheap
  compile/probe (`go build ./pkg`, a 5-line script) to settle a finding you
  cannot settle by reading — and only for a finding you will report.

## The pass (both lenses, one sweep over the hunks)

Walk the diff hunk by hunk. For each, ask BOTH lenses before moving on:

**Adversarial lens** (a real defect a reviewer must block):
1. Inputs that break it — the failure scenario must be concrete (inputs → wrong
   behavior), not "could be a problem".
2. Contract drift — does a changed function still honor what its one-hop
   callers assume? (That's what the grep hop is for.)
3. Gate-weakening — for pipeline code: can an agent/author exploit this path to
   make a real failure invisible? Fail-open paths need a stated justification.
4. Wiring — a new seam/option/hook that nothing calls is dead code; say so.
5. Tests-vs-intent — do the new tests pin the claimed behavior, and would a
   degenerate implementation pass them? Name the missing negative case.

**Simplifier lens** (the change, made smaller/clearer, behavior identical):
6. Duplication vs one-hop neighbors — an existing helper the diff re-implements
   (your grep hop already surfaced the candidates; do not go hunting further).
7. Comment restatement — flag comments that repeat the code or repeat another
   comment in the SAME diff; keep incident context and constraint-visibility
   comments (they prevent bad future "simplifications").
8. Dead branches, ceremony, mis-signaling names — only within the diff.

## Output (RIGID)

Findings-first, ranked BLOCK / HIGH / MEDIUM / LOW / STYLE. Each finding:
`file:line` — one-sentence claim — concrete failure scenario or concrete
replacement. No essays. Then:

- **Clean**: one plain list of what you checked and found sound (so the caller
  knows it was covered, not skipped).
- **UNVERIFIED**: hypotheses you could not settle within budget — never
  silently drop them, never burn budget chasing them.
- **Budget line**: `tool_calls=N/20 reads=M/15` — the caller uses this to tune
  the skill.
- Verdict: BLOCK (any BLOCK/HIGH finding) or APPROVE.

## What this skill deliberately does NOT do

- Whole-repo consumer sweeps, doc reconciliation, archaeology in .evolve/runs.
- Re-running declared-green suites, vet, gofmt, apicover, durable tiers.
- Style opinions outside the diff's own lines.
- A second agent: both lenses run in THIS pass. The commit-gate attestation
  for a run of this skill is `--reviewers "code-review-simplify,code-review"`
  (both capabilities genuinely executed).

## Caller template

```
Agent(general-purpose or <lang>-reviewer, model per risk tier):
  Follow skills/diff-review/SKILL.md exactly.
  Diff: git diff --cached in <repo>.
  Intent: <1-3 sentences>.
  Already verified (do NOT re-run): <suites/lints/gates + results>.
  Constraints: <protected surfaces, pinned contracts, applied prior findings>.
```

Model choice stays a risk decision, not a cost decision: pipeline-integrity
diffs still get an opus-class reviewer (standing rule) — the budgets, not a
weaker model, are what cut the cost.
