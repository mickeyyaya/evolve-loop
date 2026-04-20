---
name: reference
description: Reference doc.
---

> Read this file during Phase 2 prioritization. Composite multi-metric smell scoring formula and git-history enrichment signals.

# Multi-Metric Smell Scoring

Use composite scores instead of single thresholds for more accurate smell detection.

## Composite Health Score per Function

| Metric | Weight | Threshold | Score Formula |
|--------|--------|-----------|--------------|
| Cognitive complexity | 30% | >15 = smell | min(score/25, 1.0) |
| Cyclomatic complexity | 20% | >10 = smell | min(score/20, 1.0) |
| Lines of code | 15% | >50 = smell | min(lines/100, 1.0) |
| Parameter count | 10% | >3 = smell | min(params/6, 1.0) |
| Nesting depth | 15% | >4 = smell | min(depth/6, 1.0) |
| Fan-out (dependencies) | 10% | >10 = smell | min(fanout/15, 1.0) |

**Composite smell score** = weighted sum (0.0 = clean, 1.0 = severely smelly)

| Score | Rating | Action |
|-------|--------|--------|
| 0.0-0.3 | Clean | No action |
| 0.3-0.5 | Mild | Monitor, refactor if in hot path |
| 0.5-0.7 | Moderate | Refactor in next sprint |
| 0.7-1.0 | Severe | Refactor immediately |

## Git-History Enrichment

Enrich smell scores with git history signals:

| Signal | How to Compute | Impact on Priority |
|--------|---------------|-------------------|
| Change frequency | `git log --oneline <file> \| wc -l` | High churn + high smell = urgent |
| Bug correlation | Count fix commits touching this file | Bug-prone + smelly = critical |
| Co-change coupling | Files that always change together | Indicates hidden dependencies |
| Author count | Distinct authors in last 6 months | Many authors = communication cost |
