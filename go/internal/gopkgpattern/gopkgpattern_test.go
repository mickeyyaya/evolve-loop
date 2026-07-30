// gopkgpattern_test.go graduates internal/gopkgpattern into .apicover-enforce:
// every export — ModulePrefix, WholeModule, IsPackagePattern, IsRecursive, Key —
// is named and exercised across the shapes both production callers depend on
// (internal/acssuite run-time scope-lint, internal/evalqualitycheck
// authoring-time flaky-shape lint).
//
// The table is deliberately adversarial about the FALSE-POSITIVE direction: in
// both callers a wrong "yes" is the expensive answer (it demotes a real gate at
// run time, and prints a false claim to a predicate author at authoring time),
// so prose, file paths, URLs and bare words all get explicit rows.
package gopkgpattern

import "testing"

func TestIsPackagePattern(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
		why  string
	}{
		{WholeModule, true, "the bare whole-module sweep, the broadest shape there is"},
		{"./internal/core", true, "single named relative package"},
		{"./internal/bridge/...", true, "recursive relative subtree"},
		{"./cmd/evolve", true, "cmd package"},
		{ModulePrefix + "internal/core", true, "full module import path"},
		{ModulePrefix + "internal/core/...", true, "full module import path, recursive"},
		{"", false, "empty"},
		{"./", false, "bare ./ is too short to name a package"},
		{".", false, "bare dot is not the ./x shape"},
		{"internal/core", false, "no ./ prefix and not under the module prefix"},
		{"go test ./internal/core", false, "prose/argv line — whitespace disqualifies"},
		{"run the\tsuite", false, "whitespace (tab) disqualifies"},
		{"line\none", false, "whitespace (newline) disqualifies"},
		{"./internal/core/gate.go", false, "file path — extension disqualifies"},
		{"./internal/core/gate.go/...", false, "file path with a wildcard suffix still has an extension"},
		{"https://example.com/x", false, "URL, not a package pattern"},
	} {
		if got := IsPackagePattern(tc.in); got != tc.want {
			t.Errorf("IsPackagePattern(%q) = %v, want %v (%s)", tc.in, got, tc.want, tc.why)
		}
	}
}

func TestIsRecursive(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{WholeModule, true},
		{"./internal/bridge/...", true},
		{ModulePrefix + "internal/core/...", true},
		{"./internal/core", false},
		{ModulePrefix + "internal/core", false},
		{"", false},
	} {
		if got := IsRecursive(tc.in); got != tc.want {
			t.Errorf("IsRecursive(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestKey(t *testing.T) {
	for _, tc := range []struct {
		in, want, why string
	}{
		{"./internal/bridge", "internal/bridge", "relative single package"},
		{"./internal/bridge/", "internal/bridge", "trailing slash trimmed"},
		{"./internal/bridge/...", "internal/bridge", "recursive suffix trimmed"},
		{ModulePrefix + "internal/bridge", "internal/bridge", "import path normalizes to the same key"},
		{ModulePrefix + "internal/bridge/...", "internal/bridge", "recursive import path too"},
		{"  ./internal/core  ", "internal/core", "surrounding whitespace trimmed"},
		{WholeModule, "", "the whole-module sweep names no single directory"},
		{"internal/bridge", "", "unrecognized shape"},
		{"", "", "empty"},
	} {
		if got := Key(tc.in); got != tc.want {
			t.Errorf("Key(%q) = %q, want %q (%s)", tc.in, got, tc.want, tc.why)
		}
	}
}

// TestKeyAgreesAcrossSpellings is the property both callers actually rely on:
// three spellings of one package must compare equal, or a touched-set match at
// run time and a known-slow match at authoring time silently disagree.
func TestKeyAgreesAcrossSpellings(t *testing.T) {
	spellings := []string{"./internal/core", "./internal/core/...", ModulePrefix + "internal/core"}
	first := Key(spellings[0])
	if first == "" {
		t.Fatalf("Key(%q) = %q, want a non-empty key", spellings[0], first)
	}
	for _, s := range spellings[1:] {
		if got := Key(s); got != first {
			t.Errorf("Key(%q) = %q, want %q — spellings of one package must compare equal", s, got, first)
		}
	}
}

// TestRecursivePatternsAreAlsoPackagePatterns pins the invariant the callers'
// switch statements assume: IsRecursive is a REFINEMENT of IsPackagePattern, so a
// recursive shape can never slip through a caller that filters on the latter
// first.
func TestRecursivePatternsAreAlsoPackagePatterns(t *testing.T) {
	for _, s := range []string{WholeModule, "./internal/bridge/...", ModulePrefix + "internal/core/..."} {
		if IsRecursive(s) && !IsPackagePattern(s) {
			t.Errorf("%q is recursive but not a package pattern — callers filter on IsPackagePattern first and would skip it", s)
		}
	}
}
