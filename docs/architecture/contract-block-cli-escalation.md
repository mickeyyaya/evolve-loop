# Contract-Block CLI Escalation

**Status:** live · **Code:** `go/internal/core/contract_escalation.go`, `go/internal/core/cyclerun_review.go`
· **Tests:** `go/internal/core/contract_escalation_test.go`, `go/acs/cycle1291/predicates_test.go`

**Retry ladder:** at `workflow.retry_ladder.stage` `enforce`, the retry ladder replaces this escalation and the same-CLI salvage retry ([retry-ladder §5](retry-ladder.md#5-where-the-ladder-runs)). At `shadow`, this escalation runs as before. The escalation step is `escalateContractCLI` in `go/internal/core/cyclerun_correction.go`.

## The defect this closes

The correction ladder in `reviewAndGuard` re-dispatched the **same** profile CLI after a
deliverable-contract block. A `cli_fallback` chain of a profile fires only on infra exit codes
`{80,81,85,87,124,127}`, never on a contract violation. So a CLI that systematically mis-formats a
deliverable burns every correction. Then the contract-gate breaker opens and demotes `enforce→advisory`
for the rest of the run. Batch-19 (cycles 1171/1172) and batch-21 (cycle-1215) both ended that way:
a FORMAT-compliance failure silently WEAKENED a gate. The correct escape hatch is CLI escalation,
not gate demotion.

## Trigger

A correction re-dispatch escalates to a different CLI **family** when both hold:

1. `ReviewResult.Blocks >= contractEscalateAtBlock` (2). This is the consecutive-block counter of the contract gate.
   The same counter opens the circuit at 3. Never use a locally re-counted correction
   ordinal. It desyncs when a prior cycle left the breaker hot or the salvage rung consumed a
   block.
2. `contractBlocksShareIdentity(prev, reason)` — the block is the **same defect** as the one that
   triggered the previous correction. `Blocks` counts blocks, not defects; two honest violations are
   not one incapable CLI.

`Blocks == 0` (evalgate / topngate / triagecap / the build floor) never escalates: those are
task-binding or capacity rejections, and a different CLI is not the remedy.

## The breaker-exempt class (ADR-0124)

An optional evaluate phase past the ship floor, which the routing config does not list as mandatory
(`config.mandatory_phases`), is **breaker-exempt** (`ReviewInput.BreakerExempt`). Code-review is the
one such phase today. Its blocks are never counted. Its passes never reset the breaker. It never
demotes, so a malformed review cannot hand the audit a `contract_gate_demoted` waiver. Its rejections
carry `Blocks == 0`, so:
- **no second-block CLI escalation** fires for it;
- **no salvage re-prompt** fires for it either: that directive warns of a breaker about to open,
  which is false for this phase. Each correction is the plain directive.

When its ladder is exhausted, two cases are possible.

- A **present but malformed** report degrades the phase to SKIPPED with a
  WARN (`contract_exhaustion_skip`, and `REVIEW_SKIPPED` for code-review).
- An **absent** (missing or
  empty) report aborts the cycle, because a non-admitted missing deliverable never reaches Ship.

The fresh ladder (`reviewWithCorrections`) and the resume ladder (`reviewResumedDeliverable`) take both
decisions from the same helpers (`Orchestrator.reviewInputFor`, `degradesRejection`).

## Scoping

The orchestrator applies escalation to `PhaseRequest.Driver`, an opaque `llmroute.Binding` (X19a), **for that re-dispatch only**. It is a soft
overlay (`llmroute.ApplySoftOverlay`) that promotes the target to chain primary and keeps the
own chain of the profile behind it. The phase routing and the profile on disk are untouched. The
non-compliance lives on the rare failure path, while the same CLI ships the common path fine.

`escalationAllowed` runs every candidate through `policy.ValidatePin`, so an escalation can never
route a phase to a CLI family that its operator forbade with `allowed_clis`. A phase whose whole chain is
one family, and that family is the universal fallback, has no target. That case belongs to the salvage
retry, below.

## Salvage retry: the trigger with no target (cycle-1300)

`contractEscalationCLI` returns `ok=false` when a phase's whole dispatch chain is one CLI family:
there is nowhere to escalate to. Before cycle-1300 the ladder then did **nothing**. The same
incapable CLI got the same plain directive a third time and the breaker opened. So the ratchet failed
*open* only because no escalation target existed (inbox `contract-block-cli-escalation`, LIVE EVIDENCE
2026-08-05).

The remedy is a **structured re-prompt** (`composeContractSalvageRetry`). The ladder was already going to send a correction. The remedy enriches that correction in two ways:

- It adds the output of the contract validator **verbatim** under `contractSalvageRetryDirectiveHeading`.
- It adds an instruction to address each bracketed `[violation_code]` and to name the exact section or path that it refers to.

The budget of round 2 buys the
*diagnosis* when it cannot buy a different CLI family (repair economics, arXiv:2306.09896).

Three invariants, each pinned by a test in `go/internal/core/contract_salvage_retry_test.go`:

- **Breaker-neutral.** It adds no dispatch, spends no extra correction, and does not touch
  `PhaseRequest.Driver` (there is no other family; that is the premise). The remedy does not touch `ReviewResult.Blocks`, the
  own counter of the breaker. So the circuit still opens on the third strike
  as the last resort.
- **Disjoint from escalation.** It fires only on the `ok=false` arm. So a phase *with* a real
  cross-family target escalates exactly as before and never also re-prompts. The budget of one block is
  never spent twice.
- **Same trigger window as escalation.** Block 2 with a shared defect identity; never the first
  block, never a `Blocks == 0` non-contract rejection.

## Defect identity (cycle-1291)

Identity is the block's **violation-code SET**, not its rendered text.

The reason under comparison is `deliverable.summarize()`. It is a `"; "`-joined rendering of *every*
violation on the block as `[code] message`. Cycle-1289 shipped a whole-string compare of the
fingerprint-normalized reason. The cycle-1289 audit rejected it HIGH: a **partially repaired**
set reads as a different defect.

- Block 1 reports `{missing_section, missing_verdict}`.
- The correction closes one.
- Block 2 reports `{missing_verdict}` alone. This is the strongest possible incapable-CLI
  signature, because the CLI demonstrably cannot close the remaining violation.
- Yet the two strings differ and the escalation was suppressed.

Superset regressions and re-ordered or re-worded renderings
of one set failed identically.

`deliverable.Violation.Code` is the stable primitive. Prose rewording or violation
order does not change it. Two blocks are the same defect exactly when their code sets **intersect**. This covers
subset, superset and equal. It still separates the disjoint sets that the identity gate exists to keep
apart.

| block 1 → block 2 | escalates? |
|---|---|
| `{A,B}` → `{B}` (partial repair) | yes |
| `{B}` → `{A,B}` (regression) | yes |
| `{A,B}` → `{B,A}`, reworded | yes |
| `{A,B}` → `{C,D}` (disjoint) | **no** — two honest defects |
| identical code-less reasons | yes (text fall-back) |
| no prior block observed (hot breaker) | yes |

Two fall-backs are load-bearing:

- **Code-less reasons.** Not every reason on this path is a `summarize()` rendering. When *either*
  block yields no code, identity falls back to `normalizeReasonForFingerprint`. This is the own primitive of the blocker breaker. It drops identity-noise tokens such as go-test durations. If `∅ ∩ ∅` reads as
  "different defect", the ladder silently disappears for every non-summarize reason shape.
- **Hot breaker.** The contract-gate breaker is process-global. A cycle that aborted mid-ladder
  leaves it hot. Then the next phase can arrive at `Blocks >= 2` on the FIRST block of its ladder, where no
  prior block exists. So the rule is "prior block known AND differing ⇒ suppress", never
  "equal ⇒ escalate". The zero-value `contractBlockIdentity` (nothing observed) reports true.

### Import-cycle constraint

`internal/deliverable` imports `internal/core` (`reviewer.go`, `verifier.go`); `core` imports
`deliverable` nowhere. So `core.ReviewResult` **cannot** carry `[]deliverable.Violation`, because that
is an import cycle. The codes reach `core` as plain data, parsed out of the rendered reason by
`contractViolationCodeRE`. The pattern is deliberately narrow (code-shaped tokens, no spaces). Thus
bracketed prose inside a violation *message* cannot masquerade as a code and fabricate an
intersection between unrelated defects.

## Demotion is still recorded

When the breaker opens anyway, `noteContractGateDemotion` does three things:

- It emits a stderr WARN that names the phase, the CLI, **which remedy ran** and the last violation.
- It appends a `contract_gate_demoted` ledger entry.
- It stages an autofile intent through `dispositionrouter` (never a direct `.evolve/inbox` write, which
  races `inboxmover.Claim`).

All three are best-effort. The gate has already decided.

The ledger `Action` carries both remedy facts, side by side:

```
demote enforce->advisory: cli=<cli> escalated=<bool> salvage_attempted=<bool> blocks=<n>: <reason>
```

`salvage_attempted` rides *alongside* `escalated`, never instead of it. A salvage retry is not an
escalation. The two let recurrence analytics separate the three distinct diagnoses that a demotion
can carry:

- A different CLI family was tried and also failed (`escalated=true`).
- A structured re-prompt was tried and the CLI was still unable to comply (`salvage_attempted=true`).
- No remedy was possible at all (both false).

`formatContractGateDemotionWarn` reports the same three cases in prose. For that reason it takes the
whole `contractDispatch`. An operator trusts and acts on its "escalation did NOT run" line, so the line must be false only when it is false.
