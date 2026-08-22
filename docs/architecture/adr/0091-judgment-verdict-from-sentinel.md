# ADR-0091 — A judgment phase's STATED verdict is authoritative, behind a per-phase rollout stage

- **Status:** Accepted (2026-08-22) — landed at `shadow` on both judgment phases; `enforce` is NOT granted by this ADR.
- **Driving incident:** cycle-1528. `premise-challenge` concluded
  **"FAIL (BLOCK). The cycle must not proceed as framed"** with
  `premise.severity_max == CRITICAL`, emitted the canonical machine sentinel saying
  `verdict:"FAIL"`, and the cycle ran on through tdd, build, prompt-regression-eval,
  coverage-gate, adversarial-review, audit and retro. The objection was **correct** —
  it falsified the plan's load-bearing premise, and the redesign it forced shipped as
  [ADR-0090](0090-transient-disclosure-as-cause-data.md). It changed nothing at the
  time because a human read the report and hand-copied it into an inbox item.
- **Related:** [ADR-0088](0088-audit-chain-of-reasoning.md) — the shadow-stage pattern
  copied here, including the rule that the record is taken on agreement too.
- **Related:** [ADR-0034](0034-unified-deliverable-contract.md) — the verdict sentinel this
  reuses. No second grammar is introduced.
- **Composes with:** `go/internal/core/judgment_lesson.go` (PR #479) — a judgment FAIL
  teaches without halting. That recorder was correct and near-unreachable: only a
  STRUCTURAL failure could reach it, never a reasoned objection.

## Problem

`specrunner.EvaluateClassify` decided a spec-driven phase's verdict from **structure
only**: empty artifact → FAIL, missing `require_sections` → FAIL, otherwise PASS.
`premise-challenge` and `adversarial-review` declare `require_sections` and nothing
else, so a well-formed report was **always PASS regardless of what it concluded**.

The declared output signals (`premise.severity_max`, `adversarial.severity_max`) are
consumed by nothing: the only signal hook, `fail_if_signal`, hard-FAILs with
"Stage-3 signal bus not available". There was no configured path from an agent's
stated judgment to a `core.Verdict`. The loop paid for a full agent dispatch every
cycle and discarded its conclusion.

**The signal was never missing.** Measured across this repo's entire run history
(225 `premise-challenge` + `adversarial-review` reports on disk):

| | reports | sentinel present | stated FAIL | stated WARN | stated PASS |
|---|---|---|---|---|---|
| premise-challenge | 55 | 55 | **52** | 2 | 1 |
| adversarial-review | 170 | 170 | 48 | 97 | 23 |
| **total** | **225** | **225 (100%)** | **100** | **99** | **24** |

Every one of those 225 classified PASS. This is a **wiring** defect, not a parsing
problem.

## Decision

1. **Reuse the existing sentinel parser.** `phasecontract.ParseVerdictSentinel` — the
   same reader the contract gate and verdict cache already use. The inbox item
   proposed a `fail_pattern` regex over the `## Verdict` section; that would have been
   a second grammar for a signal that already has one, free to drift from it. The
   measurement above is what made the regex unnecessary: sentinel coverage is 100%.
2. **A per-phase stage word, `classify.verdict_from_sentinel`**, not a bool and not a
   global policy stage. `""` = off (byte-identical legacy), `"shadow"` = record the
   disagreement and route as before, `"enforce"` = the stated verdict decides.
3. **Shadow first, and shadow is where this ADR stops.** See Consequences — the
   population is uncalibrated, and `enforce` today would halt nearly every cycle.
4. **Fail-open.** An absent, malformed, or non-canonical stated verdict keeps the
   structural verdict. Only an unambiguous stated verdict moves anything, so a
   malformed report can never hard-block a cycle. Failing open is **announced** in a
   diagnostic — a silently discarded conclusion is the defect being cured.
5. **Structure is still evaluated first.** A stated PASS cannot launder a report that
   is missing required sections or is empty.
6. **An unknown stage word is a hard FAIL**, not a silent default-to-off. This is the
   cycle-241 declared-semantics rule the same function already applies to
   `fail_if_signal` and `verdict_on_pass`: an inert gate must fail loudly. A catalog
   test (`TestRepoPhaseCatalog_VerdictFromSentinelStageIsKnown`) catches the typo at
   authoring time so the runtime rejection stays a floor, not the discovery path.
7. **Enabling the key requires the teaching side.** `TestJudgmentTeachingPhases_Cover‑
   EveryPhaseThatCanStateItsOwnVerdict` binds any tracked phase declaring the key to
   membership in `judgmentTeachingPhases`. Bound at DECLARATION, not at enforce, so
   promoting a phase can never open the gap the recorder exists to close.

## Alternatives considered

- **Build the Stage-3 signal bus** so `fail_if_signal` works and key on
  `premise.severity_max`. Rejected as larger than the defect: the sentinel is already
  present in 100% of reports and already parsed elsewhere. The bus remains the right
  answer for signals that are not verdicts.
- **A `fail_pattern` regex over the `## Verdict` section** (the inbox item's preferred
  shape). Rejected: a second grammar, and unnecessary given 100% sentinel coverage.
  Note the repo already has a latent instance of exactly this hazard — `slugLineRE`
  forbids a backtick the persona actually emits, so its prose fallback never matches a
  real report.
- **A global policy stage** (`gates.judgment_verdict`). Rejected: it would force the
  two phases to promote together, and the measurement says they are in completely
  different states of calibration (94% vs 28% stated-FAIL).
- **Enforce immediately.** Rejected on the numbers below.

## Consequences

- **No routing changes today.** Both phases land at `shadow`; every cycle routes
  byte-identically. What changes is that `judgment-verdict-shadow.json` now appears in
  each judgment phase's workspace, carrying the stated verdict, the routed verdict, and
  `would_flip`.
- **`enforce` is not safe yet, and the shadow record is not what proves it.** The
  historical data already answers the flip-rate question, and the answer is a red
  light: **premise-challenge stated FAIL on 52 of 55 reports — 20 of 20 since
  cycle-1500.** A phase that objects 100% of the time carries no information.
  Enforcing it would halt essentially every cycle at that phase, and because
  `premise-challenge` is in `remediationDenied` there is no builder re-roll — the FAIL
  routes straight to retro.
- **So the blocking question is persona calibration, not plumbing.** The next step is
  not a longer soak; it is deciding how many of those 100 stated FAILs are genuine
  blocking objections versus a persona that says FAIL habitually **because nothing
  ever consumed it**. A signal nobody reads is a signal nobody calibrates. The shadow
  record now makes that audit mechanical instead of manual.
- **`adversarial-review` is the plausible first promotion** (28% stated FAIL, 57%
  WARN, 14% PASS) — it discriminates. It should still not be promoted before the
  WARN population is understood: WARN is the majority outcome there, and honoring it
  is a bigger behavioral change than honoring FAIL.
- **WARN carries through under enforce.** 99 of 225 reports state WARN; silently
  upgrading it to PASS would re-create this exact defect for the most common non-clean
  outcome.
- **Phases outside the catalog are untouched.** `EvaluateClassify` is byte-identical
  for every phase that does not declare the key, pinned by a no-regression test.
