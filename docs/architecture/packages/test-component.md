# test/component

## Purpose

`test/component` wires several real `internal/` adapters together against temp-directory state through the fixtures harness, with no CLI subagent and no git subprocess. It runs in the fast default suite. Tiers: [testing.md](../../../go/docs/testing.md).

## Design

- **Pattern for a new component test.**
  1. `ws := fixtures.NewWorkspace(t).Build()` gives an isolated temp `.evolve/`.
  2. Construct the real adapters against `ws.EvolveDir`.
  3. Act, then rebuild or re-read, and assert the cross-cutting property.

## Invariants

- **Durability is on disk, not in the adapter.** The storage test writes through one adapter instance and reloads through a second instance on the same `.evolve` directory. No single-function unit test covers that property.
