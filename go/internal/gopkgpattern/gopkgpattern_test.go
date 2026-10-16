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

func TestRecursivePatternsAreAlsoPackagePatterns(t *testing.T) {
	for _, s := range []string{WholeModule, "./internal/bridge/...", ModulePrefix + "internal/core/..."} {
		if IsRecursive(s) && !IsPackagePattern(s) {
			t.Errorf("%q is recursive but not a package pattern — callers filter on IsPackagePattern first and would skip it", s)
		}
	}
}
