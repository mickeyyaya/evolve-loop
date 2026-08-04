# Deliverable alignment strategy: the four-layer model
**Period:** 2026-08-04 (research + queue reprioritization; rank-1 landing same day) · **Status:** shipped (research); portfolio in-flight
**Primary artifacts:** `docs/research/deliverable-alignment-2026-08/README.md` (#409, `8c5f4285`) · queue commit `629ba575` · ship `0c7500c3` (rank-1 landing, §6.1)

*This entry is a digest and index — the research doc is the primary source and
holds the full inventory, citations, and ranked designs. Its value here is the
narrative: what question was asked, what the answer was, and what moved.*

## Problem

The operator asked a strategy question, not a bug report: *"determine if there
are more effective AI architecture solutions for aligning deliverables across
our LLM CLI … I'm not sure if the current 'find one, solve one' approach
works."* The context: four heterogeneous *interactive* CLIs driven through
tmux, no decoder access, deliverables as terminal text plus files on disk — and
a week in which the same alignment classes kept resurfacing in new costumes.

## Context & evidence

Two parallel research tracks, then fusion (doc header): (1) a complete survey
of the repo's own alignment mechanisms and their **measured** results (docs,
kb, ADRs, live runtime state, incident reports); (2) online state-of-the-art
2024–2026 on schema-conformant agent deliverables under exactly this
constraint profile. Sources cited inline in the doc; the local track leans on
the batch integrity review
([2026-08-batch-integrity-review.md](2026-08-batch-integrity-review.md)), the
862–899 false-FAIL storm doc, and the llm-output-stability research.

## Approaches considered

The strategy alternatives the research weighed, with the rejected ones named
in the doc's closing list:

- **Keep find-one-solve-one everywhere** — the operator's doubt; evaluated
  against the measured record rather than dismissed (verdict below).
- **Decoder-level constrained decoding on hosted CLIs** — explicitly not
  recommended: unreachable on tmux-driven CLI surfaces, and constraint-tax risk
  on reasoning ("constrain the envelope, never the reasoning" is the settled
  rule; harness-side validation of free-form work pays zero tax — which is
  already this architecture).
