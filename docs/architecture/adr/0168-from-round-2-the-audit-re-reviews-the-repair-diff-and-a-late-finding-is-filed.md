# ADR-0168: From round 2 the audit re-reviews the repair diff, and a late finding is filed

- **Status:** Proposed (2026-11-21). The item `review-diff-only-audit-rereview` asked for this record.
- **Plan:** [review-diff-only-audit-rereview-2026-11.md](../../plans/review-diff-only-audit-rereview-2026-11.md). **Research:** [code-review-at-anthropic-2026-10](../../research/code-review-at-anthropic-2026-10.md), section 5, rank 1. **Case:** [cycle 2035](../../incidents/2026-10-27-cycle-2035-repair-loop.md).
- **Amends:** two sentences of decision 9 of [ADR-0126](0126-every-iterative-loop-converges-or-escalates.md): "The audit is untouched" and "The audit keeps its full verdict rules".
- **Relates to:** [ADR-0158](0158-a-repair-round-runs-only-for-a-finding-the-lane-can-fix-and-the-auditor-applies-a-small-prescribed-fix.md) keeps decision 4. [ADR-0167](0167-shadow-stages-prove-themselves-on-chained-evidence.md) governs the stage key.

## Context

ADR-0126 decision 9 leaves the audit untouched. Each audit round hunts the whole tree again. About 59% of repeated flags hit code from the first pass (research section 5). Round 2 and later then block on old code that the repair never touched.

Decision 6 of ADR-0126 gives the code-review judge a verify-only scope. A late finding below HIGH is filed. A late CRITICAL or HIGH blocks until its falsification check refutes it. No production code runs that check, so a late HIGH always blocks.

## Decision

1. Round 1 of the audit is a full judgment.
2. From round 2, the host gives the auditor the previous findings and the repair hunks.
3. A finding with a new id, outside the repair hunks, is late.
4. A late MEDIUM or LOW is filed and does not block.
5. A late CRITICAL or HIGH blocks until a falsification check refutes it.
6. The host never turns an audit FAIL into PASS or WARN.

The rule ships behind its own stage key, `workflow.convergence.audit_rereview.stage`. At `shadow` the prompt and the verdict do not change. At `enforce` the scope block and the host check turn on together.

## Amendment of ADR-0126 decision 9

From round 2, a late MEDIUM or LOW no longer decides the verdict. The rest of decision 9 stands: "CRITICAL is never deferred", "no audit FAIL becomes a PASS" and "no deferrals".

## Consequences

- A late MEDIUM or LOW becomes one deduplicated inbox follow-up. A filing failure goes into the landing notes with a signal.
- The host check can only force FAIL. It never lifts a verdict.
- When the scope is not available, the round runs as a full audit and a signal names the missing input.
- The saving is small for cycle 2035. Shadow evidence over the first repaired cycles decides the flip.

## Rejected

- **A prompt-only rule:** the prompt is then the only guard.
- **A host rule that lifts a FAIL:** a FAIL can rest on another cause, and the lift hides it.
- **The convergence stage key:** a flip also flips the Stop of every loop.
