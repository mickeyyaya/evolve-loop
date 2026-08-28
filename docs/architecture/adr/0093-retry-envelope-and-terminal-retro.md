# ADR-0093 — Policy sets the retry envelope, an adjudicator chooses inside it, retro learns

- **Status:** Accepted (2026-08-28). Supersedes the eligibility half of
  [ADR-0092](0092-audit-repair-loop.md); its counter, findings-injection and
  routed-path protections are retained.
- **Driving evidence:** waves 3 and 4 (cycles 1572–1577) — six cycles, **zero ships**,
  and the loop halted itself on `consecutive-failures`. The auditor was **right every
  time**; no gate misfired. What failed was the disposition of those rejections.

## Problem

An audit FAIL routed to retro, and retro had accumulated three unrelated jobs:
**analysing** the failure, **classifying** it (`disposition.json`), and **gating** whether
a retry could happen. Every defect found in that week traces to the conflation:

- Retro's verdict answers *"is the post-mortem complete?"* and was routed as *"did the
  cycle recover?"* — sending a rejected tree to ship. Pinned by five tests since
  2026-05-23, including one whose fixture comment read *"Even with prior failures, retro
  PASS overrides and ships."* Retro is read-only outside its own artifacts, so the tree it
  would have shipped is **byte-identical** to the one the auditor rejected.
- ADR-0092 made the retry depend on retro's prose. Measured reachable on **3 of 16**
  recent failures; on wave 4, **0 of 3**.
- Retro costs **20–47 minutes** and sat on the critical path of every retry.

And the authority for all of this already existed, unconsumed. ADR-0072's category table
has always declared:

```go
CategoryCodeAuditFail: {Level: LevelTask, Action: ActionRetryWithFix,
                        FixType: "address-audit-findings", MaxRetries: 2}
```

`fp.Categories[...]` was read in exactly ONE place (`failure_dossier.go`) and only for
`Level`. **`Action`, `MaxRetries` and `FixType` were consumed nowhere.** ADR-0092 built a
parallel `max_audit_repair_attempts` knob beside it and arrived at the same cap of 2
independently.

## Decision

**1. The retry decision moves to the audit chokepoint and reads the policy table.**
`decideAfterAuditFail` reads the audit's OWN declared failure class (via the existing
`phasecontract.ReadFailureBlock`) and looks it up through the new `RetryPolicyFor`
accessor, beside `IsFloor` and `IsSystemLevel`. Verified against cycles 1572/1574/1576/1577
— **all four declare `code-audit-fail`**, so this fires where the prose-dependent rule did
not.

**2. `computeRetryEnvelope` is a pure Specification.** `(declaredClass, floorCandidate,
attempts, policy) → legal actions + halt`. Pure so the same answer comes out on the live,
routed and resume paths; policy is injected so tests drive real tables.

**3. An adjudicator chooses INSIDE the envelope and can never widen it.** A deep-tier
advisor (`RetryAdjudicator`, modelled on `FailureAdvisor`) proposes an action with an
architectural justification. Three properties keep it from becoming another
proxy-as-verdict:

- Go computes the legal set first; the proposal is clamped to it. It may always be MORE
  conservative, never less. It cannot exceed `MaxRetries` or overturn a floor.
- It is an **enhancement, never a precondition** (Null Object): absent, malformed, or
  unjustified output yields the policy default. This is the ADR-0092 failure designed out.
- It is **skipped entirely** when only one action is legal, so deep-tier cost is paid only
  where judgment is genuinely required.

**4. Retro becomes terminal and learning-only.** It runs once, when the disposition is
DECLINE. Within-cycle feedback is the audit's own `audit-fail-reason.json` — already
structured, already exactly what the auditor wrote. Retro's real value, lessons into
`instincts/lessons/` for future cycles, is unchanged.

**5. The graph gains two edges: `audit→tdd` and `audit→build`.** Updated in all three
sources that hold it — the Go literal, the `phase-registry.json` SSOT, and the
config-independent trust anchor in `transition_oracle_test.go`, whose edit is the visible
record that this widening was deliberate.

