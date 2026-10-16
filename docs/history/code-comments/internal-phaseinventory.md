# Comment history: `internal/phaseinventory`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero K1

### `go/internal/phaseinventory/phaseinventory.go:1` — above `package phaseinventory`

```text
// Package phaseinventory builds .evolve/phase-inventory.json — the structured
// index of every phase the pipeline can run: built-in registry entries plus
// user/plugin phases from all discovery roots (ADR-0038). It is the phase
// counterpart of skillinventory and follows the same cache semantics (mtime
// freshness vs TTL; --force bypass).
//
// The inventory is ADVISORY-ONLY: the orchestrator's composition root always
// re-reads the live merged catalog for routing and execution, so a stale
// inventory can weaken an advisor SELECT hint but can never break a cycle.
// Consumers: the advisor's catalog cards, `evolve phases list`, and any LLM
// reading the index offline to learn what phases exist before minting one.
```
