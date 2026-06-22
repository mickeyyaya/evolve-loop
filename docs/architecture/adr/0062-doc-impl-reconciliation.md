# ADR-0062: Documentation ↔ Implementation Reconciliation (2026-06-22 audit)

**Date:** 2026-06-23
**Status:** Accepted
**Deciders:** evolve-loop maintainers
**Related:** ADR-0012 (commit-claim coherence), ADR-0030 (phase observer), ADR-0055 (cycle dossier), ADR-0057 (merge-to-main gate), ADR-0058/0060 (transition kernel)

---

## Context

A full cross-comparison of the design corpus (61 ADRs + ~80 design docs in `docs/architecture/`) against the implementation (119 internal Go packages, ~76K LoC) was run on 2026-06-22 (12-agent fan-out + a 4-agent conceptual deep-dive; per-cluster evidence archived under the audit scratch dir). The implementation was found **sound** — nearly every Accepted ADR's mechanism matches code at high confidence. The defects clustered into three buckets:

1. **A systemic doc-debt event** — the 2026-06-18 script→Go migration deleted `legacy/scripts/` + the root `acs/` tree but left ~25 design docs, several ADR status headers, **one live config manifest** (`.evolve/commit-prefix-scope.json`), and **one code path** (`release --require-preflight`) pointing at the deleted tree.
2. **Genuine functional gaps** — features documented or *policy-declared* as live but wired to nothing (most acutely the ADR-0055 dossier subsystem: built, policy-declared, but never produced — "Potemkin enforcement").
3. **Authoritative-doc contradictions** — `CLAUDE.md` / `runtime-reference.md` name a `.evolve/policy.json` source-of-truth that does not match the checked-in file (defaults are compiled Go defaults).

## Decision

Remediate in two executable tiers (the bulk stale-doc *sweep* is a follow-up; the conceptual deep-dive dispositions are recorded below for that follow-up):

- **Tier 1 — functional fixes** (TDD, behavior-changing): wire the dossier producer + make its policy floor real (T1.1); repoint the commit-prefix manifest at the Go tree (T1.2); fix `release --require-preflight` (T1.3); implement the observer file-never-created grace (T1.4); make the delta-intent decision deterministic (T1.5); resolve the orphaned `docs/private` deny (T1.6).
- **Tier 2 — authoritative-doc truth** (docs): correct the `policy.json` source-of-truth claims (T2.1); correct the cross-family preference-vs-gate framing (T2.2); reconcile default-value contradictions (T2.3).

### Conceptual deep-dive dispositions (Tier-3 follow-up record)

