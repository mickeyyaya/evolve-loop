# Artifact Backfill

**Status:** default-on (`workflow.backfill_enabled`, default `true`)
**Implemented:** cycle-171  
**Package:** `go/internal/backfill/`  
**Orchestrator wiring:** `go/internal/core/orchestrator.go`

---

## Problem

When a phase's Write tool call times out (exit code 81, `ErrArtifactTimeout`), the
orchestrator currently aborts the cycle. The phase agent often completed its work
and emitted the full artifact to stdout — that content is captured in
`<workspace>/<phase>-stdout.clean.txt` by the stdout filter (`EVOLVE_STDOUT_FILTER`).
The work is lost, and the cycle dies, even though the output is recoverable.

---

## Mechanism

After `ErrArtifactTimeout` exhaustion (`phaseMaxAttempts` reached):

1. The orchestrator checks the resolved `WorkflowConfig.BackfillEnabled` value.
2. If not `"0"`, it calls `backfill.TryExtract(workspace, phase, artifactPath, 200)`.
3. `TryExtract` reads `<workspace>/<phase>-stdout.clean.txt` and locates the **last
   occurrence** of the canonical markdown header of the phase.
   - It extracts header-to-EOF (trimmed).
   - When `len(content) >= minLen` (200 bytes default), it writes
     the content atomically to `artifactPath`.
4. On success, the orchestrator synthesizes a `PhaseResponse{Verdict: VerdictWARN}`
   and allows the cycle to continue. Downstream phases (auditor, EGPS) see a WARN
   verdict, which is shippable but flagged.
5. On failure (no header, too short, unknown phase, I/O error), the orchestrator
   writes `<phase>-failure-diag.json` and aborts as before.

---

## Header Map

| Phase   | Header string   | Artifact path     |
|---------|----------------|-------------------|
| scout   | `# Scout Report` | `scout-report.md` |
| build   | `# Build Report` | `build-report.md` |
| audit   | `# Audit Report` | `audit-report.md` |
| tdd     | `# TDD`          | `test-report.md`  |
| intent  | `# Intent`       | `intent.md`       |
| triage  | `# Triage`       | `triage-report.md`|
| retro   | `# Retrospective Report` | `retrospective-report.md`|
| build-planner| `# Build Plan` | `build-plan.md` |

Unknown phases return `(false, nil)` — no error, no write.

---

## Minimum Length

`minLen = 200` bytes. Prevents single-line stubs or spinner-noise fragments from
being promoted to a cycle artifact. The value is hardcoded in the orchestrator
wiring; adjust by modifying the `backfill.TryExtract` call site in
`orchestrator.go`.

---

## Relation to ADR-0026 (Stage 1 Self-Healing Review)

ADR-0026 Stage 1 routes all stop conditions through `StopReviewer`. Backfill is a
complementary **recovery path** at the retry-exhaustion seam:

- **StopReviewer** decides *if* it must retry or abort, given a stop condition.
- **Backfill** provides an artifact *reconstruction* path when retry-exhaustion is
  specifically due to a write-timeout (the phase completed, but the file write timed out).

A future cycle can extend StopReviewer to distinguish "spinner-stuck" from
"completing-slowly" before it decides to try backfill or extend the retry budget.

---

## Observability

On successful backfill, the orchestrator emits to stderr:

```
[orchestrator] WARN phase <name>: ErrArtifactTimeout exhausted; backfilled artifact from stdout.clean.txt; proceeding with WARN verdict
```

The synthesized `PhaseResponse{Verdict: VerdictWARN}` flows through the normal
ledger append and audit-binding path. Operators can identify backfilled cycles by
looking for the dedicated `kind=backfill` ledger entry.

---

## Enabling

```json
{
  "workflow": {
    "backfill_enabled": true
  }
}
```

Or use the orchestrator constructor:

```go
core.WithWorkflowConfig(policy.WorkflowConfig{BackfillEnabled: true})
```

---

## Related

- `go/internal/backfill/backfill.go` — `TryExtract` implementation
- `go/internal/core/orchestrator.go` — wiring in `RunCycle` retry loop
- `go/internal/phases/retro/retro.go` — `retro` phase runner updated to align with agent output
- `docs/architecture/self-healing-gaps.md` — self-healing gaps registry
- `docs/architecture/adr/0026-self-healing-review-layer.md` — Stage 1 backlog
- `.evolve/policy.json` — persistent `workflow.backfill_enabled` setting
