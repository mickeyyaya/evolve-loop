---
name: scout
description: Use when starting a new evolve-loop cycle and the cycle goal is ambiguous or only described as a one-line objective. Generates a structured specification before any code is written. Surfaces assumptions explicitly.
---

# scout

> Sprint 3 composable skill (v8.16+). Inspired by `addyosmani/agent-skills/spec-driven-development`. Invoked as `/evolve-loop:scout` and called by the `loop` macro.

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

## Spec format (standalone deliverable)

Spec at `<workspace>/spec.md` with sections:
- `## Goal` (one paragraph)
- `## Assumptions` (numbered list)
- `## Dependency Graph` (text or ASCII)
- `## Risks` (table: risk / likelihood / mitigation)
- `## Out of Scope` (explicit deferrals)

<!-- GENERATED:phase-facts BEGIN — do not edit; run `evolve skills generate`. Sources: docs/architecture/phase-registry.json · go/internal/phasecontract · .evolve/profiles/scout.json -->
## Phase facts

| Fact | Value |
|---|---|
| Phase | `scout` (plan archetype, mandatory) |
| Persona | `agents/evolve-scout.md` |
| Profile | `.evolve/profiles/scout.json` — CLI `claude-tmux`, tier `balanced`, fan-out ×3 |
| Inputs | `intent.md` |
| Artifact | `scout-report.md` (cycle workspace) |

## Output contract

`scout-report.md` must declare:

- `## Selected Tasks` (also accepted: `## Proposed Tasks`)

<!-- GENERATED:phase-facts END -->

## Composition

Invoked by:
- `/evolve-loop:scout` (when scout-codebase sub-scout runs)
- `loop` macro skill at calibrate→research transition

This skill **does not invoke other personas**. It is a workflow used by Scout-class personas.

## Reference

For deeper detail see `skills/loop/phase1-research.md`. The spec format follows `addyosmani/agent-skills/spec-driven-development` conventions.