For the ~25 migration-stale docs, each *concept* was judged against the v20 architecture:
- **REVIVE (valuable, never built):** incremental-intent deterministic engine; PASS-cycle reflection signal (`internal/reflection` is built but orphaned); process-supervision meta-audit (audit-constitution's verdict-reasoning grading has no Go successor).
- **REWRITE (concept live, doc stale):** phase-architecture, portable-core, sequential-write-discipline, orchestrator-context-modes, retrospective-pipeline, auto-resume, checkpoint-resume, phase-recovery, phase-observer, intent-phase, research-tool, platform-compatibility, tri-layer, audit-constitution.
- **SUPERSEDE (realized better elsewhere):** phase-tracker, handoff-artifact-schema, abnormal-event-capture, capability-schema, acs-predicate-quality-gate, multi-llm-review, private-context-policy, learn-phase, dynamic-model-routing, inbox-injection-protocol.
- **DROP (obsolete):** context-window-control, fast-fail-counter-scope, `bin/check-caps`.

## Implementation log

Appended per completed slice (the durable change record; `CHANGELOG.md` is release-generated):

- **T1.1 (dossier) — done 2026-06-23.** `policy.Policy.Floor []FloorGate` now parses the `floor` key (was silently dropped); `Policy.FloorEnrolls(id)` query added. `dossier.BuildOpts.FinalVerdict` lets the producer record the real outcome (FAIL synthesizes a truthful defect+carryover). `core.writeCycleDossier`/`dossierVerdict` wired into `RunCycle` after `finalizeCycle` (best-effort + WARN). `evolve dossier verify` now FAILs when the floor enrolls `dossier-closeout` but no dossiers exist (was vacuously OK). See ADR-0055 implementation correction. TDD: policy 2 tests, dossier 4, core 3, cmd 3.
- **T1.3 (release preflight) — done 2026-06-23.** `releasepipeline.defaultFullDryRunPreflight` no longer shells out to the deleted `legacy/scripts/release/full-dry-run.sh` (which made `evolve release --require-preflight` an unconditional hard-fail); it now runs the Go-native preflight (`runPreflightLib`, dry-run + strict, tests deferred to step 1). Regression guard: `TestDefaultFullDryRunPreflight_NoDeadScript`.
- **T1.2 (commit-prefix manifest) — done 2026-06-23.** `.evolve/commit-prefix-scope.json` repointed from the deleted `legacy/scripts/**` + root `acs/**` tree to the Go tree (e.g. `feat(guards)`→`go/internal/guards/**` + `cli/guardcmd/**` + `phases/ship/**` + `go/acs/**`; `feat(routing)`→`router/llmroute/resolvellm`; `feat(audit)`→`acssuite/redteamcheck/evalgate`; `test`→`go/**/*_test.go`). The gate runs inside `evolve ship` (`cmd_ship.go` `--bypass-prefix-gate`), so the dead scopes had made every scoped `feat()/fix()` prefix un-shippable. Regression guard: `commitprefixgate.TestManifestRequiredPaths_ResolveToRealTree` asserts every glob anchors to an existing path. Comment + `_authoritative_as_of` refreshed.

- **T1.7 (purge Go→bash fallbacks) — partial 2026-06-23.** A repo-wide scan confirmed **0 `.sh`/`.py` files** in the product tree (the migration removed the scripts); what remained were Go modules still *referencing/shelling* deleted scripts. Safe removals done (TDD): `consensusdispatch.resolveBashOrNative`→`resolveNativeDispatch` (native-only, errors if no binary; bash fallback gone); `marketplacepoll.DefaultReleaseSh`→Go no-op (never executes a legacy `release.sh`); `cmd_fanout_dispatch.locateCycleStateHelper` removed (last legacy-script reference in the fanout path). **Deferred — need Go reimplementation, not deletion (flagged for a dedicated, careful effort):** (a) `cyclesimulator` default fns shell to deleted `cycle-state.sh`/`ship.sh`/`verify-ledger-chain.sh` and are NOT injected by `evolve cycle-simulator` → the command is currently broken; (b) `subagent/run.go:207` + `validateprofile.go:144` require a `<AdaptersDir>/<cli>.sh` adapter, but **no `.sh` adapters exist** and the live loop dispatches via `bridge.NewDefault` (`cmd_cycle.go:250`), so this is a broken/legacy entry on the core dispatch surface — convert to the bridge or retire only after tracing reachability; (c) `fanoutdispatch.setWorkerStatus` keeps a generic injectable `CycleStateHelperBin` bash-helper seam (tested) — needs a Go-native worker-status writer to fully de-bash. Legitimate `.sh`-as-data handlers (posteditvalidate `bash -n`, router/recon extension classify, ship/verify deny-list) are kept by design.

- **T2.1 (authoritative source-of-truth) — done 2026-06-23.** `CLAUDE.md` (gates/swarm/observer) reworded: defaults are **compiled Go defaults**, surfaced when the policy block is absent; the checked-in `.evolve/policy.json` sets only `floor`+`cli_health` and does NOT contain those keys. `runtime-reference.md` sandbox line fixed (`EVOLVE_SANDBOX` = `auto|on|off`, not `0/1`).
- **T2.2 (cross-family) — done 2026-06-23.** Added a SUPERSEDED/unbuilt banner to `dynamic-model-routing.md`: the 5-layer design + `rc=2` "cross-family invariant enforcement" was never built; cross-family is an advisory preference (`cross_family_with` profile metadata), `router/floor.go` has no family logic, the integrity floor is the sole kernel boundary, and model-level Opus-vs-Sonnet separation (`ADVERSARIAL_AUDIT`) is the live mechanism.
- **T2.3 (default-value contradictions) — done 2026-06-23.** Fixed: `phase-observer.md` stall 240→**600s** (+ noted env→policy migration); `control-flags.md` prose "47 rows"→**35**; `agents/evolve-scout.md` "five"→**six** gates. **Verified-correct (audit false-positives, no edit):** `full-tmux-control.md` `EVOLVE_OBSERVER_NUDGE_S` default 300 IS the effective policy default (`policy.go:406`; the `phaseobserver` struct zero-value 0 is only the unconfigured fallback); `dynamic-phase-routing.md` proposer=haiku (Propose tier) and `WithRouting`-absent→Stage:Off (`router/policy.go:57`) are both accurate.

- **T1.4 (observer file-never-created grace) — resolved by amendment 2026-06-23.** ADR-0030 amended: the 90s `FileNeverCreatedGraceS` (Decision §4) was never built; correctness is covered by the 600s stall (`lastEventTS` init at phase-start, `phaseobserver.go:226`) plus the tmux liveness probe and per-call tmux timeout (newer than ADR-0030). The fast-grace is a deferred latency optimization, intentionally not added to the phase-killer (false-positive risk > marginal benefit).
- **T1.5 (delta-intent determinism) — done 2026-06-23.** The Rule-5 violation (the LLM was asked to compare `goal_hash` to `state.json:currentBatch.goalHash`, `intent.go:64`) is fixed: the intent `hooks` now implements `runner.Skipper.ShouldSkip` (the existing pre-dispatch seam, `runner.go:264`), making the unchanged-goal decision a DETERMINISTIC code comparison (`req.GoalHash` vs `readBatchGoalHash(state.json)`). On match → `SKIPPED`→scout WITHOUT any LLM dispatch; on change → intent runs for delta synthesis; any uncertainty (full mode, blank hash, unreadable state) FAILS OPEN. The prompt no longer asks the LLM to decide unchanged. TDD: 4 tests in `intent_skip_test.go`. The broader deterministic-intent *engine* (5-condition decision tree, patch-merge) remains REVIVE-1.
- **T1.6 (docs/private deny_subpaths) — corrected scope; enforcement deferred to a security-reviewed effort.** Broader than the audit framed it: `sandbox.deny_subpaths` is declared in ~85 profiles but `profiles.Sandbox.DenySubpaths` (profiles.go:96) has **zero production consumers** — the entire sandbox deny-list is unenforced, not just `docs/private`. The secret-redaction half is covered by `panetrust.RedactSecrets` + ADR-0047 and file-isolation by per-cycle worktree isolation, so there is no acute breach, but a security control declared fleet-wide and enforced nowhere is a latent gap. Resolution = ENFORCE `DenySubpaths` in the sandbox/context path as a dedicated change WITH the project-mandated security-reviewer (sandbox code) — not a rushed 85-file edit or hand-wired enforcement under this work stream.

## Consequences

- The dossier closeout invariant is now real going forward; pre-producer history honestly reports FAIL under `evolve dossier verify` (those dossiers never existed) — accurate, not a regression. `verify` is a standalone gate (not in CI/ship), so tightening it cannot destabilize the loop.
- Authoritative docs will state the true source-of-truth (compiled defaults; policy.json overrides), removing a recurring operator/agent misread.
- The Tier-3 REVIVE items are logged as design opportunities, not silently dropped.
