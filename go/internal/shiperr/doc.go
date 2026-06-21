// Package shiperr is the structured ship→orchestrator error protocol: the
// [ShipError] value object plus its Code/Class/Stage vocabularies that the ship
// phase emits when it cannot execute and the orchestrator matches to decide
// recovery (debugger / re-run / block).
//
// HOW: a single value type [ShipError] (Code + Class + Stage + Message + Debug
// map) built by [NewShipError] and recovered from a wrapped error chain by
// [AsShipError] via errors.As. [ShipError.Error] renders the stable single-line
// "[CODE/class @stage] message"; [ShipError.DebugString] renders the Debug map
// with sorted keys for deterministic ledger/debugger output. The Code/Class/Stage
// constants are a stable wire vocabulary (ledger entries, the debugger persona,
// and tests key off the exact strings). Pure data + formatting — no I/O, no
// dependency on any other internal package.
//
// WHY: this is a SHARED PROTOCOL between two layers — the ship phase CONSTRUCTS
// ShipErrors, the core orchestrator MATCHES them. Hosting it in a zero-dependency
// leaf (rather than in core, where it used to live) means BOTH ship→leaf and
// core→leaf import edges point one way, so the protocol type can never form an
// import cycle regardless of which side grows. (DIP: depend on a shared
// abstraction, not on each other.) core re-exports every symbol via type aliases
// + const re-declarations + thin func wrappers so existing call sites are
// unchanged; new code may depend on this leaf directly.
//
// Key exported symbols:
//   - [ShipError] (+ [ShipError.Error], [ShipError.DebugString]) — the value object
//   - [NewShipError], [AsShipError] — constructor + error-chain extractor
//   - [ShipErrorClass] + ShipClass* — severity vocabulary (drives recursion/framing)
//   - [ShipStage] + Stage* — which ship stage failed
//   - [ShipErrorCode] + Code* — the precise, stable failure identities
//
// Depended on by: internal/core (the only direct importer today — it re-exports
// the protocol; the ship phase and others construct/match ShipErrors through
// those core re-exports until call sites migrate to this leaf directly).
// Depends on: nothing (standard library only).
package shiperr
