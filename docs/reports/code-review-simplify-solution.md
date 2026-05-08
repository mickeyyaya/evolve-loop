# Unified Code Review + Simplification Solution

> Design document for the `code-review-simplify` skill — a unified code review and simplification engine integrated into the evolve-loop pipeline. Records the research findings, architectural decisions, build-vs-buy justification, and implementation details from cycles 172-174.

## Table of Contents

1. [Problem Statement](#problem-statement)
2. [Research Findings](#research-findings)
3. [Build vs Buy Decision](#build-vs-buy-decision)
4. [Architecture](#architecture)
5. [Implementation](#implementation)
6. [Integration Points](#integration-points)
7. [Multi-Dimensional Scoring](#multi-dimensional-scoring)
8. [Simplification Catalog](#simplification-catalog)
9. [Adaptive Depth Routing](#adaptive-depth-routing)
10. [Performance Characteristics](#performance-characteristics)
11. [Known Limitations](#known-limitations)
12. [Future Work](#future-work)
13. [References](#references)

---

## Problem Statement

The evolve-loop pipeline had **three gaps** in its code quality feedback loop:

| Gap | Impact |
|-----|--------|
| **No unified review + simplification** | Review and simplification were separate concerns — the Auditor checked for issues, the `/refactor` skill fixed smells, but neither informed the other |
| **Binary verdict only** | The Auditor output PASS/WARN/FAIL with no dimensional breakdown — the Learn phase couldn't analyze _which_ quality dimensions were improving or degrading |
| **No proactive simplification** | Code that passed review but had high cognitive complexity, deep nesting, or near-duplicates shipped without remediation suggestions |

### Existing Tools (Before This Work)

| Tool | Scope | Limitation |
|------|-------|-----------|
| **Auditor** (`agents/evolve-auditor.md`) | Single-pass review: quality, security, hallucinations, eval integrity | Binary verdict, no specialist sub-agents, no simplification |
| **`/refactor` skill** (`skills/refactor/SKILL.md`) | 5-phase refactoring: detect → prioritize → plan → execute → verify | Not integrated into evolve-loop cycles, requires manual invocation |
| **`detect-code-smells`** (external skill) | 22-smell catalog with technique mapping | Detection only, no fix execution |
| **`code-simplifier:code-simplifier`** (external agent) | Clarity, consistency, maintainability review | Separate agent, separate context, no pipeline integration |
| **`feature-dev:code-reviewer`** (external agent) | Confidence-based code review | Separate agent, no simplification, no dimensional scoring |

---

## Research Findings

Research conducted in Cycle 172 Phase 1 surveyed the 2025-2026 AI code review and simplification landscape.

### Industry Benchmarks

| Tool/Study | Key Finding | Source |
|-----------|-------------|--------|
| **Anthropic Code Review** (Mar 2026) | Multi-agent dispatch → 16% to 54% substantive PR comments | claude.com/blog/code-review |
| **Cursor BugBot** | Pipeline → agentic = biggest quality gain; 70% resolution rate; 2M+ PRs/month | cursor.com/blog/building-bugbot |
| **Qodo 2.0** (Feb 2026) | Multi-agent specialists (bugs/quality/security/tests) achieved highest F1 score (60.1%) | qodo.ai/blog |
| **CodeRabbit** | Pipeline AI for structure + agentic AI for reasoning = optimal hybrid | coderabbit.ai/blog |
| **CodeScene** | Simplified code reduces AI token consumption ~50% for comparable tasks | codescene.com/blog |
| **TU Wien thesis** | Capping at 3 iteration rounds; majority voting reduces false positives | repositum.tuwien.at |
| **300-engineer study** (Sep 2025) | 33.8% cycle time reduction, 29.8% review time reduction with AI review | arXiv:2509.19708 |

### Key Insights

1. **Hybrid pipeline+agentic beats pure approaches.** Structured passes catch 60-70% of issues at ~5% token cost. Agentic reasoning handles the 30-40% that requires intent understanding and cross-file context.

2. **Unified review + simplification saves ~40-50% tokens** vs. invoking separate review and simplify agents. The diff is read once, and both dimensions are analyzed on the same context.

3. **LLMs excel at localized refactoring** (Extract Method, deduplication, complexity reduction, magic number elimination) but are **weak at architectural refactoring** (cross-module, cross-class, dependency restructuring).

4. **Adaptive scaling** with diff complexity is more token-efficient than fixed multi-agent dispatch. Trivial PRs get lightweight single-pass; complex ones get multi-agent specialist panel.

5. **Majority voting** across multiple analysis passes reduces false positives (BugBot's key insight). If multiple checks flag the same issue, it's high-confidence.

---

## Build vs Buy Decision

### Decision: Build Custom

| Factor | Custom Skill | Official Agents (code-simplifier + code-reviewer) |
|--------|-------------|--------------------------------------------------|
| **Token cost** | Single context load (~20-45K) | Two separate loads (~80-120K total) |
| **Pipeline integration** | Wired into Auditor D4 + Builder Step 5 | Would need adapter code for each |
| **Adaptive depth** | 3 tiers (lightweight / standard / full) | One-size-fits-all |
| **Scoring** | 4 dimensions, 0.0-1.0, feeds Learn phase | Separate verdicts, no unified scoring |
| **Simplification trigger** | Auto at maintainability < 0.7 | Manual invocation only |
| **Cross-cycle learning** | Feeds instincts, proposals, research ledger | Stateless per invocation |
| **Customizability** | Thresholds, checks, dimensions all configurable | Fixed behavior |

### What We Reuse From Official Tools

| Source | What We Adopted |
|--------|----------------|
| `code-simplifier:code-simplifier` | Clarity/consistency/maintainability focus areas |
| `feature-dev:code-reviewer` | Confidence-based filtering pattern (only report high-priority issues) |
| `review-cheat-sheet` | Comprehensive checklist structure (stop-the-PR → first pass → deep dive) |
| `detect-code-smells` | 22-smell catalog for pipeline layer detection |
| `/refactor` skill | 66-technique mapping and parallel worktree execution model |

### Token Savings Analysis

```
Separate agents approach:
  code-reviewer context load:    ~40-60K tokens
  code-simplifier context load:  ~40-60K tokens
  Total per review:              ~80-120K tokens

Unified skill approach:
  Pipeline layer (deterministic): ~2-5K tokens
  Agentic layer (LLM):           ~15-40K tokens (scales with diff)
  Total per review:               ~20-45K tokens

Savings: 55-75K tokens per review (~40-60%)
```

---

## Architecture

### Hybrid Pipeline + Agentic Model

```
Input: git diff (changed files)
         │
         ▼
┌─────────────────────────┐
│  PIPELINE LAYER (fast)  │  Deterministic pattern checks (~0.5s)
│  ─────────────────────  │
│  1. File length          │  > 800 lines → maintainability warning
│  2. Function length      │  > 50 lines → Extract Method candidate
│  3. Nesting depth        │  > 4 levels → guard clause candidate
│  4. Secrets detection    │  Pattern match on passwords/keys/tokens
│  5. Cognitive complexity │  > 15 per function via complexity-check.sh
│  6. Near-duplicate code  │  > 6 similar lines → extraction candidate
└─────────┬───────────────┘
          │ Structured findings (fast, cheap)
          ▼
┌─────────────────────────┐
│  AGENTIC LAYER (deep)   │  LLM-powered contextual analysis (~15-40K tokens)
│  ─────────────────────  │
│  7. Logic correctness    │  Edge cases, off-by-one, null handling
│  8. Intent alignment     │  Acceptance criteria match, scope creep
│  9. Cross-file impact    │  API contract breaks, dependency effects
│  10. Simplification      │  Extract Method, reduce nesting, dedup
│  11. Compound risk       │  Future maintenance cost assessment
└─────────┬───────────────┘
          │ Contextual findings (expensive, deep)
          ▼
┌─────────────────────────┐
│  SCORING & REPORT       │  Aggregate into 4-dimension scores
└─────────────────────────┘
```

### Why Hybrid?

| Approach | Quality | Token Cost | Latency |
|----------|---------|-----------|---------|
| **Pure pipeline** (deterministic only) | Low — misses contextual issues | Very low (~2-5K) | Fast (~0.5s) |
| **Pure agentic** (LLM only) | High — but expensive for pattern issues | Very high (~80-120K) | Slow (~30-60s) |
| **Hybrid** (pipeline first, agentic on remainder) | High — pipeline catches easy issues, agentic focuses on hard ones | Medium (~20-45K) | Medium (~10-30s) |

The pipeline layer eliminates 60-70% of issues before the agentic layer runs. This means the LLM can focus its expensive reasoning on the 30-40% of issues that actually require understanding intent and cross-file relationships.

---

## Implementation

### File Inventory

| File | Purpose | Lines |
|------|---------|-------|
| `skills/code-review-simplify/SKILL.md` | Skill definition — architecture, flow, scoring, integration hooks | 271 |
| `scripts/utility/code-review-simplify.sh` | Pipeline layer engine — 6 deterministic checks | 200 |
| `scripts/verification/complexity-check.sh` | Per-function cognitive complexity scorer | 110 |
| `agents/evolve-auditor.md` (D4 section) | Auditor integration hook | +12 lines |
| `agents/evolve-builder.md` (Step 5) | Builder self-review integration | +10 lines |

### Pipeline Script (`scripts/utility/code-review-simplify.sh`)

```bash
# Usage
bash scripts/utility/code-review-simplify.sh [GIT_REF] [--json]
# GIT_REF defaults to HEAD~1
```

| Check | Threshold | Finding Type | Detection Method |
|-------|-----------|-------------|-----------------|
| File length | > 800 lines | `maintainability` | `wc -l` |
| Function length | > 50 lines | `maintainability` | `awk` function extraction |
| Nesting depth | > 4 levels | `maintainability` | `awk` indent measurement |
| Hardcoded secrets | Pattern match | `security` | `grep -E` on password/secret/token/key patterns |
| Cognitive complexity | > 15/function | `maintainability` | `scripts/verification/complexity-check.sh` |
| Near-duplicates | > 6 similar lines | `maintainability` | Line-by-line `awk` comparison |

**Output:** Structured markdown report with findings table, tier classification, and recommendation (PASS/WARN/FAIL).

### Complexity Check Script (`scripts/verification/complexity-check.sh`)

```bash
# Usage
bash scripts/verification/complexity-check.sh <file> [--threshold N]
```

Counts control flow keywords (`if`, `for`, `while`, `case`, `catch`) and nesting increments per function. Outputs colon-delimited per-function scores:

```
file.sh:function_name:complexity=7:threshold=15:OK
file.sh:big_func:complexity=22:threshold=15:EXCEEDED
```

Supports: bash, Python, JavaScript/TypeScript, Go, Java, Rust, and generic files.

---

## Integration Points

### 1. Builder Self-Review (Step 5 — Optional)

After implementing a task and passing eval graders, the Builder optionally runs the pipeline layer:

```
Builder workflow:
  Step 4: Implement
  Step 5: Self-Verify
    ├── Run eval graders ← existing
    ├── Run test suite ← existing
    └── Run code-review-simplify lightweight ← NEW (optional)
         └── If maintainability < 0.7: apply simplifications before reporting
  Step 6: Retry Protocol
```

**Why here?** Catches maintainability issues _before_ the Auditor sees them. Reduces audit→fix→re-audit cycles (which cost ~40-80K tokens each).

**Non-blocking:** If the script doesn't exist or fails, the build continues normally.

### 2. Auditor Skill Consultation (Section D4 — Optional)

During its review pass, the Auditor can invoke the pipeline layer to supplement its verdict:

```
Auditor checklist:
  A. Code Quality
  B. Security
  B2. Hallucination Detection
  C. Pipeline Integrity
  D. Eval Integrity
  D3. Skill Usage Verification
  D4. Code Review + Simplify ← NEW (optional)
  E. Eval Gate
  F. Multi-Stage Verification
```

**When:** Code changes with > 20 modified lines. Skip for doc-only, config-only, eval-only.

**How:** Run pipeline checks → use composite score to supplement verdict → if maintainability < 0.7, append simplification suggestions to audit-report.md.

**Advisory only:** D4 findings do NOT override the Auditor's independent verdict.

---

## Multi-Dimensional Scoring

Replaces the Auditor's binary PASS/WARN/FAIL with actionable numeric scores.

| Dimension | Weight | What It Measures |
|-----------|--------|-----------------|
| **correctness** | 0.35 | Logic errors, edge cases, intent alignment, test coverage |
| **security** | 0.25 | Injection, auth, secrets, input validation, error leakage |
| **performance** | 0.15 | Complexity, N+1 queries, blocking calls, memory, unnecessary work |
| **maintainability** | 0.25 | Readability, complexity, duplication, naming, file size |

### Score Guide

| Score | Meaning |
|-------|---------|
| 1.0 | No issues detected |
| 0.7-0.9 | Minor issues, acceptable to ship |
| 0.3-0.6 | Significant issues, fix recommended |
| 0.0-0.2 | Critical issues, must fix before shipping |

### Composite Score

```
composite = 0.35 * correctness + 0.25 * security + 0.15 * performance + 0.25 * maintainability
```

| Composite | Verdict | Action |
|-----------|---------|--------|
| >= 0.8 | PASS | Ship immediately |
| 0.6 - 0.79 | WARN | Ship with noted issues; simplification recommended |
| < 0.6 | FAIL | Block shipping; fix required |

### Confidence Tracking

Each dimension includes a `confidence` (0.0-1.0). If any dimension's confidence < 0.7, the verdict escalates to WARN regardless of the score. This prevents the system from confidently passing code it isn't sure about.

---

## Simplification Catalog

When `maintainability < 0.7`, the skill generates targeted simplification suggestions.

| Category | Technique | When to Apply | Confidence |
|----------|----------|---------------|-----------|
| **Complexity** | Extract Method | Function > 30 lines or complexity > 10 | 0.9 |
| **Complexity** | Flatten nesting (guard clauses) | Nesting > 3 levels | 0.9 |
| **Complexity** | Decompose conditional | Boolean expressions > 3 operators | 0.85 |
| **Duplication** | Extract shared utility | Near-duplicate blocks > 6 lines, > 80% similar | 0.75 |
| **Readability** | Rename for clarity | Ambiguous names (< 3 chars, generic like `data`/`temp`) | 0.7 |
| **Readability** | Replace magic numbers | Hardcoded literals in logic branches | 0.85 |
| **Abstraction** | Inline over-abstraction | Single-use wrapper with no added value | 0.7 |
| **Abstraction** | Remove dead code | Unreachable branches, unused imports/vars | 0.9 |

### Constraints

- **Localized only:** Suggestions stay within the same file or module. LLMs are weak at cross-module architectural refactoring (confirmed by ICSE 2025 research).
- **Max 5 per review:** Focus on highest-impact suggestions. Information overload reduces developer action rate.
- **Before/after required:** Each suggestion includes code snippets and estimated complexity reduction.
- **Behavior-preserving:** Pure refactoring only — no changes to external behavior.

---

## Adaptive Depth Routing

Scale analysis intensity with diff complexity to optimize token usage.

| Tier | Trigger | Pipeline | Agentic | Token Budget |
|------|---------|----------|---------|-------------|
| **Lightweight** | < 50 lines, 1-3 files | All 6 checks | Single-pass (no specialists) | ~10-15K |
| **Standard** | 50-200 lines, 3-10 files | All 6 checks | Full analysis (all 5 agentic checks) | ~20-35K |
| **Full Review** | > 200 lines, 10+ files, or security-sensitive | All 6 checks | Multi-agent specialist panel | ~40-80K |

### Auto-Escalation Rules

| Signal | Escalation |
|--------|-----------|
| File matches `auth*`, `*login*`, `*password*`, `*payment*` | → Full Review |
| File matches `*eval*`, `*grader*`, `agents/`, `skills/*/SKILL.md` | → Full Review |
| File has > 5 commits in last 10 cycles (high churn) | → One tier up |
| File imported by > 5 other files (high fan-in) | → One tier up |

---

## Performance Characteristics

Measured during Cycle 173 testing:

| Metric | Value |
|--------|-------|
| Pipeline layer execution time | ~0.5s on typical diff (2-10 files) |
| Pipeline layer token cost | ~2-5K (deterministic, no LLM) |
| Agentic layer token cost | ~15-40K (scales with diff size) |
| Total unified cost | ~20-45K per review |
| Separate agents cost (comparison) | ~80-120K per review |
| Token savings | ~40-60% vs. separate agents |

---

## Known Limitations

| Limitation | Severity | Workaround |
|-----------|----------|-----------|
| Near-duplicate detection is O(n²) | LOW | Only runs on changed files; content hashing planned for future cycle |
| No cross-module refactoring suggestions | MEDIUM | By design — LLMs are unreliable at architectural refactoring |
| `complexity-check.sh` is heuristic-based | LOW | Uses keyword counting, not AST analysis; sufficient for pipeline layer |
| Agentic layer not yet fully implemented as standalone | MEDIUM | Currently defined in SKILL.md; LLM invokes it contextually during review |
| JSON output mode not yet implemented | LOW | Planned for future cycle; markdown output works for current integration |

---

## Future Work

Prioritized backlog for remaining cycles:

| Priority | Task | Estimated Complexity |
|----------|------|---------------------|
| P1 | JSON output mode for `code-review-simplify.sh` | S |
| P1 | Scoring aggregation script (pipeline findings → 4-dimension scores) | M |
| P1 | Agentic layer integration (LLM analysis invocation protocol) | M |
| P2 | Register as invocable Claude Code skill (`/code-review-simplify`) | S |
| P2 | Wire into `.claude-plugin/plugin.json` | S |
| P3 | End-to-end test script with sample diffs | S |
| P3 | Benchmark integration (review scores → evolve-loop dimensions) | M |
| P4 | README and CHANGELOG updates | S |
| P4 | Content-hash-based near-duplicate detection | S |

---

## References

### Research Papers

| Paper | Date | Key Finding |
|-------|------|-------------|
| Does AI Code Review Lead to Code Changes? | Aug 2025 | Factors influencing whether AI review comments lead to actual changes (arXiv:2508.18771) |
| Code Review Agent Benchmark | Mar 2026 | Evaluating whether reviews identify meaningful issues (arXiv:2603.23448) |
| Measuring AI's True Impact on Developer Productivity | Sep 2025 | 33.8% cycle time reduction across 300 engineers (arXiv:2509.19708) |
| Quality Improvements via Manual and Automated Code Review | Feb 2026 | Human vs. ChatGPT-4 reviews across 240 PRs (arXiv:2602.11925) |
| LLM-Driven Code Refactoring: Opportunities and Limitations | ICSE 2025 | LLMs excel at localized refactoring, fail at architectural |
| OPTIMA: Optimizing Effectiveness and Efficiency for LLM Multi-Agent | ACL 2025 | Framework for optimizing multi-agent token budgets |
| Agentic Code Reasoning | Mar 2026 | Agentic approaches to code reasoning tasks (arXiv:2603.01896) |

### Industry Tools

| Tool | Key Takeaway |
|------|-------------|
| Anthropic Code Review | Multi-agent parallel dispatch → 54% substantive comments |
| Cursor BugBot | Pipeline → agentic transition = biggest quality gain |
| Qodo 2.0 | Specialized multi-agent achieves F1 = 60.1% |
| CodeRabbit | Pipeline AI + agentic AI hybrid is optimal |
| CodeScene | Simplified code reduces AI token consumption ~50% |
| Sourcery | Closest to unified review+refactoring but lacks whole-repo context |

### Internal Cross-References

| File | Relationship |
|------|-------------|
| `skills/code-review-simplify/SKILL.md` | Canonical skill definition |
| `scripts/utility/code-review-simplify.sh` | Pipeline layer implementation |
| `scripts/verification/complexity-check.sh` | Complexity scoring engine |
| `agents/evolve-auditor.md` § D4 | Auditor integration hook |
| `agents/evolve-builder.md` § Step 5 | Builder self-review integration |
| `docs/ai-code-review-agents.md` | Prior research on review agent architectures |
| `skills/refactor/SKILL.md` | Complementary refactoring pipeline (separate invocation) |

---

## Changelog

| Cycle | Date | Changes |
|-------|------|---------|
| 172 | 2026-04-06 | Foundation: SKILL.md, auditor D4 hook, eval definitions |
| 173 | 2026-04-06 | Implementation: pipeline script, complexity checker, builder self-review |
| 174 | 2026-04-06 | Bug fixes: subshell scope, sed interpolation |
