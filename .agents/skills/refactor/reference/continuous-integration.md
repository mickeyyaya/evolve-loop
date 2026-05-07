---
name: reference
description: Reference doc.
---

# Continuous Refactoring Integration

> Read this file to embed refactoring into PR review, debt tracking, and velocity dashboards rather than treating it as a separate activity.

## PR Review Integration

Run `/refactor scan` automatically on PR diffs to catch smells before merge:

| Trigger | Scope | Action |
|---------|-------|--------|
| PR opened/updated | Changed files only | Run scan pipeline, post smell report as PR comment |
| PR touches >5 files | Full affected subgraph | Run architecture analysis, flag boundary violations |
| PR increases complexity | Functions with delta >5 | Suggest specific refactoring technique inline |
| PR introduces duplicates | New duplicate blocks | Flag with jscpd report |

## Technical Debt Budget

Track refactoring debt as a measurable quantity:

| Metric | How to Compute | Target |
|--------|---------------|--------|
| Smell density | Total smells / total functions | <0.1 (10%) |
| Average composite score | Mean of all function health scores | <0.3 |
| Architecture violations | Count of boundary violations | 0 |
| Circular dependency count | DFS cycle count | 0 |
| Critical functions | Functions with composite score >0.7 | 0 |

## Refactoring Velocity Tracking

Track improvement over time:

```
| Sprint | Smells Found | Smells Fixed | Net Change | Debt Trend |
|--------|-------------|-------------|------------|------------|
| Week 1 | 45 | 0 | +45 | Baseline |
| Week 2 | 48 | 12 | -9 | Improving |
| Week 3 | 41 | 8 | -4 | Improving |
```

If debt trend reverses for 2+ sprints, escalate to team lead.
