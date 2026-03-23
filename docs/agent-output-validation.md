# Agent Output Validation

> Reference doc for verifying agent outputs beyond basic eval graders. Covers deterministic, LLM-as-Judge, and Agent-as-Judge validation layers with mappings to evolve-loop phases and implementation patterns.

---

## Table of Contents

- [Three Validation Layers](#three-validation-layers)
- [Validation Techniques](#validation-techniques)
- [Tool Correctness](#tool-correctness)
- [Mapping to Evolve-Loop](#mapping-to-evolve-loop)
- [Implementation Patterns](#implementation-patterns)
- [Prior Art](#prior-art)
- [Anti-Patterns](#anti-patterns)

---

## Three Validation Layers

| Layer | Name | Method | Speed | Cost | When to Use |
|-------|------|--------|-------|------|-------------|
| 1 | Deterministic | Schema checks, type validation, format assertions, regex matching | < 1s | Zero | Every output, every time |
| 2 | LLM-as-Judge | Single LLM scores output for quality, correctness, relevance | 2-10s | Low-Medium | After Layer 1 passes; for semantic quality |
| 3 | Agent-as-Judge | Multiple agents independently verify, then reconcile verdicts | 10-60s | Medium-High | High-stakes outputs; conflicting Layer 2 signals |

### Layer Escalation Rules

| Condition | Action |
|-----------|--------|
| Layer 1 fails | Reject immediately; do not escalate |
| Layer 1 passes, output is low-risk | Accept without further validation |
| Layer 1 passes, output is high-risk | Escalate to Layer 2 |
| Layer 2 confidence < 0.7 | Escalate to Layer 3 |
| Layer 2 confidence >= 0.7 | Accept or reject based on score |
| Layer 3 agents disagree | Flag for human review |

---

## Validation Techniques

| Technique | Layer | Description | Example | Failure Mode |
|-----------|-------|-------------|---------|--------------|
| Schema validation | 1 | Validate output structure against JSON Schema or equivalent | `ajv.validate(schema, output)` | Missing required fields, wrong types |
| Assertion checking | 1 | Assert specific values, ranges, or conditions in output | `assert output.score >= 0 && output.score <= 1` | Out-of-range values, violated invariants |
| Property verification | 1 | Verify structural properties hold across output | Check all array items satisfy a predicate | Partial corruption, inconsistent sub-structures |
| Format validation | 1 | Validate output matches expected format (Markdown, JSON, YAML) | Parse output as JSON; check exit code | Malformed output, truncated responses |
| Cross-reference validation | 1-2 | Compare output against known ground truth or prior outputs | Diff current report against baseline | Regression, drift from expected behavior |
| Semantic consistency | 2 | Use LLM to check output is internally consistent and coherent | "Does this summary accurately reflect the source?" | Hallucination, contradiction, irrelevance |
| Output diffing | 1-2 | Compare structural diff between expected and actual output | `diff <(normalize expected) <(normalize actual)` | Unexpected additions, missing sections |
| Factual grounding | 2-3 | Verify claims against source material or tool outputs | Cross-check cited files against actual file contents | Fabricated references, wrong line numbers |
| Multi-perspective review | 3 | Multiple agents review from different angles (security, correctness, style) | Scout checks feasibility; Auditor checks correctness | Single-perspective blind spots |
| Adversarial probing | 3 | Agent attempts to find failure cases in the output | "Find an input that would break this function" | Untested edge cases, hidden assumptions |

---

## Tool Correctness

Validate tool call outputs with these checks.

| Check Type | Description | Implementation |
|------------|-------------|----------------|
| Exact matching | Tool output matches expected value exactly | `assert tool_output === expected` |
| Type checking | Tool output has correct type and shape | Validate against TypeScript interface or JSON Schema |
| Constraint satisfaction | Tool output satisfies domain constraints | Check numeric ranges, string lengths, enum membership |
| Idempotency | Repeated tool calls produce same result | Run twice, compare outputs |
| Side-effect verification | Tool produced expected side effects (files created, state changed) | Check filesystem, database, or API state after call |
| Error handling | Tool returns structured errors for invalid inputs | Pass invalid input, verify error format and message |
| Timeout enforcement | Tool completes within expected time bounds | Wrap call with timeout; fail if exceeded |

### Tool Output Validation Pipeline

| Step | Action | On Failure |
|------|--------|------------|
| 1 | Check tool returned non-error status | Log error, retry once, then escalate |
| 2 | Validate output schema | Reject output, report schema violation |
| 3 | Check domain constraints | Reject output, report constraint violation |
| 4 | Cross-reference with input context | Flag inconsistency for Layer 2 review |
| 5 | Cache validated output for future diffing | N/A |

---

## Mapping to Evolve-Loop

| Evolve-Loop Component | Validation Layer | Validation Role |
|----------------------|------------------|-----------------|
| Eval graders (bash assertions) | Layer 1 | Deterministic pass/fail on Scout, Builder, Auditor outputs |
| `phase-gate.sh` | Layer 1 | Deterministic validation at phase boundaries; block promotion on failure |
| `cycle-health-check.sh` | Layer 1 + Cross-reference | Verify cycle artifacts exist, are non-empty, and reference correct cycle number |
| Auditor agent | Layer 2 + Layer 3 | LLM-as-Judge scoring of Builder output; multi-perspective review |
| Scout agent | Layer 2 | Validate task feasibility and scope before Builder executes |
| Builder agent | Layer 1 (self-check) | Run tests and linters before reporting completion |
| Gene fitness scoring | Layer 2 | LLM evaluates gene quality against fitness criteria |
| Incident detection | Layer 1 + Layer 3 | Deterministic anomaly detection plus agent-based root cause analysis |

### Evolve-Loop Validation Flow

| Phase | Validation Check | Layer |
|-------|-----------------|-------|
| Scout | Verify task output matches expected schema | 1 |
| Scout | Check task is actionable and scoped | 2 |
| Scout -> Builder gate | `phase-gate.sh` validates Scout report exists and passes eval | 1 |
| Builder | Run eval graders on built artifacts | 1 |
| Builder | Verify code compiles, tests pass, lint clean | 1 |
| Builder -> Auditor gate | `phase-gate.sh` validates Builder report and artifacts | 1 |
| Auditor | Score Builder output for correctness and quality | 2 |
| Auditor | Cross-reference Builder claims against actual file state | 2-3 |
| Auditor -> Ship gate | `phase-gate.sh` validates Auditor approval | 1 |
| Ship | `cycle-health-check.sh` validates full cycle integrity | 1 |

---

## Implementation Patterns

### Layered Validation Pipeline

| Step | Description | Implementation |
|------|-------------|----------------|
| 1 | Run all Layer 1 checks in parallel | Schema, type, format, assertion checks; fail fast on any error |
| 2 | Aggregate Layer 1 results | All must pass to proceed; collect error details for failures |
| 3 | Run Layer 2 if required by risk level | LLM scores output on 0-1 scale for each quality dimension |
| 4 | Evaluate Layer 2 confidence | If confidence >= threshold, accept/reject; otherwise escalate |
| 5 | Run Layer 3 if escalated | Launch 2-3 independent judge agents; reconcile verdicts |
| 6 | Record validation results | Cache results for regression detection and validation diffing |

### Confidence-Based Escalation

| Confidence Range | Action | Rationale |
|-----------------|--------|-----------|
| 0.9 - 1.0 | Accept without escalation | High confidence; further validation adds cost without value |
| 0.7 - 0.9 | Accept with warning logged | Acceptable confidence; log for trend monitoring |
| 0.5 - 0.7 | Escalate to next layer | Ambiguous; need stronger signal |
| 0.0 - 0.5 | Reject without escalation | Low confidence; rejection is cheaper than further validation |

### Validation Caching

| Cache Strategy | Description | Invalidation Trigger |
|----------------|-------------|---------------------|
| Schema cache | Cache compiled JSON schemas for reuse | Schema definition changes |
| Baseline cache | Cache known-good outputs for diffing | Intentional output format change |
| Judge prompt cache | Cache LLM-as-Judge system prompts | Prompt revision |
| Verdict cache | Cache Layer 2/3 verdicts for identical inputs | Input hash mismatch |

---

## Prior Art

| Source | Contribution | Relevance |
|--------|-------------|-----------|
| Dual-suite testing (unit + eval) | Separate fast deterministic tests from slow LLM-graded tests | Maps directly to Layer 1 vs Layer 2 distinction |
| LLM-as-Judge (Zheng et al., 2023) | Use strong LLMs to evaluate weaker LLM outputs on quality scales | Foundation for Layer 2; establishes scoring methodology |
| Agent-as-Judge (MultiAgent) | Multiple agents evaluate from different perspectives, then vote | Foundation for Layer 3; provides reconciliation patterns |
| Guardrails AI | Schema-based validation with LLM output correction | Layer 1 implementation with automatic retry on schema failure |
| LMQL / Outlines | Constrained decoding to ensure valid output format | Prevents Layer 1 failures at generation time |
| DeepEval | Evaluation framework with multiple metric types | Combines deterministic and LLM-based metrics in one pipeline |
| Anthropic model-graded evals | Use Claude to grade outputs against rubrics | Production-tested Layer 2 approach |
| Constitutional AI | Self-critique and revision loops | Agent-as-Judge pattern where the agent judges its own output |

---

## Anti-Patterns

| Anti-Pattern | Problem | Fix |
|-------------|---------|-----|
| Format-only validation | Checking JSON is valid but not checking content correctness | Add semantic validation (Layer 2) after format checks (Layer 1) |
| LLM self-validation | Asking the same LLM that produced output to judge it | Use a different model or agent for validation; avoid self-grading |
| Missing regression checks | No comparison against prior known-good outputs | Cache baselines; diff new outputs against them |
| Single-layer reliance | Using only deterministic checks or only LLM checks | Implement all three layers; escalate based on risk and confidence |
| Validation without action | Detecting problems but not blocking promotion | Wire validation failures into phase gates; fail the pipeline |
| Over-validation | Running Layer 3 on every output regardless of risk | Use confidence-based escalation; reserve Layer 3 for ambiguous cases |
| Stale baselines | Comparing against outdated expected outputs | Invalidate baselines when output format or requirements change |
| Prompt injection in judge | Adversarial output manipulates LLM-as-Judge scoring | Sanitize output before passing to judge; use structured scoring rubrics |
