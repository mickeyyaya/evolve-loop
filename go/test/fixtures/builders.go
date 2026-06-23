package fixtures

import "github.com/mickeyyaya/evolveloop/go/internal/core"

// Test-data builders (the Object Mother pattern): factories that return a
// valid default value so a test only states the fields it actually cares
// about. They use the repo-idiomatic functional-options style.

// LedgerEntryOption mutates a core.LedgerEntry under construction.
type LedgerEntryOption func(*core.LedgerEntry)

// NewLedgerEntry returns a valid ledger entry with sane defaults, then applies
// opts. Defaults: cycle 1, role "test", kind "phase", exit 0.
func NewLedgerEntry(opts ...LedgerEntryOption) core.LedgerEntry {
	e := core.LedgerEntry{Cycle: 1, Role: "test", Kind: "phase"}
	for _, opt := range opts {
		opt(&e)
	}
	return e
}

func WithCycle(n int) LedgerEntryOption    { return func(e *core.LedgerEntry) { e.Cycle = n } }
func WithRole(r string) LedgerEntryOption  { return func(e *core.LedgerEntry) { e.Role = r } }
func WithKind(k string) LedgerEntryOption  { return func(e *core.LedgerEntry) { e.Kind = k } }
func WithExitCode(c int) LedgerEntryOption { return func(e *core.LedgerEntry) { e.ExitCode = c } }
func WithGitHEAD(h string) LedgerEntryOption {
	return func(e *core.LedgerEntry) { e.GitHEAD = h }
}
func WithEntrySeq(seq int, prevHash string) LedgerEntryOption {
	return func(e *core.LedgerEntry) { e.EntrySeq = seq; e.PrevHash = prevHash }
}
