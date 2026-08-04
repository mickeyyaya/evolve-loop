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
one family, and that family is the universal fallback's, has no target — the ladder then behaves
exactly as it did before this feature.

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
CLI, whether escalation ran and the last violation; appends a `contract_gate_demoted` ledger entry;
and stages an autofile intent through `dispositionrouter` (never a direct `.evolve/inbox` write,
which races `inboxmover.Claim`). All three are best-effort — the gate has already decided.
