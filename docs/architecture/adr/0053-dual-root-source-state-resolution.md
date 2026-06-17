# ADR-0053: Dual-Root Resolution — Separate the SOURCE Root from the STATE Root for ACS Predicates

- **Status:** Accepted (implemented + regression-tested in the cycle-355 follow-up commit)
- **Date:** 2026-06-18
- **Driver:** the cycle-355 audit FAIL (defects H1 + M1; EGPS `red=1` on
  `cycle355/TestC355_005_FlagsCheckExitsZero`). A cycle whose deliverable
  *regenerates a generated-from-source doc* (`docs/architecture/control-flags.md`
  from the `flagregistry`) tried to prove it with an EGPS predicate that shells
  `evolve flags check`. The check red-failed correct work. This is a **systemic**
  class — the cycle-355 lesson cross-references the same "predicate reads main, not
  worktree" failure in cycles 59, 75, 7, and 93.
- **Relates to:** issue #12 (`EVOLVE_PROJECT_ROOT` pins predicates' `.evolve/`
  reads to main), ADR-0040 (`skills generate|check` projection — the sibling
  generated-doc command), the no-workaround / root-cause-redesign rule.

## Problem

The ACS suite runs each predicate with `EVOLVE_PROJECT_ROOT = mainProjectRoot(Root)`
(`cmd_acs.go`, `acssuite.go`). `mainProjectRoot` follows
`git rev-parse --git-common-dir`, so it resolves to the **main checkout regardless
of `--root`**. That is *correct* for its documented purpose (issue #12): predicates
must read `.evolve/` runtime **STATE** (history, baselines, the current
build-report) from main, because a cycle's worktree has no authoritative `.evolve/`.

But the same variable is *reused* by `evolve flags check` / `evolve skills check`
(via `envOrCwd("EVOLVE_PROJECT_ROOT")`) to locate a **SOURCE/DOC artifact** —
`control-flags.md` / `SKILL.md`. For a worktree-scoped deliverable the correct root
for that artifact is the **worktree**: the cycle commits the regenerated doc there,
and it reaches main only at *ship*, after audit. So:

1. `flags check` validated **main's** stale `control-flags.md` against the compiled
   registry → drift → exit 2 → EGPS `red=1` → audit FAIL, on correct work.
2. The only "green" was a manual `EVOLVE_PROJECT_ROOT=<main> evolve flags generate`
   — a non-durable edit to a working tree *outside the cycle's commit scope* that
   evaporated before audit. The audit correctly caught this as gaming (the
   implicit-adversarial class).

**Root cause:** `EVOLVE_PROJECT_ROOT` is semantically overloaded — it means both
"the STATE root" (→ main) and, accidentally, "the SOURCE root for reading committed
docs" (→ should be the worktree). One variable cannot serve both. The predicate
author's `cd <worktree>` was silently overridden by the suite-injected
`EVOLVE_PROJECT_ROOT=main`, which takes precedence over cwd in `envOrCwd`.

Verified empirically: the cycle-355 worktree binary against the worktree root
(`EVOLVE_PROJECT_ROOT=<worktree> <worktree>/go/bin/evolve flags check`) exits **0** —
the deliverable is correct; only the root resolution was wrong.

## Decision

Complete the missing half of the abstraction — the **dual-root pattern**:

- **`EVOLVE_PROJECT_ROOT` (STATE root)** — unchanged. The suite keeps pinning it to
  main so `.evolve/` reads resolve there (issue #12 preserved).
- **`EVOLVE_WORKTREE_ROOT` (SOURCE root)** — new. `acssuite.predicateEnv` exports it
  as the cycle's worktree (`opts.Root`, which `resolveACSSuiteRoot` already resolves
  from `cycle-state.json`'s `active_worktree`). The suite already held both roots; it
  now exports both.
- **`sourceRoot()`** — one centralized resolver (in `cmd/evolve`, beside
  `envOrCwd`). Precedence: `EVOLVE_WORKTREE_ROOT` > `EVOLVE_PROJECT_ROOT` > cwd.
  Generated-doc commands (`flags`, `skills`) resolve their doc through it. Outside
  the suite `EVOLVE_WORKTREE_ROOT` is unset, so `sourceRoot()` is **byte-identical**
  to the prior `envOrCwd("EVOLVE_PROJECT_ROOT")` (CI/dev unaffected).

Net effect: shelling `evolve flags check` / `evolve skills check` from a worktree
predicate now validates the **committed worktree artifact** automatically — no
per-predicate special-casing, no manual main regen. Defense-in-depth: the README
still recommends the *most* robust predicate (regenerate from the worktree SSOT
in-process and compare — no subprocess, no binary dependency); the subprocess path
is the safety net the dual-root pattern keeps correct.

## Alternatives considered

- **(Rejected) Predicate-authoring guidance only** — tell authors to set
  `EVOLVE_PROJECT_ROOT=<worktree>` or assert in-process. Relies on per-cycle LLM
  adoption → the recurring whack-a-mole the lesson documents. Soft, not durable.
- **(Rejected) Ship regenerates + commits main's index** — keeps main in sync so
  the *wrong-root* read happens to match. Treats the symptom (stale main), not the
  cause (overloaded root); adds main mutation to ship.
- **(Rejected) Resolve the doc via `git rev-parse --show-toplevel` of cwd, ignoring
  the env var** — works without a new var, but is implicit and changes how every
  source/doc reader honors `EVOLVE_PROJECT_ROOT` (higher surprise risk). The
  explicit named root is more intention-revealing and a pure addition.
- **(Rejected) Sweep all 22 `envOrCwd("EVOLVE_PROJECT_ROOT")` call sites** — most
  are `.evolve/` STATE or release ops that must stay on main. The generated-doc
  projection class is exactly `flags` + `skills`; fixing those fixes the class
  (KISS, no speculative breadth).

## Consequences

- **Positive:** the systemic "predicate reads main, not worktree" class is closed
  at the framework layer; `flags`/`skills check` are now correct as EGPS predicates;
  issue #12 untouched; no behavior change outside the suite.
- **Cost:** one new path-input env var (`EVOLVE_WORKTREE_ROOT`) — not a behavior
  toggle, the source half of the existing dual-root vocabulary
  (`EVOLVE_PROJECT_ROOT` already documents the "dual-root pattern"). Registered in
  `flagregistry` for consistency with its siblings.
- **Regression guard:** `TestFlagsCheck_ResolvesWorktreeRootOverProjectRoot`
  (cmd/evolve) — in-sync worktree + stale main; asserts `check` exits 0 via the
  worktree redirect and falls back to (failing) main when the var is unset.
  `TestPredicateEnv_AllBranches` covers the export.
- **Falsifiable health check:** from a clean checkout with no manual `flags generate`
  on main, `evolve acs suite --cycle N` for a doc-regenerating cycle yields `red=0`.
