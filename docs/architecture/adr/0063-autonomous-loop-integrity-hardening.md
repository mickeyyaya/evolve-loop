# ADR-0063: Autonomous-loop integrity hardening — bridge resilience + the deterministic gate ladder

Status: Accepted
Date: 2026-06-23
Relates to: ADR-0061 (live-feature-flag gate), ADR-0030 (phase observer), ADR-0039 (failure floor), ADR-0042 (EGPS Go-native). Establishes the trust model the flag-reduction campaign (`no_feature_flags`) runs the unattended loop under.
Campaign: flag-campaign-10.

## Context

The flag-reduction campaign runs the autonomous evolve loop unattended for many cycles. flag-campaign-10's first cycles exposed that **"audit PASS" conflated *"the LLM auditor looked at it"* with *"the change is correct."*** Three distinct escapes shipped a green verdict over defective work:

1. **Wedge (no liveness).** A wedged tmux server's `capture-pane` `read()` blocked forever; the bridge wait loop (`driver_tmux_repl.go`) only checked `ctx.Err()` *between* iterations, so the completion-poll, stop-review, and ctx-cancellation all froze. A cycle hung past its 2h deadline at 0% CPU and never reaped. Separately, agent REPLs ran on the **shared default** tmux socket, so a stray `tmux attach` (an operator "show progress" peek) leaked keystrokes into a live agent REPL.

2. **Fake progress.** Relaunch cycle 10 (`w2-phaserecovery-ipc`) renamed a reader to a `"EVOLVE_"+"X"` split-const, passed tests + audit + adversarial-review, and shipped a **PASS with zero net registry-row deletions** (35→35). The existing `flagceiling` guard (ADR-0061) only blocks the live count from *rising*; nothing required it to *fall*.

3. **Broken toolchain.** A cycle shipped a `go vet` failure that was *also* a real semantic bug — `string(StageEnforce)` = `string(3)` = `"\x03"`, which `channel.ResolveStage` maps to "off", **silently disabling recovery enforcement** — plus a failing unit test. The deterministic `buildSelfCheck` *detected* it (it runs `go test`, which runs `go vet`, on changed packages) but was best-effort/never-aborts; the LLM auditor missed it.

The root pattern: the trust boundary that mattered (the EGPS/audit chokepoint) ran the ACS predicate suite but **not the standard toolchain**, and **no gate enforced the campaign metric itself**. Verdict reliability depended on the LLM auditor noticing mechanical properties it cannot reliably check.

## Decision

Make a green verdict *deterministically* trustworthy, via two complementary changes.

### A. Bridge resilience — the loop survives

- **Per-call tmux timeout** — `bridge.runCmdBounded(ctx, 30s, …)` bounds every tmux subprocess call, so a wedged server returns a `DeadlineExceeded` error within 30s instead of blocking the wait loop forever. Restores ctx-cancellation + per-cycle-deadline liveness. (#209, `ab065345`)
- **Dedicated tmux socket** — `bridge.TmuxSocket = "evolve-bridge"`; `TmuxSocketArgs` prepends `-L evolve-bridge` so agent panes run on an isolated socket that operator interaction with the default socket can never reach. All consumers routed through it: `execTmux`, `swarm.ExecTmuxKill` (covers `sessionreaper`), the observer probe (`socketTmuxRunner`). (#210, `26d5ecf1`)

### B. The deterministic gate ladder — the verdict is honest

Three deterministic gates run at the audit/EGPS chokepoint (`acssuite.Run`, host-side, *before* ship — see `phases/audit/audit.go`). Each is a `red_count` contributor: failing one fails audit, independent of the LLM.

1. **`flagceiling`** (ADR-0061, `go/acs/regression/flagceiling`): the live operator-flag count may not *rise* versus the `main` baseline.
2. **`flagprogress`** (#212, `go/acs/regression/flagprogress`): during an active campaign (`EVOLVE_FLAG_CAMPAIGN=1`), `len(flagregistry.All)` at the working tree must be **strictly less** than at HEAD (the cycle's parent — audit runs before ship commits, so the working tree carries the deletion while HEAD is the parent). A cycle that nets zero deletions cannot ship. This gates the *metric*, which a cosmetic refactor cannot fake.
3. **`buildselfcheck` enforcement** (#214, `go/acs/regression/buildselfcheck`): the deterministic post-build self-check (`core.buildSelfCheck`, which already ran `go test` — hence `go vet` — on every changed package) is now a hard `red_count` gate via the `.evolve/build-selfcheck.json` artifact. `buildSelfCheck` clears the artifact at the start of every build so a passing retry can't inherit a stale failure. A cycle whose changed packages don't build/vet/test green cannot ship.

## Consequences

- For the flag campaign, **audit PASS ⟹ correct**: the change reduces the registry (flagprogress), builds/vets/tests green (buildselfcheck), and doesn't regress the live count (flagceiling) — all deterministically, not at the LLM's discretion. Verified reductions are confirmed in seconds, not forensically re-audited.
- The gates are **dormant outside their context** (flagprogress skips without `EVOLVE_FLAG_CAMPAIGN=1`; the buildselfcheck artifact is absent when no cycle ran), so normal main / dev / CI runs are unaffected.
- **Known limits — review + CI remain the backstops for what gates structurally cannot test:**
  - *Semantic cross-process contracts.* A reader renamed to a split-const IPC key that nothing injects (`EVOLVE_PHASE_RECOVERY_STAGE`) silently disabled the manual `evolve phase-observer` path. No deterministic gate can exercise an operator-set env on a standalone subprocess; **review on the integration PR** caught it (fixed + behavioral test added, #216).
  - *Per-symbol apicover coverage.* A new exported symbol whose only test lives in another package passes `go test` but fails `apicover -enforce`. **CI apicover-enforce** caught it (fixed with same-package tests, #217 follow-up).
- **Generalizable principle (the session's through-line):** *any check that lives only in CI lets an autonomous loop ship the gap it covers.* The durable fix is to move mechanical-correctness checks into the per-cycle deterministic gate (the audit chokepoint), and reserve human/LLM review for the semantic contracts the gates cannot test. Folding `apicover -enforce` + `golangci-lint unused` into `buildselfcheck` is the next step in this direction (tracked).

See `knowledge-base/research/flag-campaign-10-hardening-and-learnings-2026-06-23.md` for the operational narrative, the integration mechanic, the under-delivery → convergence-pass strategy, and the recurring gotchas.
