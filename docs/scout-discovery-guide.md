# Scout Discovery Guide

Reference guidelines for the Scout agent's discovery and analysis phases. Extracted from `agents/evolve-scout.md` to improve modularity.

## Dimension Evaluation

Evaluate across these dimensions (severity: CRITICAL/HIGH/MEDIUM/LOW):

- **Stability:** error handling, edge cases, test coverage gaps
- **Code quality:** tech debt, duplication, dead code, large files (>800 lines)
- **Security:** exposed secrets, unvalidated inputs, dependency vulnerabilities
- **Architecture:** coupling issues, missing abstractions, scalability bottlenecks
- **Features:** missing functionality, gaps vs goal requirements

Focus on what's actionable. Skip dimensions with no findings.

## Self-Improvement Heuristics

| Signal | Threshold | Proposed Task |
|--------|-----------|---------------|
| `instinctsExtracted == 0` | 2+ consecutive cycles | Instinct-enrichment: review recent builds for extractable patterns |
| `auditIterations > 1.2` (avg) | Last 3 cycles | Builder guidance: add instincts or genes for recurring failure patterns |
| `stagnationPatterns > 0` | Any cycle in last 3 | Task diversity: broaden discovery scope or change strategy |
| `successRate < 0.8` | Last 2 cycles | Task sizing: reduce complexity, prefer S over M tasks |
| `pendingImprovements` not empty | Any entries present | Include as high-priority task candidates |
| Deferred task in `stateJson.evaluatedTasks` with `revisitAfter` date that has passed | Any present | Re-propose the deferred task as a new candidate (capability gap signal) |
| Instinct with `confidence >= 0.6`, not yet graduated, uncited for 3+ consecutive cycles | Any present | Surface as feature-driving task candidate — the pattern exists but isn't being applied (capability gap signal) |

When an introspection heuristic fires, generate a task candidate labeled `source: "introspection"` in the scout report. Introspection tasks compete with codebase-discovered tasks during prioritization — they are not automatically selected, but get a priority boost (treat as priority level 2, after pipeline-blocking issues).

**Capability Gap Scanner:** The last two heuristic signals above form the capability gap scanner. When either signal fires, generate a task candidate labeled `source: "capability-gap"` instead of `source: "introspection"`. These signals surface work the loop previously deferred or has encoded as a learned pattern but never acted on. Capability-gap candidates receive the same priority boost as introspection tasks (priority level 2).

## Hotspot Detection Method

During full scan, identify hotspots by:
1. **Fan-in** — `grep -r "import.*<filename>" --include="*.{ts,py,go}" | wc -l` for each source file. Top 5 by import count.
2. **Size** — Top 5 largest source files by line count.
3. **Churn** — `git log --oneline --follow -- <file> | wc -l` for source files. Top 5 by commit count.

Hotspots help prioritize: fixing a hotspot file has outsized impact; adding complexity to a hotspot file is risky.
