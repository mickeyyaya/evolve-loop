# Domain adapters — what `.evolve/domain.json` actually does

`docs/reference/configuration.md` has documented a project-level adapter record since v8:

```json
{"domain": "coding", "evalMode": "bash", "shipMechanism": "git", "buildIsolation": "worktree"}
```

Until ADR-0099 slice 2 (2026-09-09) **no Go code read it** — the record was consumed only by the
legacy prompt-layer skills (`skills/loop/*`). This page states the implemented contract.

## Implemented (Go, ADR-0099)

| Field | Reader | Effect |
|---|---|---|
| `domain` | `config.LoadDomain` → `Domain.DefaultDeliverableKind()` | `writing` / `research` ⇒ the project's default deliverable kind is `document`; anything else (`coding`, `design`, `mixed`, unset) ⇒ `code`. The kernel seeds it into the scout/triage dispatch context as `deliverable_kind_default`; a task that declares no kind inherits it, and scout writes the cycle's `deliverable_kind:` header from the tasks it selects. |

The deliverable kind is the ONLY axis the pipeline switches on. Everything else — the spine, the
trust kernel, ship-as-commit, the dossier — is deliverable-agnostic (see the ADR).

## Not implemented (legacy prompt-layer vocabulary)

`evalMode`, `shipMechanism` and `buildIsolation` are parsed for round-trip fidelity only. The
design they described (`rubric` grading, `file-save`/`export` shipping, `file-copy` isolation) was
superseded: solutions live **in this repo**, ship as commits under `solution(<slug>)`, and are
graded by the document contract (`internal/solutioncheck`) plus the audit phase's rubric judgment.

## Where the document contract lives

- Shape: `docs/architecture/phase-registry.json` → `config.deliverable_kinds.document`.
- Engine: `go/internal/solutioncheck` (pure, LLM-free) — projected as the build handoff floor
  (`core.SolutionFloorChecks`), the `evolve solution check <solutions/slug>` CLI, and the audit
  gate line (`audit.Config.CheckSolution`).
- Kind signal: `router.RoutingSignals.DeliverableKind()` (triage > scout > `code`), read from the
  report headers — ADR-0099 slice 1.
