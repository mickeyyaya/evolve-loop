// Package paralleltest is the one parallel-by-default guard of the package test trees.
// Each package test reads its own sources and lists its serial tests; the guard flags a test that is
// neither parallel nor listed, and a parallel test that changes process state directly, through a
// helper or through a method on a package-level value.
// See docs/explain/builds/cycle-1869-01m4rvyfk91nmt5wksrgbwsq88.md.
package paralleltest
