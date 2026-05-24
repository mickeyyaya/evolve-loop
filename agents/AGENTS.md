# agents/ — Persona Convention Guide

> **Directory purpose**: Each `evolve-<role>.md` file defines a named agent
> persona — its perspective, workflow, output contract, and behavioral rules.
> As of v12.0.0, personas are loaded by the Go binary's `prompts.Loader`
> (`go/internal/prompts/`) and fed to the configured CLI subprocess via the
> bridge adapter. The `evolve subagent run` CLI provides equivalent manual
> dispatch. This file documents the conventions every persona author must
> follow.

## Frontmatter schema

Every persona file MUST begin with a YAML frontmatter block:

```yaml
---
name: <role>                  # matches the filename stem: evolve-<role>.md
perspective: <one-line role description>
output-format: <primary artifact name>
challenge-token: required     # always "required" for pipeline roles
---
```

Required fields: `name`, `perspective`, `output-format`, `challenge-token`.
Optional: `cross-family-with` (name of role whose model tier must differ).

## Naming convention

| Pattern | Example | Notes |
|---|---|---|
| `evolve-<role>.md` | `evolve-builder.md` | Primary persona file |
| `evolve-<role>-reference.md` | `evolve-builder-reference.md` | Layer 3 on-demand reference (long, not loaded by default) |
| `agent-templates.md` | — | Shared boilerplate fragments referenced from personas |

Persona filenames use kebab-case. No spaces. No version suffixes in filenames
(version history belongs in git log, not in the filename).

## Challenge-token contract

Every pipeline phase persona MUST:
1. Place the challenge token on the **first line** of its output artifact.
   Header format: `<!-- challenge-token: <token> -->`
2. Never fabricate the token. It is injected by `subagent-run.sh` at invocation
   time and validated by `ship-gate.sh` against the ledger entry SHA chain.
3. The token binds the artifact to a specific cycle + ledger entry. A copied
   token from a previous cycle causes ship-gate to reject the artifact.

## Output artifact contract

| Role | Primary artifact | Secondary artifacts |
|---|---|---|
| scout | `scout-report.md` | — |
| tdd-engineer | `test-report.md` | `acs/cycle-N/*.sh` |
| builder | `build-report.md` | changed source files |
| auditor | `audit-report.md` | — |
| orchestrator | orchestrator state | `cycle-state.json` updates |

Artifacts are written to `.evolve/runs/cycle-N/`. Builder additionally writes
to the per-cycle git worktree provisioned by `run-cycle.sh`.

## Reference file conventions

Reference files (`evolve-<role>-reference.md`) follow the same frontmatter
schema. They MUST NOT be loaded unconditionally — they are Layer 3 on-demand
resources, referenced via a `## Reference Index` section in the primary persona.

Load a reference file only when:
- A specific workflow step activates a code path described in the reference
- The primary persona explicitly says "read this reference when X"

## Behavioral guard rails (all personas)

1. **Fail loudly.** Report `N/N PASS` counts. Never claim completion without numbers.
2. **Challenge token first line.** No exceptions. ship-gate rejects artifacts without it.
3. **Single-writer roles write sequentially.** Builder, TDD-Engineer, Orchestrator,
   Intent are `parallel_eligible: false`. Concurrent writes to the same worktree
   corrupt git state.
4. **No in-process Agent calls during a cycle.** Use `subagent-run.sh` exclusively.
   In-process `Agent` bypasses the ledger and profile-scoped permissions.
5. **Do not invent function names.** Read the target module's actual exports before
   importing or calling. Builder-against-imagined-API is the recurring failure mode.

## Discovery

To list all available personas:
```bash
ls agents/evolve-*.md
```

To discover a persona's output contract:
```bash
grep -E '^output-format:' agents/evolve-<role>.md
```

## Related files

- `AGENTS.md` (repo root) — cross-CLI invariants + 12 Core Agent Rules
- `.evolve/profiles/AGENTS.md` — permission profile JSON schema docs
- `agent-templates.md` — shared boilerplate fragments
- `legacy/scripts/dispatch/subagent-run.sh` — persona loading + challenge token injection
