// Package trustkernel holds black-box tests that PIN the trust-kernel
// invariants — the small set of safety properties the whole evolve-loop relies
// on (ship gate, audit-binding, routing integrity floor, phase-transition
// legality, profile validity). Each test exercises the real exported Go code in
// go/internal/... (never grep over source), is named for the BEHAVIOR it pins
// (never cycle-pegged), and maps to a knowledge/architecture/*.md doc via the
// invariant table in go/docs/testing.md. See PORTING-LEDGER.md for the
// categorization of the legacy go/acs/cycle*/predicates_test.go ports.
package trustkernel
