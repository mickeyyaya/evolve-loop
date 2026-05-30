// Package core is the host-agnostic heart of evolve-loop: the cycle
// orchestrator, the phase state machine, and the port interfaces every
// adapter implements.
//
// Orchestrator.RunCycle (orchestrator.go) drives one cycle — Scout discovers
// work, the pipeline turns RED tests GREEN, an adversarial Auditor judges the
// result, and Ship commits it or the cycle ends without shipping. The
// orchestrator is a pure driver: it never performs I/O or branches on a
// feature flag directly, delegating every side effect through injected ports
// (Storage, Ledger, PhaseRunner, Bridge, Guard, Observer, plus the routing
// PhaseAdvisor) wired by functional options at the composition root.
//
// Phase (phase.go) enumerates the lifecycle stages and backs a legality oracle
// (CanTransition) so only valid phase transitions occur; the routing advisor
// may propose insertions/skips but the kernel disposes. ShipError (errors.go)
// is the structured failure that makes Ship a pure executor: a ShipError is
// intercepted and routed to the Debugger recovery phase rather than aborting
// the cycle. State, CycleState, and LedgerEntry (ports.go) are the persisted
// records the Storage and Ledger ports own.
package core
