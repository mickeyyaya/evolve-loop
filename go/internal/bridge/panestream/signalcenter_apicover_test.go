package panestream

// signalcenter_apicover_test.go — names the exported SignalCenter type as an AST
// identifier so apicover -enforce (Phase 5) tracks it.
//
// The behavioral suite in signalcenter_test.go exercises the Facade only through
// the NewSignalCenter constructor (and names the type only in comments), which
// leaves the exported type *token* unreferenced in any test AST — apicover flags
// it "UNCOVERED (no test names it)" and hard-fails repo-wide CI (the recurring
// warnship_apicover_ci_gap class, panestream is enrolled in go/.apicover-enforce).
// The full behavioral contract (observe/aggregate/register/empty/concurrency) is
// already covered there; this declaration adds only the missing type reference.
var _ *SignalCenter = NewSignalCenter()
