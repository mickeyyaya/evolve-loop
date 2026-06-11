# Soak Report — Batch #6 (R8.4, quiet host, tmux-only) — 2026-06-12

**Result: PASS — 4 consecutive PASS+delivered cycles (294–297), release bar (≥3) exceeded.**

## Request

Validate the concurrency-campaign + killer-B fixes under a quiet-host, tmux-only soak:
≥3 consecutive autonomous PASS+delivered cycles on the goal *"small, well-scoped
reliability and test-quality improvements"* (priority: finish the
`swarm-tests-relative-worktree-base` inbox defect — residue isolation + relative-base
guard), then release. Goal hash `98f198ac` (sha256 prefix).

## Timeline

| Event | Detail |
|---|---|
| 2026-06-11 22:51 | Soak #6 launched (pid 61945), preflight 5/6 PASS (known Darwin nested-sandbox WARN), all 3 tmux drivers boot-smoked incl. newly-routed agy-tmux |
| 2026-06-11 23:31 | **Host reboot** mid-cycle-294 (interrupted during mutation-gate; 8 phases complete, worktree intact) |
| 2026-06-12 ~01:00 | Operator recovery session: 3 resume-protocol defects hit in sequence (below) |
| 2026-06-12 01:00–02:00 | Resume-binding fix shipped `6707d1a7` (TDD, dual-review, /commit gate, CI green both workflows) |
| 2026-06-12 02:09 | Cycle 294 resumed (attempt 3) → PASS, shipped `b0002acd` |
| 2026-06-12 02:15–03:20 | Cycles 295–297 fresh batch, preflight 5/6 PASS → all PASS+shipped |

## Recovery: three resume-protocol defects (the reboot was a live fire drill)

1. **Checkpoint block clobbered by every `WriteCycleState`** — `cycle-state.json` is
   whole-file-replaced from a struct with no `Checkpoint` field
   (`go/internal/adapters/storage/statejson.go`), so the block written by the
   phase-boundary checkpointer survives only the tiny inter-phase window; any mid-phase
   crash finds "no live checkpoint". Filed as HIGH inbox defect
   (`checkpoint-block-clobbered-by-writecyclestate`); **fixed by cycle 295** —
   checkpoint-preserving `WriteCycleState` + tests (`statejson_checkpoint_test.go`).
   Operator workaround during recovery: hand-synthesized checkpoint block
   (reason `operator-requested`, mirroring `checkpoint.Compose`).

2. **Resume path emitted no audit/build provenance bindings** —
   `RunCycleFromPhase` lacked the `recordAuditBinding`/`recordBuildBinding` emissions
   `RunCycle` has, so a resumed audit→ship always bound to a stale auditor ledger entry
   and failed `AUDIT_BINDING_HEAD_MOVED`. Fixed interactively (shared
   `emitPhaseBindings` helper called from both loops + 2 regression tests), shipped
   `6707d1a7` via `/commit` → `evolve ship --class manual`, CI green.
   Secondary finding fixed by cycle 296: `Phase.IsValid()` rejected inserted phases as
   resume targets (operator had to resume from `audit` instead of `mutation-gate`);
   resume now accepts any phase with a registered runner.

3. **`GIT_FF_MERGE_DIVERGED` after the fix commit moved main** — cycle-294's branch
   (based on pre-fix main) could not ff-merge. Operator recovery: rebase the cycle
   commit onto new main (zero file overlap), soft-reset so work was pending again,
   re-resume from audit for a fresh binding → clean ship.

## Cycle deliverables (all audited, EGPS red_count==0, shipped to main)

| Cycle | Commit | Delivered |
|---|---|---|
| 294 | `b0002acd` | Swarm relative-base guard (`provision.go`) + worktree-test residue isolation (t.TempDir pinning across swarm/swarmrunner tests) + ACS cycle294 predicates + 2 evals — the goal's priority item |
| 295 | `1c34ce29` | **Checkpoint-preserving `WriteCycleState`** (closes the operator's HIGH inbox defect) + worktree guard + ACS cycle295 predicates |
| 296 | `e25a67d8` | Resume-from-inserted-phase fix (`resume.go` IsValid relaxation) + provision hardening + ACS cycle296 predicates |
| 297 | `f3af4524` | `looppreflight` freeze extended to claude CLI (version-freeze eval) + worktreeBase relative-projectroot guard eval + ACS cycle297 predicates |

Batch cost: $0.00 metered (subscription OAuth). Preflight (looppreflight pkg) validated
both launches in ~30s each — including the agy-tmux driver on its first routed batch.

## Disposition / follow-ups

- **Inbox**: `swarm-tests-relative-worktree-base` (worked by 294, in processing/cycle-294),
  `checkpoint-block-clobbered-by-writecyclestate` (fixed by 295, in processing/cycle-295);
  claude-cli-version-freeze + blank-pane items consumed by the batch.
- **carryoverTodos**: 6 stale P0 entries (cycles 280–291 failure modes: inserted-phase
  worktree dispatch, bridge artifact timeouts) cleared — all demonstrated fixed by the
  4-PASS soak.
- **Branch `cycle-293` kept** (tip `c50c0204`, 1 unique commit: tmux-socket/test-isolation
  work, 503 insertions, never shipped — cycle was reset). Needs explicit
  salvage-or-drop disposition; partially superseded by 294/296/297 ships.
- **Worktrees/branches**: cycles 286–297 worktrees removed, branches deleted (all tips
  on main) except cycle-293.
- **Persona-lint WARNs** at ship (tester/triage/memo/reflector tool+artifact mismatches)
  — pre-existing, non-blocking; candidates for a hygiene cycle.
- **Known WARN**: Darwin nested-sandbox EPERM (host-capabilities preflight) — degrades
  gracefully, expected on this host.

## Release

≥3 consecutive PASS+delivered satisfied (4/3). Released as **v18.6.0** (see
CHANGELOG.md / release-notes; v18.5.0 was tagged 2026-06-11 12:36, before this
soak — its deliverables land in v18.6.0).
