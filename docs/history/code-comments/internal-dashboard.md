# Comment history: `internal/dashboard`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/dashboard/apicover_named_test.go:16` — above `func TestAPICoverNamedExports(t *testing.T) {`

```text
// TestAPICoverNamedExports names and EXERCISES every exported symbol of this
// package (ADR-0069 new-package graduation) through the shapes its real
// consumer — cmd_dashboard.go — relies on: the one-shot Collect for
// --snapshot, New/Options/Server for the served mode, the artifact reader the
// detail page calls, and the closed state vocabulary the page colours by.
```

### `go/internal/dashboard/cycle.go:260` — above `func assignState(cs CycleSummary, loop LoopStatus) CycleSummary {`

```text
// assignState derives the closed-vocabulary state type and its human name.
// "halted" is the ADR-0072 SYSTEM level (policy.LevelSystem), never a literal;
// the brake and the running cycle both come from the loop status.
```

### `go/internal/dashboard/doc.go:1` — above `package dashboard`

```text
// Package dashboard is the read-only observability surface behind
// `evolve dashboard` (ADR-0095): a stdlib-only local web UI that renders the
// loop's on-disk state — `.evolve/cycle-state.json`, the per-cycle run
// workspaces under `.evolve/runs/`, the committed dossiers under
// `knowledge-base/cycles/`, and the inbox — as a live page a human can read
// to answer: is the loop alive, what is queued, what did each cycle do, what
// went wrong, and how is the ship rate trending.
//
// Design rules the package holds itself to (see the design spec in
// docs/superpowers/specs/2026-09-02-ship-rate-harness-and-pipeline-dashboard-design.md):
//
//   - Every read is best-effort. An absent or half-written artifact zero-values
//     its field and lands in Snapshot.Warnings; it never fails the page.
//   - The package never takes the loop's flock sidecars and never writes. It
//     relies on the writers' atomic-rename discipline: a reader sees the old
//     file or the new one, never a torn one.
//   - No fsnotify (the module is vendored stdlib-only): change detection is a
//     mtime fingerprint poll that feeds one Server-Sent-Events stream.
//   - Everything rendered is LLM-authored text. The server ships a static
//     shell and JSON; the client places content into the DOM with textContent
//     only. Artifact reads are allowlisted by name, symlink-safe
//     (reportdoc.OpenRegularNoFollow) and size-capped.
//   - Beliefs owned elsewhere are imported, not re-declared: cycle workspace
//     paths (core.RunWorkspacePath), cycle-state path
//     (core.ResolveCycleStatePath), liveness (runlease.Fresh), inbox items
//     (inboxbatch.LoadDir), dossiers (dossier.ParseJSON), phase timing
//     (phasetiming.Read). Phase ORDER is taken from the observed timing log,
//     not from a fourth copy of the canonical list.
```

### `go/internal/dashboard/findings_test.go:8` — above `const reportBothShapes = '# Audit Report — Cycle 1605 (round 3)`

```text
// The auditor writes findings as `### <ID> (<SEVERITY>[, qualifier]) — <title>`
// under `## Issues`. Both heading shapes observed live (cycle-1605 final round
// and cycle-1604 round 1) must parse; the verdict may be declared as
// `## Verdict` + a bold line or inline `**Verdict: X**`.
```

### `go/internal/dashboard/model.go:185` — above `DurationMS int64 'json:"duration_ms"'`

```text
// DurationMS is wall-clock; Attempt is the attempt count recorded by the
// dispatcher; Round is the 1-based occurrence of this phase name within the
// cycle (audit round 2 = the second "audit" entry).
```

### `go/internal/dashboard/model.go:216` — above `Rounds []AuditRound 'json:"rounds,omitempty"'`

```text
// Rounds is the immutable repair-round history, round 1 first.
```

### `go/internal/dashboard/plan_test.go:3` — above `import (`

```text
// plan_test.go — the per-cycle phase plan the board renders: the registry's
// mandatory set (config.mandatory_phases — the set the router's floor
// enforces), every phase the cycle ran with a status
// (pass/warn/fail/ongoing/pending/unreached/skipped), the contract-gate mark
// from the cycle's Signal Center stream, the counts, and how the advisor's
// proposal fared. Operator ask, 2026-09-14: "how many phases are required,
// how many passed, which is ongoing" — on the board.
```

### `go/internal/dashboard/plan_test.go:141` — above `func TestPhasePlan_WarnSkippedAndRepeatedOngoing(t *testing.T) {`

```text
// A WARN phase is not a passed phase; a mandatory phase the cycle went past
// without running is skipped, not pending; a phase both executed and ongoing
// (audit round 2 dispatched after round 1 failed) is ongoing with its rounds.
```
