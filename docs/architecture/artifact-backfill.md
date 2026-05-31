# Artifact Backfill

**Status:** opt-in (gate: `EVOLVE_BACKFILL_ENABLED=1`, default `0`)  
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

1. The orchestrator checks `EVOLVE_BACKFILL_ENABLED`.
2. If `"1"`, it calls `backfill.TryExtract(workspace, phase, artifactPath, 200)`.
3. `TryExtract` reads `<workspace>/<phase>-stdout.clean.txt`, locates the **last
   occurrence** of the phase's canonical markdown header, extracts header-to-EOF
   (trimmed), and — when `len(content) >= minLen` (200 bytes default) — writes
   the content atomically to `artifactPath`.
4. On success, the orchestrator synthesizes a `PhaseResponse{Verdict: VerdictWARN}`
   and allows the cycle to continue. Downstream phases (auditor, EGPS) see a WARN
   verdict, which is shippable but flagged.
5. On failure (no header, too short, unknown phase, I/O error), the orchestrator
   falls through to write `<phase>-failure-diag.json` and abort as before.

---

## Header Map

| Phase   | Header string   | Artifact path     |
|---------|----------------|-------------------|
| scout   | `# Scout Report` | `scout-report.md` |
| build   | `# Build Report` | `build-report.md` |
| audit   | `# Audit Report` | `audit-report.md` |
| tdd     | `# TDD`          | `tdd-report.md`   |
| intent  | `# Intent`       | `intent-report.md`|
| triage  | `# Triage`       | `triage-report.md`|

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

- **StopReviewer** decides *whether* to retry or abort given a stop condition.
- **Backfill** provides an artifact *reconstruction* path when retry-exhaustion is
  specifically due to a write-timeout (the phase completed, but the file write timed out).

A future cycle could extend StopReviewer to distinguish "spinner-stuck" from
"completing-slowly" before deciding to attempt backfill vs. extend the retry budget.

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

```bash
export EVOLVE_BACKFILL_ENABLED=1
evolve loop ...
```

Or via `CycleRequest.Env`:

```go
req.Env["EVOLVE_BACKFILL_ENABLED"] = "1"
```

---

## Related

- `go/internal/backfill/backfill.go` — `TryExtract` implementation
- `go/internal/core/orchestrator.go` — wiring in `RunCycle` retry loop
- `docs/architecture/adr/0026-self-healing-review-layer.md` — Stage 1 backlog
- `CLAUDE.md` — env-var table entry for `EVOLVE_BACKFILL_ENABLED`
