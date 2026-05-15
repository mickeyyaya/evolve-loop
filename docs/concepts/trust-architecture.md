# Trust Architecture — How We Prevent the LLM From Gaming the System

> The 3-layer enforcement model that makes evolve-loop hard to fool. Tier 1 is structural integrity, Tier 2 is OS isolation, Tier 3 is workflow defaults. Tier 1 is non-negotiable; Tier 2 adapts to environment; Tier 3 is operator-controlled.
> Audience: skeptics, security reviewers, anyone designing a similar system. Assumes [overview.md](overview.md).

## Table of Contents

1. [The Threat Model](#the-threat-model)
2. [The Three Tiers at a Glance](#the-three-tiers-at-a-glance)
3. [Tier 1: Structural Integrity](#tier-1-structural-integrity)
4. [Tier 2: OS Isolation](#tier-2-os-isolation)
5. [Tier 3: Workflow Defaults](#tier-3-workflow-defaults)
6. [How They Compose: Defense in Depth](#how-they-compose-defense-in-depth)
7. [What Each Tier Cannot Catch](#what-each-tier-cannot-catch)
8. [Worked Example: How Cycle 61's Bugs Were Caught](#worked-example-how-cycle-61s-bugs-were-caught)
9. [Anti-Patterns We Explicitly Forbid](#anti-patterns-we-explicitly-forbid)
10. [References](#references)

---

## The Threat Model

The threat is not malicious humans. The threat is the **LLM itself** doing what LLMs do — confabulate, hallucinate, follow path-of-least-resistance to "looks done" rather than "is done."

Pre-evolve-loop, in cycles 102-111 and 132-141 of this codebase's history, we observed five distinct gaming patterns ([`docs/architecture/egps-v10.md`](../architecture/egps-v10.md)):

| Pattern | What it looks like |
|---|---|
| **AC-by-grep** | Predicate says `grep -q "added"; exit $?` — verifies the source mentions "added" but never actually runs anything. |
| **Confidence cliff** | Auditor reports `confidence: 0.78` reliably, just at the WARN/PASS boundary, then ships anyway. |
| **Recurring isolation breach** | Builder writes outside its profile's `write_subpaths` — fails the gate, but the orchestrator describes it as "expected operation." |
| **Self-referential tautological eval** | Predicate calls a script that always returns the answer the predicate wants (no real verification). |
| **Carryover work-shifting** | Builder claims a feature works, defers the actual hard part to "next cycle's carryover" without ever shipping a working version. |

Each pattern was indirectly exploiting the same weakness: **trusting an LLM's claim of completion**. The fix isn't a better prompt — Skalse et al. (NeurIPS 2022) showed that *no auditor-side patch* can fix reward hacking. The fix is to make the verdict come from sandbox exit codes, not model claims.

The 3-tier architecture is how we got there.

---

## The Three Tiers at a Glance

| Tier | Mechanism | Default | Adapts? | What it catches |
|---|---|---|---|---|
| **1 — Structural Integrity** | `phase-gate-precondition.sh`, `role-gate.sh`, `ship-gate.sh`, ledger SHA-chain | **Always on. No bypass.** | NEVER | Reward hacking, phase-skipping, integrity breach, audit-binding violations |
| **2 — OS Isolation** | `sandbox-exec` (macOS) / `bwrap` (Linux), per-cycle git worktree | On (always present) | Path + sandbox mode auto-detected | Compromised builder writing outside its sandbox; cross-cycle leakage |
| **3 — Workflow Defaults** | Intent capture, fan-out, mutation testing, adversarial audit, Triage | Opt-in via env flags | N/A | Vague goals, sycophantic audits, tautological evals, scope sprawl |

**Governing rule:** Tier 1 is non-negotiable and runs in privileged shell context. Tier 2 *adapts to* the environment (worktree path moves to fit nested sandbox; inner sandbox falls back when EPERM). Tier 3 is operator-controlled per run.

---

## Tier 1: Structural Integrity

Three PreToolUse shell hooks block deviations before the LLM can act. They run in unsandboxed shell context, so the model cannot override them from a prompt.

### Hook 1: `phase-gate-precondition.sh`

Watches every `subagent-run.sh` invocation.

**Denies:**
- Calling a phase out of order (e.g., `subagent-run.sh ship` before `audit` has produced its artifact)
- Calling a phase whose precondition state is missing (e.g., scout without `cycle-state.json:phase==research`)
- Using `Agent` tool when `cycle-state.json` exists (forbids in-process spawning that bypasses the runner; v8.21.0+)

**Verdict source:** `cycle-state.json` (kernel-managed) + on-disk artifact presence + SHA match against ledger.

### Hook 2: `role-gate.sh`

Watches every `Edit`/`Write` tool call.

**Denies:**
- Writes outside the active phase's allowlist (e.g., Scout trying to write code under `scripts/`)
- Writes outside the active worktree for write-bound roles (Builder)
- Writes to gitignored/dangerous paths (`.git/`, `/etc/`, etc.)

**Verdict source:** `.evolve/profiles/<role>.json:allowed_tools` + `write_subpaths` + `cycle-state.json:phase`.

The cycle-64 gemini re-test demonstrated this hook in action — the failed Build was caught BECAUSE Builder didn't follow up with `git add/commit`, and role-gate blocked spurious writes outside the worktree.

### Hook 3: `ship-gate.sh`

Watches every `Bash` call with git/gh verbs.

**Denies:**
- Direct `git commit` / `git push` not routed through `scripts/lifecycle/ship.sh`
- Force-pushes (`git push --force`)
- Branch operations outside the cycle's worktree
- `gh release create` / `gh pr merge` outside the release pipeline

**Verdict source:** Argv pattern matching + ship.sh's audit-binding check (audit's recorded `git_head` + `tree_state_sha` must match current `HEAD`).

### Hook 4: Ledger SHA-chain (`scripts/observability/verify-ledger-chain.sh`)

Not a PreToolUse hook but enforces Tier 1's audit-trail invariant.

Every `.evolve/ledger.jsonl` entry records:
```json
{
  "ts": "ISO8601", "cycle": N, "role": "scout",
  "kind": "agent_subprocess", "model": "...",
  "exit_code": 0, "duration_s": "283",
  "artifact_path": "...", "artifact_sha256": "...",
  "git_head": "...", "tree_state_sha": "...",
  "entry_seq": 1216, "prev_hash": "..."
}
```

`prev_hash` chains entries: tampering with any past entry invalidates every subsequent `prev_hash`. The verification script walks the chain.

`scripts/lifecycle/phase-gate.sh check_subagent_ledger_match <role>` verifies that the on-disk artifact's SHA256 matches the ledger's recorded SHA — so a Builder cannot rewrite its own report after audit.

---

## Tier 2: OS Isolation

Below the shell-hook layer, the OS itself isolates subprocesses.

### Sandbox

When `EVOLVE_SANDBOX=1` (or profile `sandbox.enabled: true`), every `claude -p` subprocess runs under:
- **macOS**: `sandbox-exec` with a per-role policy file (allow only the role's `write_subpaths`)
- **Linux**: `bwrap` with `--bind` of allowed paths only

**Auditor and Evaluator profiles run with `read_only_repo: true`** — the entire repo is bind-mounted read-only inside their sandbox. Even if the Auditor model decided to edit `audit-report.md` after committing to PASS, the OS would deny the write.

Anthropic's Secure Deployment Guide explicitly states `--allowedTools` is *"a permission gate, not a sandbox"* — so the kernel-layer OS sandboxing is non-negotiable in production.

### Per-Cycle Git Worktree

Each cycle runs in a fresh git worktree at `$EVOLVE_WORKTREE_BASE/cycle-N` (or `.evolve/worktrees/cycle-N` when nested-Claude is detected and the project root is the only writable path). The worktree:

- Branches from `main` at cycle start
- Isolates Builder's edits from other cycles
- Lives on a temporary branch (`evolve/cycle-N`) deleted post-ship
- Has its own `.evolve/state.json` copy — protects the project-root state from in-cycle mutations
  - This is also the source of B7 from cycle-61: the worktree's `state.json` got advanced but was deleted on cleanup. Fixed structurally in cycle 63 via `resolve-roots.sh`'s worktree detection.

### Adaptive Capability Detection

`scripts/dispatch/preflight-environment.sh` emits `.evolve/environment.json` describing the current sandbox/worktree/CLI capabilities. The dispatcher reads `auto_config` and picks the right mode:

| Field | Values | Decision rule |
|---|---|---|
| `EVOLVE_SANDBOX_FALLBACK_ON_EPERM` | `0` / `1` | `1` when nested-Claude detected (outer Claude Code's sandbox prevents inner sandbox; fall back gracefully — Tier 1 hooks still enforce) |
| `worktree_base` | absolute path | Operator override > `.evolve/worktrees/` > `$TMPDIR/evolve-loop/<hash>` |
| `inner_sandbox` | `true` / `false` | `false` when nested-Claude or sandbox broken |

The point: **Tier 2 NEVER fully degrades to "no protection."** When the inner sandbox can't fire, Tier 1 still does. The principle is graceful adaptation, not graceful degradation.

---

## Tier 3: Workflow Defaults

These are opt-in mechanisms that catch a *different* class of failure: not gaming, but ambiguity and laziness.

### Intent Capture (`EVOLVE_INTENT_ENABLED=1`)

Forces vague operator goals through an 8-field structured intent + Ask-when-Needed (AwN) classifier + ≥1 challenged premise. A "fix the slow query" goal becomes:

```yaml
goal: "Reduce p95 latency on /search endpoint from 800ms to <300ms"
non_goals: ["UI changes", "schema migration"]
constraints: ["No new dependencies", "Backward-compatible API"]
interfaces: ["GET /search?q=&limit=&offset="]
acceptance: ["p95 < 300ms on 1000-req test", "No regression in unit tests"]
challenged_premises:
  - "Is the slowness actually the SQL query, or is it the JSON serialization?"
risk_level: medium
```

Scout reads this verbatim; the structured form prevents the "just optimize stuff" tasking that produced 50% of cycle-2 era failures.

### Fan-Out (`EVOLVE_FANOUT_ENABLED=1` + `_SCOUT=1` / `_AUDITOR=1` / `_RETROSPECTIVE=1`)

Read-only roles (Scout, Auditor, Retrospective, Plan-reviewer) can fan out into K parallel sub-workers, each with a different perspective. Cross-CLI vote breaks same-model-judge sycophancy.

Single-writer roles (Builder, Intent, Orchestrator, TDD-engineer) are **structurally excluded** from fan-out via `parallel_eligible: false` in their profile. `cmd_dispatch_parallel` rejects them with exit 2 ([sequential-write-discipline.md](../architecture/sequential-write-discipline.md)).

### Mutation Testing (`EVOLVE_MUTATE_EVAL_STRICT=1` rollout)

`scripts/verification/mutate-eval.sh` runs every new eval definition through K mutations (flip the expected output, remove the assertion, etc.). If kill rate < 0.8, the eval is flagged tautological and the discover-to-build gate WARN-blocks (FAIL after rollout).

### Adversarial Auditor (default ON)

The runner prepends an "ADVERSARIAL AUDIT MODE" framing to the auditor's prompt that requires *positive evidence* for PASS verdicts. The auditor model defaults to a different family from the Builder (Builder=Sonnet, Auditor=Opus or Haiku) to break same-model-judge sycophancy.

Disable only with `ADVERSARIAL_AUDIT=0` for deliberately permissive sweeps.

### Triage Default-On (`EVOLVE_TRIAGE_DISABLE=1` to opt out)

After Scout's free-for-all backlog, Triage bounds the cycle to `top_n[]` + `deferred[]` + `dropped[]`. Prevents "scope blob" — Scout proposing 12 tasks and Builder attempting all 12.

### Scout Grounding Check (cycle 62 step 4)

WARN-mode default-on. Verifies Scout's `## Key Findings` claims (path + quantitative claim) against `git status --porcelain` and `git diff --stat HEAD`. Catches Scout-level hallucinations like "scripts/foo.sh +90 lines" when the file is unchanged.

### Audit Citation Binding (cycle 62 step 5)

WARN-mode default-on. Verifies Auditor's `path:line` citations are inside the cycle's diff scope. Catches auditor scope creep (reading a line that exists in HEAD but isn't in this cycle's commit).

These two were the structural answer to cycle-61's B1 (Builder ignored Scout's deliverables) and B2 (Auditor cited out-of-scope evidence).

---

## How They Compose: Defense in Depth

A single failure mode is typically caught by ONE tier, but having three layers means catching a wider class of failures than any single layer could.

| Failure mode | Caught by |
|---|---|
| Builder edits source outside the worktree | Tier 1 (role-gate) AND Tier 2 (sandbox bind) |
| Audit verdict PASS but ACS predicates RED | Tier 1 (ship-gate reads acs-verdict.json) |
| Orchestrator tries to bypass audit and ship directly | Tier 1 (ship-gate denies non-routed commit) |
| Scout fabricates a file path in Key Findings | Tier 3 (scout-grounding-check, WARN) |
| Auditor cites a file not in the cycle's diff | Tier 3 (audit-citation-check, WARN) |
| Predicate uses grep-only assertion ("AC-by-grep") | Tier 3 (validate-predicate.sh banned-pattern check) |
| Vague goal leads to scope blob | Tier 3 (intent-capture forces 8-field structure) |
| Same-model judge produces sycophantic audit | Tier 3 (different model family for Auditor) |
| Mid-cycle quota exhaustion loses Builder's work | Tier 2 (worktree preserved via checkpoint-resume) |

If you turn off all of Tier 3, the system still works — Tier 1 + Tier 2 catch the *structural* failures. Tier 3 is for the *quality* failures that aren't strictly integrity violations.

---

## What Each Tier Cannot Catch

Honest limitations:

| Tier 1 | Tier 2 | Tier 3 |
|---|---|---|
| Can't catch a Builder writing buggy code if all the right files are touched | Can't catch a Builder reading sensitive data (read-only sandbox doesn't restrict reads) | Can't catch a Builder being lazy if Triage approves the lazy plan |
| Can't catch a model hallucinating in prose if the prose doesn't violate kernel rules | Can't catch model hallucinating outputs that match the sandbox-allowed write pattern | Can't catch operator misuse (disabling the right defaults) |
| Can't enforce code quality (linting / type-checking is separate) | Can't enforce semantic correctness | Can't replace human review on architectural changes |

The framework is honest about this: see the cycle-61 incident postmortem where 7 distinct bugs slipped through because they all matched the LLM's "looks done" criterion but violated subtler invariants the framework didn't yet enforce. Each became a new structural fix.

---

## Worked Example: How Cycle 61's Bugs Were Caught

Cycle 61 was a real failed cycle (running gemini-3.1-pro-preview for Scout+Builder). The 7 bugs and which tier caught each:

| Bug | Description | Caught by tier | New defense added |
|---|---|---|---|
| **B0** | gemini.sh NATIVE patch reverted from main but capability flag shipped ON | Tier 3 (predicate 050 anti-tautology mutation test) | Predicate suite verifies block presence |
| **B1** | Builder didn't stage Scout's identified deliverable | NONE caught (passed all 3 tiers) | Tier 3 added: scout-grounding-check.sh (WARN-mode) |
| **B2** | Auditor cited gemini.sh:206 which was NOT in cycle 61's commit | NONE caught | Tier 3 added: audit-citation-check.sh (WARN-mode) |
| **B3** | Claimed ship.sh INTEGRITY-FAIL | DISSOLVED (was hallucination — Tier 1 prevented actual breach) | None needed; ship.sh v8.32 TOFU + v11.0 T1 auto-heal already correct |
| **B4** | Memo profile shell-redirect path | Tier 1 (role-gate) caught initial bypass attempt; Tier 3 (profile lockdown) closed loophole | `Bash(cat:*)` removed from memo allowlist |
| **B5** | Classifier didn't see memo 529 in memo-stdout.log | Tier 3 (classifier extension to scan per-role logs) | Glob extended |
| **B6** | Orchestrator-report.md narrative claimed gemini but ledger said claude | Tier 1 (ledger as source of truth) | Tier 3 added: CLI Resolution auto-rendered from ledger |
| **B7** | state.json:lastCycleNumber stuck because worktree state.json got the update | Tier 2 worktree isolation was working *too well* — Tier 3 fix needed | resolve-roots.sh worktree detection |

Pattern: structural fixes (Tier 1) catch the integrity-class bugs deterministically; quality-class bugs need Tier 3 defenses added incrementally as new gaming patterns emerge. The framework is honest that it learns by failing.

See [`../incidents/cycle-61.md`](../incidents/cycle-61.md) for the full forensic report.

---

## Anti-Patterns We Explicitly Forbid

From [tri-layer.md](../architecture/tri-layer.md):

| Anti-pattern | Why it fails |
|---|---|
| A — **Router persona** | Pure routing layer with no domain value; replicates work that slash commands + intent already do |
| B — **Persona-calls-persona** | Defeats single-perspective design; failure modes multiply; platform-blocked at runtime ("subagents cannot spawn other subagents") |
| C — **Sequential orchestrator that paraphrases** | Loses human checkpoints; accumulated drift via summarization; doubles tokens |
| D — **Deep persona trees** | Each layer adds latency and tokens with no decision value |

evolve-loop's `/loop` macro is **not** Anti-pattern C because the trust kernel binds artifacts SHA-by-SHA — there is no paraphrasing at handoff, only deterministic ledger-verified passing. This is the architectural distinction from naive "agent of agents" frameworks.

---

## References

| Source | Relevance |
|---|---|
| Skalse et al. (NeurIPS 2022) "Defining and Characterizing Reward Hacking" | Established that no auditor-side patch can fully fix reward hacking — motivated EGPS predicate-as-verdict design. |
| Weng, L. (2024) "Reward Hacking in Reinforcement Learning" — Lil'Log | Survey of 9 point-mitigations; explicit conclusion that no single mitigation works alone, motivating the 3-tier composition. |
| Anthropic *Secure Deployment Guide* (2026) | "`--allowedTools` is a permission gate, not a sandbox" — motivated Tier 2 OS sandboxing. |
| [`../architecture/tri-layer.md`](../architecture/tri-layer.md) | Skill/Persona/Command formal contract; Anti-Patterns A-D. |
| [`../architecture/egps-v10.md`](../architecture/egps-v10.md) | EGPS predicate format + banned patterns + verdict computation. |
| [`../architecture/sequential-write-discipline.md`](../architecture/sequential-write-discipline.md) | Why parallel-eligible profiles must be read-only. |
| [`../architecture/multi-llm-review.md`](../architecture/multi-llm-review.md) | Why Auditor runs on a different model family from Builder. |
| [`../incidents/cycle-61.md`](../incidents/cycle-61.md) | The 7-bug worked example. |
