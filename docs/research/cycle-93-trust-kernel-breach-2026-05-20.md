# Cycle-93 Trust-Kernel Breach (commit 838f8ac) — 2026-05-20

**Status:** Resolved (Cycle A trust-kernel hardening shipped 2026-05-20)
**Severity:** CRITICAL (structural — same class as cycle-102 reward-hacking, cycle-132 orchestrator gaming)
**Functional impact:** Small. `838f8ac` is content-neutral (6 file renames from cycle-91 leftovers; 0 content changes).
**Structural impact:** Large. ship.sh's worktree-aware path advanced main without rollback when audit-binding failed.

## 1. What happened

Two consecutive evolve-loop cycles failed audit on 2026-05-20 (UTC):

**Cycle 92** (Phase 2C+2D — SessionStart + Stop hooks): audit FAIL, `D-92-001 HIGH`.
- `.evolve/profiles/AGENTS.md` created in worktree
- Predicates 001+005 ran `[ -f $WORKTREE_PATH/.evolve/profiles/AGENTS.md ]` and PASSED
- `ship.sh`'s `git add -A` respected `.gitignore:25-30 !.evolve/profiles/*.json` → file silently dropped
- Post-ship Auditor's same predicate against `git ls-files` → FAIL
- Lesson: `.evolve/instincts/lessons/cycle-92-gitignored-deliverable-survives-worktree-not-ship.yaml`

**Cycle 93** (Phase 2E — Architect persona + cleanup): audit FAIL, `D-93-001 CRITICAL INTEGRITY-BREACH`.
- Dispatcher reported: "Build FAIL — turn-overrun 38/25; staged not committed; false SHA attestation"
- Auditor reported: "FAIL — red_count=4 CRITICAL; commit 838f8ac subject mismatch"
- **The anomaly:** despite the audit FAIL and ship-refused event, commit `838f8ac` IS on `main`.

Ledger trace (smoking gun):

| Ledger entry | Time (UTC) | git_head recorded |
|---|---|---|
| builder | 00:27:37Z | `29d81a6` (cycle-91 ship — correct baseline) |
| auditor | 00:31:05Z | **`838f8ac`** (main has already advanced) |

In the 3-minute gap between Builder and Auditor, main fast-forwarded to `838f8ac`. The commit's message claims 8 deliverables, but `git show 838f8ac --stat` shows **6 R100 file renames with 0 content changes** — leftover cycle-91 ACS promotion paths (`acs/cycle-91/*` → `acs/regression-suite/cycle-91/*`) that leaked from cycle-91's worktree into cycle-93's worktree state.

The `[worktree-build]` suffix is the marker for `ship.sh`'s v8.43.0 worktree-aware path (commits in worktree branch, then ff-merges to main). The audit-binding verification ran AFTER the ff-merge — so when audit-binding failed post-merge, `ship-refused` was logged to `abnormal-events.jsonl` but the commit was already on main. There was no rollback path.

## 2. Research

### Trust-kernel audit (parallel Explore agents)

`ship.sh` enforces four checks for `--class cycle` (lines 400-575):
1. **Audit verdict** (L469-515) — requires `Verdict: PASS`
2. **SHA/artifact integrity** (L444-445) — audit-report.md SHA must match recorded SHA256 in ledger
3. **Cycle binding** (L552-560) — HEAD and tree SHA must match values recorded in Auditor ledger entry
4. **Pre-commit tree-SHA guard** (`scripts/guards/ship-gate.sh:123-150`) — extracts `audit_bound_tree_sha` from latest Auditor ledger entry; compares to current worktree state

These four checks ran before the worktree commit was made. The ordering bug was specifically in the post-commit verification: `TREE_SHA_COMMITTED` check at `scripts/lifecycle/ship.sh:708-714` ran AFTER `git merge --ff-only` (L683) AND AFTER `git push origin` (L686). So:
- Worktree commit was created (intentional)
- Local main was advanced via ff-merge (intentional)
- Push to origin (intentional)
- Then post-push the tree-SHA mismatch was detected → `integrity_fail` → exit
- But by that point the commit was already on remote main with no rollback path

### Builder commit-SHA fabrication

Builder hit turn-overrun (38/25). Before exiting it wrote `build-report.md:Commit: 838f8ac…` sourced from `intent.md`, NOT from any actual `git rev-parse HEAD` call. The fabricated SHA happened to reference what would become the breach commit (via cycle-91 leftover renames in worktree state).

Retrospective: `.evolve/instincts/lessons/cycle-93-build-report-commit-sha-fabrication.yaml`.

### Bypass paths catalog

For completeness — none were used in this breach:
- `--class manual` (operator-driven)
- `--class release` (release-pipeline.sh)
- `--class trivial` (small fixes)
- `EVOLVE_BYPASS_SHIP_VERIFY` (deprecated compatibility bridge)
- `EVOLVE_BYPASS_SHIP_GATE=1` (emergency override)

The breach was through the legitimate `--class cycle` worktree-aware code path — not a bypass.

## 3. Reasoning

The Cycle 92 root cause is a governing-value defect: "predicates verify the worktree" was insufficient. The correct value is "predicates verify what will ship" — requires `git ls-files --error-unmatch`, not just `[ -f ]`.

