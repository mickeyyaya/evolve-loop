# Agent Interpretability

> Reference doc for making agent decisions explainable. Covers explanation
> dimensions, structured decision traces, and implementation patterns for
> the evolve-loop pipeline.

## Table of Contents

- [Three Dimensions of Explainability](#three-dimensions-of-explainability)
- [Explanation Techniques](#explanation-techniques)
- [Structured Decision Traces](#structured-decision-traces)
- [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
- [Implementation Patterns](#implementation-patterns)
- [Prior Art](#prior-art)
- [Anti-Patterns](#anti-patterns)

---

## Three Dimensions of Explainability

Every agent decision has three orthogonal dimensions. Capture all three to
produce a complete explanation.

| Dimension   | Question Answered       | What to Record                                | Example Output                                              |
|-------------|-------------------------|-----------------------------------------------|-------------------------------------------------------------|
| **Process** | How did the agent decide? | Steps executed, tools invoked, data consumed | "Ran 4 candidate searches, filtered by relevance > 0.8"     |
| **Content** | What was decided?        | Final action, parameters, artifacts produced  | "Selected refactor-cache task, confidence 0.91"              |
| **Rationale** | Why was this chosen?  | Criteria weights, trade-offs, rejected alternatives | "Chose cache refactor over logging fix: higher impact score" |

### Dimension Coverage Checklist

| Artifact             | Process | Content | Rationale | Gap to Close                        |
|----------------------|---------|---------|-----------|-------------------------------------|
| Scout report         | Partial | Yes     | Partial   | Add explicit rejection reasons      |
| Build report         | Yes     | Yes     | No        | Add rationale per implementation step |
| Audit report         | No      | Yes     | Yes       | Add audit methodology trace         |
| experiments.jsonl    | No      | Yes     | No        | Add decision context field          |

---

## Explanation Techniques

| Technique                    | Category    | Description                                                        | Complexity | Faithfulness |
|------------------------------|-------------|--------------------------------------------------------------------|------------|--------------|
| Chain-of-thought traces      | Process     | Record the agent's reasoning steps as a sequential log             | Low        | High         |
| Decision logs                | Content     | Persist each decision point with inputs, outputs, and timestamps   | Low        | High         |
| Attention visualization      | Process     | Surface which context tokens or files the agent weighted most      | High       | Medium       |
| Counterfactual explanations  | Rationale   | Show what would have changed if a different input or choice applied | Medium     | High         |
| Causal traces                | Rationale   | Link each decision to the upstream cause that triggered it         | Medium     | High         |

### Technique Selection Guide

| Goal                                  | Recommended Technique          |
|---------------------------------------|--------------------------------|
| Debug why an agent picked a wrong task | Counterfactual explanations   |
| Verify the agent followed the pipeline | Chain-of-thought traces       |
| Audit decision integrity post-hoc      | Decision logs + causal traces |
| Communicate decisions to humans        | Decision logs + counterfactuals |
| Detect reward hacking                  | Causal traces                 |

---

## Structured Decision Traces

Use the following format for every non-trivial agent decision.

### Decision Trace Schema

| Field              | Type       | Required | Description                                      |
|--------------------|------------|----------|--------------------------------------------------|
| `traceId`          | string     | Yes      | Unique identifier (e.g., `cycle-142-scout-01`)   |
| `timestamp`        | ISO 8601   | Yes      | When the decision was made                        |
| `agent`            | string     | Yes      | Agent role: Scout, Builder, or Auditor            |
| `phase`            | string     | Yes      | Pipeline phase: scout, build, audit, ship, learn  |
| `context`          | object     | Yes      | Inputs available at decision time                 |
| `alternatives`     | array      | Yes      | Options considered with scores                    |
| `selected`         | string     | Yes      | Chosen alternative ID                             |
| `rationale`        | string     | Yes      | Human-readable explanation of why                 |
| `confidence`       | float      | Yes      | 0.0-1.0 confidence in the decision                |
| `counterfactual`   | string     | No       | What would change under a different choice         |

### Example Trace

```json
{
  "traceId": "cycle-142-scout-01",
  "timestamp": "2026-03-24T10:15:00Z",
  "agent": "Scout",
  "phase": "scout",
  "context": {
    "openTasks": 12,
    "recentFailures": ["cache-ttl-bug"],
    "priorityWeights": { "feature": 1.0, "bugfix": 0.8, "security": 0.6 }
  },
  "alternatives": [
    { "id": "add-streaming-api", "score": 0.91, "type": "feature" },
    { "id": "fix-cache-ttl", "score": 0.85, "type": "bugfix" },
    { "id": "patch-xss-filter", "score": 0.72, "type": "security" }
  ],
  "selected": "add-streaming-api",
  "rationale": "Highest weighted score; no blocking bugs in critical path",
  "confidence": 0.91,
  "counterfactual": "If cache-ttl-bug were in critical path, fix-cache-ttl would score 0.95"
}
```

---

## Mapping to Evolve-Loop

Map each evolve-loop artifact to an explainability role.

| Artifact              | Explainability Role        | Dimension Covered | Current State       | Improvement Action                          |
|-----------------------|----------------------------|--------------------|---------------------|---------------------------------------------|
| `scout-report.md`    | Decision trace             | Process + Content  | Lists selected task | Add `alternatives` and `rationale` fields   |
| `build-report.md`    | Process trace              | Process + Content  | Step-by-step log    | Add per-step rationale and confidence       |
| `audit-report.md`    | Rationale documentation    | Content + Rationale| Pass/fail verdicts  | Add methodology trace and evidence links    |
| `experiments.jsonl`   | Decision history           | Content            | Outcome records     | Add decision context and counterfactuals    |
| `phase-gate.sh`      | Process integrity check    | Process            | Deterministic gate  | Log gate results to decision trace          |

### Agent-Specific Explainability

| Agent       | Primary Explanation Need              | Trace Focus                                  |
|-------------|---------------------------------------|----------------------------------------------|
| **Scout**   | Why this task over others?            | Alternatives considered, scoring weights      |
| **Builder** | Why this implementation approach?     | Design choices, rejected patterns, trade-offs |
| **Auditor** | Why pass or fail?                     | Evidence gathered, criteria applied, thresholds |

---

## Implementation Patterns

### Pattern 1: Add Structured Traces to Build Reports

| Step | Action                                                              |
|------|---------------------------------------------------------------------|
| 1    | Define a `decisionTrace` section in the build-report template       |
| 2    | Record each implementation choice as a trace entry                  |
| 3    | Include alternatives considered and rejection reasons               |
| 4    | Attach confidence scores to non-trivial decisions                   |
| 5    | Validate trace completeness in the Auditor phase                    |

### Pattern 2: Generate Human-Readable Summaries

| Step | Action                                                              |
|------|---------------------------------------------------------------------|
| 1    | Extract decision traces from the cycle workspace                    |
| 2    | Group by agent role (Scout, Builder, Auditor)                       |
| 3    | Render each trace as: "Agent chose X over Y because Z"             |
| 4    | Include confidence level as a qualifier (high/medium/low)           |
| 5    | Append to the cycle's learn phase output                            |

### Pattern 3: Track Decision Confidence

| Confidence Range | Label   | Action Required                                |
|------------------|---------|------------------------------------------------|
| 0.9 - 1.0       | High    | Proceed; log trace for audit                   |
| 0.7 - 0.89      | Medium  | Proceed; flag for human review if time permits |
| 0.5 - 0.69      | Low     | Pause; request additional context or escalate  |
| Below 0.5       | Uncertain | Do not proceed; escalate to human operator   |

### Pattern 4: Decision Diff Between Cycles

| Step | Action                                                              |
|------|---------------------------------------------------------------------|
| 1    | Load decision traces from cycle N and cycle N-1                     |
| 2    | Compare selected tasks, confidence levels, and rationale            |
| 3    | Flag reversals (chose opposite of prior cycle) for review           |
| 4    | Track confidence trends over time to detect drift                   |

---

## Prior Art

| Source                              | Contribution                                              | Relevance to Agents                              |
|-------------------------------------|-----------------------------------------------------------|--------------------------------------------------|
| Mechanistic interpretability        | Map internal model representations to concepts            | Understand why an agent weights certain features  |
| LIME (Local Interpretable Model-agnostic Explanations) | Perturb inputs to explain individual predictions | Adapt for explaining single agent decisions       |
| SHAP (SHapley Additive exPlanations) | Attribute prediction to input features using game theory | Quantify which context fields drove a decision    |
| Anthropic interpretability research | Identify features in neural networks; scaling monosemanticity | Ground truth for what models "know" vs. "guess"  |
| Chain-of-thought prompting          | Elicit step-by-step reasoning from LLMs                   | Direct application as process traces              |
| Constitutional AI                   | Self-critique and revision loops                          | Parallel to Auditor rationale generation          |
| ReAct framework                     | Interleave reasoning and action traces                    | Template for structured decision logging          |

### Adapting LIME/SHAP for Agents

| Classic ML Step              | Agent Adaptation                                          |
|------------------------------|-----------------------------------------------------------|
| Perturb input features       | Vary context fields (e.g., remove a task from the queue)  |
| Measure prediction change    | Observe which task the Scout selects instead               |
| Attribute importance          | Rank context fields by decision impact                    |
| Visualize feature weights    | Render as a table in the scout-report                     |

---

## Anti-Patterns

| Anti-Pattern               | Description                                                     | Detection Signal                                    | Mitigation                                          |
|----------------------------|-----------------------------------------------------------------|-----------------------------------------------------|-----------------------------------------------------|
| Rationalization            | Post-hoc justification that does not reflect actual reasoning   | Rationale contradicts process trace                  | Cross-check rationale against logged steps           |
| Trace overload             | Recording every micro-decision, drowning signal in noise        | Trace files exceed 1000 lines per cycle              | Filter to non-trivial decisions (confidence < 0.95)  |
| Opaque decisions           | No trace recorded; decision appears from nowhere                | Missing `decisionTrace` section in reports           | Enforce trace presence in phase-gate validation      |
| Unfaithful explanations    | Explanation sounds plausible but does not match causal path      | Counterfactual test contradicts stated rationale     | Run counterfactual checks on high-impact decisions   |
| Explanation theater        | Verbose explanations that add no information                    | High word count, low information density             | Require structured format; reject free-form prose    |
| Confidence anchoring       | Always reporting high confidence regardless of actual certainty  | Confidence variance near zero across many decisions  | Calibrate against historical accuracy rates          |
| Selective transparency     | Explaining easy decisions well, hiding hard ones                | Trace completeness drops for low-confidence choices  | Mandate traces for all decisions below 0.9 confidence |
