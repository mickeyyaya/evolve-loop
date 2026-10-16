# cmd/apicover

## Purpose

`cmd/apicover` is a thin wrapper. The measurement lives in `internal/apicover`, so the `evolve apicover` subcommand, the in-process API gate of the audit and this binary share one implementation. Nothing rebuilds it at run time. Flags and behaviour: the `internal/apicover` package. Origin: [ADR-0050](../adr/0050-modularization-and-unified-phase-io.md).

## Design

- The `main` package holds no logic beyond the call into `internal/apicover`.

## Invariants

- A change to the measurement belongs in `internal/apicover`, never here.
