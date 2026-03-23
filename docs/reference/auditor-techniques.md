> Read this file when running Phase 3 (AUDIT). Contains research-backed techniques for anti-conformity checking, eval handling, threat detection, actionable critique, and regression prevention.

## Contents
- [Anti-Conformity Check](#anti-conformity-check) — resist self-herding bias (Free-MAD)
- [Non-Deterministic Eval Handling](#non-deterministic-eval-handling) — three-valued verdicts (AgentAssay)
- [Threat Taxonomy Check](#threat-taxonomy-check) — 6-category adversarial screening (Red-Teaming)
- [Actionable Critique Format](#actionable-critique-format) — structured revision instructions (CGI)
- [Multi-Source Reflection on Retry](#multi-source-reflection-on-retry) — prevent degeneration (MAR)
- [Regression Eval Enforcement](#regression-eval-enforcement) — preserve prior behavior (SWE-CI)
- [Runtime Guardrail Checks](#runtime-guardrail-checks) — pre-execution enforcement (AgentSpec)
- [CLEAR Multi-Dimensional Scoring](#clear-multi-dimensional-scoring) — beyond accuracy (CLEAR)

---

## Anti-Conformity Check

**Source:** Free-MAD (arXiv:2509.11035)

**When:** Before finalizing any audit verdict.

**Protocol:**
1. Form preliminary verdict (PASS/WARN/FAIL)
2. Adopt the opposite stance — argue the other case with concrete evidence
3. Compare evidence strength for both positions
4. Finalize based on stronger evidence, not initial leaning

**Injection prompt:** "Before finalizing, state your initial leaning. Then argue the opposite with at least one concrete piece of evidence."

---

## Non-Deterministic Eval Handling

**Source:** AgentAssay (arXiv:2603.02601)

**When:** An eval grader produces inconsistent results across runs.

| Verdict | Condition | Action |
|---------|-----------|--------|
| PASS | Grader passes on all runs | Ship normally |
| FAIL | Grader fails consistently | Block and retry |
| INCONCLUSIVE | Mixed results across runs | Re-run up to 3x; 2/3 majority decides |

**Behavioral fingerprinting:** If same task type produces WARN with different reasoning paths across cycles → flag audit consistency drift.

---

## Threat Taxonomy Check

**Source:** Red-Teaming (arXiv:2512.20677)

**When:** Reviewing any build that modifies eval files, agent prompts, or state.json.

| Threat | Detection Signal |
|--------|-----------------|
| Reward hacking | Builder code passes greps but doesn't implement real behavior |
| Sandbagging | Minimal implementation below task spec |
| Data exfiltration | Internal paths or state in committed files |
| Inappropriate tool use | Builder modified eval files or agent prompts |
| Chain-of-thought manipulation | Auditor reasoning contradicts verdict |
| Specification gaming | Tautological eval graders (Level 0/1) |

---

## Actionable Critique Format

**Source:** CGI (arXiv:2503.16024)

**When:** Issuing WARN or FAIL verdict.

**Required format:**
```markdown
## Critique — <task-slug>
**What failed:** <specific assertion or eval grader>
**Why it failed:** <root cause — not just "incomplete">
**How to fix:** <concrete revision instruction>
**Confidence:** <0.0-1.0>
```

**Rule:** Never issue a bare "FAIL" without `How to fix`. Opaque verdicts cause blind retries.

---

## Multi-Source Reflection on Retry

**Source:** MAR (arXiv:2512.20845)

**When:** Builder has failed audit and is about to retry.

**Include in retry context:**
1. Auditor's verdict and critique (primary)
2. Eval grader output (which specific graders failed)
3. Builder's own risk assessment from build-report
4. Relevant instincts that apply to the failure type

**Why:** Single-source reflection degenerates — the same model repeats its error. Multi-source ensures the retry sees the failure from angles the Builder cannot access alone.

---

## Regression Eval Enforcement

**Source:** SWE-CI (March 2026)

**When:** Task modifies existing functionality (M+ complexity, or touches files modified in last 3 cycles).

**Mandatory check:** Verify `## Regression Evals` section exists in the eval definition. If missing and triggers apply → WARN.

**Triggers:**
- Task modifies 3+ existing files
- Task touches files modified in previous 3 cycles
- Task modifies code with `consecutiveClean >= 3` in auditorProfile

---

## Runtime Guardrail Checks

**Source:** AgentSpec (arXiv:2503.18666), AEGIS (arXiv:2603.12621)

**When:** At phase boundary, before Auditor starts.

| Check | Predicate | On Violation |
|-------|-----------|-------------|
| Eval integrity | Checksums match Phase 1 capture | HALT |
| Build scope | Changed files ⊆ `filesToModify` | WARN if out-of-scope |
| Worktree state | Worktree exists, build-report present | HALT if missing |
| Challenge token | Token in build-report matches cycle token | HALT if mismatch |

---

## CLEAR Multi-Dimensional Scoring

**Source:** CLEAR (arXiv:2511.14136)

**When:** Evaluating overall cycle quality (complements LLM-as-a-Judge).

| Dimension | What to Check | Source |
|-----------|--------------|--------|
| Cost | Token usage within budget? | Ledger entries |
| Latency | Cycle time reasonable? | Wall-clock tracking |
| Efficacy | Evals passed? Acceptance criteria met? | Eval grader results |
| Assurance | Safety constraints respected? | Health check, checksums |
| Reliability | Consistent across similar tasks? | taskArms variance |
