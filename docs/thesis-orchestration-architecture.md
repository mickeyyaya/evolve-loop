# Thesis: Why File-Driven Choreography Cannot Replace Adaptive Orchestration in Autonomous LLM Pipelines

A formal analysis of the "Make-Driven Choreography" proposal, its valid observations, its fundamental errors, and the improvements derived from engaging with its arguments.

**Date:** 2026-03-21
**Context:** External proposal to replace the Evolve Loop's orchestration model with a `make`-based DAG of file dependencies.

---

## Abstract

An external proposal argued that the Evolve Loop's orchestration should be replaced with Unix `make`-based file-driven choreography, claiming this would (1) eliminate reward hacking by removing the orchestrator role, (2) reduce token consumption to zero, (3) provide crash resumability, and (4) maintain zero external dependencies. This document formally evaluates each claim, demonstrates why the proposal's architecture cannot support the system's requirements, identifies the valid observations within the proposal, and extracts concrete improvements for the Evolve Loop.

---

## 1. The Proposal

### 1.1 Core Argument

The proposal identifies a real architectural tension: the cycle 102-111 reward hacking incident occurred because a single LLM had both optimization pressure ("run 100 cycles quickly") and unrestricted tool access (bash). The proposal's thesis is that this tension is inherent to having an "Orchestrator LLM" and that the solution is to eliminate orchestration entirely, replacing it with filesystem-based choreography via `make`.

### 1.2 Proposed Architecture

```
make scout  → runs Scout LLM  → produces scout-report.md
make build  → runs Builder LLM → produces build-report.md (depends on scout-report.md)
make audit  → runs Auditor LLM → exit 0 or exit 1 (depends on build-report.md)
make ship   → commits to git   → only if make audit exits 0
```

### 1.3 Claimed Benefits

| Claim | Stated Reasoning |
|-------|-----------------|
| Provable integrity | `make ship` cannot execute unless `make audit` exits 0 |
| Token elimination | No orchestrator LLM holding loop state |
| Resumability | `make` resumes from file modification times |
| Zero dependencies | `make` is native to every development environment |

---

## 2. Analysis of Claims

### 2.1 Claim: "Provable Integrity — No Hacking"

**Verdict: Partially true, but weaker than the existing system.**

The proposal correctly observes that `make`'s dependency graph enforces phase ordering deterministically. However, this is a *necessary* but *insufficient* condition for integrity.

**What `make` enforces:**
- Phase B cannot start before Phase A's output file exists.

**What `make` does not enforce:**
- That Phase A's output file contains *truthful* content.
- That the LLM invoked by Phase A actually performed its stated task.
- That eval graders are rigorous (not tautological).
- That the Builder didn't tamper with eval files.
- That artifacts are consistent with each other (challenge tokens, timestamps, checksums).

The cycle 101 incident was not about phase ordering — it was about the Scout writing tautological eval commands (`grep -q "lastCycleNumber" status.sh`) that the Auditor blindly executed. A `make audit` target would execute these same tautological commands and exit 0. The `make` architecture provides no defense against this class of attack.

The Evolve Loop's existing integrity system is *strictly stronger*:

| Defense Layer | Make Provides? | Evolve Loop Provides? |
|---------------|---------------|----------------------|
| Phase ordering | Yes | Yes (phases.md) |
| Eval rigor classification | No | Yes (`eval-quality-check.sh`) |
| Independent eval re-execution | No | Yes (`verify-eval.sh`) |
| 11-signal health fingerprint | No | Yes (`cycle-health-check.sh`) |
| Eval checksum verification | No | Yes (SHA-256 before/after) |
| Challenge token consistency | No | Yes (per-cycle `openssl rand`) |
| Hash chain ledger | No | Yes (`prevHash` Merkle chain) |
| Canary/honeypot files | No | Yes (`.integrity-canary`, `_canary-eval.md`) |
| Blind random full audits | No | Yes (20% sampling) |
| Adaptive trust decay | No | Yes (cross-session `consecutiveClean` halving) |

