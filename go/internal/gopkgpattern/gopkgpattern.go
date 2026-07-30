// Package gopkgpattern is the shared scope rule for recognizing and normalizing
// `go test` PACKAGE PATTERN strings found in Go SOURCE — specifically, the rule
// the two ACS predicate lints must never disagree about:
//
//   - internal/acssuite (scopelint.go) applies it at RUN time, demoting a cycle
//     predicate that sweeps packages outside the cycle's git-derived touched set;
//   - internal/evalqualitycheck (flakylint.go) applies it at AUTHORING time,
//     flagging suite-scope shells before the predicate ever enters the corpus.
//
// Both previously carried private copies (isPackagePattern/patternKey vs
// isFlakyPkgPattern/flakyPkgKey). Two copies of one rule is exactly the drift
// this leaf removes: a shape the authoring lint stops recognizing would silently
// stop being demoted at run time too, and vice versa. The gcpolicy-from-gc
// precedent: extract the pure decision, leave the policy in the callers.
//
// Zero dependencies beyond the stdlib by design — the authoring-time pre-flight
// must not drag in the suite runner (policy/verifylock), and the run-time lint
// must not drag in the eval-quality CLI.
//
// Scope note: this is NOT the repo's only package-path normalizer. internal/
// ciparity, internal/phases/audit and internal/changedpkgs each normalize
// patterns for their own purpose (CI parity, changed-package derivation) over
// different inputs — git paths and `go list` output rather than string literals
// in agent-authored source. Folding those in would couple unrelated decisions;
// this leaf's contract is deliberately just the AST-literal scope rule.
package gopkgpattern

import (
	"path/filepath"
	"strings"
)

// ModulePrefix is the import-path prefix of this repo's Go module. A string
// starting with it is a package reference, not prose.
const ModulePrefix = "github.com/mickeyyaya/evolve-loop/go/"

// WholeModule is the bare recursive whole-module sweep — the broadest, most
// false-red-prone `go test` argument there is. Callers that need to treat it
// specially (acssuite maps it to a never-in-scope key) compare against this
// constant rather than re-spelling the literal.
const WholeModule = "./..."

// IsPackagePattern reports whether s is shaped like a `go test` package
// argument. Deliberately conservative — in BOTH callers a false positive is
// costlier than a false negative (it demotes a real gate at run time, and prints
// a false claim to a predicate author at authoring time), so only unambiguous
// shapes match:
//
//   - "./x", "./x/y/..." relative patterns and the bare "./..." whole-module sweep;
//   - module import paths under ModulePrefix.
//
// File paths (anything with an extension), prose (anything with whitespace), and
// URLs never match.
func IsPackagePattern(s string) bool {
	if s == "" || strings.ContainsAny(s, " \t\n") {
		return false
	}
	if s == WholeModule {
		return true
	}
	if filepath.Ext(strings.TrimSuffix(s, "/...")) != "" {
		return false
	}
	if strings.HasPrefix(s, "./") && len(s) > 2 {
		return true
	}
	return strings.HasPrefix(s, ModulePrefix)
}

// IsRecursive reports whether the pattern expands to a whole subtree ("./..." or
// any "<pkg>/..." suffix) rather than a single named package. A recursive sweep
// builds and loads every package beneath it, so narrowing it with `-run` does
// not bound its cost — which is why callers treat it differently from a single
// known-slow package.
func IsRecursive(s string) bool {
	return s == WholeModule || strings.HasSuffix(s, "/...")
}

// Key normalizes a pattern to a module-relative directory key, so
// "./internal/bridge/...", "./internal/bridge" and the full import path all
// compare equal ("internal/bridge"). Unrecognized shapes — and the bare "./..."
// whole-module sweep, which names no single directory — yield "".
func Key(p string) string {
	p = strings.TrimSpace(p)
	if p == WholeModule {
		return ""
	}
	p = strings.TrimSuffix(p, "/...")
	p = strings.TrimSuffix(p, "/")
	switch {
	case strings.HasPrefix(p, "./"):
		return strings.TrimPrefix(p, "./")
	case strings.HasPrefix(p, ModulePrefix):
		return strings.TrimPrefix(p, ModulePrefix)
	}
	return ""
}
