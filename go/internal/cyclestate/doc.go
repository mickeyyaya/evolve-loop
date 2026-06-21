// Package cyclestate is the irreducible domain vocabulary of an evolve-loop
// cycle: the typed phase identity and the verdict/outcome label sets that every
// other layer (orchestrator, router, phase runners, EGPS predicates, ledger)
// reads and serializes.
//
// HOW: it holds only pure, dependency-free value types — [Phase] (a string-backed
// stage identity with its constant set, [Phase.String] and [Phase.IsValid]) and
// the verdict/cycle-outcome string constants with their [IsVerdict] guard. It has
// NO behavior that touches I/O, config, routing, or git; the cycle state machine
// and its transition rules deliberately stay in package core (they depend on
// router/config and so are not leaf-pure).
//
// WHY: these identifiers are the most-depended-on symbols in the module (~1k
// references). Hoisting them into a zero-dependency leaf applies the Stable-
// Dependencies Principle — the foundation everything imports now depends on
// nothing itself, so it cannot drag a cycle through the graph. package core
// re-exports the symbols via type aliases + const re-declarations, so existing
// call sites are unchanged; new code may depend on this leaf directly. The
// constants are a byte-identity boundary: their wire strings are pinned by test
// because ledger/state JSON and the EGPS gate match on them verbatim.
//
// Key exported symbols:
//   - [Phase] (+ Phase* constants, [Phase.String], [Phase.IsValid]) — stage identity
//   - Verdict* constants + [IsVerdict] — per-phase outcome vocabulary
//   - CycleOutcome* constants — cycle-level FinalVerdict labels
//
// Depended on by: internal/core (re-exports), and any package needing the
// vocabulary directly; depends on: nothing (standard library only).
package cyclestate
