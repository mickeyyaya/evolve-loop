# ADR-0102: Explanation review — reasoning is the gate, format is advisory

- **Status:** Accepted (operator decision, 2026-09-13)
- **Supersedes in part:** the rejected alternative "loosen the review gate to prose-only" in
  [ADR-0097](0097-read-only-phase-worktree-fence.md) and the grounding rule it defended
- **Related:** [ADR-0088](0088-audit-chain-of-reasoning.md) (the deterministic gate outranks the
  narrative); the Build document's delivery is host-gated independently of the reviewer's section —
  the build floor reviewer (`core/reviewer.go`, `mandatoryExplanationReviewer`), the remediation
  reseal (`core/cyclerun_remediate.go`) and the ship-time native explanation gate
  (`phases/ship/native_explanation_gate.go`, fail-closed); [ADR-0101](0101-signal-center.md)
  (advisories reach the operator through the phase record and the Signal Center)

## Context

The explanation-documentation review is a section of the audit report (and of the
retrospective) in which the reviewer judges the Build's explanation document. Until this ADR a
deterministic gate (`validateExplanationReview` → `explanationdocs.ValidateReviewedHandoff` →
`reportdoc.RequirePathLineEvidenceAt`) forced the audit verdict to FAIL whenever the section's
SHAPE was off: a missing section, a status word outside the enum, a build-status or document
echo that did not match the host handoff, or — the common case — Evidence that named the
document and the material paths without a literal `path:line` for every one of them.

On 2026-09-13 batch 2 produced no task ship for three waves. Cycles 1638 and 1640 both carried
an auditor narrative of PASS and were overridden to FAIL by this gate alone
(`verdict-conflict: auditor narrative=PASS but 1 deterministic gate(s) forced FAIL [explanation
documentation qualitative review]`, reason `explanation review Evidence must cite … with
path:line evidence`). The audit-repair disposition then re-dispatched the BUILD, which cannot
change the auditor's citations, so the continuation cycles (1641, 1645 re-running 1638's
snapshot) failed the same way — a poison loop. The retrospectives confirmed the builds were
sound and the reviews reasoned about the documents; what was missing was the citation form.

## Decision

The reviewer's **reasoning** is the gate. The section's **format** and any missing **context**
are advisory.

- Blocking (the verdict may still be forced to FAIL):
  1. the reasoning floor — the review's Evidence must say something (`reportdoc.RequireReasoning`,
     the existing 20-rune floor; a token or canned Evidence is not a review, and a missing or
     duplicated review section is no review text at all — absence is never laxer than thinness);
  2. (audit; retro records it as an advisory — see Consequences) a missing Build delivery
     reviewed as anything but `Status: FAIL` — the reviewer's honesty
     rule: VERIFIED over nothing is not reasoning (delivery itself is host-gated by the build
     floor reviewer, the remediation reseal and the ship-time native explanation gate); the error
     names the host's reason (`BuildExplanationError`) when the host lost the handoff;
  3. host-side defects in the handoff itself (a missing view, an unknown handoff status) —
     these are pipeline faults and must fail loudly.
- Advisory (recorded on the phase record as `warning` diagnostics, prefixed
  `explanation documentation review (advisory, ADR-0102): …`, never touching the verdict):
  unparsable fields (the floor then applies to the Evidence line as written — no Evidence line is
  no reasoning), a status outside `VERIFIED`/`NEEDS_CORRECTION`,
  the build-status echo, the document / SHA echo, the not-applicable document rule, the
  `path:line` citations and their line existence, the reviewer's own `NEEDS_CORRECTION`
  judgment, and (retro) the correction-todo bookkeeping.

The contract keeps ONE home: `explanationdocs.ValidateReviewedHandoff` now returns a `Review`
(`Status`, `Advisories`) and reserves its error for host-side defects; the audit and retro gates
return `(advisories, err)` and apply their phase policy to it. The guard
`TestExplanationReviewGates_ShareContractCore` still pins the delegation.

## Consequences

- A PASS narrative with a thin citation form ships. The advisory rides the C1 record (ADR-0100
  PR-4 makes a phase's diagnostics persist) and the `phase.outcome` signal, so the shape can be
  improved by the persona texts without burning cycles; the auditor persona keeps asking for
  `path:line` citations — the ask stands, the block does not.
- The audit-repair disposition no longer re-dispatches a Build for an auditor's citation form.
  A thin, missing or duplicated review still fails the audit and still routes through
  audit-repair (a Build re-dispatch) when the deliverable-contract gate — which demands the
  section first and corrects the auditor — is demoted; correcting the auditor there instead is
  a follow-up.
- The reasoning-floor ladder (section → fields → floor) has ONE home,
  `reportdoc.ReasonedReview`; the guard `TestExplanationReviewGates_ShareContractCore` forbids
  either gate to locate the section or apply the floor itself.
- `RequirePathLineEvidence` / `RequirePathLineEvidenceAt` keep their strict semantics; their one
  production caller is the contract (now advisory) — the `evolve phase verify` self-check never
  called the citation rule, so it and the phase gates agree.
- The retrospective's correction-todo rules are advisory too: a retro that reasons but
  mis-files its todo is recorded, not failed.
- Retro treats every lost handoff as advisory — the Build never delivering and the host losing
  the handoff after the audit saw it alike (the advisory carries `(host: …)` from
  `BuildExplanationError`) — because the post-mortem's verdict gates no ship. The contract's
  nil-view error is a guard for direct callers: both phase gates return before delegating when
  the handoff is absent.
- Persistence: an advisory on a PASS reaches the C1 record only because #577 (ADR-0100 PR-4)
  keeps a PASS's warning trail on the record — this change lands after it;
  `TestRecordPhaseOutcome_AnAdvisoryOnAPassRidesTheRecord` (core) pins that seam and the audit
  package pins the other half (`Classify` emits the advisory on a PASS).
- The advisory prefix has one home, `explanationdocs.AdvisoryPrefix`, used by both phases and
  pinned by both phase suites.

## Verification

Red-first: the audit and retro gate tests whose intent flipped (shape → advisory) were observed
red at the API level, the cycle-1638 shape (`TestClassify_PathOnlyCitationsKeepThePassVerdictAndRecordTheAdvisory`)
red on behaviour; the contract table pins every advisory and the two loud host defects; the
reasoning floor and the missing-delivery rule keep their blocking tests; the retro phase-level
proof (`TestRun_PreviousFAIL_LostHandoffAdvisoryRidesTheRecord`) shows the advisory on the
record with a PASS verdict. Mutants killed by name
(eighteen): advisories not recorded (audit and retro), the shape forcing FAIL again, the reasoning
floor removed, `NEEDS_CORRECTION` blocking again, the missing-delivery rule dropped, the contract's
citations blocking again, a missing section advisory again (both phases, and again through the
shared ladder), a duplicated section skipping the floor (twice), a host defect dropping the findings
(both phases), the host reason dropped, the prefix not applied, the garbled-section floor measuring
the body instead of the Evidence line.
