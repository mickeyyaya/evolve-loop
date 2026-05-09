> **Sequential-Write Discipline (v8.55.0+)** — Codifies the rule that only READ-ONLY/summarizing subagents may run in parallel; WRITE-capable subagents must run sequentially as single-writers. Read this before adding a new agent role, fan-out subtask, or modifying `cmd_dispatch_parallel()`.

## Table of Contents

1. [The Two Orthogonal Questions](#the-two-orthogonal-questions)
2. [The Structural Rule](#the-structural-rule)
3. [Role Taxonomy](#role-taxonomy)
4. [Profile Schema](#profile-schema)
5. [Dispatch-Time Enforcement](#dispatch-time-enforcement)
6. [The Concurrency Cap](#the-concurrency-cap)
7. [Adding a New Role](#adding-a-new-role)
8. [Why This Matters](#why-this-matters)

---

## The Two Orthogonal Questions

Parallel fan-out raises two independent decisions that v8.55+ codifies separately:

| Question | Answer | Mechanism |
|---|---|---|
| **Discipline**: which roles MAY run in parallel? | Only READ-ONLY summarizers (default-deny) | `parallel_eligible` field in profile JSON |
| **Cap**: how many parallel-eligible workers run AT ONCE? | Default 2 (was 4 pre-v8.55) | `EVOLVE_FANOUT_CONCURRENCY` env var, FIFO semaphore in `fanout-dispatch.sh:73` |

These are independent. A profile may be `parallel_eligible: true` but still run alone if its declared `parallel_subtasks` count is 1. The cap throttles *eligible* workers; it does not promote ineligible ones.

---

## The Structural Rule

```
A profile MAY declare `parallel_eligible: true` if and only if:
  (a) the agent's role is READ-ONLY on filesystem, ledger, state.json, and git
  (b) the agent's output is a SUMMARY artifact (markdown report) that does not
      directly mutate any persistent state
  (c) cross-worker conflicts are merge-able by the aggregator (concat, verdict
      vote, lessons union, cross-cli-vote)

If `parallel_eligible: false` (default), the agent MUST NOT be invoked via
fanout-dispatch.sh / cmd_dispatch_parallel. Attempting to do so returns
exit 2 (PROFILE-ERROR) at dispatch time.

A profile WITHOUT the field defaults to `false` (safe default).
```

The rule generalizes the v8.16 Builder single-writer invariant: "Builder is excluded from fan-out — single-writer invariant on the worktree" was *one* role-specific rule. Now it is *the* rule for every role.

---

## Role Taxonomy

| Role | `parallel_eligible` | Reason | Fan-out subtasks |
|---|---|---|---|
| **scout** | `true` | READ-ONLY codebase + research + eval scan | codebase / research / evals |
| **auditor** | `true` | READ-ONLY validators (lint/regression/build-quality/eval-replay) | 4 lenses |
| **retrospective** | `true` | READ-ONLY analysis of cycle artifacts | instinct / gene / topology |
| **plan-reviewer** | `true` | READ-ONLY review across 4 lenses (CEO/Eng/Design/Security) | 4 lenses |
| **evaluator** | `true` | READ-ONLY scoring against rubric | 1 (no fan-out today) |
| **inspirer** | `true` | READ-ONLY external research | 1 (no fan-out today) |
| **builder** | **`false`** | **WRITES files in worktree — single-writer invariant on the cycle's worktree** | n/a |
| **intent** | `false` | WRITES `intent.md`, mutates conversation state | n/a |
| **orchestrator** | `false` | WRITES cycle-state, ledger, dispatches other roles | n/a |
| **tdd-engineer** | `false` | WRITES failing tests (Builder's contract) | n/a |

The boundary is sharp: every role that writes a file Builder later reads, mutates state.json/ledger, or owns a worktree is `false`. Every role that produces a markdown summary the aggregator merges is `true`.

---

## Profile Schema

Each profile under `.evolve/profiles/<role>.json` declares:

```json
{
  "parallel_eligible": true,
  "_parallel_eligible_reason": "READ-ONLY summarizers; aggregator merges via verdict vote",
  "parallel_subtasks": [
    { "name": "codebase", "prompt_template": "..." },
    { "name": "research", "prompt_template": "..." }
  ]
}
```

The `_parallel_eligible_reason` is documentation — not consumed by code. It exists so a future maintainer reviewing why a role was added to the eligible list can read the rationale next to the declaration.

---

## Dispatch-Time Enforcement

`scripts/dispatch/subagent-run.sh:cmd_dispatch_parallel()` reads the field at the top of the function and structurally rejects any agent missing the opt-in:

```bash
local parallel_eligible
parallel_eligible=$(jq -r '.parallel_eligible // false' "$profile" 2>/dev/null)
if [ "$parallel_eligible" != "true" ]; then
    log "PROFILE-ERROR: agent '$agent' is not parallel-eligible..."
    exit 2
fi
```

This is **default-deny**: a profile that lacks the field is rejected. Third-party plugins extending evolve-loop with their own profiles must opt in explicitly. The error message tells them what is missing and how to fix it.

Coverage: `scripts/tests/parallelization-discipline-test.sh` (12 tests) hardcodes the canonical taxonomy and asserts the dispatcher rejects ineligible roles. If a future commit accidentally flips `builder.parallel_eligible: true`, the suite fails loudly.

---

## The Concurrency Cap

`scripts/dispatch/fanout-dispatch.sh:73` resolves the cap via:

```bash
CONCURRENCY="${EVOLVE_FANOUT_CONCURRENCY:-2}"   # default 2 since v8.55
```

The cap is enforced by a bash-3.2-portable FIFO semaphore (FD 9 with N pre-populated tokens; each worker acquires one before spawning, returns it on subshell exit). Workers spawn as soon as a token is free; WAIT-ALL semantics ensure the dispatcher returns only after every worker completes.

### Why default 2

| Subtask count per role | Wall-time at cap=4 | Wall-time at cap=2 | Total token cost |
|---|---|---|---|
| Scout (3) | 1× (all parallel) | ≈ 1.5× (2 + 1 trailing) | identical |
| Auditor (4) | 1× | 2× (two batches) | identical |
| Retrospective (3) | 1× | ≈ 1.5× | identical |

**Total tokens are unchanged; peak burn rate is halved.** For continuous `/loop` runs on subscription quota, the slower-but-steadier burn profile is the difference between completing a multi-hour run and hitting a rate limit. Operators on API plans with no rate concerns can opt back to 4 (or higher) via the env var.

The number `2` (not `1`) preserves the multi-perspective benefit of fan-out — at any given moment ≥ 2 perspectives run in parallel, which is the minimum for MAJORITY-PASS / FAIL-VETO consensus from the v8.53 cross-CLI framework.

### Override

```bash
EVOLVE_FANOUT_CONCURRENCY=4 /loop  # API-plan operator restoring pre-v8.55 default
EVOLVE_FANOUT_CONCURRENCY=1 /loop  # serialize entirely (degenerate)
```

Per-profile overrides are deliberately *not* supported. Per-environment is enough; per-profile is scope creep deferred to v8.56+.

---

## Adding a New Role

When introducing a new agent profile under `.evolve/profiles/<role>.json`:

1. **Declare `parallel_eligible` explicitly.** Default-deny means missing field → rejected at dispatch.
2. **If `true`, add `_parallel_eligible_reason`** explaining how the role satisfies the (a)/(b)/(c) clauses.
3. **If `true`, add the role to the canonical taxonomy** in `scripts/tests/parallelization-discipline-test.sh` so the test asserts your declaration is intentional.
4. **Audit the role's prompt and tools.** If it can `git commit`, write to `state.json`, or modify a Builder-owned file, the answer is `false`.
5. **Document the role in this file's [Role Taxonomy](#role-taxonomy) table.**

If unsure, choose `false`. Promoting later is safe; demoting later is a bug surface.

---

## Why This Matters

| Lens | Statement |
|---|---|
| **Builder single-writer invariant (v8.16)** | Was a one-role-specific rule. Now it is *the* rule, generalized via the canonical taxonomy. |
| **Conway's law** | Pipeline architecture mirrors the read-only/parallel + sequential-write organizational principle. |
| **CAP analog for agent systems** | Sequential-write trades availability (latency) for consistency (correctness compounds over multi-day runs). |
| **Little's Law (queueing theory)** | Average concurrency × wall-time ≈ total work. Halving concurrency doubles wall-time but holds total token cost constant; what changes is the **burn rate** (tokens/second), which is exactly what subscription rate-limiters measure. |

The discipline rule defends *correctness*. The concurrency cap defends *operability*. Together, they allow evolve-loop to run as a continuous loop without (a) write-conflicts compounding errors over hours, or (b) burst token consumption tripping rate limits.

## See Also

- [tri-layer.md](tri-layer.md) — Skill / Persona / Command separation; Pattern 3 fan-out + the 5 endorsed orchestration patterns.
- [phase-architecture.md](phase-architecture.md) — Cycle phase ordering and the trust kernel.
- [multi-llm-review.md](multi-llm-review.md) — Cross-CLI consensus framework (v8.53/v8.54) that fan-out enables.
- `scripts/dispatch/fanout-dispatch.sh` — FIFO semaphore implementation.
- `scripts/dispatch/subagent-run.sh:cmd_dispatch_parallel()` — Profile-side enforcement check.
- `scripts/tests/parallelization-discipline-test.sh` — Regression suite hardcoding the taxonomy.
- `scripts/tests/fanout-dispatch-test.sh` — Concurrency-cap behavior tests (default, override, edge cases).
