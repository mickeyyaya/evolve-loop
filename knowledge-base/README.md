# knowledge-base/

> Archival research dossiers. **Excluded from default agent context by design** — read on demand only via `scripts/research/kb-search.sh` or human-driven `Read`/`Grep`.

This directory is the canonical home for long-form research, cycle retrospectives, and discovery records that informed evolve-loop's design but are too voluminous to load into every agent prompt.

## Why this exists separately from `docs/`

The repo has two parallel surfaces, with different audiences and context-load behavior:

| Surface | Audience | Default agent visibility | Lifecycle |
|---|---|---|---|
| `docs/architecture/`, `docs/concepts/`, `docs/reference/`, `docs/incidents/` | Agents (during reasoning) + humans | **Visible** — agents can `Read`/`Grep` freely | Normative; updated as system changes |
| `docs/research/` | Agents (on-demand) + humans | **Visible but rarely cited** — load only when persona instructs | Reference-quality research |
| `knowledge-base/research/` | Humans + retro audits | **Excluded from default context** (`EVOLVE_KB_SEARCH_PATHS` opt-in) | Archival; immutable after promotion |
| `docs/private/` | Humans only | **Kernel-blocked** — agents structurally cannot read | Exploratory backlog |

The split is enforced via `scripts/lifecycle/role-context-builder.sh` (which never auto-loads `knowledge-base/`) and `EVOLVE_KB_SEARCH_PATHS` defaulting to include `knowledge-base/research/` only for the explicit `kb-search.sh` tool. Rationale: agents shouldn't read 200 KB of dossier to find one fact during normal reasoning.

## What belongs here

`knowledge-base/research/` contains four artifact types:

1. **Cycle retrospectives** — `v10-17-0-release-debrief.md`, `cycle-21-cost-attribution.md`. Multi-cycle synthesis explaining what happened across a batch.
2. **Pattern dossiers** — `watchdog-post-memo-sigterm-pattern-2026-05-20.md`, `dual-root-plugin-pattern-bite-2026-05-20.md`. A recurring failure mode observed in 3+ cycles, with research on mechanism and proposed structural fix.
3. **External research records** — `agentic-pipeline-enforcement-2026.md`, `self-correcting-pipelines-ghosh-2026.md`, `token-reduction-2026-may.md`. Paper reviews, library evaluations, prior-art surveys whose conclusions informed shipped features.
4. **One-shot discoveries** — `hermes-agent-proxy-integration.md`, `p-new-6-api-key-constraint.md`. A single investigation whose findings are too narrow for an ADR but valuable to preserve.

Files use the **6-part incident template** (per `feedback_detailed_incident_reports`): What happened / Research / Reasoning / Fix / Lessons / References. Cross-link sibling dossiers via `[[wiki-link]]` syntax for the agent-readable graph.

## What does NOT belong here

| If it's... | Put it in... |
|---|---|
| Settled system design that agents must cite during normal operation | `docs/architecture/` |
| Teaching-first primer for new readers | `docs/concepts/` |
| Forensic post-mortem of a production incident with timeline + accountability | `docs/incidents/` |
| Operational runbook (how to do X) | `docs/guides/` |
| Per-version release narrative | `docs/operations/release-archive.md` + `docs/operations/release-notes/` |
| Agent technique reference cited by a persona file | `docs/reference/` |
| Exploratory backlog, half-baked notes, agent-shouldn't-see-this | `docs/private/` |
| Architecture Decision Record with explicit accept/reject | `docs/architecture/adr/` |

## Promotion: when does a dossier graduate to `docs/architecture/`?

A research dossier is **promoted** to `docs/architecture/` when ALL of the following hold:

1. The pattern or system property has been **validated across ≥3 cycles** (or one cycle plus reproducible eval)
2. The behavior is now **shipped in a production release** and constitutes a contract agents rely on
3. At least one **persona file or skill** would cite it during normal operation
4. The content is **stable** — no expected changes from further cycles

Promotion creates a *new* `docs/architecture/<topic>.md` summarizing the settled design; the original dossier remains in `knowledge-base/research/` as historical context. Cross-link the architecture doc back to the dossier under "References."

**Example**: `knowledge-base/research/eval-grader-best-practices.md` was promoted to `docs/eval-grader-best-practices.md` once it became enforceable at gates. The original was archived to `knowledge-base/research/archived-2026-05-19/` rather than deleted, preserving the research trail.

## Promotion: when does a dossier graduate to `docs/incidents/`?

A research dossier becomes an **incident** when:

- It documents a real failure with user-visible impact (cycle was killed, work was lost, integrity invariant was broken)
- There is a chronological timeline with commit SHAs and timestamps
- Resolution involved either a code fix that shipped or a documented operator workaround
- Future operators benefit from reading it before troubleshooting similar symptoms

Incidents differ from pattern dossiers: incidents close, patterns persist until a structural fix lands. The cycle-94→98 watchdog SIGTERM is currently a **pattern** (heartbeat-touch fix not yet shipped); once shipped, the pattern dossier closes and a single incident report could summarize the cycle-94→98 batch.

## Archive: `archived-2026-05-19/`

The `archived-2026-05-19/` subdirectory holds dossiers that have been promoted to canonical locations elsewhere — they're kept here for git-blame traceability rather than active reference. Five files were archived 2026-05-19 when `docs/architecture/`, `docs/reference/`, and `docs/eval-grader-best-practices.md` were established as canonical homes.

Do not modify archived files. If a topic needs updating, modify the canonical version and (if the change is substantive) write a new dossier here referencing the previous archived version.

## Index of active dossiers

See `git ls-files knowledge-base/research/*.md | sort` for the current set. Active dossiers organize by **topic + date** (e.g., `watchdog-post-memo-sigterm-pattern-2026-05-20.md`), not by cycle number — the same pattern may recur in cycles 94 and 98, and the dossier covers both.

## Operator runbook

When you finish a cycle (or batch) and uncover non-trivial research:

```bash
# 1. Write the dossier (6-part template)
$EDITOR knowledge-base/research/<topic>-YYYY-MM-DD.md

# 2. Cross-link from any docs/architecture/ that references the topic
$EDITOR docs/architecture/<related>.md   # add "See: knowledge-base/research/<topic>.md"

# 3. Commit alongside the cycle's feat-commit (or batch retrospective)
git add knowledge-base/research/<topic>.md
ship.sh --class manual "docs: <topic> dossier from cycle-N retro"
```

The dossier is searchable by future cycles via `bash scripts/research/kb-search.sh <query>` (which honors `EVOLVE_KB_SEARCH_PATHS`).

## See also

- `docs/README.md` — the agent-visible doc-tree taxonomy
- `docs/architecture/private-context-policy.md` — full policy on context exclusion
- `CLAUDE.md` — `EVOLVE_KB_SEARCH_PATHS` env-var contract
- `AGENTS.md` — Knowledge Stewardship Rule (every cycle's research MUST land in one of `docs/` or `knowledge-base/`)