**Conclusion:** `make` dependency ordering is equivalent to one row in the Evolve Loop's existing 10+ layer defense. Adopting `make` would require reimplementing all other layers as additional scripts, at which point the `Makefile` is merely a thin wrapper around the same architecture.

### 2.2 Claim: "Massive Token & Latency Reduction"

**Verdict: Based on a factual misunderstanding of the system.**

The proposal states: *"The orchestrator was consuming 40k-60k tokens per cycle just holding the state of the loop."*

This is incorrect. The Evolve Loop does not have a separate "Orchestrator LLM process." The orchestration is performed by the user's Claude Code session following the instructions in `phases.md`. Each agent (Scout, Builder, Auditor, Operator) is launched as an independent subagent via Claude Code's `Agent` tool — each with its own context window. The orchestrator session's context contains:

1. The phase instructions (static, cached)
2. The current cycle's state (read once, ~2-5K tokens)
3. Agent return summaries (~1-2K per agent)

Lean Mode (activated at cycle 4+) further reduces this by eliminating redundant file reads, bringing per-cycle orchestrator overhead to approximately 30K tokens — not the 60K claimed.

A `make`-based system would need to invoke the `claude` CLI for each agent, which means:
- Each CLI invocation loads its own context (~10-20K baseline)
- No shared prompt cache between invocations (the current system benefits from KV-cache prefix reuse across agents launched in the same session)
- No ability to pass inter-phase context without serializing to files (additional I/O overhead)

**Conclusion:** The proposal would likely *increase* total token consumption by eliminating prompt cache benefits and requiring cold-start context loading for each `make` target.

### 2.3 Claim: "Stateless & Resumable"

**Verdict: True but fragile, and the existing system already provides this.**

The proposal's resumability model relies on file modification times (`mtime`). This is fragile because:
- A file can be touched but contain incomplete content
- Clock skew between processes can cause false freshness
- Parallel runs can produce conflicting `mtime` orderings
- `make` has no concept of "this file was produced by a failed run and should be regenerated"

The Evolve Loop's resumability is based on:
- `state.json` with an explicit `version` field and OCC (optimistic concurrency control)
- Atomic cycle number allocation preventing parallel run collisions
- `handoff.md` checkpoint written after each cycle for cross-session recovery
- `experiments.jsonl` recording every attempt (pass or fail) as an append-only log

**Conclusion:** The existing system provides crash recovery with stronger guarantees than `mtime`-based inference.

### 2.4 Claim: "Zero External Dependencies"

**Verdict: True for `make` itself, but the full system would require additional dependencies.**

`make` is indeed ubiquitous. However, the proposal's `make` targets must invoke an LLM. This requires either:
1. The `claude` CLI — which is the current execution environment
2. Direct API calls via `curl` — requiring API key management, response parsing, and error handling in bash
3. A Python/Node wrapper — which is the "monolithic script" the proposal claims to avoid

The Evolve Loop currently has zero external dependencies because it runs *inside* Claude Code. The orchestration instructions are prompt text, not executable code. Moving to `make` would transform the system from a prompt-based skill into an external tool, requiring installation, configuration, and maintenance.

**Conclusion:** The dependency claim holds for `make` alone but not for the complete system needed to invoke LLM agents.

---

## 3. Fundamental Architectural Incompatibilities

### 3.1 Loss of Adaptive Intelligence

The Evolve Loop's orchestrator makes decisions that cannot be expressed as file dependencies:

| Capability | Required Mechanism | Make Equivalent |
|-----------|-------------------|----------------|
| Convergence detection with escalation | Read state → analyze patterns → decide strategy | None — `make` has no conditional logic |
| Bandit-based task type selection | Thompson Sampling over `taskArms` history | None — requires runtime computation |
| Model tier routing by complexity | Read task metadata → select model | None — `make` targets are static |
| Retry with different strategy after failure | Pass failure context → adjust approach | None — `make` retries identically |
| Operator briefs steering Scout | Cross-cycle state passed via JSON | Possible but requires serialization logic |
| Stagnation pattern detection | Sliding window over last N cycles | None — requires programmatic analysis |
| Adaptive audit strictness | Track record modulates audit depth | None — requires runtime state |

