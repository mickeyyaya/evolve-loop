# ADR-0055: Cycle Dossier

**Date:** 2026-06-18  
**Status:** Accepted  
**Deciders:** evolve-loop maintainers  
**Related:** ADR-0039 (failure floor), ADR-0044 (phase recovery), ADR-0052 (advisor maximization)

---

## Status

Accepted — D1 committed in `34da46fe`; D2–D4 implemented in cycle 2.

---

## Context

Every evolve-loop cycle produces structured data — phase reports, ledger entries, lessons, carryover todos — but this data lives exclusively in gitignored runtime directories (`.evolve/`, per-cycle `workspace/`) and is lost across sessions, branches, and loop restarts. Concretely:

1. **Scout blindness:** The next cycle's Scout cannot read prior-cycle outcomes; it re-discovers the same gaps cycle after cycle.
2. **FAIL experience loss:** When a cycle fails, its `why` (defects) and `fix` (carryover) are written to `state.json:failedApproaches[]` but never aggregated into a single committed artifact that cross-session tools can read.
3. **No learning compounding:** The loop cannot improve from its own history because the history is not durable.

The goal: aggregate every cycle's structured data into ONE committed artifact (`knowledge-base/cycles/cycle-N.json`) that any future cycle, session, or script can read as the single source of truth.

### Prior art and constraints

- `state.json` carries `failedApproaches[]` and `carryoverTodos[]` but is a runtime mutable file — not suitable as an immutable learning record.
- The ledger (`ledger.jsonl`) is an append-only hash-chain of phase events, but is not human-readable and requires the ledger adapter to parse.
- Phase reports (`*-report.md`) are workspace-scoped and gitignored.
- The Go struct must be the SSOT; `schemas/cycle-dossier.schema.json` is the committed human/cross-tool reference kept in sync by a drift test.

---

## Decision

Introduce the `go/internal/dossier` package implementing the Dossier aggregate type with:

### D1 — Core type (committed in `34da46fe`)

- `Dossier` struct with `Cycle`, `Goal`, `FinalVerdict`, `Phases`, `Defects`, `Lessons`, `Carryover`.
- `PhaseRecord`, `Defect`, `Lesson`, `Carryover` value types.
- `Validate()` as the deterministic trust boundary; the load-bearing invariant: a FAIL verdict MUST carry ≥1 defect (why) AND ≥1 carryover (the fix work) — no failure's experience is silently dropped.

### D2 — Recorder (cycle 2)

- `BuildOpts` + `Build(cycle int, opts BuildOpts) (*Dossier, error)` — assembles a Dossier from runtime artifacts; validates cycle > 0.
- `RenderJSON(*Dossier) ([]byte, error)` — indented JSON via `encoding/json`.
- `RenderMarkdown(*Dossier) ([]byte, error)` — human-readable markdown via `text/template`.
- `ParseJSON([]byte) (*Dossier, error)` — deserialisation for the verify CLI.
- `Write(d *Dossier, dir string, commit bool) error` — atomic temp+rename via `atomicwrite.Bytes`; creates `cycle-N.json` + `cycle-N.md`.
- `schemas/cycle-dossier.schema.json` — JSON Schema draft-07, kept in sync with the Go struct by a drift test.
- `core.ApplyDefectsAsCarryoverTodos(state *State, record FailedRecord)` exported function — promotes each `FailedRecord.Defects[]` entry into its own `CarryoverTodo` (one per defect, not one generic todo per cycle). This is the D2 contract: individual defects must be individually actionable.

### D3 — ACS gate (cycle 2)

- `go/acs/redteam/cycle_dossier_test.go` — standing red-team predicates: `TestCycleDossier` (dossiers present for completed cycles), `TestCycleDossier_MissingDossier` (asserts red on absent dossier), `TestCycleDossier_SkipsInProgress` (skips in-progress cycles).
- `.evolve/policy.json` — `dossier-closeout` gate enrolled in the floor.
- `go/cmd/evolve/cmd_dossier.go` — `evolve dossier verify` subcommand (read-only; validates all `knowledge-base/cycles/*.json`; exit 0 all-OK, exit 1 any invalid).

### D4 — Documentation (cycle 2)

- `docs/architecture/adr/0055-cycle-dossier.md` (this file).
- `agents/evolve-scout.md` — new §2.5 Prior Cycle Dossier Recall step: Scout reads `knowledge-base/cycles/` before task selection.
- `docs/operations/runtime-reference.md` — documents `evolve dossier verify`.

---

## Consequences

### Positive

1. **Scout recall:** Prior-cycle dossiers are now readable structured facts; Scout can avoid re-discovering the same gaps.
2. **FAIL learning compounding:** A FAIL verdict now carries durable evidence of both the failure root cause (defects) and the remediation work (carryover). The `Validate()` invariant enforces this at write time.
3. **Cross-session auditability:** Any session, script, or external tool can read `knowledge-base/cycles/cycle-N.json` without access to gitignored runtime state.
4. **Individual defect trackability:** `ApplyDefectsAsCarryoverTodos` breaks one generic failure todo into per-defect todos, making each defect individually addressable by the next cycle's Scout and Builder.

### Negative / trade-offs

1. **Commit noise:** Each cycle adds two files to `knowledge-base/cycles/` (`cycle-N.json` + `cycle-N.md`). This is intentional — the growth is bounded (one pair per cycle) and the files are the learning record.
2. **Build overhead:** `dossier.Build + Write` runs in `finalizeCycle` (post-loop). It is non-blocking (errors WARN, not fail the cycle) and deterministic (no LLM call), so cycle latency impact is negligible.
3. **Schema drift risk:** The Go struct is the SSOT; `schemas/cycle-dossier.schema.json` is a derived artifact. The `TestSchema_NoDrift` drift test guards this in CI.

### Neutral

- The `commit bool` parameter in `Write` is reserved for a future slice that will git-commit the dossier files. For now, pass `false`; the caller commits via the normal ship path.
- `Build` currently synthesises a minimal `"cycle-recorded"` phase when no `LedgerPath` is provided. A future slice will wire the real ledger walk (producing one `PhaseRecord` per ledger entry).

---

## Implementation Notes

- **Minimal change principle:** `Build` is intentionally thin — it does not yet walk the ledger or read workspace reports. This keeps the D2 slice focused and reversible; ledger integration is a future cycle.
- **Atomic writes:** `Write` uses `atomicwrite.Bytes` (temp+rename) — the same pattern used throughout the codebase. No custom atomicWrite reimplementation.
- **No LLM calls in dossier:** All dossier operations (Build, Render, Write, Validate) are deterministic. LLM judgment (e.g. lesson synthesis) is a future concern, not part of the D1–D4 contract.
- **CLI is read-only:** `evolve dossier verify` never mutates state. It is safe to run mid-batch, in CI, or from any session.
