---
name: evaluator
description: Use when the user invokes /evaluator or asks to evaluate, assess, score, or independently audit code quality, project health, or improvement priorities with multi-dimensional scoring and anti-gaming defenses
argument-hint: "[target] [--scope task|project|strategic] [--depth quick|standard|deep]"
---

> Independent evaluation engine. 5-layer architecture, 6 scoring dimensions, anti-gaming defenses, strategic direction guidance. Works standalone or integrated with evolve-loop.

## Contents
- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Stage 1: SCOPE](#stage-1-scope)
- [Stage 2: GRADE](#stage-2-grade)
- [Stage 3: DETECT](#stage-3-detect)
- [Stage 4: SCORE](#stage-4-score)
- [Stage 5: DIRECT](#stage-5-direct)
- [Isolation Principles](#isolation-principles)
- [Depth Control](#depth-control)
- [Evolve-Loop Integration](#evolve-loop-integration)
- [Output Schema](#output-schema)
- [Reference](#reference-read-on-demand)

## Quick Start

```bash
# Evaluate recent changes (task scope)
/evaluator --scope task

# Full project health assessment
/evaluator --scope project --depth deep

# Strategic direction guidance — what should improve next?
/evaluator --scope strategic

# Evaluate specific files
/evaluator src/auth/ --scope task --depth standard
```

**Parse arguments:**
- First positional → `target` (file/directory path, or omit for auto-detect from git diff)
- `--scope task|project|strategic` → evaluation breadth (default: `task`)
- `--depth quick|standard|deep` → analysis thoroughness (default: `standard`)

## Architecture

5-layer evaluation engine: **SCOPE → GRADE → DETECT → SCORE → DIRECT**

```
Layer 5: META-EVAL ──── Eval-of-eval: red-team the evaluator itself
         │                (runs periodically, not every invocation)
Layer 4: DIRECT ──────── Strategic guidance: what should improve next
         │
Layer 3: SCORE ───────── 6-dimension quality assessment (0.0-1.0 each)
         │
Layer 2: DETECT ──────── Anti-gaming: perturbation tests + saturation monitor
         │
Layer 1: GRADE ───────── Deterministic checks + model-based judgment
         │
Input: ───────────────── Target code / project / strategic question
```

**Why 5 layers?** Each layer serves a distinct purpose and can run independently. Layers 1-3 produce scores. Layer 4 produces recommendations. Layer 5 validates the evaluator itself. This separation ensures the evaluator can't be gamed by optimizing for any single layer.

**Research basis:** EDDOps (arXiv:2411.13768) — evaluation as continuous governing function; AISI Inspect Toolkit — physical isolation of evaluator from evaluated system; EST (arXiv:2507.05619) — gaming detection at 2.1% overhead.

## Stage 1: SCOPE

Determine what to evaluate and configure the pipeline.

| Scope | Target | What Gets Evaluated | When to Use |
|-------|--------|--------------------|----- |
| `task` | Changed files (git diff) or specified path | Code quality of specific changes | After implementation, before commit |
| `project` | Full codebase | Overall project health across 6 dimensions | Periodic health check, onboarding |
| `strategic` | Project trajectory | Score trends, regression risks, improvement priorities | Planning, roadmap decisions |

**Auto-detection:** If no target specified:
- `task` scope: `git diff HEAD --name-only` (uncommitted) or `git diff HEAD~1 --name-only` (last commit)
- `project` scope: scan all source files in project root
- `strategic` scope: read `.evolve/state.json` benchmark history if available, else scan codebase

## Stage 2: GRADE (Layer 1)

Run deterministic checks and model-based judgment. Two passes: fast pipeline + deep analysis.

### Pass 1: Pipeline Checks (deterministic)

Reuse `scripts/utility/code-review-simplify.sh` if available, else run equivalent checks:

| Check | Threshold | Dimension Fed |
|-------|-----------|--------------|
| File length | > 800 lines | maintainability |
| Function length | > 50 lines | maintainability |
| Nesting depth | > 4 levels | maintainability |
| Hardcoded secrets | Any match | security |
| Cognitive complexity | > 15/function | maintainability |
| Near-duplicates | > 6 similar lines | maintainability |
| Test existence | Source file has test | completeness |
| TODO/FIXME density | > 5 per file | completeness |

### Pass 2: Model-Based Assessment (contextual)

LLM analyzes code with pipeline findings as context:

| Assessment | Dimension Fed | Prompt Focus |
|-----------|--------------|-------------|
| Logic correctness | correctness | Edge cases, off-by-one, null handling, boundary conditions |
| Requirement alignment | completeness | Does the code do what was asked? Missing requirements? |
| Architectural coherence | architecture | Coupling, cohesion, dependency direction, abstraction levels |
| Future adaptability | evolution | How easy to extend? Hardcoded assumptions? Flexible interfaces? |
| Security review | security | Auth gaps, injection vectors, error info leakage |

**Anti-bias protocol (SURE pipeline):** Apply these checks to model-based assessment:
- **Verbosity bias:** Penalize unnecessarily long code — shorter is not worse
- **Self-preference bias:** Use different evaluation framing than the builder's system prompt
- **Blind trust bias:** Do not assume pipeline PASS means code is correct
- **Confidence requirement:** Score < 0.7 confidence → flag as uncertain, require evidence

## Stage 3: DETECT (Layer 2)

Anti-gaming defenses. Ensures scores reflect actual quality, not format gaming.

### Perturbation Test (EST-inspired)

Based on the Evaluator Stress Test (arXiv:2507.05619):

1. **Take the graded output** from Stage 2
2. **Apply format perturbation:** Strip comments, reorganize imports, rename variables to generic, remove whitespace formatting — preserve semantic content
3. **Re-grade the perturbed version** using Stage 2
4. **Compute gaming indicator:** `G = |original_score - perturbed_score| / original_score`
   - G < 0.1 → **Clean** (score reflects content, not format)
   - G 0.1-0.3 → **Suspect** (some format dependency, flag for review)
   - G > 0.3 → **Gaming detected** (score inflated by format, penalize -20%)

**When to run:** STANDARD and DEEP depth only. Skip for QUICK (token budget constraint).

### Saturation Monitor

Track dimension scores over time (requires `.evolve/state.json` or local history):

| Signal | Threshold | Action |
|--------|-----------|--------|
| Any dimension at 1.0 for 3+ evaluations | Saturation | Introduce harder criteria for that dimension |
| All dimensions above 0.9 | Near-saturation | Flag for meta-evaluation (are criteria too easy?) |
| Score drop > 0.2 in one evaluation | Regression | Highlight as critical finding |

### Proxy-True Correlation

If test results or bug reports are available, compare eval scores to actual outcomes:

| Proxy Metric | True Metric | Expected Correlation |
|-------------|------------|---------------------|
| correctness score | Test pass rate | > 0.7 |
| security score | Vulnerability scan results | > 0.6 |
| maintainability score | Code review time | > 0.5 (inverse) |

Correlation below threshold → flag evaluator for recalibration.

See [reference/anti-gaming.md](reference/anti-gaming.md) for full protocol.

## Stage 4: SCORE (Layer 3)

Aggregate findings into 6 scoring dimensions. Each dimension: 0.0-1.0 with confidence.

| Dimension | Weight | What It Measures | Automated Signals |
|-----------|--------|-----------------|-------------------|
| **correctness** | 0.25 | Logic errors, edge cases, test coverage | Test pass rate, complexity score |
| **security** | 0.20 | Vulnerabilities, secrets, input validation | Secret scan, injection patterns |
| **maintainability** | 0.20 | Complexity, readability, file/function sizes | Pipeline checks, nesting depth |
| **architecture** | 0.15 | Modularity, coupling, dependency direction | File count, import graph, abstraction |
| **completeness** | 0.10 | Requirements coverage, docs, error handling | TODO count, doc existence, try/catch |
| **evolution** | 0.10 | Future adaptability, extensibility | Interface flexibility, hardcoded values |

**Composite:** `composite = sum(dimension * weight)`

**Verdict mapping:**

| Composite | Verdict | Action |
|-----------|---------|--------|
| >= 0.8 | **STRONG** | Project is healthy; maintain current trajectory |
| 0.6 - 0.79 | **ADEQUATE** | Functional but has improvement areas; see DIRECT recommendations |
| 0.4 - 0.59 | **NEEDS WORK** | Significant issues; prioritize DIRECT recommendations |
| < 0.4 | **CRITICAL** | Fundamental problems; immediate action required |

See [reference/scoring-dimensions.md](reference/scoring-dimensions.md) for detailed rubric.

## Stage 5: DIRECT (Layer 4)

Strategic guidance — what should improve next, backed by evidence.

### Recommendation Generation

For each dimension scoring below 0.8:

1. **Identify root cause** — which specific checks or assessments drove the low score?
2. **Map to actionable fix** — what concrete change would improve it?
3. **Score priority:** `priority = (1.0 - dimension_score) * dimension_weight * feasibility`
4. **Evidence link** — cite specific files, lines, or findings

### Output: Ranked Improvement Plan

```markdown
## Improvement Priorities

| # | Dimension | Score | Gap | Recommendation | Evidence | Feasibility |
|---|-----------|-------|-----|---------------|----------|-------------|
| 1 | security | 0.55 | -0.25 | Add input validation to API handlers | src/api/handler.ts:45 — unsanitized user input | 0.9 |
| 2 | evolution | 0.60 | -0.20 | Extract hardcoded config to environment | 12 hardcoded URLs across 4 files | 0.8 |
```

### Regression Alerts

If `.evolve/state.json` is available, compare current scores to historical high-water marks:
- Dimension dropped > 0.1 from HWM → **REGRESSION ALERT**
- 3+ consecutive evaluations trending down → **TREND ALERT**

## Isolation Principles

The evaluator is designed to be **independent from what it evaluates**:

| Principle | Implementation | Research Basis |
|-----------|---------------|---------------|
| **Physical isolation** | Evaluator runs as separate skill invocation, not embedded in build pipeline | AISI Inspect Toolkit |
| **Different perspective** | Model-based assessment uses evaluation-focused prompt, not builder's prompt | CALM self-preference bias (arXiv:2410.02736) |
| **Read-only** | Evaluator never modifies source code — only observes and scores | Anthropic eval principles |
| **Tamper resistance** | Scoring rubric in reference files, not inline — harder to influence | verify-eval.sh pattern |
| **Evidence requirement** | Every score must link to specific observation (file:line or metric) | EDDOps evidence-linked changes |

## Depth Control

| Depth | Pipeline | Model Assessment | Gaming Detection | Token Budget | Duration |
|-------|---------|-----------------|-----------------|-------------|----------|
| **quick** | Full | Single-pass (3 assessments) | Skip | ~15K | ~30s |
| **standard** | Full | Full (5 assessments) | Perturbation test | ~35K | ~2 min |
| **deep** | Full | Full + multi-judge panel | Full EST + saturation | ~60K | ~4 min |

Default: **standard**

## Evolve-Loop Integration

### Phase 4 Delegation (Optional)

When invoked from evolve-loop's Auditor phase:
- **Trigger:** `strategy == "harden"` OR `forceFullAudit == true`
- **Invocation (in-process):** `/evaluator --scope task --depth standard`
- **Invocation (subprocess-isolated, REQUIRED in production cycles):**

  ```bash
  echo "/evaluator --scope task --depth standard" | \
      bash scripts/dispatch/subagent-run.sh evaluator "$CYCLE" "$WORKSPACE_PATH"
  ```

  The runner enforces the evaluator profile (`.evolve/profiles/evaluator.json`) which is read-only at the filesystem level (no Edit/Write outside the evaluator-output artifact) and explicitly disallows WebSearch/WebFetch — the evaluator must score the artifacts on disk, not invent context from the network. Legacy fallback: `LEGACY_AGENT_DISPATCH=1` for one A/B cycle.

- **Result:** Dimension scores merged into audit-report.md under `## Evaluator Scores`
- **Impact:** Advisory — supplements Auditor verdict, does not override

### Standalone Use

Works without any evolve-loop infrastructure. Requires only: file system access, git (for diff), and optionally WebSearch for reference lookup.

## Output Schema

### Evaluation Report (markdown)

```markdown
# Evaluation Report

## Summary
- **Scope:** task | project | strategic
- **Target:** <files or project>
- **Composite:** 0.XX — STRONG | ADEQUATE | NEEDS WORK | CRITICAL
- **Gaming check:** Clean | Suspect | Gaming detected

## Dimension Scores
| Dimension | Score | Confidence | Key Finding |
|-----------|-------|------------|-------------|

## Findings (priority-ranked)
| # | Severity | Dimension | Location | Description |

## Improvement Priorities
| # | Dimension | Gap | Recommendation | Evidence | Feasibility |

## Anti-Gaming Results
| Test | Result | Detail |
```

### JSON variant

```json
{
  "scope": "task|project|strategic",
  "composite": 0.0,
  "verdict": "STRONG|ADEQUATE|NEEDS_WORK|CRITICAL",
  "gamingCheck": "clean|suspect|gaming_detected",
  "dimensions": {
    "correctness": {"score": 0.0, "confidence": 0.0, "findings": []},
    "security": {"score": 0.0, "confidence": 0.0, "findings": []},
    "maintainability": {"score": 0.0, "confidence": 0.0, "findings": []},
    "architecture": {"score": 0.0, "confidence": 0.0, "findings": []},
    "completeness": {"score": 0.0, "confidence": 0.0, "findings": []},
    "evolution": {"score": 0.0, "confidence": 0.0, "findings": []}
  },
  "priorities": [],
  "regressions": []
}
```

## Reference (read on demand)

| File | When to Read |
|------|-------------|
| [reference/scoring-dimensions.md](reference/scoring-dimensions.md) | Detailed 6-dimension rubric with 5-point scales |
| [reference/anti-gaming.md](reference/anti-gaming.md) | EST protocol, saturation detection, gaming patterns |
| [reference/eval-lifecycle.md](reference/eval-lifecycle.md) | Self-improving evaluation, drift detection, adaptive difficulty |
| [docs/evaluator-research.md](../../docs/evaluator-research.md) | Full research archive (14 papers, benchmarks, techniques) |
