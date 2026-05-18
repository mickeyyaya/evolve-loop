# Retrospective Reference (Layer 3 — on-demand)

> This is the retrospective agent's deep-reference file. Sections here are
> loaded only when the common flow requires format details or schema lookup.
> In the typical FAIL/WARN cycle path, most of this content is not needed.
> Stage 9 cold-move, Cycle 78. Companion to `agents/evolve-retrospective.md`.

---

## Table of Contents

- [Section: digest-format-template](#section-digest-format-template)
- [Section: handoff-schema](#section-handoff-schema)
- [Section: diagnose-reference](#section-diagnose-reference)

---

<!-- ANCHOR:digest-format-template -->
## Section: digest-format-template

Loaded when writing `lessons-digest.md` and the exact format is needed.

```markdown
# Cycle N Retrospective Digest

## Root cause (1 sentence)
<the underlying assumption that turned out to be wrong>

## Lessons (one bullet per lesson)
- **inst-LXXX** (errorCategory): <pattern> — <what to check>
- ...

## Carryover TODOs (top 3 by priority)
- [high] todo-<slug>: <action> (evidence: <pointer>)
- ...

## Contradicted instincts
- <inst-NNN>: <why> (recommend confidence -X.X)
```

Keep the digest under 500 tokens (≈ 2000 chars). The detail YAMLs in
`.evolve/instincts/lessons/` remain the long-form audit trail; the digest
is the "elevator pitch" agents read first.

---

<!-- ANCHOR:handoff-schema -->
## Section: handoff-schema

The `handoff-retrospective.json` (Step 6 in the hot prompt) is formalized
in C3 against `schemas/handoff/audit-report.schema.json` as the closest
available schema.

**Schema:** `schemas/handoff/audit-report.schema.json`

### Required fields

| Field | Type | Description |
|---|---|---|
| `cycle` | int | Cycle number |
| `auditVerdict` | `"FAIL"` \| `"WARN"` \| `"SHIP_GATE_DENIED"` | Trigger verdict |
| `lessonIds` | string[] | IDs of lesson YAMLs written (e.g., `["inst-L042"]`) |
| `errorCategory` | string | `planning` \| `tool-use` \| `reasoning` \| `context` \| `integration` |
| `failedStep` | string | `scout` \| `build` \| `audit` |
| `systemic` | bool | True if ≥2 prior failures with same `errorCategory` |
| `contradictedInstincts` | string[] | IDs of instincts this failure invalidates |
| `preventiveActionCount` | int | Count of distinct preventive actions listed |

Write `retrospective-report.md` first (prose), then `handoff-retrospective.json` (structured).

---

<!-- ANCHOR:diagnose-reference -->
## Section: diagnose-reference

| When | Read this |
|---|---|
| Diagnosing a recurring phase-agent failure or persistent WARN/FAIL | [agents/evolve-diagnose-reference.md](agents/evolve-diagnose-reference.md) |
