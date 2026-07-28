// Package cycle1158 holds the ACS predicates for the cycle-1158 eval
// (.evolve/evals/land-cycle-1156-lifecycle-seam-with-audit-fixes.md). The
// predicates themselves live in predicates_test.go behind `//go:build acs`.
//
// This file carries NO build tag on purpose. ACS predicates are environment
// assertions ("the ADR documents X", "the export is retired"), not unit tests,
// so they must not run in the ordinary suite — but a directory whose only file
// is tag-excluded fails to build at all ("build constraints exclude all Go
// files"), which turns a correctly-tagged package into a red `go test ./...`.
// One untagged, empty package clause makes the package build away to nothing
// without `-tags acs` instead of failing, which is the property
// go/acs/cycle1160's predicate 007 asserts.
package cycle1158
