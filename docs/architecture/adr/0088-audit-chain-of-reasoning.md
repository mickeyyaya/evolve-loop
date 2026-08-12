# ADR-0088 — The audit verdict is the conclusion of a chain of reasoning, not an assertion

**Status:** Accepted (core landed; persona + dispatch wiring staged)
**Date:** 2026-08-12
**Related:** ADR-0084 (gate-integrity invariants), ADR-0087 (scope-delta adjudication), incident 2026-08-10 (persona-strip lobotomy)

## Problem

An auditor that reads the diff and declares `PASS` has produced a judgement whose support is invisible. Everything downstream must either trust it or re-derive it — and the failures that matter most are precisely the ones a diff-only reading **cannot** see.

A human reviewer catches those instantly, and not by being cleverer. They hold the whole chain at once: what was asked, what was planned, what the tests demanded, what the builder claimed, what the bytes do, what the gates actually ran. Reading only the last of those, nobody could catch them either.

The four failures reviewers name out loud are not properties of a diff. **They are incoherences between stages:**

| Failure | What it actually is | Invisible to |
|---|---|---|
| **derailed** | internally consistent, delivers something other than what was asked | diff alone (needs the intent) |
| **specious** | the narrative is larger than the bytes | diff alone (needs the build report) |
| **paradoxical** | the implementation satisfies the tests *because the tests were moved to it* | any single link — each reads fine alone |
| **deceptive** | the evidence was produced by the party being judged | diff alone (needs the gate outputs) |

We have been trying to catch these with proxies instead — a substring match that force-FAILed four batches on "closed" inside "disclosed"; a scope rule that discarded a complete working implementation four times; a class label that skipped an entire evidence layer when declared. Every one is the same mistake: **a proxy standing in for the reviewer's actual question.**

## Decision

The auditor's obligation is to produce the **chain**. The verdict is a function of it.

### 1. The chain (`RequiredLinks`)

Seven relationships, in reasoning order. Each is a question a human reviewer asks without noticing they are asking it; writing them down is what makes the check auditable — and what makes its *absence* detectable.

| Link | The question | Catches |
|---|---|---|
| `intent-fidelity` | Does the intent capture the queued item, or was the task restated into something easier? | the failure that leaves **no trace in the diff at all** |
| `selection-fidelity` | Is the work triage committed to the work the intent describes? | solving a different item |
| `specification-fidelity` | Do the tests **encode** the acceptance criteria, or something weaker? | half of *paradoxical* |
| `implementation-fidelity` | Does the code satisfy the tests as they now stand? | the other half |
| `narrative-fidelity` | Does the build report describe what the bytes do? | *specious* |
| `delivery-fidelity` | Does the change deliver the **intent**, not merely something coherent? | *derailed* |
| `evidence-fidelity` | Were the gate results produced by running the gates, over these bytes? | *deceptive* |

Every link carries a **status**, a **finding** (the auditor's reasoning, in its own words) and a **citation** — something a third party can go and look at. A link without a citation is the auditor's opinion wearing the shape of a finding, and is rejected as malformed.

### 2. Entailment (`Conclude`)

```
all links coherent        → PASS
any link incoherent       → FAIL, naming the link
any link unverifiable     → WARN
any required link missing → FAIL — the chain is incomplete
```

`Conclude` never reads the `Finding` prose. The auditor supplies statuses and citations; the conclusion is arithmetic over them.

**This is the mechanism.** Judgement stays where judgement belongs — *is this link coherent?* — and determinism where that belongs — *given these statuses, what follows?* An auditor **cannot assert PASS over an incoherent link, because the verdict is not the auditor's to assert.** No amount of persona wording achieves that property; it is why this is code and not another paragraph in a prompt.

Two deliberate asymmetries:

- **Unverifiable is not a defect, and not a pass.** An auditor forced to choose between "fine" and "broken" for something it could not see will choose "fine" — that is how unverifiability becomes a laundering channel. `WARN` is the honest middle: not established, therefore not asserted.
- **A missing link fails harder than a negative finding.** Silence about a relationship is the cheapest way to avoid reporting it, so omission must not land in a gentler bucket than an honest bad answer.

### 3. Evidence entitlement — every judging phase reads its predecessors

The chain is only walkable if the judge was given what the links are measured against. So entitlement is a contract, not a convenience, and it applies to **every judging phase** — audit, adversarial-review, coverage-gate, plan-review, inherited-defect-reconcile, retrospective — not to the audit alone. `IsJudging` follows the act of deciding, not seniority: producing phases get nothing extra, because prompts that grow without bound are how the persona-strip incident happened (a truncated audit prompt cost 15 of 30 cycles before anyone noticed the auditor had never been shown the rules it was held to).

`linkEvidence` maps each link to the artifacts it is read from, so "does every required link have something behind it?" is checkable rather than hoped for.

And the teeth: **`ConcludeWithEvidence` downgrades to `unverifiable` any link reported coherent whose evidence was never supplied.** Without that, a phase dispatched without `intent.md` could still report delivery-fidelity coherent and PASS — a chain *narrated* rather than walked, indistinguishable on paper from a real one. The downgrade is not an accusation; it is the honest status for a relationship nobody was in a position to check.

## Consequences

**Gained.** The verdict becomes reviewable rather than trusted — a FAIL names the link that broke, and a PASS shows the seven relationships it rests on. The four failure names become mechanically produced from link statuses, so they are reported the same way every time instead of depending on whether the reader noticed. Withheld evidence surfaces as a qualified verdict instead of a confident one.

**Accepted.** The audit prompt carries more context, and the audit report carries a structured chain block. Both are small next to a wasted cycle — and the entitlement is bounded by link, not "everything".

**Relationship to ADR-0087.** The scope-delta discriminators (surface, effect, corroboration) are *evidence types used inside links*, not a decision procedure. They were drifting toward being another proxy tower; the chain is where the judgement actually lives, and they now feed it.

**Rollout.** Shadow first: emit the chain and compute the conclusion alongside the existing verdict for a wave, and compare. Where they disagree, the chain says which link the old verdict was silent about.

## Implementation

`go/internal/auditchain` — pure, no I/O, table-tested (98.2% coverage, apicover 28/28 with 0 false-green, race-clean). `auditchain.go` carries the chain, entailment and diagnosis; `evidence_access.go` the entitlement contract and the evidence-aware conclusion.

**Staged, not shipped:** the auditor-persona section that instructs the chain (tail-placed, per the persona-strip lesson), the runner change that supplies `RequiredEvidence` to every judging phase, the `chain` block in the audit report contract (schema single-sourced with a literal example, ADR-0084 I2), and the shadow comparison.