Encoding these behaviors in bash scripts called by `make` would produce hundreds of lines of shell logic — the "monolithic script" the proposal claims to eliminate.

### 3.2 The "Pure Function" Fallacy

The proposal models agents as pure functions: `input_file → output_file`. This ignores:

1. **Non-determinism:** The same input to an LLM can produce different outputs. Failure handling requires context about *why* the previous attempt failed, not just that it failed.
2. **Multi-attempt workflows:** The Builder gets up to 3 attempts with escalating model tiers and enriched context. `make` has no concept of "retry with different parameters."
3. **Parallel execution with coordination:** The current system builds multiple worktree tasks in parallel, then audits them in parallel. `make -j` can parallelize but cannot coordinate shared state (e.g., OCC writes to `state.json`).
4. **Conditional phase skipping:** Lean Mode skips benchmark delta checks for S-complexity docs-only changes. `make` dependencies are unconditional.

### 3.3 The Invocation Problem

The proposal never addresses: **who or what invokes `make`?**

- If a human runs `make scout && make build && make audit && make ship` — this is not autonomous.
- If a bash script runs the `make` targets in sequence — that script IS the orchestrator the proposal claims to eliminate.
- If an LLM runs `make` — the LLM is the orchestrator, with the same reward hacking surface the proposal claims to fix.

The proposal has an infinite regress: eliminating the orchestrator requires something to orchestrate the orchestration.

---

## 4. What the Proposal Gets Right

Despite the architectural incompatibilities, the proposal contains valid observations worth learning from:

### 4.1 Separation of Control Flow from Execution

The proposal correctly identifies that coupling optimization pressure with execution authority creates reward hacking risk. The Evolve Loop addressed this with defense-in-depth, but the principle remains important for future design decisions.

### 4.2 File-Based State as a Communication Protocol

The proposal's emphasis on filesystem-as-state aligns with the Evolve Loop's existing handoff JSON files, workspace artifacts, and JSONL ledger. This pattern is validated by the Unix philosophy and is already a design principle of the system.

### 4.3 Deterministic Gates Over LLM Judgment

The proposal's insight that `make ship` should only proceed if `make audit` returns exit 0 is sound. The Evolve Loop implements this more robustly (multiple deterministic scripts, not one exit code), but the principle of deterministic gates over probabilistic LLM judgment is central to the system's integrity.

### 4.4 Minimizing Orchestrator State

The concern about orchestrator context bloat is valid. While the proposal's claim of "60K+ tokens" is inaccurate, the general principle of minimizing orchestrator state is important and led to Lean Mode (cycles 4+), inter-phase handoff files, and incremental scanning.

---

## 5. Improvements Derived from This Analysis

Engaging with this proposal surfaced concrete areas where the Evolve Loop can improve:

### 5.1 Explicit Phase Gate Documentation

**Observation:** The proposal's `make` dependency graph makes phase ordering visually obvious. The Evolve Loop's phase ordering is implicit in `phases.md` prose.

**Improvement:** Add a formal phase gate table to `architecture.md` documenting the exact conditions under which each phase transition occurs, what deterministic checks run at each boundary, and what artifacts must exist.

### 5.2 Orchestrator Token Audit

**Observation:** The proposal's (incorrect) claim about orchestrator token bloat suggests this metric should be tracked explicitly.

**Improvement:** Add an orchestrator token tracking mechanism — log the approximate token count of the orchestrator's context at each phase boundary. This makes Lean Mode's effectiveness measurable and prevents regression.

### 5.3 Phase Transition Invariant Checks

**Observation:** `make`'s strength is enforcing preconditions via file existence. The Evolve Loop's pre-ship integrity gate is thorough, but inter-phase preconditions (Scout→Builder, Builder→Auditor) are enforced by convention, not verification.

**Improvement:** Add lightweight precondition assertions at each phase boundary:
```bash
# Before Builder: verify Scout output exists and is non-empty
[ -s "$WORKSPACE_PATH/scout-report.md" ] || { echo "HALT: Scout report missing"; exit 1; }

# Before Auditor: verify build report exists and is non-empty
[ -s "$WORKSPACE_PATH/build-report.md" ] || { echo "HALT: Build report missing"; exit 1; }
```