## What adversarial review changed (both reviewers returned BLOCK)

The first implementation shipped none of this. An architect and a go-reviewer pass
found six real defects, two of them regressions this ADR itself introduced:

- **The graph widening was global (CRITICAL).** Adding `audit→tdd`/`audit→build` made
  them legal for the ROUTING ADVISOR too, which validates proposals through the same
  graph. At `advisory` — the live default — it could have granted a retry the envelope
  refused, on a path that never calls `consumeAuditRepairGrant`, leaving the only bound
  at `defaultMaxPhaseIterations` (32) rather than `MaxRetries` (2); routed backwards
  after a halt; or overridden `audit→ship` on a PASSING audit. Fixed with
  `decisionOnlyEdge`: the edges stay in the ONE legality graph so the deterministic
  decision can schedule them, but `transitionLegal` refuses them to the advisor. The
  adjudicator could not widen the envelope; the router could have widened it *around*
  the adjudicator.
- **A new prose-driven HALT (HIGH).** The envelope halted on any `Level == system`
  declared class with no `IsFloor` check and no corroboration — so `transport-hang` or
  `non-progress` (system-level, `Floor: false`) would have stopped the batch on an
  auditor's word. That is the disease this ADR exists to cure, reintroduced one layer
  over. Now ONLY the deterministic candidate halts here; a system-level declared class
  declines to retro, where the existing two-gate floor adjudicates it.
- **The resume surface was inert.** `RunCycleFromPhase` never called the disposition, so
  a resumed cycle could never repair — on the very surface that exists for recovery, and
  contradicting this ADR's own parity claim. Now branched, and pinned by a test that
  drives `RunCycleFromPhase` (the previous guard was a `strings.Contains` grep that
  passed while the code was dead).
- Class strings are now normalized (`failurelog.NormalizeLegacy`) before the policy
  lookup — the repo runs two vocabularies, so `audit-fail` would have declined and
  `infrastructure-systemic` would have been unrecognised.
- `parseAdjudication` reuses `lastBalancedSpan` instead of a naive first-`{`/last-`}`
  slice, which discarded a valid final answer whenever an artifact carried prose or an
  earlier object.
- The cycle-scoped lesson match gained a digit-boundary check: cycle 157's gate was
  satisfied by cycle 1574's lesson.

One reviewer finding was **rejected on evidence**: that the lessons directory contains no
`inst-L*` files. It contains none in the *console* plane (and none in a worktree, where
the path is gitignored), but the runtime plane — where the loop writes — holds 135, and
every recent failing cycle produced them and no `cycle-<N>-*` files.

## Consequences

**The ADR-0072 floor is preserved and evaluated EARLIER.** Gate 1 (the deterministic
dossier candidate) is computed from cycle state and on-disk artifacts, so it needs no
retrospective and now halts before any retry. Gate 2 (agent-authored `failure-decision.json`)
is a retro output and therefore applies on the terminal path only.

**Stated tradeoff:** an agent-classified system failure that the deterministic gate misses
burns up to `MaxRetries` retries before halting at retro. Bounded waste, no lost halt.

**One retry authority, not two.** `max_audit_repair_attempts`, `auditRepairCap`, and
`decideRepairEligibility` are deleted. ADR-0092's corroboration rule survives — prose
contradicted by two deterministic signals still loses — but it no longer gates retries.

**Re-earn, not override.** The principle any future retry layer must satisfy: a retry is
legitimate only when the work CHANGES and the verdict is re-earned on a different tree.
Re-running the same judge on the same artifact until it relents is forbidden, which is why
`policy.go` correctly keeps judgment phases out of `RemediablePhases`.

**Known limitation.** The re-entry choice (tdd vs build) is only as good as the
adjudicator's reasoning, and with no adjudicator wired it always defaults to tdd. That is
deliberate — the conservative, more thorough path — but it means the cheaper `retry@build`
route is unused until the persona is dispatched in production.
