# Multi-CLI + dynamic-advisor + swarm validation run — 2026-05-31

**Config:** `EVOLVE_DYNAMIC_ROUTING=advisory` + `EVOLVE_SWARM_STAGE=advisory` + `EVOLVE_SWARM_CONCURRENCY=2`,
strategy=balanced. Per-phase CLIs: **builder=`agy-tmux` (Gemini)**, **auditor=`codex-tmux` (OpenAI)**,
**all reasoning/coordination=`claude-tmux`**. Goal: improve the loop's own latency + error-handling +
self-heal + artifact backfill.

**Headline:** the multi-CLI run did exactly what a validation should — it systematically surfaced
that **the pipeline was Claude-shaped, not CLI-agnostic.** Four distinct bugs, all the same class:
a place where the harness assumed Claude-builder behavior and broke when agy/Gemini ran the build
phase honestly. Three are fixed + shipped; the fourth is an architectural decision (below).

## Per-cycle outcomes

| Cycle | Builder booted? | Built real work? | Audit | Shipped? | Blocker |
|---|---|---|---|---|---|
| 154 | ❌ exit=80 | — | — | no | agy `-m` manifest → REPL boot timeout |
| 156 | ✅ Gemini | ✅ +888 LOC (backfill, tests, doc) | FAIL | no | builder committed → auditor `git diff HEAD` empty |
| 158 | ✅ | ✅ +874 LOC | FAIL (Structure) | no | stale `<!-- Challenge:` template header |
| 160 | ✅ | ✅ (leaked to main) | — | no | **build wrote to main tree, not worktree** |

## Bugs fixed & shipped

### 1. agy `-m` flag → REPL boot timeout (`0af8cc1`, CI green)
`agy-tmux`/`agy` manifests injected `-m gemini-3.5-flash`; agy 1.0.3 has no `-m` flag, so the REPL
never booted (60s timeout → exit 80) and the batch aborted with no fallback. Fixed: manifest
`model_tier→noop` (both manifests + catalog), 5 stale-`-m` test assertions corrected, `cli_fallback`
added to builder+auditor (the fallback chain was dead by default — no profile set it).
→ `docs/incidents/cycle-154-agy-tmux-m-flag-repl-boot-timeout.md`

### 2. Builder commits → auditor sees empty diff (`f96537c`, CI green)
Builder is instructed to `git commit … [worktree-build]` (crash-safety), but the auditor
(`evolve-auditor.md:57`: "Run `git diff HEAD`") and the audit-binding both inspect the *pending* diff
— empty after a commit. agy followed the commit instruction literally; every cycle's work was
discarded. Fixed (Option C): `normalizeWorktreeToBase` soft-resets the worktree to the cycle base
after build, re-exposing committed work as pending — no change to the auditor or the security binding.
**Validated at runtime in cycle 158** (log: "worktree-normalize: soft-reset …"; auditor then ran the
tests and passed Matches-ACs). → `docs/incidents/cycle-156-builder-commit-vs-audit-pending-diff.md`

### 3. Stale `<!-- Challenge:` template (`0853934`, CI green)
3 agent templates used the stale capital-C header; the auditor enforces canonical `<!-- challenge-token:`.
agy emitted the stale form → Structure FAIL (every other check PASS, `red_count=0`). Fixed all 3.
→ `docs/incidents/cycle-158-stale-challenge-token-template-drift.md`

## 4. OPEN — worktree write-confinement for non-Claude builders (architectural)

Cycle 160's build wrote its **entire output to the main tree**, not its worktree
(`go/internal/phases/backfill/`, tests, `go/evolve`, a doc). The Go-level tree-diff guard caught the
tracked-file leak (`go/evolve`) and aborted (good defense-in-depth), but the root cause is:

- **The role-gate that confines writes to the worktree is a Claude-Code PreToolUse hook.** It does
  not intercept agy/codex tool calls (they run in tmux as a different CLI).
