# Unmerged-work ledger — consolidation triage (2026-05-30)

Captured at consolidation start (main @ `5436919`). Every item below is preserved in the
Stage-0 recovery net (see [history-recovery.md](history-recovery.md)) **before** any branch deletion.
This table drives Stage 2 (merge keepers) and Stage 5 (prune the rest).

## Recommendation legend

- **KEEP** — merge onto `consolidation/go-only` (then it reaches `main`).
- **REVIEW** — needs a human/security decision before keep-vs-drop.
- **DROP** — superseded or absorbed; knowledge already captured; abandon (recoverable from bundle).

## Worktree branches

| Item | Commits ahead | Content | Rec |
|---|---|---|---|
| `ship-recovery-e2e` (worktree `evolve-loop-e2e`) | 4 | `b212a33` advisor-centric ship-error recovery + debugger phase; `89943c9` evolve-cycle 2; `25ccb11`+`4442e35` GoDoc | **KEEP** — active ship-as-executor recovery build; the session's current feature |
| `cycle-3` (worktree under evolve-loop-e2e) | 2 | `b212a33`+`89943c9` — subset of ship-recovery-e2e | **DROP** — strict subset of ship-recovery-e2e; merging that covers it |
| `egps-skip-fix` (worktree `evolve-loop-egps`) | 0 commits, **14 files uncommitted** | ACS cycle-57/80/84 predicates + acsrunner/acssuite + audit_gaps test changes | **REVIEW** → likely fold into Stage 3 (test redesign); patch saved |
| `cycle-144` (worktree) | uncommitted | observer_test.go change + build-report.md delete + acs/cycle-144/ | **DROP** — observer fix; verify covered by Stage 2 WS-E (liveness) or re-derive in Stage 3 |
| `cycle-125`, `feat/e2e-cli-scenarios` (worktrees) | 0 (at main) | no drift | **DROP** — already at main |

## Remote-only feature branches (`origin/feat/ws-*`, never pulled)

These 7 look load-bearing — the any-CLI/sandbox/liveness fixes. Triage each against current `main`
(some may already be absorbed via PR #27 / later cycles).

| Branch | Commit | Intent | Rec |
|---|---|---|---|
| `feat/ws-a-abs-root` | `0e1ca42` | systemic absolute project-root via shared AbsoluteRoot helper | **REVIEW→KEEP** if not in main |
| `feat/ws-b-sandbox` | `b4f9897` | CLI-agnostic sandbox confinement + tree-diff guard | **REVIEW→KEEP** (security-relevant) |
| `feat/ws-c-inert-warn` | `34f8ee0` | warn on inert PhaseEnable=On under static routing | **REVIEW→KEEP** |
| `feat/ws-d-soft-fail` | `5aadd93` | optional-phase soft-fail + empty-output quota classify | **REVIEW→KEEP** |
| `feat/ws-e-liveness` | `9b89190` | liveness backstop + per-phase review gate | **REVIEW→KEEP** (supersedes cycle-144?) |
| `feat/ws-f-ollama` | `c22afcd` | ollama-tmux driver — local/cloud LLM via model tag | **REVIEW→KEEP** |
| `feat/ws-g-multi-cli` | `78fb0c1` | any-CLI/any-model/any-phase pipeline | **REVIEW→KEEP** (matrix invariant) |

Per-branch check: `git log main..origin/feat/ws-X --oneline` and `git cherry main origin/feat/ws-X`
to see whether the commit is already equivalent in main before merging.

## Local `cycle-*` branches (38)

Mostly single-commit worktree-build artifacts (docs/refactor/test commits, cycles 121–153). Each commit
is captured in the bundle. **DROP all** after confirming via `git cherry main cycle-N` that the change is
either already in main or intentionally abandoned. Representative keepers to double-check before dropping:
`cycle-121` (DefaultDeliverableReviewer), `cycle-147` (dead-code cleanup), `cycle-151/152` (coverage tests).

## Stashes (5) — archived as `stash-archive/0..4` in the bundle

| # | Origin | Summary | Rec |
|---|---|---|---|
| 0 | feat/tmux-recipe | ship: reset go/evolve before ff-merge (cycle-153 fix) | **DROP** — one-off ship fix, likely already applied |
| 1 | main | cycle-119 P-NEW-23 budget-hint — **Gemini-injected during scout; cross-CLI trust bypass (issue#2)** | **REVIEW (SECURITY)** — do not merge without security review; see [[incidents/pattern-library]] Pattern C |
| 2 | main | cycle-104 bridge files (post-audit, route to bridge-local) | **DROP** — superseded by shipped bridge work |
| 3 | main | cycle-25-orphan: token-optimization agents/*.md (lost on worktree delete) | **REVIEW** — may contain useful agent prompts |
| 4 | main | cycle-25 Builder WIP (auditor-rejected classifier defects) | **DROP** — rejected work, kept for inspection only |

## Already safe

- **PR #27** (`b15c650` tmux recipe engine) — **MERGED** to main ✓
- **v13.0.0** — tagged ✓
- local `main` ↔ `origin/main` — in sync ✓
