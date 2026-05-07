---
name: evolve-spec
description: Use when starting a new evolve-loop cycle and the cycle goal is ambiguous or only described as a one-line objective. Generates a structured specification before any code is written. Surfaces assumptions explicitly.
---

# evolve-spec

> Sprint 3 composable skill (v8.16+). Inspired by `addyosmani/agent-skills/spec-driven-development`. Used by the `/scout` slash command and called by the `evolve-loop` macro.

## When to invoke

- Cycle goal is one line ("add fan-out to auditor") and details haven't been fleshed out
- The change touches multiple files
- About to make an architectural decision

## When NOT to invoke

- Cycle goal already has explicit task list with file paths
- Single-line bug fix
- Pre-defined task from `failedApproaches` retry queue (the spec already exists)

## Workflow

| Step | Action | Exit criteria |
|---|---|---|
| 1 | Read cycle goal from `.evolve/state.json` | Goal extracted |
| 2 | List explicit assumptions about scope, stack, constraints | At least 3 numbered assumptions |
| 3 | Identify dependency graph (what depends on what) | Dependency tree drawn |
| 4 | Surface unknowns + risks | Risks enumerated with mitigation hints |
| 5 | Write spec to `<workspace>/spec.md` | File present, fresh, contains challenge token |

## Output contract

Spec at `<workspace>/spec.md` with sections:
- `## Goal` (one paragraph)
- `## Assumptions` (numbered list)
- `## Dependency Graph` (text or ASCII)
- `## Risks` (table: risk / likelihood / mitigation)
- `## Out of Scope` (explicit deferrals)

## Composition

Invoked by:
- `/scout` slash command (when scout-codebase sub-scout runs)
- `evolve-loop` macro skill at calibrate→research transition

This skill **does not invoke other personas**. It is a workflow used by Scout-class personas.

## Reference

For deeper detail see `skills/evolve-loop/phase1-research.md`. The spec format follows `addyosmani/agent-skills/spec-driven-development` conventions.
