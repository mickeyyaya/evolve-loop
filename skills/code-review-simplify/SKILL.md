---
name: code-review-simplify
description: Unified code review and simplification skill for the evolve-loop pipeline. Combines structured pattern checks with agentic reasoning in a single pass to review and simplify code changes.
---

> Unified code review + simplification. Single-pass, multi-dimensional scoring, adaptive depth. Integrates with evolve-loop auditor and builder phases.

## Contents
- [Architecture](#architecture) — hybrid pipeline+agentic model
- [Single-Pass Flow](#single-pass-flow) — read diff once, analyze both dimensions
- [Multi-Dimensional Scoring](#multi-dimensional-scoring) — 4 dimensions with numeric scores
- [Adaptive Depth Routing](#adaptive-depth-routing) — scale analysis with diff complexity
- [Integration Hooks](#integration-hooks) — evolve-loop auditor and builder wiring
- [Simplification Catalog](#simplification-catalog) — what to simplify and when
- [Output Schema](#output-schema) — structured review+simplify report

## Architecture

Hybrid pipeline+agentic model. Structured passes handle known patterns cheaply; agentic reasoning handles contextual issues that require understanding intent.

```
Input: git diff (changed files)
         │
         ▼
┌─────────────────────────┐
│  PIPELINE LAYER (fast)  │  Deterministic pattern checks
│  ─────────────────────  │
│  1. Complexity scan     │  Cognitive complexity, nesting depth
│  2. Smell detection     │  22-smell catalog from detect-code-smells
│  3. Security scan       │  OWASP patterns, secrets, injection
│  4. Style check         │  Naming, file size, function length
│  5. Duplication check   │  Near-duplicate code blocks
└─────────┬───────────────┘
          │ Structured findings
          ▼
┌─────────────────────────┐
│  AGENTIC LAYER (deep)   │  LLM-powered contextual analysis
│  ─────────────────────  │
│  6. Logic correctness   │  Edge cases, off-by-one, null handling
│  7. Intent alignment    │  Does change match acceptance criteria?
│  8. Cross-file impact   │  Dependency effects, API contract breaks
│  9. Simplification      │  Extract Method, reduce nesting, dedup
│  10. Compound risk      │  Future maintenance cost assessment
└─────────┬───────────────┘
          │ Contextual findings
          ▼
┌─────────────────────────┐
│  SCORING & REPORT       │  Aggregate into multi-dimensional scores
│  ─────────────────────  │
│  Composite score 0.0-1.0│
│  Per-dimension scores   │
│  Simplification actions │
│  Priority-ranked issues │
└─────────────────────────┘
```

**Why hybrid?** Pipeline catches 60-70% of issues at ~5% of the token cost. Agentic layer focuses expensive reasoning on the 30-40% that requires context. Research: Cursor BugBot's biggest quality leap was pipeline→agentic; Anthropic's review dispatches parallel specialist agents.

**Token budget:** Pipeline layer ~2-5K tokens. Agentic layer ~15-40K tokens (scales with diff). Total: ~20-45K per review pass.

## Single-Pass Flow

Read the diff once. Run both review and simplification analysis on the same context. This saves ~40-50% tokens vs. invoking separate review and simplify agents.

### Step 1: LOAD (once)

```bash
DIFF=$(git diff HEAD~1 --stat)
DIFF_LINES=$(git diff HEAD~1 --numstat | awk '{s+=$1+$2} END {print s}')
CHANGED_FILES=$(git diff HEAD~1 --name-only)
```

### Step 2: PIPELINE (structured checks)

Run deterministic checks on each changed file:

| Check | Tool | Threshold | Finding Type |
|-------|------|-----------|-------------|
| Cognitive complexity | `scripts/complexity-check.sh` | > 15 per function | `complexity` |
| Nesting depth | grep-based | > 4 levels | `complexity` |
| Function length | line count | > 50 lines | `maintainability` |
| File length | line count | > 800 lines | `maintainability` |
| Near-duplicates | content hash | > 6 similar lines | `maintainability` |
| Hardcoded secrets | pattern match | any match | `security` |
| Injection vectors | pattern match | any match | `security` |
| Naming conventions | project config | deviation | `style` |

### Step 3: AGENTIC (contextual analysis)

LLM analyzes the diff with pipeline findings as context:

| Analysis | What to Check | Score Impact |
|----------|---------------|-------------|
| Logic correctness | Edge cases, boundary conditions, null/undefined, off-by-one | correctness score |
| Intent alignment | Acceptance criteria match, no scope creep, no missing requirements | correctness score |
| Cross-file impact | Breaking API changes, dependency effects, import correctness | correctness + security |
| Simplification opportunities | Extract Method candidates, reducible nesting, inlineable abstractions | maintainability score |
| Performance concerns | N+1 queries, missing indexes, unnecessary allocations, blocking calls | performance score |
| Security review | Auth checks, input validation, error info leakage | security score |

### Step 4: SCORE & REPORT

Aggregate findings into the multi-dimensional scoring output (see below).

## Multi-Dimensional Scoring

Four dimensions, each scored 0.0 to 1.0. Replaces binary PASS/FAIL with actionable numeric scores.

| Dimension | Weight | What It Measures | Score Guide |
|-----------|--------|-----------------|-------------|
| **correctness** | 0.35 | Logic errors, edge cases, intent alignment, test coverage | 1.0 = no issues; 0.7 = minor edge case; 0.3 = logic bug; 0.0 = critical flaw |
| **security** | 0.25 | Injection, auth, secrets, input validation, error leakage | 1.0 = hardened; 0.7 = minor gap; 0.3 = exploitable; 0.0 = critical vuln |
| **performance** | 0.15 | Complexity, N+1, blocking, memory, unnecessary work | 1.0 = optimal; 0.7 = acceptable; 0.3 = slow path; 0.0 = denial-of-service risk |
| **maintainability** | 0.25 | Readability, complexity, duplication, naming, file size | 1.0 = clean; 0.7 = minor smell; 0.3 = high cognitive load; 0.0 = unmaintainable |

**Composite score:** `composite = 0.35*correctness + 0.25*security + 0.15*performance + 0.25*maintainability`

**Verdict mapping:**

| Composite | Verdict | Action |
|-----------|---------|--------|
| >= 0.8 | PASS | Ship immediately |
| 0.6 - 0.79 | WARN | Ship with noted issues; simplification recommended |
| < 0.6 | FAIL | Block shipping; fix required |

**Simplification trigger:** If `maintainability < 0.7`, auto-generate simplification suggestions (see Simplification Catalog).

**Confidence:** Each dimension includes a `confidence` (0.0-1.0). If any dimension's confidence < 0.7, escalate to WARN regardless of score.

## Adaptive Depth Routing

Scale analysis intensity with diff complexity. Small changes get lightweight review; large changes get full multi-agent analysis.

| Tier | Trigger | Pipeline | Agentic | Token Budget |
|------|---------|----------|---------|-------------|
| **Lightweight** | < 50 changed lines, 1-3 files | Full pipeline checks | Single-pass agentic (no specialists) | ~10-15K |
| **Standard** | 50-200 lines, 3-10 files | Full pipeline checks | Full agentic analysis (all 5 checks) | ~20-35K |
| **Full Review** | > 200 lines, 10+ files, or security-sensitive | Full pipeline checks | Multi-agent specialist panel: correctness + security + performance agents | ~40-80K |

**Security-sensitive detection:** Files matching these patterns auto-escalate to full review:
- `auth*`, `*login*`, `*password*`, `*token*`, `*secret*`
- `*payment*`, `*billing*`, `*checkout*`
- `*eval*`, `*grader*`, `*.evolve/evals/*`
- Any file in `agents/`, `skills/*/SKILL.md`

**Risk-based routing:** Files with high churn (> 5 commits in last 10 cycles) or high fan-in (imported by > 5 other files) get escalated one tier.

## Integration Hooks

### Evolve-Loop Auditor Integration

The auditor invokes this skill as an optional enhancement to its review pass:

```
Auditor Standard Flow:
  1. Read build-report.md
  2. Run code quality checks          ← ENHANCED by pipeline layer
  3. Run security checks               ← ENHANCED by security scan
  4. Run hallucination detection        (unchanged)
  5. Run pipeline integrity checks      (unchanged)
  6. Run eval verification              (unchanged)
  7. Generate verdict                   ← ENHANCED by multi-dimensional scoring
```

**Auditor invocation:** When the auditor encounters code changes (not doc-only or config-only), it can invoke this skill's structured checks to supplement its review. The skill's composite score feeds into the auditor's verdict logic.

**Configuration in evolve-auditor.md:**
```markdown
### Optional Skill Consultation
- **code-review-simplify**: For code changes, invoke `skills/code-review-simplify/SKILL.md` 
  pipeline layer. Use composite score to supplement verdict. If maintainability < 0.7, 
  append simplification suggestions to audit-report.md.
```

### Evolve-Loop Builder Integration

The builder can invoke this skill post-implementation for self-review:

```
Builder Self-Review (after implementation, before reporting):
  1. Run eval graders (existing)
  2. Run code-review-simplify lightweight tier
  3. If maintainability < 0.7: apply simplification suggestions before reporting
  4. Include self-review score in build-report.md
```

**Builder invocation:** After implementing a task and before writing `build-report.md`, the builder runs this skill's lightweight tier on its own changes. Simplification suggestions with `maintainability < 0.7` are applied inline. This catches issues before the auditor sees them, reducing audit-fix cycles.

### Standalone Invocation

The skill can be invoked directly outside the evolve-loop:

```bash
# Review + simplify changed files
/code-review-simplify [--tier lightweight|standard|full] [--files <paths>]
```

## Simplification Catalog

When `maintainability < 0.7`, generate simplification suggestions from this catalog. Prioritize by impact and confidence.

| Category | Technique | When to Apply | Confidence |
|----------|----------|---------------|-----------|
| **Complexity** | Extract Method | Function > 30 lines or complexity > 10 | High (0.9) |
| **Complexity** | Flatten nesting | Nesting > 3 levels, guard clauses applicable | High (0.9) |
| **Complexity** | Decompose conditional | Complex boolean expressions (> 3 operators) | High (0.85) |
| **Duplication** | Extract shared utility | Near-duplicate blocks (> 6 lines, > 80% similar) | Medium (0.75) |
| **Readability** | Rename for clarity | Ambiguous names (< 3 chars, generic like `data`/`temp`) | Medium (0.7) |
| **Readability** | Replace magic numbers | Hardcoded literals in logic branches | High (0.85) |
| **Abstraction** | Inline over-abstraction | Single-use wrapper with no added value | Medium (0.7) |
| **Abstraction** | Remove dead code | Unreachable branches, unused imports/vars | High (0.9) |

**Constraints:**
- Only suggest localized refactorings (same file or module). LLMs are weak at cross-module architectural refactoring.
- Max 5 simplification suggestions per review. Focus on highest-impact.
- Each suggestion must include before/after code snippets and estimated complexity reduction.
- Never suggest simplification that changes external behavior (pure refactoring only).

## Output Schema

The skill produces a structured report:

```markdown
# Code Review + Simplify Report

## Summary
- **Tier:** lightweight | standard | full review
- **Changed:** X files, Y lines
- **Composite Score:** 0.XX
- **Verdict:** PASS | WARN | FAIL

## Dimension Scores
| Dimension | Score | Confidence | Key Finding |
|-----------|-------|------------|-------------|
| correctness | 0.X | 0.X | <summary> |
| security | 0.X | 0.X | <summary> |
| performance | 0.X | 0.X | <summary> |
| maintainability | 0.X | 0.X | <summary> |

## Issues (priority-ranked)
| # | Severity | Dimension | File:Line | Description | Suggestion |
|---|----------|-----------|-----------|-------------|-----------|

## Simplification Suggestions
| # | Technique | File:Line | Before (snippet) | After (snippet) | Impact |
|---|----------|-----------|-------------------|-----------------|--------|

## Pipeline Findings
<structured check results>

## Agentic Findings
<contextual analysis results>
```

**JSON variant** (for programmatic consumption by auditor/builder):

```json
{
  "tier": "lightweight|standard|full",
  "composite": 0.0,
  "verdict": "PASS|WARN|FAIL",
  "dimensions": {
    "correctness": {"score": 0.0, "confidence": 0.0, "findings": []},
    "security": {"score": 0.0, "confidence": 0.0, "findings": []},
    "performance": {"score": 0.0, "confidence": 0.0, "findings": []},
    "maintainability": {"score": 0.0, "confidence": 0.0, "findings": []}
  },
  "simplifications": [],
  "issueCount": {"critical": 0, "high": 0, "medium": 0, "low": 0}
}
```