- **Unbounded repair loops** — rejected: dominated by resampling past two
  rounds (repair-economics literature; "escalate the auditor, not the
  generator").
- **A generic "more predicates" push** — rejected: the predicate layer is
  already clean under mutation testing; its waste is contention flakes, a
  different problem.
- **Single-model self-certification anywhere** — rejected; industry consensus
  for this constraint profile is contracts in the harness type system, model
  never trusted to self-certify.

## Decision & reasoning

**The verdict on find-one-solve-one:** *"it is the right verification posture
and the wrong prevention strategy"* (doc §1). Where it works, measurably: all 7
of the prior week's closed failure classes stayed closed; the adversarial audit
holds the best catch-rate per unit deployed in the system; and the
continuation-defect-ledger grind — 5 rounds, 4 distinct real defects, then a
PASS — is find-one-solve-one operating *as a hardening crucible*
([2026-08-continuation-defect-ledger.md](2026-08-continuation-defect-ledger.md)).
Where it fails, measurably: class recurrence in costumes (scope disease needed
four incidents before a class mechanism was attempted — and that mechanism is
dormant and structurally blind to two of the four costumes); fix latency as a
failure mode of its own (the contract-block escalation item stranded for two
batches while its class recurred twice —
[2026-08-contract-block-escalation.md](2026-08-contract-block-escalation.md));
and the frontier moving above the artifact layer (the integrity review found
the code clean and the gaming in status accounting, a layer with no contracts
at all until §3.8/§3.9).

**The four-layer model** (doc §4) assigns each failure class its layer:

| Layer | Owns | State |
|---|---|---|
| L1 Generation-point | format/placement classes | ADR-0034 + contract tail — correct and near-ceiling; remaining lever is per-CLI (Claude hooks, Ollama grammar) |
| L2 Transport & salvage | harness-side losses (~65% of failures), recoverable-malformed output | file-authoritative verdicts shipped; **schema-aligned salvage layer missing — highest-leverage gap** |
| L3 Verification | semantic wrongness, gaming, vacuous work | EGPS + adversarial audit, best-in-system; extend with the weak-verifier invariant stack; every gate needs FP≈0 evidence |
| L4 Process/accounting supervision | cross-cycle laundering, status fiction, fix latency | the open frontier; ledger + §3.8/§3.9 are its first mechanization |

The synthesis: find-one-solve-one is L3/L4's *operating mode* — correct there.
The operator's instinct is right about L1/L2: those layers should be
class-preventive by construction, and each new instance found there signals the
layer needs a mechanism, not another patch.

## Implementation

The research shipped as `docs/research/deliverable-alignment-2026-08/README.md`
(#409), and its ranked 7-move portfolio (doc §5) was filed to the **live**
queue in the same breath (commit `629ba575`): contract-block-cli-escalation
0.96 (rank 1 — refiled, because the original existed only in the tracked
snapshot: a live instance of the stranding it documents),
schema-aligned-salvage-layer 0.9 (rank 2), crossartifact-invariant-stack 0.85
(rank 5), claude-stop-hook-finish-gate 0.8 (rank 6); two-stage verdict minting
(rank 3) folds into the existing verdict-sentinel-as-tool-call 0.86; retry
ladder economics (rank 4) folds into rank 1's landing; rank 7
(accounting-layer mechanization) was already in flight. §6 fixes the baselines
future landings must measure against and obliges each landing to append its
issue/gap/solution record per operating-policy §3.8.

## Results (measured)

The measured record the verdict rests on (doc §2):

- **~65% of alignment failures were harness defects, not agent defiance** —
  and the single largest loss event was the harness discarding verified-green
  work; transport robustness has paid better than any agent-facing constraint.
- **Contract tail took `bad_verdict` to zero** (batch-19+), but saturates at
  model capability: agy ignored the identical contract 7/7.
- **The breaker is a fail-open ratchet under weak CLIs**: 3 live CIRCUIT OPEN
  firings, all on weak-CLI phases — a non-compliant CLI demotes
  enforce→advisory.
- **The code layer is clean under live mutation testing** (2026-08-04 review):
  zero tautological predicates, zero tampering, all 7 FAILs honest.

What already landed from the portfolio: **rank 1** — the identity-gated CLI
escalation, landed same day via ship `0c7500c3` with its §6.1 landing record
(10/10 tests PASS; live firing counts still to be measured against the
3-firings baseline); **rank 7** — the continuation-defect-ledger, landed
through the 1279→1292 lineage. Ranks 2/5/6 queued; recoverable-malformed rate
is "not yet instrumented — the salvage layer's first deliverable is the
measurement" (doc §6).

## Retrospective — what we learned

- **Name the layer before fixing the instance.** The repo had already built
  the right mechanism for each layer at least once — what was missing was the
  map saying which failures deserve a mechanism and which deserve a patch.
- **A strategy review can produce falsifiable output.** The portfolio shipped
  with weights, named risks, baselines, and a measurement obligation — not
  recommendations prose.
- **The research validated itself in passing.** Its rank-1 item turned out to
  be missing from the live queue — the exact stranding pathology it was
  documenting — and its own landing record misattributes the ledger landing to
  a lane number (cycle-1286, where the dossiers show the green landing as
  cycle-1287), a small live specimen of the §3.9 label-vs-diff rule.
- **Open question:** whether L2's salvage layer actually converts 8–15%
  malformed calls into saves on these CLIs — the first deliverable is the
  instrument, not the claim.

## Links

- Primary source: `docs/research/deliverable-alignment-2026-08/README.md`
- Policy hooks: `docs/operations/operating-policy.md` §3.8/§3.9
- Sibling entries: [2026-08-batch-integrity-review.md](2026-08-batch-integrity-review.md) ·
  [2026-08-continuation-defect-ledger.md](2026-08-continuation-defect-ledger.md) ·
  [2026-08-contract-block-escalation.md](2026-08-contract-block-escalation.md) ·
  [2026-07-llm-output-stability.md](2026-07-llm-output-stability.md) ·
  [2026-08-scope-disease.md](2026-08-scope-disease.md)
