# Contract-Block CLI Escalation

**Status:** live · **Code:** `go/internal/core/contract_escalation.go`, `go/internal/core/cyclerun_review.go`
· **Tests:** `go/internal/core/contract_escalation_test.go`, `go/acs/cycle1291/predicates_test.go`

## The defect this closes

The correction ladder in `reviewAndGuard` re-dispatched the **same** profile CLI after a
deliverable-contract block. A profile's `cli_fallback` chain fires only on infra exit codes
`{80,81,85,124,127}` — never on a contract violation — so a CLI that systematically mis-formats a
deliverable burns every correction and the contract-gate breaker opens, demoting `enforce→advisory`
for the rest of the run. Batch-19 (cycles 1171/1172) and batch-21 (cycle-1215) both ended that way:
a FORMAT-compliance failure silently WEAKENED a gate. The correct escape hatch is CLI escalation,
not gate demotion.

## Trigger

A correction re-dispatch escalates to a different CLI **family** when both hold:

1. `ReviewResult.Blocks >= contractEscalateAtBlock` (2) — the contract gate's own consecutive-block
   counter, the same one that will open the circuit at 3. Never a locally re-counted correction
   ordinal, which desyncs whenever a prior cycle left the breaker hot or the salvage rung consumed a
   block.
2. `contractBlocksShareIdentity(prev, reason)` — the block is the **same defect** as the one that
   triggered the previous correction. `Blocks` counts blocks, not defects; two honest violations are
   not one incapable CLI.

`Blocks == 0` (evalgate / topngate / triagecap / the build floor) never escalates: those are
task-binding or capacity rejections, and a different CLI is not the remedy.

## Scoping

Escalation is applied to `PhaseRequest.ModelRoutingCLI` **for that re-dispatch only**, as a soft
overlay (`llmroute.ApplySoftOverlay`) that promotes the target to chain primary while keeping the
profile's own chain behind it. The phase's routing and the profile on disk are untouched: the
non-compliance lives on the rare failure path while the same CLI ships the common path fine.
`escalationAllowed` runs every candidate through `policy.ValidatePin`, so an escalation can never
route a phase to a CLI family its operator forbade via `allowed_clis`. A phase whose whole chain is
one family, and that family is the universal fallback's, has no target — that case is the salvage
retry's, below.

## Salvage retry: the trigger with no target (cycle-1300)

`contractEscalationCLI` returns `ok=false` when a phase's whole dispatch chain is one CLI family:
there is nowhere to escalate to. Before cycle-1300 the ladder then did **nothing** — the same
incapable CLI got the same plain directive a third time and the breaker opened, so the ratchet failed
*open* purely for want of an escalation target (inbox `contract-block-cli-escalation`, LIVE EVIDENCE
2026-08-05).

The remedy is a **structured re-prompt** (`composeContractSalvageRetry`): the correction the ladder
was already going to send is enriched with the contract validator's output **verbatim** under
`contractSalvageRetryDirectiveHeading`, plus an instruction to address each bracketed
`[violation_code]` by naming the exact section or path it refers to. Round 2's budget buys the
*diagnosis* when it cannot buy a different CLI family (repair economics, arXiv:2306.09896).

Three invariants, each pinned by a test in `go/internal/core/contract_salvage_retry_test.go`:

- **Breaker-neutral.** It adds no dispatch, spends no extra correction, and does not touch
  `ModelRoutingCLI` (there is no other family — that is the premise). `ReviewResult.Blocks`, the
  breaker's own counter, is untouched by the remedy, so the circuit still opens on the third strike
  as the last resort.
- **Disjoint from escalation.** It fires only on the `ok=false` arm, so a phase *with* a real
  cross-family target escalates exactly as before and never also re-prompts — one block's budget is
  never spent twice.
- **Same trigger window as escalation.** Block 2 with a shared defect identity; never the first
  block, never a `Blocks == 0` non-contract rejection.

## Defect identity (cycle-1291)

Identity is the block's **violation-code SET**, not its rendered text.

The reason under comparison is `deliverable.summarize()` — a `"; "`-joined rendering of *every*
violation on the block as `[code] message`. Cycle-1289 shipped a whole-string compare of the
fingerprint-normalized reason, and the cycle-1289 audit rejected it HIGH: a **partially repaired**
set reads as a different defect. Block 1 reports `{missing_section, missing_verdict}`, the correction
closes one, block 2 reports `{missing_verdict}` alone — the strongest possible incapable-CLI
signature, since the CLI demonstrably cannot close the remaining violation — yet the two strings
differ and the escalation was suppressed. Superset regressions and re-ordered or re-worded renderings
of one set failed identically.

`deliverable.Violation.Code` is the stable primitive: untouched by prose rewording or violation
order. Two blocks are the same defect exactly when their code sets **intersect** — which covers
subset, superset and equal, and still separates the disjoint sets the identity gate exists to keep
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
  block yields no code, identity falls back to `normalizeReasonForFingerprint` (the blocker breaker's
  own primitive, which drops identity-noise tokens such as go-test durations). Reading `∅ ∩ ∅` as
  "different defect" would silently delete the ladder for every non-summarize reason shape.
- **Hot breaker.** The contract-gate breaker is process-global, so a cycle that aborted mid-ladder
  leaves it hot and the next phase can arrive at `Blocks >= 2` on its ladder's FIRST block, where no
  prior block exists. The rule is therefore "prior block known AND differing ⇒ suppress", never
  "equal ⇒ escalate": the zero-value `contractBlockIdentity` (nothing observed) reports true.

### Import-cycle constraint

`internal/deliverable` imports `internal/core` (`reviewer.go`, `verifier.go`); `core` imports
`deliverable` nowhere. `core.ReviewResult` therefore **cannot** carry `[]deliverable.Violation` — that
is an import cycle. The codes reach `core` as plain data, parsed out of the rendered reason by
`contractViolationCodeRE`. The pattern is deliberately narrow (code-shaped tokens, no spaces) so
bracketed prose inside a violation *message* cannot masquerade as a code and fabricate an
intersection between unrelated defects.

## Demotion is still recorded

When the breaker opens anyway, `noteContractGateDemotion` emits a stderr WARN naming the phase, the
CLI, **which remedy ran** and the last violation; appends a `contract_gate_demoted` ledger entry; and
stages an autofile intent through `dispositionrouter` (never a direct `.evolve/inbox` write, which
races `inboxmover.Claim`). All three are best-effort — the gate has already decided.

The ledger `Action` carries both remedy facts, side by side:

```
demote enforce->advisory: cli=<cli> escalated=<bool> salvage_attempted=<bool> blocks=<n>: <reason>
```

`salvage_attempted` rides *alongside* `escalated`, never instead of it — a salvage retry is not an
escalation. The two are what let recurrence analytics separate the three distinct diagnoses a demotion
can carry: a different CLI family was tried and also failed (`escalated=true`), a structured re-prompt
was tried and the CLI still could not comply (`salvage_attempted=true`), or no remedy was possible at
all (both false). `formatContractGateDemotionWarn` reports the same three cases in prose and takes the
whole `contractDispatch` for that reason: its "escalation did NOT run" line is one an operator trusts
and acts on, so it must be false only when it is false.
