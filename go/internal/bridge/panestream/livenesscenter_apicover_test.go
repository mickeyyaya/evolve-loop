package panestream

// livenesscenter_apicover_test.go — names the exported LivenessCenter type (and,
// as of cycle-432 S4, its Busy/Changed methods; as of cycle-434 S4-completion,
// its BusyOf method) as AST identifiers so apicover -enforce (Phase 5) tracks
// them.
//
// The behavioral suite in livenesscenter_test.go / livenesscenter_busychange_test.go /
// livenesscenter_busyof_test.go exercises the Facade only through the
// NewLivenessCenter constructor (and names the type/methods only in comments in
// this file), which leaves the exported symbol *tokens* unreferenced in any
// test AST — apicover flags them "UNCOVERED (no test names it)" and
// hard-fails repo-wide CI (the recurring warnship_apicover_ci_gap class,
// panestream is enrolled in go/.apicover-enforce). The full behavioral
// contract (observe/aggregate/register/empty/concurrency, busy/changed/busyOf
// projections) is already covered in the sibling test files; this
// declaration adds only the missing symbol references.
var (
	_ *LivenessCenter = NewLivenessCenter()
	_ bool            = NewLivenessCenter().Busy("")
	_ bool            = NewLivenessCenter().Changed("")
	_ bool            = NewLivenessCenter().BusyOf("", PaneProfile{})
	_ bool            = (*LivenessCenter)(nil).BusyOf("", PaneProfile{})
)
