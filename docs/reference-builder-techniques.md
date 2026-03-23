> Read this file when running Phase 2 (BUILD). Contains research-backed techniques for implementation, error recovery, structured output, prompt variants, and budget awareness.

## Contents
- [Targeted Error Recovery](#targeted-error-recovery) — classify failures before retry (AgentDebug)
- [Process Reward Trajectory](#process-reward-trajectory) — score each build step (AgentPRM)
- [Prompt Variant Switching](#prompt-variant-switching) — alternate patterns on failure (AutoPDL)
- [Draft-Then-Constrain Output](#draft-then-constrain-output) — structured output without quality loss (DCCD)
- [Budget-Aware Scaling](#budget-aware-scaling) — explore/exploit by remaining budget (BATS)
- [Uncertainty Gating](#uncertainty-gating) — check uncertainty before state changes (AUQ)
- [Secure Code Patterns](#secure-code-patterns) — layered security eval approach
- [Configuration Variant Tracking](#configuration-variant-tracking) — log config for joint optimization (ARTEMIS)

---

## Targeted Error Recovery

**Source:** AgentDebug (arXiv:2509.25370)

**When:** Builder receives a retry after Auditor FAIL.

**Read `retryContext` from Auditor.** Classify failure into 5 dimensions:

| Dimension | Symptom | Fix Strategy |
|-----------|---------|-------------|
| Memory | Read wrong file, applied stale instinct | Re-read target files, check instinct currency |
| Reflection | Self-verified as PASS but Auditor found FAIL | Add per-grader result logging, not aggregate |
| Planning | Wrong file scope, missed dependency | Re-enumerate `filesToModify` from acceptance criteria |
| Action | Wrong edit target, misapplied diff | Verify edit location matches design step |
| System | Eval timeout, missing tool | Report capability gap, do not retry blindly |

**Rule:** Never retry without `retryContext`. If none provided, request it from the Auditor report.

---

## Process Reward Trajectory

**Source:** AgentPRM (arXiv:2502.10325)

**When:** After each build attempt (pass or fail).

Decompose execution into 4 phases and score each:

| Phase | What to Score | Positive Signal |
|-------|--------------|----------------|
| Design | Explicit file list + approach rationale present? | Files enumerated before implementation |
| Discovery | Reads precede writes for each modified file? | No blind edits |
| Implementation | Changed files match design phase file list? | No scope creep |
| Self-verification | All graders executed with per-grader results? | Not just aggregate PASS |

Record in `experiments.jsonl` with per-phase scores.

---

## Prompt Variant Switching

**Source:** AutoPDL (arXiv:2504.04365)

**When:** Same task type fails twice (eval pass rate < 0.7).

| Failure Count | Action |
|--------------|--------|
| 1st failure | Retry with same prompt pattern |
| 2nd failure | Switch pattern: CoT → ReAct, or ReAct → structured decomposition |
| 3rd failure | Drop task, log both patterns as failed |

Log `promptPattern` field in `experiments.jsonl` for Operator analysis.

---

## Draft-Then-Constrain Output

**Source:** DCCD (arXiv:2603.03305)

**When:** Producing structured output (build-report tables, JSON decision traces).

**Protocol:**
1. **Draft stage:** Reason freely about the problem (no format constraints)
2. **Constrain stage:** Extract structured fields from the draft

**Why:** Constrained decoding during generation causes "projection tax" — 39% confidence loss, up to 24pp accuracy degradation. Separating reasoning from formatting preserves quality.

---

## Budget-Aware Scaling

**Source:** BATS (arXiv:2511.17006)

**When:** At each build step, check `budgetRemaining` from context.

| Budget Remaining | Strategy |
|-----------------|----------|
| >60% of perTask | **Explore** — consider multiple approaches, read extra files |
| 30-60% | **Prioritize** — commit to highest-confidence approach |
| <30% | **Exploit** — minimize file reads, no new exploration |

---

## Uncertainty Gating

**Source:** AUQ (arXiv:2601.15703, arXiv:2602.05073)

**When:** Before any state-changing action (file edit, commit).

| Action Type | Risk Level | Protocol |
|------------|-----------|----------|
| Information-gathering (read, grep) | Low | Proceed without check |
| State-changing (edit, write) | High | Assess uncertainty first |
| Reflective (self-verify) | Calibration | Use for confidence adjustment |

**Rule:** If uncertainty > threshold before a state-changing action → trigger explicit reflection (deeper analysis) before proceeding.

---

## Secure Code Patterns

**Source:** arXiv:2602.05868

**When:** Task touches auth, input handling, API endpoints, or sensitive data.

| Security Detection Layer | Technique | Detection Rate |
|--------------------------|-----------|---------------|
| SecLayer-L1 | `grep` for known-bad patterns | ~40% |
| SecLayer-L2 | Static analyzer (semgrep, bandit) | ~65% |
| SecLayer-L3 | Behavioral test (run code, check behavior) | ~80% |
| SecLayer-L4 | Instinct-based review (prior vulnerability patterns) | ~90% |

**Rule:** Security-sensitive tasks require SecLayer-L3+ eval graders, not just L1/L2. Note: Security Detection Layers (vulnerability detection rate) are distinct from Eval Rigor Levels (eval grader quality, Rigor-L0 through L3) — see [adversarial-eval-coevolution.md](adversarial-eval-coevolution.md).

---

## Configuration Variant Tracking

**Source:** ARTEMIS (arXiv:2512.09108)

**When:** After each build attempt.

Log `configVariant` in `experiments.jsonl`:
```json
{"configVariant": {"builderTier": "tier-2", "promptPattern": "CoT", "auditorStrictness": "normal"}}
```

Enables Operator to identify which configuration combinations produce highest success rates.
