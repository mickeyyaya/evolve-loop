> **ADR — Research-as-Tool Architecture**
> Status: active | Cycle range: 87–89 | Supersedes: inline-research ad-hoc patterns
> Authors: evolve-loop pipeline | Last updated: cycle-89

# Research-as-Tool Architecture

## Table of Contents

1. [Overview](#overview)
2. [KB-First Directive](#kb-first-directive)
3. [Hook Contract](#hook-contract)
4. [Profile Schema](#profile-schema)
5. [Env Var Reference](#env-var-reference)
6. [Evolution History](#evolution-history)
7. [Cross-References](#cross-references)

---

## Overview

The research-as-tool subsystem governs how evolve-loop agents use knowledge retrieval during a cycle. The guiding principle is **KB-first**: agents must consult the local knowledge base (`scripts/research/kb-search.sh`) before escalating to live web search tools (`WebSearch`, `WebFetch`). This keeps costs low, avoids rate limits on Scout-heavy cycles, and keeps research results reproducible from cached KB content.

Three cycle milestones delivered this subsystem:
- **Cycle 87 (Phase A):** Added `research-quota-gate.sh` hook, `kb-search.sh` utility, and widened 7 agent profiles with `research_quota` fields and allowed-tool entries.
- **Cycle 88 (Phase B):** Retired the Phase-1 dispatch path, added `gate_intent_to_discover`, wired Scout with inline research using the new plumbing.
- **Cycle 89 (Phase C):** Widened KB-first access to 6 non-Scout personas, published this ADR, and surfaced 4 env vars in CLAUDE.md.

---

## KB-First Directive

> **Research quota rule (canonical text — single source, referenced from persona files):**
> Try `scripts/research/kb-search.sh` first; escalate to WebSearch only if KB hits are sparse (< 3 results) or evidently outdated. When escalating, log the query and the KB hit count that triggered the escalation. Combine KB results with web results rather than discarding KB findings.

All 6 non-Scout persona files (`evolve-intent.md`, `evolve-triage.md`, `evolve-tdd-engineer.md`, `evolve-builder.md`, `evolve-auditor.md`, `evolve-retrospective.md`) carry a one-line pointer here. Verification: `grep -rl "kb-search.sh first" agents/` must return ≥ 6 paths.

### When to escalate

| Trigger | Action |
|---------|--------|
| KB hits < 3 for a query | Escalate to WebSearch |
| All KB hits > 5 cycles old | Escalate to WebSearch |
| Query is time-sensitive (release notes, breaking changes) | Escalate immediately |
| KB hits are contradictory without resolution | Escalate to WebSearch for tiebreak |

### When NOT to escalate

- Architecture questions answerable from `docs/` or `knowledge-base/research/`
- Code questions answerable by reading the current source tree
- Questions about this repo's own conventions

---

## Hook Contract

File: `scripts/hooks/research-quota-gate.sh`

The hook is a `PreToolUse` gate that intercepts `WebSearch` and `WebFetch` calls.

| Field | Value |
|-------|-------|
| stdin | JSON `{"tool_name": "...", "tool_input": {...}}` |
| stdout (on deny) | JSON block message |
| rc = 0 | Allow |
| rc = 2 | Deny |

Quota counters are written to `cycle-state.json` under `researchQuota.<agent>`. The hook increments counters even when `EVOLVE_RESEARCH_HOOK_DISABLED=1` (telemetry-only mode).

### Per-agent quotas (default from profiles)

| Agent | `web_search` | `web_fetch` | `kb_search` |
|-------|-------------|------------|-------------|
| evolve-scout | 10 | 15 | 20 |
| evolve-intent | 3 | 5 | 20 |
| evolve-triage | 3 | 5 | 20 |
| evolve-tdd-engineer | 3 | 5 | 20 |
| evolve-builder | 3 | 5 | 20 |
| evolve-auditor | 3 | 5 | 20 |
| evolve-retrospective | 3 | 5 | 20 |

---

## Profile Schema

Each agent profile (`.evolve/profiles/<agent>.json`) includes:

```json
"research_quota": {
  "web_search": 3,
  "web_fetch": 5,
  "kb_search": 20
}
```

`allowed_tools` includes `"WebSearch"`, `"WebFetch"`, and `"Bash(scripts/research/kb-search.sh:*)"`.

### Dual-entry note (verified-intentional)

Several profiles retain `"WebSearch"` and `"WebFetch"` in both `allowed_tools` and `disallowed_tools`. This dual-entry was present when Phase A predicates passed, indicating the Claude Code permission model resolves it correctly: explicit `allowed_tools` entries within scope supersede `disallowed_tools` entries. This state is **intentional** — `disallowed_tools` provides a default-deny baseline; `allowed_tools` grants scoped exceptions. Do not remove either entry without verifying the runner's resolution order.

---

## Env Var Reference

| Subsystem | Env var | Default | Effect |
|-----------|---------|---------|--------|
| Research tool | `EVOLVE_ALLOW_DEEP_RESEARCH` | `0` | When `1`, lifts per-agent quota cap; records `deep_overrides` counter. Does not disable hook telemetry. |
| Research tool | `EVOLVE_RESEARCH_QUOTA_SOFT` | *(planned)* | Soft quota: allows over-quota calls but emits WARN in guards.log. Not yet implemented in `research-quota-gate.sh` as of cycle-89. |
| Research tool | `EVOLVE_RESEARCH_HOOK_DISABLED` | `0` | When `1`, hook is a no-op but counters still increment (telemetry-only mode). |
| Research tool | `EVOLVE_KB_SEARCH_PATHS` | `knowledge-base/research/:.evolve/instincts/lessons/:docs/research/` | Colon-separated root paths for `kb-search.sh`. |

---

## Evolution History

| Cycle | Phase | Deliverable |
|-------|-------|-------------|
| 87 | A — Foundation | `scripts/hooks/research-quota-gate.sh`, `scripts/research/kb-search.sh`, 7 profile `research_quota` fields added |
| 88 | B — Scout migrate | `gate_intent_to_discover` gate, Scout inline-research wiring, Phase-1 dispatch retired |
| 89 | C — Access widening | KB-first directive added to 6 non-Scout personas, this ADR published, 4 env vars surfaced in CLAUDE.md, `docs/research/online-researcher-patterns.md` created |

---

## Cross-References

- Hook implementation: [`scripts/hooks/research-quota-gate.sh`](../../scripts/hooks/research-quota-gate.sh)
- KB search utility: [`scripts/research/kb-search.sh`](../../scripts/research/kb-search.sh)
- Query patterns reference: [`docs/research/online-researcher-patterns.md`](../research/online-researcher-patterns.md)
- Scout persona (inline-research wiring): [`agents/evolve-scout.md`](../../agents/evolve-scout.md)
- Profile directory: [`.evolve/profiles/`](../../.evolve/profiles/)