- **The OS sandbox** that would otherwise confine them is disabled on nested-macOS
  (`EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1`, required because `sandbox-exec` can't nest).

So on this platform there is **no write-confinement enforcement for a non-Claude builder**. agy wrote
to `project_root` instead of `worktree` (both are in its prompt). This is not a one-line fix.

### Options for #4

- **A — Confine non-Claude builders (CHOSEN 2026-05-31).** Make the build-phase leak *recoverable*:
  relocate the build's main-tree writes into the worktree, then restore main. Most faithful to
  "any CLI builds"; doubles as the self-heal the goal asks for.
- **B — Claude builds, agy elsewhere.** builder=`claude-tmux`, agy on a non-writing phase. Pragmatic.
- **C — Untrack `go/evolve`.** Gitignore the built binary. Narrow; pairs with A.

### Implementation plan for A (ready for a fresh session — do NOT rush in a long session)

cwd is already the worktree (`driver_tmux_repl.go:122`), but agy used absolute `project_root` paths
from its prompt, so cwd-confinement alone won't catch it. Recovery design:

1. **Full pre-build dirty baseline.** The existing tree-guard `beforeDirty` uses
   `git diff --name-only HEAD` = **tracked-modified only** — it does NOT see untracked files (which is
   why cycle-160's `go/evolve` was flagged but `backfill/` + tests were not). Capture a FULL
   `git status --porcelain` baseline of `projectRoot` at cycle start (alongside the existing
   `worktreeBaseSHA` capture at orchestrator.go ~507), storing the set of already-dirty paths.
2. **`recoverBuildLeak(ctx, projectRoot, worktree, baseline)`** (new orchestrator helper, mirror
   `normalizeWorktreeToBase`'s best-effort style): run `git status --porcelain` in projectRoot; for
   each path NOT in `baseline`:
   - `??` (untracked-new) → `os.Rename(projectRoot/p, worktree/p)` (mkdir parent) — relocate the
     build's output into the worktree where audit/ship expect it.
   - ` M`/`M ` (modified tracked) → `git -C projectRoot checkout -- p` — discard (e.g. a rebuilt
     `go/evolve` the cycle shouldn't carry).
   - renames/deletes → conservatively return false (don't auto-handle; abort).
   Return true iff projectRoot is clean of new leaks afterward.
3. **Wire** into the post-build guard handling (orchestrator.go ~800): when `next == PhaseBuild` and
   `!res.OK()` (not SnapshotMissed), call `recoverBuildLeak`; on success log + continue (the relocated
   work proceeds to audit), on failure fall through to the existing `res.Error(...)` abort. Keep it
   additive so non-leaking cycles are byte-identical.
4. **TDD** (real-git, mirror `worktree_normalize_test.go`): (a) untracked leak relocated to worktree +
   main clean; (b) tracked-modified leak discarded + main clean; (c) pre-existing baseline dirt left
   untouched; (d) rename/delete → false (abort path preserved).
5. **Pairs with C:** also gitignore `go/evolve` (or stop tracking it) so a rebuilt binary never even
   reaches the guard.

**Two subtleties found while scoping (must handle):**

- **Stage the relocated files.** After `os.Rename`-ing leaked untracked files into the worktree, run
  `git -C worktree add -A`. Otherwise they stay untracked and the auditor's `git diff HEAD` won't show
  them — the same blind spot Option C fixed. (Then Option-C `normalizeWorktreeToBase` runs after, as a
  no-op when the builder made no worktree commit.)
- **Run recovery UNCONDITIONALLY after build, not only on guard-trip.** The tree-guard's
  `git diff --name-only HEAD` sees only *tracked* leaks (it flagged `go/evolve` but NOT the untracked
  `backfill/`+tests). A pure-untracked leak passes the guard yet still escapes. So call
  `recoverBuildLeak` for every build phase (no-op when clean), placed BEFORE the guard check — then a
  recovery failure still falls through to the guard's loud abort for safety.

Caveat to weigh: relocating *arbitrary* leaked paths assumes they're intended build output. If a
non-Claude builder ever modified an unrelated main file, recovery would carry it into the cycle. The
baseline-subtraction limits blast radius to build-introduced paths; consider an allowlist
(`go/**`, `docs/**`, `acs/**`, …) if paranoia warrants.

## Queued follow-ups (documented, not yet done)

- Derive the CLI fallback chain from `allowed_clis` when `cli_fallback` is unset, + WARN when a
  profile has multiple `allowed_clis` but a single-candidate chain (the fallback machinery is
  dead-by-default).
- TDD `tdd-report.md` handoff sometimes missing → spine "proceeding fail-open" WARN every cycle.
- POSTHOC metric sidecars (`builder-usage.json`, `builder-timing.json`) not emitted for the agy path →
  unverifiable `num_turns`/`duration_ms` (MEDIUM).
- Live-boot capability probe: `setup detect` rated agy "ready" (binary+auth) while its REPL couldn't
  boot — a one-shot `agy --print` smoke test would catch flag breakage pre-run.

## What the run validated about latency/swarm/routing

- **Dynamic-advisor routing (advisory):** worked — router ran at cycle start, computed the phase plan,
  the integrity-floor clamp fired (`ship-requires-tdd: tdd=skip→tdd=run`).
- **Swarm (advisory, 2 workers):** gated on; no leak/abort attributable to it.
- **Latency:** not yet measurable end-to-end — the loop's own goal (persist `DurationMS` to
  `phase-timing.json`) was Finding 1 the Scout selected, but no cycle shipped it, so per-phase timing
  is still not captured. (This is itself a finding: the pipeline can't yet self-measure latency.)
