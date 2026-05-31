# Incident: cycle-156 — builder commits worktree, auditor inspects `git diff HEAD` (empty) → FAIL → real work discarded

**Date:** 2026-05-31
**Severity:** HIGH (every cycle with a committing builder ships nothing; surfaced by agy/Gemini as builder)
**Run:** multi-CLI validation (cycles 156–160), builder=`agy-tmux` (Gemini), auditor=`codex-tmux`
**Status:** ROOT-CAUSED — fix pending architecture decision

---

## Symptom

Cycle 156's build did **real, substantial, on-goal work** — committed in the worktree as
`0631352 feat: phase latency tracking, diagnostics, and backfill [worktree-build]` (12 files,
+888/−5: `orchestrator.go` timing+diag+retry+backfill wire-in, new `go/internal/phases/backfill/`
package + tests, new latency/error-recovery doc, ACS predicates).

Yet the audit verdict was **FAIL**:

> *"Build report claims T2/T3/T4/T5 landed, but the tree still lacks `backfill.go` … `orchestrator.go`
> still returns on timeout."*

and the cycle ended `SKIPPED_UNKNOWN` — *"HEAD did not advance … worktree changes were discarded."*
The work was thrown away.

## Root cause — a three-way contract conflict on commit state

| Component | What it does | File |
|---|---|---|
| **Builder** | `git add -A && git commit -m "… [worktree-build]"` — **commits** its work | `agents/evolve-builder.md:235`, `evolve-builder-reference.md:86` |
| **Auditor** | *"Run `git diff HEAD` yourself to verify changes"* — inspects **pending** diff | `agents/evolve-auditor.md:57` |
| **Orchestrator binding** | `tree_state_sha = sha256(git diff HEAD)` — also **pending** | `go/internal/core/orchestrator.go:301` |

The builder **commits**; the auditor + binding inspect **`git diff HEAD`**, which is **empty after a
commit**. So the auditor sees no changes → FAIL → the cycle's committed work is discarded.

The orchestrator's own comment (orchestrator.go:282-288) states the *intended* model:
*"stage everything and write a tree … No commit is made (write-tree only); ship re-stages anyway"* —
i.e. it assumes the builder leaves **uncommitted** changes. The builder's commit instruction violates
that assumption.

### Why it surfaced now (multi-CLI exposure)

This is an "any CLI × any phase must execute" failure. Claude builders historically appear to have
*not* committed (or the harness tolerated it), so `git diff HEAD` showed their work. **agy (Gemini)
followed `evolve-builder.md:235` literally and committed** — exposing that the audit/binding flow
implicitly assumed Claude-builder behavior. The pipeline built a feature to avoid discarding useful
work on failure (backfill), then discarded its own useful work via this binding bug.

## Fix options

| Opt | Change | Pros | Cons |
|---|---|---|---|
| **A** | Auditor + binding diff against the **cycle base** (`git diff <base>..HEAD`) instead of `HEAD` | committed work counts; keeps builder checkpoint commits | touches the security-sensitive binding + ship's tree-SHA comparison; auditor must learn the base |
| **B** | Builder **does not commit** (stage only / leave pending) | simplest; matches the documented "uncommitted, ship re-stages" model + auditor's `git diff HEAD` | loses checkpoint-on-hard-exit safety (evolve-builder.md:230 rationale) |
| **C** ⭐ | Build phase **soft-resets** the builder's `[worktree-build]` commits to the cycle base **before audit** (`git reset --soft <base>`) | preserves checkpoint-DURING-build; CLI-agnostic; **no change** to auditor prompt or binding (they keep `git diff HEAD`, which now shows the un-committed work); all components agree | one new orchestrator/build-phase step; must resolve the correct base SHA |

**Recommendation: Option C.** It reconciles both contracts without touching the security-sensitive
binding or the auditor prompt: the builder keeps committing for crash-safety during the build, and the
build→audit seam normalizes the worktree back to pending changes so `git diff HEAD` (used by both the
auditor and the binding) consistently reflects the work. Lowest blast radius, highest internal
consistency.

## Cross-references

- `docs/incidents/cycle-154-agy-tmux-m-flag-repl-boot-timeout.md` (sibling multi-CLI exposure: agy builder)
- recent audit-binding commits: *"bind ship to the audited worktree CHANGES tree, not HEAD^{tree}"*,
  *"orchestrator writes the auditor binding ledger entry"*
