// Package sysexec is the single canonical seam for executing external
// commands (git, gh, go, tmux, …) across the codebase.
//
// Production code depends on the [RunFunc] type and the package-level
// [DefaultRunner]; tests inject an in-memory fake (see
// go/test/fixtures.FakeExec) to avoid the fork/exec wall that otherwise
// dominates the suite's wall-clock time.
//
// sysexec deliberately imports nothing from internal/* (only the standard
// library). That keeps it a leaf package importable by production code, by
// white-box (package foo) tests, AND by the go/test/fixtures harness without
// ever creating an import cycle — fixtures imports core, so the seam could not
// live in core.
//
// The common "run a command and read its stdout" case is served by the
// [Capture], [Output] and [CombinedOutput] helpers so callers need not wire
// io.Writers by hand.
package sysexec
