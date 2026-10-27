# internal/phaseorder

## Purpose

`internal/phaseorder` ports the legacy `list-phase-order.sh`. `List` reads `docs/architecture/phase-registry.json` and emits the phases in order.

## Design

- `List` returns `HardcodedOrder`, the canonical sequence, when the registry file does not exist. X18d removed the switch that turned the registry off.
- `List` returns an error only when the registry exists and cannot be parsed. This keeps the exit-code-1 contract of the bash original.

## Invariants

- A path that exists but cannot be read (a directory gives EISDIR) takes the read-error branch. `TestList_StatOKButReadFails` pins it without a seam.
