# Plan: a capability failure climbs one bounded retry ladder

- **Status:** accepted, 2026-10-19. The operator decided Q1 to Q3 of §6 on 2026-10-19. L5 and L6 are built at stage `shadow` (2026-10-27, [§5](#5-components-and-order)).
- **Research:** [retry-ladder-2026-10](../research/retry-ladder-2026-10.md). **Decision:** [ADR-0142](../architecture/adr/0142-a-capability-failure-climbs-one-bounded-retry-ladder.md). **Design:** [retry-ladder](../architecture/retry-ladder.md).
- **Related:** [ADR-0096](../architecture/adr/0096-repair-rounds-escalate-and-carry-findings.md) (repair rounds raise the tier), [ADR-0126](../architecture/adr/0126-every-iterative-loop-converges-or-escalates.md) (converge or escalate), [ADR-0130](../architecture/adr/0130-a-retry-carries-the-evidence-of-the-attempt-it-replaces.md) (a retry carries the evidence), [ADR-0141](../architecture/adr/0141-every-phase-handoff-is-a-request-build-check-loop-with-three-exits.md) (the three exits of a handoff).

## Table of contents

- [1. The problem](#1-the-problem)
- [2. A worked example](#2-a-worked-example)
- [3. Candidate designs](#3-candidate-designs)
- [4. Decisions](#4-decisions)
- [5. Components and order](#5-components-and-order)
- [6. Open decisions for the operator](#6-open-decisions-for-the-operator)
- [7. Verification](#7-verification)
- [8. Limits](#8-limits)

## 1. The problem

The operator asked for a stronger retry. When an agent cannot meet the request and the deliverable, the retry gets a stronger tier or more effort.

The research found three facts:

- About half of the build correction rounds fail again: 65 `rejected_again` against 59 `accepted` (F2.8).
- No correction round raises the tier or the effort. The contract ladder changes only the CLI family (F2.4), and effort has no per-try field (F2.5).
- The two raises that exist, the repair round of ADR-0096 and the deep floor of ADR-0076, are off for build and tdd. The pin `agents.builder.model=balanced` of PR #981 drops every tier overlay (F2.3).

A raise is not always the fix. Two of the four recent aborts were spec failures: the request asked for something that the lane cannot do (F2.11). A stronger model only adds cost there.

## 2. A worked example

Cycle 1966 (lane `history-refusal-names-the-evolve-comments-command-lane`), as it ran on 2026-10-19:

| Try | Rung | Result |
|---|---|---|
| 1 | agy, Gemini 3.8 Flash High, balanced | the floor rejects: `TestC1966_002` stays red |
| 2 | the same | the same failure |
| 3 | claude, Sonnet, balanced (agy exited 85, a quota exit) | the floor rejects |
| 4 | the same | the build aborts after its corrections |

The same cycle under the ladder of this plan:

| Try | Rung | Why |
|---|---|---|
| 1 | agy, Flash High, balanced (R0) | the base rung |
| 2 | deep tier, fresh session, failure digest (R2) | the class is capability. The effort step (R1) is skipped, because Flash High is the top effort of the balanced agy rung |
| 3 | none: stop | if try 2 fails with the same fingerprint, two tiers failed on the same check. The ladder reclassifies it as a spec failure and routes it to the console with the dossier |

The ladder uses two or three tries in place of four, and the second try is a stronger model with a clean context.

## 3. Candidate designs

| Design | Summary |
|---|---|
| A. Patch each path | Remove the builder pin. Add a tier raise inside the contract escalation. |
| B. One retry ladder | A deterministic classifier and one decider give the rung of each retry. Every retry path asks the decider. The rung carries an effort field. |
| C. Critic first | On a capability failure, a short deep-tier critic phase writes the digest. The executor stays at its base tier. |
| D. Learned first tier | A bandit router picks the first tier of each try from logged outcomes. There is no raise on retry. |
| E. Always deep | Build and tdd always run at deep. |

Each criterion scores 1 (poor) to 5 (good).

| Criterion | A | B | C | D | E |
|---|---|---|---|---|---|
| Fixes the observed capability failures (F2.10, F2.11) | 3 | 4 | 3 | 2 | 4 |
| Bounds the cost (F3.1, F3.3) | 3 | 4 | 5 | 4 | 1 |
| One rule for every retry path, no duplication | 1 | 5 | 3 | 4 | 4 |
| Can run in shadow and be measured (F2.12) | 2 | 5 | 4 | 2 | 3 |
| Low change risk | 4 | 3 | 3 | 2 | 5 |
| **Total (of 25)** | **13** | **21** | **18** | **14** | **17** |

**B wins.** It is the only design that puts the classifier, the bound and the telemetry in one place.

- A spreads the rule over two paths and keeps the gap of F2.6, where remediation clears a raise.
- C is cheaper, but no local data shows that a critic alone fixes our failures. It becomes a shadow arm of B (D12).
- D needs the telemetry that does not exist yet (F2.12). It is a later stage of B.
- E costs the most, and the Argon week window ran out on 2026-10-18 when build ran at deep.

## 4. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | A new package `internal/retryladder` holds the classifier and the decider as pure functions. It uses the one-tier raise of `internal/convergence` and adds no second tier table. | F2.7 | A copy of the raise arithmetic: two tables drift. |
| D2 | The classifier is deterministic Go code. It returns `infra`, `quota`, `halt`, `spec`, `capability` or `unknown`. Exit 85 is `quota` only when the usage probe or the bench confirms it. | F2.11, F3.8 | An LLM classifier: the classes follow from codes and paths that the host already has. |
| D3 | There are three rungs. R0 is the base. R1 is a fresh session, the digest and one effort step at the same tier. R2 is a fresh session, the digest and one tier step. After R2 the ladder stops. | F3.3, F3.5, F3.6 | A third tier step: F3.3 shows no gain. A same-context retry: F3.5. |
| D4 | The ladder skips R1 when the fingerprint repeats after R0, or when the family has no higher effort at the tier. | F2.9 | Always try effort first: it wastes a try on a stuck failure. |
| D5 | When R2 fails with the fingerprint of an earlier rung, the ladder reclassifies the failure as `spec` and stops. The handoff takes its blocked exit (ADR-0141). | F3.3, F2.11 | A further retry: two tiers that fail on one check point to the request. |
| D6 | Only `capability` climbs. `unknown` can take R1 only. `infra` and `quota` keep the present retries at the same rung. `halt` (exits 2, 3, 10, 86 and 99) and `spec` never climb. | F2.11, F3.8 | One rule for all classes: it raises the cost of spec failures. |
| D7 | `agents.<agent>.model` is the base tier (R0), not a ceiling. Two new keys bound the ladder: `agents.<agent>.max_tier` and `max_effort`. The overlay of the ladder outranks the pin. The first-try advisor overlay still respects the pin. | F2.3 | Remove the pin: the advisor then raises the first build to deep and drops Flash. |
| D8 | `PhaseRequest` gets `ModelRoutingEffort`, and the launch uses it over the tier default. The family realizer caps it to the steps that the family has. | F2.5 | An effort raise by a tier raise only: it cannot express "the same model, more effort". |
| D9 | Before a tier step, the decider reads the bench and the quota windows of each model group. It picks the first healthy family at the target tier. With no headroom it records `LADDER_NO_HEADROOM`: a failure at R0 goes to R1, and a failure at R1 stops. | the Argon week window of 2026-10-18 | A raise that ignores quota: it walls the deep group. |
| D10 | The rung is sticky for the rest of the item, and one cycle runs one item. The correction ladder, the audit repair round and gate remediation read and keep it. The next item starts at R0, or at R2 when its last failure was `capability` at R1 or higher. | F2.6, F2.2 | A reset per path: remediation undoes a raise (F2.6). |
| D11 | Judge seats never climb: the auditor, the adversarial review, the code reviewer and the spec verifiers. The ladder serves build, tdd, scout, triage, fault localization and bug reproduction. | ADR-0141 | Raise the judge: the grade then depends on the try it grades. |
| D12 | The critic rung of design C is a shadow arm. The decider records the rung that a critic arm chooses. A later plan decides it on the data. | F3.9 | Adopt it now: no local measure. |
| D13 | Each try records its rung: the class, the fingerprint, the effort, the tier, the served model id, fresh or kept context, and the outcome. `evolve ladder report` gives the success and the cost per rung. | F2.12, F3.10 | Measure from the logs later: the fields are not there. |
| D14 | `workflow.retry_ladder.stage` is `shadow` as a compiled default. At shadow the decider records its rung and the dispatch does not change. The operator moves it to `enforce`. | the stage pattern of ADR-0139 | Enforce at once: the classifier is unproven. |
| D15 | The same-rung corrections of `fleet.contract_correction_retries` run first. The correction loop asks the decider when that budget is spent. | the operator decision of 2026-10-27, "1 correction, then escalate"; cycle 2035 | Ask the decider on each correction: the first failure then climbs at once, and the one same-rung correction of the operator does not run. |
| D16 | After an audit FAIL, the decider chooses `audit_fix` when each defect has a prescription and the estimated fix is inside the limits of `workflow.retry_ladder.audit_fix`. A cycle chooses it at most once. | the operator decision of 2026-10-27, "make sure audit will involve fix and trigger another review for its changes; balance cost and output"; cycle 2035 | A builder repair round for each prescribed fix: cycle 2035 spent 65 build minutes on a fix of about 20 lines. |

### Consequences

- Good: a capability failure gets a stronger try within the cycle, and a spec failure stops early.
- Good: the ladder replaces the CLI-only escalation of the contract ladder with one rule, so a reader finds every raise in one package.
- Bad: a raised try costs more tokens, and a deep raise uses the week window of the deep group. D9 and the cap of two raised tries bound it.
- Changed on purpose: today any failure starts the next build at deep (`failure_count >= 1`). With D10, only a `capability` failure at R1 or higher does. A spec failure starts the next item at R0.
- Bad: the pin change of D7 alters `cliroute` precedence. A test pins the three cases: the first try, a ladder raise and an advisor raise.

### What changes our mind

- Count 30 capability failures in the report of D13. If R2 fixes fewer than 1 in 4 of them, the tier step is not worth its cost. Then the critic arm of D12 replaces it.
- If R1 fixes as many failures as R2, the effort step becomes the default and the tier step becomes rare.

## 5. Components and order

| # | Component | Home | Route |
|---|---|---|---|
| L0 | The try record: rung, class, fingerprint, effort and served model in `llm-calls.ndjson`, and `attempt` counts the correction rounds | `internal/bridge`, `internal/core` | console |
| L1 | The classifier, with the fixtures of cycles 1946, 1950, 1953, 1965 and 1966 | `internal/retryladder` (new) | lane |
| L2 | The decider: rung, skip, stop and the shadow critic arm | `internal/retryladder` | lane |
| L3 | The effort overlay: `ModelRoutingEffort`, `llmroute.Overlay.Effort` and the realizer cap | `internal/core`, `internal/llmroute`, `internal/bridge` | console |
| L4 | The pin as base: `max_tier`, `max_effort` and the ladder overlay in `cliroute` | `internal/cliroute`, `internal/policy` | console |
| L5 | The wiring at shadow: the correction ladder, the repair round, remediation and the next-item start rung | `internal/core` | console |
| L6 | The fresh session and the digest on a raised rung (ADR-0130 postmortem plus the verbatim failure) | `internal/core`, `internal/bridge` | console |
| L7 | `evolve ladder report` | `cmd/evolve` | lane |
| L8 | The move to `enforce`, after one wave of shadow data. Inbox item `retry-ladder-l8-enforce`, an operator decision (2026-10-27). | `.evolve/policy.json` | operator |

L1, L2 and L7 can run in lanes, because the new package is not protected. L0 comes first, so the shadow wave of L5 has data.

### Status of L5 and L6 (2026-10-27)

| Component | Status | Evidence |
|---|---|---|
| L5 | built. The correction loop, the resumed correction and the audit repair round ask one decider. Gate remediation keeps its rung, and the next-item rule stores the first rung. The checked-in policy sets `workflow.retry_ladder.stage` to `shadow`. | `go/internal/core/retry_ladder.go`, [design §5](../architecture/retry-ladder.md#5-where-the-ladder-runs) |
| L6 | built. A raised try at `enforce` has `context` `fresh` and a digest inside `workflow.retry_ladder.digest_tokens`. The tier step comes from the routing data of the phase. | `go/internal/retryladder/digest.go`, [design §5](../architecture/retry-ladder.md#the-fresh-session-and-the-digest-l6) |
| `audit_fix` | the decision is built (D16). The handler is built in `internal/auditfix`: the auditor applies the patch, and a fresh seat reviews the fix. The wiring is open. | `go/internal/retryladder/audit_fix.go`, `go/internal/auditfix/auditfix.go`, [design §5](../architecture/retry-ladder.md#the-audit_fix-rung) |
| L8 | open. The operator moves the stage to `enforce` after one wave of `LADDER_DECISION` data. | the inbox item of L8 |

The operator decided on 2026-10-27: "1 correction, then escalate" (D15). The console sets `fleet.contract_correction_retries` to 1 in a separate policy change.

Two gaps stay open:

- The failure record of an item has no rung. So at `enforce`, the next-item rule starts a `capability` failure at R2 without the check "at R1 or higher" of D10.
- A resumed run keeps the ladder state of its correction loop only inside that loop. A later phase of the resumed run starts again from the rung in the checkpoint.

## 6. Open decisions for the operator

| Id | Question | Default |
|---|---|---|
| Q1 | The highest tier that the ladder can reach | Decided: `deep`. The top tier on retry adds no gain (F3.3), and on agy-claude deep and top are the same model. |
| Q2 | The highest effort that the ladder can reach without the operator | Decided: `high`. `xhigh` only by `max_effort`; `max` never by the ladder (F3.6, F3.7). |
| Q3 | The critic arm of design C | Decided: a shadow record only (D12) |
| Q4 | Judge seats stay out of the ladder | yes (D11), the default |

## 7. Verification

- **Classifier:** each fixture cycle gives its class of F2.11 (1946, 1950 and 1953 give `spec`; 1965 and 1966 give `capability`). A red test comes first.
- **Decider:**
  - a table test covers each rung, the R1 skip of D4, the stop of D5 and the no-headroom hold of D9;
  - a property test shows that no input gives more than two raised tries, a tier above `max_tier` or an effort above `max_effort`.
- **Pin precedence:** three cases in `cliroute`. The first try keeps the pin, a ladder raise outranks it, and an advisor raise does not.
- **Sticky rung:** a remediation round after an R2 correction keeps R2 (the gap of F2.6).
- **Telemetry:** a cycle with two correction rounds writes three try records with rungs R0, R1 and R2.
- **Shadow:** at `shadow` the dispatch requests are byte-identical to today. A golden test of the request proves it.

## 8. Limits

- The ladder cannot fix a request that is wrong. It can only stop sooner and say so.
- Shadow records the choice of the ladder, not its effect. The help shows only at `enforce`, in the report of D13.
- The fingerprint uses the violation codes of the contract escalation. Two different defects with the same code look the same.