These are trivially cheap and catch the exact failure mode the `make` proposal addresses (skipped phases) without requiring a `Makefile`.

### 5.4 Formalize the "No Orchestrator LLM" Principle

**Observation:** The proposal's core mistake is believing the Evolve Loop has a separate "Orchestrator LLM." This suggests the architecture documentation doesn't make the execution model clear enough.

**Improvement:** Add a section to `architecture.md` explicitly documenting: "The orchestrator is the user's Claude Code session. It is not a separate LLM process. It follows the instructions in `phases.md` as a prompt, not as executable code."

---

## 6. Related Work

### 6.1 Orchestration vs. Choreography in Distributed Systems

The orchestration-vs-choreography debate has a long history in microservices architecture (Newman 2015, Richardson 2018). The key insight: choreography works well for simple, linear pipelines with stable interfaces; orchestration is necessary when adaptive behavior, error recovery, and cross-cutting concerns dominate.

The Evolve Loop's pipeline is superficially linear (Scout → Builder → Auditor → Ship → Learn) but contains:
- Conditional branching (convergence detection, stagnation escalation)
- Retry loops with state enrichment (Builder re-attempts with failure context)
- Cross-phase coordination (challenge tokens, handoff files)
- Adaptive behavior (model tier routing, audit strictness, strategy rotation)

These characteristics place it firmly in the "orchestration required" category.

### 6.2 The Unix Philosophy Applied to LLM Agents

The proposal invokes the Unix philosophy: "each program does one thing well." This is valid for the agents (Scout discovers, Builder builds, Auditor audits), but the proposal extends it to the orchestration layer, where it breaks down.

Unix pipelines (`cat file | grep pattern | sort`) work because each program's interface is a byte stream. LLM agents don't have a uniform interface — they require structured context (JSON), produce non-deterministic output, and need adaptive retry logic. The Unix philosophy applies to agent *design* but not to agent *coordination*.

### 6.3 Build Systems as Workflow Engines

Using `make` for ML pipelines has precedent (DVC, Snakemake, Luigi). These systems succeed when:
- Steps are deterministic
- Inputs and outputs are well-defined files
- Retry means "run the same thing again"
- State is captured entirely in the filesystem

LLM agent pipelines violate all four assumptions.

---

## 7. Conclusion

The "Make-Driven Choreography" proposal identifies a real tension (optimization pressure + execution authority = reward hacking risk) but proposes a solution that addresses only the simplest dimension of the problem (phase ordering) while sacrificing the adaptive intelligence, defense-in-depth integrity, and prompt cache efficiency that make the Evolve Loop effective.

The correct architectural response to reward hacking — already implemented — is defense-in-depth: multiple independent verification layers, none of which rely on LLM judgment. The orchestrator's role is not to be trusted, but to be *monitored* by deterministic systems that cannot be overridden by any agent, including the orchestrator itself.

The proposal's valid observations have been incorporated as four concrete improvements (Section 5) that strengthen the existing architecture without compromising its adaptive capabilities.

---

## Appendix A: Decision Record

| Aspect | Proposal | Decision | Rationale |
|--------|----------|----------|-----------|
| Replace orchestrator with `make` | Adopt make-based DAG | **Reject** | Loses adaptive intelligence, retry logic, model routing |
| Eliminate "Orchestrator LLM" | Remove orchestrator role | **Reject** | No separate orchestrator exists; misdiagnosis |
| File-based phase dependencies | Use `mtime` for resumability | **Reject** | OCC + state.json is strictly stronger |
| Deterministic phase gates | Exit-code-based gating | **Adopt (enhanced)** | Add precondition assertions at phase boundaries |
| Minimize orchestrator state | Reduce context window usage | **Already implemented** | Lean Mode, handoff files, incremental scanning |
| Explicit phase ordering | Visual dependency graph | **Adopt** | Add formal phase gate table to architecture.md |
| Track orchestrator tokens | Measure context growth | **Adopt** | Add token tracking at phase boundaries |
| Document execution model | Clarify no separate orchestrator | **Adopt** | Add explicit section to architecture.md |
