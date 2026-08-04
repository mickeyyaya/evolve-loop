# Fingerprint identity: four generations to an honest breaker

**Period:** 2026-07-26 → 2026-08-04 · **Status:** shipped (similarity-tier follow-up queued)
**Primary artifacts:** commits `6961783e` (#353, the breaker), `b93e56a0` (#358), `bc2e3236`, `550b62d2` (#368), `9a613a50` (#370), `9268836b` (#376) · runtime `.evolve/inbox/consumed/2026-08-04T06-40-00Z-pipeline-defect-pipeline-blocker.json` · runtime `.evolve/runs/cycle-1289/audit-report.md` · [docs/research/failed-loop-analysis-2026-07](../research/failed-loop-analysis-2026-07/README.md)

## Problem

ADR-0072's mid-batch pipeline-blocker breaker (`6961783e`, #353, v22.8.0) halts a batch when an
identical failure fingerprint recurs 3× — on the theory that "identical failure identities cannot
be distinct honest defects." That theory is only as good as the identity. If the fingerprint hashes
**boilerplate**, three *distinct* honest defects collide and the breaker false-halts a healthy
batch (precision failure). If the fingerprint hashes **noise** — durations, cycle numbers — three
recurrences of *one* defect split apart and the breaker never fires (recall failure). Both failure
modes occurred, repeatedly, and each cost either a halted batch or a blind grind. It took four
generations of identity repair to make the breaker honest.

## Context & evidence

The crash-deduplication literature predicted the whole arc. The failed-loop research pass states
it directly: "Stack-hash dedup both over-splits (noise tokens) and over-merges (generic frames) —
the EXACT arc of our fingerprint saga (#368 content-free templates, #370 duration tokens, #376
constant epilogue reasons)" ([failed-loop-analysis §2, citing Igor CCS'21 / ECHO](../research/failed-loop-analysis-2026-07/README.md)).
Fingerprint identity is a precision-recall problem, and every generation below is one side of that
trade-off being paid down.

The incident chain, in order:

- **Cycles 1054/1060 (precursor, `b93e56a0`, #358, v22.8.0):** two *different tasks* failing on
  the verdict path collided on a constant fallback string; "a third would have false-tripped the
  identical-fingerprint breaker." Per-failure distinguishers added.
- **Cycles 1107/1115/1116 (`bc2e3236`, v22.8.0):** three DISTINCT whole-suite contention flakes
  collided into one content-free `red_count=1` fingerprint at the EGPS gate-block and
  false-tripped the breaker — "the 1054/1060 class at the gate-block."
- **Batch-14, cycles 1137/1139/1143 (`550b62d2`, #368, v22.9.0):** the breaker halted on
  `audit|verdict-fail|39ee310f… recurred 3x` — but the three cycles were three distinct,
  *progressing* auditor findings on one task (zero coverage → grace never defaulted → gc.mode=off
  ignored). Every agent-graded FAIL wrote the same content-free router line as its only reason:
  "sha256(boilerplate) == sha256(boilerplate)."
- **Cycles 1146/1148 (`9a613a50`, #370, v22.9.0):** the opposite direction. Two genuinely
  identical EGPS reds differed ONLY in go-test durations (1.478s vs 1.495s) — fingerprints split,
  the identical rule never counted, and the builder ground blind (it also never saw the remedy:
  continuation findings pointed at the identity shell instead of the reason artifact).
- **Batch-19, cycles 1197/1199/1207 (`9268836b`, #376, v22.11.1):** three aborts in phase build
  for *distinct* reasons, but the abnormal-exit epilogue wrote only its constant template — three
  Unexplained digests, one fingerprint, and the breaker "honestly halted batch-19 at cycle-1208
  ('fix the missing reason-writers')."
- **2026-08-04, cycle-1286 (runtime consumed inbox item):** fingerprint `ship|unknown|96f17cfe3dfe`
  recurred 3× — this time the collision was *honest* (three identical GIT_PUSH_REJECTED lane-ship
  failures from one console-merge divergence), but the class tag was still content-free: the
  consumed_by note records "ship-failure fingerprints carry class 'unknown' — they should carry
  the ship error code (GIT_PUSH_REJECTED) so the breaker names the class"
  (`.evolve/inbox/consumed/2026-08-04T06-40-00Z-pipeline-defect-pipeline-blocker.json`, runtime).
- **Cycle-1289 (runtime audit-report.md):** the identity discipline reached a *consumer* of
  fingerprints — the contract-block CLI escalation — and the round-1 audit caught a subset-collapse:
  with exact-equality identity, "a partially-repaired violation set reads as a different defect,
  suppressing the escalation and driving the gate to fail-open" (finding D1, HIGH), and the
  normalizer was "a no-op on the contract gate's actual reason vocabulary" (D2).

## Approaches considered

- **Raise the recurrence ceiling.** Never seriously entertained — it is ADR-0072's rejected
  alternative 4 restated: it treats the count, not the identity, and just multiplies the waste
  before the (still wrong) halt.
- **Weaken or disable the breaker after a false halt.** Rejected each time; every generation
  *narrowed the identity* instead. #368 in fact **deliberately widened** one rule: reasons that are
  empty or pure fallback boilerplate now self-mark `Unexplained` and route to the
  unexplained-failures diagnosability rule — "three content-free failures across DIFFERENT phases
  now halt where nothing halted before … that is a diagnosability breakdown and this is its honest
  rule" (`550b62d2`).
- **Hand-rolled failure-block reader (#368, round 1).** BLOCKed by adversarial review: it dropped
  the placeholder-echo guard and "would have minted a CONSTANT cross-task identity from the printed
  contract example" — the fix would have reintroduced the disease. The landed version reuses
  `phasecontract.ReadFailureBlock`, the existing verdict authority.
- **Semantic test names as identity (bc2e3236, round 1).** Caught by adversarial review: the
  two-part `TestC<cycle>_<Name>` shape leaks cycle numbers, so the same defect re-audited next
  cycle would never collide. The normalizer strips the C-group and index independently, pinned
  against the live ac_id corpus.
- **Fold ALL numeric tokens (#370).** Rejected as overbroad: decimal duration tokens fold, but
  integer config tokens (`-timeout 300s`) stay identity-bearing, pinned by a negative test —
  precision and recall balanced token-class by token-class.
- **Exempt teardown-shaped causes from the identical population (#376 review, HIGH).** Argued and
  rejected, decision pinned with a dedicated test: "one infra condition mowing down N lanes is the
  recurring-infra shape ADR-0072 wants stopped at the ceiling — the marker keeps the halt legible"
  (`9268836b`).
- **Equality-only escalation trigger (cycle-1289).** Rejected in favor of "prior reason known AND
  differing ⇒ suppress": the contract-gate breaker is process-global, so a hot breaker can reach
  the block ceiling on a ladder's FIRST block with no prior reason to compare — equality-only would
  have silently deleted the hot-breaker escape hatch
  ([deliverable-alignment §6.1](../research/deliverable-alignment-2026-08/README.md)).

## Decision & reasoning

The through-line decision, made once per generation and never reversed: **the breaker's rule stays;
the identity gets more honest.** Concretely, identity must be (a) *content-bearing* — carry the
defect head, never a template (bc2e3236, #368); (b) *noise-free* — fold cycle tokens, narrative
verdicts, and durations, the "identity-noise family" (#370 names it as its third member); and
(c) *written at every exit path* — an abort with no reason-writer is itself a defect, surfaced by
the unexplained-failures rule rather than laundered into a fingerprint (#368, #376). The reasoning
for keeping the rule strict comes back to the same place the literature lands: a guardrail that
gates work needs a demonstrably-near-zero false-positive rate — the deliverable-alignment survey
notes SWE-agent's identical shipped-guardrail rule and calls the 1054/1060 breaker lesson
"independently derived" ([§3.4](../research/deliverable-alignment-2026-08/README.md)).

A second decision emerged in #370: identity and *remedy* are different artifacts. The
content-free digest had been doing double duty as the continuation findings source, so attempt 2
repeated attempt 1 byte-for-byte while the actual instruction ("rename the file") sat unread in
`audit-fail-reason.json`. FindingsPath now targets the reason artifact, fenced in the build prompt
as verbatim failure DATA, not instructions.

## Implementation

| Generation | Commit / release | Mechanism |
|---|---|---|
| Precursor | `b93e56a0` #358, v22.8.0 | per-failure distinguishers in the verdict-path fallback reason |
| Gate-block identity | `bc2e3236`, v22.8.0 | EGPS gate-block diagnostics carry cycle-normalized red-predicate identity (`freeTextCycleTokens`-style normalizer strips C-group + index independently); `go/internal/phases/audit/audit_egps_red_identity_test.go` |
| Content-bearing identity | `550b62d2` #368, v22.9.0 | defect-first distinguisher via `phasecontract.ReadFailureBlock` → `defectHead`; `FailureDigest.Unexplained` self-marking + `isUnexplainedDigest` routing; writer/detector templates composed from shared functions pinned by test; 11 new tests; `go/internal/core/failure_digest_identity_test.go` |
| Duration folding + findings | `9a613a50` #370, v22.9.0 | `normalizeReasonForFingerprint` folds decimal duration tokens (integer config tokens pinned identity-bearing); continuation FindingsPath re-aimed at the reason artifact |
| Epilogue cause | `9268836b` #376, v22.11.1 | named `retErr` threaded into the deferred epilogue; `causeHead` projector, cycle-normalized, **tail-kept** truncation ("error chains carry identity at the end"); `teardown=` marker with pinned population decision |

Every generation shipped RED-first with adversarial review (two BLOCK→APPROVE rounds on #368; a
BLOCK→resolved HIGH on #376), per the pipeline-first standing policy. The 2026-08 consumer-side
extension — fingerprint-gated CLI escalation, `contractBlocksShareIdentity` reusing
`normalizeReasonForFingerprint` rather than a second hashing scheme — landed at cycle-1289 with a
three-axis suite (NEGATIVE/SEMANTIC/EDGE, 10/10 PASS) after the round-1 subset-collapse audit
([deliverable-alignment §6.1](../research/deliverable-alignment-2026-08/README.md);
runtime `.evolve/runs/cycle-1289/audit-report.md`).

## Results (measured)

- Each false-halt class stopped recurring after its generation landed: no further
  `red_count=1`-style gate-block collisions after bc2e3236; the batch-14 halt cycles now mint
  three distinct fingerprints (`e13e1f3b`/`82af952e`/`df5a771f`, "live-proven against the halt
  artifacts," `550b62d2`); the 1146/1148 recurrence shape is counted post-#370.
- The breaker's *honest* halts kept working and got legible: batch-19's halt was correct
  (missing reason-writers — a real diagnosability defect), and after #376 "batch-21's halt became
  the first in this project's history readable straight from the digest," producing a queued fix
  item from one line ([change-log-2026-07-30.md §4](../../docs/operations/change-log-2026-07-30.md)).
- The 2026-08-04 cycle-1286 halt collided honestly (one root cause, three lanes) — precision held;
  the residual is class-tag content (`ship|unknown` → carry `GIT_PUSH_REJECTED`), filed as
  `ship-push-only-recovery` 0.8 (runtime consumed inbox item).
- Scorecard from the alignment survey: "Needed 4 generations of fingerprint-identity repair to
  stop false halts; `ship|unknown` class tag still content-free"
  ([deliverable-alignment §2](../research/deliverable-alignment-2026-08/README.md)).

## Retrospective — what we learned

- **Fingerprint identity is a precision-recall dial, not a hash choice.** Over-merge halts healthy
  batches; over-split blinds the breaker. The field's stack-hash dedup literature had already
  mapped both failure modes; reading it earlier would have predicted generations 2–4 from
  generation 1 ([failed-loop-analysis §2](../research/failed-loop-analysis-2026-07/README.md)).
- **The identity-noise family is enumerable:** cycle tokens, narrative verdicts, durations,
  template boilerplate, constant epilogues — each found by a live incident, each now pinned by a
  test. New reason-writers must budget for a normalizer review, not just a writer.
- **Diagnosability is enforceable.** The most counterintuitive move — *widening* the halt surface
  in #368 so content-free failures halt as their own class — converted "the breaker lied" into
  "the reason-writer is missing," which is fixable. Batch-19's halt was that rule working.
- **Identity has consumers, and each consumer re-fights the same battle.** The cycle-1289
  subset-collapse showed a fingerprint *consumer* (escalation gating) re-deriving raw string
  equality and re-importing the fail-open bug; the fix was to share the one normalizer. Single
  primitive, projected everywhere.
- **Open:** the LCS/structural-similarity near-identical band the literature recommends
  (`breaker-fingerprint-similarity-tier`, queued 0.86); ship-failure class tags; and the
  flake-authorship class behind the gate-block collisions (`acs-metapredicate-suite-scope`,
  queued from bc2e3236).

## Links

- [ADR-0072](../../docs/architecture/adr/0072-system-failure-policy-and-halt.md) (the breaker is its mid-batch extension, #353)
- [False-FAIL storm entry](2026-07-false-fail-storm.md) — where the halt floor came from
- [LLM output stability entry](2026-07-llm-output-stability.md) — the contract-gate breaker's fail-open ratchet, the escalation this identity work gates
- [change-log-2026-07-30.md](../../docs/operations/change-log-2026-07-30.md) §4 (epilogue cause, in reality)
- Research: [failed-loop-analysis-2026-07](../research/failed-loop-analysis-2026-07/README.md) (crash-dedup mapping) ·
  [deliverable-alignment-2026-08](../research/deliverable-alignment-2026-08/README.md) (§2 scorecard, §6.1 cycle-1289 landing)
- Runtime evidence (read-only): `.evolve/inbox/consumed/2026-08-04T06-40-00Z-pipeline-defect-pipeline-blocker.json` · `.evolve/runs/cycle-1289/`
