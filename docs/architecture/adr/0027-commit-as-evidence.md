# ADR-0027: Commit-as-Evidence for Phase Completion

> **Invariant: a commit is the evidence of a phase's deliverable — for every phase, without exception.** A phase has delivered iff it produced a commit on the per-cycle worktree branch; completion is "HEAD advanced," never "a file appeared at the exact path I'm polling." This holds uniformly across intent → scout → triage → tdd → build → tester → audit → ship → learn. There is no path-polling fallback as the source of truth. Deterministic, tamper-evident, and aligned with the project's existing audit-binding. **Status: Proposed** — design captured for deliberate implementation; not yet built.

- **Status:** Proposed (design)
- **Date:** 2026-05-27
- **Relates:** ADR-0024 (tolerate + relocate non-canonical artifact writes — the brittle workaround this supersedes), ADR-0026 (self-healing review layer — the reviewer consumes completion signals)

## Context

Phase completion is currently inferred by **polling a filesystem path** (`bridge.artifactReady` polls `cfg.Artifact`). This has produced a recurring class of failures, twice in one session (cycle 109/110):

| Symptom | Cause |
|---|---|
| scout killed at 300s while streaming | wall-clock timeout on the poll (fixed in ADR-0026) |
| triage `exit=81` despite finishing | agent wrote `triage-decision.md` to a `workspace/` subdir; the poll watched the canonical path; the ADR-0024 relocate safety-net did **not** fire (cause still unconfirmed) |
| triage artifact had literal `<token from runner>` | doc didn't use `$CHALLENGE_TOKEN`; poll is token-blind so it didn't matter, but it would fail downstream |

The root brittleness: **path-polling couples completion to where the agent put the file**, which agents get wrong (subdirs, placeholders, timing). Each miss needs a new heuristic (relocate, deeper scan, token-awareness) — whack-a-mole.

## Decision

**A commit is the evidence of a phase's deliverable — universally, for every phase.** This is an invariant, not a per-phase choice: the orchestrator advances iff the phase produced its commit; detection is uniform (`git`), never a per-phase path heuristic. No phase is exempt and there is no "if the file shows up at path X" fallback that can substitute for the commit.

| Phase | Deliverable (committed) | Evidence |
|---|---|---|
| intent | `intent.md` | worktree-branch commit |
| scout | `scout-report.md` | worktree-branch commit |
| triage | `triage-decision.md` / `.json` | worktree-branch commit |
| tdd | predicate scripts | worktree-branch commit |
| build | production code + `build-report.md` | worktree-branch commit |
| tester | predicates + `tester-report.md` | worktree-branch commit |
| audit | `audit-report.md` + verdict | worktree-branch commit |
| **ship** | the `main` merge commit | **the gated commit** (only ship reaches `main`) |
| learn / retro | lessons / instincts | worktree-branch commit |

Ship is not an exception to the invariant — it is its apex: every upstream phase's evidence is a worktree-branch commit, and ship is the single phase whose evidence commit lands on `main` (through the existing gate). Mechanically, a phase finishes when it has committed its artifact(s) to the per-cycle **worktree branch**; the orchestrator detects completion by inspecting git, not by polling a path:

```
Phase agent  → writes artifact(s) anywhere in the worktree, then commits
Completion   → HEAD advanced on the worktree branch (deterministic)
Detection    → orchestrator reads the artifact FROM the commit tree
               (git show <sha>:**/triage-decision.md) — path-placement-agnostic
Verify       → commit trailer carries challenge-token + phase; ledger binds the SHA
```

Why this is the right shape for *this* codebase:
- **Eliminates the path-detection layer.** The artifact is in the commit tree wherever the agent put it; `git` finds it by name/glob. "Wrote to `workspace/`" stops mattering — relocate (ADR-0024) becomes unnecessary.
- **Extends an existing pattern, not a new one.** Audit-binding already binds verdicts to `tree_state_sha`; the ledger is a `prev_hash` chain. Commit-as-evidence is the same git-anchored-provenance philosophy applied to every phase.
- **Tamper-evident + naturally resumable.** Each phase commit is an auditable checkpoint; `--resume` can key off the last phase commit.

## Trust-kernel interaction (the hard part)

The kernel invariant is **"only `evolve ship` commits, and only to `main`"** (`evolve guard ship` blocks all other git commit/push). Naively letting every phase commit violates it. It is compatible **iff**:

1. Phase commits target the **per-cycle worktree branch**, never `main`. `ship` still exclusively gates the worktree→`main` ff-merge — the main-branch guarantee is unchanged.
2. `evolve guard ship` is relaxed to permit commits **on a worktree branch** (detect via `git rev-parse --show-toplevel` ≠ project root, or branch name prefix), while still denying any commit/push that targets `main`.
3. Today only **Builder** runs in a worktree; scout/intent/triage/audit write to `.evolve/runs/` outside a branch. Commit-as-evidence requires **every phase to operate on the worktree branch** — the largest structural change.

## Rollout (incremental, but the end-state is the universal invariant)

The invariant above is the destination for **all** phases; these steps are how we get there safely, not a license for some phases to stay on path-polling permanently.

1. **Transitional `git`-based completion signal (smallest first step):** before every phase is moved onto a branch, make the orchestrator's completion check `git`-based — the runner computes `git hash-object` of the produced artifact (or the phase emits a commit) and the bridge treats *that* object as "done," reading the artifact from it. This is scaffolding that already kills path-polling brittleness with a minimal kernel touch; it is **not** the end-state.
2. **Every phase on the worktree branch:** move intent/scout/triage/tdd/build/tester/audit/learn onto the per-cycle worktree branch so each commits its deliverable natively; relax `guard ship` to permit worktree-branch commits while still denying any commit/push to `main`. After this step the invariant holds uniformly.
3. **Commit trailer + ledger bind:** standardize an `Evolve-Phase:` / `Challenge-Token:` trailer on every phase commit; bind the phase commit SHA in the ledger (it replaces the artifact-SHA field's role).
4. **Retire path-polling + relocate (ADR-0024):** once completion is git-anchored for every phase, delete `artifactReady`'s path-polling/relocate fallback — there is no longer a non-commit source of truth to maintain.

## Consequences

- **+** Removes an entire failure class (path mismatch, subdir confusion, relocate gaps, token-blind polling).
- **+** Stronger provenance; phase outputs become first-class git history, consistent with audit-binding.
- **+** Per-phase checkpoints improve `--resume` granularity.
- **−** Largest version (steps 2–3) touches the trust kernel + the phase/worktree model — must preserve "only ship touches main." Do it deliberately, with the kernel parity tests green.
- **−** Until step 2, the hybrid (step 1) still relies on the agent producing *an* artifact (just not at an exact path); a totally silent phase is still caught by the ADR-0026 reviewer.

## Open questions

- Object-hash vs commit: is a per-phase commit worth the git overhead, or is `git hash-object` of the artifact (no commit) a sufficient deterministic signal for the hybrid step?
- Does committing `.evolve/runs/` artifacts to the worktree branch bloat history? (Likely scope the evidence to a trailer + the artifact, not the whole `runs/` tree.)
