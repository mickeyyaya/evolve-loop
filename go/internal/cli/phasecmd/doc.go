// Package phasecmd holds the `evolve` CLI handlers for the phase subsystem —
// phase (run one phase in-process, + verify/lint subcommands), phases
// (list/validate/create the catalog), serve-phase (envelope-framed subprocess),
// phase-order, phase-inventory, phase-observer, and phase-watchdog.
//
// HOW: each exported Run* function has the standard subcommand signature
// (args []string, stdin io.Reader, stdout, stderr io.Writer) int and is wired
// into cmd/evolve/registry.go. The in-process runners dispatch through
// internal/phases/registry (built-in phases self-register via blank imports);
// the rest delegate to internal/* packages (phaseinventory, phaseobserver,
// phasewatchdog, phasespec, phasecontract). Shared CLI helpers come from
// cmd/evolve/cmdutil; prompts loaders from cmdutil.NewPromptsLoader.
//
// WHY: cmd/evolve was a 77-handler package main; grouping the phase handlers
// (SRP) shrinks the composition root and makes them directly testable. cmd is a
// fan-in-0 sink, so the extraction carries no import-cycle risk. The phase
// registry's test-snapshot idiom now lives in registry.SnapshotForTest (one
// home shared by every cmd test suite, instead of a per-package copy).
//
// Key exported symbols:
//   - [RunPhase], [RunServePhase] — in-process / subprocess phase execution
//   - [RunPhases] — the user-definable-phases catalog surface
//   - [RunPhaseOrder], [RunPhaseInventory] — registry order + advisor index
//   - [RunPhaseObserver], [RunPhaseWatchdog] — liveness / stall detection
//
// Depends on: cmd/evolve/cmdutil + the internal/* phase implementations.
package phasecmd
