# Handoff Artifact Schema — Architecture Decision Record

> **C2-handoff-schemas** | Cycle 38 | Status: SHIPPED | File: `docs/architecture/handoff-artifact-schema.md`

## TOC
- [Context](#context)
- [Schema Definitions](#schema-definitions)
- [Lint Script Interface](#lint-script-interface)
- [Phase-Gate Integration](#phase-gate-integration)
- [Schema Design Decisions](#schema-design-decisions)
- [Promotion Path (C4/C5)](#promotion-path-c4c5)

---

## Context

evolve-loop relies on three markdown handoff artifacts per cycle:

| Artifact | Producer | Consumer |
|----------|----------|---------|
| `scout-report.md` | Scout | Builder, Triage, Orchestrator |
| `build-report.md` | Builder | Auditor |
| `audit-report.md` | Auditor | Orchestrator, ship.sh |

Before C2, there was no structural contract for these files. A phase agent could produce a malformed artifact (missing challenge token, missing verdict, missing task sections) and downstream phases would silently mis-parse or hallucinate content. C2 defines machine-readable schemas and a bash lint script that validates artifacts against them.

**Design constraint**: bash-native, jq-readable JSON — no external validators (no Python/Node/ajv). This matches the project's existing tooling pattern and ensures zero new dependencies.

---

## Schema Definitions

Schema files live at `schemas/handoff/<type>-report.schema.json`.

### Common fields

| Field | Type | Purpose |
|-------|------|---------|
| `$id` | string | Identifier URI |
| `artifact_type` | string | `scout-report` / `build-report` / `audit-report` |
| `version` | number | Schema version (1) |
| `required_first_line` | object | Pattern that line 1 must match (challenge-token check) |
| `required_sections[]` | array | Sections where ANY pattern in `.patterns[]` satisfies |
| `conditional_sections[]` | array | Sections required only when `.condition` is met |
| `required_content[]` | array | Patterns that must appear anywhere in artifact |
| `min_words` | number | Minimum word count |

### scout-report.schema.json invariants

| Check | Pattern | Notes |
|-------|---------|-------|
| Line 1 | `^<!-- challenge-token:` | Anti-forgery |
| Proposed Tasks | `^## Proposed Tasks` OR `^<!-- ANCHOR:proposed_tasks -->` | Builder input |
| Exit Criteria | `^## Exit Criteria` OR `^## Acceptance Criteria` OR `^<!-- ANCHOR:acceptance_criteria -->` | Auditor input |
| Carryover Decisions | `^## Carryover Decisions` | Conditional: only when `carryoverTodos[]` non-empty (v8.57.0 Layer S) |
| min_words | 100 | Substance check |

### build-report.schema.json invariants

| Check | Pattern | Notes |
|-------|---------|-------|
| Line 1 | `^<!-- challenge-token:` | Anti-forgery |
| Changes | `^## Changes` OR `^<!-- ANCHOR:diff_summary -->` | Diff summary |
| Self-Verification | `^## Self-Verification` OR `^<!-- ANCHOR:test_results -->` | Test results |
| Quality Signals | `^## Quality Signals` | Required quality section |
| Status field | `\*\*Status:\*\*` | PASS / FAIL declaration |
| min_words | 100 | Substance check |

### audit-report.schema.json invariants

| Check | Pattern | Notes |
|-------|---------|-------|
| Line 1 | `^<!-- challenge-token:` | Anti-forgery |
| Artifacts Reviewed | `^## Artifacts Reviewed` | What was examined |
| Verdict | `^## Verdict` | Verdict section |
| Verdict value | `\*\*(PASS\|WARN\|FAIL)\*\*` | Machine-parseable verdict |
| Confidence | `Confidence: 0\.[0-9]` | Score required |
| min_words | 100 | Substance check |

---

## Lint Script Interface

```bash
bash scripts/tests/validate-handoff-artifact.sh \
    --artifact <PATH>          # markdown artifact to validate
    --type scout|build|audit   # selects schema file
    [--state <state.json>]     # required for conditional_sections evaluation
```

**Exit codes**:
- `0` — PASS, no violations
- `1` — violations found (printed to stdout as `VIOLATION[<name>]: <message>`)
- `2` — usage error or missing dependency

**Condition support**: `has_carryover_todos` checks `jq '.carryoverTodos | length > 0' state.json`. Without `--state`, all conditional sections are skipped (backward-compatible).

---

## Phase-Gate Integration

Added as soft WARNs (not FAILs) in three gates — C4/C5 promote to FAIL:

| Gate | Artifact | Gate function |
|------|----------|--------------|
| `discover-to-build` | `scout-report.md` | `gate_discover_to_build` |
| `build-to-audit` | `build-report.md` | `gate_build_to_audit` |
| `audit-to-ship` | `audit-report.md` | `gate_audit_to_ship` |

Each integration is guarded with `[ -x "scripts/tests/validate-handoff-artifact.sh" ]` for backward compatibility: if the script is absent, the gate continues without error.

**Why soft WARN in C2**: Promoting immediately to FAIL would block cycles on the first rollout cycle where agents haven't been updated to produce the new required sections. The WARN→FAIL promotion ladder (v8.55 pattern) lets one verification cycle confirm correctness before enforcing.

---

## Schema Design Decisions

**Why custom JSON format instead of JSON Schema Draft 2020-12?**

JSON Schema validators (ajv, etc.) are designed for JSON documents. These artifacts are Markdown. There is no standard JSON Schema validator for Markdown structural validation. A custom format with explicit `required_sections` / `required_content` arrays is simpler and more readable for this use case.

**Why `patterns[]` array (any-match) instead of a single pattern?**

Artifact producers (Scout, Builder, Auditor) write sections with or without `<!-- ANCHOR:... -->` anchors. The any-match approach lets the schema accept both forms without requiring agents to change their output format.

**Why `min_words: 100`?**

The existing `check_artifact_substance` in phase-gate.sh checks for minimum word count (set to 50 words). C2 schemas use 100 words as a higher bar — an artifact with fewer than 100 words is almost certainly malformed or truncated.

---

## Promotion Path (C4/C5)

| Cycle | Change | Target |
|-------|--------|--------|
| C2 (cycle 38) | Schema files + lint script + soft WARNs in phase-gate | Shipped |
| C4 (cycle 39) | Promote `gate_discover_to_build` WARN → FAIL for scout-report | Queue |
| C5 (cycle 39) | Promote `gate_build_to_audit` and `gate_audit_to_ship` WARNs → FAILs | Queue |
