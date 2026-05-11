# Self-Healing Agents

> Reference document for automatic recovery and self-repair patterns in agent systems.
> Apply these patterns to build agents that detect faults, diagnose root causes, and
> restore correct operation without human intervention.

## Table of Contents

1. [Fault Detection](#fault-detection)
2. [Healing Architecture](#healing-architecture)
3. [Recovery Strategies](#recovery-strategies)
4. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
5. [Prior Art](#prior-art)
6. [Anti-Patterns](#anti-patterns)

---

## Fault Detection

Detect faults early and categorize them before attempting any recovery action.

| Fault Type | Detection Method | Signal | Threshold | Severity |
|---|---|---|---|---|
| **Timeout** | Wall-clock timer per agent phase | Elapsed time exceeds budget | 2x expected duration | HIGH |
| **Output Validation Failure** | Schema check on agent output | Missing required fields, wrong types, empty output | Any schema violation | CRITICAL |
| **Eval Grader Failure** | Automated eval suite after build | Eval score below passing threshold | Score < configured minimum | CRITICAL |
| **State Corruption** | Checksum or hash comparison on workspace files | Unexpected file modifications, missing files, conflicting changes | Any untracked mutation | HIGH |
| **Resource Exhaustion** | Monitor context window usage, disk, memory | Token count approaching limit, disk full, OOM | >80% utilization | MEDIUM |

### Detection Principles

| Principle | Rationale |
|---|---|
| Detect at the boundary, not in the core | Keep detection logic separate from business logic |
| Prefer deterministic checks over heuristic checks | Reduce false positives that trigger unnecessary healing |
| Log every detection event with full context | Enable post-mortem analysis and pattern recognition |
| Set distinct thresholds per fault type | Avoid one-size-fits-all thresholds that miss edge cases |

---

## Healing Architecture

Use a four-agent pattern to separate concerns in the healing pipeline.

| Role | Responsibility | Input | Output |
|---|---|---|---|
| **Detector** | Monitor agent execution and raise fault signals | Agent output, timing data, resource metrics | Fault event with type, severity, and context |
| **Diagnoser** | Analyze the fault event to identify root cause | Fault event, execution logs, prior fault history | Diagnosis record: root cause, contributing factors, confidence |
| **Remediator** | Execute the appropriate recovery strategy | Diagnosis record, available recovery options | Recovery action taken, new agent state |
| **Verifier** | Confirm the recovery restored correct operation | Post-recovery output, original success criteria | Pass/fail verdict, residual risk assessment |

### Pipeline Flow

| Step | Action | Gate Condition |
|---|---|---|
| 1 | Detector raises fault event | Fault signal exceeds threshold |
| 2 | Diagnoser analyzes root cause | Diagnosis confidence > 0.7 |
| 3 | Remediator selects and executes strategy | Strategy matches diagnosis category |
| 4 | Verifier runs success criteria | All evals pass post-recovery |
| 5 | If Verifier fails, escalate or re-enter at step 2 | Max retry count not exceeded |

### Role Boundaries

| Constraint | Rationale |
|---|---|
| Detector must not attempt remediation | Separation prevents masking faults |
| Diagnoser must not modify agent state | Diagnosis should be read-only to avoid side effects |
| Remediator must log every action taken | Enable rollback and audit trail |
| Verifier must use the same criteria as the original eval | Prevent weakened acceptance standards post-recovery |

---

## Recovery Strategies

Select the appropriate strategy based on fault type and diagnosis.

| Strategy | When to Use | Implementation | Max Attempts | Risk |
|---|---|---|---|---|
| **Retry with Backoff** | Transient failures (timeouts, rate limits) | Exponential backoff: 1s, 2s, 4s, 8s | 3 | Low — bounded by attempt cap |
| **Retry with Parameter Adjustment** | Output validation failure due to prompt ambiguity | Modify temperature, add constraints, clarify instructions | 2 | Medium — parameter changes may shift behavior |
| **Fallback to Simpler Approach** | Complex strategy fails repeatedly | Reduce scope, use simpler model, decompose task | 1 | Medium — may produce lower-quality output |
| **Checkpoint Rollback** | State corruption, partial writes | Restore workspace to last known-good checkpoint | 1 | Low — deterministic restore |
| **Escalation to Human** | All automated strategies exhausted, or critical safety concern | Halt execution, emit structured alert with full context | N/A | None — human decides next step |

### Strategy Selection Matrix

| Fault Type | First Strategy | Second Strategy | Final Strategy |
|---|---|---|---|
| Timeout | Retry with backoff | Fallback to simpler approach | Escalation |
| Output validation failure | Retry with parameter adjustment | Fallback to simpler approach | Escalation |
| Eval grader failure | Retry with parameter adjustment | Checkpoint rollback + retry | Escalation |
| State corruption | Checkpoint rollback | Escalation | — |
| Resource exhaustion | Fallback to simpler approach | Escalation | — |

---

## Mapping to Evolve-Loop

Map each self-healing concept to its concrete implementation in the evolve-loop system.

| Self-Healing Concept | Evolve-Loop Implementation | Details |
|---|---|---|
| **Detector** | `phase-gate.sh` | Run at every phase boundary; validate artifacts exist and pass structural checks |
| **Diagnoser** | `failedApproaches` array in cycle state | Store root cause, attempted strategy, and failure context for each failed attempt |
| **Remediator — Retry** | Builder retry (max 3 attempts) | Builder re-executes with adjusted prompt incorporating failure context from `failedApproaches` |
| **Remediator — Rollback** | Worktree discard on failure | Delete the worktree and start fresh, preventing state corruption from propagating |
| **Remediator — Fallback** | Scout selects simpler task on repeated failure | After 3 Builder failures, Scout deprioritizes the task and selects an alternative |
| **Verifier** | Auditor agent with eval suite | Auditor runs test suite, checks coverage, validates rubric criteria after Builder completes |
| **Escalation** | Operator HALT signal | Operator emits HALT when all retries exhausted or critical integrity violation detected |

### Evolve-Loop Healing Flow

| Phase | Fault Detected | Recovery Action |
|---|---|---|
| Scout | Scout produces invalid report | Retry Scout with tighter constraints; HALT after 2 failures |
| Builder | Build fails tests | Store in `failedApproaches`, retry with diagnosis context (max 3) |
| Builder | Worktree has state corruption | Discard worktree, create fresh worktree, retry from scratch |
| Auditor | Audit score below threshold | Return to Builder with Auditor feedback; retry build |
| Ship | Merge conflict or CI failure | Rollback merge, re-run Builder with updated base branch |
| Learn | Metrics extraction fails | Log warning, skip metrics update, continue to next cycle |

---

## Prior Art

Draw from established self-healing systems in other domains.

| Domain | System / Concept | Key Insight for Agents | Reference |
|---|---|---|---|
| **Biology** | Immune system: innate + adaptive response | Layer fast heuristic checks (innate) with slower learned responses (adaptive); maintain a memory of past faults | Hofmeyr & Forrest, "Architecture for an Artificial Immune System" |
| **Biology** | DNA repair mechanisms | Detect errors at checkpoints, repair before propagating; prefer high-fidelity repair over fast repair | Alberts et al., "Molecular Biology of the Cell" |
| **Infrastructure** | Kubernetes self-healing (liveness/readiness probes, restart policies) | Define health checks declaratively; let the orchestrator handle restarts; separate probe logic from application logic | Kubernetes documentation, pod lifecycle |
| **Infrastructure** | Netflix Chaos Engineering (Chaos Monkey, FIT) | Inject faults intentionally to validate healing paths; test recovery in production-like conditions | Basiri et al., "Chaos Engineering" (O'Reilly) |
| **Reinforcement Learning** | RL from failure history | Treat each failure as a negative reward signal; update policy to avoid repeated failure modes; maintain a failure replay buffer | Schaul et al., "Prioritized Experience Replay" |
| **Software** | Erlang/OTP supervisor trees | Isolate failures to individual processes; restart failed processes without affecting siblings; use escalation hierarchy (restart → restart parent → shutdown) | Armstrong, "Programming Erlang" |
| **Software** | Circuit breaker pattern | Track failure rate; open the circuit after threshold exceeded to prevent cascading failures; half-open to test recovery | Nygard, "Release It!" |

---

## Anti-Patterns

Avoid these common mistakes when implementing self-healing agents.

| Anti-Pattern | Description | Consequence | Correct Alternative |
|---|---|---|---|
| **Infinite Retry Loop** | Retry without a maximum attempt count or backoff | Resource exhaustion, blocked pipeline, wasted tokens | Cap retries at 3; escalate after exhaustion |
| **Healing Without Diagnosis** | Apply a fix without identifying root cause | Mask the real problem; recurrence guaranteed | Always run Diagnoser before Remediator |
| **Masking Failures** | Catch errors silently and report success | Corrupt downstream state; false confidence in system health | Log every fault; never suppress error signals |
| **Over-Aggressive Self-Repair** | Trigger healing on minor deviations or noise | Unnecessary restarts; instability from constant churn | Set meaningful thresholds; tolerate non-critical variance |
| **Healing the Symptom** | Fix the output without fixing the cause | Repeated failures on similar inputs; no learning | Store root cause in `failedApproaches`; adjust strategy |
| **Skipping Verification** | Assume recovery succeeded without re-running evals | Broken state propagates to next phase | Always run Verifier after every Remediator action |
| **Single Point of Healing** | Only one recovery path for all fault types | Cannot handle diverse failure modes | Maintain a strategy matrix indexed by fault type |
