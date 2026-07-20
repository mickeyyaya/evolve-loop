# False-FAIL blast-radius audit & recovery ledger (cycles 862–899)

**Status:** CLOSED (cycle-986) — prevention SHIPPED (`ad446a76`, ADR-0072 floor) **and** all
three false-FAILed features recovered + landed on main: tier-fallback via `6b4e4096` (PR #331)
and skill-overlay via `daf993e8` (PR #333), scoped-review in `internal/core`. The genuine FAILs
(889/894/895/896) remain classified do-**NOT**-land (see Disposition). Stale recovery inbox item
`recover-false-fail-features-876-897-898` retired the same cycle to end the livelock.
**Date:** 2026-07-17 (opened) → 2026-07-21 (CLOSED).

## What happened

The clean-exit deliverable-authority bug (root-caused + fixed in `38b961d2`) caused the
runner to synthesize a **FAIL** verdict from contaminated tmux scrollback whenever a
phase agent exited clean-and-idle — even though the on-disk `audit-report.md` verdict was
**PASS** and `acs-verdict.json` was **PASS**. The loop treated each occurrence as an
ordinary task-level audit FAIL: it released the inbox items back to `.evolve/inbox/` root
with no memory of *why* they failed, triage re-selected the same task next cycle, and the
task produced the same forged FAIL. Over cycles **862→899** this discarded verified-green
work repeatedly and re-attempted the same features cycle after cycle.

## Classification method

For each cycle in the range: compare the **recorded** outcome against the cycle's own
**on-disk artifacts** (`audit-report.md` evolve-verdict + `acs-verdict.json` verdict) and
whether retro ran (retro fires only on FAIL/WARN).

- **audit=PASS + acs=PASS + recorded-FAIL** = false-FAIL (the forgery signature).
- **audit=FAIL on disk** = genuine failure (the auditor itself wrote FAIL) — correctly rejected.

## Disposition

| Cycle(s) | On-disk verdict | Feature | Disposition |
|---|---|---|---|
| 862, 870, 884, 888, 897 | PASS (884 WARN) | **Overlay / `/evo:fable` skill-injection** | FALSE-FAIL → recover (best impl: **897**; 884 flags a security gate: `skills/fable/` must join `ProtectedSurfaceManifest` via human-gated manual ship) |
| 876 | PASS | **Tier-fallback (fable→opus on quota)** | FALSE-FAIL → recover. `TierChain`+`DispatchTiered`+`Plan.Tiers`; applies cleanly; PASS+ACS 7/7. **INERT** — ships a deliberately-RED reproduction test proving the production call-site swap (runner `Dispatch`→`DispatchTiered`) is NOT done. Landing it = the operator's urgent fable→opus fallback |
| 866, 877, 883, 898 | PASS | **Scoped-review (audit scope to touched pkgs)** | FALSE-FAIL → recover (best PASS: **898**) |
| 882 | PASS | Fleet partition | Already on main except its ACS predicate test — effectively landed |
| 899 | PASS | MergeFallbacks (preserve operator tier_fallbacks on `models refresh`) | ✅ Already recovered in `38b961d2` |
| 889, 894, 895, 896 | **FAIL** | Scoped-review / mergehunks iterations | GENUINE FAIL (real zero-coverage / live-reproduced defect / cross-lane contamination) — correctly rejected; do **NOT** land as-is |

**Net:** 10 cycles false-FAILed; they cluster into **3 distinct features absent from main**
(overlay-skill-injection, tier-fallback, scoped-review), each built + verified-green but
never landed. The loop re-attempted overlay-skill-injection 5×, tier-fallback 1×, and
scoped-review 8× (4 false-FAIL + 4 genuine-FAIL iterations), burning tokens for the same
result — the textbook livelock ADR-0072 exists to end.

## Where the work survives

The deliverables were left as **uncommitted** working changes in the per-cycle git
worktrees `.evolve/worktrees/cycle-21f9f7ae-{876,884,897,898,...}` (branch HEAD == base
SHA; no commit on top). They are durable while the worktrees exist but are NOT git-committed.
To harvest a deliverable: `cd <worktree> && git add -A && git diff --cached`.

## Recovery plan — CLOSED (all landed cycle ≤986)

The recovery landed the **best verified implementation of each distinct feature** through the
normal pipeline (TDD + audit + ship) — **not** a blind stack of overlapping old diffs. Final
disposition (verified green this cycle across runner/bridge/guards/core):

1. **tier-fallback (876)** — ✅ LANDED `6b4e4096` (PR #331): the production call-site swap
   (runner `Dispatch`→`DispatchTiered`, `runner.go:704`) is done, so fable/opus→sonnet failover
   is LIVE on exit 85 (no longer inert; reproduction test passes).
2. **skill-overlay / `/evo:fable` injection (897, 884)** — ✅ LANDED `daf993e8` (PR #333):
   config-driven overlay injection for phase agents; `skills/fable/` is in
   `ProtectedSurfaceManifest` (`guards/integrity_surface.go`), closing the cycle-884 security gate.
3. **scoped-review (898)** — ✅ LANDED in `internal/core` (`composition_scoped_review.go`) with
   the real coverage the 889/894/895/896 iterations lacked.

The genuine FAILs **889/894/895/896** are NOT recovered — they remain do-**NOT**-land per the
Disposition table (real zero-coverage / live-reproduced / cross-lane-contamination defects).
The stale inbox item `recover-false-fail-features-876-897-898` (weight 0.93) was retired this
cycle so triage stops re-selecting completed work.

## Prevention (shipped)

ADR-0072's deterministic floor (`ad446a76`) makes this class self-arresting: a forged
verdict now HALTS the loop (`stop_reason=system_failure_halt`) with an escalation dossier +
P0 pipeline-repair item, instead of silently re-attempting the task. It would have stopped
this storm at **cycle 862** instead of 899. See
[docs/architecture/adr/0072-system-failure-policy-and-halt.md](../architecture/adr/0072-system-failure-policy-and-halt.md).
