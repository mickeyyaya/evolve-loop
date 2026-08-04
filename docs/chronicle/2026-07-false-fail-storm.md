# False-FAIL storm 862–899 and file-authoritative verdicts

**Period:** 2026-07-16 → 2026-07-21 (storm + prevention + recovery; follow-on family closed 2026-07-20) · **Status:** shipped
**Primary artifacts:** [docs/operations/false-fail-recovery-862-899.md](../../docs/operations/false-fail-recovery-862-899.md) · [ADR-0072](../../docs/architecture/adr/0072-system-failure-policy-and-halt.md) · commits `38b961d2`, `94252425` (#336), `a87046e8` (#335), `ad446a76`, `8e2afef0`, `3c5ed711` · CHANGELOG v22.3.0–v22.4.2

## Problem

The harness itself forged failure verdicts. The clean-exit deliverable-authority bug caused the
runner to synthesize a **FAIL** from contaminated tmux scrollback whenever a phase agent exited
clean-and-idle — even though the on-disk `audit-report.md` verdict was **PASS** and
`acs-verdict.json` was **PASS** ([false-fail-recovery-862-899.md](../../docs/operations/false-fail-recovery-862-899.md)).
The blast radius was not one cycle but a livelock spanning cycles **862→899**: each false FAIL
released the inbox item back to the queue root with no memory of why it failed, triage re-selected
the same task next cycle, and the task produced the same forged FAIL. Net damage, from the recovery
ledger: **10 cycles false-FAILed**, clustering into **3 distinct verified-green features that never
reached main** — overlay-skill-injection re-attempted **5×**, tier-fallback **1×**, scoped-review
**8×** (4 false-FAIL + 4 genuine-FAIL iterations) — verified work discarded every time, tokens
burned for an identical outcome.

## Context & evidence

- **The forgery signature.** The recovery ledger's classification method: for each cycle, compare
  the recorded outcome against the cycle's own on-disk artifacts. `audit=PASS + acs=PASS +
  recorded-FAIL` = false-FAIL; `audit=FAIL on disk` = genuine. By that test, cycles 862, 866, 870,
  876, 877, 882, 883, 884 (WARN), 888, 897, 898, 899 carried green artifacts under a recorded FAIL,
  while 889/894/895/896 were genuine FAILs the auditor itself wrote
  ([Disposition table](../../docs/operations/false-fail-recovery-862-899.md)).
- **Why the breaker didn't save us.** ADR-0072's context section: in fluent mode (`strict_audit`
  off) the failure-adapter's BLOCK rules degrade to PROCEED, and the consecutive-fail breaker's
  default is bypassed by the fleet/resume paths that re-triage released items. The loop had **no
  progress monovariant** — it could repeat work indefinitely without checking whether the outcome
  was changing ([ADR-0072 §Context](../../docs/architecture/adr/0072-system-failure-policy-and-halt.md)).
- **A sibling bug the week before.** Cycle-859's false-FAIL was a ctx-cancel SIGKILL (exit −1)
  hard-failing via the substantive-error door instead of consulting the green-ACS PASS deliverable;
  fixed separately (CHANGELOG v22.3.0, "deliverable-authority ctx-cancel -1 false-FAIL
  (cycle-859)"). The storm proved the class was wider than that one path.
- **Where the discarded work survived.** Uncommitted working changes in the per-cycle worktrees
  `.evolve/worktrees/cycle-21f9f7ae-{876,884,897,898,…}` — durable while the worktrees existed, but
  not git-committed ([recovery ledger, "Where the work survives"](../../docs/operations/false-fail-recovery-862-899.md)).

## Approaches considered

ADR-0072 records four alternatives, all real:

1. **Smarter deterministic retry only (no orchestrator judgment).** Rejected: a pure classifier
   reading the recorded verdict is fooled by a forged verdict — "a broken pipeline can lie about
   whose fault it is." Deterministic logic was kept only as floor + fallback.
2. **Orchestrator full authority, no Go floor.** Rejected by the operator: a mis-classification
   could still let a system failure be retried.
3. **Quarantine the poison task, keep looping.** Insufficient for system-level failures: a broken
   pipeline poisons *all* tasks, not one. Quarantine was kept for task-level repetition only.
4. **Raise the consecutive-fail breaker threshold.** Treats the symptom (count), not the cause
   (incoherence); still repeats N times before stopping. The coherence floor stops at N=1.

A fifth "approach" was the one actually running for 38 cycles: **believing the recorded verdict**.
The follow-on family made its cost explicit — for the 930/931/932 false-HALTs, "three prior LLM
retros pattern-matched this to an agent-stall story; the artifact fingerprints refute it (the
sessions completed cleanly; the 'idle spinner' was the runner running its own gates)"
(commit `8e2afef0`). Retros reading the same poisoned surface reproduced the same wrong diagnosis.

## Decision & reasoning

Three mutually reinforcing decisions:

1. **File-authoritative verdicts — "pane never a verdict source."** For any contracted phase, the
   verdict authority is the on-disk artifact, never tmux scrollback (commit `94252425`, #336,
   v22.4.1). The pane is a transport that can be contaminated by anything an agent prints; the
   artifact is what the phase actually attested.
2. **Settle-retry the artifact, and widen the window.** The clean-exit path settle-retries the
   on-disk verdict before concluding failure (`38b961d2`, v22.3.0); the window was widened 3→15
   when cycle-921 showed late-flushed deliverables still losing the race (`a87046e8`, #335,
   v22.4.1).
3. **ADR-0072: classify, justify, halt.** The architectural fix. Two missing properties were named:
   **(P1) coherence** — a recorded verdict is only trustworthy if it agrees with the artifacts the
   phases wrote — and **(P2) termination** — the loop may only repeat work after a coherent,
   progressing, task-level result. A three-layer split (declarative policy / orchestrator judgment /
   Go-enforced floor) with two categories that ALWAYS halt regardless of what the orchestrator
   says: `verdict-incoherence` and `infra-systemic`. Named trade-off, accepted deliberately: an
   autonomous overnight run now halts on a system failure instead of grinding — "better to
   halt-and-wait than burn tokens repeating" ([ADR-0072 §Consequences](../../docs/architecture/adr/0072-system-failure-policy-and-halt.md)).

## Implementation

- **v22.3.0 (2026-07-17):** clean-exit-idle deliverable-authority fix + cycle-899 MergeFallbacks
  recovery (`38b961d2`); ADR-0072 deterministic floor, slices S1–S3+S6 (`ad446a76` graduated
  `internal/coherence`; `CheckVerdictCoherence`, `failure-dossier.json`,
  `stop_reason=system_failure_halt`, auto-filed P0 `pipeline-defect-*` item). CHANGELOG v22.3.0.
- **v22.4.1 (2026-07-19):** file-authoritative verdict source (#336) + settle-retry window 3→15
  (#335). CHANGELOG v22.4.1.
- **Recovery (closed cycle-986, 2026-07-21):** the best verified implementation of each feature was
  landed through the normal pipeline — not a blind stack of old diffs: tier-fallback `6b4e4096`
  (PR #331, incl. the production call-site swap `Dispatch`→`DispatchTiered` at `runner.go:704` that
  the cycle-876 build had deliberately proven missing with a RED reproduction test); skill-overlay
  `daf993e8` (PR #333, with `skills/fable/` added to `ProtectedSurfaceManifest` per the cycle-884
  security gate); scoped-review in `internal/core` (`composition_scoped_review.go`). The genuine
  FAILs 889/894/895/896 were **not** landed. The stale recovery inbox item was retired the same
  cycle to end a second-order livelock of re-selecting completed recovery work
  ([recovery ledger §Recovery plan](../../docs/operations/false-fail-recovery-862-899.md)).
- **Follow-on family (v22.4.2 + successor):** the floor's first weeks produced false HALTs of its
  own — cycles 930/931/932: the audit agent wrote PASS + ship-eligible ACS, then the runner's
  CI-parity integration-tier gate false-REDded under fleet contention, and
  `detectVerdictIncoherence` never populated `SubstantiveError` (zero value at the sole call site),
  so a legitimately-diagnosed FAIL with green artifacts was labeled "pipeline-forged" and halted
  the batch — "the maximal-cost outcome for a flaky gate." Fixed in `8e2afef0` (v22.4.2):
  diagnosed downgrades stamp `CycleState.AuditFailReasons` at the shared record chokepoint and the
  floor reads ONLY that field (no agent-writable file can talk the floor out of halting);
  integration tier scoped away from env-exclusive packages. Residual flakes 943/950/955 closed by
  `3c5ed711`: CI-parity env scrub + serialized retake-on-red under a cross-lane flock.

## Results (measured)

- **Storm arithmetic:** 10 false-FAILed cycles across a 38-cycle span; 3 green features discarded
  and later recovered; re-attempt counts 5×/1×/8×. All three features landed on main by cycle-986
  ([recovery ledger](../../docs/operations/false-fail-recovery-862-899.md)).
- **Counterfactual, stated by both the ADR and the ledger:** the floor "would have stopped this
  storm at **cycle 862** instead of 899" — on the first forged verdict.
- **The floor works both ways:** it has since halted real pipeline defects (see
  [the fingerprint entry](2026-07-fingerprint-identity.md) — batch-14, batch-19, cycle-1286), and
  the later deliverable-alignment survey scores it "would have stopped the 862 storm at cycle 862
  … halted 2 real pipeline defects this week"
  ([docs/research/deliverable-alignment-2026-08](../research/deliverable-alignment-2026-08/README.md), §2).
- **The false-HALT family closed:** after `8e2afef0` + `3c5ed711`, tier false-REDs are absorbed as
  visible WARNs (red-then-green) or genuine FAILs with real offenders (red-then-red); 4 regression
  tests pin the behavior (commit `3c5ed711`).

## Retrospective — what we learned

- **A pane is a transport, not a testimony.** Any verdict surface an agent can print into is
  forgeable by accident. "Pane never a verdict source" generalized into the transport layer of the
  later four-layer alignment model, where the survey's sharpest line lands: "the worst incidents
  were the transport's own … the harness forging FAILs, not agents misbehaving"
  ([deliverable-alignment §2](../research/deliverable-alignment-2026-08/README.md)).
- **Coherence must be computed, not assumed.** Comparing the recorded verdict against independent
  on-disk artifacts is a pure function (`CheckVerdictCoherence`) — cheap, deterministic, and the
  only thing that catches a lying pipeline.
- **Fingerprint artifacts before believing retros.** Three LLM retros in a row reproduced the same
  refuted stall narrative for 930/931/932 (`8e2afef0`). Diagnosis starts at `state.json` and the
  run dir, not at the previous retro's story.
- **A halt floor needs its own hardening.** The floor traded a 38-cycle livelock class for a
  false-HALT class (930/931/932), which then needed two more commits and — one layer up — the
  entire fingerprint-identity campaign ([sibling entry](2026-07-fingerprint-identity.md)). Every
  new guardrail is a new thing that can false-positive; budget for that from day one.
- **Open at close:** builder-side self-checks remained outside the verification lock, and the
  quarantine/accounting half of ADR-0072 (S5) later proved unreachable in wave mode — picked up in
  [docs/operations/change-log-2026-07-30.md](../../docs/operations/change-log-2026-07-30.md) §2.

## Links

- [ADR-0072 — system-failure policy and halt](../../docs/architecture/adr/0072-system-failure-policy-and-halt.md)
- [False-FAIL blast-radius audit & recovery ledger](../../docs/operations/false-fail-recovery-862-899.md)
- CHANGELOG: v22.3.0, v22.4.0, v22.4.1, v22.4.2 ([CHANGELOG.md](../../CHANGELOG.md))
- Sibling entries: [Fingerprint identity](2026-07-fingerprint-identity.md) ·
  [Quota-detection regex drift](2026-07-quota-regex-drift.md) (the other false-FAIL asymmetry) ·
  [LLM output stability](2026-07-llm-output-stability.md)
- Research: [deliverable-alignment-2026-08](../research/deliverable-alignment-2026-08/README.md) (L2 transport layer) ·
  [llm-output-stability-2026-07](../research/llm-output-stability-2026-07/README.md) (~65% harness-defect finding)
