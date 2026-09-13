# Build Explanation — Cycle 1663

## Build Binding
- Cycle: 1663
- Base SHA: 6cf877724bedb65dc7c4f8940b2c7b419894f9e3

## Summary
Committed cycle dossiers now preserve an orchestrator-provided system-failure classification in both JSON and Markdown, so a lost landing remains distinguishable from an ordinary warning after runtime artifacts disappear.

## Rationale
The dossier producer already receives all other closeout state, and the canonical `cyclestate.SystemFailureSignal` already defines the required wire fields. Passing that existing value through the two closeout callers is the smallest compatible change and avoids a duplicate record type or a second artifact reader.

## Changed Areas
- `.evolve/inbox/2026-08-23T12-01-00Z-lost-ship-dossier-evidence.json` — removes the resolved task from the active inbox so it is no longer eligible for redispatch after this cycle consumes it.
- `.evolve/inbox/consumed/2026-08-23T12-01-00Z-lost-ship-dossier-evidence.json` — preserves the resolved task and its lifecycle evidence in the consumed inbox instead of deleting the source record.
- `go/internal/core/cycle_closeout.go` — passes the finalized cycle result's system-failure signal into the normal dossier closeout so landing-loss evidence reaches the committed record.
- `go/internal/core/cyclerun_epilogue.go` — passes the same signal through abnormal closeout so both production paths preserve identical failure evidence.
- `go/internal/core/dossier_producer.go` — adds the optional signal to the package-private parameter value and forwards it into dossier construction without changing verdict routing.
- `go/internal/dossier/build.go` — accepts the canonical signal in `BuildOpts` and projects it unchanged onto the assembled dossier.
- `go/internal/dossier/dossier.go` — adds an optional `system_failure` field using the existing cyclestate type, preserving nil-output compatibility through `omitempty`.
- `go/internal/dossier/render.go` — renders category, level, verbatim evidence, and halt state for operators reading the Markdown dossier.
- `schemas/cycle-dossier.schema.json` — declares the optional top-level field and the canonical signal shape so external readers remain aligned with Go serialization.

## Design Decisions
The implementation reuses `cyclestate.SystemFailureSignal` rather than introducing a dossier-specific copy. The field is optional and passed only from the cycle result, so a transient ship error without a detected lost landing cannot fabricate evidence. Existing verdict mapping remains unchanged because this cycle adds durable evidence, not a new classification policy.

## Verification
The normal and abnormal production callers are exercised with real lost-landing artifacts; negative tests cover a landed sibling and byte-identical ordinary PASS output. A bidirectional schema drift test covers the new wire field, affected package suites pass, and restored cycle-1544 ACS bindings execute the same behavior tests.

## Compatibility
Ordinary and nil-signal dossiers retain their previous JSON and Markdown bytes. The new JSON member is optional, and existing verdict values and public function signatures remain unchanged.

## Limitations
The Markdown evidence is rendered verbatim because the deterministic signal owns a bounded operator-facing line; this change does not add a separate evidence truncation or redaction policy.
