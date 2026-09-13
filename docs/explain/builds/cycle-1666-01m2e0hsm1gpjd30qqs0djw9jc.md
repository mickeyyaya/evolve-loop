# Build Explanation — Cycle 1666

## Build Binding
- Cycle: 1666
- Base SHA: 02f6c4ef049e0547ffe46c6db59a2a1d96d06896

## Summary
New cycle dossiers now carry a forward-only schema discriminator, and a read-only audit identifies legacy retro-skip claims that conflict with surviving execution receipts.

## Rationale
The discriminator preserves historical records while giving consumers an explicit trust boundary. This is safer than rewriting 134 committed dossiers and satisfies the inbox requirement to choose only one remediation.

## Changed Areas
- `.evolve/inbox/2026-07-30T22-15-00Z-dossier-corpus-carries-retro-mislabel.json` — removes the selected task record from the active inbox as part of the normal consumption lifecycle, preventing it from remaining eligible for duplicate dispatch.
- `.evolve/inbox/consumed/2026-07-30T22-15-00Z-dossier-corpus-carries-retro-mislabel.json` — preserves that same task record in the consumed archive so its acceptance context and dispatch history remain available as cycle evidence.
- `go/cmd/evolve/cmd_dossier.go` — adds the read-only `dossier retro-mislabel` command so operators can re-derive the affected count from corpus records and execution receipts.
- `go/cmd/evolve/main.go` — exposes the new dossier audit in the top-level CLI help so the production surface is discoverable.
- `go/internal/dossier/build.go` — stamps the current discriminator at the sole construction boundary so every newly produced dossier is identifiable.
- `go/internal/dossier/dossier.go` — defines the optional wire field and current version while leaving unversioned legacy records distinguishable as zero.
- `go/internal/dossier/read.go` — centralizes skipped-phase trust classification and cross-checks legacy claims against run artifacts and ledger receipts.
- `schemas/cycle-dossier.schema.json` — declares the optional integer discriminator without invalidating the unversioned historical corpus.

## Design Decisions
Version 2 marks all new records, while absence remains the legacy signal and re-rendering does not backfill it. Any nonzero version is trusted for this specific retro-mislabel boundary so future schema bumps cannot reclassify version 2 as legacy. The consumer returns explicit trusted, contradicted, unverified, or absent states instead of a misleading boolean.

## Verification
The frozen TDD tests exercise version stamping and round-tripping, both receipt sources, wrong-cycle and corrupt-ledger edges, absent-corpus failure, read-only CLI behavior, schema lockstep, and the two producer goldens. The real corpus audit derives 134 candidates, 134 contradicted labels, and zero uncorroborated records.

## Compatibility
Legacy JSON remains parseable and byte-stable when re-rendered because `schema_version` is optional and omitted at zero. Existing dossier fields and command behavior remain unchanged.

## Limitations
The audit intentionally does not rewrite historical dossiers, and malformed legacy dossier files continue to follow the existing best-effort `ReadCommitted` behavior.
