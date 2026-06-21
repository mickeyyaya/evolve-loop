// Package semvercheck reports whether a string is a strict semantic version.
//
// HOW: a single [IsSemver] predicate matching the numeric X.Y.Z form via one
// compiled regexp — no pre-release or build-metadata suffixes (the codebase
// only mints and validates plain release tags).
//
// WHY: release/version code (version-bump, release-consistency, marketplace
// poll) needs ONE canonical, allocation-free semver check instead of ad-hoc
// regexps duplicated across call sites (DRY). A zero-dependency leaf so any
// layer can validate a version string without taking a dependency.
//
// Key exported symbols:
//   - [IsSemver] — true iff s is a strict X.Y.Z numeric version
//
// Depends on: standard library only.
package semvercheck
