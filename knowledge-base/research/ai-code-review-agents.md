# AI Code Review Agents

> Reference document for AI-powered code review patterns, architectures, and calibration techniques.
> Apply these patterns to build review agents that catch real defects without drowning developers in noise.

## Table of Contents

1. [Review Agent Architectures](#review-agent-architectures)
2. [Risk-Based Depth Routing](#risk-based-depth-routing)
3. [Attribution-Based Feedback](#attribution-based-feedback)
4. [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
5. [Prior Art](#prior-art)
6. [Anti-Patterns](#anti-patterns)

---

## Review Agent Architectures

Choose an architecture based on codebase size, defect diversity, and latency budget.

### Single Reviewer

| Aspect | Detail |
|---|---|
| **Structure** | One agent reviews all changes in a single pass |
| **Strengths** | Low latency, simple orchestration, consistent voice |
| **Weaknesses** | Limited expertise depth, blind spots in specialized domains |
| **Best For** | Small codebases, rapid iteration, low-risk changes |
| **Prompt Design** | Provide a rubric covering security, correctness, style, and performance in one system prompt |

### Specialist Panel

Run multiple focused reviewers in parallel, then merge findings.

| Specialist | Focus Area | Rubric Scope |
|---|---|---|
| **Security Reviewer** | Injection, auth, secret leakage, input validation | OWASP Top 10, CWE categories |
| **Performance Reviewer** | Algorithmic complexity, memory allocation, I/O patterns | Big-O analysis, profiling heuristics |
| **Style Reviewer** | Naming, formatting, idiomatic patterns, documentation | Project style guide, language conventions |
| **Correctness Reviewer** | Logic errors, edge cases, type safety, invariant violations | Test coverage gaps, assertion density |

| Aspect | Detail |
|---|---|
| **Strengths** | Deep domain expertise per specialist, parallel execution |
| **Weaknesses** | Higher token cost, potential duplicate findings, merge complexity |
| **Dedup Strategy** | Hash findings by file+line+category; keep highest-severity duplicate |
| **Best For** | Large codebases, high-risk changes, regulated environments |

### Hierarchical (Scan-Triage-Deep Review)

Three-stage pipeline that balances cost and thoroughness.

| Stage | Agent | Action | Token Budget |
|---|---|---|---|
| **1. Scan** | Lightweight model (Haiku) | Flag suspect files and hunks | Low |
| **2. Triage** | Medium model (Sonnet) | Classify flagged items by severity and confidence | Medium |
| **3. Deep Review** | Heavy model (Opus) | Analyze only high-severity, low-confidence items | High |

| Aspect | Detail |
|---|---|
| **Strengths** | Cost-efficient — most code never reaches the expensive stage |
| **Weaknesses** | Scan stage false negatives propagate downstream |
| **Mitigation** | Randomly sample 5-10% of scan-passed files for deep review to detect blind spots |

---

## Risk-Based Depth Routing

Route review depth by file risk score to allocate review budget where it matters.

### Risk Signal Sources

| Signal | Description | Weight |
|---|---|---|
| **Churn Rate** | Number of commits touching the file in the last 90 days | High |
| **Bug History** | Count of bug-fix commits associated with the file | High |
| **Complexity** | Cyclomatic complexity or line count | Medium |
| **Author Tenure** | Time since author's first commit to this file | Medium |
| **Test Coverage** | Percentage of lines covered by tests | Medium |
| **Recency of Last Bug** | Days since last bug-fix commit | Low |

### Depth Tiers

| Tier | Risk Score | Review Action | Example |
|---|---|---|---|
| **Deep** | High (top 10%) | Full specialist panel review, line-by-line analysis | Auth module, payment processing |
| **Standard** | Medium (10-50%) | Single reviewer with full rubric | Business logic, API endpoints |
| **Scan** | Low (bottom 50%) | Automated linting + spot-check summary | Config files, generated code, static assets |

### Routing Algorithm

1. Compute risk score for each changed file using weighted signal sum.
2. Normalize scores to percentiles across the codebase.
3. Assign depth tier based on percentile thresholds.
4. Override tier upward if the diff contains security-sensitive patterns (auth, crypto, SQL).
5. Log routing decisions for later calibration.

---

## Attribution-Based Feedback

Track which review findings developers accept or reject to calibrate agent accuracy over time.

### Feedback Loop

| Step | Action | Data Captured |
|---|---|---|
| **1. Emit** | Agent produces a finding with category, severity, confidence, file, line | Finding record |
| **2. Present** | Finding shown to developer as inline comment or summary item | Presentation timestamp |
| **3. Resolve** | Developer accepts (fixes), dismisses (won't fix), or disputes (false positive) | Resolution label |
| **4. Record** | Store (finding, resolution) pair in feedback log | Attribution record |
| **5. Aggregate** | Compute per-category precision and recall over rolling window | Calibration metrics |

### Calibration Metrics

| Metric | Formula | Target |
|---|---|---|
| **Precision** | Accepted findings / Total findings | > 70% |
| **False Positive Rate** | Dismissed findings / Total findings | < 15% |
| **Category Precision** | Accepted in category / Total in category | Varies by category |
| **Confidence Calibration** | Correlation between stated confidence and actual acceptance rate | > 0.7 |
| **Drift Detection** | Week-over-week precision delta | < 5% absolute change |

### Calibration Actions

| Condition | Action |
|---|---|
| Precision drops below 60% | Raise confidence threshold for emitting findings |
| Single category FP rate exceeds 25% | Retune or disable that category's rubric |
| Confidence uncalibrated (low correlation) | Add calibration examples to few-shot prompt |
| Drift exceeds 5% for 2+ weeks | Trigger full rubric review and prompt update |

---

## Mapping to Evolve-Loop

Apply these review patterns to the evolve-loop's existing agent pipeline.

### Agent Role Mapping

| Review Concept | Evolve-Loop Component | Implementation |
|---|---|---|
| Code reviewer | **Auditor** agent | Auditor already reviews Builder output against rubric criteria |
| Specialist panel | Auditor rubric sections | Split Auditor rubric into security, correctness, and style sub-scores |
| Risk-based routing | Scout task selection | Scout prioritizes high-churn, high-bug-history files for review |
| Attribution feedback | `auditorProfile` tracking | Track Auditor accuracy per rubric criterion over cycles |
| Automated checks | Eval graders | Bash graders provide deterministic pass/fail on structural requirements |
| Confidence calibration | Calibration mismatch detection | Compare Auditor confidence scores against eval grader outcomes |

### Calibration Mismatch Detection

Detect when the Auditor's judgment diverges from ground truth.

| Mismatch Type | Signal | Response |
|---|---|---|
| **Auditor approves, grader fails** | Auditor overestimates quality | Lower Auditor pass threshold, add failing pattern to rubric |
| **Auditor rejects, grader passes** | Auditor is too strict or hallucinating defects | Review rejected findings for false positives, relax rubric |
| **Auditor high-confidence wrong** | Confidence is miscalibrated | Add contrastive examples to Auditor prompt |
| **Consistent category misses** | Blind spot in rubric | Add new rubric criterion or specialist sub-agent |

### Integration Points

| Point | Detail |
|---|---|
| **Scout report** | Include file risk scores so Builder and Auditor can adjust effort |
| **Builder output** | Tag generated vs. hand-modified code so Auditor applies appropriate depth |
| **Audit report** | Record per-finding confidence and category for attribution tracking |
| **Learning phase** | Aggregate attribution data across cycles to update `auditorProfile` |

---

## Prior Art

| System | Org | Approach | Key Metric | Notes |
|---|---|---|---|---|
| **Graphite Agent** | Graphite | LLM reviewer integrated into PR workflow | 5-8% false positive rate | Aggressive noise filtering, focuses on correctness over style |
| **CodeRabbit** | CodeRabbit | Multi-pass LLM review with incremental re-review | Configurable severity thresholds | Supports custom review profiles per repo |
| **Amazon CodeGuru** | AWS | ML-trained on internal Amazon code reviews | Reduced reviewer load by ~25% | Uses historical bug data for risk-based prioritization |
| **Meta SapFix** | Meta | Automated patch generation + review for static analysis findings | Fixes ~50% of Infer warnings automatically | Combines review with repair — reviews its own patches |
| **Google AI Review** | Google | LLM-assisted code review studied at scale | Developers found AI comments useful ~40% of the time | 2024 study; highlights the challenge of precision at scale |
| **GitHub Copilot Review** | GitHub | LLM reviewer triggered on PR creation | Early access, metrics not yet published | Integrated into existing PR review UX |

### Key Takeaways from Prior Art

| Insight | Source | Implication |
|---|---|---|
| False positive rate is the critical metric | Graphite, Google | Optimize for precision over recall — missed findings are cheaper than noise |
| Developers ignore agents with FP > 15% | Google study | Set hard FP ceiling and auto-disable categories that exceed it |
| Risk-based routing reduces cost 3-5x | CodeGuru | Apply depth routing before invoking expensive models |
| Self-repair amplifies review value | SapFix | Combine review findings with automated fix suggestions |

---

## Anti-Patterns

Avoid these failure modes when building AI code review agents.

| Anti-Pattern | Description | Consequence | Mitigation |
|---|---|---|---|
| **Alert Fatigue** | Emitting too many low-severity findings per review | Developers stop reading review comments entirely | Cap findings per review (e.g., max 10), prioritize by severity |
| **Style Nit Overload** | Flooding reviews with formatting and naming suggestions | Real defects buried under trivial feedback | Separate style checks into a linter; exclude from AI review |
| **False Positive Flooding** | Low-confidence findings emitted without filtering | Trust erodes, developers dismiss all agent findings | Enforce minimum confidence threshold (e.g., 0.7) before emitting |
| **Context-Free Review** | Reviewing generated or scaffolded code as if hand-written | Irrelevant findings on boilerplate code | Tag generated code; apply scan-only depth to tagged files |
| **Stale Rubric** | Review rubric does not evolve with codebase patterns | Agent flags patterns the team has already adopted as standard | Update rubric monthly using attribution feedback data |
| **Ignoring Calibration** | No feedback loop tracking acceptance/rejection of findings | No mechanism to detect drift or improve over time | Implement attribution-based feedback from day one |
| **Monolithic Prompt** | Cramming all review concerns into a single massive prompt | Degraded performance on all categories | Split into specialist prompts or hierarchical stages |
| **Reviewing Without Tests** | Agent reviews code correctness without access to test results | Findings lack grounding, miss tested invariants | Provide test output and coverage data as review context |
