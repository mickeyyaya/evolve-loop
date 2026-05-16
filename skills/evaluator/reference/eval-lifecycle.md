# Evaluation Lifecycle — Self-Improving Evaluation

> Reference for the `/evaluator` skill's self-improvement mechanisms. Based on EDDOps (arXiv:2411.13768) and metacognitive learning research.

## Contents
- [4-Stage Lifecycle](#4-stage-lifecycle)
- [Drift Detection](#drift-detection)
- [Adaptive Difficulty](#adaptive-difficulty)
- [Evidence-Linked Changes](#evidence-linked-changes)
- [Meta-Evaluation (Layer 5)](#meta-evaluation-layer-5)

---

## 4-Stage Lifecycle

Evaluation criteria are not static — they evolve through stages as the project matures.

| Stage | When | Criteria Focus | Technique |
|-------|------|---------------|-----------|
| **1. Baseline** | New project / first evaluation | Basic quality gates | Pinned thresholds (file size, test existence, no secrets) |
| **2. Calibration** | After 3-5 evaluations | Tune scoring to project norms | Compare automated scores to human judgment; adjust weights |
| **3. Steady State** | Ongoing operation | Drift detection + regression alerts | Continuous monitoring; adaptive probes on anomaly signals |
| **4. Evolution** | When criteria saturate or requirements change | Expand and deepen criteria | Version eval plans; add new dimensions; retire obsolete checks |

### Stage Transitions

| From → To | Trigger | Action |
|-----------|---------|--------|
| Baseline → Calibration | 3 evaluations completed | Compare scores to developer perception; adjust if > 0.3 gap |
| Calibration → Steady State | Scores correlate with human judgment (r > 0.6) | Lock weights; enable regression alerts |
| Steady State → Evolution | Saturation detected OR requirements change | Generate harder criteria; expand dimension scope |
| Evolution → Steady State | New criteria validated (A/B test passes) | Lock new criteria; reset saturation counters |

---

## Drift Detection

Monitor for two types of drift:

### Score Drift (evaluator becoming stale)

| Signal | Meaning | Response |
|--------|---------|----------|
| Scores plateau (std < 0.05 over 5 evals) | Criteria no longer differentiate quality levels | Introduce new checks or harder thresholds |
| Scores always high (> 0.9 all dimensions) | Criteria too easy or project genuinely excellent | Run meta-evaluation to distinguish |
| Scores correlate poorly with outcomes | Evaluator measuring wrong things | Recalibrate against actual test results, bug rates |

### Requirement Drift (project evolving past evaluator)

| Signal | Meaning | Response |
|--------|---------|----------|
| New patterns not covered by any dimension | Feature evolution outpaced criteria | Add checks for new patterns to relevant dimension |
| Team workflow changed (e.g., added CI/CD) | Operational expectations evolved | Add operational completeness checks |
| Technology stack changed | Existing checks may be irrelevant | Regenerate automated checks for new stack |

---

## Adaptive Difficulty

When evaluation criteria saturate, automatically introduce harder variants.

### Difficulty Levels per Dimension

| Dimension | Level 1 (baseline) | Level 2 (intermediate) | Level 3 (advanced) |
|-----------|-------------------|----------------------|-------------------|
| correctness | Tests exist | Tests cover edge cases | Property-based tests + mutation testing |
| security | No hardcoded secrets | Input validated at boundaries | Defense in depth + rate limiting + audit logging |
| maintainability | Files < 800 lines | Functions < 50 lines, complexity < 15 | Single responsibility, complexity < 10, zero dead code |
| architecture | No circular deps | Clear module boundaries | Dependency injection, interface-based coupling |
| completeness | README exists | API docs + error handling | Runbooks + monitoring + health checks |
| evolution | Config file exists | Feature flags for variants | Plugin architecture + backward-compatible APIs |

### Graduation Protocol

1. Detect saturation (dimension at 1.0 for 3+ evals)
2. Advance that dimension to next difficulty level
3. Re-evaluate with harder criteria
4. If score differentiates quality → adopt new level
5. If score is still 1.0 → advance again (max Level 3)

---

## Evidence-Linked Changes

Every evaluation-driven change must trace back to specific findings.

### Evidence Chain

```
Evaluation finding → Recommendation → Implementation → Re-evaluation
       ↑                                                     │
       └─────────────── feedback loop ────────────────────────┘
```

### Evidence Record Format

| Field | Example |
|-------|---------|
| `finding_id` | `eval-175-sec-001` |
| `dimension` | `security` |
| `score_before` | 0.55 |
| `location` | `src/api/handler.ts:45` |
| `description` | "Unsanitized user input in SQL query" |
| `recommendation` | "Use parameterized queries" |
| `implemented_cycle` | 176 |
| `score_after` | 0.85 |
| `verdict` | IMPROVED (+0.30) |

This creates an audit trail connecting every quality improvement to its evaluation origin.

---

## Meta-Evaluation (Layer 5)

Periodic evaluation of the evaluator itself. Not every invocation — triggered by specific conditions.

### Triggers

| Condition | Meta-Eval Action |
|-----------|-----------------|
| Gaming detected (G > 0.3) in 3+ consecutive evals | Red-team the evaluator — attempt to fool it with crafted inputs |
| All dimensions saturated (> 0.9) for 5+ evals | Review if criteria are too easy or project genuinely excellent |
| Proxy-true correlation drops below 0.5 | Evaluator no longer measuring what matters — recalibrate |
| Major project change (new language, framework, architecture) | Regenerate automated checks for new context |

### Red-Team Protocol

1. **Generate adversarial inputs:** Code that looks good but has subtle bugs (off-by-one, race conditions, missing auth)
2. **Run evaluator on adversarial inputs:** Does it catch the issues?
3. **Score detection rate:** If < 60%, evaluator needs strengthening
4. **Strengthen weak spots:** Add checks specifically for missed issue types
5. **Re-test:** Verify improved detection without increasing false positives

### Human Calibration Checkpoints

Even with automated meta-evaluation, periodic human review is essential:

| Frequency | What to Review |
|-----------|---------------|
| Monthly | Sample 5 evaluation reports — do scores match your judgment? |
| After major changes | Full re-evaluation — are criteria still relevant? |
| After gaming detection | Specific findings — was the gaming call correct? |

**Research basis:** Anthropic's principle: "You won't know if your graders are working well unless you read the transcripts."
