# Auditor Reference (Layer 3 — on-demand)

Sections here are loaded when the auditor's primary flow needs deeper rules
that are conditional on cycle history (consecutiveClean streak length, force-
full-audit flags, strategy mode). v8.64.0 Campaign D Cycle D2 split.

The auditor's compact role-card (Layer 1) lives at
`agents/evolve-auditor.md` and includes a `## Reference Index` pointing here.

---

## Section: adaptive-strictness

Loaded when the auditor needs to decide which sections of the Single-Pass
Review Checklist to run vs. skip. The full table + rationale is here; the
Layer 1 persona summarizes the rule in one line.

Read `auditorProfile` from context:

```json
{
  "feature":     {"passFirstAttempt": 0, "consecutiveClean": 0},
  "stability":   {"passFirstAttempt": 0, "consecutiveClean": 0},
  "security":    {"passFirstAttempt": 0, "consecutiveClean": 0},
  "techdebt":    {"passFirstAttempt": 0, "consecutiveClean": 0},
  "performance": {"passFirstAttempt": 0, "consecutiveClean": 0}
}
```

**Checklist rules by streak length:**

| `consecutiveClean` | Checklist | Rationale |
|---------------------|-----------|-----------|
| `< 3` (new/unstable) | Full checklist (all sections) | Not enough trust built |
| `3-7` (stable) | Can skip C (Pipeline Integrity) ONLY if no agent/skill files modified | Core quality checks always run |
| `>= 8` (long streak) | **Full checklist mandatory** (streak verification audit) | Long streaks mask slow drift — increase rigor |

**Sections A (Code Quality), B (Security), B2 (Hallucination), D (Eval
Integrity) are NEVER skipped.**

**Random full-audit sampling:** Orchestrator passes `"forceFullAudit": true`
~20% of the time. When true, run ALL sections regardless of streak. Do NOT
mention in report whether this was a random sample (blind monitoring).

**Always run full checklist when:**
- `strategy` is `harden` or `repair`
- Task modifies agent/skill files or `.claude-plugin/`
- Build report flags risks
- `forceFullAudit` is true
- `consecutiveClean >= 8` (streak verification)

**Cross-session decay (orchestrator):** New session halves all
`consecutiveClean` values (rounded down).

**Profile update (orchestrator):** PASS first attempt + no issues =
`consecutiveClean += 1`. WARN/FAIL/MEDIUM+ issue = reset to 0.