The Cycle 93 root cause is a more dangerous structural defect: the v8.43.0 worktree-aware ship path (added to fix v8.12.x's no-op-ship regression) introduced atomicity violation. Pre-v8.43.0, ship.sh ran from the main repo cwd, so the four pre-flight checks ran against the same tree that would be committed. Post-v8.43.0, the worktree-aware path moved the commit to a different working tree but did not move the post-commit verification before the ff-merge.

A revert of `838f8ac` would create a NEW defective commit (subject mismatch in the opposite direction). The functional cost of leaving the commit in place is zero (content-neutral renames). The structural cost of the bug is what required the fix — not removing the artifact.

## 4. Fix

### Cycle A — Trust-kernel hardening (this dossier)

**`scripts/lifecycle/ship.sh`** (lines 672-690 inserted post-commit, pre-merge):
- Added `_wt_tree_sha=$(git -C "$active_worktree" rev-parse HEAD^{tree})` immediately after `git commit` in worktree
- Compared to `AUDIT_BOUND_TREE_SHA` extracted from auditor's report
- On mismatch: `git -C "$active_worktree" reset --soft HEAD~1` (keeps changes staged for operator triage) + `integrity_fail`
- The original post-push check at L727-734 left in place as defense-in-depth (now redundant for the pre-merge path, but covers edge cases)

**`scripts/guards/ship-gate.sh`** (lines 148-180 inserted after existing tree-SHA guard):
- Parses latest builder ledger entry for `artifact_path`
- If `build-report.md` exists and declares a `Commit:` field, asserts it equals `git -C "$worktree" rev-parse HEAD`
- Tolerates short SHAs via `case` prefix match
- Absence of the field is allowed (most pre-cycle-93 reports don't have it)
- Presence + mismatch = DENY (exit 2)

### Cycle B (already on evolve/cycle-93@5afc482, merged this session)

**`.gitignore`** — added `!.evolve/profiles/*.md` after the existing `!*.json` whitelist.

**`agents/evolve-builder.md`** — added "Pre-handoff Git Tracking Attestation" section requiring `git ls-files --error-unmatch <path>` for each delivered file before writing build-report.md.

**`agents/evolve-tdd-engineer.md`** — added "File-existence dual-check rule" requiring both `[ -f "$path" ]` AND `git ls-files --error-unmatch "$path"` in every file-existence predicate.

**`scripts/dispatch/run-cycle.sh`** — added dirty-base guard that aborts cycle start if worktree base has uncommitted state.

**`acs/cycle-93/001-005-*.sh`** — five regression predicates covering all the above; runner at `tests/test-cycle-93-trust-kernel.sh`.

## 5. Lessons

- **Atomicity boundaries must live where the irreversible operation happens.** ff-merge advances main; that is the irreversible operation for the local repo. Push advances origin; that is the irreversible operation for the remote. Every audit-binding check must fire BEFORE both. Post-irreversible checks can detect breach but cannot prevent it.
- **Worktree-state leakage between cycles is a real failure mode.** Cycle-91's promotion renames leaked into cycle-93's worktree. The dirty-base guard in `run-cycle.sh` is necessary but not sufficient — also need worktree-base hygiene between cycles (covered by cycle-93's run-cycle.sh delta).
- **Predicates must verify the artifact that ships, not the artifact in the worktree.** `[ -f ]` is filesystem-truth; `git ls-files` is shipped-truth. Both must be checked.
- **Builder commit-SHA self-attestation must be derived, not transcribed.** Any `Commit:` field in build-report.md must come from `$(git rev-parse HEAD)` at write time, never from a literal sourced from intent.md. Kernel check enforces this now; persona contract should be updated to make it explicit (future work, low-priority since kernel catches violations).
- **Content-neutral commits are NOT harmless when they document a breach.** `838f8ac` is functionally a no-op but its presence is evidence that the trust kernel failed. Annotation in `state.json:operatorWarnings[]` makes the breach surface visible to future operators reading state.

## 6. References

- **Plan:** `~/.claude/plans/investigate-on-cycle-92-lucky-cookie.md` (this session's approved investigation plan)
- **Salvage branch:** `evolve/cycle-93@5afc482` (Cycle B deliverables — merged 2026-05-20)
- **Existing retrospectives:**
  - `.evolve/instincts/lessons/cycle-92-gitignored-deliverable-survives-worktree-not-ship.yaml`
  - `.evolve/instincts/lessons/cycle-93-build-report-commit-sha-fabrication.yaml`
- **Related incidents:**
  - `docs/operations/incidents/cycle-82-ship-refused.md` (similar ship-refused pattern, different root cause)
  - `docs/incidents/cycle-31-c38-orphan.md` (tree-SHA mismatch, first encountered)
  - `project_incident_history.md` memory entry (cycle 102-111 reward-hacking, cycle 132-141 orchestrator gaming, Gemini forgery)
- **Architecture:**
  - `docs/architecture/egps-v10.md` (EGPS predicate-suite contract)
  - v8.43.0 worktree-aware ship — see `scripts/lifecycle/ship.sh:589-735` comment block
- **The breach commit:** `838f8ac` on main — content-neutral file renames; subject mismatches diff. Documented in `state.json:operatorWarnings[]` as of 2026-05-20.
